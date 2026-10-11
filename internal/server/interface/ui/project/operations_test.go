package project

import (
	"context"
	"errors"
	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/apiclient"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
	"testing"
)

func TestProjectOperations_UseTypedAdapterAndSceneVersion(t *testing.T) {
	ctx := context.Background()
	api := apiclient.NewMemory()
	p := NewProject("")
	p.client = api
	var failures []error
	p.tasks.Report = func(err error) { failures = append(failures, err) }
	p.createScene(app.Context{})
	if p.selectedSceneID == "" || len(p.scenes) != 1 || p.currentSceneVersion() != 1 {
		t.Fatalf("scene creation lost selection: %+v", p.scenes)
	}
	wt, err := api.CreateWidgetType(ctx, apiv0.CreateWidgetTypeRequest{Name: "counter", DefaultWidth: 120, DefaultHeight: 60})
	if err != nil {
		t.Fatal(err)
	}
	p.loadWidgetTypes(app.Context{})
	p.createWidget(app.Context{}, wt.ID.String(), 10, 20)
	if len(p.widgets) != 1 || p.currentSceneVersion() != 2 || p.selectedWidgetID != p.widgets[0].ID.String() {
		t.Fatalf("widget creation lost aggregate version: %+v", p.widgets)
	}
	p.editingWidgetName = "renamed"
	p.saveWidgetName(app.Context{})
	w, err := api.GetWidget(ctx, p.scenes[0].ID, p.widgets[0].ID)
	if err != nil || w.Name != "renamed" || p.currentSceneVersion() != 3 {
		t.Fatalf("widget save failed: %+v %v", w, err)
	}
	p.applyNewSceneVersion(p.selectedSceneID, 2)
	if p.currentSceneVersion() != 3 || p.widgets[0].SceneVersion != 3 {
		t.Fatal("late response rolled aggregate version back")
	}
	p.editingScenePropsName = "renamed scene"
	p.saveSceneProperties(app.Context{})
	if p.currentSceneVersion() != 4 || p.widgets[0].SceneVersion != 4 {
		t.Fatal("scene save failed to propagate shared version")
	}
	p.deleteWidget(app.Context{}, p.selectedWidgetID)
	if len(p.widgets) != 0 || p.selectedWidgetID != "" || p.currentSceneVersion() != 5 {
		t.Fatal("widget delete did not clear selection/version")
	}
	p.deleteScene(app.Context{}, p.selectedSceneID)
	if len(p.scenes) != 0 || p.selectedSceneID != "" {
		t.Fatal("scene delete retained invalid selection")
	}
	if len(failures) != 0 {
		t.Fatalf("unexpected client errors: %v", failures)
	}
}

func TestProjectOperations_ConflictReloadsServerAndAllowsRetry(t *testing.T) {
	api := apiclient.NewMemory()
	p := NewProject("")
	p.client = api
	var failures []error
	p.tasks.Report = func(err error) { failures = append(failures, err) }
	p.createScene(app.Context{})
	sc := p.scenes[0]
	_, err := api.UpdateScene(context.Background(), sc.ID, apiv0.UpdateSceneRequest{Name: "remote", Width: sc.Width, Height: sc.Height, Version: sc.Version})
	if err != nil {
		t.Fatal(err)
	}
	p.editingScenePropsName = "stale local save"
	p.saveSceneProperties(app.Context{})
	if len(failures) != 1 || !errors.Is(failures[0], apiclient.ErrConflict) {
		t.Fatalf("missing conflict: %v", failures)
	}
	if p.currentSceneVersion() != 2 || p.editingScenePropsName != "remote" {
		t.Fatalf("conflict did not reload server state: %+v", p.scenes)
	}
	p.editingScenePropsName = "retry"
	p.saveSceneProperties(app.Context{})
	if p.currentSceneVersion() != 3 {
		t.Fatal("retry remained on stale version")
	}
}

func TestProjectTagOperations_KeepRejectedFormAndClearDeletedSelection(t *testing.T) {
	api := apiclient.NewMemory()
	p := NewProject("")
	p.client = api
	var failures []error
	p.tasks.Report = func(err error) { failures = append(failures, err) }
	p.newTagName = "counter"
	p.newTagType = "integer"
	p.creatingTag = true
	p.createTag(app.Context{})
	if p.creatingTag || len(p.tags) != 1 || p.tags[0].Value != "0" || p.selectedTagID == "" {
		t.Fatal("tag create failed")
	}
	storedID := p.selectedTagID
	p.newTagName = "counter"
	p.newTagType = "integer"
	p.creatingTag = true
	p.createTag(app.Context{})
	if !p.creatingTag || p.newTagName != "counter" || len(failures) != 1 || !errors.Is(failures[0], apiclient.ErrConflict) {
		t.Fatal("rejected form was discarded")
	}
	p.deleteTag(app.Context{})
	if len(p.tags) != 0 || p.selectedTagID != "" {
		t.Fatal("deleted tag remained selected")
	}
	if _, err := api.GetTag(context.Background(), uuid.MustParse(storedID)); !errors.Is(err, apiclient.ErrNotFound) {
		t.Fatal("tag was not deleted")
	}
}
