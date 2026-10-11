// Package httpserver adds deterministic shutdown of browser preconnections to
// net/http. A connection that has not begun a request must not delay shutdown.
package httpserver

import (
	"context"
	"net"
	"net/http"
	"sync"
)

type Server struct {
	*http.Server
	mu             sync.Mutex
	newConnections map[net.Conn]struct{}
	draining       bool
}

func New(addr string, handler http.Handler) *Server {
	s := &Server{newConnections: make(map[net.Conn]struct{})}
	s.Server = &http.Server{Addr: addr, Handler: handler, ConnState: s.connectionState}
	return s
}

func (s *Server) connectionState(conn net.Conn, state http.ConnState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state == http.StateNew {
		if s.draining {
			_ = conn.Close()
		} else {
			s.newConnections[conn] = struct{}{}
		}
	} else {
		delete(s.newConnections, conn)
	}
}

// net/http normally waits more than five seconds before considering StateNew
// connections idle. Chromium opens these speculatively on project navigation.
// Close them before the normal Shutdown, which still drains active requests.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.draining = true
	for conn := range s.newConnections {
		_ = conn.Close()
		delete(s.newConnections, conn)
	}
	s.mu.Unlock()
	return s.Server.Shutdown(ctx)
}
