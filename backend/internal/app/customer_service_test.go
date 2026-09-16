package app

import "testing"

func TestCustomerServiceDefaultsKeepSupportAvailable(t *testing.T) {
	setting := defaultCustomerServiceSetting()
	if setting.SchemaVersion != 3 {
		t.Fatalf("expected schema version 3, got %d", setting.SchemaVersion)
	}
	if !setting.Enabled || !setting.DesktopEnabled || !setting.MobileEnabled {
		t.Fatalf("default visibility = %#v", setting)
	}
	if setting.Position != "bottom-right" || setting.DisplayType != "circle" || setting.Color != "#2563EB" || setting.Label != "联系客服" {
		t.Fatalf("default presentation = %#v", setting)
	}
	if setting.Draggable || setting.ButtonSize != 56 || setting.OffsetX != 24 || setting.OffsetY != 24 {
		t.Fatalf("default placement = %#v", setting)
	}
	public := publicCustomerServiceSetting(nil, setting)
	if public.Configured || public.Revision != "builtin" || public.ImageConfigured || public.ImageURL != "" {
		t.Fatalf("public defaults = %#v", public)
	}
}

func TestValidateCustomerServiceSettingRejectsUnsafeValues(t *testing.T) {
	valid := defaultCustomerServiceSetting()
	if err := validateCustomerServiceSetting(valid); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*CustomerServiceSetting)
	}{
		{name: "position", mutate: func(value *CustomerServiceSetting) { value.Position = "center" }},
		{name: "display type", mutate: func(value *CustomerServiceSetting) { value.DisplayType = "html" }},
		{name: "color", mutate: func(value *CustomerServiceSetting) { value.Color = "red" }},
		{name: "empty label", mutate: func(value *CustomerServiceSetting) { value.Label = "" }},
		{name: "missing custom image", mutate: func(value *CustomerServiceSetting) { value.DisplayType = "custom-image" }},
		{name: "offset", mutate: func(value *CustomerServiceSetting) { value.OffsetX = 7 }},
		{name: "button size", mutate: func(value *CustomerServiceSetting) { value.ButtonSize = 19 }},
		{name: "tutorial scheme", mutate: func(value *CustomerServiceSetting) { value.TutorialURL = "javascript:alert(1)" }},
		{name: "tutorial missing host", mutate: func(value *CustomerServiceSetting) { value.TutorialURL = "https:///guide" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := valid
			test.mutate(&value)
			if err := validateCustomerServiceSetting(value); err == nil {
				t.Fatalf("value accepted: %#v", value)
			}
		})
	}
}

func TestValidateCustomerServiceSettingAcceptsTutorialURL(t *testing.T) {
	value := defaultCustomerServiceSetting()
	value.TutorialURL = "https://docs.example.com/guide"
	if err := validateCustomerServiceSetting(value); err != nil {
		t.Fatalf("valid tutorial URL rejected: %v", err)
	}
	public := publicCustomerServiceSetting(nil, value)
	if public.TutorialURL != value.TutorialURL {
		t.Fatalf("public tutorial URL = %q", public.TutorialURL)
	}
}
