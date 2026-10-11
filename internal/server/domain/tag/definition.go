package tag

import (
	"errors"
	"slices"

	"github.com/kipitix/growscada/internal/server/domain/entityorigin"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/version"
)

var ErrInvalidDefinition = errors.New("invalid tag definition")

// Definition is immutable configuration. Values and Quality belong to State,
// which has its own write order and never participates in this edit Version.
// Tag remains the identity type used by bindings and device links.
type Definition struct {
	id      id.ID[Tag]
	name    TagName
	tagType TagType
	origin  entityorigin.Origin
	version version.Version[Definition]
	pending []event.Event
}

func CreateDefinition(anID id.ID[Tag], name TagName, tagType TagType, origin entityorigin.Origin) (Definition, error) {
	d, err := ReconstituteDefinition(anID, name, tagType, origin, version.Initial[Definition]())
	if err != nil {
		return Definition{}, err
	}
	d.pending = []event.Event{NewTagCreatedEvent(anID)}
	return d, nil
}
func ReconstituteDefinition(anID id.ID[Tag], name TagName, tagType TagType, origin entityorigin.Origin, v version.Version[Definition]) (Definition, error) {
	if _, err := NewTagName(name.String()); err != nil || !tagType.IsValid() || !origin.IsValid() {
		return Definition{}, ErrInvalidDefinition
	}
	return Definition{id: anID, name: name, tagType: tagType, origin: origin, version: v}, nil
}
func (d Definition) ID() id.ID[Tag]                       { return d.id }
func (d Definition) Name() TagName                        { return d.name }
func (d Definition) Type() TagType                        { return d.tagType }
func (d Definition) Origin() entityorigin.Origin          { return d.origin }
func (d Definition) Version() version.Version[Definition] { return d.version }
func (d Definition) PendingEvents() []event.Event         { return slices.Clone(d.pending) }

// Rename returns a new intent at the loaded version; only the repository may
// advance the edit counter after a successful compare-and-swap.
func (d Definition) Rename(expected version.Version[Definition], name TagName) (Definition, error) {
	if expected != d.version {
		return Definition{}, ErrTagConflict
	}
	updated, err := ReconstituteDefinition(d.id, name, d.tagType, d.origin, d.version)
	if err != nil {
		return Definition{}, err
	}
	updated.pending = append(slices.Clone(d.pending), NewDefinitionUpdatedEvent(d.id))
	return updated, nil
}
func (d Definition) Delete() Definition {
	d.pending = append(slices.Clone(d.pending), NewTagDeletedEvent(d.id))
	return d
}
