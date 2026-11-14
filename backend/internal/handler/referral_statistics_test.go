package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"infinite-canvas/backend/internal/auth"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestReferralStatisticsRoutesRequireSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerReferralRoutes(router.Group("/api"), &service.Service{})
	for _, path := range []string{"/api/admin/referrals/statistics", "/api/admin/referrals/statistics/users", "/api/admin/referrals/users/someone/invitees"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status %d body %s", path, response.Code, response.Body.String())
		}
	}
}

func TestReferralStatisticsHTTPContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "statistics.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { conn.Close() })
	if err := db.AutoMigrate(&model.User{}, &model.AuthSession{}, &model.AdminPermissionGrant{}, &model.ReferralProfile{}, &model.PaymentOrder{}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"full", "allowed", "denied", "user"} {
		role, level := model.UserRoleAdmin, model.AdminLevelScoped
		if id == "full" {
			level = model.AdminLevelFull
		}
		if id == "user" {
			role = model.UserRoleUser
		}
		if err := db.Create(&model.User{ID: id, Username: id, Role: role, AdminLevel: level, Status: model.UserStatusActive}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.AuthSession{ID: id, UserID: id, TokenHash: auth.HashToken("test"), ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.AdminPermissionGrant{UserID: "allowed", Permission: model.AdminPermissionReferrals}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]model.ReferralProfile{{UserID: "full", Code: "ABC234"}, {UserID: "user", Code: "DEF567", InviterID: "full"}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.PaymentOrder{ID: "paid", UserID: "user", IdempotencyKey: "paid", MerchantOrderNo: "paid", Status: model.PaymentOrderCredited, AmountFen: 1234, Currency: "CNY", ProductKind: model.TopupProductKindCredits}).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	registerReferralRoutes(router.Group("/api"), service.New(repository.New(db), t.TempDir()))
	call := func(actor, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/referrals"+path, nil)
		if actor != "" {
			req.AddCookie(&http.Cookie{Name: service.SessionCookieName, Value: actor + ".test"})
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	for _, path := range []string{"/statistics", "/statistics/users", "/users/full/invitees"} {
		for _, actor := range []string{"", "user", "denied", "full", "allowed"} {
			want := http.StatusOK
			if actor == "" {
				want = http.StatusUnauthorized
			} else if actor == "user" || actor == "denied" {
				want = http.StatusForbidden
			}
			response := call(actor, path)
			if response.Code != want {
				t.Fatalf("%s %s: %d %s", actor, path, response.Code, response.Body.String())
			}
			var body struct {
				Code int            `json:"code"`
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if want == http.StatusOK && (body.Code != 0 || body.Data == nil) || want != http.StatusOK && body.Code == 0 {
				t.Fatalf("invalid envelope: %s", response.Body.String())
			}
			if want == http.StatusOK && path == "/statistics" && (body.Data["paymentAmountFen"] != float64(1234) || body.Data["paidInviteeCount"] != float64(1)) {
				t.Fatalf("incorrect overview JSON: %s", response.Body.String())
			}
		}
	}
	for _, path := range []string{"/statistics/users?page=abc", "/statistics/users?page=0", "/statistics/users?page=-1", "/statistics/users?pageSize=0", "/statistics/users?pageSize=101", "/statistics/users?sortBy=username", "/users/full/invitees?pageSize=101"} {
		if response := call("full", path); response.Code != 400 {
			t.Fatalf("invalid query %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	if response := call("allowed", "/users/missing/invitees"); response.Code != 404 {
		t.Fatalf("missing inviter: %d %s", response.Code, response.Body.String())
	}
}
