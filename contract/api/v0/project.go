package apiv0

import "github.com/google/uuid"

type ProjectResponse struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	Version        int       `json:"version"`
}
type GetProjectsResponse struct {
	Projects []ProjectResponse `json:"projects"`
}
type CreateProjectRequest struct {
	Name string `json:"name"`
}
type UpdateProjectRequest struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// MainProjectID is the migration's initial project. Older unscoped routes are
// compatibility aliases for it; new clients use explicit project routes.
const MainProjectID = "00000000-0000-4000-8000-000000000001"
