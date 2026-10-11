package growctl_test

import (
	"context"
	"strings"
	"testing"

	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/apiclient"
	"github.com/kipitix/growscada/internal/server/servertest"
)

func TestProjectContext_ManifestAndFlagIsolateApplyAndPrune(t *testing.T) {
	ctx := context.Background()
	srv, _ := servertest.StartAPI(t, postgres.DB(t))
	api := apiclient.New(srv.URL, srv.Client())
	a, err := api.CreateProject(ctx, apiv0.CreateProjectRequest{Name: "growctl A"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := api.CreateProject(ctx, apiv0.CreateProjectRequest{Name: "growctl B"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = api.DeleteProject(ctx, a.ID); _, _ = api.DeleteProject(ctx, b.ID) })
	if _, err := runGrowctl(t, srv, twoTags, "apply", "--project", a.ID.String()); err != nil {
		t.Fatal(err)
	}
	manifest := strings.ReplaceAll(twoTags, "apiVersion: growscada/v0.1", "apiVersion: growscada/v0.2")
	manifest = strings.ReplaceAll(manifest, "metadata:\n", "metadata:\n  project: "+b.ID.String()+"\n")
	if _, err := runGrowctl(t, srv, manifest, "apply"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGrowctl(t, srv, manifest, "apply", "--project", a.ID.String()); err == nil {
		t.Fatal("mismatched flag/manifest accepted")
	}
	if _, _, err := execGrowctl(t, srv, "", "delete", "--pattern", "*", "--yes", "--project", a.ID.String()); err != nil {
		t.Fatal(err)
	}
	if tags, err := api.ForProject(a.ID).ListTags(ctx); err != nil || len(tags) != 0 {
		t.Fatalf("project A not deleted: %v %v", tags, err)
	}
	if tags, err := api.ForProject(b.ID).ListTags(ctx); err != nil || len(tags) != 2 {
		t.Fatalf("project B was affected: %v %v", tags, err)
	}
}
