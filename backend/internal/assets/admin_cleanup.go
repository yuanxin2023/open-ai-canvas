package assets

import (
	"encoding/json"
	"strings"
)

type MediaCleanup struct {
	Payload        string
	Changed        bool
	HadMedia       bool
	HasMedia       bool
	RemovedNodeIDs []string
	ChangedNodeIDs []string
}

// These branches are provenance or presentation, not independently usable media.
func nonMediaBranch(key string) bool {
	switch key {
	case "chatSessions", "messages", "history", "prompt", "references", "generation", "generationSpec", "inputSnapshot", "batchInputSnapshot", "coverUrl", "coverStorageKey", "thumbnail", "thumbnailUrl", "thumbnailStorageKey", "previewContent", "videoPreview", "imageRotation":
		return true
	}
	return false
}

func mediaLocator(key string) bool {
	switch key {
	case "storageKey", "resourceId", "resourceID", "content", "url", "dataUrl", "src", "modelUrl", "audioUrl", "videoUrl", "imageUrl":
		return true
	}
	return false
}

func usableLocator(value string) bool {
	return ResourceID(value) != "" || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "data:") || strings.HasPrefix(value, "blob:")
}

func HasMedia(value any) bool {
	switch item := value.(type) {
	case map[string]any:
		if kind, _ := item["type"].(string); kind == "text" || kind == "markdown" || kind == "script" || kind == "html" || kind == "svg" {
			for key, child := range item {
				if key != "metadata" && !nonMediaBranch(key) && HasMedia(child) {
					return true
				}
			}
			return false
		}
		for key, child := range item {
			if nonMediaBranch(key) {
				continue
			}
			if str, ok := child.(string); ok && mediaLocator(key) && (usableLocator(str) || ((key == "resourceId" || key == "resourceID") && ValidID(str) != "")) {
				return true
			}
			if HasMedia(child) {
				return true
			}
		}
	case []any:
		for _, child := range item {
			if HasMedia(child) {
				return true
			}
		}
	}
	return false
}

func ownDeletedMedia(item map[string]any, deleted map[string]struct{}) bool {
	for key, child := range item {
		str, ok := child.(string)
		if !ok || !mediaLocator(key) {
			continue
		}
		id := ResourceID(str)
		if key == "resourceId" || key == "resourceID" {
			id = ValidID(str)
		}
		if _, exists := deleted[id]; exists && id != "" {
			return true
		}
	}
	return false
}

func cleanMediaValue(value any, deleted map[string]struct{}) any {
	switch item := value.(type) {
	case map[string]any:
		primaryDeleted := ownDeletedMedia(item, deleted)
		for key, child := range item {
			if str, ok := child.(string); ok {
				id := resourceIDForField(key, str)
				_, gone := deleted[id]
				if (gone && id != "") || (primaryDeleted && mediaLocator(key)) {
					delete(item, key)
				}
				continue
			}
			cleaned := cleanMediaValue(child, deleted)
			if key == "mediaVersions" {
				if versions, ok := cleaned.([]any); ok {
					kept := make([]any, 0, len(versions))
					for _, v := range versions {
						if obj, ok := v.(map[string]any); ok && HasMedia(obj["media"]) {
							kept = append(kept, v)
						}
					}
					cleaned = kept
				}
			}
			if cleaned == nil {
				delete(item, key)
			} else {
				item[key] = cleaned
			}
		}
		return item
	case []any:
		kept := make([]any, 0, len(item))
		for _, child := range item {
			hadMedia := HasMedia(child)
			if cleaned := cleanMediaValue(child, deleted); cleaned != nil && (!hadMedia || HasMedia(cleaned)) {
				kept = append(kept, cleaned)
			}
		}
		return kept
	}
	return value
}

// CleanMediaDocument handles current canvas state, never counts chat or snapshots.
// Versions are promoted before generic sanitization so surviving outputs keep their node.
func CleanMediaDocument(raw string, deleted map[string]struct{}, forcedNodes map[string]struct{}) (MediaCleanup, error) {
	result := MediaCleanup{Payload: raw, RemovedNodeIDs: []string{}}
	var root map[string]any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return result, err
	}
	if root == nil {
		return result, nil
	}
	result.HadMedia = HasMedia(root)
	nodes, canvas := root["nodes"].([]any)
	removed := map[string]struct{}{}
	if canvas {
		kept := make([]any, 0, len(nodes))
		for _, value := range nodes {
			node, ok := value.(map[string]any)
			if !ok {
				kept = append(kept, value)
				continue
			}
			id, _ := node["id"].(string)
			beforeNode, _ := json.Marshal(node)
			_, forced := forcedNodes[id]
			metadata, _ := node["metadata"].(map[string]any)
			had := HasMedia(node)
			if metadata != nil && ownDeletedMedia(metadata, deleted) {
				delete(metadata, "assetId")
				for key := range metadata {
					if mediaLocator(key) {
						delete(metadata, key)
					}
				}
				if versions, ok := metadata["mediaVersions"].([]any); ok {
					for index := len(versions) - 1; index >= 0; index-- {
						version, _ := versions[index].(map[string]any)
						media, _ := version["media"].(map[string]any)
						if media != nil && !ownDeletedMedia(media, deleted) && HasMedia(media) {
							for key, child := range media {
								metadata[key] = child
							}
							generation, _ := version["generation"].(map[string]any)
							for _, key := range []string{"prompt", "model", "producedModel", "size", "quality", "seconds", "references", "taskId", "taskDurationMs"} {
								delete(metadata, key)
								if value, ok := generation[key]; ok {
									metadata[key] = value
								}
							}
							metadata["currentMediaVersionId"] = version["id"]
							break
						}
					}
				}
			}
			cleaned := cleanMediaValue(node, deleted)
			if forced || (had && !HasMedia(cleaned)) {
				removed[id] = struct{}{}
				result.RemovedNodeIDs = append(result.RemovedNodeIDs, id)
				result.ChangedNodeIDs = append(result.ChangedNodeIDs, id)
				continue
			}
			if cleaned != nil {
				afterNode, _ := json.Marshal(cleaned)
				if string(beforeNode) != string(afterNode) {
					result.ChangedNodeIDs = append(result.ChangedNodeIDs, id)
				}
				kept = append(kept, cleaned)
			}
		}
		root["nodes"] = kept
	}
	// Clean non-node structures too, including timeline and previs media.
	for key, child := range root {
		if key == "nodes" {
			continue
		}
		if str, ok := child.(string); ok {
			if _, gone := deleted[resourceIDForField(key, str)]; gone {
				delete(root, key)
			}
			continue
		}
		cleaned := cleanMediaValue(child, deleted)
		if cleaned == nil {
			delete(root, key)
		} else {
			root[key] = cleaned
		}
	}
	pruneNodeReferences(root, removed)
	result.HasMedia = HasMedia(root)
	encoded, err := json.Marshal(root)
	if err != nil {
		return result, err
	}
	var original any
	_ = json.Unmarshal([]byte(raw), &original)
	canonical, _ := json.Marshal(original)
	result.Changed = string(encoded) != string(canonical)
	result.Payload = string(encoded)
	return result, nil
}

func pruneNodeReferences(value any, removed map[string]struct{}) {
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			if str, ok := child.(string); ok && (strings.HasSuffix(key, "NodeId") || key == "nodeId" || key == "parentId") {
				if _, gone := removed[str]; gone {
					delete(item, key)
				}
				continue
			}
			if array, ok := child.([]any); ok {
				kept := make([]any, 0, len(array))
				for _, entry := range array {
					drop := false
					if str, ok := entry.(string); ok && (strings.HasSuffix(key, "NodeIds") || key == "nodeIds" || key == "children") {
						_, drop = removed[str]
					}
					if obj, ok := entry.(map[string]any); ok {
						for _, field := range []string{"fromNodeId", "toNodeId", "nodeId"} {
							id, _ := obj[field].(string)
							if _, gone := removed[id]; gone {
								drop = true
							}
						}
					}
					if !drop {
						pruneNodeReferences(entry, removed)
						kept = append(kept, entry)
					}
				}
				item[key] = kept
			} else {
				pruneNodeReferences(child, removed)
			}
		}
	case []any:
		for _, child := range item {
			pruneNodeReferences(child, removed)
		}
	}
}
