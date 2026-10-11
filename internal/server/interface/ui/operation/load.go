package operation

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"

	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/interface/ui/livescene"
	"github.com/kipitix/growscada/internal/server/interface/ui/toast"
	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
)

// Each server-backed list is (re)loaded through a uiutil.Reloader, on mount,
// on server events and after a reconnection.

func (o *Operation) loadWidgetTypes(ctx app.Context) {
	uiutil.LoadJSON(o.compoCtx, &o.widgetTypesReloader, uiutil.JSONLoad[getWidgetTypesResponse]{
		URL: o.apiServerURL + apiv0.PathPrefix + "/widget-types", Reload: o.loadWidgetTypes,
		Apply: func(ctx app.Context, result getWidgetTypesResponse) {
			byID := make(map[string]livescene.WidgetType, len(result.WidgetTypes))
			for _, wt := range result.WidgetTypes {
				byID[wt.ID] = wt
			}
			o.widgetTypes = byID
		},
	})
}

func (o *Operation) loadScenes(ctx app.Context) {
	uiutil.LoadJSON(o.compoCtx, &o.scenesReloader, uiutil.JSONLoad[getScenesResponse]{
		URL: o.apiServerURL + apiv0.PathPrefix + "/scenes", Reload: o.loadScenes,
		Apply: func(ctx app.Context, result getScenesResponse) {
			o.scenes = result.Scenes
			o.selectScene(ctx, selectScene(o.scenes, o.selectedSceneID))
		},
	})
}

func (o *Operation) loadScene(ctx app.Context, sceneID string) {
	ctx = o.compoCtx
	if o.sceneReloaders == nil {
		o.sceneReloaders = make(map[string]*uiutil.Reloader)
	}
	r := o.sceneReloaders[sceneID]
	if r == nil {
		r = &uiutil.Reloader{}
		o.sceneReloaders[sceneID] = r
	}
	if !r.Start() {
		return
	}
	ctx.Async(func() {
		result, problem := uiutil.FetchJSON[livescene.Scene](o.apiServerURL + apiv0.PathPrefix + "/scenes/" + sceneID)
		ctx.Dispatch(func(ctx app.Context) {
			if problem != nil {
				if problem.Status != 404 {
					ctx.NewActionWithValue(toast.ActionAdd, *problem)
				}
			} else {
				for i := range o.scenes {
					if o.scenes[i].ID == sceneID {
						o.scenes[i] = result
						break
					}
				}
			}
			if r.Done() {
				o.loadScene(ctx, sceneID)
			} else {
				delete(o.sceneReloaders, sceneID)
			}
		})
	})
}

// loadWidgets (re)loads the widgets of the selected Scene.
func (o *Operation) loadWidgets(ctx app.Context) {
	if o.selectedSceneID == "" {
		o.widgets = nil
		o.widgetsSceneID = ""
		return
	}
	sceneID := o.selectedSceneID
	uiutil.LoadJSON(o.compoCtx, &o.widgetsReloader, uiutil.JSONLoad[getWidgetsResponse]{
		URL: o.apiServerURL + apiv0.PathPrefix + "/scenes/" + sceneID + "/widgets", Reload: o.loadWidgets,
		Accept: func() bool { return o.selectedSceneID == sceneID },
		Apply: func(ctx app.Context, result getWidgetsResponse) {
			o.widgets = result.Widgets
			o.widgetsSceneID = sceneID
		},
	})
}

func (o *Operation) loadTags(ctx app.Context) {
	uiutil.LoadJSON(o.compoCtx, &o.tagsReloader, uiutil.JSONLoad[getTagsResponse]{
		URL: o.apiServerURL + apiv0.PathPrefix + "/tags", Reload: o.loadTags,
		Apply: func(ctx app.Context, result getTagsResponse) { o.tags = mergeTags(o.tags, result.Tags) },
	})
}
