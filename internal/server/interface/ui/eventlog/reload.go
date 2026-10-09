package eventlog

import apiv0 "github.com/kipitix/growscada/contract/api/v0"

// ReloadTargets says which server-backed lists a ServerEvent makes stale.
// It is the single routing table of the modes that show server data
// (Library, Project, Operation); each reloads the lists it shows.
type ReloadTargets struct {
	WidgetTypes, Scenes, Widgets, Tags bool
	SceneID                            string
}

// ReloadTargetsForScene scopes widget invalidation to its owner. Empty
// SceneID falls back to the broad reload used with older servers.
func ReloadTargetsForScene(ev ServerEvent, selectedSceneID string) ReloadTargets {
	targets := ReloadTargetsFor(ev.Type)
	if targets.Widgets && ev.SceneID != "" && ev.Type != EventResync {
		targets.Scenes = false
		targets.SceneID = ev.SceneID
		targets.Widgets = ev.SceneID == selectedSceneID
	}
	return targets
}

// AllTargets is every list: what a resync reloads, since the events
// published while disconnected are lost.
var AllTargets = ReloadTargets{WidgetTypes: true, Scenes: true, Widgets: true, Tags: true}

// ReloadTargetsFor routes a ServerEvent by its Type.
func ReloadTargetsFor(eventType string) ReloadTargets {
	switch eventType {
	case EventResync:
		return AllTargets
	case apiv0.EventWidgetTypeCreated, apiv0.EventWidgetTypeUpdated, apiv0.EventWidgetTypeDeleted:
		return ReloadTargets{WidgetTypes: true}
	case apiv0.EventSceneCreated, apiv0.EventSceneUpdated, apiv0.EventSceneDeleted:
		return ReloadTargets{Scenes: true}
	case apiv0.EventWidgetCreated, apiv0.EventWidgetUpdated, apiv0.EventWidgetDeleted:
		// A widget change bumps its Scene's version (the shared optimistic
		// lock) without a scene event, so the scenes are reloaded as well.
		return ReloadTargets{Scenes: true, Widgets: true}
	case apiv0.EventTagCreated, apiv0.EventTagUpdated, apiv0.EventTagDeleted:
		// tag_updated carries the Tag's new state; a mode that applies it
		// directly (Operation) reloads only when it cannot.
		return ReloadTargets{Tags: true}
	}
	return ReloadTargets{}
}
