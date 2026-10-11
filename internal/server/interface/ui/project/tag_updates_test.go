package project

import (
	"github.com/google/uuid"
	"testing"
)

func TestTagUpdateAndListDoNotRollBackNewerValues(t *testing.T) {
	original := []tagItem{{ID: uuid.MustParse("00000000-0000-4000-8000-000000000001"), Name: "level", Value: "1", Version: 1}}
	updated, applied := applyTagUpdate(original, tagItem{ID: uuid.MustParse("00000000-0000-4000-8000-000000000001"), Name: "level", Value: "3", Version: 3})
	if !applied || updated[0].Value != "3" || original[0].Value != "1" {
		t.Fatal("event not applied immutably")
	}
	stale, applied := applyTagUpdate(updated, original[0])
	if !applied || stale[0].Value != "3" {
		t.Fatal("stale event replaced state")
	}
	merged := mergeTagList(updated, original)
	if merged[0].Value != "3" {
		t.Fatal("racing list replaced state")
	}
	if _, applied := applyTagUpdate(original, tagItem{ID: uuid.MustParse("00000000-0000-4000-8000-000000000002"), Version: 1}); applied {
		t.Fatal("unknown tag needs a list resync")
	}
	if got := mergeTagList(updated, nil); len(got) != 0 {
		t.Fatal("deleted tags survived list resync")
	}
}

func TestDecodeTagUpdateRejectsIncompletePayload(t *testing.T) {
	for _, raw := range []string{`null`, `{}`, `{"id":"00000000-0000-4000-8000-000000000001"}`, `{"id":"00000000-0000-4000-8000-000000000001","version":-1}`, `bad`} {
		if _, ok := decodeTagUpdate([]byte(raw)); ok {
			t.Errorf("accepted %s", raw)
		}
	}
	if tag, ok := decodeTagUpdate([]byte(`{"id":"00000000-0000-4000-8000-000000000001","version":2,"value":"9007199254740993"}`)); !ok || tag.Value != "9007199254740993" {
		t.Fatal("lost raw value")
	}
}
