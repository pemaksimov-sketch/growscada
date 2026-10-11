package apiclient

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
)

func (c *Client) ListProjects(ctx context.Context) ([]apiv0.ProjectResponse, error) {
	result, err := request[apiv0.GetProjectsResponse](ctx, c, http.MethodGet, "/projects", nil, http.StatusOK)
	return result.Projects, err
}
func (c *Client) CreateProject(ctx context.Context, input apiv0.CreateProjectRequest) (apiv0.ProjectResponse, error) {
	return request[apiv0.ProjectResponse](ctx, c, http.MethodPost, "/projects", input, http.StatusCreated)
}
func (c *Client) UpdateProject(ctx context.Context, projectID uuid.UUID, input apiv0.UpdateProjectRequest) (apiv0.ProjectResponse, error) {
	return request[apiv0.ProjectResponse](ctx, c, http.MethodPut, "/projects/"+projectID.String(), input, http.StatusOK)
}
func (c *Client) DeleteProject(ctx context.Context, projectID uuid.UUID) (apiv0.ProjectResponse, error) {
	return request[apiv0.ProjectResponse](ctx, c, http.MethodDelete, "/projects/"+projectID.String(), nil, http.StatusOK)
}
