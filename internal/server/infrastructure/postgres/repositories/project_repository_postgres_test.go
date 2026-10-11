package repositories_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/project"
	"github.com/kipitix/growscada/internal/server/domain/scene"
	"github.com/kipitix/growscada/internal/server/domain/tag"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/outbox"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/repositories"
)

func saveProject(t *testing.T, repo project.Repository) project.Project {
	t.Helper()
	name, _ := project.NewName(uuid.NewString())
	p, err := repo.Save(context.Background(), project.Create(repo.NextID(), project.DefaultOrganizationID(), name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = testDB.Exec(`DELETE FROM projects WHERE id=$1`, p.ID().UUID()) })
	return p
}

func TestProjectRepository_MetadataCASAndOrganizationIsolation(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewProjectRepositoryPostgres(testDB, project.DefaultOrganizationID())
	p := saveProject(t, repo)
	name, _ := project.NewName("Renamed " + uuid.NewString())
	rename, _ := p.Rename(p.Version(), name)
	stored, err := repo.Save(ctx, rename)
	if err != nil || stored.Version().Number() != 2 || stored.Name() != name {
		t.Fatalf("rename: %+v %v", stored, err)
	}
	if _, err := repo.Save(ctx, rename); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("stale rename: %v", err)
	}
	if err := repo.Delete(ctx, p.Delete()); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("stale delete: %v", err)
	}
	otherOrg := id.NewID[project.Organization]()
	foreignRepo := repositories.NewProjectRepositoryPostgres(testDB, otherOrg)
	if _, err := foreignRepo.FindByID(ctx, p.ID()); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign project leaked: %v", err)
	}
	if err := foreignRepo.Delete(ctx, stored.Delete()); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign delete: %v", err)
	}
	if _, err := foreignRepo.Save(ctx, stored); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign save: %v", err)
	}
	if err := repo.Delete(ctx, stored.Delete()); err != nil {
		t.Fatal(err)
	}
}

func TestProjectRepositories_IsolateReadsWritesAndNaturalKeys(t *testing.T) {
	ctx := context.Background()
	projects := repositories.NewProjectRepositoryPostgres(testDB, project.DefaultOrganizationID())
	a, b := saveProject(t, projects), saveProject(t, projects)
	ar, br := repositories.ForProject(testDB, a.ID()), repositories.ForProject(testDB, b.ID())
	ta := mustSaveTag(t, ar.Tags(), createTag(t, ar.Tags(), "same.name"))
	tb := mustSaveTag(t, br.Tags(), createTag(t, br.Tags(), "same.name"))
	if ta.ID() == tb.ID() {
		t.Fatal("identical natural keys should have independent IDs")
	}
	if _, err := ar.Tags().Save(ctx, createTag(t, ar.Tags(), "same.name")); !errors.Is(err, tag.ErrTagNameTaken) {
		t.Fatalf("duplicate within project: %v", err)
	}
	if _, err := br.Tags().FindByID(ctx, ta.ID()); !errors.Is(err, tag.ErrTagNotFound) {
		t.Fatalf("foreign tag read: %v", err)
	}
	if _, err := br.Tags().Save(ctx, ta); !errors.Is(err, tag.ErrTagNotFound) {
		t.Fatalf("foreign tag update: %v", err)
	}
	ta.Delete()
	if err := br.Tags().Delete(ctx, ta); !errors.Is(err, tag.ErrTagNotFound) {
		t.Fatalf("foreign tag delete: %v", err)
	}
	if found, err := br.Tags().FindByName(ctx, tb.Name()); err != nil || found.ID() != tb.ID() {
		t.Fatalf("name lookup crossed projects: %v", err)
	}
	for _, repo := range []tag.TagRepository{ar.Tags(), br.Tags()} {
		found, err := repo.FindAll(ctx)
		if err != nil || len(found) != 1 {
			t.Fatalf("tag list leaked: %v %v", found, err)
		}
	}
	sa, sb := mustSaveScene(t, ar.Scenes(), "same scene"), mustSaveScene(t, br.Scenes(), "same scene")
	if _, err := br.Scenes().FindByID(ctx, sa.ID()); !errors.Is(err, scene.ErrSceneNotFound) {
		t.Fatalf("foreign scene read: %v", err)
	}
	if _, err := br.Scenes().Save(ctx, sa); !errors.Is(err, scene.ErrSceneNotFound) {
		t.Fatalf("foreign scene save: %v", err)
	}
	if err := br.Scenes().Delete(ctx, sa.Delete()); !errors.Is(err, scene.ErrSceneNotFound) {
		t.Fatalf("foreign scene delete: %v", err)
	}
	if found, err := br.Scenes().FindAll(ctx); err != nil || len(found) != 1 || found[0].ID() != sb.ID() {
		t.Fatalf("scene list leaked: %v", err)
	}
	// A project deletion cascades only through that project's resources.
	if err := projects.Delete(ctx, a.Delete()); err != nil {
		t.Fatal(err)
	}
	if _, err := ar.Tags().FindByID(ctx, ta.ID()); !errors.Is(err, tag.ErrTagNotFound) {
		t.Fatal("deleted project retained tag")
	}
	if _, err := br.Tags().FindByID(ctx, tb.ID()); err != nil {
		t.Fatal("other project lost tag")
	}
	if _, err := br.Scenes().FindByID(ctx, sb.ID()); err != nil {
		t.Fatal("other project lost scene")
	}
}

func TestProjectOutbox_PreservesScopeThroughDelivery(t *testing.T) {
	ctx := context.Background()
	cleanOutbox(t)
	projects := repositories.NewProjectRepositoryPostgres(testDB, project.DefaultOrganizationID())
	p := saveProject(t, projects)
	scope := repositories.ForProject(testDB, p.ID())
	tg := mustSaveTag(t, scope.Tags(), createTag(t, scope.Tags(), "scoped"))
	if err := tg.SetValue(int64(42), tag.TagQualityGood); err != nil {
		t.Fatal(err)
	}
	mustSaveTag(t, scope.Tags(), tg)
	var got []event.Event
	d := outbox.NewDispatcher(testDB, func(e event.Event) { got = append(got, e) })
	if _, err := d.DrainOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var scopedTags int
	for _, e := range got {
		if _, ok := e.(tag.TagEvent); !ok {
			continue
		}
		if e.(interface{ ScopeProjectID() string }).ScopeProjectID() != p.ID().String() {
			t.Fatalf("lost outbox scope: %+v", e)
		}
		scopedTags++
	}
	if scopedTags != 2 {
		t.Fatalf("got %d tag events", scopedTags)
	}
}
