package operation

import (
	"encoding/json"
	"github.com/google/uuid"
	"testing"

	"github.com/kipitix/growscada/internal/server/interface/ui/livescene"
)

func TestDecodeTagUpdate(t *testing.T) {
	raw := json.RawMessage(`{"id":"00000000-0000-4000-8000-000000000005","name":"speed","type":"integer","value":"42","quality":"uncertain","version":3}`)
	got, ok := decodeTagUpdate(raw)
	want := livescene.Tag{ID: uuid.MustParse("00000000-0000-4000-8000-000000000005"), Name: "speed", Type: "integer", Value: "42", Quality: string(livescene.QualityUncertain), Version: 3}
	if !ok || got != want {
		t.Errorf("got %+v, %v; want %+v", got, ok, want)
	}
	for _, bad := range []string{``, `null`, `{}`, `not json`} {
		if _, ok := decodeTagUpdate(json.RawMessage(bad)); ok {
			t.Errorf("decodeTagUpdate(%q) should fail", bad)
		}
	}
}

func tagV(id, value string, version int) livescene.Tag {
	return livescene.Tag{ID: uuid.MustParse(id), Type: "integer", Value: value, Quality: string(livescene.QualityGood), Version: version}
}

func TestApplyTagUpdate(t *testing.T) {
	tags := map[string]livescene.Tag{"00000000-0000-4000-8000-000000000005": tagV("00000000-0000-4000-8000-000000000005", "1", 5)}

	if got := applyTagUpdate(tags, tagV("00000000-0000-4000-8000-000000000005", "2", 6)); got["00000000-0000-4000-8000-000000000005"].Value != "2" {
		t.Errorf("a newer Version must apply, got %+v", got["00000000-0000-4000-8000-000000000005"])
	}
	if tags["00000000-0000-4000-8000-000000000005"].Value != "1" {
		t.Error("the given map must not be modified")
	}
	if got := applyTagUpdate(tags, tagV("00000000-0000-4000-8000-000000000005", "0", 4)); got["00000000-0000-4000-8000-000000000005"].Value != "1" {
		t.Errorf("an older Version must be ignored, got %+v", got["00000000-0000-4000-8000-000000000005"])
	}
	if got := applyTagUpdate(tags, tagV("00000000-0000-4000-8000-000000000005", "9", 5)); got["00000000-0000-4000-8000-000000000005"].Value != "1" {
		t.Errorf("the same Version must be ignored, got %+v", got["00000000-0000-4000-8000-000000000005"])
	}
	if got := applyTagUpdate(tags, tagV("00000000-0000-4000-8000-000000000006", "3", 1)); got["00000000-0000-4000-8000-000000000006"].Value != "3" || len(got) != 2 {
		t.Errorf("an unknown Tag must be added, got %+v", got)
	}
}

func TestMergeTags(t *testing.T) {
	known := map[string]livescene.Tag{
		"00000000-0000-4000-8000-000000000007": tagV("00000000-0000-4000-8000-000000000007", "event", 10),
		"00000000-0000-4000-8000-000000000008": tagV("00000000-0000-4000-8000-000000000008", "stale", 1),
		"00000000-0000-4000-8000-000000000009": tagV("00000000-0000-4000-8000-000000000009", "x", 1),
	}
	loaded := []livescene.Tag{
		tagV("00000000-0000-4000-8000-000000000007", "list", 9),
		tagV("00000000-0000-4000-8000-000000000008", "list", 2),
		tagV("00000000-0000-4000-8000-000000000010", "list", 1),
	}

	got := mergeTags(known, loaded)

	if got["00000000-0000-4000-8000-000000000007"].Value != "event" {
		t.Errorf("a list read before the event must not roll the value back, got %+v", got["00000000-0000-4000-8000-000000000007"])
	}
	if got["00000000-0000-4000-8000-000000000008"].Value != "list" {
		t.Errorf("a newer list entry must win, got %+v", got["00000000-0000-4000-8000-000000000008"])
	}
	if _, ok := got["00000000-0000-4000-8000-000000000009"]; ok {
		t.Error("a Tag missing from the list must be dropped")
	}
	if got["00000000-0000-4000-8000-000000000010"].Value != "list" {
		t.Errorf("a new Tag from the list must be added, got %+v", got["00000000-0000-4000-8000-000000000010"])
	}
}

func TestForgetVersions_FreshStateWinsAfterVersionsStartOver(t *testing.T) {
	// Seen before the server was restarted with a fresh database.
	known := map[string]livescene.Tag{"00000000-0000-4000-8000-000000000005": tagV("00000000-0000-4000-8000-000000000005", "old", 500)}

	forgotten := forgetVersions(known)

	if forgotten["00000000-0000-4000-8000-000000000005"].Value != "old" {
		t.Errorf("the value must stay on screen until fresh data arrives, got %+v", forgotten["00000000-0000-4000-8000-000000000005"])
	}
	if known["00000000-0000-4000-8000-000000000005"].Version != 500 {
		t.Error("the given map must not be modified")
	}
	if got := mergeTags(forgotten, []livescene.Tag{tagV("00000000-0000-4000-8000-000000000005", "list", 3)}); got["00000000-0000-4000-8000-000000000005"].Value != "list" {
		t.Errorf("the reloaded list must win over a forgotten Version, got %+v", got["00000000-0000-4000-8000-000000000005"])
	}
	if got := applyTagUpdate(forgotten, tagV("00000000-0000-4000-8000-000000000005", "event", 1)); got["00000000-0000-4000-8000-000000000005"].Value != "event" {
		t.Errorf("an event must win over a forgotten Version, got %+v", got["00000000-0000-4000-8000-000000000005"])
	}
}

func TestSelectScene(t *testing.T) {
	scenes := []livescene.Scene{{ID: uuid.MustParse("00000000-0000-4000-8000-000000000011")}, {ID: uuid.MustParse("00000000-0000-4000-8000-000000000012")}}
	if got := selectScene(scenes, "00000000-0000-4000-8000-000000000012"); got != "00000000-0000-4000-8000-000000000012" {
		t.Errorf("an existing selection must be kept, got %q", got)
	}
	if got := selectScene(scenes, "00000000-0000-4000-8000-000000000021"); got != "00000000-0000-4000-8000-000000000011" {
		t.Errorf("a deleted selection must fall back to the first Scene, got %q", got)
	}
	if got := selectScene(scenes, ""); got != "00000000-0000-4000-8000-000000000011" {
		t.Errorf("no selection must pick the first Scene, got %q", got)
	}
	if got := selectScene(nil, "00000000-0000-4000-8000-000000000011"); got != "" {
		t.Errorf("no Scenes must select nothing, got %q", got)
	}
}
