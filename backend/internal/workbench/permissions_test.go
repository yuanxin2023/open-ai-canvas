package workbench

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestWorkbenchAdminMethodsRequireIndependentPermissionBeforeDataAccess(t *testing.T) {
	gate := &allowAdmin{}
	svc := New(nil, gate, nil)
	checks := map[string]func(*model.User) error{
		"list":             func(actor *model.User) error { _, err := svc.AdminTools(actor, ""); return err },
		"create tool":      func(actor *model.User) error { _, err := svc.CreateTool(actor, ToolInput{}); return err },
		"update tool":      func(actor *model.User) error { _, err := svc.UpdateTool(actor, "tool", ToolInput{}); return err },
		"publish tool":     func(actor *model.User) error { return svc.SetToolEnabled(actor, "tool", true) },
		"unpublish tool":   func(actor *model.User) error { return svc.SetToolEnabled(actor, "tool", false) },
		"create preset":    func(actor *model.User) error { _, err := svc.CreatePreset(actor, PresetInput{}); return err },
		"update preset":    func(actor *model.User) error { _, err := svc.UpdatePreset(actor, "preset", PresetInput{}); return err },
		"publish preset":   func(actor *model.User) error { return svc.SetPresetEnabled(actor, "tool", "preset", true) },
		"unpublish preset": func(actor *model.User) error { return svc.SetPresetEnabled(actor, "tool", "preset", false) },
	}
	for _, actor := range []*model.User{
		nil,
		{ID: "user", Role: model.UserRoleUser},
		{ID: "other-admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionChannels, model.AdminPermissionSkills, model.AdminPermissionPromptTemplates}},
	} {
		for name, check := range checks {
			t.Run(name, func(t *testing.T) {
				if err := check(actor); err == nil {
					t.Fatal("expected denial before accessing the nil repository")
				}
			})
		}
	}
	if gate.audited != 0 {
		t.Fatal("denied operations produced success audit records")
	}

	svc, _, audit := newWorkbenchTest(t)
	for _, actor := range []*model.User{
		{ID: "full", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull},
		{ID: "tools", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionWorkbenchTools}},
	} {
		tool, err := svc.CreateTool(actor, ToolInput{Kind: KindImage, CategoryID: "recommended", Name: "Reviewed", Enabled: false})
		if err != nil {
			t.Fatal(err)
		}
		preset, err := svc.CreatePreset(actor, PresetInput{ToolID: tool.ID, Name: "Reviewed preset", PromptTemplate: "A scene", Enabled: false})
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.SetToolEnabled(actor, tool.ID, true); err != nil {
			t.Fatal(err)
		}
		if err := svc.SetPresetEnabled(actor, tool.ID, preset.ID, true); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.AdminTools(actor, KindImage); err != nil {
			t.Fatal(err)
		}
	}
	if audit.audited != 8 {
		t.Fatalf("audit count = %d, want 8", audit.audited)
	}
}
