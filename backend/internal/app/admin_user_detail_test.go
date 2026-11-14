package app

import (
	"encoding/json"
	"errors"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestAdminUserDetailInviter(t *testing.T) {
	svc, db, actor := quotaTestService(t, "sqlite")
	assertInviter := func(t *testing.T, wantID, wantUsername string) {
		t.Helper()
		detail, err := svc.AdminUserDetail(actor, "one")
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(detail)
		if err != nil {
			t.Fatal(err)
		}
		var response map[string]json.RawMessage
		if err := json.Unmarshal(data, &response); err != nil {
			t.Fatal(err)
		}
		if wantID == "" {
			if detail.Inviter != nil || string(response["inviter"]) != "null" {
				t.Fatalf("expected explicit null inviter, got %s", response["inviter"])
			}
			return
		}
		if detail.Inviter == nil || detail.Inviter.ID != wantID || detail.Inviter.Username != wantUsername {
			t.Fatalf("inviter = %+v, want %s / %s", detail.Inviter, wantID, wantUsername)
		}
		var summary map[string]any
		if err := json.Unmarshal(response["inviter"], &summary); err != nil || len(summary) != 2 {
			t.Fatalf("expected only inviter identity summary, got %s: %v", response["inviter"], err)
		}
	}

	t.Run("missing profile remains absent", func(t *testing.T) {
		assertInviter(t, "", "")
		var count int64
		if err := db.Model(&model.ReferralProfile{}).Where("user_id = ?", "one").Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("detail read created referral profile: count=%d err=%v", count, err)
		}
	})
	t.Run("unbound profile", func(t *testing.T) {
		if err := db.Create(&model.ReferralProfile{UserID: "one", Code: "ABC234"}).Error; err != nil {
			t.Fatal(err)
		}
		assertInviter(t, "", "")
	})
	t.Run("bound inviter including disabled account", func(t *testing.T) {
		if err := db.Model(&model.ReferralProfile{}).Where("user_id = ?", "one").Update("inviter_id", "two").Error; err != nil {
			t.Fatal(err)
		}
		assertInviter(t, "two", "usrtwo")
		if err := db.Model(&model.User{}).Where("id = ?", "two").Update("status", model.UserStatusDisabled).Error; err != nil {
			t.Fatal(err)
		}
		assertInviter(t, "two", "usrtwo")
	})
	t.Run("deleted inviter", func(t *testing.T) {
		if err := db.Delete(&model.User{}, "id = ?", "two").Error; err != nil {
			t.Fatal(err)
		}
		assertInviter(t, "", "")
	})
	t.Run("scoped administrator cannot inspect administrator", func(t *testing.T) {
		scoped := &model.User{ID: "scoped", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionUsers}}
		if _, err := svc.AdminUserDetail(scoped, actor.ID); err == nil {
			t.Fatal("scoped administrator inspected full administrator")
		}
		if _, err := svc.AdminUserDetail(scoped, "one"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("query errors are propagated", func(t *testing.T) {
		if err := db.Migrator().DropTable(&model.ReferralProfile{}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.AdminUserDetail(actor, "one"); err == nil {
			t.Fatal("failed inviter query was presented as no inviter")
		}
	})
}

func TestAdminUserDetailRequiresUserManagementPermission(t *testing.T) {
	for _, actor := range []*model.User{
		nil,
		{ID: "user", Role: model.UserRoleUser},
		{ID: "scoped", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionChannels}},
	} {
		_, err := (&Service{}).AdminUserDetail(actor, "one")
		var appErr *AppError
		if !errors.As(err, &appErr) || (appErr.Status != 401 && appErr.Status != 403) {
			t.Fatalf("actor %+v: expected permission error, got %v", actor, err)
		}
	}
}
