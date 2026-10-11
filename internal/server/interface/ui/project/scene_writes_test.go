package project

import (
	"context"
	"errors"
	"testing"

	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/apiclient"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

func TestAutosave_UnchangedFieldsDoNotWrite(t *testing.T) {
	p := NewProject("")
	api := apiclient.NewMemory()
	p.client = api
	p.createScene(app.Context{})
	p.saveSceneProperties(app.Context{})
	p.startEditingScene(p.selectedSceneID, p.scenes[0].Name)
	p.commitSceneEdit(app.Context{})
	if p.currentSceneVersion() != 1 {
		t.Fatal("unchanged scene wrote a new version")
	}
	wt, err := api.CreateWidgetType(context.Background(), apiv0.CreateWidgetTypeRequest{Name: "box", DefaultWidth: 120, DefaultHeight: 60})
	if err != nil {
		t.Fatal(err)
	}
	p.createWidget(app.Context{}, wt.ID.String(), 10.12345, 20.54321)
	p.saveWidgetName(app.Context{})
	p.saveWidgetGeometry(app.Context{})
	if p.currentSceneVersion() != 2 {
		t.Fatal("unchanged widget wrote a new version")
	}
}

func TestAutosave_SerializesSceneAndWidgetWithLatestOwnVersion(t *testing.T) {
	p := NewProject("")
	api := apiclient.NewMemory()
	p.client = api
	p.createScene(app.Context{})
	sceneID := p.scenes[0].ID
	wt, err := api.CreateWidgetType(context.Background(), apiv0.CreateWidgetTypeRequest{Name: "box", DefaultWidth: 120, DefaultHeight: 60})
	if err != nil {
		t.Fatal(err)
	}
	p.createWidget(app.Context{}, wt.ID.String(), 10, 20)
	widgetID := p.widgets[0].ID
	var tasks []func()
	p.tasks.Async = func(f func()) { tasks = append(tasks, f) }
	p.tasks.Report = func(err error) { t.Fatalf("self-conflict: %v", err) }
	p.editingScenePropsName = "first"
	p.saveSceneProperties(app.Context{})
	p.editingScenePropsBG = "second"
	p.saveSceneProperties(app.Context{})
	p.editingWidgetName = "third"
	p.saveWidgetName(app.Context{})
	if len(tasks) != 1 {
		t.Fatalf("writes overlap: %d requests", len(tasks))
	}
	// SSE/list responses racing with the first write must not revert the queued
	// background or the widget editor while the remaining writes are pending.
	stale, _ := api.ListScenes(context.Background())
	p.setScenes(app.Context{}, stale)
	if p.editingScenePropsBG != "second" {
		t.Fatal("reload erased queued background")
	}
	p.loadWidgets(app.Context{})
	for len(tasks) > 0 {
		f := tasks[0]
		tasks = tasks[1:]
		f()
	}
	sc, err := api.GetScene(context.Background(), sceneID)
	if err != nil || sc.Name != "first" || sc.BackgroundHTML != "second" || sc.Version != 5 {
		t.Fatalf("lost rapid scene edits: %+v %v", sc, err)
	}
	w, err := api.GetWidget(context.Background(), sceneID, widgetID)
	if err != nil || w.Name != "third" || w.SceneVersion != 5 {
		t.Fatalf("lost queued widget edit: %+v %v", w, err)
	}
	if p.currentSceneVersion() != 5 || len(p.sceneWrites) != 0 {
		t.Fatal("queue did not drain")
	}
}

func TestAutosave_ExternalConflictCancelsDependentWrites(t *testing.T) {
	p := NewProject("")
	api := apiclient.NewMemory()
	p.client = api
	p.createScene(app.Context{})
	sc := p.scenes[0]
	var tasks []func()
	var failures []error
	p.tasks.Async = func(f func()) { tasks = append(tasks, f) }
	p.tasks.Report = func(err error) { failures = append(failures, err) }
	p.editingScenePropsName = "local first"
	p.saveSceneProperties(app.Context{})
	p.editingScenePropsBG = "local queued"
	p.saveSceneProperties(app.Context{})
	_, err := api.UpdateScene(context.Background(), sc.ID, apiv0.UpdateSceneRequest{Name: "remote", Width: sc.Width, Height: sc.Height, BackgroundHTML: "remote background", Version: sc.Version})
	if err != nil {
		t.Fatal(err)
	}
	for len(tasks) > 0 {
		f := tasks[0]
		tasks = tasks[1:]
		f()
	}
	if len(failures) != 1 || !errors.Is(failures[0], apiclient.ErrConflict) {
		t.Fatalf("missing conflict: %v", failures)
	}
	stored, _ := api.GetScene(context.Background(), sc.ID)
	if stored.Version != 2 || stored.Name != "remote" || stored.BackgroundHTML != "remote background" {
		t.Fatalf("queue overwrote external change: %+v", stored)
	}
	if p.editingScenePropsName != "remote" || p.editingScenePropsBG != "remote background" || len(p.sceneWrites) != 0 {
		t.Fatal("conflict did not restore server fields")
	}
	p.editingScenePropsName = "retry"
	p.saveSceneProperties(app.Context{})
	for len(tasks) > 0 {
		f := tasks[0]
		tasks = tasks[1:]
		f()
	}
	stored, _ = api.GetScene(context.Background(), sc.ID)
	if stored.Version != 3 || stored.Name != "retry" {
		t.Fatal("retry retained a stale version")
	}
}
