package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/project"
	"github.com/kipitix/growscada/internal/server/domain/version"
)

var (
	ErrLibraryNotFound    = errors.New("library not found")
	ErrLibraryConflict    = errors.New("library version conflict")
	ErrLibraryNameTaken   = errors.New("library name already exists in organization")
	ErrInvalidLibraryName = errors.New("library name must contain 1 to 128 characters")
	ErrReleaseNotFound    = errors.New("library release not found")
	ErrProjectPinNotFound = errors.New("project library pin not found")
)

type Name struct{ value string }

func NewLibraryName(value string) (Name, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 128 {
		return Name{}, ErrInvalidLibraryName
	}
	return Name{value: value}, nil
}
func (n Name) String() string { return n.value }

// Library is an independently versioned catalog. WidgetTypes are its Draft;
// Projects can consume them only through immutable Releases.
type Library struct {
	id             id.ID[Library]
	organizationID id.ID[project.Organization]
	name           Name
	public         bool
	version        version.Version[Library]
	pending        []event.Event
}

func CreateLibrary(libraryID id.ID[Library], organizationID id.ID[project.Organization], name Name, public bool) Library {
	l := ReconstituteLibrary(libraryID, organizationID, name, public, version.Initial[Library]())
	l.pending = []event.Event{NewLibraryCreatedEvent(libraryID)}
	return l
}
func ReconstituteLibrary(libraryID id.ID[Library], organizationID id.ID[project.Organization], name Name, public bool, v version.Version[Library]) Library {
	return Library{id: libraryID, organizationID: organizationID, name: name, public: public, version: v}
}
func (l Library) ID() id.ID[Library]                          { return l.id }
func (l Library) OrganizationID() id.ID[project.Organization] { return l.organizationID }
func (l Library) Name() Name                                  { return l.name }
func (l Library) IsPublic() bool                              { return l.public }
func (l Library) Version() version.Version[Library]           { return l.version }
func (l Library) PendingEvents() []event.Event                { return slices.Clone(l.pending) }
func (l Library) Update(expected version.Version[Library], name Name, public bool) (Library, error) {
	if expected != l.version {
		return Library{}, ErrLibraryConflict
	}
	l.name, l.public = name, public
	l.pending = append(slices.Clone(l.pending), NewLibraryUpdatedEvent(l.id))
	return l, nil
}

type LibraryEvent struct {
	event.Base
	libraryID id.ID[Library]
}

func NewLibraryUpdatedEvent(libraryID id.ID[Library], opts ...event.EventOption) LibraryEvent {
	return LibraryEvent{Base: event.NewBase(event.EventTypeLibraryUpdated, opts...), libraryID: libraryID}
}
func NewLibraryCreatedEvent(libraryID id.ID[Library], opts ...event.EventOption) LibraryEvent {
	return LibraryEvent{Base: event.NewBase(event.EventTypeLibraryCreated, opts...), libraryID: libraryID}
}
func NewLibraryEvent(eventType event.EventType, libraryID id.ID[Library], opts ...event.EventOption) LibraryEvent {
	return LibraryEvent{Base: event.NewBase(eventType, opts...), libraryID: libraryID}
}
func (e LibraryEvent) LibraryID() id.ID[Library] { return e.libraryID }

type LibraryReleasedEvent struct {
	event.Base
	libraryID id.ID[Library]
	releaseID uuid.UUID
	number    int
}

func NewLibraryReleasedEvent(libraryID id.ID[Library], releaseID uuid.UUID, number int, opts ...event.EventOption) LibraryReleasedEvent {
	return LibraryReleasedEvent{Base: event.NewBase(event.EventTypeLibraryReleased, opts...), libraryID: libraryID, releaseID: releaseID, number: number}
}
func (e LibraryReleasedEvent) LibraryID() id.ID[Library] { return e.libraryID }
func (e LibraryReleasedEvent) ReleaseID() uuid.UUID      { return e.releaseID }
func (e LibraryReleasedEvent) Number() int               { return e.number }

type Release struct {
	id        uuid.UUID
	libraryID id.ID[Library]
	number    int
	createdAt event.EventTimestamp
	content   json.RawMessage
}

func ReconstituteRelease(releaseID uuid.UUID, libraryID id.ID[Library], number int, createdAt event.EventTimestamp, content []byte) Release {
	return Release{id: releaseID, libraryID: libraryID, number: number, createdAt: createdAt, content: slices.Clone(content)}
}
func (r Release) ID() uuid.UUID                   { return r.id }
func (r Release) LibraryID() id.ID[Library]       { return r.libraryID }
func (r Release) Number() int                     { return r.number }
func (r Release) CreatedAt() event.EventTimestamp { return r.createdAt }
func (r Release) Content() json.RawMessage        { return slices.Clone(r.content) }

type ProjectPin struct {
	ProjectID id.ID[project.Project]
	Release   Release
}

type Repository interface {
	NextID() id.ID[Library]
	FindAll(context.Context, id.ID[project.Organization]) ([]Library, error)
	FindByID(context.Context, id.ID[Library]) (Library, error)
	Save(context.Context, Library) (Library, error)
	CreateRelease(context.Context, id.ID[Library], []byte) (Release, error)
	FindReleases(context.Context, id.ID[Library]) ([]Release, error)
	FindProjectPins(context.Context, id.ID[project.Project]) ([]ProjectPin, error)
	PinRelease(context.Context, id.ID[project.Project], id.ID[project.Organization], id.ID[Library], uuid.UUID) error
	UnpinLibrary(context.Context, id.ID[project.Project], id.ID[Library]) error
}

func ValidateReleaseContent(content []byte) error {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(content, &value); err != nil {
		return fmt.Errorf("invalid library release content: %w", err)
	}
	if value == nil {
		return errors.New("library release content must be a JSON object")
	}
	return nil
}
