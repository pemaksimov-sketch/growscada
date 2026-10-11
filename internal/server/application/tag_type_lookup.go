package application

import (
	"context"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/tag"
)

// TagTypeLookup exposes configuration only. Binding checks never read process values.
type TagTypeLookup interface {
	FindTagType(context.Context, id.ID[tag.Tag]) (tag.TagType, error)
}

type legacyTagTypeLookup struct{ repository tag.TagRepository }

func NewLegacyTagTypeLookup(repository tag.TagRepository) TagTypeLookup {
	return legacyTagTypeLookup{repository}
}
func (l legacyTagTypeLookup) FindTagType(ctx context.Context, tagID id.ID[tag.Tag]) (tag.TagType, error) {
	item, err := l.repository.FindByID(ctx, tagID)
	if err != nil {
		return tag.TagType{}, err
	}
	return item.Type(), nil
}

type definitionTagTypeLookup struct{ repository tag.DefinitionRepository }

func NewDefinitionTagTypeLookup(repository tag.DefinitionRepository) TagTypeLookup {
	return definitionTagTypeLookup{repository}
}
func (l definitionTagTypeLookup) FindTagType(ctx context.Context, tagID id.ID[tag.Tag]) (tag.TagType, error) {
	item, err := l.repository.FindByID(ctx, tagID)
	if err != nil {
		return tag.TagType{}, err
	}
	return item.Type(), nil
}
