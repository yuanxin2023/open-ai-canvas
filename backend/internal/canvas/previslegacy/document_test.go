package previslegacy

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/director.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func decoded(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestNormalizePreservesLegacyContent(t *testing.T) {
	raw := fixture(t)
	before := append([]byte(nil), raw...)
	normalized, err := Normalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, before) {
		t.Fatal("input was mutated")
	}
	doc, original := decoded(t, normalized), decoded(t, raw)
	if _, exists := doc["directorScenes"]; exists {
		t.Fatal("old field remains")
	}
	scene := doc["previsScenes"].([]any)[0].(map[string]any)
	old := original["directorScenes"].([]any)[0].(map[string]any)
	for _, field := range []string{"id", "cameras", "lights", "activeShotId", "createdAt", "updatedAt"} {
		if !reflect.DeepEqual(scene[field], old[field]) {
			t.Errorf("changed %s", field)
		}
	}
	for _, actor := range scene["objects"].([]any) {
		delete(actor.(map[string]any), "legacyDirector")
	}
	if !reflect.DeepEqual(scene["objects"], old["objects"]) {
		t.Fatal("model/transform/keyframes changed")
	}
	shot := scene["shots"].([]any)[0].(map[string]any)
	if shot["aspectRatio"] != "16:9" {
		t.Fatal("wrong adopted aspect")
	}
	delete(shot, "aspectRatio")
	if !reflect.DeepEqual(scene["shots"], old["shots"]) {
		t.Fatal("shot/output references changed")
	}
	if !reflect.DeepEqual(doc["custom"], original["custom"]) {
		t.Fatal("unknown content changed")
	}
	metadata := doc["nodes"].([]any)[0].(map[string]any)["metadata"].(map[string]any)
	if metadata["previsSceneId"] != "scene-1" || metadata["previsClayVideoNodeId"] != "clay-node" {
		t.Fatal("node references missing")
	}
	if metadata["thirdParty"].(map[string]any)["directorSceneId"] != "plugin-data" {
		t.Fatal("third party data changed")
	}
	again, err := Normalize(normalized)
	if err != nil || !bytes.Equal(again, normalized) {
		t.Fatal("normalization is not idempotent", err)
	}
}

func TestNormalizeRejectsAmbiguousOrBrokenLegacyDocuments(t *testing.T) {
	cases := map[string]func(map[string]any){
		"two scene fields disagree": func(doc map[string]any) { doc["previsScenes"] = []any{} },
		"metadata disagrees": func(doc map[string]any) {
			doc["nodes"].([]any)[0].(map[string]any)["metadata"].(map[string]any)["previsSceneId"] = "another"
		},
		"unknown version": func(doc map[string]any) { doc["directorScenes"].([]any)[0].(map[string]any)["version"] = 2 },
		"missing camera":  func(doc map[string]any) { doc["directorScenes"].([]any)[0].(map[string]any)["cameras"] = []any{} },
		"missing output":  func(doc map[string]any) { doc["nodes"] = doc["nodes"].([]any)[:1] },
		"missing scene":   func(doc map[string]any) { doc["directorScenes"] = []any{} },
		"duplicate scene": func(doc map[string]any) {
			scenes := doc["directorScenes"].([]any)
			doc["directorScenes"] = append(scenes, scenes[0])
		},
		"null nodes": func(doc map[string]any) { doc["nodes"] = nil },
		"non string reference": func(doc map[string]any) {
			doc["nodes"].([]any)[0].(map[string]any)["metadata"].(map[string]any)["directorShotId"] = 12
		},
		"existing marker conflict": func(doc map[string]any) {
			doc["directorScenes"].([]any)[0].(map[string]any)["objects"].([]any)[0].(map[string]any)["legacyDirector"] = false
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			doc := decoded(t, fixture(t))
			mutate(doc)
			raw, _ := json.Marshal(doc)
			original := append([]byte(nil), raw...)
			result, err := Normalize(raw)
			if _, ok := err.(*Error); !ok || result != nil || !bytes.Equal(raw, original) {
				t.Fatalf("must reject without modifying input: %v", err)
			}
		})
	}
}

func TestNormalizeAcceptsIdenticalDualFieldsAndNativeDocuments(t *testing.T) {
	doc := decoded(t, fixture(t))
	doc["previsScenes"] = doc["directorScenes"]
	raw, _ := json.Marshal(doc)
	if _, err := Normalize(raw); err != nil {
		t.Fatal(err)
	}
	native := []byte(`{"id":"native","nodes":[],"previsScenes":[],"custom":{"directorSceneId":"leave"}}`)
	result, err := Normalize(native)
	if err != nil || !bytes.Equal(result, native) {
		t.Fatal("native document changed", err)
	}
}

func TestDualFieldComparisonUsesExactNumericValues(t *testing.T) {
	if !equal(json.RawMessage(`{"focalLength":35,"scale":[1e0,1,1]}`), json.RawMessage(`{"scale":[1,1.0,1],"focalLength":35.0}`)) {
		t.Fatal("equivalent JSON numbers were treated as conflicting")
	}
	if equal(json.RawMessage(`9007199254740992`), json.RawMessage(`9007199254740993`)) {
		t.Fatal("large integer conflict was rounded away")
	}
}
