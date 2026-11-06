package app

import (
	"testing"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

func TestOfficialAdminSkillOperationsRequireModulePermission(t *testing.T) {
	svc := &Service{}
	actor := &model.User{ID: "other-admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionChannels}}
	calls := []func() error{
		func() error { _, err := svc.AdminSkills(actor, AdminSkillListRequest{}); return err },
		func() error { _, err := svc.AdminSkillDetail(actor, "skill"); return err },
		func() error { _, err := svc.AdminUpdateSkillInfo(actor, "skill", AdminSkillInfoRequest{}); return err },
		func() error { _, err := svc.AdminSetSkillEnabled(actor, "skill", false); return err },
		func() error { return svc.AdminDeleteSkill(actor, "skill") },
		func() error { _, err := svc.AdminSkillPackageFiles(actor, "skill"); return err },
		func() error { _, err := svc.AdminSkillPackageFile(actor, "skill", "SKILL.md"); return err },
		func() error { _, _, _, err := svc.AdminSkillPackageRawFile(actor, "skill", "SKILL.md"); return err },
		func() error {
			_, err := svc.AdminEditSkillPackageFiles(actor, "skill", SkillPackageEditRequest{})
			return err
		},
	}
	for index, call := range calls {
		err, ok := call().(*AppError)
		if !ok || err.Reason != kernel.ReasonAdminPermission {
			t.Fatalf("operation %d did not deny before accessing data: %#v", index, err)
		}
	}
}
