package tag

import (
	"errors"
	"math"

	"github.com/kipitix/growscada/internal/server/domain/id"
)

var ErrInvalidState = errors.New("invalid tag state")

// State is Runtime process state, independent of the Draft edit Version.
// Sequence orders observations; callers never provide an expected sequence.
// It records no outbox event. Runtime delivery/Journal own that responsibility.
type State struct {
	id       id.ID[Tag]
	tagType  TagType
	value    TagValue
	quality  TagQuality
	sequence int64
}

func ReconstituteState(anID id.ID[Tag], tagType TagType, value any, quality TagQuality, sequence int64) (State, error) {
	if !tagType.IsValid() || !quality.IsValid() || sequence < 0 {
		return State{}, ErrInvalidState
	}
	v, err := tagType.NewTagValue(value)
	if err != nil {
		return State{}, err
	}
	return State{id: anID, tagType: tagType, value: v, quality: quality, sequence: sequence}, nil
}
func (s State) ID() id.ID[Tag]      { return s.id }
func (s State) Type() TagType       { return s.tagType }
func (s State) Value() TagValue     { return s.value }
func (s State) Quality() TagQuality { return s.quality }
func (s State) Sequence() int64     { return s.sequence }
func (s State) Write(value any, quality TagQuality) (State, error) {
	if s.sequence == math.MaxInt64 {
		return State{}, ErrInvalidState
	}
	return ReconstituteState(s.id, s.tagType, value, quality, s.sequence+1)
}
