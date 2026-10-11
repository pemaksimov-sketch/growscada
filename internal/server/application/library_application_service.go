package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/application/appdto"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/library"
	"github.com/kipitix/growscada/internal/server/domain/project"
	"github.com/kipitix/growscada/internal/server/domain/version"
)

type LibraryService interface {
	FindAll(context.Context) ([]appdto.Library, error)
	Create(context.Context, string, bool) (appdto.Library, error)
	Update(context.Context, uuid.UUID, string, bool, int) (appdto.Library, error)
	FindReleases(context.Context, uuid.UUID) ([]appdto.LibraryRelease, error)
	Release(context.Context, uuid.UUID) (appdto.LibraryRelease, error)
	FindProjectPins(context.Context, uuid.UUID) ([]appdto.LibraryPin, error)
	PinRelease(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	UnpinLibrary(context.Context, uuid.UUID, uuid.UUID) error
}

type libraryService struct {
	repository     library.Repository
	organizationID id.ID[project.Organization]
	widgetTypes    func(uuid.UUID) library.WidgetTypeRepository
}

func NewLibraryService(repository library.Repository, organizationID id.ID[project.Organization], widgetTypes func(uuid.UUID) library.WidgetTypeRepository) LibraryService {
	return &libraryService{repository: repository, organizationID: organizationID, widgetTypes: widgetTypes}
}

func (s *libraryService) FindAll(ctx context.Context) ([]appdto.Library, error) {
	items, err := s.repository.FindAll(ctx, s.organizationID)
	if err != nil {
		return nil, err
	}
	result := make([]appdto.Library, len(items))
	for i, item := range items {
		result[i] = appdto.NewLibrary(item)
	}
	return result, nil
}
func (s *libraryService) Create(ctx context.Context, name string, isPublic bool) (appdto.Library, error) {
	n, err := library.NewLibraryName(name)
	if err != nil {
		return appdto.Library{}, invalidInput(err)
	}
	item, err := s.repository.Save(ctx, library.CreateLibrary(s.repository.NextID(), s.organizationID, n, isPublic))
	if err != nil {
		return appdto.Library{}, err
	}
	return appdto.NewLibrary(item), nil
}
func (s *libraryService) Update(ctx context.Context, rawID uuid.UUID, name string, isPublic bool, expected int) (appdto.Library, error) {
	n, err := library.NewLibraryName(name)
	if err != nil {
		return appdto.Library{}, invalidInput(err)
	}
	ver, err := version.New(version.WithNumber[library.Library](expected))
	if err != nil {
		return appdto.Library{}, invalidInput(err)
	}
	item, err := s.repository.FindByID(ctx, id.NewID(id.IDWithUUID[library.Library](rawID)))
	if err != nil {
		return appdto.Library{}, err
	}
	item, err = item.Update(ver, n, isPublic)
	if err != nil {
		return appdto.Library{}, err
	}
	item, err = s.repository.Save(ctx, item)
	if err != nil {
		return appdto.Library{}, err
	}
	return appdto.NewLibrary(item), nil
}
func (s *libraryService) FindReleases(ctx context.Context, rawID uuid.UUID) ([]appdto.LibraryRelease, error) {
	items, err := s.repository.FindReleases(ctx, id.NewID(id.IDWithUUID[library.Library](rawID)))
	if err != nil {
		return nil, err
	}
	result := make([]appdto.LibraryRelease, len(items))
	for i, item := range items {
		result[i] = appdto.NewLibraryRelease(item)
	}
	return result, nil
}
func (s *libraryService) Release(ctx context.Context, rawID uuid.UUID) (appdto.LibraryRelease, error) {
	libraryID := id.NewID(id.IDWithUUID[library.Library](rawID))
	if _, err := s.repository.FindByID(ctx, libraryID); err != nil {
		return appdto.LibraryRelease{}, err
	}
	items, err := s.widgetTypes(rawID).FindAll(ctx)
	if err != nil {
		return appdto.LibraryRelease{}, fmt.Errorf("list library widget types: %w", err)
	}
	types := make([]releaseWidgetType, 0, len(items))
	for _, item := range items {
		types = append(types, releaseWidgetType{ID: item.ID().UUID(), Name: item.Name().String(), HTMLTemplate: item.HtmlTemplate().String(), Script: item.Script().String(), ScriptLanguage: item.ScriptLanguage().String(), DefaultWidth: item.DefaultSize().Width(), DefaultHeight: item.DefaultSize().Height(), InputPorts: releaseInputPorts(item.InputPorts()), Version: item.Version().Number()})
	}
	content, err := json.Marshal(struct {
		WidgetTypes []releaseWidgetType `json:"widget_types"`
	}{types})
	if err != nil {
		return appdto.LibraryRelease{}, err
	}
	release, err := s.repository.CreateRelease(ctx, libraryID, content)
	if err != nil {
		return appdto.LibraryRelease{}, err
	}
	return appdto.NewLibraryRelease(release), nil
}

type releaseWidgetType struct {
	ID             uuid.UUID          `json:"id"`
	Name           string             `json:"name"`
	HTMLTemplate   string             `json:"html_template"`
	Script         string             `json:"script"`
	ScriptLanguage string             `json:"script_language"`
	DefaultWidth   int                `json:"default_width"`
	DefaultHeight  int                `json:"default_height"`
	InputPorts     []releaseInputPort `json:"input_ports"`
	Version        int                `json:"version"`
}
type releaseInputPort struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TypeHint    string `json:"type_hint"`
}

func releaseInputPorts(ports []library.InputPort) []releaseInputPort {
	result := make([]releaseInputPort, len(ports))
	for i, p := range ports {
		result[i] = releaseInputPort{Name: p.Name().String(), Description: p.Description(), TypeHint: p.TypeHint().String()}
	}
	return result
}
func (s *libraryService) FindProjectPins(ctx context.Context, rawID uuid.UUID) ([]appdto.LibraryPin, error) {
	items, err := s.repository.FindProjectPins(ctx, id.NewID(id.IDWithUUID[project.Project](rawID)))
	if err != nil {
		return nil, err
	}
	result := make([]appdto.LibraryPin, len(items))
	for i, item := range items {
		result[i] = appdto.LibraryPin{ProjectID: item.ProjectID.UUID(), Release: appdto.NewLibraryRelease(item.Release)}
	}
	return result, nil
}
func (s *libraryService) PinRelease(ctx context.Context, projectID, libraryID, releaseID uuid.UUID) error {
	return s.repository.PinRelease(ctx, id.NewID(id.IDWithUUID[project.Project](projectID)), s.organizationID, id.NewID(id.IDWithUUID[library.Library](libraryID)), releaseID)
}
func (s *libraryService) UnpinLibrary(ctx context.Context, projectID, libraryID uuid.UUID) error {
	return s.repository.UnpinLibrary(ctx, id.NewID(id.IDWithUUID[project.Project](projectID)), id.NewID(id.IDWithUUID[library.Library](libraryID)))
}
