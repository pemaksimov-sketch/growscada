package repositories_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/library"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/migrations"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/repositories"
	"github.com/pressly/goose/v3"
)

// migrationDB creates an empty database of its own on the test server, so
// that a migration can be run against data written in an older schema.
func migrationDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	name := "migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := testDB.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	db, err := sql.Open("postgres", strings.Replace(testConnStr, "/testdb?", "/"+name+"?", 1))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		_, _ = testDB.Exec("DROP DATABASE IF EXISTS " + name)
	})
	_, currentFile, _, _ := runtime.Caller(0)
	return db, filepath.Join(filepath.Dir(currentFile), "..", "migrations")
}

func TestMigrationSplit_CopiesLegacyStateIntoSeparateRuntimeDatabase(t *testing.T) {
	ctx := context.Background()
	engineeringDB, dir := migrationDB(t)
	runtimeName := "runtime_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := testDB.Exec("CREATE DATABASE " + runtimeName); err != nil {
		t.Fatal(err)
	}
	runtimeDB, err := sql.Open("postgres", strings.Replace(testConnStr, "/testdb?", "/"+runtimeName+"?", 1))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { runtimeDB.Close(); _, _ = testDB.Exec("DROP DATABASE IF EXISTS " + runtimeName) })
	if err := goose.UpToContext(ctx, engineeringDB, dir, 20261011010000); err != nil {
		t.Fatal(err)
	}
	const mainProjectID = "00000000-0000-4000-8000-000000000001"
	tagID := uuid.New()
	legacyWidgetTypeID := uuid.New()
	if _, err := engineeringDB.ExecContext(ctx, `INSERT INTO tags(id,project_id,name,type,value,quality,version) VALUES($1,$2,'legacy.counter','integer','9007199254740993','good',7)`, tagID, mainProjectID); err != nil {
		t.Fatal(err)
	}
	if _, err := engineeringDB.ExecContext(ctx, `INSERT INTO engineering.widget_types(id,name,html_template,script,script_language,default_width,default_height,input_ports,version) VALUES($1,'legacy type','','','javascript',120,60,'[]'::jsonb,1)`, legacyWidgetTypeID); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, engineeringDB, runtimeDB); err != nil {
		t.Fatal(err)
	}
	var name, kind, origin string
	var definitionVersion int
	if err := engineeringDB.QueryRowContext(ctx, `SELECT name,type,origin,version FROM engineering.tag_definitions WHERE id=$1`, tagID).Scan(&name, &kind, &origin, &definitionVersion); err != nil {
		t.Fatal(err)
	}
	if name != "legacy.counter" || kind != "integer" || origin != "Project" || definitionVersion != 1 {
		t.Fatalf("definition not preserved and normalized: %s %s %s v%d", name, kind, origin, definitionVersion)
	}
	var releaseID, libraryID uuid.UUID
	var releaseNumber int
	var releasedTypeID uuid.UUID
	if err := engineeringDB.QueryRowContext(ctx, `SELECT r.id,r.library_id,r.number,(r.content->'widget_types'->0->>'id')::uuid FROM engineering.library_releases r WHERE r.number=1`).Scan(&releaseID, &libraryID, &releaseNumber, &releasedTypeID); err != nil {
		t.Fatal(err)
	}
	if releaseNumber != 1 || libraryID != uuid.MustParse("00000000-0000-4000-8000-000000000003") || releasedTypeID != legacyWidgetTypeID {
		t.Fatalf("seed library release does not snapshot the migrated type: release=%d library=%s type=%s", releaseNumber, libraryID, releasedTypeID)
	}
	pinnedTypes := repositories.NewProjectWidgetTypeRepository(engineeringDB, uuid.MustParse(mainProjectID))
	visibleTypes, err := pinnedTypes.FindAll(ctx)
	if err != nil || len(visibleTypes) != 1 || visibleTypes[0].ID().UUID() != legacyWidgetTypeID || visibleTypes[0].Name().String() != "legacy type" {
		t.Fatalf("Main cannot read its pinned Library Release contents: %+v %v", visibleTypes, err)
	}
	var pinID uuid.UUID
	if err := engineeringDB.QueryRowContext(ctx, `SELECT release_id FROM engineering.project_library_pins WHERE project_id=$1 AND library_id=$2`, mainProjectID, libraryID).Scan(&pinID); err != nil || pinID != releaseID {
		t.Fatalf("Main is not pinned to initial Library Release: %s %v", pinID, err)
	}
	if _, err := engineeringDB.ExecContext(ctx, `UPDATE engineering.library_releases SET content='{}'::jsonb WHERE id=$1`, releaseID); err == nil {
		t.Fatal("Library Release snapshot accepted an update")
	}
	draftTypes := repositories.NewWidgetTypeRepositoryPostgres(engineeringDB)
	draftType, err := draftTypes.FindByID(ctx, id.NewID(id.IDWithUUID[library.WidgetType](legacyWidgetTypeID)))
	if err != nil {
		t.Fatal(err)
	}
	if err := draftTypes.Delete(ctx, draftType.Delete()); err != nil {
		t.Fatalf("delete from Library Draft: %v", err)
	}
	if pinned, err := pinnedTypes.FindByID(ctx, id.NewID(id.IDWithUUID[library.WidgetType](legacyWidgetTypeID))); err != nil || pinned.Name().String() != "legacy type" {
		t.Fatalf("deleting a draft type damaged the pinned release: %+v %v", pinned, err)
	}
	var value, quality string
	var sequence int64
	if err := runtimeDB.QueryRowContext(ctx, `SELECT value,quality,sequence FROM runtime.tags WHERE id=$1`, tagID).Scan(&value, &quality, &sequence); err != nil {
		t.Fatal(err)
	}
	if value != "9007199254740993" || quality != "good" || sequence != 7 {
		t.Fatalf("runtime state changed: %s %s #%d", value, quality, sequence)
	}
	var hasValue, hasQuality bool
	if err := engineeringDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='engineering' AND table_name='tag_definitions' AND column_name='value'), EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='engineering' AND table_name='tag_definitions' AND column_name='quality')`).Scan(&hasValue, &hasQuality); err != nil || hasValue || hasQuality {
		t.Fatalf("runtime columns remain in Engineering: %v %v %v", hasValue, hasQuality, err)
	}
	if err := migrations.Apply(ctx, engineeringDB, runtimeDB); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	var stillValue string
	if err := runtimeDB.QueryRowContext(ctx, `SELECT value FROM runtime.tags WHERE id=$1`, tagID).Scan(&stillValue); err != nil || stillValue != value {
		t.Fatalf("repeat migration overwrote Runtime state: %s %v", stillValue, err)
	}
}

func TestMigrationProjects_PreservesLegacyDataAndScopesOutbox(t *testing.T) {
	ctx := context.Background()
	db, dir := migrationDB(t)
	if err := goose.UpToContext(ctx, db, dir, 20261011000000); err != nil {
		t.Fatal(err)
	}
	tagID, sceneID, widgetID, typeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	queries := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO tags(id,name,type,value,quality,version) VALUES($1,'legacy.counter','integer','123','good',4)`, []any{tagID}},
		{`INSERT INTO widget_types(id,name,html_template,script,script_language) VALUES($1,'legacy type','','','javascript')`, []any{typeID}},
		{`INSERT INTO scenes(id,name,width,height,background_html,version) VALUES($1,'legacy scene',800,600,'background',7)`, []any{sceneID}},
		{`INSERT INTO widgets(id,name,type_id,scene_id) VALUES($1,'legacy widget',$2,$3)`, []any{widgetID, typeID, sceneID}},
		{`INSERT INTO outbox(type,payload) VALUES('tag_created', jsonb_build_object('id',$1::text,'timestamp','2026-10-11T00:00:00Z'))`, []any{tagID}},
	}
	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := goose.UpToContext(ctx, db, dir, 20261011010000); err != nil {
		t.Fatal(err)
	}
	const mainID = "00000000-0000-4000-8000-000000000001"
	for _, table := range []string{"tags", "scenes", "widgets", "outbox"} {
		var scope string
		if err := db.QueryRowContext(ctx, "SELECT project_id FROM "+table+" LIMIT 1").Scan(&scope); err != nil || scope != mainID {
			t.Fatalf("%s lost scope: %s %v", table, scope, err)
		}
	}
	var value string
	var tagVersion, sceneVersion int
	if err := db.QueryRowContext(ctx, `SELECT value,version FROM tags WHERE id=$1`, tagID).Scan(&value, &tagVersion); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT version FROM scenes WHERE id=$1`, sceneID).Scan(&sceneVersion); err != nil {
		t.Fatal(err)
	}
	if value != "123" || tagVersion != 4 || sceneVersion != 7 {
		t.Fatal("migration changed values or aggregate versions")
	}
	var projectName string
	if err := db.QueryRowContext(ctx, `SELECT name FROM projects WHERE id=$1`, mainID).Scan(&projectName); err != nil || projectName != "Main" {
		t.Fatal("missing Main seed")
	}
}

func TestMigrationWidgetsTypeIDForeignKey_BumpsVersionOfScenesThatLoseWidgets(t *testing.T) {
	ctx := context.Background()
	db, dir := migrationDB(t)
	if err := goose.UpToContext(ctx, db, dir, 20260926000000); err != nil {
		t.Fatalf("migrate to the previous version: %v", err)
	}

	typeID, orphanScene, keptScene := uuid.New(), uuid.New(), uuid.New()
	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	mustExec(`INSERT INTO widget_types (id, name, html_template, script, script_language)
	          VALUES ($1, 'kept type', '', '', 'javascript')`, typeID)
	mustExec(`INSERT INTO scenes (id, name, width, height, background_html, version)
	          VALUES ($1, 'orphan scene', 100, 100, '', 3), ($2, 'kept scene', 100, 100, '', 7)`,
		orphanScene, keptScene)
	mustExec(`INSERT INTO widgets (id, name, type_id, scene_id)
	          VALUES ($1, 'orphan', $2, $3), ($4, 'kept', $5, $6)`,
		uuid.New(), uuid.New(), orphanScene, uuid.New(), typeID, keptScene)

	if err := goose.UpToContext(ctx, db, dir, 20261004000000); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	versions := map[uuid.UUID]int{}
	rows, err := db.QueryContext(ctx, `SELECT id, version FROM scenes`)
	if err != nil {
		t.Fatalf("select scenes: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var v int
		if err := rows.Scan(&id, &v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		versions[id] = v
	}
	if versions[orphanScene] != 4 {
		t.Errorf("scene that lost a widget: expected version 4, got %d", versions[orphanScene])
	}
	if versions[keptScene] != 7 {
		t.Errorf("scene that kept its widgets: expected version 7, got %d", versions[keptScene])
	}
}
