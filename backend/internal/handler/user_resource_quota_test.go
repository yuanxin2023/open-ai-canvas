package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
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

func TestUserResourceQuotaHTTPContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(database.Config{Driver: "sqlite", DSN: "file:quota-http?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	for _, user := range []model.User{
		{ID: "admin", Username: "admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull, Status: model.UserStatusActive},
		{ID: "limited", Username: "limited", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, Status: model.UserStatusActive},
		{ID: "ordinary", Username: "ordinary", Role: model.UserRoleUser, Status: model.UserStatusActive},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.AuthSession{ID: user.ID, UserID: user.ID, TokenHash: auth.HashToken("token"), ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := service.New(repository.New(db), t.TempDir())
	previousRuntime := runtimeService
	t.Cleanup(func() { ConfigureRuntime(previousRuntime) })
	ConfigureRuntime(svc)
	router := gin.New()
	registerUserResourceQuotaRoutes(router.Group("/api"), svc)
	RegisterChunkedUploadRoutes(router.Group("/api"), svc)
	RegisterUserDataRoutes(router.Group("/api"), svc)
	request := func(actor, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if actor != "" {
			r.AddCookie(&http.Cookie{Name: service.SessionCookieName, Value: actor + ".token"})
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	path := "/api/admin/users/ordinary/resource-quota"
	adminPath := "/api/admin/administrators/limited/resource-quota"
	for _, tc := range []struct {
		actor, method, path, body string
		status                    int
	}{
		{"", "GET", adminPath, "", 401},
		{"ordinary", "GET", adminPath, "", 403},
		{"limited", "GET", adminPath, "", 403},
		{"limited", "PATCH", adminPath, `{"overrides":{"storedFileGB":5}}`, 403},
		{"admin", "GET", adminPath, "", 200},
		{"admin", "PATCH", adminPath, `{"overrides":{"storedFileGB":5}}`, 200},
		{"admin", "PATCH", adminPath, `{"overrides":{"storedFileGB":null}}`, 200},
		{"admin", "PATCH", adminPath, `{"overrides":{"unknown":null}}`, 400},
		{"admin", "PATCH", adminPath, `{"overrides":{"resourceUploadMB":1.5}}`, 400},
		{"admin", "PATCH", adminPath, `{"overrides":{"storedFileGB":5},"extra":1}`, 400},
		{"admin", "GET", "/api/admin/administrators/ordinary/resource-quota", "", 400},
		{"admin", "PATCH", "/api/admin/administrators/ordinary/resource-quota", `{"overrides":{"storedFileGB":5}}`, 400},
		{"admin", "GET", "/api/admin/users/admin/resource-quota", "", 400},
		{"admin", "PATCH", "/api/admin/users/admin/resource-quota", `{"overrides":{"storedFileGB":5}}`, 400},
		{"admin", "PATCH", "/api/admin/administrators/admin/resource-quota", `{"overrides":{"storedFileGB":7}}`, 200},
	} {
		w := request(tc.actor, tc.method, tc.path, tc.body)
		if w.Code != tc.status {
			t.Fatalf("%s %s %s = %d: %s", tc.actor, tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	for _, tc := range []struct {
		actor, method, body string
		status              int
	}{
		{"", "GET", "", 401}, {"ordinary", "GET", "", 403}, {"limited", "PATCH", `{"overrides":{"storedFileGB":5}}`, 403},
		{"admin", "PATCH", `{"overrides":{"unknown":null}}`, 400},
		{"admin", "PATCH", `{"overrides":{"storedFileGB":0}}`, 400},
		{"admin", "PATCH", `{"overrides":{"resourceUploadMB":1.5}}`, 400},
		{"admin", "PATCH", `{"overrides":{},"other":1}`, 400},
		{"admin", "PATCH", `{"overrides":{}}`, 400},
		{"admin", "PATCH", `{"overrides":{"storedFileGB":5}} {}`, 400},
		{"admin", "PATCH", `{"overrides":{"storedFileGB":5,"resourceUploadMB":1}}`, 200},
		{"admin", "PATCH", `{"overrides":{"storedFileGB":null}}`, 200},
	} {
		w := request(tc.actor, tc.method, path, tc.body)
		if w.Code != tc.status {
			t.Fatalf("%s %s %s = %d: %s", tc.actor, tc.method, tc.body, w.Code, w.Body.String())
		}
	}
	var response struct {
		Data service.UserResourceQuotaView `json:"data"`
	}
	w := request("admin", "GET", path, "")
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data.Overrides) != 1 || response.Data.EffectiveQuota.ResourceUploadMB != 1 || response.Data.EffectiveQuota.StoredFileGB != 20 {
		t.Fatal(response.Data)
	}
	w = request("ordinary", "POST", "/api/resources/uploads", `{"fileName":"large.png","kind":"image","size":1048576}`)
	if w.Code != 400 {
		t.Fatalf("chunk start bypassed full size limit: %d %s", w.Code, w.Body.String())
	}
	w = request("admin", "PATCH", path, `{"overrides":{"resourceUploadMB":3}}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = request("ordinary", "POST", "/api/resources/uploads", `{"fileName":"large.bin","kind":"file","size":2097152}`)
	var session struct {
		Data struct {
			UploadID string `json:"uploadId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil || w.Code != 200 || session.Data.UploadID == "" {
		t.Fatal("chunk start", w.Body.String(), err)
	}
	t.Cleanup(func() { dropChunkSession(session.Data.UploadID) })
	uploadPath := "/api/resources/uploads/" + session.Data.UploadID
	w = request("ordinary", "PUT", uploadPath+"/chunks/0", strings.Repeat("x", 2<<20))
	if w.Code != 200 {
		t.Fatal("chunk write", w.Body.String())
	}
	w = request("admin", "PATCH", path, `{"overrides":{"resourceUploadMB":1}}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = request("ordinary", "POST", uploadPath+"/complete", "{}")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "单个上传文件必须小于 1MB") {
		t.Fatalf("completion bypassed new quota: %d %s", w.Code, w.Body.String())
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "file.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), 1<<20)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/resources", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.AddCookie(&http.Cookie{Name: service.SessionCookieName, Value: "ordinary.token"})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "单个上传文件必须小于 1MB") {
		t.Fatalf("ordinary upload bypassed personal quota: %d %s", w.Code, w.Body.String())
	}
}
