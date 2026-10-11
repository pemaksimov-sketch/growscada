package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"net/http"
	"sync"
)

// Memory is a UI test adapter, not a substitute for server domain validation.
// It implements resource identity, deletion and optimistic versions, and
// copies snapshots so editing a response cannot mutate its stored state.
type Memory struct {
	mu      sync.Mutex
	tags    memoryTable[apiv0.TagResponse]
	scenes  memoryTable[apiv0.SceneResponse]
	types   memoryTable[apiv0.WidgetTypeResponse]
	widgets memoryTable[apiv0.WidgetResponse]
}

var _ Server = (*Memory)(nil)

func NewMemory() *Memory { return &Memory{} }

type memoryTable[T any] struct {
	values map[uuid.UUID]T
	order  []uuid.UUID
}

func copyMemory[T any](v T) T {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out T
	if err = json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}
func (m *memoryTable[T]) put(id uuid.UUID, v T) {
	if m.values == nil {
		m.values = map[uuid.UUID]T{}
	}
	if _, exists := m.values[id]; !exists {
		m.order = append(m.order, id)
	}
	m.values[id] = copyMemory(v)
}
func (m *memoryTable[T]) list() []T {
	out := []T{}
	for _, id := range m.order {
		if v, ok := m.values[id]; ok {
			out = append(out, copyMemory(v))
		}
	}
	return out
}
func (m *memoryTable[T]) get(id uuid.UUID) (T, error) {
	v, ok := m.values[id]
	if !ok {
		return v, memoryStatus(http.StatusNotFound, "Resource not found")
	}
	return copyMemory(v), nil
}
func memoryStatus(code int, detail string) error {
	return &StatusError{Status: code, Title: http.StatusText(code), Detail: detail, msg: fmt.Sprintf("%d: %s", code, detail)}
}
func memoryConflict() error { return memoryStatus(http.StatusConflict, "Version conflict") }

func (m *Memory) ListTags(ctx context.Context) ([]Tag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tags.list(), ctx.Err()
}
func (m *Memory) GetTag(ctx context.Context, id uuid.UUID) (Tag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Tag{}, err
	}
	return m.tags.get(id)
}
func (m *Memory) CreateTagResult(ctx context.Context, r apiv0.CreateTagRequest) (apiv0.CreateTagResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.CreateTagResponse{}, err
	}
	for _, t := range m.tags.values {
		if t.Name == r.Name {
			return apiv0.CreateTagResponse{}, memoryStatus(409, "Tag name already exists")
		}
	}
	id := uuid.New()
	m.tags.put(id, Tag{ID: id, Name: r.Name, Type: r.Type, Value: r.Value, Quality: r.Quality, Version: 1})
	return apiv0.CreateTagResponse{ID: id}, nil
}
func (m *Memory) SetTagValue(ctx context.Context, id uuid.UUID, value, quality string, version int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	t, err := m.tags.get(id)
	if err != nil {
		return 0, err
	}
	if t.Version != version {
		return 0, memoryConflict()
	}
	t.Value = value
	t.Quality = quality
	t.Version++
	m.tags.put(id, t)
	return t.Version, nil
}
func (m *Memory) DeleteTag(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := m.tags.get(id); err != nil {
		return err
	}
	delete(m.tags.values, id)
	return nil
}

func (m *Memory) ListScenes(ctx context.Context) ([]apiv0.SceneResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.scenes.list(), ctx.Err()
}
func (m *Memory) GetScene(ctx context.Context, id uuid.UUID) (apiv0.SceneResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.SceneResponse{}, err
	}
	return m.scenes.get(id)
}
func (m *Memory) CreateScene(ctx context.Context, r apiv0.CreateSceneRequest) (apiv0.CreateSceneResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.CreateSceneResponse{}, err
	}
	id := uuid.New()
	m.scenes.put(id, apiv0.SceneResponse{ID: id, Name: r.Name, Width: r.Width, Height: r.Height, BackgroundHTML: r.BackgroundHTML, Version: 1})
	return apiv0.CreateSceneResponse{ID: id}, nil
}
func (m *Memory) bumpScene(sc apiv0.SceneResponse) int {
	sc.Version++
	m.scenes.put(sc.ID, sc)
	for id, w := range m.widgets.values {
		if w.SceneID == sc.ID {
			w.SceneVersion = sc.Version
			m.widgets.put(id, w)
		}
	}
	return sc.Version
}
func (m *Memory) UpdateScene(ctx context.Context, id uuid.UUID, r apiv0.UpdateSceneRequest) (apiv0.UpdateSceneResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.UpdateSceneResponse{}, err
	}
	sc, err := m.scenes.get(id)
	if err != nil {
		return apiv0.UpdateSceneResponse{}, err
	}
	if sc.Version != r.Version {
		return apiv0.UpdateSceneResponse{}, memoryConflict()
	}
	sc.Name = r.Name
	sc.Width = r.Width
	sc.Height = r.Height
	sc.BackgroundHTML = r.BackgroundHTML
	return apiv0.UpdateSceneResponse{Version: m.bumpScene(sc)}, nil
}
func (m *Memory) DeleteScene(ctx context.Context, id uuid.UUID) (apiv0.SceneResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.SceneResponse{}, err
	}
	sc, err := m.scenes.get(id)
	if err != nil {
		return sc, err
	}
	delete(m.scenes.values, id)
	for id, w := range m.widgets.values {
		if w.SceneID == sc.ID {
			delete(m.widgets.values, id)
		}
	}
	return sc, nil
}

func (m *Memory) ListWidgets(ctx context.Context, sceneID uuid.UUID) ([]apiv0.WidgetResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := m.scenes.get(sceneID); err != nil {
		return nil, err
	}
	out := []apiv0.WidgetResponse{}
	for _, w := range m.widgets.list() {
		if w.SceneID == sceneID {
			out = append(out, w)
		}
	}
	return out, nil
}
func (m *Memory) widget(sceneID, id uuid.UUID) (apiv0.WidgetResponse, error) {
	if _, err := m.scenes.get(sceneID); err != nil {
		return apiv0.WidgetResponse{}, err
	}
	w, err := m.widgets.get(id)
	if err != nil {
		return w, err
	}
	if w.SceneID != sceneID {
		return apiv0.WidgetResponse{}, memoryStatus(404, "Widget not found in scene")
	}
	return w, nil
}
func (m *Memory) GetWidget(ctx context.Context, sceneID, id uuid.UUID) (apiv0.WidgetResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.WidgetResponse{}, err
	}
	return m.widget(sceneID, id)
}
func (m *Memory) CreateWidget(ctx context.Context, sceneID uuid.UUID, r apiv0.CreateWidgetRequest) (apiv0.CreateWidgetResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.CreateWidgetResponse{}, err
	}
	sc, err := m.scenes.get(sceneID)
	if err != nil {
		return apiv0.CreateWidgetResponse{}, err
	}
	if sc.Version != r.SceneVersion {
		return apiv0.CreateWidgetResponse{}, memoryConflict()
	}
	id := uuid.New()
	v := m.bumpScene(sc)
	m.widgets.put(id, apiv0.WidgetResponse{ID: id, Name: r.Name, SceneID: sceneID, SceneVersion: v, TypeID: r.TypeID, Position: apiv0.PositionResponse(r.Position), Size: apiv0.SizeResponse(r.Size), Origin: apiv0.OriginResponse(r.Origin), Rotation: apiv0.RotationResponse(r.Rotation), Labels: r.Labels, PortBindings: r.PortBindings})
	return apiv0.CreateWidgetResponse{ID: id, SceneVersion: v}, nil
}
func (m *Memory) UpdateWidget(ctx context.Context, sceneID, id uuid.UUID, r apiv0.UpdateWidgetRequest) (apiv0.UpdateWidgetResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.UpdateWidgetResponse{}, err
	}
	w, err := m.widget(sceneID, id)
	if err != nil {
		return apiv0.UpdateWidgetResponse{}, err
	}
	sc, err := m.scenes.get(sceneID)
	if err != nil {
		return apiv0.UpdateWidgetResponse{}, err
	}
	if sc.Version != r.SceneVersion {
		return apiv0.UpdateWidgetResponse{}, memoryConflict()
	}
	w.Name = r.Name
	w.Position = apiv0.PositionResponse(r.Position)
	w.Size = apiv0.SizeResponse(r.Size)
	w.Origin = apiv0.OriginResponse(r.Origin)
	w.Rotation = apiv0.RotationResponse(r.Rotation)
	w.TypeID = r.TypeID
	w.Labels = r.Labels
	w.PortBindings = r.PortBindings
	w.SceneVersion = m.bumpScene(sc)
	m.widgets.put(id, w)
	return apiv0.UpdateWidgetResponse{SceneVersion: w.SceneVersion}, nil
}
func (m *Memory) DeleteWidget(ctx context.Context, sceneID, id uuid.UUID) (apiv0.WidgetResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.WidgetResponse{}, err
	}
	w, err := m.widget(sceneID, id)
	if err != nil {
		return w, err
	}
	sc, err := m.scenes.get(sceneID)
	if err != nil {
		return w, err
	}
	delete(m.widgets.values, id)
	w.SceneVersion = m.bumpScene(sc)
	return w, nil
}

func (m *Memory) ListWidgetTypes(ctx context.Context) ([]apiv0.WidgetTypeResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.types.list(), ctx.Err()
}
func (m *Memory) GetWidgetType(ctx context.Context, id uuid.UUID) (apiv0.WidgetTypeResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.WidgetTypeResponse{}, err
	}
	return m.types.get(id)
}
func (m *Memory) CreateWidgetType(ctx context.Context, r apiv0.CreateWidgetTypeRequest) (apiv0.CreateWidgetTypeResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.CreateWidgetTypeResponse{}, err
	}
	id := uuid.New()
	m.types.put(id, apiv0.WidgetTypeResponse{ID: id, Name: r.Name, HtmlTemplate: r.HtmlTemplate, Script: r.Script, ScriptLanguage: r.ScriptLanguage, DefaultWidth: r.DefaultWidth, DefaultHeight: r.DefaultHeight, InputPorts: r.InputPorts, Version: 1})
	return apiv0.CreateWidgetTypeResponse{ID: id}, nil
}
func (m *Memory) UpdateWidgetType(ctx context.Context, id uuid.UUID, r apiv0.UpdateWidgetTypeRequest) (apiv0.UpdateWidgetTypeResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.UpdateWidgetTypeResponse{}, err
	}
	wt, err := m.types.get(id)
	if err != nil {
		return apiv0.UpdateWidgetTypeResponse{}, err
	}
	if wt.Version != r.Version {
		return apiv0.UpdateWidgetTypeResponse{}, memoryConflict()
	}
	wt.Name = r.Name
	wt.HtmlTemplate = r.HtmlTemplate
	wt.Script = r.Script
	wt.ScriptLanguage = r.ScriptLanguage
	wt.DefaultWidth = r.DefaultWidth
	wt.DefaultHeight = r.DefaultHeight
	wt.InputPorts = r.InputPorts
	wt.Version++
	m.types.put(id, wt)
	return apiv0.UpdateWidgetTypeResponse{Version: wt.Version}, nil
}
func (m *Memory) DeleteWidgetType(ctx context.Context, id uuid.UUID) (apiv0.WidgetTypeResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return apiv0.WidgetTypeResponse{}, err
	}
	wt, err := m.types.get(id)
	if err != nil {
		return wt, err
	}
	for _, w := range m.widgets.values {
		if w.TypeID == id {
			return wt, memoryStatus(409, "Widget type is in use")
		}
	}
	delete(m.types.values, id)
	return wt, nil
}
