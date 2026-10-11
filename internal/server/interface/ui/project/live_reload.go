package project

import (
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/interface/ui/eventlog"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

func (p *Project) handleServerEvents(ctx app.Context) {
	ctx.Handle(eventlog.ActionServerEvent, func(ctx app.Context, a app.Action) {
		ev, ok := a.Value.(eventlog.ServerEvent)
		if !ok {
			return
		}
		if ev.Type == apiv0.EventTagUpdated {
			if tag, valid := decodeTagUpdate(ev.Tag); valid {
				if updated, applied := applyTagUpdate(p.tags, tag); applied {
					p.tags = updated
					return
				}
			}
		}
		if ev.Type == eventlog.EventResync {
			for i := range p.tags {
				p.tags[i].Version = 0
				p.tags[i].Sequence = 0
			}
		}
		targets := eventlog.ReloadTargetsForScene(ev, p.selectedSceneID)
		if targets.SceneID != "" {
			p.loadScene(ctx, targets.SceneID)
		}
		if targets.WidgetTypes {
			p.loadWidgetTypes(ctx)
		}
		if targets.Scenes {
			p.loadScenes(ctx)
		}
		if targets.Widgets {
			p.loadWidgets(ctx)
		}
		if targets.Tags {
			p.loadTags(ctx)
		}
	})
}
