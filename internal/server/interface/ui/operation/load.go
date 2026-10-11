package operation

import (
	"context"
	"errors"
	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/apiclient"
	"github.com/kipitix/growscada/internal/server/interface/ui/livescene"
	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

func (o *Operation) loadWidgetTypes(ctx app.Context) {
	uiutil.Load(o.tasks, &o.widgetTypesReloader, uiutil.LoadRequest[[]apiv0.WidgetTypeResponse]{
		Call: o.client.ListWidgetTypes, Reload: func() { o.loadWidgetTypes(o.compoCtx) },
		Apply: func(items []apiv0.WidgetTypeResponse) {
			byID := make(map[string]livescene.WidgetType, len(items))
			for _, wt := range items {
				byID[wt.ID.String()] = wt
			}
			o.widgetTypes = byID
		},
	})
}
func (o *Operation) loadScenes(ctx app.Context) {
	uiutil.Load(o.tasks, &o.scenesReloader, uiutil.LoadRequest[[]apiv0.SceneResponse]{
		Call: o.client.ListScenes, Reload: func() { o.loadScenes(o.compoCtx) },
		Apply: func(items []apiv0.SceneResponse) {
			o.scenes = items
			o.selectScene(o.compoCtx, selectScene(items, o.selectedSceneID))
		},
	})
}
func (o *Operation) loadScene(ctx app.Context, sceneID string) {
	if o.sceneReloaders == nil {
		o.sceneReloaders = make(map[string]*uiutil.Reloader)
	}
	r := o.sceneReloaders[sceneID]
	if r == nil {
		r = &uiutil.Reloader{}
		o.sceneReloaders[sceneID] = r
	}
	uiutil.Load(o.tasks, r, uiutil.LoadRequest[*apiv0.SceneResponse]{
		Call: func(ctx context.Context) (*apiv0.SceneResponse, error) {
			id, err := uuid.Parse(sceneID)
			if err != nil {
				return nil, err
			}
			sc, err := o.client.GetScene(ctx, id)
			if errors.Is(err, apiclient.ErrNotFound) {
				return nil, nil
			}
			return &sc, err
		},
		Reload: func() { o.loadScene(o.compoCtx, sceneID) },
		Idle:   func() { delete(o.sceneReloaders, sceneID) },
		Apply: func(result *apiv0.SceneResponse) {
			if result == nil {
				return
			}
			for i := range o.scenes {
				if o.scenes[i].ID == result.ID && result.Version >= o.scenes[i].Version {
					o.scenes[i] = *result
					break
				}
			}
		},
	})
}
func (o *Operation) loadWidgets(ctx app.Context) {
	if o.selectedSceneID == "" {
		o.widgets = nil
		o.widgetsSceneID = ""
		return
	}
	sceneID := o.selectedSceneID
	uiutil.Load(o.tasks, &o.widgetsReloader, uiutil.LoadRequest[[]apiv0.WidgetResponse]{
		Call: func(ctx context.Context) ([]apiv0.WidgetResponse, error) {
			id, err := uuid.Parse(sceneID)
			if err != nil {
				return nil, err
			}
			return o.client.ListWidgets(ctx, id)
		},
		Reload: func() { o.loadWidgets(o.compoCtx) }, Accept: func() bool { return o.selectedSceneID == sceneID },
		Apply: func(items []apiv0.WidgetResponse) { o.widgets = items; o.widgetsSceneID = sceneID },
	})
}
func (o *Operation) loadTags(ctx app.Context) {
	uiutil.Load(o.tasks, &o.tagsReloader, uiutil.LoadRequest[[]apiv0.TagResponse]{
		Call: o.client.ListTags, Reload: func() { o.loadTags(o.compoCtx) },
		Apply: func(items []apiv0.TagResponse) { o.tags = mergeTags(o.tags, items) },
	})
}
