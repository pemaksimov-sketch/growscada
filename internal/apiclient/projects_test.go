package apiclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
)

func TestProjectClient_ScopesResourcesWithoutMutatingOriginal(t *testing.T) {
	projectID := uuid.New()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("name") != "" && r.URL.Query().Get("name") != "temperature" {
			t.Error("lost name filter")
		}
		if r.URL.Path == apiv0.PathPrefix+"/projects" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		_, _ = w.Write([]byte(`{"tags":[],"scenes":[],"widget_types":[],"projects":[]}`))
	}))
	defer srv.Close()
	base := New(srv.URL, srv.Client())
	scoped := base.ForProject(projectID)
	ctx := context.Background()
	if _, err := scoped.ListTagsFiltered(ctx, apiv0.TagsQueryName, "temperature"); err != nil {
		t.Fatal(err)
	}
	if _, err := scoped.ListScenes(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := scoped.ListWidgetTypes(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := scoped.ListProjects(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := scoped.CreateProject(ctx, apiv0.CreateProjectRequest{Name: "new"}); err != nil {
		t.Fatal(err)
	}
	if _, err := base.ListTags(ctx); err != nil {
		t.Fatal(err)
	}
	prefix := apiv0.PathPrefix + "/projects/" + projectID.String()
	if paths[0] != prefix+"/tags?name=temperature" || paths[1] != prefix+"/scenes" {
		t.Fatalf("unscoped resources: %v", paths)
	}
	for _, path := range paths[2:5] {
		if strings.Contains(path, projectID.String()) {
			t.Fatalf("organization resource scoped to project: %s", path)
		}
	}
	if paths[5] != apiv0.PathPrefix+"/tags" || base.ProjectID() != uuid.Nil {
		t.Fatal("mutated base client")
	}
	if scoped.EventsURL() != srv.URL+prefix+"/events" {
		t.Fatal("unscoped SSE")
	}
}
