package previslegacy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
)

var metadataFields = map[string]string{
	"directorSceneId": "previsSceneId", "directorShotId": "previsShotId",
	"directorPreviewNodeId": "previsPreviewNodeId", "directorDepthNodeId": "previsDepthNodeId",
	"directorNormalNodeId": "previsNormalNodeId", "directorClayVideoNodeId": "previsClayVideoNodeId",
}

type Error struct{ Path string }

func (e *Error) Error() string {
	return fmt.Sprintf("旧导演台数据无法可靠转换（%s），已停止写入，请保留原文件后核对", e.Path)
}

func equal(a, b json.RawMessage) bool {
	decode := func(raw json.RawMessage) any {
		var value any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if decoder.Decode(&value) != nil {
			return nil
		}
		return value
	}
	return equivalent(decode(a), decode(b))
}

func equivalent(a, b any) bool {
	if number, ok := a.(json.Number); ok {
		other, ok := b.(json.Number)
		if !ok {
			return false
		}
		left, leftOK := new(big.Rat).SetString(string(number))
		right, rightOK := new(big.Rat).SetString(string(other))
		return leftOK && rightOK && left.Cmp(right) == 0
	}
	switch value := a.(type) {
	case map[string]any:
		other, ok := b.(map[string]any)
		if !ok || len(value) != len(other) {
			return false
		}
		for key, child := range value {
			candidate, exists := other[key]
			if !exists || !equivalent(child, candidate) {
				return false
			}
		}
		return true
	case []any:
		other, ok := b.([]any)
		if !ok || len(value) != len(other) {
			return false
		}
		for index, child := range value {
			if !equivalent(child, other[index]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

func objectList(raw json.RawMessage, path string) ([]map[string]json.RawMessage, error) {
	var items []map[string]json.RawMessage
	if json.Unmarshal(raw, &items) != nil || items == nil {
		return nil, &Error{path}
	}
	seen := map[string]bool{}
	for _, item := range items {
		id := text(item["id"])
		if id == "" || seen[id] {
			return nil, &Error{path}
		}
		seen[id] = true
	}
	return items, nil
}

func text(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}

func CanonicalMetadataField(key string) string {
	if mapped, ok := metadataFields[key]; ok {
		return mapped
	}
	return key
}

// Scenes changes only the adopted document contract. Existing IDs, models,
// transforms and keyframes remain intact; migrated shots use the agreed 16:9.
func Scenes(raw json.RawMessage) (json.RawMessage, error) {
	scenes, err := objectList(raw, "directorScenes")
	if err != nil {
		return nil, err
	}
	for _, scene := range scenes {
		var version float64
		if json.Unmarshal(scene["version"], &version) != nil || version != 1 {
			return nil, &Error{"directorScenes.version"}
		}
		objects, err := objectList(scene["objects"], "directorScenes.objects")
		if err != nil {
			return nil, err
		}
		cameras, err := objectList(scene["cameras"], "directorScenes.cameras")
		if err != nil {
			return nil, err
		}
		if _, err := objectList(scene["lights"], "directorScenes.lights"); err != nil {
			return nil, err
		}
		shots, err := objectList(scene["shots"], "directorScenes.shots")
		if err != nil {
			return nil, err
		}
		cameraIDs, shotIDs := map[string]bool{}, map[string]bool{}
		for _, camera := range cameras {
			cameraIDs[text(camera["id"])] = true
		}
		for _, shot := range shots {
			if !cameraIDs[text(shot["cameraId"])] {
				return nil, &Error{"directorScenes.shots.cameraId"}
			}
			shotIDs[text(shot["id"])] = true
			shot["aspectRatio"] = json.RawMessage(`"16:9"`)
		}
		if active := text(scene["activeShotId"]); active != "" && !shotIDs[active] {
			return nil, &Error{"directorScenes.activeShotId"}
		}
		for _, object := range objects {
			if marker, exists := object["legacyDirector"]; exists && !bytes.Equal(bytes.TrimSpace(marker), []byte("true")) {
				return nil, &Error{"directorScenes.objects.legacyDirector"}
			}
			object["legacyDirector"] = json.RawMessage(`true`)
		}
		scene["objects"], _ = json.Marshal(objects)
		scene["shots"], _ = json.Marshal(shots)
	}
	return json.Marshal(scenes)
}

func Metadata(raw json.RawMessage) (json.RawMessage, bool, error) {
	var metadata map[string]json.RawMessage
	if json.Unmarshal(raw, &metadata) != nil || metadata == nil {
		return raw, false, nil
	}
	changed := false
	for old, key := range metadataFields {
		value, exists := metadata[old]
		if !exists {
			continue
		}
		var id string
		if json.Unmarshal(value, &id) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, false, &Error{"nodes.metadata." + old}
		}
		if next, exists := metadata[key]; exists && !equal(value, next) {
			return nil, false, &Error{"nodes.metadata." + key}
		}
		metadata[key] = value
		delete(metadata, old)
		changed = true
	}
	if !changed {
		return raw, false, nil
	}
	result, err := json.Marshal(metadata)
	return result, true, err
}

func Node(raw json.RawMessage) (json.RawMessage, bool, error) {
	var node map[string]json.RawMessage
	if json.Unmarshal(raw, &node) != nil || node == nil {
		return raw, false, nil
	}
	metadata, changed, err := Metadata(node["metadata"])
	if err != nil || !changed {
		return raw, false, err
	}
	node["metadata"] = metadata
	result, err := json.Marshal(node)
	return result, true, err
}

// Normalize is a pure projection: reads never update stored rows or history.
func Normalize(raw json.RawMessage) (json.RawMessage, error) {
	var doc map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil || doc == nil {
		return nil, &Error{"document"}
	}
	changed := false
	if nodesRaw, exists := doc["nodes"]; exists {
		var nodes []json.RawMessage
		if json.Unmarshal(nodesRaw, &nodes) == nil {
			for index, node := range nodes {
				converted, updated, err := Node(node)
				if err != nil {
					return nil, err
				}
				if updated {
					nodes[index], changed = converted, true
				}
			}
			if changed {
				doc["nodes"], _ = json.Marshal(nodes)
			}
		}
	}
	if old, exists := doc["directorScenes"]; exists {
		converted, err := Scenes(old)
		if err != nil {
			return nil, err
		}
		if next, exists := doc["previsScenes"]; exists && !equal(next, old) && !equal(next, converted) {
			return nil, &Error{"previsScenes"}
		}
		doc["previsScenes"] = converted
		delete(doc, "directorScenes")
		changed = true
	}
	if !changed {
		return raw, nil
	}
	result, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	if err := ValidateReferences(result); err != nil {
		return nil, err
	}
	return result, nil
}

func ValidateReferences(raw json.RawMessage) error {
	var doc map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil {
		return &Error{"document"}
	}
	nodesRaw, scenesRaw := doc["nodes"], doc["previsScenes"]
	if nodesRaw == nil {
		nodesRaw = json.RawMessage(`[]`)
	}
	if scenesRaw == nil {
		scenesRaw = json.RawMessage(`[]`)
	}
	nodes, err := objectList(nodesRaw, "nodes")
	if err != nil {
		return err
	}
	scenes, err := objectList(scenesRaw, "previsScenes")
	if err != nil {
		return err
	}
	nodeIDs, sceneShots := map[string]bool{}, map[string]map[string]bool{}
	for _, node := range nodes {
		nodeIDs[text(node["id"])] = true
	}
	output := func(value json.RawMessage, path string) error {
		if value == nil || string(value) == `""` {
			return nil
		}
		if !nodeIDs[text(value)] {
			return &Error{path}
		}
		return nil
	}
	for _, scene := range scenes {
		shots, err := objectList(scene["shots"], "previsScenes.shots")
		if err != nil {
			return err
		}
		ids := map[string]bool{}
		for _, shot := range shots {
			ids[text(shot["id"])] = true
			for _, key := range []string{"previewNodeId", "depthNodeId", "normalNodeId"} {
				if err := output(shot[key], "previsScenes.shots."+key); err != nil {
					return err
				}
			}
		}
		sceneShots[text(scene["id"])] = ids
	}
	for _, node := range nodes {
		var metadata map[string]json.RawMessage
		_ = json.Unmarshal(node["metadata"], &metadata)
		for _, key := range metadataFields {
			if value, exists := metadata[key]; exists {
				var id string
				if json.Unmarshal(value, &id) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
					return &Error{"nodes.metadata." + key}
				}
			}
		}
		sceneID, shotID := text(metadata["previsSceneId"]), text(metadata["previsShotId"])
		if sceneID != "" && sceneShots[sceneID] == nil {
			return &Error{"nodes.metadata.previsSceneId"}
		}
		if shotID != "" && !sceneShots[sceneID][shotID] {
			return &Error{"nodes.metadata.previsShotId"}
		}
		for _, key := range []string{"previsPreviewNodeId", "previsDepthNodeId", "previsNormalNodeId", "previsClayVideoNodeId"} {
			if err := output(metadata[key], "nodes.metadata."+key); err != nil {
				return err
			}
		}
	}
	return nil
}
