package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"infinite-canvas/backend/internal/auth"
	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	"infinite-canvas/backend/internal/service"
)

func TestUserDataCleanupChangesHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{
		&model.User{ID: "u", Username: "u", Role: model.UserRoleUser, Status: model.UserStatusActive},
		&model.AuthSession{ID: "s", UserID: "u", TokenHash: auth.HashToken("test"), ExpiresAt: time.Now().Add(time.Hour)},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := repository.New(db)
	for _, owner := range []string{"u", "other"} {
		if err := repo.Transaction(func(tx *repository.Repository) error {
			return tx.RecordUserDataCleanup(owner, []model.UserDataCleanupEntry{{Kind: "asset", EntityID: owner + "-card", Action: "delete"}})
		}); err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	RegisterUserDataRoutes(router.Group("/api"), service.New(repo, t.TempDir()))
	call := func(query string, loggedIn bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/user-data/cleanup-changes"+query, nil)
		if loggedIn {
			req.AddCookie(&http.Cookie{Name: service.SessionCookieName, Value: "s.test"})
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	if res := call("", false); res.Code != 401 {
		t.Fatalf("unauthenticated=%d", res.Code)
	}
	for _, query := range []string{"?after=-1", "?after=bad", "?after=9007199254740992"} {
		if res := call(query, true); res.Code != 400 {
			t.Fatalf("%s=%d %s", query, res.Code, res.Body.String())
		}
	}
	res := call("?after=0", true)
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"id":"u-card"`) || strings.Contains(res.Body.String(), "other-card") {
		t.Fatalf("response=%d %s", res.Code, res.Body.String())
	}
	if res := call("?after=1", true); res.Code != 200 || !strings.Contains(res.Body.String(), `"entries":[]`) {
		t.Fatalf("next=%d %s", res.Code, res.Body.String())
	}
}
