package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/application/appdto"
	"github.com/kipitix/growscada/internal/server/domain/entityorigin"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/tag"
)

type memoryDefinitions struct {
	items  map[id.ID[tag.Tag]]tag.Definition
	events []event.Event
}

func newMemoryDefinitions() *memoryDefinitions {
	return &memoryDefinitions{items: map[id.ID[tag.Tag]]tag.Definition{}}
}
func (r *memoryDefinitions) NextID() id.ID[tag.Tag] { return id.NewID[tag.Tag]() }
func (r *memoryDefinitions) FindAll(context.Context) ([]tag.Definition, error) {
	items := make([]tag.Definition, 0, len(r.items))
	for _, d := range r.items {
		items = append(items, d)
	}
	return items, nil
}
func (r *memoryDefinitions) FindByID(_ context.Context, key id.ID[tag.Tag]) (tag.Definition, error) {
	d, ok := r.items[key]
	if !ok {
		return tag.Definition{}, tag.ErrTagNotFound
	}
	return d, nil
}
func (r *memoryDefinitions) FindByName(_ context.Context, n tag.TagName) (tag.Definition, error) {
	for _, d := range r.items {
		if d.Name() == n {
			return d, nil
		}
	}
	return tag.Definition{}, tag.ErrTagNotFound
}
func (r *memoryDefinitions) Save(_ context.Context, d tag.Definition) (tag.Definition, error) {
	next := d.Version().Next()
	saved, err := tag.ReconstituteDefinition(d.ID(), d.Name(), d.Type(), d.Origin(), next)
	if err != nil {
		return tag.Definition{}, err
	}
	r.items[d.ID()] = saved
	r.events = append(r.events, d.PendingEvents()...)
	return saved, nil
}
func (r *memoryDefinitions) Delete(_ context.Context, d tag.Definition) error {
	if _, ok := r.items[d.ID()]; !ok {
		return tag.ErrTagNotFound
	}
	delete(r.items, d.ID())
	r.events = append(r.events, d.PendingEvents()...)
	return nil
}

type memoryRuntime struct {
	states map[id.ID[tag.Tag]]tag.State
	writes int
}

func newMemoryRuntime() *memoryRuntime { return &memoryRuntime{states: map[id.ID[tag.Tag]]tag.State{}} }
func (r *memoryRuntime) RegisterTag(_ context.Context, _ uuid.UUID, d tag.Definition, initial tag.State) error {
	if _, ok := r.states[d.ID()]; !ok {
		r.states[d.ID()] = initial
	}
	return nil
}
func (r *memoryRuntime) RemoveTag(_ context.Context, _ uuid.UUID, key id.ID[tag.Tag]) error {
	delete(r.states, key)
	return nil
}
func (r *memoryRuntime) ReadTag(_ context.Context, _ uuid.UUID, key id.ID[tag.Tag]) (tag.State, error) {
	s, ok := r.states[key]
	if !ok {
		return tag.State{}, tag.ErrTagNotFound
	}
	return s, nil
}
func (r *memoryRuntime) WriteTag(_ context.Context, _ uuid.UUID, key id.ID[tag.Tag], value string, q tag.TagQuality) (tag.State, error) {
	s, ok := r.states[key]
	if !ok {
		return tag.State{}, tag.ErrTagNotFound
	}
	next, err := s.Write(value, q)
	if err == nil {
		r.states[key] = next
		r.writes++
	}
	return next, err
}
func (r *memoryRuntime) RemoveProject(context.Context, uuid.UUID) error { return nil }

func TestSplitTagService_SeparatesDefinitionVersionFromRuntimeSequence(t *testing.T) {
	ctx := context.Background()
	defs := newMemoryDefinitions()
	runtime := newMemoryRuntime()
	projectID := uuid.New()
	svc := NewSplitTagService(defs, runtime, projectID)
	created, err := svc.CreateTag(ctx, appdto.CreateTagInput{Name: "counter", Type: "integer", Value: "9007199254740993", Quality: "good"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Version != 1 || created.Sequence != 1 || created.Origin != entityorigin.Project.String() || created.Value != "9007199254740993" {
		t.Fatalf("create: %+v", created)
	}
	for i, value := range []string{"9007199254740994", "9007199254740995"} {
		updated, err := svc.SetTagValueByID(ctx, appdto.UpdateTagInput{ID: created.ID, Value: value, Quality: "good", Version: -999})
		if err != nil {
			t.Fatal(err)
		}
		if updated.Version != created.Version || updated.Sequence != int64(i+2) {
			t.Fatalf("runtime write changed edit version or lost sequence: %+v", updated)
		}
		created = updated
	}
	name, _ := tag.NewTagName("renamed.counter")
	renamed, err := svc.RenameTagDefinition(ctx, created.ID, name.String(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != name.String() || renamed.Version != 2 || renamed.Sequence != 3 || renamed.Value != "9007199254740995" {
		t.Fatalf("rename changed runtime state: %+v", renamed)
	}
	if runtime.writes != 3 {
		t.Fatalf("expected three Runtime writes, got %d", runtime.writes)
	}
	var definitionUpdates int
	for _, e := range defs.events {
		if e.Type() == event.EventTypeTagDefinitionUpdated {
			definitionUpdates++
		}
	}
	if definitionUpdates != 1 {
		t.Fatalf("definition edits should be the only rename event: %v", defs.events)
	}
}
