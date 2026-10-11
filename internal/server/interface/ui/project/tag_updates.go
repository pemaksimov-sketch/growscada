package project

import (
	"encoding/json"
	"github.com/google/uuid"
	"slices"
)

func decodeTagUpdate(raw json.RawMessage) (tagItem, bool) {
	var tag tagItem
	err := json.Unmarshal(raw, &tag)
	return tag, err == nil && tag.ID != uuid.Nil && (tag.Sequence > 0 || tag.Version > 0)
}

func tagRevision(tag tagItem) int64 {
	if tag.Sequence > 0 {
		return tag.Sequence
	}
	return int64(tag.Version)
}

// A list response can race an event. Keep the highest known version for
// each existing tag; the list remains authoritative for membership/order.
func mergeTagList(known, loaded []tagItem) []tagItem {
	byID := make(map[string]tagItem, len(known))
	for _, tag := range known {
		byID[tag.ID.String()] = tag
	}
	merged := slices.Clone(loaded)
	for i, tag := range merged {
		if current, exists := byID[tag.ID.String()]; exists && tagRevision(current) > tagRevision(tag) {
			merged[i] = current
		}
	}
	return merged
}

func applyTagUpdate(tags []tagItem, updated tagItem) ([]tagItem, bool) {
	for i, tag := range tags {
		if tag.ID.String() != updated.ID.String() {
			continue
		}
		if tagRevision(tag) >= tagRevision(updated) {
			return tags, true
		}
		result := slices.Clone(tags)
		if updated.Origin == "" {
			updated.Origin = tag.Origin
		}
		result[i] = updated
		return result, true
	}
	return tags, false // an event cannot establish a complete list; reload it
}
