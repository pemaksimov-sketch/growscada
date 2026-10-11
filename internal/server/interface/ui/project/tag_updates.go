package project

import (
	"encoding/json"
	"slices"
)

func decodeTagUpdate(raw json.RawMessage) (tagItem, bool) {
	var tag tagItem
	err := json.Unmarshal(raw, &tag)
	return tag, err == nil && tag.ID != "" && tag.Version > 0
}

// A list response can race an event. Keep the highest known version for
// each existing tag; the list remains authoritative for membership/order.
func mergeTagList(known, loaded []tagItem) []tagItem {
	byID := make(map[string]tagItem, len(known))
	for _, tag := range known {
		byID[tag.ID] = tag
	}
	merged := slices.Clone(loaded)
	for i, tag := range merged {
		if current, exists := byID[tag.ID]; exists && current.Version > tag.Version {
			merged[i] = current
		}
	}
	return merged
}

func applyTagUpdate(tags []tagItem, updated tagItem) ([]tagItem, bool) {
	for i, tag := range tags {
		if tag.ID != updated.ID {
			continue
		}
		if tag.Version >= updated.Version {
			return tags, true
		}
		result := slices.Clone(tags)
		result[i] = updated
		return result, true
	}
	return tags, false // an event cannot establish a complete list; reload it
}
