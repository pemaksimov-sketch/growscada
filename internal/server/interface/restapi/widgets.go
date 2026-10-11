package restapi

import (
	"net/http"

	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/application"
)

// WidgetsHandlers handles HTTP requests related to widget instances. Widget is
// an entity of the Scene aggregate, so every route here is nested under a scene.
type WidgetsHandlers struct {
	service application.SceneService
}

func NewWidgetsHandler(s application.SceneService) *WidgetsHandlers {
	return &WidgetsHandlers{service: s}
}

// GetWidgetsBySceneID handles GET /scenes/{id}/widgets
func (h WidgetsHandlers) GetWidgetsBySceneID(w http.ResponseWriter, r *http.Request) {
	sceneID, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}

	list, err := h.service.FindWidgetsBySceneID(r.Context(), sceneID)
	if err != nil {
		sendServiceError(w, r, err)
		return
	}
	sendJSONResponse(w, http.StatusOK, newGetWidgetsResponse(list))
}

// GetWidgetsByID handles GET /scenes/{sceneId}/widgets/{widgetId}
func (h WidgetsHandlers) GetWidgetsByID(w http.ResponseWriter, r *http.Request) {
	sceneID, ok := parsePathUUID(w, r, "sceneId")
	if !ok {
		return
	}
	widgetID, ok := parsePathUUID(w, r, "widgetId")
	if !ok {
		return
	}

	found, err := h.service.FindWidgetByID(r.Context(), sceneID, widgetID)
	if err != nil {
		sendServiceError(w, r, err)
		return
	}

	sendJSONResponse(w, http.StatusOK, newWidgetResponse(found))
}

// PostWidgets handles POST /scenes/{id}/widgets
func (h WidgetsHandlers) PostWidgets(w http.ResponseWriter, r *http.Request) {
	sceneID, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}

	var request apiv0.CreateWidgetRequest
	if !decodeRequest(w, r, &request) {
		return
	}

	created, err := h.service.CreateWidget(r.Context(), sceneID, request.SceneVersion, newCreateWidgetInput(request))
	if err != nil {
		sendServiceError(w, r, err)
		return
	}

	sendJSONResponse(w, http.StatusCreated, newCreateWidgetResponse(created))
}

// PutWidgetsByID handles PUT /scenes/{sceneId}/widgets/{widgetId}
func (h WidgetsHandlers) PutWidgetsByID(w http.ResponseWriter, r *http.Request) {
	sceneID, ok := parsePathUUID(w, r, "sceneId")
	if !ok {
		return
	}
	widgetID, ok := parsePathUUID(w, r, "widgetId")
	if !ok {
		return
	}

	var request apiv0.UpdateWidgetRequest
	if !decodeRequest(w, r, &request) {
		return
	}

	updated, err := h.service.UpdateWidget(r.Context(), sceneID, widgetID, request.SceneVersion, newUpdateWidgetInput(request))
	if err != nil {
		sendServiceError(w, r, err)
		return
	}

	sendJSONResponse(w, http.StatusOK, newUpdateWidgetResponse(updated))
}

// DeleteWidgetsByID handles DELETE /scenes/{sceneId}/widgets/{widgetId}
func (h WidgetsHandlers) DeleteWidgetsByID(w http.ResponseWriter, r *http.Request) {
	sceneID, ok := parsePathUUID(w, r, "sceneId")
	if !ok {
		return
	}
	widgetID, ok := parsePathUUID(w, r, "widgetId")
	if !ok {
		return
	}

	deleted, err := h.service.DeleteWidgetByID(r.Context(), sceneID, widgetID)
	if err != nil {
		sendServiceError(w, r, err)
		return
	}

	sendJSONResponse(w, http.StatusOK, newWidgetResponse(deleted))
}
