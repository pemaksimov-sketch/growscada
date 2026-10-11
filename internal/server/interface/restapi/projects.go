package restapi

import (
	"net/http"

	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/application"
	"github.com/kipitix/growscada/internal/server/application/appdto"
)

type ProjectServices struct {
	Tags   application.TagService
	Scenes application.SceneService
}
type RouterOption func(*APIRouter)

// WithProjects exposes explicit project routes using services bound to that
// project by the composition root. The router never knows database/context keys.
func WithProjects(projects application.ProjectService, services func(uuid.UUID) ProjectServices) RouterOption {
	return func(router *APIRouter) {
		mux := router.serveMux
		base := apiv0.PathPrefix + "/projects"
		mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
			items, err := projects.FindAll(r.Context())
			if err != nil {
				sendServiceError(w, r, err)
				return
			}
			result := apiv0.GetProjectsResponse{Projects: make([]apiv0.ProjectResponse, len(items))}
			for i, p := range items {
				result.Projects[i] = projectResponse(p)
			}
			sendJSONResponse(w, http.StatusOK, result)
		})
		mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
			var input apiv0.CreateProjectRequest
			if !decodeRequest(w, r, &input) {
				return
			}
			p, err := projects.Create(r.Context(), input.Name)
			if err != nil {
				sendServiceError(w, r, err)
				return
			}
			sendJSONResponse(w, http.StatusCreated, projectResponse(p))
		})
		mux.HandleFunc("GET "+base+"/{project}", func(w http.ResponseWriter, r *http.Request) {
			projectID, ok := parsePathUUID(w, r, "project")
			if !ok {
				return
			}
			p, err := projects.FindByID(r.Context(), projectID)
			if err != nil {
				sendServiceError(w, r, err)
				return
			}
			sendJSONResponse(w, http.StatusOK, projectResponse(p))
		})
		mux.HandleFunc("PUT "+base+"/{project}", func(w http.ResponseWriter, r *http.Request) {
			projectID, ok := parsePathUUID(w, r, "project")
			if !ok {
				return
			}
			var input apiv0.UpdateProjectRequest
			if !decodeRequest(w, r, &input) {
				return
			}
			p, err := projects.Rename(r.Context(), projectID, input.Name, input.Version)
			if err != nil {
				sendServiceError(w, r, err)
				return
			}
			sendJSONResponse(w, http.StatusOK, projectResponse(p))
		})
		mux.HandleFunc("DELETE "+base+"/{project}", func(w http.ResponseWriter, r *http.Request) {
			projectID, ok := parsePathUUID(w, r, "project")
			if !ok {
				return
			}
			p, err := projects.Delete(r.Context(), projectID)
			if err != nil {
				sendServiceError(w, r, err)
				return
			}
			sendJSONResponse(w, http.StatusOK, projectResponse(p))
		})
		scoped := func(handler func(ProjectServices) http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				projectID, ok := parsePathUUID(w, r, "project")
				if !ok {
					return
				}
				if _, err := projects.FindByID(r.Context(), projectID); err != nil {
					sendServiceError(w, r, err)
					return
				}
				handler(services(projectID))(w, r)
			}
		}
		prefix := base + "/{project}"
		for _, route := range []struct {
			pattern string
			handler func(ProjectServices) http.HandlerFunc
		}{
			{"GET /tags", func(s ProjectServices) http.HandlerFunc { return NewTagsHandler(s.Tags).GetTags }},
			{"POST /tags", func(s ProjectServices) http.HandlerFunc { return NewTagsHandler(s.Tags).PostTags }},
			{"GET /tags/{id}", func(s ProjectServices) http.HandlerFunc { return NewTagsHandler(s.Tags).GetTagsByID }},
			{"DELETE /tags/{id}", func(s ProjectServices) http.HandlerFunc { return NewTagsHandler(s.Tags).DeleteTagsByID }},
			{"PATCH /tags/{id}/value", func(s ProjectServices) http.HandlerFunc { return NewTagsHandler(s.Tags).PatchTagsValue }},
			{"GET /scenes", func(s ProjectServices) http.HandlerFunc { return NewScenesHandler(s.Scenes).GetScenes }},
			{"POST /scenes", func(s ProjectServices) http.HandlerFunc { return NewScenesHandler(s.Scenes).PostScenes }},
			{"GET /scenes/{id}", func(s ProjectServices) http.HandlerFunc { return NewScenesHandler(s.Scenes).GetScenesByID }},
			{"PUT /scenes/{id}", func(s ProjectServices) http.HandlerFunc { return NewScenesHandler(s.Scenes).PutScenesByID }},
			{"DELETE /scenes/{id}", func(s ProjectServices) http.HandlerFunc { return NewScenesHandler(s.Scenes).DeleteScenesByID }},
			{"GET /scenes/{id}/widgets", func(s ProjectServices) http.HandlerFunc { return NewWidgetsHandler(s.Scenes).GetWidgetsBySceneID }},
			{"POST /scenes/{id}/widgets", func(s ProjectServices) http.HandlerFunc { return NewWidgetsHandler(s.Scenes).PostWidgets }},
			{"GET /scenes/{sceneId}/widgets/{widgetId}", func(s ProjectServices) http.HandlerFunc { return NewWidgetsHandler(s.Scenes).GetWidgetsByID }},
			{"PUT /scenes/{sceneId}/widgets/{widgetId}", func(s ProjectServices) http.HandlerFunc { return NewWidgetsHandler(s.Scenes).PutWidgetsByID }},
			{"DELETE /scenes/{sceneId}/widgets/{widgetId}", func(s ProjectServices) http.HandlerFunc { return NewWidgetsHandler(s.Scenes).DeleteWidgetsByID }},
			{"GET /events", func(ProjectServices) http.HandlerFunc { return router.eventsHandlers.GetEvents }},
		} {
			// Patterns are trusted literals, split at the method separator.
			for i, char := range route.pattern {
				if char == ' ' {
					mux.HandleFunc(route.pattern[:i]+" "+prefix+route.pattern[i+1:], scoped(route.handler))
					break
				}
			}
		}
	}
}

func projectResponse(p appdto.Project) apiv0.ProjectResponse {
	return apiv0.ProjectResponse{ID: p.ID, OrganizationID: p.OrganizationID, Name: p.Name, Version: p.Version}
}
