package app

import (
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/protocol"
)

func TestRetiredAutoDLAudioModelKeepsConfigurationButIsUnavailable(t *testing.T) {
	registry, err := protocol.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{pluginRuntime: &pluginRuntime{registry: registry}}
	channel := model.ChannelModel{ID: "saved-audio", Protocol: "autodl-comfyui-audio", Capability: "audio", Enabled: true, PriceTiers: []model.ChannelModelPriceTier{{ID: "existing-price", BillingMode: "fixed_request", PriceConfigured: true, Enabled: true, UnitPriceMicrocredits: 10000, SelectorJSON: `{}`}}}
	public, err := service.sanitizeChannelModel(&channel)
	if err != nil {
		t.Fatal(err)
	}
	if public.Available || len(public.PriceTiers) != 1 {
		t.Fatalf("retired model availability or prices changed: %#v", public)
	}
	if channel.Protocol != "autodl-comfyui-audio" || !channel.Enabled || channel.PriceTiers[0].UnitPriceMicrocredits != 10000 {
		t.Fatal("read projection changed saved configuration")
	}
	if err := service.validateGenerationInterface("audio", "autodl-comfyui-audio"); err == nil {
		t.Fatal("retired audio execution was admitted")
	}
}
