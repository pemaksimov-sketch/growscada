package apiv0

import (
	"github.com/google/uuid"
)

// Name filters of GET /tags; at most one may be given.
const (
	// TagsQueryName selects the tag with this exact (unique) name.
	TagsQueryName = "name"
	// TagsQueryNamePattern selects tags whose whole name matches a wildcard
	// pattern ("*" any sequence, "?" one character).
	TagsQueryNamePattern = "name_pattern"
	// TagsQueryNameRegex selects tags whose name matches a Go (RE2) regular
	// expression anywhere.
	TagsQueryNameRegex = "name_regex"
)

// TagResponse is the HTTP DTO for representing a tag in API responses.
type TagResponse struct {
	Origin   string    `json:"origin,omitempty"`
	Sequence int64     `json:"sequence,omitempty"`
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	Value    string    `json:"value"`
	Quality  string    `json:"quality"`
	Version  int       `json:"version"`
}

// GetTagsResponse is the HTTP DTO for a list of tags response.
type GetTagsResponse struct {
	Tags []TagResponse `json:"tags"`
}

// CreateTagRequest is the HTTP DTO for a tag creation request.
type CreateTagRequest struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Quality string `json:"quality"`
}

// CreateTagResponse is the HTTP DTO for a tag creation response.
type CreateTagResponse struct {
	ID uuid.UUID `json:"id"`
}

// UpdateTagRequest is the HTTP DTO for a tag value update request.
// Runtime writes do not accept an expected configuration Version. The legacy
// optional field is tolerated during migration and ignored by the server.
type UpdateTagRequest struct {
	Value   string `json:"value"`
	Quality string `json:"quality"`
	Version int    `json:"version,omitempty"`
}

// UpdateTagResponse is the HTTP DTO for a tag value update response.
type UpdateTagResponse struct {
	Version  int   `json:"version"`
	Sequence int64 `json:"sequence"`
}

// UpdateTagDefinitionRequest edits configuration with optimistic concurrency.
type UpdateTagDefinitionRequest struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}
