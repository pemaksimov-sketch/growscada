package repositories_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/tag"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/outbox"
)

func cleanPolicyOutbox(t *testing.T) {
	t.Helper()
	cleanOutbox(t)
	if _, err := testDB.Exec("DELETE FROM policy_outbox"); err != nil {
		t.Fatal(err)
	}
}

func appendPolicyEvent(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	appendDeleted(t, ctx, tx, id.NewID[tag.Tag]())
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func pendingPolicies(t *testing.T) int {
	t.Helper()
	var n int
	if err := testDB.QueryRow("SELECT count(*) FROM policy_outbox WHERE NOT completed").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPolicyOutbox_FailurePersistsAcrossRestartWithoutBlockingOtherSubscribers(t *testing.T) {
	cleanPolicyOutbox(t)
	appendPolicyEvent(t)
	var delivered recorder
	failure := errors.New("temporary policy failure")
	d := outbox.NewDispatcher(testDB, delivered.deliver)
	d.RegisterPolicy("failing", event.EventTypeTagDeleted, func(context.Context, event.Event) error { return failure })
	other := 0
	d.RegisterPolicy("healthy", event.EventTypeTagDeleted, func(context.Context, event.Event) error { other++; return nil })
	if n, err := d.DrainOnce(context.Background()); n != 1 || !errors.Is(err, failure) {
		t.Fatalf("drain=%d %v", n, err)
	}
	if other != 1 || pendingPolicies(t) != 1 || len(outboxTypes(t)) != 0 {
		t.Fatalf("SSE or healthy policy blocked: other=%d pending=%d", other, pendingPolicies(t))
	}
	// A new dispatcher proves the retry depends on persisted state, not memory.
	retried := 0
	d = outbox.NewDispatcher(testDB, delivered.deliver)
	d.RegisterPolicy("failing", event.EventTypeTagDeleted, func(context.Context, event.Event) error { retried++; return nil })
	if n, err := d.DrainOnce(context.Background()); n != 0 || err != nil {
		t.Fatalf("restart drain=%d %v", n, err)
	}
	if retried != 1 || pendingPolicies(t) != 0 || !slices.Equal(delivered.types(), []string{"tag_deleted"}) {
		t.Fatalf("retry=%d delivered=%v", retried, delivered.types())
	}
}

func TestPolicyOutbox_SlowSubscriberDoesNotBlockLiveEvents(t *testing.T) {
	cleanPolicyOutbox(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	var delivered recorder
	d := outbox.NewDispatcher(testDB, delivered.deliver, outbox.WithPollInterval(10*time.Millisecond))
	d.RegisterPolicy("blocked", event.EventTypeTagDeleted, func(ctx context.Context, _ event.Event) error {
		select {
		case <-started:
		default:
			close(started)
		}
		<-ctx.Done()
		return ctx.Err()
	})
	done := make(chan struct{})
	go func() { defer close(done); d.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	appendPolicyEvent(t)
	d.Notify()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("policy did not start")
	}
	appendPolicyEvent(t)
	d.Notify()
	deadline := time.Now().Add(5 * time.Second)
	for len(delivered.types()) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if len(delivered.types()) != 2 {
		t.Fatalf("live delivery blocked: %v", delivered.types())
	}
}

func TestPolicyOutbox_CompletedSubscriptionIsNotResurrectedBySourceRetry(t *testing.T) {
	cleanPolicyOutbox(t)
	appendPolicyEvent(t)
	// Simulate the crash window: policy acknowledged, source SSE not deleted.
	_, err := testDB.Exec(`INSERT INTO policy_outbox (source_seq,subscriber,type,payload,completed)
	 SELECT seq,'acked',type,payload,true FROM outbox`)
	if err != nil {
		t.Fatal(err)
	}
	var delivered recorder
	d := outbox.NewDispatcher(testDB, delivered.deliver)
	d.RegisterPolicy("acked", event.EventTypeTagDeleted, func(context.Context, event.Event) error { t.Error("acknowledged policy ran again"); return nil })
	if n, err := d.DrainOnce(context.Background()); n != 1 || err != nil {
		t.Fatalf("drain=%d %v", n, err)
	}
	var remaining int
	if err := testDB.QueryRow("SELECT count(*) FROM policy_outbox").Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("acknowledgement was not collected: %d", remaining)
	}
}
