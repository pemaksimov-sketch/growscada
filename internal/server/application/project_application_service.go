package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/application/appdto"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/project"
	"github.com/kipitix/growscada/internal/server/domain/version"
)

type ProjectService interface {
	FindAll(context.Context) ([]appdto.Project, error)
	FindByID(context.Context, uuid.UUID) (appdto.Project, error)
	Create(context.Context, string) (appdto.Project, error)
	Rename(context.Context, uuid.UUID, string, int) (appdto.Project, error)
	Delete(context.Context, uuid.UUID) (appdto.Project, error)
}
type projectService struct {
	repository     project.Repository
	organizationID id.ID[project.Organization]
}

func NewProjectService(repository project.Repository, organizationID id.ID[project.Organization]) ProjectService {
	return &projectService{repository: repository, organizationID: organizationID}
}
func (s *projectService) FindAll(ctx context.Context) ([]appdto.Project, error) {
	items, err := s.repository.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]appdto.Project, len(items))
	for i, p := range items {
		result[i] = appdto.NewProject(p)
	}
	return result, nil
}
func (s *projectService) FindByID(ctx context.Context, rawID uuid.UUID) (appdto.Project, error) {
	p, err := s.repository.FindByID(ctx, id.NewID(id.IDWithUUID[project.Project](rawID)))
	if err != nil {
		return appdto.Project{}, err
	}
	return appdto.NewProject(p), nil
}
func (s *projectService) Create(ctx context.Context, rawName string) (appdto.Project, error) {
	name, err := project.NewName(rawName)
	if err != nil {
		return appdto.Project{}, invalidInput(err)
	}
	p, err := s.repository.Save(ctx, project.Create(s.repository.NextID(), s.organizationID, name))
	if err != nil {
		return appdto.Project{}, err
	}
	return appdto.NewProject(p), nil
}
func (s *projectService) Rename(ctx context.Context, rawID uuid.UUID, rawName string, expected int) (appdto.Project, error) {
	name, err := project.NewName(rawName)
	if err != nil {
		return appdto.Project{}, invalidInput(err)
	}
	v, err := version.New(version.WithNumber[project.Project](expected))
	if err != nil {
		return appdto.Project{}, invalidInput(err)
	}
	p, err := s.repository.FindByID(ctx, id.NewID(id.IDWithUUID[project.Project](rawID)))
	if err != nil {
		return appdto.Project{}, err
	}
	p, err = p.Rename(v, name)
	if err != nil {
		return appdto.Project{}, err
	}
	p, err = s.repository.Save(ctx, p)
	if err != nil {
		return appdto.Project{}, err
	}
	return appdto.NewProject(p), nil
}
func (s *projectService) Delete(ctx context.Context, rawID uuid.UUID) (appdto.Project, error) {
	var deleted project.Project
	err := retryOnRace(project.ErrConflict, func() error {
		p, err := s.repository.FindByID(ctx, id.NewID(id.IDWithUUID[project.Project](rawID)))
		if err != nil {
			return err
		}
		deleted = p
		return s.repository.Delete(ctx, p.Delete())
	})
	if err != nil {
		return appdto.Project{}, err
	}
	return appdto.NewProject(deleted), nil
}
