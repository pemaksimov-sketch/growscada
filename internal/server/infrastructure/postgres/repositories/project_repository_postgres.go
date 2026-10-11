package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/project"
	"github.com/kipitix/growscada/internal/server/domain/version"
)

type projectRepositoryPostgres struct {
	db             *sql.DB
	organizationID id.ID[project.Organization]
	store          aggregateStore[project.Project]
}

func NewProjectRepositoryPostgres(db *sql.DB, organizationID id.ID[project.Organization], opts ...Option) project.Repository {
	o := newOptions(opts)
	return &projectRepositoryPostgres{db: db, organizationID: organizationID,
		store: aggregateStore[project.Project]{db: db, table: "projects", notFound: project.ErrNotFound, conflict: project.ErrConflict, onCommit: o.onCommit, scopeColumn: "organization_id", scopeID: organizationID.UUID()}}
}
func (r *projectRepositoryPostgres) NextID() id.ID[project.Project] {
	return id.NewID[project.Project]()
}
func (r *projectRepositoryPostgres) FindAll(ctx context.Context) ([]project.Project, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,version FROM projects WHERE organization_id=$1 ORDER BY name,id`, r.organizationID.UUID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []project.Project{}
	for rows.Next() {
		p, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
func (r *projectRepositoryPostgres) FindByID(ctx context.Context, projectID id.ID[project.Project]) (project.Project, error) {
	return r.scan(r.db.QueryRowContext(ctx, `SELECT id,name,version FROM projects WHERE id=$1 AND organization_id=$2`, projectID.UUID(), r.organizationID.UUID()))
}
func (r *projectRepositoryPostgres) scan(row interface{ Scan(...any) error }) (project.Project, error) {
	var rawID uuid.UUID
	var name string
	var number int
	if err := row.Scan(&rawID, &name, &number); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return project.Project{}, project.ErrNotFound
		}
		return project.Project{}, err
	}
	n, err := project.NewName(name)
	if err != nil {
		return project.Project{}, err
	}
	v, err := version.New(version.WithNumber[project.Project](number))
	if err != nil {
		return project.Project{}, err
	}
	return project.Reconstitute(id.NewID(id.IDWithUUID[project.Project](rawID)), r.organizationID, n, v), nil
}
func (r *projectRepositoryPostgres) Save(ctx context.Context, p project.Project) (project.Project, error) {
	if p.OrganizationID() != r.organizationID {
		return project.Project{}, project.ErrNotFound
	}
	v, err := r.store.save(ctx, p.ID(), p.Version(), p.PendingEvents(), rowWrite[project.Project]{
		insert: func(tx *sql.Tx, next version.Version[project.Project]) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO projects(id,organization_id,name,version) VALUES($1,$2,$3,$4)`, p.ID().UUID(), r.organizationID.UUID(), p.Name().String(), next.Number())
			return projectWriteError(err)
		},
		update: func(tx *sql.Tx, current, next version.Version[project.Project]) (bool, error) {
			res, err := tx.ExecContext(ctx, `UPDATE projects SET name=$1,version=$2 WHERE id=$3 AND organization_id=$4 AND version=$5`, p.Name().String(), next.Number(), p.ID().UUID(), r.organizationID.UUID(), current.Number())
			if err != nil {
				return false, projectWriteError(err)
			}
			n, err := res.RowsAffected()
			return n == 1, err
		},
	})
	if err != nil {
		return project.Project{}, err
	}
	return project.Reconstitute(p.ID(), p.OrganizationID(), p.Name(), v), nil
}
func (r *projectRepositoryPostgres) Delete(ctx context.Context, p project.Project) error {
	return r.store.delete(ctx, p.ID(), p.Version(), p.PendingEvents())
}
func projectWriteError(err error) error {
	if err == nil {
		return nil
	}
	if isUniqueViolation(err, "uq_projects_name") {
		return project.ErrNameTaken
	}
	return fmt.Errorf("cannot save project: %w", err)
}
