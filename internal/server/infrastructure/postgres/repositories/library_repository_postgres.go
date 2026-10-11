package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/library"
	"github.com/kipitix/growscada/internal/server/domain/project"
	"github.com/kipitix/growscada/internal/server/domain/version"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/outbox"
)

type libraryRepositoryPostgres struct {
	db             *sql.DB
	organizationID id.ID[project.Organization]
	onCommit       func()
}

func NewLibraryRepositoryPostgres(db *sql.DB, organizationID id.ID[project.Organization], opts ...Option) library.Repository {
	o := newOptions(opts)
	return &libraryRepositoryPostgres{db: db, organizationID: organizationID, onCommit: o.onCommit}
}

func (r *libraryRepositoryPostgres) NextID() id.ID[library.Library] {
	return id.NewID[library.Library]()
}

func (r *libraryRepositoryPostgres) FindAll(ctx context.Context, orgID id.ID[project.Organization]) ([]library.Library, error) {
	if orgID != r.organizationID {
		return nil, project.ErrNotFound
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,organization_id,name,is_public,version FROM libraries WHERE is_public OR organization_id=$1 ORDER BY is_public DESC,name,id`, orgID.UUID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []library.Library{}
	for rows.Next() {
		item, err := r.scanLibrary(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *libraryRepositoryPostgres) FindByID(ctx context.Context, rawID id.ID[library.Library]) (library.Library, error) {
	return r.scanLibrary(r.db.QueryRowContext(ctx, `SELECT id,organization_id,name,is_public,version FROM libraries WHERE id=$1 AND (is_public OR organization_id=$2)`, rawID.UUID(), r.organizationID.UUID()))
}

func (r *libraryRepositoryPostgres) scanLibrary(row interface{ Scan(...any) error }) (library.Library, error) {
	var rawID, rawOrg uuid.UUID
	var name string
	var isPublic bool
	var number int
	if err := row.Scan(&rawID, &rawOrg, &name, &isPublic, &number); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Library{}, library.ErrLibraryNotFound
		}
		return library.Library{}, err
	}
	n, err := library.NewLibraryName(name)
	if err != nil {
		return library.Library{}, err
	}
	v, err := version.New(version.WithNumber[library.Library](number))
	if err != nil {
		return library.Library{}, err
	}
	return library.ReconstituteLibrary(id.NewID(id.IDWithUUID[library.Library](rawID)), id.NewID(id.IDWithUUID[project.Organization](rawOrg)), n, isPublic, v), nil
}

func (r *libraryRepositoryPostgres) Save(ctx context.Context, l library.Library) (library.Library, error) {
	if l.OrganizationID() != r.organizationID {
		return library.Library{}, library.ErrLibraryNotFound
	}
	store := aggregateStore[library.Library]{db: r.db, table: "libraries", notFound: library.ErrLibraryNotFound, conflict: library.ErrLibraryConflict, onCommit: r.onCommit, scopeColumn: "organization_id", scopeID: r.organizationID.UUID()}
	v, err := store.save(ctx, l.ID(), l.Version(), l.PendingEvents(), rowWrite[library.Library]{
		insert: func(tx *sql.Tx, next version.Version[library.Library]) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO libraries(id,organization_id,name,is_public,version) VALUES($1,$2,$3,$4,$5)`, l.ID().UUID(), r.organizationID.UUID(), l.Name().String(), l.IsPublic(), next.Number())
			return libraryWriteError(err)
		},
		update: func(tx *sql.Tx, current, next version.Version[library.Library]) (bool, error) {
			res, err := tx.ExecContext(ctx, `UPDATE libraries SET name=$1,is_public=$2,version=$3 WHERE id=$4 AND organization_id=$5 AND version=$6`, l.Name().String(), l.IsPublic(), next.Number(), l.ID().UUID(), r.organizationID.UUID(), current.Number())
			if err != nil {
				return false, libraryWriteError(err)
			}
			n, err := res.RowsAffected()
			return n == 1, err
		},
	})
	if err != nil {
		return library.Library{}, err
	}
	return library.ReconstituteLibrary(l.ID(), l.OrganizationID(), l.Name(), l.IsPublic(), v), nil
}

func libraryWriteError(err error) error {
	if err == nil {
		return nil
	}
	if isUniqueViolation(err, "libraries_organization_id_name_key") {
		return library.ErrLibraryNameTaken
	}
	return fmt.Errorf("cannot save library: %w", err)
}

func (r *libraryRepositoryPostgres) CreateRelease(ctx context.Context, libraryID id.ID[library.Library], content []byte) (library.Release, error) {
	if err := library.ValidateReleaseContent(content); err != nil {
		return library.Release{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return library.Release{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var orgID uuid.UUID
	if err = tx.QueryRowContext(ctx, `SELECT organization_id FROM libraries WHERE id=$1 FOR UPDATE`, libraryID.UUID()).Scan(&orgID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Release{}, library.ErrLibraryNotFound
		}
		return library.Release{}, err
	}
	if orgID != r.organizationID.UUID() {
		return library.Release{}, library.ErrLibraryNotFound
	}
	var number int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(number),0)+1 FROM library_releases WHERE library_id=$1`, libraryID.UUID()).Scan(&number); err != nil {
		return library.Release{}, err
	}
	releaseID := uuid.New()
	var createdAt time.Time
	if err = tx.QueryRowContext(ctx, `INSERT INTO library_releases(id,library_id,number,content) VALUES($1,$2,$3,$4::jsonb) RETURNING created_at`, releaseID, libraryID.UUID(), number, string(content)).Scan(&createdAt); err != nil {
		return library.Release{}, err
	}
	e := library.NewLibraryReleasedEvent(libraryID, releaseID, number)
	if err = outbox.AppendProject(ctx, tx, []event.Event{e}, nil); err != nil {
		return library.Release{}, err
	}
	if err = tx.Commit(); err != nil {
		return library.Release{}, fmt.Errorf("cannot commit library release: %w", err)
	}
	if r.onCommit != nil {
		r.onCommit()
	}
	return library.ReconstituteRelease(releaseID, libraryID, number, event.NewEventTimestamp(event.EventTimestampWithTime(createdAt)), content), nil
}

func (r *libraryRepositoryPostgres) FindReleases(ctx context.Context, libraryID id.ID[library.Library]) ([]library.Release, error) {
	if _, err := r.FindByID(ctx, libraryID); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,number,created_at,content FROM library_releases WHERE library_id=$1 ORDER BY number DESC`, libraryID.UUID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []library.Release{}
	for rows.Next() {
		var releaseID uuid.UUID
		var number int
		var createdAt time.Time
		var content []byte
		if err := rows.Scan(&releaseID, &number, &createdAt, &content); err != nil {
			return nil, err
		}
		result = append(result, library.ReconstituteRelease(releaseID, libraryID, number, event.NewEventTimestamp(event.EventTimestampWithTime(createdAt)), content))
	}
	return result, rows.Err()
}

func (r *libraryRepositoryPostgres) FindProjectPins(ctx context.Context, projectID id.ID[project.Project]) ([]library.ProjectPin, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT p.project_id,r.id,r.library_id,r.number,r.created_at,r.content FROM project_library_pins p JOIN projects pr ON pr.id=p.project_id JOIN library_releases r ON r.library_id=p.library_id AND r.id=p.release_id WHERE p.project_id=$1 AND pr.organization_id=$2 ORDER BY r.library_id`, projectID.UUID(), r.organizationID.UUID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []library.ProjectPin{}
	for rows.Next() {
		var pid, releaseID, libID uuid.UUID
		var number int
		var createdAt time.Time
		var content []byte
		if err := rows.Scan(&pid, &releaseID, &libID, &number, &createdAt, &content); err != nil {
			return nil, err
		}
		result = append(result, library.ProjectPin{ProjectID: id.NewID(id.IDWithUUID[project.Project](pid)), Release: library.ReconstituteRelease(releaseID, id.NewID(id.IDWithUUID[library.Library](libID)), number, event.NewEventTimestamp(event.EventTimestampWithTime(createdAt)), content)})
	}
	return result, rows.Err()
}

func (r *libraryRepositoryPostgres) PinRelease(ctx context.Context, projectID id.ID[project.Project], orgID id.ID[project.Organization], libraryID id.ID[library.Library], releaseID uuid.UUID) error {
	if orgID != r.organizationID {
		return project.ErrNotFound
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `INSERT INTO project_library_pins(project_id,library_id,release_id) SELECT p.id,l.id,$3 FROM projects p JOIN libraries l ON l.id=$2 JOIN library_releases r ON r.library_id=l.id AND r.id=$3 WHERE p.id=$1 AND p.organization_id=$4 AND (l.is_public OR l.organization_id=$4) ON CONFLICT(project_id,library_id) DO UPDATE SET release_id=EXCLUDED.release_id,pinned_at=now()`, projectID.UUID(), libraryID.UUID(), releaseID, orgID.UUID())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return library.ErrReleaseNotFound
	}
	projectUUID := projectID.UUID()
	e := project.NewEvent(event.EventTypeProjectLibraryPinsChanged, projectID, event.WithProjectID(projectUUID.String()))
	if err = outbox.AppendProject(ctx, tx, []event.Event{e}, &projectUUID); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if r.onCommit != nil {
		r.onCommit()
	}
	return nil
}

func (r *libraryRepositoryPostgres) UnpinLibrary(ctx context.Context, projectID id.ID[project.Project], libraryID id.ID[library.Library]) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `DELETE FROM project_library_pins p USING projects pr WHERE p.project_id=$1 AND p.library_id=$2 AND pr.id=p.project_id AND pr.organization_id=$3`, projectID.UUID(), libraryID.UUID(), r.organizationID.UUID())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return library.ErrProjectPinNotFound
	}
	projectUUID := projectID.UUID()
	e := project.NewEvent(event.EventTypeProjectLibraryPinsChanged, projectID, event.WithProjectID(projectUUID.String()))
	if err = outbox.AppendProject(ctx, tx, []event.Event{e}, &projectUUID); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if r.onCommit != nil {
		r.onCommit()
	}
	return nil
}
