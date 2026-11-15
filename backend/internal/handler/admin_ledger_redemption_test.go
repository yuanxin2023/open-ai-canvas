package handler

import (
	"encoding/json"
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	"infinite-canvas/backend/internal/service"
)

func TestAdminUserLedgerRedemptionHTTP(t *testing.T) {
	router, db, cookie := profileHandlerFixture(t)
	svc := service.New(repository.New(db), t.TempDir())
	t.Cleanup(func() { _ = svc.Close() })
	RegisterAdminRoutes(router.Group("/api"), svc)
	base := "/api/admin/users/redemption-user/ledger"
	for _, path := range []string{base, base + "/missing/redemption"} {
		if rec := profileRequest(router, "GET", path, "", ""); rec.Code != 401 {
			t.Fatalf("anonymous %s: %d", path, rec.Code)
		}
		if rec := profileRequest(router, "GET", path, "", cookie); rec.Code != 403 {
			t.Fatalf("ordinary user %s: %d", path, rec.Code)
		}
	}
	if err := db.Model(&model.User{}).Where("id = ?", "profile-user").Updates(map[string]any{"role": model.UserRoleAdmin, "admin_level": model.AdminLevelFull}).Error; err != nil {
		t.Fatal(err)
	}
	admin := &model.User{ID: "profile-user", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull, Status: model.UserStatusActive}
	user := &model.User{ID: "redemption-user", Username: "redemption-user", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	created, err := svc.AdminCreateRedeemBatch(admin, service.CreateRedeemBatchRequest{AmountMicrocredits: 1000000, Count: 1, Note: "HTTP 核销测试"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RedeemCredits(user, created.Codes[0], "203.0.113.8"); err != nil {
		t.Fatal(err)
	}
	rec := profileRequest(router, "GET", base+"?type=increase", "", cookie)
	var list struct {
		Code int
		Data service.AdminUserLedgerPage
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || rec.Code != 200 || list.Code != 0 || len(list.Data.Entries) != 1 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("list: %d %s, %v", rec.Code, rec.Body.String(), err)
	}
	entry := list.Data.Entries[0]
	if entry.Redemption == nil || entry.Redemption.Code != created.Codes[0] {
		t.Fatalf("list missing code: %+v", entry)
	}
	rec = profileRequest(router, "GET", base+"/"+entry.ID+"/redemption", "", cookie)
	var detail struct {
		Code int
		Data service.AdminUserLedgerRedemption
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil || rec.Code != 200 || detail.Code != 0 || detail.Data.Code.Code != created.Codes[0] || detail.Data.Code.RedeemedBy != user.ID || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("detail: %d %s, %v", rec.Code, rec.Body.String(), err)
	}
	for _, path := range []string{base + "/missing/redemption", "/api/admin/users/profile-user/ledger/" + entry.ID + "/redemption"} {
		if missing := profileRequest(router, "GET", path, "", cookie); missing.Code != 404 {
			t.Fatalf("invalid association %s: %d %s", path, missing.Code, missing.Body.String())
		}
	}
}
