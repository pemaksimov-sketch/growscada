package apiclient

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
)

// LibraryServer is the library-specific API used by the Library workspace.
type LibraryServer interface {
	ListLibraries(context.Context) ([]apiv0.LibraryResponse, error)
	ListLibraryReleases(context.Context, uuid.UUID) ([]apiv0.LibraryReleaseResponse, error)
	PublishLibrary(context.Context, uuid.UUID) (apiv0.CreateLibraryReleaseResponse, error)
	ListLibraryWidgetTypes(context.Context, uuid.UUID) ([]apiv0.WidgetTypeResponse, error)
	CreateLibraryWidgetType(context.Context, uuid.UUID, apiv0.CreateWidgetTypeRequest) (apiv0.CreateWidgetTypeResponse, error)
	UpdateLibraryWidgetType(context.Context, uuid.UUID, uuid.UUID, apiv0.UpdateWidgetTypeRequest) (apiv0.UpdateWidgetTypeResponse, error)
	DeleteLibraryWidgetType(context.Context, uuid.UUID, uuid.UUID) (apiv0.WidgetTypeResponse, error)
	ListProjectLibraryPins(context.Context, uuid.UUID) ([]apiv0.ProjectLibraryPinResponse, error)
	ListProjectWidgetTypes(context.Context, uuid.UUID) ([]apiv0.WidgetTypeResponse, error)
	PinProjectLibraryRelease(context.Context, uuid.UUID, apiv0.PinLibraryReleaseRequest) error
	UnpinProjectLibrary(context.Context, uuid.UUID, uuid.UUID) error
}

func (c *Client) ListLibraries(ctx context.Context) ([]apiv0.LibraryResponse, error) {
	r, err := request[apiv0.GetLibrariesResponse](ctx, c, http.MethodGet, "/libraries", nil, http.StatusOK)
	return r.Libraries, err
}
func (c *Client) ListLibraryReleases(ctx context.Context, libraryID uuid.UUID) ([]apiv0.LibraryReleaseResponse, error) {
	r, err := request[apiv0.GetLibraryReleasesResponse](ctx, c, http.MethodGet, "/libraries/"+libraryID.String()+"/releases", nil, http.StatusOK)
	return r.Releases, err
}
func (c *Client) PublishLibrary(ctx context.Context, libraryID uuid.UUID) (apiv0.CreateLibraryReleaseResponse, error) {
	return request[apiv0.CreateLibraryReleaseResponse](ctx, c, http.MethodPost, "/libraries/"+libraryID.String()+"/releases", nil, http.StatusCreated)
}
func (c *Client) ListLibraryWidgetTypes(ctx context.Context, libraryID uuid.UUID) ([]apiv0.WidgetTypeResponse, error) {
	r, err := request[apiv0.GetWidgetTypesResponse](ctx, c, http.MethodGet, "/libraries/"+libraryID.String()+"/widget-types", nil, http.StatusOK)
	return r.WidgetTypes, err
}
func (c *Client) CreateLibraryWidgetType(ctx context.Context, libraryID uuid.UUID, input apiv0.CreateWidgetTypeRequest) (apiv0.CreateWidgetTypeResponse, error) {
	return request[apiv0.CreateWidgetTypeResponse](ctx, c, http.MethodPost, "/libraries/"+libraryID.String()+"/widget-types", input, http.StatusCreated)
}
func (c *Client) UpdateLibraryWidgetType(ctx context.Context, libraryID, id uuid.UUID, input apiv0.UpdateWidgetTypeRequest) (apiv0.UpdateWidgetTypeResponse, error) {
	return request[apiv0.UpdateWidgetTypeResponse](ctx, c, http.MethodPut, "/libraries/"+libraryID.String()+"/widget-types/"+id.String(), input, http.StatusOK)
}
func (c *Client) DeleteLibraryWidgetType(ctx context.Context, libraryID, id uuid.UUID) (apiv0.WidgetTypeResponse, error) {
	return request[apiv0.WidgetTypeResponse](ctx, c, http.MethodDelete, "/libraries/"+libraryID.String()+"/widget-types/"+id.String(), nil, http.StatusOK)
}

func (c *Client) ListProjectLibraryPins(ctx context.Context, projectID uuid.UUID) ([]apiv0.ProjectLibraryPinResponse, error) {
	r, err := request[apiv0.GetProjectLibraryPinsResponse](ctx, c, http.MethodGet, "/projects/"+projectID.String()+"/libraries", nil, http.StatusOK)
	return r.Pins, err
}
func (c *Client) ListProjectWidgetTypes(ctx context.Context, projectID uuid.UUID) ([]apiv0.WidgetTypeResponse, error) {
	r, err := request[apiv0.GetWidgetTypesResponse](ctx, c, http.MethodGet, "/projects/"+projectID.String()+"/widget-types", nil, http.StatusOK)
	return r.WidgetTypes, err
}
func (c *Client) PinProjectLibraryRelease(ctx context.Context, projectID uuid.UUID, input apiv0.PinLibraryReleaseRequest) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, apiv0.PathPrefix+"/projects/"+projectID.String()+"/libraries", body, http.StatusNoContent, nil)
}
func (c *Client) UnpinProjectLibrary(ctx context.Context, projectID, libraryID uuid.UUID) error {
	return c.do(ctx, http.MethodDelete, apiv0.PathPrefix+"/projects/"+projectID.String()+"/libraries/"+libraryID.String(), nil, http.StatusNoContent, nil)
}

var _ LibraryServer = (*Client)(nil)
