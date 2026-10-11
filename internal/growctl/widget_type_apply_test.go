package growctl

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	manifestv0 "github.com/kipitix/growscada/contract/manifest/v0"
)

func TestApplyWidgetTypeManifestsCreatesAndUpdatesByLibraryName(t *testing.T) {
	libraryID, widgetTypeID := uuid.New(), uuid.New()
	var existing *apiv0.WidgetTypeResponse
	var creates, updates int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		basePath := "/api/v0/libraries/" + libraryID.String() + "/widget-types"
		if r.URL.Path != basePath && r.URL.Path != basePath+"/"+widgetTypeID.String() {
			t.Errorf("path = %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			items := []apiv0.WidgetTypeResponse{}
			if existing != nil {
				items = append(items, *existing)
			}
			_ = json.NewEncoder(w).Encode(apiv0.GetWidgetTypesResponse{WidgetTypes: items})
		case http.MethodPost:
			creates++
			var request apiv0.CreateWidgetTypeRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Name != "Gauge" || request.InputPorts[0].Name != "value" {
				t.Errorf("create request = %#v", request)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(apiv0.CreateWidgetTypeResponse{ID: widgetTypeID})
		case http.MethodPut:
			updates++
			var request apiv0.UpdateWidgetTypeRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Version != existing.Version {
				t.Errorf("version = %d, want %d", request.Version, existing.Version)
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(apiv0.UpdateWidgetTypeResponse{Version: request.Version + 1})
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, server.Client())
	manifest := WidgetTypeManifest{LibraryID: libraryID, Name: "Gauge", HtmlTemplate: "<div/>", Script: "", ScriptLanguage: "javascript", DefaultWidth: 120, DefaultHeight: 60, InputPorts: []manifestv0.WidgetPort{{Name: "value", TypeHint: "number"}}}
	var out strings.Builder
	if err := applyWidgetTypeManifests(context.Background(), client, []WidgetTypeManifest{manifest}, &out); err != nil {
		t.Fatal(err)
	}
	if creates != 1 || updates != 0 || !strings.Contains(out.String(), "created widgettype/Gauge") {
		t.Fatalf("create result: counts %d/%d, output %q", creates, updates, out.String())
	}
	existing = &apiv0.WidgetTypeResponse{ID: widgetTypeID, Name: "Gauge", HtmlTemplate: "<div/>", Script: "", ScriptLanguage: "javascript", DefaultWidth: 120, DefaultHeight: 60, InputPorts: []apiv0.InputPort{{Name: "value", TypeHint: "number"}}, Version: 4}
	if err := applyWidgetTypeManifests(context.Background(), client, []WidgetTypeManifest{manifest}, &out); err != nil {
		t.Fatal(err)
	}
	if creates != 1 || updates != 0 || !strings.Contains(out.String(), "unchanged widgettype/Gauge") {
		t.Fatalf("unchanged result: counts %d/%d, output %q", creates, updates, out.String())
	}
	manifest.Script = "element.textContent = 'ready'"
	if err := applyWidgetTypeManifests(context.Background(), client, []WidgetTypeManifest{manifest}, &out); err != nil {
		t.Fatal(err)
	}
	if creates != 1 || updates != 1 || !strings.Contains(out.String(), "configured widgettype/Gauge") {
		t.Fatalf("update result: counts %d/%d, output %q", creates, updates, out.String())
	}
}
