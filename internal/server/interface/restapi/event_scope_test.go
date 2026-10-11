package restapi

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/library"
	"github.com/kipitix/growscada/internal/server/domain/tag"
	"github.com/kipitix/growscada/internal/server/interface/eventbus"
)

func TestEventHub_ProjectStreamsCannotReceiveOtherProjectTags(t *testing.T) {
	hub := newEventHub(eventbus.NewEventBus(), 10)
	defer hub.Close()
	a, b := uuid.NewString(), uuid.NewString()
	ca, _ := hub.addClient(a)
	cb, _ := hub.addClient(b)
	legacy, _ := hub.addClient()
	tagID := id.NewID[tag.Tag]()
	hub.broadcast(tag.NewTagCreatedEvent(tagID, event.WithProjectID(a)))
	select {
	case payload := <-ca.messages:
		var wire apiv0.Event
		if err := json.Unmarshal(payload, &wire); err != nil {
			t.Fatal(err)
		}
		if wire.ProjectID != a || wire.ID != tagID.String() {
			t.Fatalf("missing scope: %+v", wire)
		}
	default:
		t.Fatal("owner did not receive event")
	}
	for _, client := range []*eventClient{cb, legacy} {
		select {
		case <-client.messages:
			t.Fatal("another project's event leaked")
		default:
		}
	}
	// Library edits remain shared until the release pinning task.
	hub.broadcast(library.NewWidgetTypeCreatedEvent(id.NewID[library.WidgetType]()))
	for _, client := range []*eventClient{ca, cb, legacy} {
		select {
		case <-client.messages:
		default:
			t.Fatal("shared library event missing")
		}
	}
}
