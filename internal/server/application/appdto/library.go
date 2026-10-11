package appdto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/domain/library"
)

type Library struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Public         bool
	Version        int
}

func NewLibrary(l library.Library) Library {
	return Library{ID: l.ID().UUID(), OrganizationID: l.OrganizationID().UUID(), Name: l.Name().String(), Public: l.IsPublic(), Version: l.Version().Number()}
}

type LibraryRelease struct {
	ID        uuid.UUID
	LibraryID uuid.UUID
	Number    int
	CreatedAt time.Time
	Content   json.RawMessage
}

func NewLibraryRelease(r library.Release) LibraryRelease {
	return LibraryRelease{ID: r.ID(), LibraryID: r.LibraryID().UUID(), Number: r.Number(), CreatedAt: r.CreatedAt().Time(), Content: r.Content()}
}

type LibraryPin struct {
	ProjectID uuid.UUID
	Release   LibraryRelease
}
