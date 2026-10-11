// Package runtime owns Runtime state persistence. All queries use only
// runtime.*; Engineering supplies configuration through application.RuntimePort.
package runtime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/application"
	"github.com/kipitix/growscada/internal/server/domain/entityorigin"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/tag"
)

type Store struct {
	db      *sql.DB
	publish func(event.Event)
}

var _ application.RuntimePort = (*Store)(nil)

func New(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) WithLiveEvents(publish func(event.Event)) *Store {
	return &Store{db: s.db, publish: publish}
}

func (s *Store) RegisterTag(ctx context.Context, projectID uuid.UUID, d tag.Definition, initial tag.State) error {
	if initial.ID() != d.ID() || initial.Type() != d.Type() {
		return tag.ErrInvalidState
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO runtime.tags(id,project_id,name,type,origin,value,quality,sequence)
	 VALUES($1,$2,$3,$4,$5,$6,$7,$8)
	 ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,origin=EXCLUDED.origin
	 WHERE runtime.tags.project_id=EXCLUDED.project_id AND runtime.tags.type=EXCLUDED.type`,
		d.ID().UUID(), projectID, d.Name().String(), d.Type().String(), d.Origin().String(), initial.Value().String(), initial.Quality().String(), initial.Sequence())
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return tag.ErrTagConflict
	}
	return nil
}

func (s *Store) ReadTag(ctx context.Context, projectID uuid.UUID, tagID id.ID[tag.Tag]) (tag.State, error) {
	return scanState(s.db.QueryRowContext(ctx, `SELECT type,value,quality,sequence FROM runtime.tags WHERE project_id=$1 AND id=$2`, projectID, tagID.UUID()), tagID)
}

// WriteTag serializes observations with a row lock, assigns the next sequence
// at commit and accepts no expected Version. It writes no outbox row.
func (s *Store) WriteTag(ctx context.Context, projectID uuid.UUID, tagID id.ID[tag.Tag], value string, quality tag.TagQuality) (tag.State, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return tag.State{}, err
	}
	defer tx.Rollback()
	current, err := scanState(tx.QueryRowContext(ctx, `SELECT type,value,quality,sequence FROM runtime.tags WHERE project_id=$1 AND id=$2 FOR UPDATE`, projectID, tagID.UUID()), tagID)
	if err != nil {
		return tag.State{}, err
	}
	next, err := current.Write(value, quality)
	if err != nil {
		return tag.State{}, fmt.Errorf("runtime value: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE runtime.tags SET value=$1,quality=$2,sequence=$3 WHERE project_id=$4 AND id=$5`, next.Value().String(), next.Quality().String(), next.Sequence(), projectID, tagID.UUID()); err != nil {
		return tag.State{}, err
	}
	if err := tx.Commit(); err != nil {
		return tag.State{}, err
	}
	if s.publish != nil {
		var rawName, rawOrigin string
		if err := s.db.QueryRowContext(ctx, `SELECT name,origin FROM runtime.tags WHERE project_id=$1 AND id=$2`, projectID, tagID.UUID()).Scan(&rawName, &rawOrigin); err == nil {
			name, _ := tag.NewTagName(rawName)
			origin, _ := entityorigin.Parse(rawOrigin)
			s.publish(tag.NewStateUpdatedEvent(name, next, origin, event.WithProjectID(projectID.String())))
		}
	}
	return next, nil
}

func (s *Store) RemoveTag(ctx context.Context, projectID uuid.UUID, tagID id.ID[tag.Tag]) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM runtime.tags WHERE project_id=$1 AND id=$2`, projectID, tagID.UUID())
	return err
}
func (s *Store) RemoveProject(ctx context.Context, projectID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM runtime.tags WHERE project_id=$1`, projectID)
	return err
}
func scanState(row interface{ Scan(...any) error }, tagID id.ID[tag.Tag]) (tag.State, error) {
	var kind, value, quality string
	var sequence int64
	if err := row.Scan(&kind, &value, &quality, &sequence); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return tag.State{}, tag.ErrTagNotFound
		}
		return tag.State{}, err
	}
	t, err := tag.NewTagType(kind)
	if err != nil {
		return tag.State{}, err
	}
	q, err := tag.NewTagQuality(quality)
	if err != nil {
		return tag.State{}, err
	}
	return tag.ReconstituteState(tagID, t, value, q, sequence)
}
