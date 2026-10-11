package growctl

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestProjectManifest_SelectsScopeAndRejectsMixedProjects(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	client := NewClient("http://localhost:9090", http.DefaultClient)
	if err := selectManifestProject(client, []TagManifest{{ProjectID: a.String()}}); err != nil || client.ProjectID() != a {
		t.Fatalf("manifest scope: %v", err)
	}
	if err := selectManifestProject(client, []TagManifest{{ProjectID: b.String()}}); err == nil {
		t.Fatal("overrode CLI/context scope")
	}
	if client.ProjectID() != a {
		t.Fatal("rejected manifest changed scope")
	}
	client = NewClient("http://localhost:9090", http.DefaultClient)
	if err := selectManifestProject(client, []TagManifest{{ProjectID: a.String()}, {ProjectID: b.String()}}); err == nil {
		t.Fatal("mixed project documents accepted")
	}
	if client.ProjectID() != uuid.Nil {
		t.Fatal("partially selected a rejected manifest")
	}
}

func TestProjectManifest_ParsesProjectUUID(t *testing.T) {
	projectID := uuid.NewString()
	document := "apiVersion: growscada/v0.2\nkind: Tag\nmetadata:\n  project: PROJECT\n  name: counter\nspec: {type: integer, initialValue: 0, initialQuality: good}\n"
	items, err := ParseManifests([]byte(strings.ReplaceAll(document, "PROJECT", projectID)))
	if err != nil || len(items) != 1 || items[0].ProjectID != projectID {
		t.Fatalf("project manifest: %v %v", items, err)
	}
	for _, invalid := range []string{"not-a-project", uuid.Nil.String()} {
		if _, err := ParseManifests([]byte(strings.ReplaceAll(document, "PROJECT", invalid))); err == nil {
			t.Fatalf("accepted %s", invalid)
		}
	}
}
