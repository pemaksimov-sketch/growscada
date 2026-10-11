package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/tag"
)

// RuntimePort is the only way Engineering accesses process state. The local
// implementation owns its own connection; task 52 supplies the HTTP adapter.
// Registration is idempotent and never overwrites an existing process value.
type RuntimePort interface {
	RegisterTag(context.Context, uuid.UUID, tag.Definition, tag.State) error
	RemoveTag(context.Context, uuid.UUID, id.ID[tag.Tag]) error
	ReadTag(context.Context, uuid.UUID, id.ID[tag.Tag]) (tag.State, error)
	WriteTag(context.Context, uuid.UUID, id.ID[tag.Tag], string, tag.TagQuality) (tag.State, error)
	RemoveProject(context.Context, uuid.UUID) error
}
