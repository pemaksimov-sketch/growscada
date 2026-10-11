package application_test

import (
	"context"
	"github.com/kipitix/growscada/internal/server/application"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/scene"
	"testing"

	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/outbox"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/repositories"
	"github.com/kipitix/growscada/internal/server/interface/eventbus"
)

func cleanOutbox(t *testing.T) {
	t.Helper()
	if _, err := testDB.ExecContext(context.Background(), "DELETE FROM policy_outbox; DELETE FROM outbox"); err != nil {
		t.Fatalf("cleanOutbox: %v", err)
	}
}

func reconciliationDispatcher(deliver func(event.Event), scenes scene.SceneRepository) *outbox.Dispatcher {
	d := outbox.NewDispatcher(testDB, deliver)
	p := application.NewWidgetReconciliationPolicy(repositories.NewWidgetTypeRepositoryPostgres(testDB), scenes)
	d.RegisterPolicy("reconcile-widget-bindings", event.EventTypeWidgetTypeUpdated, p.Handle)
	return d
}

func drainReconciliation(t *testing.T, scenes scene.SceneRepository) []event.Event {
	t.Helper()
	if scenes == nil {
		scenes = repositories.NewSceneRepositoryPostgres(testDB)
	}
	var delivered []event.Event
	d := reconciliationDispatcher(func(e event.Event) { delivered = append(delivered, e) }, scenes)
	if _, err := d.DrainOnce(context.Background()); err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	return delivered
}

// deliveredOnCommit empties the outbox and returns an EventBus that receives
// the outbox's events right after each commit of a repository built with the
// returned option: the dispatcher drains the outbox in the committing call,
// so a test sees the events as soon as the service returns.
func deliveredOnCommit(t *testing.T) (eventbus.EventBus, repositories.Option) {
	t.Helper()
	cleanOutbox(t)
	bus := eventbus.NewEventBus()
	d := outbox.NewDispatcher(testDB, bus.Publish)
	return bus, repositories.NotifyOnCommit(func() {
		if _, err := d.DrainOnce(context.Background()); err != nil {
			t.Errorf("DrainOnce: %v", err)
		}
	})
}
