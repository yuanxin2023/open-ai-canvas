package prompts

import (
	"errors"
	"testing"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserPromptLifecycleIsScopedToOwner(t *testing.T) {
	svc := newUserPromptTestService(t)
	created, err := svc.CreateUserPrompt("user-1", UserPromptRequest{
		Title: "  雨夜霓虹  ", Description: "  电影感开场  ", Mode: model.InspirationModeImage,
		Prompt: "  雨夜街道，霓虹倒影  ", Tags: []string{" 霓虹 ", "电影感", "霓虹"}, Source: " 原创 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Title != "雨夜霓虹" || created.Prompt != "雨夜街道，霓虹倒影" || len(created.Tags) != 2 {
		t.Fatalf("prompt was not normalized: %+v", created)
	}

	page, err := svc.ListUserPrompts("user-1", "倒影", model.InspirationModeImage, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Prompts) != 1 || page.Prompts[0].ID != created.ID || len(page.Prompts[0].Tags) != 2 {
		t.Fatalf("owner prompt was not listed with decoded tags: %+v", page)
	}
	foreign, err := svc.ListUserPrompts("user-2", "", "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if foreign.Total != 0 || len(foreign.Prompts) != 0 {
		t.Fatalf("another user can see owner prompts: %+v", foreign)
	}

	if _, err := svc.UpdateUserPrompt("user-2", created.ID, UserPromptRequest{Title: "越权", Mode: model.InspirationModeText, Prompt: "越权修改"}); appErrorStatus(err) != 404 {
		t.Fatalf("cross-user update status = %d, error = %v", appErrorStatus(err), err)
	}
	updated, err := svc.UpdateUserPrompt("user-1", created.ID, UserPromptRequest{Title: "新版", Mode: model.InspirationModeVideo, Prompt: "缓慢推进镜头"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "新版" || updated.Mode != model.InspirationModeVideo {
		t.Fatalf("prompt was not updated: %+v", updated)
	}
	if err := svc.DeleteUserPrompt("user-2", created.ID); appErrorStatus(err) != 404 {
		t.Fatalf("cross-user delete status = %d, error = %v", appErrorStatus(err), err)
	}
	if err := svc.DeleteUserPrompt("user-1", created.ID); err != nil {
		t.Fatal(err)
	}
}

func TestUserPromptCoverMustBeOwnedReadyImage(t *testing.T) {
	svc, db := newUserPromptTestServiceWithDB(t)
	resources := []model.Resource{
		{ID: "ready-image", UserID: "user-1", Kind: "image", Status: model.ResourceStatusReady, MimeType: "image/webp"},
		{ID: "foreign-image", UserID: "user-2", Kind: "image", Status: model.ResourceStatusReady, MimeType: "image/png"},
		{ID: "ready-video", UserID: "user-1", Kind: "video", Status: model.ResourceStatusReady, MimeType: "video/mp4"},
	}
	if err := db.Create(&resources).Error; err != nil {
		t.Fatal(err)
	}
	base := UserPromptRequest{Title: "封面测试", Mode: model.InspirationModeImage, Prompt: "测试提示词"}
	for _, resourceID := range []string{"foreign-image", "ready-video", "missing"} {
		req := base
		req.CoverResourceID = resourceID
		if _, err := svc.CreateUserPrompt("user-1", req); appErrorStatus(err) != 400 {
			t.Fatalf("resource %q status = %d, error = %v", resourceID, appErrorStatus(err), err)
		}
	}
	valid := base
	valid.CoverResourceID = "ready-image"
	created, err := svc.CreateUserPrompt("user-1", valid)
	if err != nil || created.CoverResourceID != "ready-image" {
		t.Fatalf("owned image was rejected: prompt=%+v error=%v", created, err)
	}
}

func TestUserPromptValidationRejectsInvalidFields(t *testing.T) {
	svc := newUserPromptTestService(t)
	tests := []UserPromptRequest{
		{Mode: model.InspirationModeImage, Prompt: "正文"},
		{Title: "标题", Mode: "audio", Prompt: "正文"},
		{Title: "标题", Mode: model.InspirationModeText},
		{Title: "标题", Mode: model.InspirationModeText, Prompt: "正文", CoverURL: "http://example.com/cover.png"},
		{Title: "标题", Mode: model.InspirationModeText, Prompt: "正文", CoverResourceID: "resource", CoverURL: "https://example.com/cover.png"},
	}
	for index, req := range tests {
		if _, err := svc.CreateUserPrompt("user-1", req); appErrorStatus(err) != 400 {
			t.Fatalf("case %d status = %d, error = %v", index, appErrorStatus(err), err)
		}
	}
	if _, err := svc.ListUserPrompts("", "", "", 1, 20); appErrorStatus(err) != 401 {
		t.Fatalf("anonymous list status = %d, error = %v", appErrorStatus(err), err)
	}
}

func newUserPromptTestService(t *testing.T) *Service {
	t.Helper()
	svc, _ := newUserPromptTestServiceWithDB(t)
	return svc
}

func newUserPromptTestServiceWithDB(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+kernel.NewID()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.UserPrompt{}, &model.Resource{}); err != nil {
		t.Fatal(err)
	}
	return New(repository.New(db), nil), db
}

func appErrorStatus(err error) int {
	var appError *kernel.AppError
	if errors.As(err, &appError) {
		return appError.Status
	}
	return 0
}
