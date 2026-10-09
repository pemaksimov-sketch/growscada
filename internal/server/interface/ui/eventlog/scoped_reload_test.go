package eventlog

import (
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"testing"
)

func TestContractEventNamesMatchDomainAndPresentation(t *testing.T) {
	names := apiv0.EventTypes()
	if len(names) != len(eventTypeInfoByType) || len(names) != len(event.AllEventTypes()) {
		t.Fatal("event catalogs differ")
	}
	for _, name := range names {
		if _, ok := eventTypeInfoByType[name]; !ok {
			t.Errorf("missing presentation for %s", name)
		}
		if _, err := event.NewEventType(name); err != nil {
			t.Error(err)
		}
	}
}

func TestWidgetEventsReloadOnlyTheirScene(t *testing.T) {
	for _, name := range []string{apiv0.EventWidgetCreated, apiv0.EventWidgetUpdated, apiv0.EventWidgetDeleted} {
		for _, selected := range []string{"a", "b", ""} {
			got := ReloadTargetsForScene(ServerEvent{Type: name, SceneID: "a"}, selected)
			want := ReloadTargets{SceneID: "a", Widgets: selected == "a"}
			if got != want {
				t.Errorf("%s selected %s: %+v, want %+v", name, selected, got, want)
			}
		}
		if got := ReloadTargetsForScene(ServerEvent{Type: name}, "a"); got != ReloadTargetsFor(name) {
			t.Fatal("older server fallback lost")
		}
	}
	if got := ReloadTargetsForScene(ServerEvent{Type: EventResync}, "a"); got != AllTargets {
		t.Fatal("resync must reload all")
	}
}
