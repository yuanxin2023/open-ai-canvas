package skills

import (
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPlatformSkillAvailabilityValidatesRequestBeforeRepository(t *testing.T) {
	svc := &Service{}
	for name, request := range map[string]AdminSkillAvailabilityRequest{
		"empty":     {},
		"blank id":  {SkillIDs: []string{" "}},
		"duplicate": {SkillIDs: []string{"skill-a", " skill-a "}},
		"too many":  {SkillIDs: make([]string, maxAdminSkillBatch+1)},
	} {
		t.Run(name, func(t *testing.T) {
			if err := svc.SetPlatformSkillAvailability(request, "admin-1", nil); err == nil {
				t.Fatal("invalid request was accepted")
			}
		})
	}
	if err := svc.SetPlatformSkillCategoryAvailability("unknown", false, "admin-1", nil); err == nil || !strings.Contains(err.Error(), "未知") {
		t.Fatalf("unknown category err = %v", err)
	}
}

func TestPlatformSkillAvailabilityKeepsCategoryAndChildStateIndependent(t *testing.T) {
	svc, db := newAvailabilityTestService(t)
	seedAvailabilitySkills(t, db)

	catalog, err := svc.AdminSkills(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Skills) != 2 || !catalog.Skills[0].EffectiveAvailable || !catalog.Skills[1].EffectiveAvailable {
		t.Fatalf("default catalog = %#v", catalog)
	}

	if err := svc.SetPlatformSkillAvailability(AdminSkillAvailabilityRequest{SkillIDs: []string{"platform-b"}, Available: false}, "admin-1", nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetPlatformSkillCategoryAvailability("drama", false, "admin-1", nil); err != nil {
		t.Fatal(err)
	}
	catalog, err = svc.AdminSkills(true)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range catalog.Skills {
		if item.EffectiveAvailable {
			t.Fatalf("category-disabled skill remains effective: %#v", item)
		}
	}
	childState := map[string]bool{}
	for _, item := range catalog.Skills {
		childState[item.SkillID] = item.Available
	}
	if !childState["platform-a"] || childState["platform-b"] {
		t.Fatalf("category gate changed child states: %#v", catalog.Skills)
	}

	if err := svc.SetPlatformSkillCategoryAvailability("drama", true, "admin-1", nil); err != nil {
		t.Fatal(err)
	}
	catalog, err = svc.AdminSkills(true)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]AdminSkillItem{}
	for _, item := range catalog.Skills {
		byID[item.SkillID] = item
	}
	if !byID["platform-a"].EffectiveAvailable || byID["platform-b"].EffectiveAvailable {
		t.Fatalf("restored category did not preserve child states: %#v", byID)
	}
}

func TestPlatformSkillAvailabilityHidesPlatformSkillsButNotUserSkills(t *testing.T) {
	svc, db := newAvailabilityTestService(t)
	seedAvailabilitySkills(t, db)
	if err := svc.SetPlatformSkillCategoryAvailability("drama", false, "admin-1", nil); err != nil {
		t.Fatal(err)
	}

	list, err := svc.Skills("user-1", SkillListRequest{Scope: "public", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Skills) != 1 || list.Skills[0].SkillID != "user-public" {
		t.Fatalf("public list = %#v", list.Skills)
	}
	if _, err := svc.SkillDetail("user-1", "platform-a"); err == nil || err.Error() != "该平台技能已停用" {
		t.Fatalf("disabled platform skill detail err = %v", err)
	}
	if _, err := svc.SkillDetail("user-1", "user-private"); err != nil {
		t.Fatalf("owner could not read private user skill: %v", err)
	}
}

func TestPlatformSkillBatchRejectsUserSkillWithoutPartialUpdate(t *testing.T) {
	svc, db := newAvailabilityTestService(t)
	seedAvailabilitySkills(t, db)
	err := svc.SetPlatformSkillAvailability(AdminSkillAvailabilityRequest{SkillIDs: []string{"platform-a", "user-public"}, Available: false}, "admin-1", nil)
	if err == nil {
		t.Fatal("mixed platform/user batch was accepted")
	}
	var count int64
	if err := db.Model(&model.SkillPlatformState{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("mixed batch persisted %d partial states", count)
	}
}

func newAvailabilityTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.UserIdentity{}, &model.Skill{}, &model.SkillVersion{}, &model.SkillFile{}, &model.UserSkillState{}, &model.SkillPlatformState{}, &model.SkillCategoryPlatformState{}); err != nil {
		t.Fatal(err)
	}
	return New(repository.New(db), t.TempDir(), nil), db
}

func seedAvailabilitySkills(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	rows := []model.Skill{
		{ID: "platform-a", Name: "平台 A", Description: "A", AuthorName: "平台", VersionLabel: "1.0", Status: model.SkillStatusEnabled, Source: 3, Tag: "drama", CreatedAt: now, UpdatedAt: now},
		{ID: "platform-b", Name: "平台 B", Description: "B", AuthorName: "社区", VersionLabel: "1.0", Status: model.SkillStatusEnabled, Source: 3, Tag: "drama", CreatedAt: now, UpdatedAt: now.Add(time.Second)},
		{ID: "user-public", OwnerID: "user-1", Name: "用户公开", Description: "U", VersionLabel: "1.0", Status: model.SkillStatusEnabled, Source: model.SkillSourceUser, Tag: "drama", CreatedAt: now, UpdatedAt: now},
		{ID: "user-private", OwnerID: "user-1", Name: "用户私有", Description: "P", VersionLabel: "1.0", Status: model.SkillStatusEnabled, Source: model.SkillSourceUser, Tag: "drama", IsPrivate: true, CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
}
