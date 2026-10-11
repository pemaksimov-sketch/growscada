package apiclient

import (
	"context"
	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
)

// Server is the UI's typed boundary. Client is the HTTP adapter; Memory is
// a deterministic adapter for testing UI operations without a running server.
type Server interface {
	ListTags(context.Context) ([]Tag, error)
	GetTag(context.Context, uuid.UUID) (Tag, error)
	CreateTagResult(context.Context, apiv0.CreateTagRequest) (apiv0.CreateTagResponse, error)
	SetTagValue(context.Context, uuid.UUID, string, string, int) (int, error)
	DeleteTag(context.Context, uuid.UUID) error
	ListScenes(context.Context) ([]apiv0.SceneResponse, error)
	GetScene(context.Context, uuid.UUID) (apiv0.SceneResponse, error)
	CreateScene(context.Context, apiv0.CreateSceneRequest) (apiv0.CreateSceneResponse, error)
	UpdateScene(context.Context, uuid.UUID, apiv0.UpdateSceneRequest) (apiv0.UpdateSceneResponse, error)
	DeleteScene(context.Context, uuid.UUID) (apiv0.SceneResponse, error)
	ListWidgets(context.Context, uuid.UUID) ([]apiv0.WidgetResponse, error)
	GetWidget(context.Context, uuid.UUID, uuid.UUID) (apiv0.WidgetResponse, error)
	CreateWidget(context.Context, uuid.UUID, apiv0.CreateWidgetRequest) (apiv0.CreateWidgetResponse, error)
	UpdateWidget(context.Context, uuid.UUID, uuid.UUID, apiv0.UpdateWidgetRequest) (apiv0.UpdateWidgetResponse, error)
	DeleteWidget(context.Context, uuid.UUID, uuid.UUID) (apiv0.WidgetResponse, error)
	ListWidgetTypes(context.Context) ([]apiv0.WidgetTypeResponse, error)
	GetWidgetType(context.Context, uuid.UUID) (apiv0.WidgetTypeResponse, error)
	CreateWidgetType(context.Context, apiv0.CreateWidgetTypeRequest) (apiv0.CreateWidgetTypeResponse, error)
	UpdateWidgetType(context.Context, uuid.UUID, apiv0.UpdateWidgetTypeRequest) (apiv0.UpdateWidgetTypeResponse, error)
	DeleteWidgetType(context.Context, uuid.UUID) (apiv0.WidgetTypeResponse, error)
}

var _ Server = (*Client)(nil)
