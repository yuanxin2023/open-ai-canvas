package app

import (
	"context"
	"encoding/base64"
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/protocol"
)

func TestOpenAIImageLocalReferencesWithoutPublicURL(t *testing.T) {
	service, _, imageBytes := cloudAgentVisionFixture(t)
	t.Setenv("CANVAS_PUBLIC_BASE_URL", "")
	input := canvasGenerationInput{
		Mode: "image", Prompt: "Keep the character consistent with @图片1",
		Config:          providerConfig{InterfaceType: string(model.ChannelInterfaceOpenAIImage), Model: "test-image"},
		ReferenceImages: []providerMedia{{ID: "character", StorageKey: "resource:ref-one", DataURL: "data:image/png;base64,ZmFrZQ=="}},
	}
	ctx := ensureOfficialProtocolAdapter(context.Background(), input.Config.InterfaceType)
	policy := providerMediaHydrationPolicyFor(ctx, input)
	if policy.requireURL {
		t.Fatal("local OpenAI image references must not require a public URL")
	}
	if !policy.preferURL || !policy.imageOnly || !policy.ownedInlineOnly || policy.maxBytes <= 0 {
		t.Fatal("OpenAI references must prefer URLs and enforce the image boundary")
	}
	if err := service.hydrateGenerationMedia("user", &input, policy); err != nil {
		t.Fatal(err)
	}
	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageBytes)
	if input.ReferenceImages[0].DataURL != want || input.ReferenceImages[0].URL != "" {
		t.Fatal("reference did not hydrate from the authorized local resource")
	}
	adapter, ok := declarativeProtocolAdapterForContext(ctx, input.Config.InterfaceType)
	if !ok {
		t.Fatal("OpenAI image adapter is missing")
	}
	request, err := adapter.BuildCreate(ctx, protocol.RequestContext{Request: protocolRequestFromInput(input)})
	if err != nil {
		t.Fatal(err)
	}
	images := creationMaps(request.Body.(map[string]any)["images"])
	if request.Path != "/v1/images/edits" || len(images) != 1 || images[0]["image_url"] != want {
		t.Fatal("image edit request did not retain inline image bytes")
	}
	foreign := canvasGenerationInput{ReferenceImages: []providerMedia{{StorageKey: "resource:ref-one"}}}
	if err := service.hydrateGenerationMedia("other-user", &foreign, policy); err == nil {
		t.Fatal("another user's reference resource was accepted")
	}
	bare := canvasGenerationInput{ReferenceImages: []providerMedia{{DataURL: want}}}
	if err := service.hydrateGenerationMedia("user", &bare, policy); err == nil {
		t.Fatal("untracked inline bytes bypassed resource ownership")
	}
}

func TestOpenAIImageInlineReferencesEnforceStoredResourceBoundary(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		updates  map[string]any
		maxBytes int64
	}{
		{"not ready", map[string]any{"status": "uploading"}, 0},
		{"not an image", map[string]any{"mime_type": "video/mp4"}, 0},
		{"exceeds limit", nil, 1},
		{"unreadable object cannot use injected bytes", map[string]any{"provider": "not-a-provider"}, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			service, db, _ := cloudAgentVisionFixture(t)
			if scenario.updates != nil {
				if err := db.Model(&model.Resource{}).Where("id = ?", "ref-one").Updates(scenario.updates).Error; err != nil {
					t.Fatal(err)
				}
			}
			input := canvasGenerationInput{Mode: "image", Config: providerConfig{InterfaceType: "openai-image", Model: "image"}, ReferenceImages: []providerMedia{{StorageKey: "resource:ref-one", DataURL: "data:image/png;base64,ZmFrZQ=="}}}
			policy := providerMediaHydrationPolicyFor(ensureOfficialProtocolAdapter(context.Background(), "openai-image"), input)
			if scenario.maxBytes > 0 {
				policy.maxBytes = scenario.maxBytes
			}
			if err := service.hydrateGenerationMedia("user", &input, policy); err == nil {
				t.Fatal("invalid stored resource was accepted")
			}
		})
	}
}
