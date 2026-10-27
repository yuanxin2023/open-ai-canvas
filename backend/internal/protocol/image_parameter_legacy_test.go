package protocol

import (
	"reflect"
	"testing"
)

func TestLegacyImageOptionsPreserveProtocolMeaning(t *testing.T) {
	for _, tc := range []struct {
		protocol    string
		input, want map[string]any
		invalid     bool
	}{
		{"openai-image", map[string]any{"quality": "2k", "imageResolution": "4k"}, map[string]any{"quality": "medium", "imageResolution": "4k"}, false},
		{"openai-image", map[string]any{"quality": "1k"}, map[string]any{"quality": "low"}, false},
		{"openai-image", map[string]any{"quality": "4k"}, map[string]any{"quality": "high"}, false},
		{"openai-image", map[string]any{"quality": "auto"}, map[string]any{"quality": "auto"}, false},
		{"grok-image", map[string]any{"quality": "2K"}, map[string]any{"imageResolution": "2k"}, false},
		{"grok-image", map[string]any{"quality": "2k", "imageResolution": "1k"}, nil, true},
		{"grok-image", map[string]any{"quality": "4k"}, map[string]any{"imageResolution": "2k"}, false},
		{"grok-image", map[string]any{"quality": "high"}, map[string]any{"imageResolution": "2k"}, false},
		{"gemini-image", map[string]any{"quality": "high"}, map[string]any{"imageResolution": "4k"}, false},
		{"gemini-image", map[string]any{"quality": "auto"}, map[string]any{"imageResolution": "auto"}, false},
		{"grok-image", map[string]any{"quality": "ultra"}, nil, true},
		{"grok-image", map[string]any{"quality": 2}, nil, true},
		{"grok-image", map[string]any{"quality": "auto", "imageResolution": "2k"}, map[string]any{"imageResolution": "2k"}, false},
		{"gemini-image", map[string]any{"quality": "4k"}, map[string]any{"imageResolution": "4k"}, false},
		{"subrouter-gemini-image", map[string]any{"quality": "2k", "rawMode": false, "seed": 0}, map[string]any{"imageResolution": "2k", "rawMode": false, "seed": 0}, false},
		{"agnes-image", map[string]any{"resolution": "1k"}, map[string]any{"imageResolution": "1k"}, false},
		{"km-kemei-seedream", map[string]any{"quality": "1.5k"}, map[string]any{"imageResolution": "1.5k"}, false},
		{"custom-seedream", map[string]any{"quality": "ultra"}, map[string]any{"quality": "ultra"}, false},
		{"grok-image", map[string]any{"quality": "*"}, map[string]any{"imageResolution": "*"}, false},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			before := map[string]any{}
			for k, v := range tc.input {
				before[k] = v
			}
			got, err := NormalizeLegacyImageOptions(tc.protocol, tc.input)
			if (err != nil) != tc.invalid {
				t.Fatalf("projection error=%v", err)
			}
			if !tc.invalid && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v want %#v", got, tc.want)
			}
			if !reflect.DeepEqual(tc.input, before) {
				t.Fatal("projection mutated saved input")
			}
		})
	}
}
