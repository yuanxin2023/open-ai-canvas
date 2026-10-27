package skills

import (
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func TestSkillClassificationRequiresCurrentModulePermission(t *testing.T) {
	svc, db, full := newCurationTest(t)
	for _, test := range []struct {
		name       string
		permission model.AdminPermission
		status     model.UserStatus
		allowed    bool
	}{
		{"skills", model.AdminPermissionSkills, model.UserStatusActive, true},
		{"other-module", model.AdminPermissionChannels, model.UserStatusActive, false},
		{"disabled", model.AdminPermissionSkills, model.UserStatusDisabled, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			actor := &model.User{ID: test.name, Username: test.name, Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{test.permission}, Status: test.status}
			if err := db.Create(actor).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.AdminPermissionGrant{UserID: actor.ID, Permission: test.permission}).Error; err != nil {
				t.Fatal(err)
			}
			_, readErr := svc.SkillCuration(actor, true)
			category, createErr := svc.CreateSkillLibraryCategory(actor, SkillLibraryCategoryMutationRequest{Name: test.name, Scope: "platform"})
			if test.allowed {
				if readErr != nil || createErr != nil {
					t.Fatalf("skill administrator denied: %v / %v", readErr, createErr)
				}
				// The cached actor still has its grant; the database now revokes it.
				if err := db.Where("user_id = ?", actor.ID).Delete(&model.AdminPermissionGrant{}).Error; err != nil {
					t.Fatal(err)
				}
				if err := svc.DeleteSkillLibraryCategory(actor, category.ID); appErrorStatus(err) != 403 {
					t.Fatalf("revoked grant accepted for deletion: %v", err)
				}
			} else if appErrorStatus(readErr) != 403 || appErrorStatus(createErr) != 403 {
				t.Fatalf("unauthorized administrator accepted: %v / %v", readErr, createErr)
			}
			var before model.SkillCurationSetting
			if err := db.First(&before, 1).Error; err != nil {
				t.Fatal(err)
			}
			if err := svc.repo.UpdateSkillCuration(actor.ID, before.Revision, "revoked", func(repo *repository.Repository) error { return repo.SetSkillCurationEnabled(true) }); appErrorStatus(err) != 403 {
				t.Fatalf("transaction did not recheck current module permission: %v", err)
			}
			var after model.SkillCurationSetting
			if err := db.First(&after, 1).Error; err != nil {
				t.Fatal(err)
			}
			var audits int64
			if err := db.Model(&model.AdminAuditEvent{}).Count(&audits).Error; err != nil {
				t.Fatal(err)
			}
			if after != before || audits != 0 {
				t.Fatal("denied change modified the curation revision, setting or audit")
			}
		})
	}
	if _, err := svc.SkillCuration(full, true); err != nil {
		t.Fatalf("full administrator denied: %v", err)
	}
}

func TestSkillClassificationPreservesLocalHiddenState(t *testing.T) {
	svc, db, _ := newCurationTest(t)
	if err := db.Model(&model.SkillCurationSetting{}).Where("id = 1").Update("enabled", true).Error; err != nil {
		t.Fatal(err)
	}
	category := model.SkillCurationCategory{ID: "child", RootTag: "drama", Name: "Story", NormalizedName: "story", Enabled: true}
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	for _, skill := range []model.Skill{
		{ID: "enabled", Name: "Enabled", Tag: "drama", Source: model.SkillSourcePlatform, Status: model.SkillStatusEnabled},
		{ID: "hidden", Name: "Hidden", Tag: "drama", Source: model.SkillSourcePlatform, Status: model.SkillStatusEnabled},
		{ID: "hidden-tag", Name: "Hidden tag", Tag: "creative", Source: model.SkillSourcePlatform, Status: model.SkillStatusEnabled},
		{ID: "personal-public", Name: "Personal public", Tag: "creative", Source: model.SkillSourceUser, Status: model.SkillStatusEnabled},
		{ID: "private", Name: "Private", Tag: "drama", Source: model.SkillSourceUser, IsPrivate: true, OwnerID: "other", Status: model.SkillStatusEnabled},
	} {
		for _, row := range []any{
			&skill,
			&model.SkillCurationRootAssignment{SkillID: skill.ID, RootID: "drama"},
			&model.SkillCurationAssignment{SkillID: skill.ID, CategoryID: category.ID},
			&model.UserSkillState{ID: "reader:" + skill.ID, UserID: "reader", SkillID: skill.ID, Added: true},
		} {
			if err := db.Create(row).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, row := range []any{
		&model.SkillPlatformState{SkillID: "hidden", Available: false},
		&model.SkillCategoryPlatformState{Tag: "creative", Available: false},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	public, err := svc.repo.SkillCuration(false)
	if err != nil || len(public.RootAssignments) != 2 || len(public.Assignments) != 2 {
		t.Fatalf("hidden/private skill associations leaked: %+v / %v", public, err)
	}
	counts, err := svc.repo.PublicSkillCategoryCounts()
	if err != nil || counts["drama"] != 1 || counts["creative"] != 1 {
		t.Fatalf("public counts diverged from local visibility: %+v / %v", counts, err)
	}
	library, err := svc.SkillLibraryCategories("reader", "mine")
	if err != nil || library.TotalCount != 2 {
		t.Fatalf("personal category counts leaked hidden/private entries: %+v / %v", library, err)
	}
	rows, count, err := svc.repo.Skills(repository.SkillListFilter{UserID: "reader", Scope: "mine", PlatformRootID: "drama", Limit: 20})
	if err != nil || count != 2 || len(rows) != 2 {
		t.Fatalf("moving a hidden skill to another root bypassed availability: %+v / %d / %v", rows, count, err)
	}
	for _, id := range []string{"hidden", "hidden-tag"} {
		if _, err := svc.SkillDetail("reader", id); appErrorStatus(err) != 403 {
			t.Fatalf("hidden skill detail accessible: %s / %v", id, err)
		}
		if _, err := svc.SetSkillLibraryCategory("reader", id, ""); appErrorStatus(err) != 403 {
			t.Fatalf("hidden skill assignment accepted: %s / %v", id, err)
		}
	}
}
