package project

import (
	"context"
	"errors"
	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/apiclient"
	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

func (p *Project) loadWidgetTypes(ctx app.Context) {
	uiutil.Load(p.tasks, &p.widgetTypesReloader, uiutil.LoadRequest[[]apiv0.WidgetTypeResponse]{
		Call: func(ctx context.Context) ([]apiv0.WidgetTypeResponse, error) {
			if api, ok := p.client.(apiclient.LibraryServer); ok {
				return api.ListProjectWidgetTypes(ctx, p.projectID)
			}
			return p.client.ListWidgetTypes(ctx)
		}, Reload: func() { p.loadWidgetTypes(p.compoCtx) },
		Apply: func(items []apiv0.WidgetTypeResponse) { p.widgetTypes = items },
	})
}
func (p *Project) loadScenes(ctx app.Context) {
	uiutil.Load(p.tasks, &p.scenesReloader, uiutil.LoadRequest[[]apiv0.SceneResponse]{
		Call: p.client.ListScenes, Reload: func() { p.loadScenes(p.compoCtx) },
		Apply: func(items []apiv0.SceneResponse) { p.setScenes(p.compoCtx, items) },
	})
}

// loadScene refreshes only the owner, including an empty Scene's version.
func (p *Project) loadScene(ctx app.Context, sceneID string) {
	if q := p.writesFor(sceneID); q != nil {
		q.refreshScene = true
		return
	}
	if p.sceneReloaders == nil {
		p.sceneReloaders = make(map[string]*uiutil.Reloader)
	}
	r := p.sceneReloaders[sceneID]
	if r == nil {
		r = &uiutil.Reloader{}
		p.sceneReloaders[sceneID] = r
	}
	uiutil.Load(p.tasks, r, uiutil.LoadRequest[*apiv0.SceneResponse]{
		Call: func(ctx context.Context) (*apiv0.SceneResponse, error) {
			id, err := uuid.Parse(sceneID)
			if err != nil {
				return nil, err
			}
			sc, err := p.client.GetScene(ctx, id)
			if errors.Is(err, apiclient.ErrNotFound) {
				return nil, nil
			}
			return &sc, err
		},
		Reload: func() { p.loadScene(p.compoCtx, sceneID) },
		Idle:   func() { delete(p.sceneReloaders, sceneID) },
		Apply: func(result *apiv0.SceneResponse) {
			if q := p.writesFor(sceneID); q != nil {
				q.refreshScene = true
				return
			}
			if result == nil {
				return
			}
			if old, exists := p.findScene(sceneID); exists && result.Version >= old.Version {
				if sceneID == p.selectedSceneID {
					p.mergeSceneEditingFields(old, *result)
				}
				for i := range p.scenes {
					if p.scenes[i].ID.String() == sceneID {
						p.scenes[i] = *result
						break
					}
				}
			}
		},
	})
}
func (p *Project) isInteracting() bool {
	return p.draggingWidgetID != "" || p.draggingOriginID != "" || p.rotatingWidgetID != "" || p.resizingWidgetID != ""
}
func (p *Project) loadWidgets(ctx app.Context) {
	if p.selectedSceneID == "" {
		p.widgets = nil
		return
	}
	if p.isInteracting() {
		p.widgetsReloadDeferred = true
		return
	}
	if q := p.writesFor(p.selectedSceneID); q != nil {
		q.refreshWidgets = true
		return
	}
	sceneID := p.selectedSceneID
	uiutil.Load(p.tasks, &p.widgetsReloader, uiutil.LoadRequest[[]apiv0.WidgetResponse]{
		Call: func(ctx context.Context) ([]apiv0.WidgetResponse, error) {
			id, err := uuid.Parse(sceneID)
			if err != nil {
				return nil, err
			}
			return p.client.ListWidgets(ctx, id)
		},
		Reload: func() { p.loadWidgets(p.compoCtx) }, Accept: func() bool { return p.selectedSceneID == sceneID },
		Apply: func(items []apiv0.WidgetResponse) {
			if q := p.writesFor(sceneID); q != nil {
				q.refreshWidgets = true
				return
			}
			if p.isInteracting() {
				p.widgetsReloadDeferred = true
				return
			}
			p.setWidgets(p.compoCtx, sceneID, items)
		},
	})
}
func (p *Project) loadTags(ctx app.Context) {
	uiutil.Load(p.tasks, &p.tagsReloader, uiutil.LoadRequest[[]apiv0.TagResponse]{
		Call: p.client.ListTags, Reload: func() { p.loadTags(p.compoCtx) },
		Apply: func(items []apiv0.TagResponse) { p.setTags(p.compoCtx, mergeTagList(p.tags, items)) },
	})
}
