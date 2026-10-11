package project

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/apiclient"
	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

func (p *Project) loadProjectLibraries(ctx app.Context) {
	api, ok := p.client.(apiclient.LibraryServer)
	if !ok {
		return
	}
	uiutil.Request(p.tasks, func(ctx context.Context) ([]apiv0.LibraryResponse, error) { return api.ListLibraries(ctx) }, func(items []apiv0.LibraryResponse) {
		p.libraries = items
		if p.selectedLibraryID == "" && len(items) > 0 {
			p.selectedLibraryID = items[0].ID.String()
		}
		p.loadProjectLibraryReleases(p.compoCtx)
	}, nil)
	uiutil.Request(p.tasks, func(ctx context.Context) ([]apiv0.ProjectLibraryPinResponse, error) {
		return api.ListProjectLibraryPins(ctx, p.projectID)
	}, func(items []apiv0.ProjectLibraryPinResponse) { p.libraryPins = items }, nil)
}

func (p *Project) loadProjectLibraryReleases(ctx app.Context) {
	api, ok := p.client.(apiclient.LibraryServer)
	if !ok || p.selectedLibraryID == "" {
		return
	}
	libraryID, err := uuid.Parse(p.selectedLibraryID)
	if err != nil {
		return
	}
	uiutil.Request(p.tasks, func(ctx context.Context) ([]apiv0.LibraryReleaseResponse, error) {
		return api.ListLibraryReleases(ctx, libraryID)
	}, func(items []apiv0.LibraryReleaseResponse) {
		p.libraryReleases = items
		if len(items) > 0 {
			p.selectedReleaseID = items[0].ID.String()
		} else {
			p.selectedReleaseID = ""
		}
	}, nil)
}

func (p *Project) renderLibrariesPanel() app.UI {
	options := make([]app.UI, 0, len(p.libraries))
	for _, item := range p.libraries {
		options = append(options, app.Option().Value(item.ID.String()).Selected(item.ID.String() == p.selectedLibraryID).Text(item.Name))
	}
	releases := make([]app.UI, 0, len(p.libraryReleases))
	for _, item := range p.libraryReleases {
		releases = append(releases, app.Option().Value(item.ID.String()).Selected(item.ID.String() == p.selectedReleaseID).Text("Release "+strconv.Itoa(item.Number)))
	}
	pins := make([]app.UI, 0, len(p.libraryPins))
	for _, pin := range p.libraryPins {
		label := "Library " + pin.LibraryID.String()
		for _, lib := range p.libraries {
			if lib.ID == pin.LibraryID {
				label = lib.Name
				break
			}
		}
		libraryID := pin.LibraryID
		pins = append(pins, app.Div().Style("display", "flex").Style("gap", "8px").Style("padding", "8px").Style("border-bottom", "1px solid var(--border)").Body(
			app.Span().Text(label+" · Release "+strconv.Itoa(pin.Release.Number)),
			app.Button().Text("Unpin").OnClick(func(ctx app.Context, e app.Event) { p.unpinLibrary(ctx, libraryID) }),
		))
	}
	return app.Div().Style("padding", "12px").Style("overflow", "auto").Body(
		app.H3().Text("Pinned library releases"),
		app.Div().Style("display", "flex").Style("gap", "8px").Style("align-items", "center").Body(
			app.Select().ID("project-library-picker").Body(options...).OnChange(func(ctx app.Context, e app.Event) {
				p.selectedLibraryID = ctx.JSSrc().Get("value").String()
				p.loadProjectLibraryReleases(ctx)
			}),
			app.Select().ID("project-library-release-picker").Body(releases...).OnChange(func(ctx app.Context, e app.Event) { p.selectedReleaseID = ctx.JSSrc().Get("value").String() }),
			app.Button().Text("Pin release").OnClick(func(ctx app.Context, e app.Event) { p.pinLibraryRelease(ctx) }),
		),
		app.Div().Style("margin-top", "12px").Body(pins...),
	)
}

func (p *Project) pinLibraryRelease(ctx app.Context) {
	libraryID, err := uuid.Parse(p.selectedLibraryID)
	if err != nil {
		return
	}
	releaseID, err := uuid.Parse(p.selectedReleaseID)
	if err != nil {
		return
	}
	p.confirmLibraryPinChange(ctx, libraryID, &releaseID)
}

func (p *Project) unpinLibrary(ctx app.Context, libraryID uuid.UUID) {
	p.confirmLibraryPinChange(ctx, libraryID, nil)
}

type releaseSnapshot struct {
	WidgetTypes []releaseWidgetType `json:"widget_types"`
}

type releaseWidgetType struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	InputPorts []struct {
		Name string `json:"name"`
	} `json:"input_ports"`
}

type widgetImpact struct {
	scene string
	name  string
	issue string
}

type sceneWidgets struct {
	scene apiv0.SceneResponse
	items []apiv0.WidgetResponse
}

// confirmLibraryPinChange previews the actual effect across every scene before
// changing a project's library pins. The draft's Widgets and their bindings
// remain saved; this warning tells the operator which references will no
// longer resolve through the selected pinned releases.
func (p *Project) confirmLibraryPinChange(ctx app.Context, libraryID uuid.UUID, releaseID *uuid.UUID) {
	api, ok := p.client.(apiclient.LibraryServer)
	if !ok {
		return
	}
	type snapshot struct {
		pins    []apiv0.ProjectLibraryPinResponse
		scenes  []apiv0.SceneResponse
		types   []apiv0.WidgetTypeResponse
		widgets []sceneWidgets
		target  apiv0.LibraryReleaseResponse
	}
	uiutil.Request(p.tasks, func(ctx context.Context) (snapshot, error) {
		var result snapshot
		var err error
		if result.pins, err = api.ListProjectLibraryPins(ctx, p.projectID); err != nil {
			return result, err
		}
		if result.scenes, err = p.client.ListScenes(ctx); err != nil {
			return result, err
		}
		if result.types, err = api.ListProjectWidgetTypes(ctx, p.projectID); err != nil {
			return result, err
		}
		for _, scene := range result.scenes {
			items, err := p.client.ListWidgets(ctx, scene.ID)
			if err != nil {
				return result, err
			}
			result.widgets = append(result.widgets, sceneWidgets{scene, items})
		}
		if releaseID != nil {
			items, err := api.ListLibraryReleases(ctx, libraryID)
			if err != nil {
				return result, err
			}
			for _, item := range items {
				if item.ID == *releaseID {
					result.target = item
					break
				}
			}
			if result.target.ID == uuid.Nil {
				return result, apiclient.ErrNotFound
			}
		}
		return result, nil
	}, func(data snapshot) {
		var target *apiv0.LibraryReleaseResponse
		if releaseID != nil {
			target = &data.target
		}
		impacts := analyzeLibraryPinImpact(data.types, data.pins, libraryID, target, data.widgets)
		if len(impacts) > 0 {
			var lines []string
			for _, item := range impacts {
				lines = append(lines, fmt.Sprintf("• %s / %s: %s", item.scene, item.name, item.issue))
			}
			if !app.Window().Call("confirm", "Смена Library Release затронет Widgets:\n"+strings.Join(lines, "\n")+"\n\nПродолжить?").Bool() {
				return
			}
		}
		uiutil.Request(p.tasks, func(ctx context.Context) (struct{}, error) {
			if releaseID != nil {
				return struct{}{}, api.PinProjectLibraryRelease(ctx, p.projectID, apiv0.PinLibraryReleaseRequest{LibraryID: libraryID, ReleaseID: *releaseID})
			}
			return struct{}{}, api.UnpinProjectLibrary(ctx, p.projectID, libraryID)
		}, func(struct{}) { p.loadProjectLibraries(p.compoCtx) }, nil)
	}, nil)
}

func snapshotTypes(content json.RawMessage) []releaseWidgetType {
	var snapshot releaseSnapshot
	_ = json.Unmarshal(content, &snapshot)
	return snapshot.WidgetTypes
}

func analyzeLibraryPinImpact(current []apiv0.WidgetTypeResponse, pins []apiv0.ProjectLibraryPinResponse, libraryID uuid.UUID, target *apiv0.LibraryReleaseResponse, widgets []sceneWidgets) []widgetImpact {
	postTypes := make(map[uuid.UUID]releaseWidgetType, len(current))
	for _, item := range current {
		ports := make([]struct {
			Name string `json:"name"`
		}, len(item.InputPorts))
		for i, port := range item.InputPorts {
			ports[i].Name = port.Name
		}
		postTypes[item.ID] = releaseWidgetType{ID: item.ID, Name: item.Name, InputPorts: ports}
	}
	for _, pin := range pins {
		if pin.LibraryID == libraryID {
			for _, item := range snapshotTypes(pin.Release.Content) {
				delete(postTypes, item.ID)
			}
		}
	}
	if target != nil {
		for _, item := range snapshotTypes(target.Content) {
			postTypes[item.ID] = item
		}
	}
	impacts := make([]widgetImpact, 0)
	for _, group := range widgets {
		for _, widget := range group.items {
			oldType, existed := findWidgetType(current, widget.TypeID)
			newType, remains := postTypes[widget.TypeID]
			if existed && !remains {
				impacts = append(impacts, widgetImpact{group.scene.Name, widget.Name, "WidgetType «" + oldType.Name + "» станет недоступен"})
				continue
			}
			if !remains {
				continue
			}
			ports := make(map[string]bool)
			for _, port := range newType.InputPorts {
				ports[port.Name] = true
			}
			for _, binding := range widget.PortBindings {
				if !ports[binding.PortName] {
					impacts = append(impacts, widgetImpact{group.scene.Name, widget.Name, "потеряет входной порт «" + binding.PortName + "»"})
				}
			}
		}
	}
	return impacts
}

func findWidgetType(items []apiv0.WidgetTypeResponse, id uuid.UUID) (apiv0.WidgetTypeResponse, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return apiv0.WidgetTypeResponse{}, false
}
