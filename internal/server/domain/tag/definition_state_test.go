package tag

import (
	"errors"
	"testing"

	"github.com/kipitix/growscada/internal/server/domain/entityorigin"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/version"
)

func TestDefinitionAndState_HaveIndependentConsistencyBoundaries(t *testing.T) {
	anID := id.NewID[Tag]()
	name, _ := NewTagName("counter")
	d, err := ReconstituteDefinition(anID, name, TagTypeInteger, entityorigin.Project, version.Committed[Definition]())
	if err != nil {
		t.Fatal(err)
	}
	s, err := ReconstituteState(anID, TagTypeInteger, "9007199254740993", TagQualityGood, 1)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Write("9007199254740994", TagQualityBad)
	if err != nil {
		t.Fatal(err)
	}
	second, err := first.Write("9007199254740995", TagQualityUncertain)
	if err != nil {
		t.Fatal(err)
	}
	if s.Value().String() != "9007199254740993" || first.Sequence() != 2 || second.Sequence() != 3 || second.Quality() != TagQualityUncertain {
		t.Fatalf("state writes lost order or mutated the previous observation: %+v %+v %+v", s, first, second)
	}
	newName, _ := NewTagName("renamed")
	updated, err := d.Rename(d.Version(), newName)
	if err != nil || updated.Name() != newName || d.Name() != name || d.Version().Number() != 1 {
		t.Fatalf("process writes affected the immutable definition: %+v %+v %v", d, updated, err)
	}
	if _, err := d.Rename(d.Version().Next(), newName); !errors.Is(err, ErrTagConflict) {
		t.Fatal("stale definition edit accepted")
	}
	if _, ok := any(second).(interface{ PendingEvents() []event.Event }); ok {
		t.Fatal("runtime value writes must not record outbox events")
	}
}

func TestDefinition_RejectsMissingOriginAndStateRejectsInvalidValues(t *testing.T) {
	name, _ := NewTagName("counter")
	if _, err := CreateDefinition(id.NewID[Tag](), name, TagTypeInteger, entityorigin.Origin{}); err == nil {
		t.Fatal("invalid Origin accepted")
	}
	for _, input := range []string{"", "Unknown", "unknown", "Projectx"} {
		if _, err := entityorigin.Parse(input); err == nil {
			t.Fatalf("accepted origin %q", input)
		}
	}
	for _, origin := range []entityorigin.Origin{entityorigin.Project, entityorigin.Runtime} {
		parsed, err := entityorigin.Parse(origin.String())
		if err != nil || parsed != origin {
			t.Fatalf("Origin round trip: %v", err)
		}
	}
	s, _ := ReconstituteState(id.NewID[Tag](), TagTypeInteger, "0", TagQualityGood, 0)
	if _, err := s.Write("9223372036854775808", TagQualityGood); err == nil {
		t.Fatal("integer overflow accepted")
	}
	if _, err := s.Write("1", TagQuality{}); err == nil {
		t.Fatal("invalid Quality accepted")
	}
}
