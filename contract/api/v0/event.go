package apiv0

// Event is the payload of every message of the event stream (GET
// /events, sent as SSE `data:`), whatever the domain event. Tag carries the
// Tag's full new state, in the shape of GET /tags/{id}, on tag_updated only.
// Readers skip event types they do not know: a new type is a MINOR change.
type Event struct {
	Type      string       `json:"type"`
	Timestamp string       `json:"timestamp"`
	ID        string       `json:"id,omitempty"`
	SceneID   string       `json:"scene_id,omitempty"`   // owning scene for widget events
	ProjectID string       `json:"project_id,omitempty"` // empty for shared library/system events
	Tag       *TagResponse `json:"tag,omitempty"`
}

// Event names are shared by API producers and consumers. Unknown names must
// still be tolerated, so Event.Type intentionally remains an open string.
const (
	EventSystemReady               = "system_ready"
	EventTagCreated                = "tag_created"
	EventTagUpdated                = "tag_updated"
	EventTagDeleted                = "tag_deleted"
	EventWidgetTypeCreated         = "widget_type_created"
	EventWidgetTypeUpdated         = "widget_type_updated"
	EventWidgetTypeDeleted         = "widget_type_deleted"
	EventWidgetCreated             = "widget_created"
	EventWidgetUpdated             = "widget_updated"
	EventWidgetDeleted             = "widget_deleted"
	EventSceneCreated              = "scene_created"
	EventSceneUpdated              = "scene_updated"
	EventSceneDeleted              = "scene_deleted"
	EventClientConnected           = "client_connected"
	EventClientDisconnected        = "client_disconnected"
	EventProjectCreated            = "project_created"
	EventProjectUpdated            = "project_updated"
	EventProjectDeleted            = "project_deleted"
	EventTagDefinitionUpdated      = "tag_definition_updated"
	EventLibraryCreated            = "library_created"
	EventLibraryUpdated            = "library_updated"
	EventLibraryReleased           = "library_released"
	EventProjectLibraryPinsChanged = "project_library_pins_changed"
)

func EventTypes() []string {
	return []string{EventSystemReady, EventTagCreated, EventTagUpdated, EventTagDeleted,
		EventWidgetTypeCreated, EventWidgetTypeUpdated, EventWidgetTypeDeleted,
		EventWidgetCreated, EventWidgetUpdated, EventWidgetDeleted,
		EventSceneCreated, EventSceneUpdated, EventSceneDeleted,
		EventClientConnected, EventClientDisconnected, EventProjectCreated, EventProjectUpdated, EventProjectDeleted, EventTagDefinitionUpdated,
		EventLibraryCreated, EventLibraryUpdated, EventLibraryReleased, EventProjectLibraryPinsChanged}
}
