package canvas

import (
	"encoding/json"
	"os"
	"testing"
)

func legacyOperationFixture(t *testing.T) (map[string]json.RawMessage, map[string]json.RawMessage) {
	t.Helper()
	raw, err := os.ReadFile("previslegacy/testdata/director.json")
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := NormalizePrevisDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	var original, canonical map[string]json.RawMessage
	if json.Unmarshal(raw, &original) != nil || json.Unmarshal(normalized, &canonical) != nil {
		t.Fatal("fixture invalid")
	}
	return original, canonical
}

func TestLegacySceneOperationKeepsDigestAndChecksCanonicalBaseline(t *testing.T) {
	original, canonical := legacyOperationFixture(t)
	var scenes []map[string]any
	_ = json.Unmarshal(original["directorScenes"], &scenes)
	scenes[0]["title"] = "Edited title"
	after, _ := json.Marshal(scenes)
	req := CanvasOperationsRequest{MutationID: "original-lost-response-id", Operations: []CanvasOperation{{Type: "set_field", Field: "directorScenes", BeforeValue: original["directorScenes"], Value: after}}}
	digest := CanvasOperationsDigest(req)
	converted, changed, err := migratePrevisOperations(req)
	if err != nil || !changed {
		t.Fatal("legacy request not converted", err)
	}
	if CanvasOperationsDigest(req) != digest || converted.MutationID != req.MutationID || req.Operations[0].Field != "directorScenes" {
		t.Fatal("retry identity/request mutated")
	}
	if err := applyCanvasOperations(canonical, converted.Operations); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	_ = json.Unmarshal(canonical["previsScenes"], &got)
	if got[0]["title"] != "Edited title" {
		t.Fatal("edit lost")
	}
	if err := applyCanvasOperations(canonical, converted.Operations); err == nil {
		t.Fatal("stale beforeValue bypassed concurrency check")
	}
}

func TestLegacyNodeOperationsTranslateOnlyKnownMetadataPaths(t *testing.T) {
	req := CanvasOperationsRequest{Operations: []CanvasOperation{{Type: "patch_node", ID: "shot-node", Patch: map[string]json.RawMessage{"metadata.directorShotId": json.RawMessage(`"shot-2"`), "metadata.plugin.directorShotId": json.RawMessage(`"unchanged"`)}, Before: map[string]json.RawMessage{"metadata.directorShotId": json.RawMessage(`"shot-1"`)}}}}
	converted, changed, err := migratePrevisOperations(req)
	if err != nil || !changed {
		t.Fatal(err)
	}
	if string(converted.Operations[0].Patch["metadata.previsShotId"]) != `"shot-2"` || string(converted.Operations[0].Patch["metadata.plugin.directorShotId"]) != `"unchanged"` {
		t.Fatal("incorrect path mapping")
	}
	if _, exists := req.Operations[0].Patch["metadata.previsShotId"]; exists {
		t.Fatal("original request changed")
	}
	req.Operations[0].Patch["metadata.previsShotId"] = json.RawMessage(`"conflict"`)
	if _, _, err := migratePrevisOperations(req); err == nil {
		t.Fatal("ambiguous paths accepted")
	}
}

func TestLegacyCreateCanvasAndWholeNodeContracts(t *testing.T) {
	original, _ := legacyOperationFixture(t)
	var nodes []json.RawMessage
	_ = json.Unmarshal(original["nodes"], &nodes)
	req := CanvasOperationsRequest{Operations: []CanvasOperation{{Type: "create_canvas", Patch: map[string]json.RawMessage{"directorScenes": original["directorScenes"]}}, {Type: "create_node", Node: nodes[0]}, {Type: "remove_node", Expected: nodes[0]}}}
	converted, changed, err := migratePrevisOperations(req)
	if err != nil || !changed {
		t.Fatal(err)
	}
	if _, exists := converted.Operations[0].Patch["directorScenes"]; exists {
		t.Fatal("old create field remains")
	}
	for _, raw := range []json.RawMessage{converted.Operations[1].Node, converted.Operations[2].Expected} {
		var node map[string]any
		_ = json.Unmarshal(raw, &node)
		if node["metadata"].(map[string]any)["previsSceneId"] != "scene-1" {
			t.Fatal("whole node was not converted")
		}
	}
	req.Operations[0].Patch["previsScenes"] = json.RawMessage(`[]`)
	if _, _, err := migratePrevisOperations(req); err == nil {
		t.Fatal("ambiguous create fields accepted")
	}
}
