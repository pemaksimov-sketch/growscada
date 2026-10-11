package application

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/application/appdto"
	"github.com/kipitix/growscada/internal/server/domain/entityorigin"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/tag"
)

type splitTagService struct {
	definitions tag.DefinitionRepository
	runtime     RuntimePort
	projectID   uuid.UUID
}

var _ TagService = (*splitTagService)(nil)

func NewSplitTagService(definitions tag.DefinitionRepository, runtime RuntimePort, projectID uuid.UUID) *splitTagService {
	return &splitTagService{definitions: definitions, runtime: runtime, projectID: projectID}
}
func (s *splitTagService) FindAllTags(ctx context.Context) ([]appdto.Tag, error) {
	defs, err := s.definitions.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]appdto.Tag, 0, len(defs))
	for _, d := range defs {
		item, err := s.read(ctx, d)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
func (s *splitTagService) FindTagByID(ctx context.Context, rawID uuid.UUID) (appdto.Tag, error) {
	d, err := s.definitions.FindByID(ctx, id.NewID(id.IDWithUUID[tag.Tag](rawID)))
	if err != nil {
		return appdto.Tag{}, err
	}
	return s.read(ctx, d)
}
func (s *splitTagService) FindTagByName(ctx context.Context, rawName string) (appdto.Tag, error) {
	name, err := tag.NewTagName(rawName)
	if err != nil {
		return appdto.Tag{}, invalidInput(err)
	}
	d, err := s.definitions.FindByName(ctx, name)
	if err != nil {
		return appdto.Tag{}, err
	}
	return s.read(ctx, d)
}
func (s *splitTagService) FindTagsByNamePattern(ctx context.Context, pattern string) ([]appdto.Tag, error) {
	m, err := tag.NewTagNamePattern(pattern)
	if err != nil {
		return nil, invalidInput(err)
	}
	return s.match(ctx, m)
}
func (s *splitTagService) FindTagsByNameRegex(ctx context.Context, expr string) ([]appdto.Tag, error) {
	m, err := tag.NewTagNameRegex(expr)
	if err != nil {
		return nil, invalidInput(err)
	}
	return s.match(ctx, m)
}
func (s *splitTagService) match(ctx context.Context, matcher tag.TagNameMatcher) ([]appdto.Tag, error) {
	defs, err := s.definitions.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	items := []appdto.Tag{}
	for _, d := range defs {
		if matcher.Matches(d.Name()) {
			item, err := s.read(ctx, d)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
	}
	return items, nil
}
func (s *splitTagService) CreateTag(ctx context.Context, input appdto.CreateTagInput) (appdto.Tag, error) {
	name, err := tag.NewTagName(input.Name)
	if err != nil {
		return appdto.Tag{}, invalidInput(err)
	}
	t, err := tag.NewTagType(input.Type)
	if err != nil {
		return appdto.Tag{}, invalidInput(err)
	}
	// Optional initial value/Quality is a separate Runtime write, never part
	// of the immutable definition or any future Draft/Revision export.
	quality := tag.TagQualityBad
	initialValue := defaultProcessValue(t)
	if input.Quality != "" {
		quality, err = tag.NewTagQuality(input.Quality)
		if err != nil {
			return appdto.Tag{}, invalidInput(err)
		}
		initialValue = input.Value
	}
	if _, err := t.NewTagValue(initialValue); err != nil {
		return appdto.Tag{}, invalidInput(err)
	}
	d, err := tag.CreateDefinition(s.definitions.NextID(), name, t, entityorigin.Project)
	if err != nil {
		return appdto.Tag{}, invalidInput(err)
	}
	d, err = s.definitions.Save(ctx, d)
	if err != nil {
		return appdto.Tag{}, err
	}
	if err := s.ensureRegistered(ctx, d); err != nil {
		return appdto.Tag{}, err
	}
	state, err := s.runtime.WriteTag(ctx, s.projectID, d.ID(), initialValue, quality)
	if err != nil {
		return appdto.Tag{}, err
	}
	return tagDTO(d, state), nil
}
func (s *splitTagService) SetTagValueByID(ctx context.Context, input appdto.UpdateTagInput) (appdto.Tag, error) {
	d, err := s.definitions.FindByID(ctx, id.NewID(id.IDWithUUID[tag.Tag](input.ID)))
	if err != nil {
		return appdto.Tag{}, err
	}
	q, err := tag.NewTagQuality(input.Quality)
	if err != nil {
		return appdto.Tag{}, invalidInput(err)
	}
	if _, err := d.Type().NewTagValue(input.Value); err != nil {
		return appdto.Tag{}, invalidInput(err)
	}
	if err := s.ensureRegistered(ctx, d); err != nil {
		return appdto.Tag{}, err
	}
	state, err := s.runtime.WriteTag(ctx, s.projectID, d.ID(), input.Value, q)
	if err != nil {
		return appdto.Tag{}, err
	}
	return tagDTO(d, state), nil
}
func (s *splitTagService) DeleteTagByID(ctx context.Context, rawID uuid.UUID) (appdto.Tag, error) {
	var deleted appdto.Tag
	err := retryOnRace(tag.ErrTagConflict, func() error {
		d, err := s.definitions.FindByID(ctx, id.NewID(id.IDWithUUID[tag.Tag](rawID)))
		if err != nil {
			return err
		}
		deleted, err = s.read(ctx, d)
		if err != nil {
			return err
		}
		if err := s.definitions.Delete(ctx, d.Delete()); err != nil {
			return err
		}
		return s.runtime.RemoveTag(ctx, s.projectID, d.ID())
	})
	return deleted, err
}
func (s *splitTagService) ensureRegistered(ctx context.Context, d tag.Definition) error {
	initial, err := tag.ReconstituteState(d.ID(), d.Type(), defaultProcessValue(d.Type()), tag.TagQualityBad, 0)
	if err != nil {
		return err
	}
	return s.runtime.RegisterTag(ctx, s.projectID, d, initial)
}
func (s *splitTagService) read(ctx context.Context, d tag.Definition) (appdto.Tag, error) {
	state, err := s.runtime.ReadTag(ctx, s.projectID, d.ID())
	if errors.Is(err, tag.ErrTagNotFound) {
		if err := s.ensureRegistered(ctx, d); err != nil {
			return appdto.Tag{}, err
		}
		state, err = s.runtime.ReadTag(ctx, s.projectID, d.ID())
	}
	if err != nil {
		return appdto.Tag{}, err
	}
	return tagDTO(d, state), nil
}
func tagDTO(d tag.Definition, state tag.State) appdto.Tag {
	return appdto.Tag{ID: d.ID().UUID(), Name: d.Name().String(), Type: d.Type().String(), Origin: d.Origin().String(), Value: state.Value().String(), Quality: state.Quality().String(), Version: d.Version().Number(), Sequence: state.Sequence()}
}
func defaultProcessValue(t tag.TagType) string {
	switch t {
	case tag.TagTypeInteger:
		return "0"
	case tag.TagTypeBoolean:
		return "false"
	default:
		return ""
	}
}
