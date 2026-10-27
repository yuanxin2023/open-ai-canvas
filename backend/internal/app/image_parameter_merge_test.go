package app

import (
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/protocol"
)

func TestImageMergeProjectsLegacyCapabilitiesWithoutChangingPriceMeaning(t *testing.T) {
	for _, tc := range []struct{ provider, legacy, field, expected string }{
		{"openai-image", "2k", "quality", "medium"},
		{"grok-image", "high", "imageResolution", "2k"},
		{"gemini-image", "medium", "imageResolution", "2k"},
		{"subrouter-gemini-image", "high", "imageResolution", "4k"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			image := &ImageCapabilityConfig{Quality: ImageQualityConfig{Supported: true, Values: []string{tc.legacy}, Default: tc.legacy}}
			if err := projectLegacyImageCapability(tc.provider, image); err != nil {
				t.Fatal(err)
			}
			actual := image.Quality.Default
			if tc.field == "imageResolution" {
				actual = image.Resolution.Default
				if image.Quality.Supported {
					t.Fatal("resolution leaked back into quality")
				}
			}
			if actual != tc.expected {
				t.Fatalf("got %s want %s", actual, tc.expected)
			}
		})
	}
}

func TestImageMergeRejectsAmbiguousCapabilityAndPriceConditions(t *testing.T) {
	image := &ImageCapabilityConfig{Quality: ImageQualityConfig{Supported: true, Values: []string{"2k"}, Default: "2k"}, Resolution: ImageResolutionConfig{Supported: true, Values: []string{"1k"}, Default: "1k"}}
	if err := projectLegacyImageCapability("grok-image", image); err == nil {
		t.Fatal("conflicting legacy declarations were silently chosen")
	}
	params := []protocol.ImageParameter{{Name: "imageResolution", Label: "Resolution", Type: "string", Values: []any{"1k", "2k"}, Pricing: true}}
	if _, err := normalizedImagePriceSelector(model.ChannelModel{Protocol: "grok-image"}, params, map[string]string{"quality": "2k", "imageResolution": "1k"}); err == nil {
		t.Fatal("ambiguous price accepted")
	}
	if matched, _ := matchSKUSelector(map[string]string{"imageResolution": "2k"}, map[string]string{"imageResolution": "1.5k"}); matched {
		t.Fatal("unconfigured 1.5K selected a neighboring price")
	}
}

func TestImageMergeUsesEquivalentPriceAndNeverFallsThroughInvalidTier(t *testing.T) {
	parameters := []protocol.ImageParameter{{Name: "quality", Label: "Quality", Type: "string", Values: []any{"low", "medium", "high"}, Pricing: true}}
	spec := CapabilitySpec{Capability: "image", ImageParameters: parameters}
	channel := model.ChannelModel{Protocol: "openai-image", Capability: "image", PriceTiers: []model.ChannelModelPriceTier{
		{ID: "exact", Enabled: true, PriceConfigured: true, SelectorJSON: `{"quality":"2k"}`, UnitPriceMicrocredits: 1_230_000},
		{ID: "fallback", Enabled: true, PriceConfigured: true, SelectorJSON: `{}`, UnitPriceMicrocredits: 10_000},
	}}
	intent := ModelRequestIntent{Capability: "image", Options: map[string]any{"quality": "medium"}}
	tier := channelModelPriceTierForIntent(channel, intent, spec)
	if tier == nil || tier.ID != "exact" || tier.UnitPriceMicrocredits != 1_230_000 {
		t.Fatalf("equivalent price not retained: %#v", tier)
	}
	if channel.PriceTiers[0].SelectorJSON != `{"quality":"2k"}` {
		t.Fatal("read projection rewrote the saved tier")
	}
	channel.PriceTiers[0].SelectorJSON = `{"quality":"ultra"}`
	if tier := channelModelPriceTierForIntent(channel, intent, spec); tier != nil {
		t.Fatalf("invalid tier fell through to another price: %#v", tier)
	}
}
