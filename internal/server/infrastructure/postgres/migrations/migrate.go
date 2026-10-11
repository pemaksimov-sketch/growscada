// Package migrations applies the legacy bootstrap and then independent node
// chains. The only transfer of old process values goes through RuntimePort.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/domain/entityorigin"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/tag"
	"github.com/kipitix/growscada/internal/server/domain/version"
	runtimestore "github.com/kipitix/growscada/internal/server/infrastructure/postgres/runtime"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

//go:embed *.sql engineering/*.sql runtime/*.sql
var files embed.FS

// WithEngineeringSearchPath prepares an application connection after the
// bootstrap tables have moved into engineering. The migration runner itself
// always uses explicit public/engineering version tables.
func WithEngineeringSearchPath(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		if err != nil {
			return "", fmt.Errorf("parse engineering database DSN: %w", err)
		}
		return "", errors.New("engineering database DSN must be a URL")
	}
	query := u.Query()
	query.Set("options", "-csearch_path=engineering,public")
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func Apply(ctx context.Context, engineeringDB, runtimeDB *sql.DB) error {
	if err := applyChain(ctx, engineeringDB, files, "public.goose_db_version"); err != nil {
		return fmt.Errorf("legacy migrations: %w", err)
	}
	if _, err := runtimeDB.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS runtime`); err != nil {
		return err
	}
	runtimeFiles, _ := fs.Sub(files, "runtime")
	if err := applyChain(ctx, runtimeDB, runtimeFiles, "runtime.goose_db_version"); err != nil {
		return fmt.Errorf("runtime migrations: %w", err)
	}
	// An interrupted copy is safe to repeat: RegisterTag keeps existing values.
	var legacyExists bool
	if err := engineeringDB.QueryRowContext(ctx, `SELECT to_regclass('engineering.tags') IS NOT NULL`).Scan(&legacyExists); err != nil {
		return err
	}
	if legacyExists {
		if err := copyProcessState(ctx, engineeringDB, runtimestore.New(runtimeDB)); err != nil {
			return fmt.Errorf("process state migration: %w", err)
		}
	}
	engineeringFiles, _ := fs.Sub(files, "engineering")
	if err := applyChain(ctx, engineeringDB, engineeringFiles, "engineering.goose_db_version"); err != nil {
		return fmt.Errorf("engineering migrations: %w", err)
	}
	return nil
}
func applyChain(ctx context.Context, db *sql.DB, files fs.FS, table string) error {
	store, err := database.NewStore(database.DialectPostgres, table)
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider("", db, files, goose.WithStore(store))
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}
func copyProcessState(ctx context.Context, db *sql.DB, target *runtimestore.Store) error {
	rows, err := db.QueryContext(ctx, `SELECT id,project_id,name,type,value,quality,version FROM engineering.tags ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rawID, projectID uuid.UUID
		var rawName, kind, value, quality string
		var sequence int64
		if err := rows.Scan(&rawID, &projectID, &rawName, &kind, &value, &quality, &sequence); err != nil {
			return err
		}
		name, err := tag.NewTagName(rawName)
		if err != nil {
			return err
		}
		t, err := tag.NewTagType(kind)
		if err != nil {
			return err
		}
		q, err := tag.NewTagQuality(quality)
		if err != nil {
			return err
		}
		tagID := id.NewID(id.IDWithUUID[tag.Tag](rawID))
		d, err := tag.ReconstituteDefinition(tagID, name, t, entityorigin.Project, version.Committed[tag.Definition]())
		if err != nil {
			return err
		}
		s, err := tag.ReconstituteState(tagID, t, value, q, sequence)
		if err != nil {
			return err
		}
		if err := target.RegisterTag(ctx, projectID, d, s); err != nil {
			return err
		}
	}
	return rows.Err()
}
