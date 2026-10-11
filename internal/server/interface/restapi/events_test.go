package restapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/scene"
	"github.com/kipitix/growscada/internal/server/domain/tag"
	"github.com/kipitix/growscada/internal/server/domain/version"
	"github.com/kipitix/growscada/internal/server/interface/eventbus"
	"github.com/kipitix/growscada/internal/server/interface/restapi"
)

// blockingResponseWriter lets a test simulate an SSE client whose second
// write (its first *event*, after the initial ": connected" comment) stalls
// forever, e.g. a dead TCP connection — so the handler's backpressure
// handling can be exercised without a real network client.
type blockingResponseWriter struct {
	header     http.Header
	mu         sync.Mutex
	buf        bytes.Buffer
	writeCount int
	startOnce  sync.Once
	started    chan struct{}
	unblock    chan struct{}
}

func newBlockingResponseWriter() *blockingResponseWriter {
	return &blockingResponseWriter{
		header:  make(http.Header),
		started: make(chan struct{}),
		unblock: make(chan struct{}),
	}
}

func (w *blockingResponseWriter) Header() http.Header { return w.header }
func (w *blockingResponseWriter) WriteHeader(int)     {}

func (w *blockingResponseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.writeCount++
	n := w.writeCount
	w.mu.Unlock()

	if n == 2 {
		w.startOnce.Do(func() { close(w.started) })
		<-w.unblock
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *blockingResponseWriter) Flush() {}

// syncRecorder wraps httptest.ResponseRecorder with a mutex. The handler
// under test writes to it from its own goroutine (started by
// runEventsHandler) while the test goroutine polls its body/headers via
// waitFor; httptest.ResponseRecorder gives no such guarantee on its own,
// since its Write/Body/Header aren't synchronized against concurrent use.
type syncRecorder struct {
	mu  sync.Mutex
	rec *httptest.ResponseRecorder
}

func newSyncRecorder() *syncRecorder {
	return &syncRecorder{rec: httptest.NewRecorder()}
}

func (s *syncRecorder) Header() http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rec.Header()
}

func (s *syncRecorder) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rec.Write(p)
}

func (s *syncRecorder) WriteHeader(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rec.WriteHeader(code)
}

func (s *syncRecorder) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rec.Flush()
}

// body returns the response body accumulated so far; safe to call
// concurrently with the handler still writing.
func (s *syncRecorder) body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rec.Body.String()
}

func (s *syncRecorder) code() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rec.Code
}

func runEventsHandler(t *testing.T, h *restapi.EventsHandlers, w http.ResponseWriter) (cancel func(), done <-chan struct{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v0/events", nil)
	ctx, cancelFn := context.WithCancel(req.Context())
	req = req.WithContext(ctx)

	doneCh := make(chan struct{})
	go func() {
		h.GetEvents(w, req)
		close(doneCh)
	}()
	return cancelFn, doneCh
}

func waitFor(t *testing.T, d time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}

func TestEventsHandlers_WidgetEventsCarrySceneID(t *testing.T) {
	bus := eventbus.NewEventBus()
	h := restapi.NewEventsHandler(bus, 100)
	defer h.Close()
	rec := newSyncRecorder()
	cancel, done := runEventsHandler(t, h, rec)
	defer func() { cancel(); <-done }()
	wid := id.NewID[scene.Widget]()
	sid := id.NewID[scene.Scene]()
	waitFor(t, time.Second, func() bool { return strings.Contains(rec.body(), ": connected") })
	for _, ev := range []event.Event{scene.NewWidgetCreatedEvent(wid, sid), scene.NewWidgetUpdatedEvent(wid, sid), scene.NewWidgetDeletedEvent(wid, sid)} {
		bus.Publish(ev)
	}
	waitFor(t, time.Second, func() bool { return strings.Contains(rec.body(), "widget_deleted") })
	count := 0
	for _, msg := range sseEventMessages(t, rec.body()) {
		if !strings.HasPrefix(msg["type"].(string), "widget_") {
			continue
		}
		count++
		if msg["id"] != wid.String() || msg["scene_id"] != sid.String() {
			t.Errorf("wrong widget owner: %#v", msg)
		}
	}
	if count != 3 {
		t.Fatalf("got %d widget events", count)
	}
}

func TestEventsHandlers_GetEvents_SetsSSEHeaders(t *testing.T) {
	bus := eventbus.NewEventBus()
	h := restapi.NewEventsHandler(bus, 100)

	rec := newSyncRecorder()
	cancel, done := runEventsHandler(t, h, rec)
	waitFor(t, time.Second, func() bool { return strings.Contains(rec.body(), ": connected") })
	cancel()
	<-done

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}
}

func TestEventsHandlers_GetEvents_StreamsPublishedEvent(t *testing.T) {
	bus := eventbus.NewEventBus()
	h := restapi.NewEventsHandler(bus, 100)

	rec := newSyncRecorder()
	cancel, done := runEventsHandler(t, h, rec)
	waitFor(t, time.Second, func() bool { return strings.Contains(rec.body(), ": connected") })

	tagID := id.NewID[tag.Tag]()
	bus.Publish(tag.NewTagCreatedEvent(tagID))

	waitFor(t, time.Second, func() bool { return strings.Contains(rec.body(), "tag_created") })
	cancel()
	<-done

	body := rec.body()
	if !strings.Contains(body, `"type":"tag_created"`) {
		t.Errorf("expected body to contain tag_created event, got %q", body)
	}
	if !strings.Contains(body, tagID.String()) {
		t.Errorf("expected body to contain tag id %s, got %q", tagID, body)
	}
}

// sseEventMessages parses every `data:` line of an SSE body as JSON.
func sseEventMessages(t *testing.T, body string) []map[string]any {
	t.Helper()
	var msgs []map[string]any
	for _, line := range strings.Split(body, "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			t.Fatalf("invalid event JSON %q: %v", data, err)
		}
		msgs = append(msgs, m)
	}
	return msgs
}

func TestEventsHandlers_GetEvents_TagUpdatedCarriesTagState(t *testing.T) {
	bus := eventbus.NewEventBus()
	h := restapi.NewEventsHandler(bus, 100)

	rec := newSyncRecorder()
	cancel, done := runEventsHandler(t, h, rec)
	waitFor(t, time.Second, func() bool { return strings.Contains(rec.body(), ": connected") })

	name, _ := tag.NewTagName("pressure")
	value, _ := tag.TagTypeInteger.NewTagValue(int64(42))
	aVersion, _ := version.New(version.WithNumber[tag.Tag](7))
	aTag, err := tag.ReconstituteTag(id.NewID[tag.Tag](), name, tag.TagTypeInteger, value, tag.TagQualityUncertain, aVersion)
	if err != nil {
		t.Fatalf("NewTag: %v", err)
	}
	bus.Publish(tag.NewTagCreatedEvent(aTag.ID()))
	bus.Publish(tag.NewTagUpdatedEvent(aTag))

	waitFor(t, time.Second, func() bool { return strings.Contains(rec.body(), "tag_updated") })
	cancel()
	<-done

	var created, updated map[string]any
	for _, m := range sseEventMessages(t, rec.body()) {
		switch m["type"] {
		case "tag_created":
			created = m
		case "tag_updated":
			updated = m
		}
	}
	if created == nil || updated == nil {
		t.Fatalf("expected tag_created and tag_updated events, got %q", rec.body())
	}
	if _, ok := created["tag"]; ok {
		t.Errorf("tag_created must not carry a tag, got %v", created)
	}

	want := map[string]any{
		"id":      aTag.ID().String(),
		"name":    "pressure",
		"type":    "integer",
		"value":   "42",
		"quality": "uncertain",
		"version": float64(7),
	}
	got, ok := updated["tag"].(map[string]any)
	if !ok {
		t.Fatalf("tag_updated: expected tag object, got %v", updated["tag"])
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("tag_updated tag.%s: expected %v, got %v", k, v, got[k])
		}
	}
}

func TestEventsHandlers_GetEvents_PublishesConnectAndDisconnectEvents(t *testing.T) {
	bus := eventbus.NewEventBus()
	h := restapi.NewEventsHandler(bus, 100)

	var mu sync.Mutex
	var seen []event.EventType
	bus.Subscribe(event.EventTypeClientConnected, func(e event.Event) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, e.Type())
	})
	bus.Subscribe(event.EventTypeClientDisconnected, func(e event.Event) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, e.Type())
	})

	rec := newSyncRecorder()
	cancel, done := runEventsHandler(t, h, rec)
	waitFor(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen) >= 1
	})
	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 || seen[0] != event.EventTypeClientConnected || seen[1] != event.EventTypeClientDisconnected {
		t.Errorf("expected [connected, disconnected], got %v", seen)
	}
}

func TestEventsHandlers_GetEvents_MultipleClientsEachReceiveBroadcast(t *testing.T) {
	bus := eventbus.NewEventBus()
	h := restapi.NewEventsHandler(bus, 100)

	rec1 := newSyncRecorder()
	cancel1, done1 := runEventsHandler(t, h, rec1)
	rec2 := newSyncRecorder()
	cancel2, done2 := runEventsHandler(t, h, rec2)

	waitFor(t, time.Second, func() bool {
		return strings.Contains(rec1.body(), ": connected") && strings.Contains(rec2.body(), ": connected")
	})

	bus.Publish(scene.NewSceneCreatedEvent(id.NewID[scene.Scene]()))

	waitFor(t, time.Second, func() bool {
		return strings.Contains(rec1.body(), "scene_created") && strings.Contains(rec2.body(), "scene_created")
	})

	cancel1()
	cancel2()
	<-done1
	<-done2
}

// TestEventsHandlers_Publish_NoClientsDoesNotAllocate guards the fix for the
// CODE_REVIEW.md finding that eventHub.broadcast used to marshal every
// domain event to JSON synchronously in the publishing goroutine (i.e. the
// REST request path) even with zero SSE tabs open. No GetEvents call is
// made here, so the hub never gains a client.
func TestEventsHandlers_Publish_NoClientsDoesNotAllocate(t *testing.T) {
	bus := eventbus.NewEventBus()
	_ = restapi.NewEventsHandler(bus, 100) // subscribes the hub to bus; no client connects

	e := tag.NewTagCreatedEvent(id.NewID[tag.Tag]())

	allocs := testing.AllocsPerRun(200, func() {
		bus.Publish(e)
	})

	if allocs > 0 {
		t.Errorf("expected zero allocations broadcasting with no SSE clients, got %.2f allocs/op", allocs)
	}
}

// BenchmarkEventsHandlers_Publish_NoClients measures the same no-client
// broadcast path for manual profiling; run with `make bench`.
func BenchmarkEventsHandlers_Publish_NoClients(b *testing.B) {
	bus := eventbus.NewEventBus()
	_ = restapi.NewEventsHandler(bus, 100)

	e := tag.NewTagCreatedEvent(id.NewID[tag.Tag]())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(e)
	}
}

// Close must end an idle stream without requiring request cancellation.
func TestEventsHandlers_Close_StopsForwardingNewEvents(t *testing.T) {
	bus := eventbus.NewEventBus()
	h := restapi.NewEventsHandler(bus, 100)

	rec := newSyncRecorder()
	cancel, done := runEventsHandler(t, h, rec)
	defer cancel()
	waitFor(t, time.Second, func() bool { return strings.Contains(rec.body(), ": connected") })

	tagID := id.NewID[tag.Tag]()
	bus.Publish(tag.NewTagCreatedEvent(tagID))
	waitFor(t, time.Second, func() bool { return strings.Contains(rec.body(), "tag_created") })

	h.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close did not terminate the idle SSE handler")
	}

	sceneID := id.NewID[scene.Scene]()
	bus.Publish(scene.NewSceneCreatedEvent(sceneID))

	if strings.Contains(rec.body(), "scene_created") {
		t.Error("expected no events to be forwarded after Close(), but scene_created was received")
	}

	cancel()
	<-done
}

func TestEventsHandlers_GetEvents_RejectsAfterClose(t *testing.T) {
	h := restapi.NewEventsHandler(eventbus.NewEventBus(), 100)
	h.Close()
	rec := newSyncRecorder()
	cancel, done := runEventsHandler(t, h, rec)
	defer cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("a new SSE request remained open after Close")
	}
	if rec.code() != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.code())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected JSON problem response, got %q", ct)
	}
}

func TestEventsHandlers_Close_ConcurrentWithDisconnectAndPublish(t *testing.T) {
	bus := eventbus.NewEventBus()
	h := restapi.NewEventsHandler(bus, 100)
	var cancels []func()
	var handlers []<-chan struct{}
	for i := 0; i < 10; i++ {
		rec := newSyncRecorder()
		cancel, done := runEventsHandler(t, h, rec)
		defer cancel()
		cancels = append(cancels, cancel)
		handlers = append(handlers, done)
		waitFor(t, time.Second, func() bool { return strings.Contains(rec.body(), ": connected") })
	}
	start := make(chan struct{})
	var workers sync.WaitGroup
	for _, cancel := range cancels {
		workers.Add(3)
		go func() { defer workers.Done(); <-start; h.Close() }()
		go func() { defer workers.Done(); <-start; cancel() }()
		go func() {
			defer workers.Done()
			<-start
			bus.Publish(tag.NewTagCreatedEvent(id.NewID[tag.Tag]()))
		}()
	}
	close(start)
	workersDone := make(chan struct{})
	go func() { workers.Wait(); close(workersDone) }()
	select {
	case <-workersDone:
	case <-time.After(time.Second):
		t.Fatal("concurrent Close, disconnect and Publish deadlocked")
	}
	for _, done := range handlers {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("SSE handler did not terminate")
		}
	}
	h.Close()
}

// TestEventsHandlers_GetEvents_RejectsBeyondMaxClients guards the fix for
// the CODE_REVIEW.md finding that GET /api/v0/events accepted an unbounded
// number of concurrent SSE connections. A small maxClients (2) keeps the
// test from needing to open 100 real connections.
func TestEventsHandlers_GetEvents_RejectsBeyondMaxClients(t *testing.T) {
	bus := eventbus.NewEventBus()
	h := restapi.NewEventsHandler(bus, 2)

	rec1 := newSyncRecorder()
	cancel1, done1 := runEventsHandler(t, h, rec1)
	rec2 := newSyncRecorder()
	cancel2, done2 := runEventsHandler(t, h, rec2)
	waitFor(t, time.Second, func() bool {
		return strings.Contains(rec1.body(), ": connected") && strings.Contains(rec2.body(), ": connected")
	})

	rec3 := newSyncRecorder()
	cancel3, done3 := runEventsHandler(t, h, rec3)
	<-done3
	cancel3()

	if rec3.code() != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when over the connection limit, got %d", rec3.code())
	}
	if ct := rec3.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected a JSON problem response, got Content-Type %q, body %q", ct, rec3.body())
	}

	// Freeing a slot must let a new connection through.
	cancel1()
	<-done1

	rec4 := newSyncRecorder()
	cancel4, done4 := runEventsHandler(t, h, rec4)
	waitFor(t, time.Second, func() bool { return strings.Contains(rec4.body(), ": connected") })
	cancel4()
	<-done4

	cancel2()
	<-done2
}

// TestEventsHandlers_GetEvents_SlowClientIsDroppedWithoutBlockingPublish is
// the key architectural guarantee: a client that never drains its buffer
// must not block EventBus.Publish for the rest of the application, and must
// eventually be disconnected once its buffer overflows.
func TestEventsHandlers_GetEvents_SlowClientIsDroppedWithoutBlockingPublish(t *testing.T) {
	bus := eventbus.NewEventBus()
	h := restapi.NewEventsHandler(bus, 100)

	w := newBlockingResponseWriter()
	cancel, done := runEventsHandler(t, h, w)
	defer cancel()

	// Registering the client (hub.addClient) races with this goroutine, so
	// keep publishing single events until one actually reaches the client's
	// Write call (proving it was registered) and stalls there.
	deadline := time.After(time.Second)
publishUntilStarted:
	for {
		bus.Publish(tag.NewTagCreatedEvent(id.NewID[tag.Tag]()))
		select {
		case <-w.started:
			break publishUntilStarted
		case <-deadline:
			t.Fatal("handler never reached its first event write")
		case <-time.After(5 * time.Millisecond):
		}
	}

	// Flood well past the per-client buffer capacity. Publish must not block
	// even though the client's consumer is stuck in Write.
	publishDone := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			bus.Publish(tag.NewTagCreatedEvent(id.NewID[tag.Tag]()))
		}
		close(publishDone)
	}()

	select {
	case <-publishDone:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a slow/stalled SSE client")
	}

	// Release the stalled write; the handler should drain and exit because
	// the hub closed its channel once the buffer overflowed.
	close(w.unblock)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not exit after its client was dropped for overflow")
	}
}
