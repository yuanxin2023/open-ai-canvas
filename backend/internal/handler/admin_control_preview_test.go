package handler

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

func TestControlPreviewHTTPPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "preview.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.AutoMigrate(&model.User{}, &model.AuthSession{}, &model.AdminPermissionGrant{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.BillingOrder{}, &model.PaymentOrder{}, &model.Task{}, &model.ApiCallLog{}, &model.SystemSetting{}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"full", "scoped", "allowed", "user"} {
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
	if err := db.Create(&model.AdminPermissionGrant{UserID: "allowed", Permission: model.AdminPermissionControlPreview}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AdminPermissionGrant{UserID: "scoped", Permission: model.AdminPermissionAnalyticsOverview}).Error; err != nil {
		t.Fatal(err)
	}
	svc := service.New(repository.New(db), t.TempDir())
	router := gin.New()
	RegisterAdminAnalyticsRoutes(router.Group("/api"), svc)
	call := func(actor, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/analytics/control-preview"+path, nil)
		if actor != "" {
			req.AddCookie(&http.Cookie{Name: service.SessionCookieName, Value: actor + ".test"})
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	paths := []string{"", "/users", "/users/user", "/model-users?model=free&capability=text"}
	for _, path := range paths {
		for _, actor := range []string{"", "user", "scoped", "full", "allowed"} {
			response := call(actor, path)
			want := 200
			if actor == "" {
				want = 401
			} else if actor == "user" || actor == "scoped" {
				want = 403
			}
			if response.Code != want {
				t.Fatalf("%s %s: %d %s", actor, path, response.Code, response.Body.String())
			}
			if want == 200 && (!strings.Contains(response.Body.String(), `"code":0`) || strings.Contains(response.Body.String(), "passwordHash")) {
				t.Fatalf("invalid projection: %s", response.Body.String())
			}
		}
	}
	for _, path := range []string{"?period=bad", "/users?page=abc", "/users?pageSize=101", "/model-users?model=x"} {
		if response := call("full", path); response.Code != 400 {
			t.Fatalf("invalid query %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	if response := call("full", "/users/full"); response.Code != 404 {
		t.Fatalf("admin detail status: %d %s", response.Code, response.Body.String())
	}
}
