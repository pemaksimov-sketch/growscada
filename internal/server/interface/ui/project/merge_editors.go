package project

import (
	"fmt"
	"strconv"

	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// setScenes replaces the scene list, keeps a valid selection (falling back to
// the first scene) and merges the scene properties editor.
func (p *Project) setScenes(ctx app.Context, scenes []sceneItem) {
	old, known := p.findScene(p.selectedSceneID)
	prevSceneID := p.selectedSceneID

	// A version never goes back: a response that raced with our own mutation
	// must not undo the version bump that mutation already recorded.
	for i := range scenes {
		if prev, ok := p.findScene(scenes[i].ID.String()); ok && prev.Version > scenes[i].Version {
			scenes[i].Version = prev.Version
		}
	}
	p.scenes = scenes

	if _, exists := p.findScene(p.selectedSceneID); p.selectedSceneID != "" && !exists {
		p.selectedSceneID = ""
		p.clearWidgetSelection(ctx)
	}
	if p.selectedSceneID == "" && len(scenes) > 0 {
		p.selectedSceneID = scenes[0].ID.String()
	}

	if current, ok := p.findScene(p.selectedSceneID); ok && known && p.selectedSceneID == prevSceneID {
		p.mergeSceneEditingFields(old, current)
	} else {
		p.syncSceneEditingFields()
	}

	if p.selectedSceneID != prevSceneID {
		p.tasks.Store("project:sceneID", p.selectedSceneID)
		if p.selectedSceneID != "" {
			p.loadWidgets(ctx)
		}
	}
}

func (p *Project) findScene(id string) (sceneItem, bool) {
	for _, sc := range p.scenes {
		if sc.ID.String() == id {
			return sc, true
		}
	}
	return sceneItem{}, false
}

// mergeSceneEditingFields updates each scene property field that still holds
// the old server value.
func (p *Project) mergeSceneEditingFields(old, current sceneItem) {
	p.editingScenePropsName = uiutil.MergeField(p.editingScenePropsName, old.Name, current.Name)
	p.editingScenePropsWidth = uiutil.MergeField(p.editingScenePropsWidth, strconv.Itoa(old.Width), strconv.Itoa(current.Width))
	p.editingScenePropsHeight = uiutil.MergeField(p.editingScenePropsHeight, strconv.Itoa(old.Height), strconv.Itoa(current.Height))
	p.editingScenePropsBG = uiutil.MergeField(p.editingScenePropsBG, old.BackgroundHTML, current.BackgroundHTML)
}

// setWidgets replaces the selected scene's widgets and merges the widget
// properties editor.
func (p *Project) setWidgets(ctx app.Context, sceneID string, widgets []widgetItem) {
	old, known := p.findWidget(p.selectedWidgetID)
	p.widgets = widgets

	// Widgets carry their Scene's version; adopt it if it is newer.
	if len(widgets) > 0 {
		if sc, ok := p.findScene(sceneID); ok && widgets[0].SceneVersion > sc.Version {
			p.applyNewSceneVersion(sceneID, widgets[0].SceneVersion)
		}
	}

	if p.selectedWidgetID == "" {
		return
	}
	current, exists := p.findWidget(p.selectedWidgetID)
	switch {
	case !exists:
		p.clearWidgetSelection(ctx)
	case !known:
		p.syncEditingFields(current)
	default:
		p.mergeWidgetEditingFields(old, current)
	}
}

func (p *Project) findWidget(id string) (widgetItem, bool) {
	for _, w := range p.widgets {
		if w.ID.String() == id {
			return w, true
		}
	}
	return widgetItem{}, false
}

// widgetEditFields is the widget properties editor's text, formatted as the
// editor shows it.
type widgetEditFields struct {
	name, posX, posY, posZ, width, height, originX, originY, rotation string
}

func editFieldsOf(w widgetItem) widgetEditFields {
	return widgetEditFields{
		name:     w.Name,
		posX:     fmt.Sprintf("%.1f", w.Position.X),
		posY:     fmt.Sprintf("%.1f", w.Position.Y),
		posZ:     strconv.Itoa(w.Position.Z),
		width:    strconv.Itoa(w.Size.Width),
		height:   strconv.Itoa(w.Size.Height),
		originX:  fmt.Sprintf("%.3f", w.Origin.X),
		originY:  fmt.Sprintf("%.3f", w.Origin.Y),
		rotation: fmt.Sprintf("%.1f", w.Rotation.Degrees),
	}
}

func (p *Project) editFields() widgetEditFields {
	return widgetEditFields{
		name:     p.editingWidgetName,
		posX:     p.editingPosX,
		posY:     p.editingPosY,
		posZ:     p.editingPosZ,
		width:    p.editingWidth,
		height:   p.editingHeight,
		originX:  p.editingOriginX,
		originY:  p.editingOriginY,
		rotation: p.editingRotation,
	}
}

func (p *Project) setEditFields(f widgetEditFields) {
	p.editingWidgetName = f.name
	p.editingPosX = f.posX
	p.editingPosY = f.posY
	p.editingPosZ = f.posZ
	p.editingWidth = f.width
	p.editingHeight = f.height
	p.editingOriginX = f.originX
	p.editingOriginY = f.originY
	p.editingRotation = f.rotation
}

func (p *Project) syncEditingFields(w widgetItem) {
	p.setEditFields(editFieldsOf(w))
}

// mergeWidgetEditingFields updates each widget property field that still holds
// the old server value.
func (p *Project) mergeWidgetEditingFields(old, current widgetItem) {
	p.setEditFields(mergeEditFields(p.editFields(), editFieldsOf(old), editFieldsOf(current)))
}

func mergeEditFields(local, old, current widgetEditFields) widgetEditFields {
	m := uiutil.MergeField[string]
	return widgetEditFields{
		name:     m(local.name, old.name, current.name),
		posX:     m(local.posX, old.posX, current.posX),
		posY:     m(local.posY, old.posY, current.posY),
		posZ:     m(local.posZ, old.posZ, current.posZ),
		width:    m(local.width, old.width, current.width),
		height:   m(local.height, old.height, current.height),
		originX:  m(local.originX, old.originX, current.originX),
		originY:  m(local.originY, old.originY, current.originY),
		rotation: m(local.rotation, old.rotation, current.rotation),
	}
}

// setTags replaces the tag list and drops the selection if its tag is gone.
func (p *Project) setTags(ctx app.Context, tags []tagItem) {
	p.tags = tags
	if p.selectedTagID == "" {
		return
	}
	for _, t := range tags {
		if t.ID.String() == p.selectedTagID {
			return
		}
	}
	p.selectedTagID = ""
	p.tasks.Store("project:tagID", "")
}
