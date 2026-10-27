package app

import (
	"encoding/json"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestAppearanceVersion13PreservesExplicitIdentityAndVisuals(t *testing.T) {
	svc, db, _, admin := newAppearanceTestService(t)
	value := defaultAppearanceSetting()
	value.SchemaVersion = 13
	value.BrandName = string([]rune{24433, 31574})
	value.Canvas.AgentName = value.BrandName
	value.SEOTitle = value.BrandName
	value.ComposerGlowColor = "#123456"
	value.ComposerGlowEnabled = false
	value.ComposerGlowIntensity = 48
	value.ComposerGlowSize = 125
	value.ComposerGlowPositionX = 68
	value.ComposerGlowPositionY = -25
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	original := string(encoded)
	if err := db.Create(&model.SystemSetting{Key: appearanceSettingKey, ValueJSON: original}).Error; err != nil {
		t.Fatal(err)
	}
	current, err := svc.AdminAppearance(admin)
	if err != nil {
		t.Fatal(err)
	}
	if current.SchemaVersion != 14 || current.BrandName != value.BrandName || current.Canvas.AgentName != value.Canvas.AgentName || current.SEOTitle != value.SEOTitle {
		t.Fatalf("adding announcements repeated the old identity migration: %+v", current)
	}
	if current.Public.Updates.Enabled || len(current.Public.Updates.Releases) != 0 || current.ComposerGlowColor != value.ComposerGlowColor || current.ComposerGlowEnabled || current.ComposerGlowIntensity != 48 || current.ComposerGlowSize != 125 || current.ComposerGlowPositionX != 68 || current.ComposerGlowPositionY != -25 {
		t.Fatalf("existing visual settings or announcement defaults changed: %+v", current)
	}
	var stored model.SystemSetting
	if err := db.First(&stored, "key = ?", appearanceSettingKey).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ValueJSON != original {
		t.Fatal("reading a v13 setting rewrote the stored configuration")
	}
	saved, err := svc.UpdateAppearance(admin, current.AppearanceSetting)
	if err != nil {
		t.Fatal(err)
	}
	if saved.SchemaVersion != 14 || saved.BrandName != value.BrandName || saved.Updates.Enabled {
		t.Fatalf("explicit save lost existing values: %+v", saved)
	}
}

func TestAppearanceIgnoresExcludedPurchaseConfiguration(t *testing.T) {
	value := defaultAppearanceSetting()
	if err := json.Unmarshal([]byte(`{"redeemPurchaseUrl":"https://example.com/shop","skinId":"old","skinThemes":[]}`), &value); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"redeemPurchaseUrl", "skinId", "skinThemes"} {
		if strings.Contains(string(encoded), field) {
			t.Fatalf("excluded field retained: %s", field)
		}
	}
}

func TestAppearanceAnnouncementRequiresAppearancePermission(t *testing.T) {
	svc := &Service{}
	for _, actor := range []*model.User{
		{ID: "user", Role: model.UserRoleUser},
		{ID: "channel-admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionChannels}},
	} {
		if _, err := svc.AdminAppearance(actor); err == nil {
			t.Fatal("unauthorized appearance read accepted")
		}
		if _, err := svc.UpdateAppearance(actor, defaultAppearanceSetting()); err == nil {
			t.Fatal("unauthorized appearance write accepted")
		}
	}
}
