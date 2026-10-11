package tag

import (
	"context"
	"github.com/kipitix/growscada/internal/server/domain/id"
)

// DefinitionRepository stores only configuration. No method reads or writes a
// process value; implementations must not query the Runtime schema.
type DefinitionRepository interface {
	NextID() id.ID[Tag]
	FindAll(context.Context) ([]Definition, error)
	FindByID(context.Context, id.ID[Tag]) (Definition, error)
	FindByName(context.Context, TagName) (Definition, error)
	Save(context.Context, Definition) (Definition, error)
	Delete(context.Context, Definition) error
}
