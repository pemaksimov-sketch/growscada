package library

import (
	"context"
	"errors"
	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/apiclient"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
	"testing"
)

func TestLibraryOperations_SaveRenamePreservesUnappliedHTMLAndDelete(t *testing.T) {
	api := apiclient.NewMemory()
	l := NewLibrary("")
	l.client = api
	var failures []error
	l.tasks.Report = func(err error) { failures = append(failures, err) }
	l.createItem(app.Context{})
	id := uuid.MustParse(l.selectedID)
	if len(l.widgetTypes) != 1 || l.editedHTML != defaultHTML {
		t.Fatal("new type editor was not populated")
	}
	l.editedHTML = "<div>saved</div>"
	l.applyChanges(app.Context{})
	stored, err := api.GetWidgetType(context.Background(), id)
	if err != nil || stored.Version != 2 || stored.HtmlTemplate != l.editedHTML {
		t.Fatalf("apply failed: %+v %v", stored, err)
	}
	l.editedHTML = "<div>unsaved</div>"
	l.startEditing(l.selectedID, "New Widget Type")
	l.editingName = "renamed"
	l.commitEdit(app.Context{})
	stored, err = api.GetWidgetType(context.Background(), id)
	if err != nil || stored.Name != "renamed" || stored.HtmlTemplate != "<div>saved</div>" || l.editedHTML != "<div>unsaved</div>" {
		t.Fatalf("rename published or erased an unapplied edit: %+v %v", stored, err)
	}
	l.deleteItem(app.Context{})
	if l.selectedID != "" || len(l.widgetTypes) != 0 || l.editedHTML != "" {
		t.Fatal("deleted editor remained selected")
	}
	if len(failures) != 0 {
		t.Fatal(failures)
	}
}

func TestLibraryOperations_ConflictPreservesDraftRefreshesVersionAndRetries(t *testing.T) {
	api := apiclient.NewMemory()
	l := NewLibrary("")
	l.client = api
	var failures []error
	l.tasks.Report = func(err error) { failures = append(failures, err) }
	l.createItem(app.Context{})
	id := uuid.MustParse(l.selectedID)
	input := updateInput(l.widgetTypes[0])
	input.Name = "remote rename"
	if _, err := api.UpdateWidgetType(context.Background(), id, input); err != nil {
		t.Fatal(err)
	}
	l.editedHTML = "<div>local draft</div>"
	l.applyChanges(app.Context{})
	if len(failures) != 1 || !errors.Is(failures[0], apiclient.ErrConflict) {
		t.Fatalf("missing conflict: %v", failures)
	}
	if l.editedHTML != "<div>local draft</div>" || l.widgetTypes[0].Version != 2 || l.editedName != "remote rename" {
		t.Fatal("conflict lost draft or left stale metadata")
	}
	l.applyChanges(app.Context{})
	stored, err := api.GetWidgetType(context.Background(), id)
	if err != nil || stored.Version != 3 || stored.HtmlTemplate != l.editedHTML {
		t.Fatal("retry failed")
	}
	// A rejected deletion must keep the selected editor.
	sc, err := api.CreateScene(context.Background(), apiv0.CreateSceneRequest{Name: "used", Width: 100, Height: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.CreateWidget(context.Background(), sc.ID, apiv0.CreateWidgetRequest{TypeID: id, SceneVersion: 1}); err != nil {
		t.Fatal(err)
	}
	l.deleteItem(app.Context{})
	if l.selectedID != id.String() || len(failures) != 2 || !errors.Is(failures[1], apiclient.ErrConflict) {
		t.Fatal("rejected delete cleared the editor")
	}
}
