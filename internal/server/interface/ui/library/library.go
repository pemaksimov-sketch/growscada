package library

import (
	"context"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/apiclient"

	"github.com/maxence-charriere/go-app/v10/pkg/app"

	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/interface/ui/eventlog"
	"github.com/kipitix/growscada/internal/server/interface/ui/uidto"
	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
)

// ── Component ─────────────────────────────────────────────────────────────────

type Library struct {
	app.Compo
	// compoCtx is the component's own context, captured in OnMount. Async work
	// dispatches its result through it: an event handler's ctx belongs to the
	// element that fired the event, and go-app silently drops Dispatch once that
	// element is unmounted (e.g. a tab that turns into its rename input), which
	// would lose the result and leave a Reloader loading forever.
	compoCtx          app.Context
	ThemeMode         string // exported so go-app detects theme changes and re-renders
	client            apiclient.Server
	tasks             uiutil.Tasks
	widgetTypes       []widgetTypeItem
	libraries         []apiv0.LibraryResponse
	selectedLibraryID string
	releases          []apiv0.LibraryReleaseResponse
	loading           bool
	reloader          uiutil.Reloader
	selectedID        string
	editedName        string
	editedHTML        string
	editedScript      string
	editedInputValues map[string]string
	editedScriptLang  string
	editedInputPorts  []uidto.InputPortDTO
	editingID         string
	editingName       string
	// add-port form state
	newPortName string
	newPortDesc string
	newPortType string
}

func NewLibrary(apiServerURL string) *Library {
	return &Library{client: apiclient.New(apiServerURL, http.DefaultClient)}
}

func (l *Library) OnMount(ctx app.Context) {
	l.compoCtx = ctx
	l.tasks = uiutil.BrowserTasks(ctx)
	ctx.ObserveState("theme", &l.ThemeMode)
	ctx.LocalStorage().Get("library:selectedID", &l.selectedID)
	ctx.LocalStorage().Get("library:selectedLibraryID", &l.selectedLibraryID)
	l.loadLibraries(ctx)

	ctx.Handle(eventlog.ActionServerEvent, func(ctx app.Context, a app.Action) {
		if ev, ok := a.Value.(eventlog.ServerEvent); ok && eventlog.ReloadTargetsFor(ev.Type).WidgetTypes {
			l.loadList(ctx)
		}
	})
}

func (l *Library) loadLibraries(ctx app.Context) {
	api, ok := l.client.(apiclient.LibraryServer)
	if !ok {
		l.loadList(ctx)
		return
	}
	uiutil.Request(l.tasks, func(ctx context.Context) ([]apiv0.LibraryResponse, error) { return api.ListLibraries(ctx) }, func(items []apiv0.LibraryResponse) {
		l.libraries = items
		found := false
		for _, item := range items {
			if item.ID.String() == l.selectedLibraryID {
				found = true
				break
			}
		}
		if !found && len(items) > 0 {
			l.selectedLibraryID = items[0].ID.String()
			l.tasks.Store("library:selectedLibraryID", l.selectedLibraryID)
		}
		l.loadList(l.compoCtx)
		l.loadReleases(l.compoCtx)
	}, nil)
}

func (l *Library) loadReleases(ctx app.Context) {
	api, ok := l.client.(apiclient.LibraryServer)
	if !ok || l.selectedLibraryID == "" {
		return
	}
	libraryID, err := uuid.Parse(l.selectedLibraryID)
	if err != nil {
		return
	}
	uiutil.Request(l.tasks, func(ctx context.Context) ([]apiv0.LibraryReleaseResponse, error) {
		return api.ListLibraryReleases(ctx, libraryID)
	}, func(items []apiv0.LibraryReleaseResponse) { l.releases = items }, nil)
}

func (l *Library) publishRelease(ctx app.Context) {
	api, ok := l.client.(apiclient.LibraryServer)
	if !ok || l.selectedLibraryID == "" {
		return
	}
	libraryID, err := uuid.Parse(l.selectedLibraryID)
	if err != nil {
		return
	}
	uiutil.Request(l.tasks, func(ctx context.Context) (apiv0.CreateLibraryReleaseResponse, error) {
		return api.PublishLibrary(ctx, libraryID)
	}, func(apiv0.CreateLibraryReleaseResponse) { l.loadReleases(l.compoCtx) }, nil)
}

func (l *Library) createWidgetType(ctx context.Context, input apiv0.CreateWidgetTypeRequest) (apiv0.CreateWidgetTypeResponse, error) {
	if api, ok := l.client.(apiclient.LibraryServer); ok && l.selectedLibraryID != "" {
		id, err := uuid.Parse(l.selectedLibraryID)
		if err != nil {
			return apiv0.CreateWidgetTypeResponse{}, err
		}
		return api.CreateLibraryWidgetType(ctx, id, input)
	}
	return l.client.CreateWidgetType(ctx, input)
}
func (l *Library) updateWidgetType(ctx context.Context, id uuid.UUID, input apiv0.UpdateWidgetTypeRequest) (apiv0.UpdateWidgetTypeResponse, error) {
	if api, ok := l.client.(apiclient.LibraryServer); ok && l.selectedLibraryID != "" {
		libraryID, err := uuid.Parse(l.selectedLibraryID)
		if err != nil {
			return apiv0.UpdateWidgetTypeResponse{}, err
		}
		return api.UpdateLibraryWidgetType(ctx, libraryID, id, input)
	}
	return l.client.UpdateWidgetType(ctx, id, input)
}
func (l *Library) deleteWidgetType(ctx context.Context, id uuid.UUID) (apiv0.WidgetTypeResponse, error) {
	if api, ok := l.client.(apiclient.LibraryServer); ok && l.selectedLibraryID != "" {
		libraryID, err := uuid.Parse(l.selectedLibraryID)
		if err != nil {
			return apiv0.WidgetTypeResponse{}, err
		}
		return api.DeleteLibraryWidgetType(ctx, libraryID, id)
	}
	return l.client.DeleteWidgetType(ctx, id)
}

// ── Data loading ──────────────────────────────────────────────────────────────

// loadList (re)loads the widget type list; concurrent requests are coalesced.
func (l *Library) loadList(ctx app.Context) {
	l.loading = true
	uiutil.Load(l.tasks, &l.reloader, uiutil.LoadRequest[[]apiv0.WidgetTypeResponse]{
		Call: func(ctx context.Context) ([]apiv0.WidgetTypeResponse, error) {
			if api, ok := l.client.(apiclient.LibraryServer); ok && l.selectedLibraryID != "" {
				libraryID, err := uuid.Parse(l.selectedLibraryID)
				if err != nil {
					return nil, err
				}
				return api.ListLibraryWidgetTypes(ctx, libraryID)
			}
			return l.client.ListWidgetTypes(ctx)
		}, Reload: func() { l.loadList(l.compoCtx) },
		Finished: func() { l.loading = false },
		Apply:    func(result []apiv0.WidgetTypeResponse) { l.setWidgetTypes(l.compoCtx, result) },
	})
}

// setWidgetTypes replaces the list and brings the editor of the selected type
// up to date: a type seen for the first time fills the editor, a known one is
// merged field by field so unsaved edits survive changes made elsewhere, and a
// deleted one drops the selection.
func (l *Library) setWidgetTypes(ctx app.Context, items []widgetTypeItem) {
	old, known := l.findItem(l.selectedID)
	l.widgetTypes = items
	if l.selectedID == "" {
		return
	}
	current, exists := l.findItem(l.selectedID)
	switch {
	case !exists:
		l.clearSelection()
		l.tasks.Store("library:selectedID", "")
	case !known:
		l.selectItem(l.selectedID)
	default:
		l.mergeSelected(old, current)
	}
}

// clearSelection deselects the widget type and empties its editor.
func (l *Library) clearSelection() {
	l.selectedID = ""
	l.editedHTML = ""
	l.editedScript = ""
	l.editedInputValues = make(map[string]string)
	l.editedInputPorts = nil
}

func (l *Library) findItem(id string) (widgetTypeItem, bool) {
	for _, it := range l.widgetTypes {
		if it.ID.String() == id {
			return it, true
		}
	}
	return widgetTypeItem{}, false
}

// mergeSelected updates each editor field that still holds the old server value.
// Preview input values and the add-port form are editor-only state and kept.
func (l *Library) mergeSelected(old, current widgetTypeItem) {
	l.editedName = uiutil.MergeField(l.editedName, old.Name, current.Name)
	l.editedHTML = uiutil.MergeField(l.editedHTML, old.HtmlTemplate, current.HtmlTemplate)
	l.editedScript = uiutil.MergeField(l.editedScript, old.Script, current.Script)
	l.editedScriptLang = uiutil.MergeField(l.editedScriptLang, old.ScriptLanguage, current.ScriptLanguage)
	if slices.Equal(l.editedInputPorts, old.InputPorts) {
		l.editedInputPorts = slices.Clone(current.InputPorts)
	}
}

// ── Selection helpers ─────────────────────────────────────────────────────────

func (l *Library) selectItem(id string) {
	for _, it := range l.widgetTypes {
		if it.ID.String() == id {
			l.selectedID = id
			l.editedName = it.Name
			l.editedHTML = it.HtmlTemplate
			l.editedScript = it.Script
			l.editedScriptLang = it.ScriptLanguage
			l.editedInputValues = make(map[string]string)
			ports := make([]uidto.InputPortDTO, len(it.InputPorts))
			copy(ports, it.InputPorts)
			l.editedInputPorts = ports
			l.newPortName = ""
			l.newPortDesc = ""
			l.newPortType = ""
			return
		}
	}
}

func (l *Library) startEditing(id, currentName string) {
	l.editingID = id
	l.editingName = currentName
}

// ── Render ────────────────────────────────────────────────────────────────────

func (l *Library) Render() app.UI {
	return app.Div().
		Style("display", "flex").
		Style("flex-direction", "row").
		Style("flex", "1").
		Style("min-height", "0").
		Style("gap", "8px").
		Body(
			l.renderListColumn(),
			l.renderEditorColumn("HTML Template", "html-template", l.editedHTML, func(ctx app.Context, e app.Event) {
				l.editedHTML = ctx.JSSrc().Get("value").String()
			}, true),
			l.renderEditorColumn("Script", "script-editor", l.editedScript, func(ctx app.Context, e app.Event) {
				l.editedScript = ctx.JSSrc().Get("value").String()
			}, true),
			l.renderInputPortsColumn(),
			l.renderInputDataColumn(),
			l.renderPreviewColumn(),
		)
}
