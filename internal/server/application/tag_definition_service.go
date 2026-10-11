package application

import (
	"context"
	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/application/appdto"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/tag"
	"github.com/kipitix/growscada/internal/server/domain/version"
)

type TagDefinitionService interface {
	RenameTagDefinition(context.Context, uuid.UUID, string, int) (appdto.Tag, error)
}

func (s *splitTagService) RenameTagDefinition(ctx context.Context, rawID uuid.UUID, rawName string, expected int) (appdto.Tag, error) {
	d, err := s.definitions.FindByID(ctx, id.NewID(id.IDWithUUID[tag.Tag](rawID)))
	if err != nil {
		return appdto.Tag{}, err
	}
	expectedVersion, err := version.New(version.WithNumber[tag.Definition](expected))
	if err != nil {
		return appdto.Tag{}, invalidInput(err)
	}
	name, err := tag.NewTagName(rawName)
	if err != nil {
		return appdto.Tag{}, invalidInput(err)
	}
	updated, err := d.Rename(expectedVersion, name)
	if err != nil {
		return appdto.Tag{}, err
	}
	updated, err = s.definitions.Save(ctx, updated)
	if err != nil {
		return appdto.Tag{}, err
	}
	if err := s.ensureRegistered(ctx, updated); err != nil { return appdto.Tag{}, err }
	return s.read(ctx, updated)
}
