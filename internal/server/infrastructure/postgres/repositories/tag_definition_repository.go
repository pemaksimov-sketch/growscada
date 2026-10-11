package repositories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/domain/entityorigin"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/project"
	"github.com/kipitix/growscada/internal/server/domain/tag"
	"github.com/kipitix/growscada/internal/server/domain/version"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/outbox"
)

type tagDefinitionRepository struct {
	db        *sql.DB
	projectID id.ID[project.Project]
	onCommit  func()
}

var _ tag.DefinitionRepository = (*tagDefinitionRepository)(nil)

func NewTagDefinitionRepository(db *sql.DB, projectID id.ID[project.Project], opts ...Option) tag.DefinitionRepository {
	return &tagDefinitionRepository{db: db, projectID: projectID, onCommit: newOptions(opts).onCommit}
}
func (r *tagDefinitionRepository) NextID() id.ID[tag.Tag] { return id.NewID[tag.Tag]() }
func (r *tagDefinitionRepository) FindAll(ctx context.Context) ([]tag.Definition, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,type,origin,version FROM engineering.tag_definitions WHERE project_id=$1 ORDER BY name,id`, r.projectID.UUID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []tag.Definition{}
	for rows.Next() {
		d, err := scanDefinition(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}
func (r *tagDefinitionRepository) FindByID(ctx context.Context, tagID id.ID[tag.Tag]) (tag.Definition, error) {
	return scanDefinition(r.db.QueryRowContext(ctx, `SELECT id,name,type,origin,version FROM engineering.tag_definitions WHERE project_id=$1 AND id=$2`, r.projectID.UUID(), tagID.UUID()))
}
func (r *tagDefinitionRepository) FindByName(ctx context.Context, name tag.TagName) (tag.Definition, error) {
	return scanDefinition(r.db.QueryRowContext(ctx, `SELECT id,name,type,origin,version FROM engineering.tag_definitions WHERE project_id=$1 AND name=$2`, r.projectID.UUID(), name.String()))
}
func scanDefinition(row interface{ Scan(...any) error }) (tag.Definition, error) {
	var rawID uuid.UUID
	var rawName, rawType, rawOrigin string
	var number int
	if err := row.Scan(&rawID, &rawName, &rawType, &rawOrigin, &number); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return tag.Definition{}, tag.ErrTagNotFound
		}
		return tag.Definition{}, err
	}
	name, err := tag.NewTagName(rawName)
	if err != nil {
		return tag.Definition{}, err
	}
	kind, err := tag.NewTagType(rawType)
	if err != nil {
		return tag.Definition{}, err
	}
	origin, err := entityorigin.Parse(rawOrigin)
	if err != nil {
		return tag.Definition{}, err
	}
	v, err := version.New(version.WithNumber[tag.Definition](number))
	if err != nil {
		return tag.Definition{}, err
	}
	return tag.ReconstituteDefinition(id.NewID(id.IDWithUUID[tag.Tag](rawID)), name, kind, origin, v)
}
func (r *tagDefinitionRepository) Save(ctx context.Context, d tag.Definition) (tag.Definition, error) {
	if d.Origin() != entityorigin.Project {
		return tag.Definition{}, tag.ErrInvalidDefinition
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return tag.Definition{}, err
	}
	defer tx.Rollback()
	next := d.Version().Next()
	if !d.Version().IsCommitted() {
		_, err = tx.ExecContext(ctx, `INSERT INTO engineering.tag_definitions(id,project_id,name,type,origin,version) VALUES($1,$2,$3,$4,$5,$6)`, d.ID().UUID(), r.projectID.UUID(), d.Name().String(), d.Type().String(), d.Origin().String(), next.Number())
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `UPDATE engineering.tag_definitions SET name=$1,type=$2,origin=$3,version=$4 WHERE id=$5 AND project_id=$6 AND version=$7`, d.Name().String(), d.Type().String(), d.Origin().String(), next.Number(), d.ID().UUID(), r.projectID.UUID(), d.Version().Number())
		if err == nil {
			n, e := result.RowsAffected()
			err = e
			if err == nil && n != 1 {
				err = r.missingOrConflict(ctx, tx, d.ID())
			}
		}
	}
	if err != nil {
		return tag.Definition{}, tagWriteError("save tag definition", err)
	}
	pid := r.projectID.UUID()
	if err := outbox.AppendProject(ctx, tx, d.PendingEvents(), &pid); err != nil {
		return tag.Definition{}, err
	}
	if err := tx.Commit(); err != nil {
		return tag.Definition{}, err
	}
	if r.onCommit != nil && len(d.PendingEvents()) > 0 {
		r.onCommit()
	}
	return tag.ReconstituteDefinition(d.ID(), d.Name(), d.Type(), d.Origin(), next)
}
func (r *tagDefinitionRepository) Delete(ctx context.Context, d tag.Definition) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `DELETE FROM engineering.tag_definitions WHERE id=$1 AND project_id=$2 AND version=$3`, d.ID().UUID(), r.projectID.UUID(), d.Version().Number())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return r.missingOrConflict(ctx, tx, d.ID())
	}
	pid := r.projectID.UUID()
	if err := outbox.AppendProject(ctx, tx, d.PendingEvents(), &pid); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if r.onCommit != nil && len(d.PendingEvents()) > 0 {
		r.onCommit()
	}
	return nil
}
func (r *tagDefinitionRepository) missingOrConflict(ctx context.Context, tx *sql.Tx, tagID id.ID[tag.Tag]) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM engineering.tag_definitions WHERE id=$1 AND project_id=$2)`, tagID.UUID(), r.projectID.UUID()).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return tag.ErrTagConflict
	}
	return tag.ErrTagNotFound
}
