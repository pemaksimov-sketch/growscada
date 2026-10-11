package library

import (
	"context"
	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

const defaultHTML = `<div class="widget"></div>`
const defaultScript = "function update() {\n}"

func (l *Library) createItem(ctx app.Context) {
	input := apiv0.CreateWidgetTypeRequest{Name: "New Widget Type", HtmlTemplate: defaultHTML, Script: defaultScript, ScriptLanguage: "javascript", DefaultWidth: 120, DefaultHeight: 60, InputPorts: []apiv0.InputPort{}}
	uiutil.Request(l.tasks, func(ctx context.Context) (apiv0.CreateWidgetTypeResponse, error) {
		return l.client.CreateWidgetType(ctx, input)
	}, func(result apiv0.CreateWidgetTypeResponse) {
		l.selectedID = result.ID.String()
		l.tasks.Store("library:selectedID", l.selectedID)
		l.editedName = input.Name
		l.editedHTML = input.HtmlTemplate
		l.editedScript = input.Script
		l.editedScriptLang = input.ScriptLanguage
		l.editedInputValues = make(map[string]string)
		l.editedInputPorts = nil
		l.loadList(l.compoCtx)
	}, nil)
}
func (l *Library) deleteItem(ctx app.Context) {
	if l.selectedID == "" {
		return
	}
	deletedID := l.selectedID
	uiutil.Request(l.tasks, func(ctx context.Context) (apiv0.WidgetTypeResponse, error) {
		id, err := uuid.Parse(deletedID)
		if err != nil {
			return apiv0.WidgetTypeResponse{}, err
		}
		return l.client.DeleteWidgetType(ctx, id)
	}, func(apiv0.WidgetTypeResponse) {
		if l.selectedID == deletedID {
			l.clearSelection()
			l.tasks.Store("library:selectedID", "")
		}
		l.loadList(l.compoCtx)
	}, nil)
}
func updateInput(item widgetTypeItem) apiv0.UpdateWidgetTypeRequest {
	ports := item.InputPorts
	if ports == nil {
		ports = []apiv0.InputPort{}
	}
	return apiv0.UpdateWidgetTypeRequest{Name: item.Name, HtmlTemplate: item.HtmlTemplate, Script: item.Script, ScriptLanguage: item.ScriptLanguage, DefaultWidth: item.DefaultWidth, DefaultHeight: item.DefaultHeight, InputPorts: ports, Version: item.Version}
}
func (l *Library) updateItem(id uuid.UUID, input apiv0.UpdateWidgetTypeRequest) {
	uiutil.Request(l.tasks, func(ctx context.Context) (apiv0.UpdateWidgetTypeResponse, error) {
		return l.client.UpdateWidgetType(ctx, id, input)
	}, func(result apiv0.UpdateWidgetTypeResponse) {
		for i := range l.widgetTypes {
			if l.widgetTypes[i].ID == id && l.widgetTypes[i].Version < result.Version {
				l.widgetTypes[i].Version = result.Version
			}
		}
		l.loadList(l.compoCtx)
	}, func() { l.loadList(l.compoCtx) })
}
func (l *Library) applyChanges(ctx app.Context) {
	current, found := l.findItem(l.selectedID)
	if !found {
		return
	}
	input := updateInput(current)
	input.Name = l.editedName
	input.HtmlTemplate = l.editedHTML
	input.Script = l.editedScript
	input.ScriptLanguage = l.editedScriptLang
	input.InputPorts = append([]apiv0.InputPort{}, l.editedInputPorts...)
	l.updateItem(current.ID, input)
}
func (l *Library) commitEdit(ctx app.Context) {
	if l.editingID == "" {
		return
	}
	id, name := l.editingID, l.editingName
	l.editingID = ""
	l.editingName = ""
	found, exists := l.findItem(id)
	if !exists {
		return
	}
	for i := range l.widgetTypes {
		if l.widgetTypes[i].ID == found.ID {
			l.widgetTypes[i].Name = name
			break
		}
	}
	if l.selectedID == id {
		l.editedName = name
	}
	input := updateInput(found)
	input.Name = name
	l.updateItem(found.ID, input)
}
