package restapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/application"
	"github.com/kipitix/growscada/internal/server/domain/library"
	"github.com/kipitix/growscada/internal/server/domain/project"
	"github.com/kipitix/growscada/internal/server/domain/scene"
	"github.com/kipitix/growscada/internal/server/domain/tag"
)

func parsePathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	value, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		sendJSONResponse(w, http.StatusBadRequest, NewBadRequest(err.Error(), r.URL.Path))
		return uuid.Nil, false
	}
	return value, true
}

func serviceProblem(r *http.Request, err error) *ProblemDetails {
	if errors.Is(err, application.ErrInvalidInput) {
		return NewBadRequest(err.Error(), r.URL.Path)
	}
	sceneID := r.PathValue("sceneId")
	if sceneID == "" {
		sceneID = r.PathValue("id")
	}
	for _, item := range []struct {
		cause        error
		resource, id string
	}{
		{project.ErrNotFound, "Project", r.PathValue("project")},
		{scene.ErrSceneNotFound, "Scene", sceneID},
		{scene.ErrWidgetNotFound, "Widget", r.PathValue("widgetId")},
		{tag.ErrTagNotFound, "Tag", r.PathValue("id")},
		{library.ErrWidgetTypeNotFound, "WidgetType", r.PathValue("id")},
		{library.ErrLibraryNotFound, "Library", r.PathValue("library")},
		{library.ErrReleaseNotFound, "LibraryRelease", r.PathValue("release")},
		{library.ErrProjectPinNotFound, "ProjectLibraryPin", r.PathValue("library")},
	} {
		if errors.Is(err, item.cause) {
			return NewNotFound(item.resource, item.id, r.URL.Path)
		}
	}
	for _, item := range []struct {
		cause    error
		resource string
	}{
		{project.ErrConflict, "project"},
		{project.ErrNameTaken, "project"},
		{scene.ErrSceneConflict, "scene"},
		{tag.ErrTagConflict, "tag"},
		{tag.ErrTagNameTaken, "tag"},
		{library.ErrWidgetTypeConflict, "widget type"},
		{library.ErrWidgetTypeInUse, "widget type"},
		{library.ErrLibraryConflict, "library"},
		{library.ErrLibraryNameTaken, "library"},
	} {
		if errors.Is(err, item.cause) {
			return NewConflict(item.resource, err.Error(), r.URL.Path)
		}
	}
	return NewInternalError("an internal error occurred", r.URL.Path)
}

// sendServiceError is the only mapping from application/domain errors to HTTP.
// A name-filtered tag listing keeps its established empty-list semantics.
func sendServiceError(w http.ResponseWriter, r *http.Request, err error) {
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/tags") && r.URL.Query().Has(apiv0.TagsQueryName) && errors.Is(err, tag.ErrTagNotFound) {
		sendJSONResponse(w, http.StatusOK, newGetTagsResponse(nil))
		return
	}
	problem := serviceProblem(r, err)
	if problem.Status == http.StatusInternalServerError {
		sendInternalError(w, r, err)
		return
	}
	sendJSONResponse(w, problem.Status, problem)
}
