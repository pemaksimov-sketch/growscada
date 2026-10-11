package project

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/interface/ui/toast"
	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
	"strconv"
)

func (p *Project) createScene(ctx app.Context) {
	uiutil.Request(p.tasks, func(ctx context.Context) (apiv0.CreateSceneResponse, error) {
		return p.client.CreateScene(ctx, apiv0.CreateSceneRequest{Name: "New Scene", Width: 1920, Height: 1080})
	}, func(result apiv0.CreateSceneResponse) {
		p.selectedSceneID = result.ID.String()
		p.widgets = nil
		p.tasks.Store("project:sceneID", p.selectedSceneID)
		p.clearWidgetSelection(p.compoCtx)
		p.loadScenes(p.compoCtx)
		p.loadWidgets(p.compoCtx)
	}, nil)
}
func (p *Project) confirmDeleteScene(ctx app.Context, sceneID, sceneName string) {
	confirm := func(count int) {
		message := fmt.Sprintf("Delete scene %q?", sceneName)
		if count > 0 {
			message = fmt.Sprintf("Scene %q contains %d widget(s). Delete the scene and all of its widgets?", sceneName, count)
		}
		if app.Window().Call("confirm", message).Bool() {
			p.deleteScene(p.compoCtx, sceneID)
		}
	}
	tasks := p.tasks
	tasks.Report = nil // Counting is advisory: still ask if it fails.
	uiutil.Request(tasks, func(ctx context.Context) ([]apiv0.WidgetResponse, error) {
		id, err := uuid.Parse(sceneID)
		if err != nil {
			return nil, err
		}
		return p.client.ListWidgets(ctx, id)
	}, func(items []apiv0.WidgetResponse) { confirm(len(items)) }, func() { confirm(-1) })
}
func (p *Project) deleteScene(ctx app.Context, sceneID string) {
	uiutil.Request(p.tasks, func(ctx context.Context) (apiv0.SceneResponse, error) {
		id, err := uuid.Parse(sceneID)
		if err != nil {
			return apiv0.SceneResponse{}, err
		}
		return p.client.DeleteScene(ctx, id)
	}, func(apiv0.SceneResponse) { p.loadScenes(p.compoCtx) }, nil)
}
func (p *Project) syncSceneEditingFields() {
	for _, sc := range p.scenes {
		if sc.ID.String() == p.selectedSceneID {
			p.editingScenePropsName = sc.Name
			p.editingScenePropsWidth = strconv.Itoa(sc.Width)
			p.editingScenePropsHeight = strconv.Itoa(sc.Height)
			p.editingScenePropsBG = sc.BackgroundHTML
			return
		}
	}
	p.editingScenePropsName = ""
	p.editingScenePropsWidth = ""
	p.editingScenePropsHeight = ""
	p.editingScenePropsBG = ""
}
func (p *Project) updateScene(sc sceneItem) {
	sceneRequest(p, sc.ID.String(), func(ctx context.Context, version int) (apiv0.UpdateSceneResponse, error) {
		return p.client.UpdateScene(ctx, sc.ID, apiv0.UpdateSceneRequest{Name: sc.Name, Width: sc.Width, Height: sc.Height, BackgroundHTML: sc.BackgroundHTML, Version: version})
	}, func(result apiv0.UpdateSceneResponse) int { return result.Version }, nil)
}
func (p *Project) saveSceneProperties(ctx app.Context) {
	if p.selectedSceneID == "" {
		return
	}
	var nf numberFields
	w := nf.int("width", p.editingScenePropsWidth)
	h := nf.int("height", p.editingScenePropsHeight)
	if nf.err != nil {
		p.compoCtx.NewActionWithValue(toast.ActionAdd, toast.ClientError(nf.err))
		p.syncSceneEditingFields()
		return
	}
	sc, found := p.findScene(p.selectedSceneID)
	if !found {
		return
	}
	if sc.Name == p.editingScenePropsName && sc.Width == w && sc.Height == h && sc.BackgroundHTML == p.editingScenePropsBG {
		return
	}
	sc.Name = p.editingScenePropsName
	sc.Width = w
	sc.Height = h
	sc.BackgroundHTML = p.editingScenePropsBG
	for i := range p.scenes {
		if p.scenes[i].ID == sc.ID {
			p.scenes[i] = sc
			break
		}
	}
	// A rejected optimistic save restores server fields; later local edits survive.
	p.syncSceneEditingFields()
	p.updateScene(sc)
}
func (p *Project) startEditingScene(id, name string) {
	p.editingSceneID = id
	p.editingSceneName = name
}
func (p *Project) commitSceneEdit(ctx app.Context) {
	if p.editingSceneID == "" {
		return
	}
	id, name := p.editingSceneID, p.editingSceneName
	p.editingSceneID = ""
	p.editingSceneName = ""
	sc, found := p.findScene(id)
	if !found {
		return
	}
	if sc.Name == name {
		return
	}
	sc.Name = name
	for i := range p.scenes {
		if p.scenes[i].ID == sc.ID {
			p.scenes[i] = sc
			break
		}
	}
	if p.selectedSceneID == id {
		p.editingScenePropsName = name
	}
	p.updateScene(sc)
}
