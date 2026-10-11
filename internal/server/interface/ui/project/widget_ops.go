package project

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/maxence-charriere/go-app/v10/pkg/app"

	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/interface/ui/toast"
	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
)

// ── Widget data loading ───────────────────────────────────────────────────────

// currentSceneVersion returns the last-known version of the selected scene,
// the sole optimistic-lock boundary shared by the scene and all its widgets.
func (p *Project) currentSceneVersion() int {
	for _, sc := range p.scenes {
		if sc.ID.String() == p.selectedSceneID {
			return sc.Version
		}
	}
	return 0
}

// applyNewSceneVersion records a scene version bump (from a widget mutation)
// in both the scene list and every currently loaded widget, so the next
// mutation is checked against the up-to-date version.
func (p *Project) applyNewSceneVersion(sceneID string, newVersion int) {
	for i := range p.scenes {
		if p.scenes[i].ID.String() == sceneID {
			if newVersion > p.scenes[i].Version {
				p.scenes[i].Version = newVersion
			}
			newVersion = p.scenes[i].Version
			break
		}
	}
	if p.selectedSceneID == sceneID {
		for i := range p.widgets {
			if newVersion > p.widgets[i].SceneVersion {
				p.widgets[i].SceneVersion = newVersion
			}
		}
	}
}

// ── Widget CRUD ───────────────────────────────────────────────────────────────

func (p *Project) createWidget(ctx app.Context, typeID string, x, y float64) {
	if p.selectedSceneID == "" {
		return
	}
	name, width, height := "Widget", 120, 60
	for _, wt := range p.widgetTypes {
		if wt.ID.String() == typeID {
			name = wt.Name
			if wt.DefaultWidth > 0 {
				width = wt.DefaultWidth
			}
			if wt.DefaultHeight > 0 {
				height = wt.DefaultHeight
			}
			break
		}
	}
	sceneID := p.selectedSceneID
	version := p.currentSceneVersion()
	uiutil.Request(p.tasks, func(ctx context.Context) (apiv0.CreateWidgetResponse, error) {
		sceneUUID, err := uuid.Parse(sceneID)
		if err != nil {
			return apiv0.CreateWidgetResponse{}, err
		}
		typeUUID, err := uuid.Parse(typeID)
		if err != nil {
			return apiv0.CreateWidgetResponse{}, err
		}
		return p.client.CreateWidget(ctx, sceneUUID, apiv0.CreateWidgetRequest{Name: name, Position: apiv0.PositionRequest{X: x, Y: y}, Size: apiv0.SizeRequest{Width: width, Height: height}, Origin: apiv0.OriginRequest{X: 0.5, Y: 0.5}, TypeID: typeUUID, SceneVersion: version, Labels: []string{}, PortBindings: []apiv0.PortBinding{}})
	}, func(result apiv0.CreateWidgetResponse) {
		p.applyNewSceneVersion(sceneID, result.SceneVersion)
		if p.selectedSceneID == sceneID {
			p.selectedWidgetID = result.ID.String()
			p.tasks.Store("project:widgetID", p.selectedWidgetID)
			p.syncEditingFields(widgetItem{Name: name, Position: positionDTO{X: x, Y: y}, Size: sizeDTO{Width: width, Height: height}, Origin: originDTO{X: 0.5, Y: 0.5}})
		}
		p.loadWidgets(p.compoCtx)
	}, func() { p.loadScene(p.compoCtx, sceneID); p.loadWidgets(p.compoCtx) })
}
func (p *Project) deleteWidget(ctx app.Context, widgetID string) {
	sceneID := p.selectedSceneID
	uiutil.Request(p.tasks, func(ctx context.Context) (apiv0.WidgetResponse, error) {
		sc, err := uuid.Parse(sceneID)
		if err != nil {
			return apiv0.WidgetResponse{}, err
		}
		id, err := uuid.Parse(widgetID)
		if err != nil {
			return apiv0.WidgetResponse{}, err
		}
		return p.client.DeleteWidget(ctx, sc, id)
	}, func(result apiv0.WidgetResponse) {
		p.applyNewSceneVersion(sceneID, result.SceneVersion)
		if p.selectedWidgetID == widgetID {
			p.clearWidgetSelection(p.compoCtx)
		}
		filtered := make([]widgetItem, 0, len(p.widgets))
		for _, w := range p.widgets {
			if w.ID.String() != widgetID {
				filtered = append(filtered, w)
			}
		}
		p.widgets = filtered
	}, nil)
}
func (p *Project) saveWidgetName(ctx app.Context) {
	if strings.TrimSpace(p.editingWidgetName) == "" {
		return
	}
	idx := p.selectedWidgetIdx()
	if idx < 0 {
		return
	}
	p.widgets[idx].Name = p.editingWidgetName
	p.putWidget(ctx, p.widgets[idx])
}

func (p *Project) saveWidgetGeometry(ctx app.Context) {
	idx := p.selectedWidgetIdx()
	if idx < 0 {
		return
	}
	var nf numberFields
	posX := nf.float("position X", p.editingPosX)
	posY := nf.float("position Y", p.editingPosY)
	posZ := nf.int("position Z", p.editingPosZ)
	width := nf.int("width", p.editingWidth)
	height := nf.int("height", p.editingHeight)
	originX := nf.float("origin X", p.editingOriginX)
	originY := nf.float("origin Y", p.editingOriginY)
	rotDeg := nf.float("rotation", p.editingRotation)
	if nf.err != nil {
		p.compoCtx.NewActionWithValue(toast.ActionAdd, toast.ClientError(nf.err))
		p.syncEditingFields(p.widgets[idx])
		return
	}

	p.widgets[idx].Position = positionDTO{X: posX, Y: posY, Z: posZ}
	p.widgets[idx].Size = sizeDTO{Width: width, Height: height}
	p.widgets[idx].Origin = originDTO{X: originX, Y: originY}
	p.widgets[idx].Rotation = rotationDTO{Degrees: rotDeg}
	// Show the fields as the widget now holds them, so that the reload after
	// a rejected save puts the server's values back (mergeWidgetEditingFields
	// keeps only fields the user changed since).
	p.syncEditingFields(p.widgets[idx])
	p.putWidget(ctx, p.widgets[idx])
}

func (p *Project) bindPort(ctx app.Context, portName, tagID string) {
	if p.selectedWidgetID == "" || portName == "" || tagID == "" {
		return
	}
	idx := p.selectedWidgetIdx()
	if idx < 0 {
		return
	}
	found := false
	tagUUID, err := uuid.Parse(tagID)
	if err != nil {
		p.compoCtx.NewActionWithValue(toast.ActionAdd, toast.ClientError(err))
		return
	}
	for i, b := range p.widgets[idx].PortBindings {
		if b.PortName == portName {
			p.widgets[idx].PortBindings[i].TagID = tagUUID
			found = true
			break
		}
	}
	if !found {
		p.widgets[idx].PortBindings = append(p.widgets[idx].PortBindings, portBindingDTO{
			PortName: portName,
			TagID:    tagUUID,
		})
	}
	p.putWidget(ctx, p.widgets[idx])
}

func (p *Project) unbindPort(ctx app.Context, portName string) {
	if p.selectedWidgetID == "" {
		return
	}
	idx := p.selectedWidgetIdx()
	if idx < 0 {
		return
	}
	filtered := make([]portBindingDTO, 0, len(p.widgets[idx].PortBindings))
	for _, b := range p.widgets[idx].PortBindings {
		if b.PortName != portName {
			filtered = append(filtered, b)
		}
	}
	p.widgets[idx].PortBindings = filtered
	p.putWidget(ctx, p.widgets[idx])
}

func (p *Project) putWidget(ctx app.Context, w widgetItem) {
	sceneID := p.selectedSceneID
	input := apiv0.UpdateWidgetRequest{Name: w.Name, Position: apiv0.PositionRequest(w.Position), Size: apiv0.SizeRequest(w.Size), Origin: apiv0.OriginRequest(w.Origin), Rotation: apiv0.RotationRequest(w.Rotation), TypeID: w.TypeID, PortBindings: w.PortBindings, Labels: w.Labels, SceneVersion: p.currentSceneVersion()}
	if input.PortBindings == nil {
		input.PortBindings = []apiv0.PortBinding{}
	}
	if input.Labels == nil {
		input.Labels = []string{}
	}
	uiutil.Request(p.tasks, func(ctx context.Context) (apiv0.UpdateWidgetResponse, error) {
		id, err := uuid.Parse(sceneID)
		if err != nil {
			return apiv0.UpdateWidgetResponse{}, err
		}
		return p.client.UpdateWidget(ctx, id, w.ID, input)
	}, func(result apiv0.UpdateWidgetResponse) { p.applyNewSceneVersion(sceneID, result.SceneVersion) }, func() {
		p.loadScene(p.compoCtx, sceneID)
		p.loadWidgets(p.compoCtx)
	})
}

// ── Drag finalisation ─────────────────────────────────────────────────────────

// widgetGeometry is the part of a widget a canvas drag changes.
type widgetGeometry struct {
	Position positionDTO
	Size     sizeDTO
	Origin   originDTO
	Rotation rotationDTO
}

func geometryOf(w widgetItem) widgetGeometry {
	return widgetGeometry{Position: w.Position, Size: w.Size, Origin: w.Origin, Rotation: w.Rotation}
}

// draggedWidgetID returns the widget a canvas drag (move, origin, rotate or
// resize) is changing, or "" when none is.
func (p *Project) draggedWidgetID() string {
	for _, id := range []string{p.draggingWidgetID, p.draggingOriginID, p.rotatingWidgetID, p.resizingWidgetID} {
		if id != "" {
			return id
		}
	}
	return ""
}

// dragChangedWidget reports whether the drag that is ending changed w: a
// click (no move) or a drag back to the start leaves nothing to save, and
// saving it anyway would bump the scene version for every other client.
func (p *Project) dragChangedWidget(w widgetItem) bool {
	return p.dragDidMove && geometryOf(w) != p.dragStartGeometry
}

func (p *Project) finalizeAllDrags(ctx app.Context) {
	anyDrag := false
	for _, idPtr := range []*string{
		&p.draggingWidgetID,
		&p.draggingOriginID,
		&p.rotatingWidgetID,
		&p.resizingWidgetID,
	} {
		if *idPtr != "" {
			anyDrag = true
			id := *idPtr
			*idPtr = ""
			p.saveDraggedWidget(ctx, id)
		}
	}
	if anyDrag {
		p.dragJustEnded = p.dragDidMove
		p.dragDidMove = false
	}
	if p.widgetsReloadDeferred {
		p.widgetsReloadDeferred = false
		p.loadWidgets(ctx)
	}
}

func (p *Project) saveDraggedWidget(ctx app.Context, id string) {
	for i := range p.widgets {
		if p.widgets[i].ID.String() == id {
			if !p.dragChangedWidget(p.widgets[i]) {
				break
			}
			if p.selectedWidgetID == id {
				p.syncEditingFields(p.widgets[i])
			}
			p.putWidget(ctx, p.widgets[i])
			break
		}
	}
}
