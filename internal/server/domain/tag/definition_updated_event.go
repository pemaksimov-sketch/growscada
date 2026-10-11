package tag

import (
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
)

// DefinitionUpdatedEvent invalidates configuration, never a process value.
type DefinitionUpdatedEvent struct{ tagEventImpl }

func NewDefinitionUpdatedEvent(tagID id.ID[Tag], opts ...event.EventOption) DefinitionUpdatedEvent {
	return DefinitionUpdatedEvent{tagEventImpl{Base: event.NewBase(event.EventTypeTagDefinitionUpdated, opts...), tagID: tagID}}
}
