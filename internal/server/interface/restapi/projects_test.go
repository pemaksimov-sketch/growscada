package restapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/apiclient"
	"github.com/kipitix/growscada/internal/server/application"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/project"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/repositories"
	"github.com/kipitix/growscada/internal/server/interface/eventbus"
	"github.com/kipitix/growscada/internal/server/interface/restapi"
)

func TestProjectAPI_CRUDAndIsolation(t *testing.T) {
	ctx := context.Background()
	projects := repositories.NewProjectRepositoryPostgres(testDB, project.DefaultOrganizationID())
	projectSvc := application.NewProjectService(projects, project.DefaultOrganizationID())
	main := repositories.ForProject(testDB, project.MainID())
	types := repositories.NewWidgetTypeRepositoryPostgres(testDB)
	var removedRuntimeProjects []uuid.UUID
	router := restapi.NewRouter(application.NewTagService(main.Tags()), application.NewWidgetTypeService(types, main.Scenes(), main.Tags()), application.NewSceneService(main.Scenes(), types, main.Tags()), eventbus.NewEventBus(), 10,
		restapi.WithProjects(projectSvc, func(projectID uuid.UUID) restapi.ProjectServices {
			scope := repositories.ForProject(testDB, id.NewID(id.IDWithUUID[project.Project](projectID)))
			return restapi.ProjectServices{Tags: application.NewTagService(scope.Tags()), Scenes: application.NewSceneService(scope.Scenes(), types, scope.Tags())}
		}, func(_ context.Context, projectID uuid.UUID) error {
			removedRuntimeProjects = append(removedRuntimeProjects, projectID)
			return nil
		}))
	defer router.Close()
	srv := httptest.NewServer(router.ServeMux())
	defer srv.Close()
	api := apiclient.New(srv.URL, srv.Client())
	a, err := api.CreateProject(ctx, apiv0.CreateProjectRequest{Name: "A " + uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	b, err := api.CreateProject(ctx, apiv0.CreateProjectRequest{Name: "B " + uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = testDB.Exec(`DELETE FROM projects WHERE id IN($1,$2)`, a.ID, b.ID) })
	ac, bc := api.ForProject(a.ID), api.ForProject(b.ID)
	input := apiv0.CreateTagRequest{Name: "shared.name", Type: "integer", Value: "0", Quality: "good"}
	tagA, err := ac.CreateTagResult(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bc.CreateTagResult(ctx, input); err != nil {
		t.Fatal(err)
	}
	if list, err := ac.ListTagsFiltered(ctx, apiv0.TagsQueryName, "absent"); err != nil || len(list) != 0 {
		t.Fatalf("empty scoped filter: %v %v", list, err)
	}
	if _, err := bc.GetTag(ctx, tagA.ID); !errors.Is(err, apiclient.ErrNotFound) {
		t.Fatalf("foreign tag read: %v", err)
	}
	if err := bc.DeleteTag(ctx, tagA.ID); !errors.Is(err, apiclient.ErrNotFound) {
		t.Fatalf("foreign tag delete: %v", err)
	}
	sc, err := ac.CreateScene(ctx, apiv0.CreateSceneRequest{Name: "isolated", Width: 800, Height: 600})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bc.GetScene(ctx, sc.ID); !errors.Is(err, apiclient.ErrNotFound) {
		t.Fatalf("foreign scene read: %v", err)
	}
	if _, err := api.UpdateProject(ctx, a.ID, apiv0.UpdateProjectRequest{Name: "Renamed " + uuid.NewString(), Version: a.Version}); err != nil {
		t.Fatal(err)
	}
	if _, err := api.UpdateProject(ctx, a.ID, apiv0.UpdateProjectRequest{Name: "stale", Version: a.Version}); !errors.Is(err, apiclient.ErrConflict) {
		t.Fatalf("stale project rename: %v", err)
	}
	if _, err := api.DeleteProject(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if len(removedRuntimeProjects) != 1 || removedRuntimeProjects[0] != a.ID {
		t.Fatalf("project deletion did not clean its Runtime state: %v", removedRuntimeProjects)
	}
	if _, err := ac.ListTags(ctx); !errors.Is(err, apiclient.ErrNotFound) {
		t.Fatalf("deleted parent read: %v", err)
	}
	if list, err := bc.ListTags(ctx); err != nil || len(list) != 1 {
		t.Fatalf("other project lost tags: %v %v", list, err)
	}
	if _, err := api.DeleteProject(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	response, err := srv.Client().Get(srv.URL + apiv0.PathPrefix + "/projects/not-a-uuid/tags")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatal("invalid project UUID accepted")
	}
}
