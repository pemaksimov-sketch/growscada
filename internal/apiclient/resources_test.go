package apiclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/kipitix/growscada/contract"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/apiclient"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResourceClientRoutesAndVersions(t *testing.T) {
	ctx := context.Background()
	sid, id := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name, method, path, response string
		status                       int
		call                         func(*apiclient.Client) (any, error)
	}{
		{"list scenes", "GET", "/scenes", `{"scenes":[]}`, 200, func(c *apiclient.Client) (any, error) { return c.ListScenes(ctx) }},
		{"get scene", "GET", "/scenes/" + sid.String(), `{"name":"screen","version":3}`, 200, func(c *apiclient.Client) (any, error) { return c.GetScene(ctx, sid) }},
		{"create scene", "POST", "/scenes", `{"id":"` + sid.String() + `"}`, 201, func(c *apiclient.Client) (any, error) {
			return c.CreateScene(ctx, apiv0.CreateSceneRequest{Name: "screen", Width: 100, Height: 100})
		}},
		{"update scene", "PUT", "/scenes/" + sid.String(), `{"version":4}`, 200, func(c *apiclient.Client) (any, error) {
			return c.UpdateScene(ctx, sid, apiv0.UpdateSceneRequest{Name: "screen", Width: 100, Height: 100, Version: 3})
		}},
		{"delete scene", "DELETE", "/scenes/" + sid.String(), `{"name":"screen","version":4}`, 200, func(c *apiclient.Client) (any, error) { return c.DeleteScene(ctx, sid) }},
		{"list widgets", "GET", "/scenes/" + sid.String() + "/widgets", `{"widgets":[]}`, 200, func(c *apiclient.Client) (any, error) { return c.ListWidgets(ctx, sid) }},
		{"get widget", "GET", "/scenes/" + sid.String() + "/widgets/" + id.String(), `{"name":"gauge","scene_version":3}`, 200, func(c *apiclient.Client) (any, error) { return c.GetWidget(ctx, sid, id) }},
		{"create widget", "POST", "/scenes/" + sid.String() + "/widgets", `{"id":"` + id.String() + `","scene_version":4}`, 201, func(c *apiclient.Client) (any, error) {
			return c.CreateWidget(ctx, sid, apiv0.CreateWidgetRequest{Name: "gauge", SceneVersion: 3})
		}},
		{"update widget", "PUT", "/scenes/" + sid.String() + "/widgets/" + id.String(), `{"scene_version":4}`, 200, func(c *apiclient.Client) (any, error) {
			return c.UpdateWidget(ctx, sid, id, apiv0.UpdateWidgetRequest{Name: "gauge", SceneVersion: 3})
		}},
		{"delete widget", "DELETE", "/scenes/" + sid.String() + "/widgets/" + id.String(), `{"name":"gauge","scene_version":4}`, 200, func(c *apiclient.Client) (any, error) { return c.DeleteWidget(ctx, sid, id) }},
		{"list types", "GET", "/widget-types", `{"widget_types":[]}`, 200, func(c *apiclient.Client) (any, error) { return c.ListWidgetTypes(ctx) }},
		{"get type", "GET", "/widget-types/" + id.String(), `{"name":"gauge","version":3}`, 200, func(c *apiclient.Client) (any, error) { return c.GetWidgetType(ctx, id) }},
		{"create type", "POST", "/widget-types", `{"id":"` + id.String() + `"}`, 201, func(c *apiclient.Client) (any, error) {
			return c.CreateWidgetType(ctx, apiv0.CreateWidgetTypeRequest{Name: "gauge"})
		}},
		{"update type", "PUT", "/widget-types/" + id.String(), `{"version":4}`, 200, func(c *apiclient.Client) (any, error) {
			return c.UpdateWidgetType(ctx, id, apiv0.UpdateWidgetTypeRequest{Name: "gauge", Version: 3})
		}},
		{"delete type", "DELETE", "/widget-types/" + id.String(), `{"name":"gauge","version":4}`, 200, func(c *apiclient.Client) (any, error) { return c.DeleteWidgetType(ctx, id) }},
		{"create tag result", "POST", "/tags", `{"id":"` + id.String() + `"}`, 201, func(c *apiclient.Client) (any, error) {
			return c.CreateTagResult(ctx, apiv0.CreateTagRequest{Name: "tag"})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method || r.URL.Path != apiv0.PathPrefix+tc.path {
					t.Errorf("wrong route: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get(contract.SchemaVersionHeader) != apiv0.SchemaVersion().String() {
					t.Error("schema header lost")
				}
				if r.Method == "POST" || r.Method == "PUT" {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if r.Method == "PUT" && body["version"] != float64(3) && body["scene_version"] != float64(3) {
						t.Errorf("expected version not sent: %v", body)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.response))
			}))
			defer srv.Close()
			if _, err := tc.call(apiclient.New(srv.URL, srv.Client())); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestResourceClientPreservesProblemDetailsAndContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		w.Write([]byte(`{"detail":"scene changed"}`))
	}))
	defer srv.Close()
	client := apiclient.New(srv.URL, srv.Client())
	if _, err := client.UpdateScene(context.Background(), uuid.New(), apiv0.UpdateSceneRequest{}); !errors.Is(err, apiclient.ErrConflict) {
		t.Fatalf("conflict lost: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.ListWidgetTypes(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
