package app

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminSkillAvailabilityRequiresAdminAndWritesAudit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+newID()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.AdminAuditEvent{}, &model.User{}, &model.UserIdentity{}, &model.Skill{}, &model.UserSkillState{}, &model.SkillPlatformState{}, &model.SkillCategoryPlatformState{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create(&model.Skill{ID: "platform-a", Name: "平台技能", Description: "测试", AuthorName: "平台", Status: model.SkillStatusEnabled, Source: 3, Tag: "drama", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db)}
	ordinary := &model.User{ID: "user-1", Role: model.UserRoleUser}
	if _, err := svc.AdminInstallSkillUpload(ordinary, "markdown", nil, SkillInstallRequest{}); err == nil {
		t.Fatal("ordinary user reached platform skill upload")
	}
	if _, err := svc.AdminInstallGitHubSkill(ordinary, SkillGitHubInstallRequest{URL: "https://github.com/example/example"}); err == nil {
		t.Fatal("ordinary user reached platform GitHub install")
	}
	request := AdminSkillAvailabilityRequest{SkillIDs: []string{"platform-a"}, Available: false}
	if _, err := svc.UpdateAdminSkillAvailability(ordinary, request); err == nil {
		t.Fatal("ordinary user changed platform skill availability")
	}
	admin := &model.User{ID: "admin-1", Role: model.UserRoleAdmin}
	catalog, err := svc.UpdateAdminSkillAvailability(admin, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Skills) != 1 || catalog.Skills[0].EffectiveAvailable {
		t.Fatalf("updated catalog = %#v", catalog)
	}
	var audit model.AdminAuditEvent
	if err := db.First(&audit, "action = ?", "skill.availability.batch_update").Error; err != nil {
		t.Fatal(err)
	}
	if audit.ActorUserID != admin.ID || audit.TargetType != "skill" {
		t.Fatalf("audit = %#v", audit)
	}
}
