package httpserver

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdown_ClosesPreconnectionsAndDrainsActiveRequests(t *testing.T) {
	started, finish := make(chan struct{}), make(chan struct{})
	s := New("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-finish
		_, _ = io.WriteString(w, "finished")
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	go s.Serve(listener)
	// This socket never sends an HTTP request, just like browser preconnect.
	preconnect, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer preconnect.Close()
	response := make(chan string, 1)
	go func() {
		r, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			response <- err.Error()
			return
		}
		defer r.Body.Close()
		data, err := io.ReadAll(r.Body)
		if err != nil {
			response <- err.Error()
			return
		}
		response <- string(data)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	shutdown := make(chan error, 1)
	go func() { shutdown <- s.Shutdown(ctx) }()
	// A pending real request is preserved during graceful shutdown.
	select {
	case err := <-shutdown:
		t.Fatalf("active request was not drained: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(finish)
	select {
	case body := <-response:
		if body != "finished" {
			t.Fatalf("active request lost: %s", body)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case err := <-shutdown:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("preconnection delayed shutdown")
	}
	_ = preconnect.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := preconnect.Read(make([]byte, 1)); err == nil {
		t.Fatal("preconnection is still open")
	}
}
