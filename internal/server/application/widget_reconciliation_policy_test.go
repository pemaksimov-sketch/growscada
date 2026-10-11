package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kipitix/growscada/internal/server/application"
	"github.com/kipitix/growscada/internal/server/application/appdto"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/project"
	"github.com/kipitix/growscada/internal/server/domain/scene"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/outbox"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/repositories"
)

func TestProjectReconciliation_UpdatesEveryProjectAndPreservesEventScope(t *testing.T) {
	cleanScenes(t)
	cleanWidgetTypes(t)
	cleanOutbox(t)
	ctx := context.Background()
	projects := application.NewProjectService(repositories.NewProjectRepositoryPostgres(testDB, project.DefaultOrganizationID()), project.DefaultOrganizationID())
	types := repositories.NewWidgetTypeRepositoryPostgres(testDB)
	wtSvc := newWidgetTypeService()
	input := appdto.WidgetTypeInput{Name: "shared sensor", HtmlTemplate: "<div/>", ScriptLanguage: "javascript", DefaultWidth: 100, DefaultHeight: 100, InputPorts: []appdto.InputPort{{Name: "value"}}}
	wt, err := wtSvc.CreateWidgetType(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	var scopes []application.WidgetBindingScope
	var services []application.SceneService
	var scenes []appdto.Scene
	var widgets []appdto.Widget
	owners := map[string]string{}
	for _, name := range []string{"policy A", "policy B"} {
		p, err := projects.Create(ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = testDB.Exec("DELETE FROM projects WHERE id=$1", p.ID) })
		r := repositories.ForProject(testDB, id.NewID(id.IDWithUUID[project.Project](p.ID)))
		scopes = append(scopes, application.WidgetBindingScope{Scenes: r.Scenes(), Tags: application.NewLegacyTagTypeLookup(r.Tags())})
		tg, err := application.NewTagService(r.Tags()).CreateTag(ctx, appdto.CreateTagInput{Name: "same.name", Type: "integer", Value: "1", Quality: "good"})
		if err != nil {
			t.Fatal(err)
		}
		svc := application.NewSceneService(r.Scenes(), types, r.Tags())
		sc, err := svc.CreateScene(ctx, appdto.SceneInput{Name: "same scene", Width: 800, Height: 600})
		if err != nil {
			t.Fatal(err)
		}
		w, err := svc.CreateWidget(ctx, sc.ID, sc.Version, appdto.WidgetInput{Name: "bound", Width: 100, Height: 100, TypeID: wt.ID, PortBindings: []appdto.PortBinding{{PortName: "value", TagID: tg.ID}}})
		if err != nil {
			t.Fatal(err)
		}
		services, scenes, widgets = append(services, svc), append(scenes, sc), append(widgets, w)
		owners[sc.ID.String()] = p.ID.String()
	}
	cleanOutbox(t)
	input.InputPorts = nil
	if _, err := wtSvc.UpdateWidgetType(ctx, wt.ID, wt.Version, input); err != nil {
		t.Fatal(err)
	}
	var delivered []event.Event
	d := outbox.NewDispatcher(testDB, func(e event.Event) { delivered = append(delivered, e) })
	policy := application.NewProjectWidgetReconciliationPolicy(types, func(context.Context) ([]application.WidgetBindingScope, error) { return scopes, nil })
	d.RegisterPolicy("reconcile-widget-bindings", event.EventTypeWidgetTypeUpdated, policy.Handle)
	if _, err := d.DrainOnce(ctx); err != nil {
		t.Fatal(err)
	}
	for i, svc := range services {
		w, err := svc.FindWidgetByID(ctx, scenes[i].ID, widgets[i].ID)
		if err != nil || len(w.PortBindings) != 0 {
			t.Fatalf("project %d not reconciled: %+v %v", i, w, err)
		}
	}
	counts := map[string]int{}
	for _, e := range delivered {
		w, ok := e.(scene.WidgetEvent)
		if !ok || e.Type() != event.EventTypeWidgetUpdated {
			continue
		}
		owner := e.(interface{ ScopeProjectID() string }).ScopeProjectID()
		if owner != owners[w.SceneID().String()] {
			t.Fatalf("reconciliation event crossed projects: %s", owner)
		}
		counts[owner]++
	}
	for _, owner := range owners {
		if counts[owner] != 1 {
			t.Fatalf("missing scoped event: %v", counts)
		}
	}
}

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
