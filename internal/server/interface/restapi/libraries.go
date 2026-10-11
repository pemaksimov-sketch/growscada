package restapi

import (
	"net/http"

	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/application"
	"github.com/kipitix/growscada/internal/server/application/appdto"
)

// WithLibraries exposes organization libraries and project library pins.
func WithLibraries(service application.LibraryService, widgetTypeServices ...func(uuid.UUID) application.WidgetTypeService) RouterOption {
	return func(router *APIRouter) {
		mux := router.serveMux
		base := apiv0.PathPrefix + "/libraries"
		mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
			items, err := service.FindAll(r.Context())
			if err != nil {
				sendServiceError(w, r, err)
				return
			}
			result := apiv0.GetLibrariesResponse{Libraries: make([]apiv0.LibraryResponse, len(items))}
			for i, item := range items {
				result.Libraries[i] = libraryResponse(item)
			}
			sendJSONResponse(w, http.StatusOK, result)
		})
		mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
			var input apiv0.CreateLibraryRequest
			if !decodeRequest(w, r, &input) {
				return
			}
			item, err := service.Create(r.Context(), input.Name, input.Public)
			if err != nil {
				sendServiceError(w, r, err)
				return
			}
			sendJSONResponse(w, http.StatusCreated, libraryResponse(item))
		})
		mux.HandleFunc("PUT "+base+"/{library}", func(w http.ResponseWriter, r *http.Request) {
			libraryID, ok := parsePathUUID(w, r, "library")
			if !ok {
				return
			}
			var input apiv0.UpdateLibraryRequest
			if !decodeRequest(w, r, &input) {
				return
			}
			item, err := service.Update(r.Context(), libraryID, input.Name, input.Public, input.Version)
			if err != nil {
				sendServiceError(w, r, err)
				return
			}
			sendJSONResponse(w, http.StatusOK, libraryResponse(item))
		})
		mux.HandleFunc("GET "+base+"/{library}/releases", func(w http.ResponseWriter, r *http.Request) {
			libraryID, ok := parsePathUUID(w, r, "library")
			if !ok {
				return
			}
			items, err := service.FindReleases(r.Context(), libraryID)
			if err != nil {
				sendServiceError(w, r, err)
				return
			}
			result := apiv0.GetLibraryReleasesResponse{Releases: make([]apiv0.LibraryReleaseResponse, len(items))}
			for i, item := range items {
				result.Releases[i] = libraryReleaseResponse(item)
			}
			sendJSONResponse(w, http.StatusOK, result)
		})
		mux.HandleFunc("POST "+base+"/{library}/releases", func(w http.ResponseWriter, r *http.Request) {
			libraryID, ok := parsePathUUID(w, r, "library")
			if !ok {
				return
			}
			item, err := service.Release(r.Context(), libraryID)
			if err != nil {
				sendServiceError(w, r, err)
				return
			}
			sendJSONResponse(w, http.StatusCreated, apiv0.CreateLibraryReleaseResponse{ID: item.ID, Number: item.Number})
		})
		if len(widgetTypeServices) > 0 && widgetTypeServices[0] != nil {
			for _, route := range []struct {
				pattern string
				handler func(*WidgetTypesHandlers, http.ResponseWriter, *http.Request)
			}{
				{"GET " + base + "/{library}/widget-types", func(h *WidgetTypesHandlers, w http.ResponseWriter, r *http.Request) { h.GetWidgetTypes(w, r) }},
				{"POST " + base + "/{library}/widget-types", func(h *WidgetTypesHandlers, w http.ResponseWriter, r *http.Request) { h.PostWidgetTypes(w, r) }},
				{"GET " + base + "/{library}/widget-types/{id}", func(h *WidgetTypesHandlers, w http.ResponseWriter, r *http.Request) { h.GetWidgetTypesByID(w, r) }},
				{"PUT " + base + "/{library}/widget-types/{id}", func(h *WidgetTypesHandlers, w http.ResponseWriter, r *http.Request) { h.PutWidgetTypesByID(w, r) }},
				{"DELETE " + base + "/{library}/widget-types/{id}", func(h *WidgetTypesHandlers, w http.ResponseWriter, r *http.Request) { h.DeleteWidgetTypesByID(w, r) }},
			} {
				mux.HandleFunc(route.pattern, func(w http.ResponseWriter, r *http.Request) {
					libraryID, ok := parsePathUUID(w, r, "library")
					if !ok {
						return
					}
					r.SetPathValue("id", r.PathValue("id"))
					route.handler(NewWidgetTypesHandler(widgetTypeServices[0](libraryID)), w, r)
				})
			}
		}
		projectBase := apiv0.PathPrefix + "/projects/{project}/libraries"
		mux.HandleFunc("GET "+projectBase, func(w http.ResponseWriter, r *http.Request) {
			projectID, ok := parsePathUUID(w, r, "project")
			if !ok {
				return
			}
			items, err := service.FindProjectPins(r.Context(), projectID)
			if err != nil {
				sendServiceError(w, r, err)
				return
			}
			result := apiv0.GetProjectLibraryPinsResponse{Pins: make([]apiv0.ProjectLibraryPinResponse, len(items))}
			for i, item := range items {
				result.Pins[i] = apiv0.ProjectLibraryPinResponse{LibraryID: item.Release.LibraryID, Release: libraryReleaseResponse(item.Release)}
			}
			sendJSONResponse(w, http.StatusOK, result)
		})
		mux.HandleFunc("POST "+projectBase, func(w http.ResponseWriter, r *http.Request) {
			projectID, ok := parsePathUUID(w, r, "project")
			if !ok {
				return
			}
			var input apiv0.PinLibraryReleaseRequest
			if !decodeRequest(w, r, &input) {
				return
			}
			if err := service.PinRelease(r.Context(), projectID, input.LibraryID, input.ReleaseID); err != nil {
				sendServiceError(w, r, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		mux.HandleFunc("DELETE "+projectBase+"/{library}", func(w http.ResponseWriter, r *http.Request) {
			projectID, ok := parsePathUUID(w, r, "project")
			if !ok {
				return
			}
			libraryID, ok := parsePathUUID(w, r, "library")
			if !ok {
				return
			}
			if err := service.UnpinLibrary(r.Context(), projectID, libraryID); err != nil {
				sendServiceError(w, r, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	}
}

func libraryResponse(item appdto.Library) apiv0.LibraryResponse {
	return apiv0.LibraryResponse{ID: item.ID, OrganizationID: item.OrganizationID, Name: item.Name, Public: item.Public, Version: item.Version}
}
func libraryReleaseResponse(item appdto.LibraryRelease) apiv0.LibraryReleaseResponse {
	return apiv0.LibraryReleaseResponse{ID: item.ID, LibraryID: item.LibraryID, Number: item.Number, CreatedAt: item.CreatedAt, Content: item.Content}
}
