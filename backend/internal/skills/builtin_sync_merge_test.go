package skills

import (
	"os"
	"path/filepath"
	"testing"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func builtinMergeTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	svc, db := newSkillLibraryCategoryTestService(t)
	if err := db.AutoMigrate(&model.SkillVersion{}, &model.SkillFile{}, &model.BuiltinSkillTombstone{}); err != nil {
		t.Fatal(err)
	}
	return svc, db
}

func TestBuiltinSyncPreservesLegacyVersionAndExplicitAvailability(t *testing.T) {
	svc, db := builtinMergeTestService(t)
	const id = "16000000000001"
	const previousBody = "# Existing local workflow\n\nKeep this historical body.\n"
	old := model.Skill{ID: id, Name: "Existing local workflow", Description: "Existing description", Instruction: previousBody, Source: model.SkillSourcePlatform, OwnerID: "old-source-owner", Tag: "social", Status: model.SkillStatusEnabled}
	for _, row := range []any{
		&old,
		&model.SkillPlatformState{SkillID: id, Available: false},
		&model.UserSkillState{ID: "relationship", UserID: "reader", SkillID: id, Added: true, Liked: true, LibraryCategoryID: "personal-category"},
		&model.Skill{ID: "private-copy", Name: "Private copy", Instruction: "User-owned content", OwnerID: "reader", Source: model.SkillSourceUser, SourceType: "zip", Status: model.SkillStatusEnabled, IsPrivate: true, CurrentVersionID: "copy-version"},
		&model.BuiltinSkillTombstone{SkillID: "16000000000002"},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.EnsureBuiltinSkills(); err != nil {
		t.Fatal(err)
	}
	var versions []model.SkillVersion
	if err := db.Where("skill_id = ?", id).Find(&versions).Error; err != nil || len(versions) != 2 {
		t.Fatalf("legacy and new versions must coexist: %+v / %v", versions, err)
	}
	found := false
	for _, version := range versions {
		body, err := svc.readSkillArchiveEntry(&version, "SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		found = found || string(body) == previousBody
	}
	if !found {
		t.Fatal("old body was replaced without preserving its immutable archive")
	}
	var current model.Skill
	if err := db.First(&current, "id = ?", id).Error; err != nil || current.Tag != "social" || current.OwnerID != "old-source-owner" {
		t.Fatalf("catalog refresh changed the deployed tag or ownership: %+v / %v", current, err)
	}
	var newStates int64
	if err := db.Model(&model.SkillPlatformState{}).Where("available = ?", false).Count(&newStates).Error; err != nil || newStates != 38 {
		t.Fatalf("new optional skills or old hidden state lost: %d / %v", newStates, err)
	}
	if err := db.Model(&model.SkillPlatformState{}).Where("skill_id = ?", "16000000000073").Update("available", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureBuiltinSkills(); err != nil {
		t.Fatal(err)
	}
	if available, err := svc.repo.PlatformSkillAvailable(model.Skill{ID: "16000000000073", Source: model.SkillSourcePlatform}); err != nil || !available {
		t.Fatalf("restart reset an explicit administrator enable: %v / %v", available, err)
	}
	var versionCount int64
	if err := db.Model(&model.SkillVersion{}).Where("skill_id = ?", id).Count(&versionCount).Error; err != nil || versionCount != 2 {
		t.Fatalf("unchanged restart created extra versions: %d / %v", versionCount, err)
	}
	var relationship model.UserSkillState
	if err := db.First(&relationship, "id = ?", "relationship").Error; err != nil || !relationship.Added || !relationship.Liked || relationship.LibraryCategoryID != "personal-category" {
		t.Fatalf("installed relationship changed: %+v / %v", relationship, err)
	}
	var deleted int64
	if err := db.Model(&model.Skill{}).Where("id = ?", "16000000000002").Count(&deleted).Error; err != nil || deleted != 0 {
		t.Fatal("deleted built-in skill was resurrected")
	}
	var copy model.Skill
	if err := db.First(&copy, "id = ?", "private-copy").Error; err != nil || copy.Instruction != "User-owned content" || copy.CurrentVersionID != "copy-version" || !copy.IsPrivate {
		t.Fatalf("user copy changed: %+v / %v", copy, err)
	}
}

func TestBuiltinIdentityConflictsRollBackCatalogAndNewArchives(t *testing.T) {
	for _, collision := range []model.Skill{
		{ID: "yingce-image-editing", Name: "Old official identity", Source: model.SkillSourcePlatform, Status: model.SkillStatusEnabled},
		{ID: "16000000000073", Name: "User identity collision", Source: model.SkillSourceUser, Status: model.SkillStatusEnabled, IsPrivate: true},
	} {
		t.Run(collision.ID, func(t *testing.T) {
			svc, db := builtinMergeTestService(t)
			if err := db.Create(&collision).Error; err != nil {
				t.Fatal(err)
			}
			if err := svc.EnsureBuiltinSkills(); err == nil {
				t.Fatal("ambiguous identity was overwritten")
			}
			for _, check := range []struct {
				row   any
				count int64
			}{{&model.Skill{}, 1}, {&model.SkillVersion{}, 0}, {&model.SkillPlatformState{}, 0}} {
				var count int64
				if err := db.Model(check.row).Count(&count).Error; err != nil || count != check.count {
					t.Fatalf("failed sync partially committed: %T / %d / %v", check.row, count, err)
				}
			}
			if err := filepath.WalkDir(svc.dataDir, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !entry.IsDir() && filepath.Ext(path) == ".zip" {
					t.Errorf("rolled-back archive remains: %s", filepath.Base(path))
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
