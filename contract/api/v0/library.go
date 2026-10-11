package apiv0

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type LibraryResponse struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	Public         bool      `json:"public"`
	Version        int       `json:"version"`
}
type GetLibrariesResponse struct {
	Libraries []LibraryResponse `json:"libraries"`
}
type CreateLibraryRequest struct {
	Name   string `json:"name"`
	Public bool   `json:"public"`
}
type UpdateLibraryRequest struct {
	Name    string `json:"name"`
	Public  bool   `json:"public"`
	Version int    `json:"version"`
}
type LibraryReleaseResponse struct {
	ID        uuid.UUID       `json:"id"`
	LibraryID uuid.UUID       `json:"library_id"`
	Number    int             `json:"number"`
	CreatedAt time.Time       `json:"created_at"`
	Content   json.RawMessage `json:"content"`
}
type GetLibraryReleasesResponse struct {
	Releases []LibraryReleaseResponse `json:"releases"`
}
type CreateLibraryReleaseResponse struct {
	ID     uuid.UUID `json:"id"`
	Number int       `json:"number"`
}
type PinLibraryReleaseRequest struct {
	LibraryID uuid.UUID `json:"library_id"`
	ReleaseID uuid.UUID `json:"release_id"`
}
type ProjectLibraryPinResponse struct {
	LibraryID uuid.UUID              `json:"library_id"`
	Release   LibraryReleaseResponse `json:"release"`
}
type GetProjectLibraryPinsResponse struct {
	Pins []ProjectLibraryPinResponse `json:"pins"`
}
