package skills

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func TestAdminManagementUsesOfficialStatusAndPreservesUserRelations(t *testing.T) {
	svc, db, actor := newCurationTest(t)
	row := model.Skill{ID: "managed", OwnerID: "reader", Name: "Managed skill", Description: "Description", Tag: "drama", Source: model.SkillSourceUser, SourceType: "markdown", Status: skillStatusEnabled}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	relationship := model.UserSkillState{ID: "joined", UserID: "reader", SkillID: row.ID, Added: true, Liked: true}
	if err := db.Create(&relationship).Error; err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		item, err := svc.AdminSetSkillEnabled(actor, row.ID, enabled)
		if err != nil || item.Enabled != enabled {
			t.Fatalf("set status %v: %#v / %v", enabled, item, err)
		}
		_, readErr := svc.SkillDetail("reader", row.ID)
		if enabled != (readErr == nil) {
			t.Fatalf("detail availability did not follow official status: %v / %v", enabled, readErr)
		}
		var saved model.UserSkillState
		if err := db.First(&saved, "id = ?", relationship.ID).Error; err != nil || !saved.Added || !saved.Liked {
			t.Fatalf("status toggle changed installed relationship: %#v / %v", saved, err)
		}
	}
	if err := db.Create(&model.SkillPlatformState{SkillID: row.ID, Available: false}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SkillDetail("reader", row.ID); err != nil {
		t.Fatalf("retired local state still overrides official status: %v", err)
	}
}

func TestBuiltinSyncPreservesOfficialAdminEditFlags(t *testing.T) {
	svc, db := builtinMergeTestService(t)
	if err := svc.EnsureBuiltinSkills(); err != nil {
		t.Fatal(err)
	}
	var before model.Skill
	if err := db.First(&before, "id = ?", "16000000000001").Error; err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().UTC()
	if err := db.Model(&before).Updates(map[string]any{"name": "Administrator name", "instruction": "Administrator content", "admin_info_edited_at": stamp, "admin_content_edited_at": stamp}).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureBuiltinSkills(); err != nil {
		t.Fatal(err)
	}
	var after model.Skill
	if err := db.First(&after, "id = ?", before.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Name != "Administrator name" || after.Instruction != "Administrator content" || after.CurrentVersionID != before.CurrentVersionID {
		t.Fatalf("embedded sync overwrote administrator edits: %#v", after)
	}
}
