// Package project defines the isolation boundary for operational entities.
package project

import (
	"context"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/version"
)

type Organization struct{}

var (
	ErrNotFound    = errors.New("project not found")
	ErrConflict    = errors.New("project version conflict")
	ErrNameTaken   = errors.New("project name already exists in organization")
	ErrInvalidName = errors.New("project name must contain 1 to 128 characters")
)

// Seed IDs are stable across databases, manifests and local preferences.
func MainID() id.ID[Project] {
	return id.NewID(id.IDWithUUID[Project](uuid.MustParse("00000000-0000-4000-8000-000000000001")))
}
func DefaultOrganizationID() id.ID[Organization] {
	return id.NewID(id.IDWithUUID[Organization](uuid.MustParse("00000000-0000-4000-8000-000000000002")))
}

type Name struct{ value string }

func NewName(value string) (Name, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 128 {
		return Name{}, ErrInvalidName
	}
	return Name{value}, nil
}
func (n Name) String() string { return n.value }

// Project is immutable. Its Version protects metadata, independently of the
// Scene versions and runtime values held inside its isolation boundary.
type Project struct {
	id             id.ID[Project]
	organizationID id.ID[Organization]
	name           Name
	version        version.Version[Project]
	pending        []event.Event
}

func Create(anID id.ID[Project], organizationID id.ID[Organization], name Name) Project {
	p := Reconstitute(anID, organizationID, name, version.Initial[Project]())
	p.pending = []event.Event{NewEvent(event.EventTypeProjectCreated, anID)}
	return p
}
func Reconstitute(anID id.ID[Project], organizationID id.ID[Organization], name Name, v version.Version[Project]) Project {
	return Project{id: anID, organizationID: organizationID, name: name, version: v}
}
func (p Project) ID() id.ID[Project]                  { return p.id }
func (p Project) OrganizationID() id.ID[Organization] { return p.organizationID }
func (p Project) Name() Name                          { return p.name }
func (p Project) Version() version.Version[Project]   { return p.version }
func (p Project) PendingEvents() []event.Event        { return slices.Clone(p.pending) }
func (p Project) Rename(expected version.Version[Project], name Name) (Project, error) {
	if expected != p.version {
		return Project{}, ErrConflict
	}
	p.name = name
	p.pending = append(slices.Clone(p.pending), NewEvent(event.EventTypeProjectUpdated, p.id))
	return p, nil
}
func (p Project) Delete() Project {
	p.pending = append(slices.Clone(p.pending), NewEvent(event.EventTypeProjectDeleted, p.id))
	return p
}

type Event struct {
	event.Base
	projectID id.ID[Project]
}

func NewEvent(kind event.EventType, projectID id.ID[Project], opts ...event.EventOption) Event {
	return Event{Base: event.NewBase(kind, opts...), projectID: projectID}
}
func (e Event) ProjectID() id.ID[Project] { return e.projectID }

type Repository interface {
	NextID() id.ID[Project]
	FindAll(context.Context) ([]Project, error)
	FindByID(context.Context, id.ID[Project]) (Project, error)
	Save(context.Context, Project) (Project, error)
	Delete(context.Context, Project) error
}
