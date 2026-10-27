package app

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func newTestFeedbackService(t *testing.T) (*Service, *model.User, *model.User, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "feedback.db")+"?_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.SystemSetting{}, &model.AdminAuditEvent{}, &model.Resource{}, &model.UserFeedback{}, &model.FeedbackResourceReference{}); err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	svc := &Service{repo: repo}
	admin := &model.User{ID: "admin_1", Username: "admin", Role: "admin", AdminLevel: model.AdminLevelFull, Status: "active", CreatedAt: time.Now()}
	user := &model.User{ID: "user_1", Username: "testuser", Email: "test@example.com", Role: "user", Status: "active", CreatedAt: time.Now()}
	for _, value := range []any{admin, user, &model.SystemSetting{Key: "feature_availability", ValueJSON: `{"userFeedbackEnabled":true}`}, &model.Resource{ID: "owned-image", UserID: user.ID, Kind: "image", MimeType: "image/png", Status: model.ResourceStatusReady, Size: 1024}} {
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	return svc, admin, user, db
}

func feedbackTestRequest() SubmitFeedbackRequest {
	return SubmitFeedbackRequest{RequestID: "feedback-request-0001", Type: model.FeedbackTypeBug, Title: "视频下载按钮未响应", Content: "点击下载按钮无反应", Attachments: `[{"id":"owned-image","name":"screen.png","url":"https://untrusted.example/ignored","type":"video","size":1}]`, Environment: `{"userAgent":"Chrome","pathname":"/canvas/private?token=secret"}`}
}

func TestFeedbackSubmitReplayOwnershipAndReferences(t *testing.T) {
	svc, _, user, db := newTestFeedbackService(t)
	req := feedbackTestRequest()
	fb, err := svc.SubmitFeedback(user, req)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := svc.SubmitFeedback(user, req)
	if err != nil || replay.ID != fb.ID {
		t.Fatalf("replay: %#v %v", replay, err)
	}
	list, err := svc.MyFeedbacks(user)
	if err != nil || len(list) != 1 {
		t.Fatalf("duplicate submission: %d, %v", len(list), err)
	}
	var items []model.FeedbackAttachment
	if err := json.Unmarshal([]byte(fb.Attachments), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Size != 1024 || items[0].Type != "image" || items[0].ResourceID != "owned-image" {
		t.Fatalf("untrusted attachment metadata retained: %#v", items)
	}
	req.Content = "different"
	_, err = svc.SubmitFeedback(user, req)
	var appErr *kernel.AppError
	if !errors.As(err, &appErr) || appErr.Reason != "feedback_submission_conflict" {
		t.Fatalf("key reuse accepted: %v", err)
	}
	stranger := &model.User{ID: "other", Role: "user", Status: "active"}
	req.RequestID = "feedback-request-0002"
	if _, err := svc.SubmitFeedback(stranger, req); err == nil {
		t.Fatal("foreign attachment accepted")
	}
	own, err := svc.MyFeedbacks(stranger)
	if err != nil || len(own) != 0 {
		t.Fatalf("cross-account list: %v %v", own, err)
	}
	if err := db.Where("id = ?", "owned-image").Delete(&model.Resource{}).Error; err == nil {
		t.Fatal("FK did not protect attached resource")
	}
}

func TestFeedbackPermissionsVersionAndArchivePreserveReply(t *testing.T) {
	svc, admin, user, db := newTestFeedbackService(t)
	fb, err := svc.SubmitFeedback(user, feedbackTestRequest())
	if err != nil {
		t.Fatal(err)
	}
	scoped := &model.User{ID: "scoped", Role: "admin", AdminLevel: model.AdminLevelScoped}
	if _, err := svc.AdminListFeedbacks(scoped, repository.FeedbackFilter{}); err == nil {
		t.Fatal("ungranted admin can read")
	}
	if _, err := svc.OpenFeedbackAttachment(scoped, fb.ID, "owned-image", ""); err == nil {
		t.Fatal("ungranted admin can read attachment")
	}
	if _, err := svc.OpenFeedbackAttachment(&model.User{ID: "stranger"}, fb.ID, "owned-image", ""); err == nil {
		t.Fatal("stranger can read attachment")
	}
	scoped.AdminPermissions = []model.AdminPermission{model.AdminPermissionFeedbacks}
	if list, err := svc.AdminListFeedbacks(scoped, repository.FeedbackFilter{}); err != nil || list.Total != 1 {
		t.Fatalf("granted admin: %v", err)
	}
	reply := "已处理"
	updated, err := svc.AdminUpdateFeedback(admin, fb.ID, AdminUpdateFeedbackRequest{ExpectedVersion: fb.Version, Status: model.FeedbackStatusResolved, AdminReply: &reply})
	if err != nil || updated.Version != 2 {
		t.Fatalf("reply: %v %v", updated, err)
	}
	if _, err := svc.AdminUpdateFeedback(admin, fb.ID, AdminUpdateFeedbackRequest{ExpectedVersion: 1, Status: model.FeedbackStatusClosed}); err == nil {
		t.Fatal("stale update accepted")
	}
	archived, err := svc.AdminArchiveFeedback(admin, fb.ID, true, 2)
	if err != nil || !archived.Archived || archived.AdminReply != reply || archived.Version != 3 {
		t.Fatalf("archive lost reply: %v %v", archived, err)
	}
	if err := svc.AdminDeleteFeedback(admin, fb.ID, 2); err == nil {
		t.Fatal("stale delete accepted")
	}
	if err := svc.AdminDeleteFeedback(admin, fb.ID, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.repo.ResourceForUser(user.ID, "owned-image"); err != nil {
		t.Fatal("feedback deletion deleted media")
	}
	var count int64
	if err := db.Model(&model.AdminAuditEvent{}).Count(&count).Error; err != nil || count != 3 {
		t.Fatalf("audit: %d %v", count, err)
	}
}

func TestFeedbackClosedFeatureAndInvalidInputsRejectWrites(t *testing.T) {
	svc, _, user, db := newTestFeedbackService(t)
	for _, mutate := range []func(*SubmitFeedbackRequest){
		func(req *SubmitFeedbackRequest) { req.Attachments = "invalid" },
		func(req *SubmitFeedbackRequest) { req.Environment = `{"currentUrl":"https://host/?key=secret"}` },
		func(req *SubmitFeedbackRequest) { req.Type = "invalid" },
		func(req *SubmitFeedbackRequest) { req.RequestID = "" },
		func(req *SubmitFeedbackRequest) { req.Attachments = `[{"id":"owned-image"},{"id":"owned-image"}]` },
	} {
		req := feedbackTestRequest()
		mutate(&req)
		if _, err := svc.SubmitFeedback(user, req); err == nil {
			t.Fatalf("invalid input accepted: %#v", req)
		}
	}
	if err := db.Model(&model.SystemSetting{}).Where("key = ?", "feature_availability").Update("value_json", `{"userFeedbackEnabled":false}`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitFeedback(user, feedbackTestRequest()); err == nil {
		t.Fatal("closed feature accepted submit")
	}
	if _, err := svc.UploadFeedbackAttachment(user, nil); err == nil {
		t.Fatal("closed feature accepted upload")
	}
	if list, err := svc.MyFeedbacks(user); err != nil || len(list) != 0 {
		t.Fatalf("closed history: %v %v", list, err)
	}
}
