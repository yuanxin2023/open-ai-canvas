package app

import "testing"

func TestCanvasAppearanceBuiltinAvatarPreservesExistingChoices(t *testing.T) {
	if defaultCanvasAppearance().AvatarType != "bundle" {
		t.Fatal("new installations must use the built-in mascot")
	}
	for _, avatarType := range []string{"bundle", "orb"} {
		value := defaultCanvasAppearance()
		value.AvatarType = avatarType
		value.AgentName = "自有助手"
		value.Live2DResourceID = "retained-model"
		value.Live2DEntry = "model.model3.json"
		actual, err := normalizeCanvasAppearance(value)
		if err != nil || actual != value {
			t.Fatalf("existing identity or model was overwritten for %s: %+v %v", avatarType, actual, err)
		}
	}
}

func TestAppearanceBuiltinAvatarRoundTrip(t *testing.T) {
	svc, _, _, admin := newAppearanceTestService(t)
	value := defaultAppearanceSetting()
	value.BrandName = "自有工作台"
	value.Canvas.AgentName = "自有助手"
	value.Canvas.AvatarType = "bundle"
	value.ComposerGlowColor = "#123456"
	if _, err := svc.UpdateAppearance(admin, value); err != nil {
		t.Fatal(err)
	}
	public, err := svc.Appearance()
	if err != nil {
		t.Fatal(err)
	}
	if public.Canvas.AvatarType != "bundle" || public.BrandName != value.BrandName || public.Canvas.AgentName != value.Canvas.AgentName || public.ComposerGlowColor != value.ComposerGlowColor {
		t.Fatalf("built-in avatar save changed unrelated presentation: %+v", public)
	}
}
