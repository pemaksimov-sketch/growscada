package appdto

import (
	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/domain/project"
)

type Project struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Version        int
}

func NewProject(p project.Project) Project {
	return Project{ID: p.ID().UUID(), OrganizationID: p.OrganizationID().UUID(), Name: p.Name().String(), Version: p.Version().Number()}
}
