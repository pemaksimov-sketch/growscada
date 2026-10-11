package project

import (
	"context"
	"strconv"

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
	api, ok := p.client.(apiclient.LibraryServer)
	if !ok {
		return
	}
	libraryID, err := uuid.Parse(p.selectedLibraryID)
	if err != nil {
		return
	}
	releaseID, err := uuid.Parse(p.selectedReleaseID)
	if err != nil {
		return
	}
	uiutil.Request(p.tasks, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, api.PinProjectLibraryRelease(ctx, p.projectID, apiv0.PinLibraryReleaseRequest{LibraryID: libraryID, ReleaseID: releaseID})
	}, func(struct{}) { p.loadProjectLibraries(p.compoCtx) }, nil)
}

func (p *Project) unpinLibrary(ctx app.Context, libraryID uuid.UUID) {
	api, ok := p.client.(apiclient.LibraryServer)
	if !ok {
		return
	}
	uiutil.Request(p.tasks, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, api.UnpinProjectLibrary(ctx, p.projectID, libraryID)
	}, func(struct{}) { p.loadProjectLibraries(p.compoCtx) }, nil)
}
