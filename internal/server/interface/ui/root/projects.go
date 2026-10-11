package root

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/interface/ui/eventlog"
	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

func (r *Root) loadProjects() {
	uiutil.Request(r.tasks, r.client.ListProjects, func(projects []apiv0.ProjectResponse) {
		initialLoad := r.projectsLoading
		r.projects, r.projectsLoading = projects, false
		for _, p := range projects {
			if p.ID == r.selectedProjectID {
				return
			}
		}
		r.selectedProjectID = uuid.Nil
		if len(projects) > 0 {
			r.selectedProjectID = projects[0].ID
		}
		r.tasks.Store("root:projectID", r.selectedProjectID.String())
		if !initialLoad && r.selectedProjectID != uuid.Nil {
			r.switchProject(r.selectedProjectID)
		}
	}, func() { r.projectsLoading = false })
}

// Reloading gives every mounted mode and the SSE stream a fresh project scope.
// It also keeps async responses from the previous project out of the new one.
func (r *Root) switchProject(projectID uuid.UUID) {
	r.tasks.Store("root:projectID", projectID.String())
	app.Window().Get("location").Call("reload")
}

func (r *Root) renderProjectPicker() app.UI {
	options := make([]app.UI, 0, len(r.projects))
	for _, p := range r.projects {
		options = append(options, app.Option().Value(p.ID.String()).Selected(p.ID == r.selectedProjectID).Text(p.Name))
	}
	return app.Div().Style("display", "flex").Style("gap", "6px").Style("align-items", "center").Body(
		app.Label().For("project-picker").Text("Current project"),
		app.Select().ID("project-picker").Body(options...).OnChange(func(ctx app.Context, e app.Event) {
			projectID, err := uuid.Parse(ctx.JSSrc().Get("value").String())
			if err == nil {
				r.switchProject(projectID)
			}
		}),
		app.Button().Text("New project").OnClick(func(ctx app.Context, e app.Event) {
			name := app.Window().Call("prompt", "Project name")
			if !name.Truthy() {
				return
			}
			uiutil.Request(r.tasks, func(ctx context.Context) (apiv0.ProjectResponse, error) {
				return r.client.CreateProject(ctx, apiv0.CreateProjectRequest{Name: name.String()})
			}, func(p apiv0.ProjectResponse) {
				r.tasks.Store("root:mode", string(ModeProject))
				r.switchProject(p.ID)
			}, nil)
		}),
		app.Button().Text("Rename project").Disabled(r.selectedProjectID == uuid.Nil).OnClick(func(ctx app.Context, e app.Event) {
			for _, p := range r.projects {
				if p.ID != r.selectedProjectID {
					continue
				}
				name := app.Window().Call("prompt", "Project name", p.Name)
				if !name.Truthy() || name.String() == p.Name {
					return
				}
				uiutil.Request(r.tasks, func(ctx context.Context) (apiv0.ProjectResponse, error) {
					return r.client.UpdateProject(ctx, p.ID, apiv0.UpdateProjectRequest{Name: name.String(), Version: p.Version})
				}, func(apiv0.ProjectResponse) { r.loadProjects() }, func() { r.loadProjects() })
				return
			}
		}),
		app.Button().Text("Delete project").Disabled(r.selectedProjectID == uuid.Nil).OnClick(func(ctx app.Context, e app.Event) {
			for _, p := range r.projects {
				if p.ID != r.selectedProjectID {
					continue
				}
				if !app.Window().Call("confirm", fmt.Sprintf("Delete project %q with all of its tags, scenes and widgets?", p.Name)).Bool() {
					return
				}
				uiutil.Request(r.tasks, func(ctx context.Context) (apiv0.ProjectResponse, error) { return r.client.DeleteProject(ctx, p.ID) }, func(apiv0.ProjectResponse) { r.switchProject(uuid.Nil) }, nil)
				return
			}
		}),
	)
}

func (r *Root) observeProjectEvents(ctx app.Context) {
	ctx.Handle(eventlog.ActionServerEvent, func(ctx app.Context, a app.Action) {
		if ev, ok := a.Value.(eventlog.ServerEvent); ok && eventlog.ReloadTargetsFor(ev.Type).Projects {
			r.loadProjects()
		}
	})
}
