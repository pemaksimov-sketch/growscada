package repositories

import (
	"database/sql"

	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/project"
	"github.com/kipitix/growscada/internal/server/domain/scene"
	"github.com/kipitix/growscada/internal/server/domain/tag"
)

// ProjectRepositories binds a typed Project ID before exposing any resource
// repository. Application/domain interfaces remain free of infrastructure
// scope and never obtain an unscoped repository from this factory.
type ProjectRepositories struct {
	db        *sql.DB
	projectID id.ID[project.Project]
	opts      []Option
}

func ForProject(db *sql.DB, projectID id.ID[project.Project], opts ...Option) ProjectRepositories {
	return ProjectRepositories{db: db, projectID: projectID, opts: opts}
}
func (r ProjectRepositories) Tags() tag.TagRepository {
	return newProjectTagRepository(r.db, r.projectID, r.opts...)
}
func (r ProjectRepositories) Scenes() scene.SceneRepository {
	return newProjectSceneRepository(r.db, r.projectID, r.opts...)
}
