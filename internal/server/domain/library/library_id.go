package library

import (
	"github.com/google/uuid"
)

// DefaultLibraryID is the stable identifier for the seeded GrowSCADA library.
func DefaultLibraryID() uuid.UUID {
	return uuid.MustParse("00000000-0000-4000-8000-000000000003")
}
