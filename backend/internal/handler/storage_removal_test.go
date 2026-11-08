package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"infinite-canvas/backend/internal/auth"
	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	"infinite-canvas/backend/internal/service"
)

func TestStorageRemovalHTTPContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(database.Config{Driver: "sqlite", DSN: t.TempDir() + "/storage.db"})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	since := time.Now().Add(-8 * 24 * time.Hour)
	for _, row := range []any{
		&model.User{ID: "one", Username: "ordinary", Role: "user", Status: "active"},
		&model.AuthSession{ID: "session", UserID: "one", TokenHash: auth.HashToken("token"), ExpiresAt: time.Now().Add(time.Hour)},
		&model.UserResourceQuota{UserID: "one", OverridesJSON: `{"storedFileGB":1}`},
		&model.Resource{ID: "file", UserID: "one", Status: model.ResourceStatusReady, Size: 2 << 30},
		&model.UserStorageState{UserID: "one", OverLimitSince: &since},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := service.New(repository.New(db), t.TempDir())
	previous := runtimeService
	t.Cleanup(func() { ConfigureRuntime(previous) })
	ConfigureRuntime(svc)
	router := gin.New()
	RegisterCanvasAPI(router.Group("/api"), svc)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: service.SessionCookieName, Value: "session.token"})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	w := request("GET", "/api/resources/storage-usage", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var envelope struct {
		Data struct {
			Usage map[string]any `json:"usage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Usage) != 3 || envelope.Data.Usage["totalBytes"] != float64(1<<30) || envelope.Data.Usage["overLimit"] != true {
		t.Fatal(envelope)
	}
	for _, path := range []string{"/api/resources/storage-grants", "/api/admin/users/one/storage", "/api/admin/users/one/storage-grants", "/api/admin/storage-grants/gift/revoke"} {
		for _, method := range []string{"GET", "POST"} {
			if w := request(method, path, "{}"); w.Code != 404 {
				t.Fatalf("removed route %s %s: %d", method, path, w.Code)
			}
		}
	}
	// 旧的八天超额状态不能在业务入参校验前拒绝写入。
	w = request("PUT", "/api/assets/any", "{")
	if w.Code != 400 {
		t.Fatalf("legacy readonly state intercepted write: %d %s", w.Code, w.Body.String())
	}
}
