package project

import (
	"errors"
	"strings"
	"testing"

	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/version"
)

func TestProject_RenameIsImmutableAndRejectsStaleVersion(t *testing.T) {
	name, _ := NewName(" Main ")
	p := Create(id.NewID[Project](), DefaultOrganizationID(), name)
	if p.Name().String() != "Main" || len(p.PendingEvents()) != 1 || p.PendingEvents()[0].Type() != event.EventTypeProjectCreated {
		t.Fatal("invalid creation")
	}
	p = Reconstitute(p.ID(), p.OrganizationID(), p.Name(), version.Committed[Project]())
	newName, _ := NewName("Second")
	changed, err := p.Rename(p.Version(), newName)
	if err != nil || changed.Name() != newName || p.Name() != name || len(p.PendingEvents()) != 0 {
		t.Fatal("rename mutated original")
	}
	if _, err := p.Rename(p.Version().Next(), newName); !errors.Is(err, ErrConflict) {
		t.Fatal("stale rename accepted")
	}
	deleted := p.Delete()
	if len(deleted.PendingEvents()) != 1 || deleted.PendingEvents()[0].Type() != event.EventTypeProjectDeleted {
		t.Fatal("delete event missing")
	}
	copy := deleted.PendingEvents()
	copy[0] = nil
	if deleted.PendingEvents()[0] == nil {
		t.Fatal("pending events leaked")
	}
}

func TestProjectName(t *testing.T) {
	for _, value := range []string{"", "  ", strings.Repeat("я", 129)} {
		if _, err := NewName(value); !errors.Is(err, ErrInvalidName) {
			t.Fatalf("accepted %q", value)
		}
	}
	if _, err := NewName(strings.Repeat("я", 128)); err != nil {
		t.Fatal(err)
	}
}
