package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kipitix/growscada/internal/server/application/appdto"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/scene"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/repositories"
)

type failingReconciliationRepository struct {
	scene.SceneRepository
	failID  id.ID[scene.Scene]
	failure error
}

func (r *failingReconciliationRepository) Save(ctx context.Context, sc scene.Scene) (scene.Scene, error) {
	if sc.ID() == r.failID && r.failure != nil {
		return nil, r.failure
	}
	return r.SceneRepository.Save(ctx, sc)
}

func TestWidgetReconciliationPolicy_PartialFailureRetriesOnlyRemainingScenes(t *testing.T) {
	cleanScenes(t)
	cleanWidgetTypes(t)
	cleanOutbox(t)
	ctx := context.Background()
	wtSvc, scSvc := newWidgetTypeService(), newSceneService()
	input := appdto.WidgetTypeInput{Name: "retry sensor", HtmlTemplate: "<div/>", ScriptLanguage: "javascript", DefaultWidth: 100, DefaultHeight: 100, InputPorts: []appdto.InputPort{{Name: "value"}}}
	wt, err := wtSvc.CreateWidgetType(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	var scenes []appdto.Scene
	var widgets []appdto.Widget
	for _, name := range []string{"first", "second"} {
		sc, err := scSvc.CreateScene(ctx, appdto.SceneInput{Name: name, Width: 800, Height: 600})
		if err != nil {
			t.Fatal(err)
		}
		w, err := scSvc.CreateWidget(ctx, sc.ID, sc.Version, appdto.WidgetInput{Name: "bound", Width: 100, Height: 100, TypeID: wt.ID, PortBindings: []appdto.PortBinding{{PortName: "value", TagID: createBoundTag(t)}}})
		if err != nil {
			t.Fatal(err)
		}
		scenes = append(scenes, sc)
		widgets = append(widgets, w)
	}
	cleanOutbox(t)
	input.InputPorts = nil
	if _, err := wtSvc.UpdateWidgetType(ctx, wt.ID, wt.Version, input); err != nil {
		t.Fatalf("type save must succeed independently of policy: %v", err)
	}
	before, err := scSvc.FindWidgetByID(ctx, scenes[0].ID, widgets[0].ID)
	if err != nil || len(before.PortBindings) != 1 {
		t.Fatalf("service reconciled synchronously: %v %v", before, err)
	}
	failure := errors.New("scene storage temporarily unavailable")
	repo := &failingReconciliationRepository{SceneRepository: repositories.NewSceneRepositoryPostgres(testDB), failID: id.NewID(id.IDWithUUID[scene.Scene](scenes[1].ID)), failure: failure}
	var delivered []event.Event
	d := reconciliationDispatcher(func(e event.Event) { delivered = append(delivered, e) }, repo)
	if _, err := d.DrainOnce(ctx); !errors.Is(err, failure) {
		t.Fatalf("want retryable failure, got %v", err)
	}
	first, _ := scSvc.FindWidgetByID(ctx, scenes[0].ID, widgets[0].ID)
	second, _ := scSvc.FindWidgetByID(ctx, scenes[1].ID, widgets[1].ID)
	if len(first.PortBindings) != 0 || len(second.PortBindings) != 1 {
		t.Fatalf("wrong partial result: %v %v", first.PortBindings, second.PortBindings)
	}
	var pending int
	if err := testDB.QueryRow("SELECT count(*) FROM policy_outbox WHERE NOT completed").Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("retry not durable: %d %v", pending, err)
	}
	// Restart with a healthy repository. The first Scene must keep its version.
	d = reconciliationDispatcher(func(e event.Event) { delivered = append(delivered, e) }, repo.SceneRepository)
	if _, err := d.DrainOnce(ctx); err != nil {
		t.Fatal(err)
	}
	after, _ := scSvc.FindWidgetByID(ctx, scenes[0].ID, widgets[0].ID)
	second, _ = scSvc.FindWidgetByID(ctx, scenes[1].ID, widgets[1].ID)
	if after.SceneVersion != first.SceneVersion || len(second.PortBindings) != 0 {
		t.Fatalf("retry not idempotent: %d -> %d, second=%v", first.SceneVersion, after.SceneVersion, second.PortBindings)
	}
	counts := map[string]int{}
	for _, e := range delivered {
		if w, ok := e.(scene.WidgetEvent); ok && e.Type() == event.EventTypeWidgetUpdated {
			counts[w.SceneID().String()]++
		}
	}
	for _, sc := range scenes {
		if counts[sc.ID.String()] != 1 {
			t.Fatalf("scene events must be delivered once after retry: %v", counts)
		}
	}
}
