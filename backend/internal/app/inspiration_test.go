package app

import (
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNormalizeInspirationRequest(t *testing.T) {
	request := InspirationRequest{
		Title:       "  测试灵感  ",
		Description: "  卡片说明  ",
		Mode:        model.InspirationModeImage,
		Prompt:      "  生成一张电影感海报  ",
		Tags:        []string{" 海报 ", "海报", "CINEMA", "cinema"},
		CoverURL:    "https://example.com/cover.webp",
		CoverWidth:  900,
		CoverHeight: 1200,
	}
	normalized, tagsJSON, err := normalizeInspirationRequest(request, false)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Title != "测试灵感" || normalized.Description != "卡片说明" || normalized.Prompt != "生成一张电影感海报" {
		t.Fatalf("text fields were not normalized: %+v", normalized)
	}
	if len(normalized.Tags) != 2 || normalized.Tags[0] != "海报" || normalized.Tags[1] != "CINEMA" {
		t.Fatalf("tags were not trimmed and deduplicated: %+v", normalized.Tags)
	}
	if tagsJSON != `["海报","CINEMA"]` {
		t.Fatalf("unexpected tags JSON: %s", tagsJSON)
	}

	request.CoverURL = "/short-drama-styles/legacy.jpg"
	if _, _, err := normalizeInspirationRequest(request, false); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("new inspirations must reject built-in relative covers: %v", err)
	}
	if _, _, err := normalizeInspirationRequest(request, true); err != nil {
		t.Fatalf("existing migrated cover should remain editable: %v", err)
	}
	request.CoverWidth = 0
	if _, _, err := normalizeInspirationRequest(request, true); err == nil || !strings.Contains(err.Error(), "尺寸") {
		t.Fatalf("inspirations must require confirmed cover dimensions: %v", err)
	}
}

func TestInspirationCreateEnableAndOrderLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Inspiration{}, &model.InspirationCoverDraft{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	svc := New(repository.New(db), t.TempDir())
	admin := &model.User{ID: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	user := &model.User{ID: "user", Role: model.UserRoleUser, Status: model.UserStatusActive}
	first, err := svc.CreateInspiration(admin, InspirationRequest{Title: "第一条", Description: "卡片说明", Mode: model.InspirationModeImage, Prompt: "生成提示词", CoverURL: "https://example.com/one.webp", CoverWidth: 900, CoverHeight: 1200, Tags: []string{" 海报 ", "海报"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != model.InspirationStatusDisabled || len(first.Tags) != 1 {
		t.Fatalf("unexpected created inspiration: %+v", first)
	}
	active, err := svc.Inspirations(user)
	if err != nil || len(active) != 0 {
		t.Fatalf("disabled inspiration leaked: %+v, %v", active, err)
	}
	if _, err := svc.SetInspirationStatus(admin, first.ID, model.InspirationStatusActive); err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateInspiration(admin, InspirationRequest{Title: "第二条", Description: "另一张卡片", Mode: model.InspirationModeText, Prompt: "写一段文字", CoverURL: "https://example.com/two.webp", CoverWidth: 1600, CoverHeight: 900})
	if err != nil {
		t.Fatal(err)
	}
	order, err := svc.InspirationOrder(admin)
	if err != nil || len(order) != 2 {
		t.Fatalf("order = %+v, %v", order, err)
	}
	if err := svc.SaveInspirationOrder(admin, InspirationOrderRequest{IDs: []string{second.ID, first.ID}, ExpectedIDs: []string{first.ID, second.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveInspirationOrder(admin, InspirationOrderRequest{IDs: []string{first.ID, second.ID}, ExpectedIDs: []string{first.ID, second.ID}}); err == nil {
		t.Fatal("stale order snapshot should be rejected")
	}
}
