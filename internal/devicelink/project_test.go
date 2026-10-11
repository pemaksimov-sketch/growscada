package devicelink_test

import (
	"context"
	"errors"
	"testing"

	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/apiclient"
	"github.com/kipitix/growscada/internal/devicelink"
	"github.com/kipitix/growscada/internal/server/servertest"
)

func TestProjectLink_ResolvesAndWritesOnlyConfiguredProject(t *testing.T) {
	ctx := context.Background()
	srv, _ := servertest.StartAPI(t, postgres.DB(t))
	api := apiclient.New(srv.URL, srv.Client())
	a, err := api.CreateProject(ctx, apiv0.CreateProjectRequest{Name: "device A"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := api.CreateProject(ctx, apiv0.CreateProjectRequest{Name: "device B"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = api.DeleteProject(ctx, a.ID); _, _ = api.DeleteProject(ctx, b.ID) })
	ac, bc := api.ForProject(a.ID), api.ForProject(b.ID)
	input := apiv0.CreateTagRequest{Name: "counter", Type: "integer", Value: "0", Quality: "good"}
	ta, err := ac.CreateTagResult(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	tb, err := bc.CreateTagResult(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	link := devicelink.NewRESTLink(ac)
	resolved, err := link.Resolve(ctx, "counter")
	if err != nil || resolved.ID != ta.ID {
		t.Fatalf("wrong project resolution: %+v %v", resolved, err)
	}
	if err := link.Write(ctx, resolved, "9007199254740993", devicelink.QualityGood); err != nil {
		t.Fatal(err)
	}
	storedA, _ := ac.GetTag(ctx, ta.ID)
	storedB, _ := bc.GetTag(ctx, tb.ID)
	if storedA.Value != "9007199254740993" || storedB.Value != "0" {
		t.Fatal("project write isolation lost")
	}
	forged := resolved
	forged.ID = tb.ID
	if err := link.Write(ctx, forged, "42", devicelink.QualityGood); !errors.Is(err, devicelink.ErrTagNotFound) {
		t.Fatalf("foreign tag write accepted: %v", err)
	}
}
