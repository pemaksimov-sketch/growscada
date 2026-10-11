package apiclient

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"net/http"
)

func request[T any](ctx context.Context, c *Client, method, path string, body any, status int) (T, error) {
	var result T
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			return result, err
		}
	}
	err := c.do(ctx, method, apiv0.PathPrefix+path, encoded, status, &result)
	return result, err
}

func (c *Client) CreateTagResult(ctx context.Context, input apiv0.CreateTagRequest) (apiv0.CreateTagResponse, error) {
	return request[apiv0.CreateTagResponse](ctx, c, http.MethodPost, "/tags", input, http.StatusCreated)
}

func (c *Client) ListScenes(ctx context.Context) ([]apiv0.SceneResponse, error) {
	r, err := request[apiv0.GetScenesResponse](ctx, c, http.MethodGet, "/scenes", nil, http.StatusOK)
	return r.Scenes, err
}
func (c *Client) GetScene(ctx context.Context, id uuid.UUID) (apiv0.SceneResponse, error) {
	return request[apiv0.SceneResponse](ctx, c, http.MethodGet, "/scenes/"+id.String(), nil, http.StatusOK)
}
func (c *Client) CreateScene(ctx context.Context, input apiv0.CreateSceneRequest) (apiv0.CreateSceneResponse, error) {
	return request[apiv0.CreateSceneResponse](ctx, c, http.MethodPost, "/scenes", input, http.StatusCreated)
}
func (c *Client) UpdateScene(ctx context.Context, id uuid.UUID, input apiv0.UpdateSceneRequest) (apiv0.UpdateSceneResponse, error) {
	return request[apiv0.UpdateSceneResponse](ctx, c, http.MethodPut, "/scenes/"+id.String(), input, http.StatusOK)
}
func (c *Client) DeleteScene(ctx context.Context, id uuid.UUID) (apiv0.SceneResponse, error) {
	return request[apiv0.SceneResponse](ctx, c, http.MethodDelete, "/scenes/"+id.String(), nil, http.StatusOK)
}

func widgetsPath(sceneID uuid.UUID) string { return "/scenes/" + sceneID.String() + "/widgets" }
func (c *Client) ListWidgets(ctx context.Context, sceneID uuid.UUID) ([]apiv0.WidgetResponse, error) {
	r, err := request[apiv0.GetWidgetsResponse](ctx, c, http.MethodGet, widgetsPath(sceneID), nil, http.StatusOK)
	return r.Widgets, err
}
func (c *Client) GetWidget(ctx context.Context, sceneID, id uuid.UUID) (apiv0.WidgetResponse, error) {
	return request[apiv0.WidgetResponse](ctx, c, http.MethodGet, widgetsPath(sceneID)+"/"+id.String(), nil, http.StatusOK)
}
func (c *Client) CreateWidget(ctx context.Context, sceneID uuid.UUID, input apiv0.CreateWidgetRequest) (apiv0.CreateWidgetResponse, error) {
	return request[apiv0.CreateWidgetResponse](ctx, c, http.MethodPost, widgetsPath(sceneID), input, http.StatusCreated)
}
func (c *Client) UpdateWidget(ctx context.Context, sceneID, id uuid.UUID, input apiv0.UpdateWidgetRequest) (apiv0.UpdateWidgetResponse, error) {
	return request[apiv0.UpdateWidgetResponse](ctx, c, http.MethodPut, widgetsPath(sceneID)+"/"+id.String(), input, http.StatusOK)
}
func (c *Client) DeleteWidget(ctx context.Context, sceneID, id uuid.UUID) (apiv0.WidgetResponse, error) {
	return request[apiv0.WidgetResponse](ctx, c, http.MethodDelete, widgetsPath(sceneID)+"/"+id.String(), nil, http.StatusOK)
}

func (c *Client) ListWidgetTypes(ctx context.Context) ([]apiv0.WidgetTypeResponse, error) {
	r, err := request[apiv0.GetWidgetTypesResponse](ctx, c, http.MethodGet, "/widget-types", nil, http.StatusOK)
	return r.WidgetTypes, err
}
func (c *Client) GetWidgetType(ctx context.Context, id uuid.UUID) (apiv0.WidgetTypeResponse, error) {
	return request[apiv0.WidgetTypeResponse](ctx, c, http.MethodGet, "/widget-types/"+id.String(), nil, http.StatusOK)
}
func (c *Client) CreateWidgetType(ctx context.Context, input apiv0.CreateWidgetTypeRequest) (apiv0.CreateWidgetTypeResponse, error) {
	return request[apiv0.CreateWidgetTypeResponse](ctx, c, http.MethodPost, "/widget-types", input, http.StatusCreated)
}
func (c *Client) UpdateWidgetType(ctx context.Context, id uuid.UUID, input apiv0.UpdateWidgetTypeRequest) (apiv0.UpdateWidgetTypeResponse, error) {
	return request[apiv0.UpdateWidgetTypeResponse](ctx, c, http.MethodPut, "/widget-types/"+id.String(), input, http.StatusOK)
}
func (c *Client) DeleteWidgetType(ctx context.Context, id uuid.UUID) (apiv0.WidgetTypeResponse, error) {
	return request[apiv0.WidgetTypeResponse](ctx, c, http.MethodDelete, "/widget-types/"+id.String(), nil, http.StatusOK)
}
