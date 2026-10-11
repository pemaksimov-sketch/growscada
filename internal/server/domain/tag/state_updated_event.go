package tag

import (
	"github.com/kipitix/growscada/internal/server/domain/entityorigin"
	"github.com/kipitix/growscada/internal/server/domain/event"
)

// StateUpdatedEvent is emitted after a Runtime commit. It is transient live
// delivery, not an engineering aggregate's pending event or an outbox row.
type StateUpdatedEvent struct {
	tagEventImpl
	name   TagName
	state  State
	origin entityorigin.Origin
}

func NewStateUpdatedEvent(name TagName, state State, origin entityorigin.Origin, opts ...event.EventOption) StateUpdatedEvent {
	return StateUpdatedEvent{tagEventImpl: tagEventImpl{Base: event.NewBase(event.EventTypeTagUpdated, opts...), tagID: state.ID()}, name: name, state: state, origin: origin}
}
func (e StateUpdatedEvent) State() State                { return e.state }
func (e StateUpdatedEvent) Name() TagName               { return e.name }
func (e StateUpdatedEvent) Origin() entityorigin.Origin { return e.origin }
