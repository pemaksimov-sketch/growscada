package restapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/application"
	"github.com/kipitix/growscada/internal/server/domain/library"
	"github.com/kipitix/growscada/internal/server/domain/scene"
	"github.com/kipitix/growscada/internal/server/domain/tag"
)

func TestServiceProblem(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v0/scenes/scene-id/widgets/widget-id", nil)
	request.SetPathValue("id", "resource-id")
	request.SetPathValue("sceneId", "scene-id")
	request.SetPathValue("widgetId", "widget-id")
	for _, tc := range []struct {
		name   string
		err    error
		status int
		detail string
	}{
		{"scene", scene.ErrSceneNotFound, 404, "Scene with ID 'scene-id' not found"},
		{"widget", scene.ErrWidgetNotFound, 404, "Widget with ID 'widget-id' not found"},
		{"tag", tag.ErrTagNotFound, 404, "Tag with ID 'resource-id' not found"},
		{"type", library.ErrWidgetTypeNotFound, 404, "WidgetType with ID 'resource-id' not found"},
		{"scene conflict", scene.ErrSceneConflict, 409, ""},
		{"tag conflict", tag.ErrTagConflict, 409, ""},
		{"duplicate tag", tag.ErrTagNameTaken, 409, ""},
		{"type conflict", library.ErrWidgetTypeConflict, 409, ""},
		{"type in use", library.ErrWidgetTypeInUse, 409, ""},
		{"invalid input", application.ErrInvalidInput, 400, ""},
		{"unknown", errors.New("database credentials must stay private"), 500, "an internal error occurred"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problem := serviceProblem(request, fmt.Errorf("internal repository detail: %w", tc.err))
			if problem.Status != tc.status {
				t.Errorf("status = %d, want %d", problem.Status, tc.status)
			}
			if tc.detail != "" && problem.Detail != tc.detail {
				t.Errorf("detail = %q, want %q", problem.Detail, tc.detail)
			}
			if problem.Instance != request.URL.Path {
				t.Errorf("instance = %q", problem.Instance)
			}
		})
	}
	request.SetPathValue("sceneId", "")
	if got := serviceProblem(request, scene.ErrSceneNotFound).Detail; got != "Scene with ID 'resource-id' not found" {
		t.Errorf("scene's id path parameter: %s", got)
	}
}

func TestParsePathUUID(t *testing.T) {
	want := uuid.New()
	for _, raw := range []string{want.String(), "invalid", ""} {
		request := httptest.NewRequest(http.MethodGet, "/api/v0/tags/"+raw, nil)
		request.SetPathValue("id", raw)
		recorder := httptest.NewRecorder()
		got, ok := parsePathUUID(recorder, request, "id")
		if raw == want.String() {
			if !ok || got != want {
				t.Fatalf("valid id rejected: %v, %v", got, ok)
			}
		} else if ok || recorder.Code != http.StatusBadRequest {
			t.Errorf("invalid id accepted: %q, %v, status %d", raw, ok, recorder.Code)
		}
	}
}

func TestSendServiceError_MissingTagFilterIsEmptyList(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v0/tags?name=missing", nil)
	recorder := httptest.NewRecorder()
	sendServiceError(recorder, request, fmt.Errorf("lookup: %w", tag.ErrTagNotFound))
	if recorder.Code != 200 || recorder.Body.String() != `{"tags":[]}` {
		t.Fatalf("missing name filter: %d %s", recorder.Code, recorder.Body.String())
	}
}
