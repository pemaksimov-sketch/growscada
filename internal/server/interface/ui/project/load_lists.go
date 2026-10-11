package project

import (
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/interface/ui/toast"
	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

func (p *Project) loadWidgetTypes(ctx app.Context) {
	uiutil.LoadJSON(p.compoCtx, &p.widgetTypesReloader, uiutil.JSONLoad[getWidgetTypesResponse]{
		URL: p.apiServerURL + apiv0.PathPrefix + "/widget-types", Reload: p.loadWidgetTypes,
		Apply: func(ctx app.Context, result getWidgetTypesResponse) { p.widgetTypes = result.WidgetTypes },
	})
}

func (p *Project) loadScenes(ctx app.Context) {
	uiutil.LoadJSON(p.compoCtx, &p.scenesReloader, uiutil.JSONLoad[getScenesResponse]{
		URL: p.apiServerURL + apiv0.PathPrefix + "/scenes", Reload: p.loadScenes,
		Apply: func(ctx app.Context, result getScenesResponse) { p.setScenes(ctx, result.Scenes) },
	})
}

// loadScene refreshes only the owner of a changed widget, including the
// version when deleting the last widget leaves an empty widget list.
func (p *Project) loadScene(ctx app.Context, sceneID string) {
	ctx = p.compoCtx
	if p.sceneReloaders == nil {
		p.sceneReloaders = make(map[string]*uiutil.Reloader)
	}
	r := p.sceneReloaders[sceneID]
	if r == nil {
		r = &uiutil.Reloader{}
		p.sceneReloaders[sceneID] = r
	}
	if !r.Start() {
		return
	}
	ctx.Async(func() {
		result, problem := uiutil.FetchJSON[sceneItem](p.apiServerURL + apiv0.PathPrefix + "/scenes/" + sceneID)
		ctx.Dispatch(func(ctx app.Context) {
			if problem != nil {
				if problem.Status != 404 {
					ctx.NewActionWithValue(toast.ActionAdd, *problem)
				}
			} else if old, exists := p.findScene(sceneID); exists && result.Version >= old.Version {
				if sceneID == p.selectedSceneID {
					p.mergeSceneEditingFields(old, result)
				}
				for i := range p.scenes {
					if p.scenes[i].ID == sceneID {
						p.scenes[i] = result
						break
					}
				}
			}
			if r.Done() {
				p.loadScene(ctx, sceneID)
			} else {
				delete(p.sceneReloaders, sceneID)
			}
		})
	})
}

// isInteracting reports whether a widget is being dragged, rotated or resized
// on the canvas; replacing p.widgets then would make it jump under the cursor.
func (p *Project) isInteracting() bool {
	return p.draggingWidgetID != "" || p.draggingOriginID != "" ||
		p.rotatingWidgetID != "" || p.resizingWidgetID != ""
}

// loadWidgets (re)loads the widgets of the selected scene. During a canvas
// interaction the reload is deferred until it ends (finalizeAllDrags).
func (p *Project) loadWidgets(ctx app.Context) {
	if p.selectedSceneID == "" {
		p.widgets = nil
		return
	}
	if p.isInteracting() {
		p.widgetsReloadDeferred = true
		return
	}
	sceneID := p.selectedSceneID
	uiutil.LoadJSON(p.compoCtx, &p.widgetsReloader, uiutil.JSONLoad[getWidgetsResponse]{
		URL: p.apiServerURL + apiv0.PathPrefix + "/scenes/" + sceneID + "/widgets", Reload: p.loadWidgets,
		Accept: func() bool { return p.selectedSceneID == sceneID },
		Apply: func(ctx app.Context, result getWidgetsResponse) {
			if p.isInteracting() {
				p.widgetsReloadDeferred = true
				return
			}
			p.setWidgets(ctx, sceneID, result.Widgets)
		},
	})
}

func (p *Project) loadTags(ctx app.Context) {
	uiutil.LoadJSON(p.compoCtx, &p.tagsReloader, uiutil.JSONLoad[getTagsResponse]{
		URL: p.apiServerURL + apiv0.PathPrefix + "/tags", Reload: p.loadTags,
		Apply: func(ctx app.Context, result getTagsResponse) { p.setTags(ctx, mergeTagList(p.tags, result.Tags)) },
	})
}
