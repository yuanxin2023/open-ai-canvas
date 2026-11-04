package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"infinite-canvas/backend/internal/auth"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	"infinite-canvas/backend/internal/service"
	_ "modernc.org/sqlite"
)

func TestAgentInstalledSkillsRemainAvailableWhenLibraryClosed(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: "file:" + t.Name() + "?mode=memory&cache=shared"}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.UserResourceQuota{}, &model.User{}, &model.AuthSession{}, &model.UserIdentity{}, &model.Skill{}, &model.SkillVersion{}, &model.SkillFile{}, &model.UserSkillState{}, &model.SkillPlatformState{}, &model.SkillCategoryPlatformState{}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"owner", "other"} {
		if err := db.Create(&model.User{ID: id, Username: id, Role: model.UserRoleUser, Status: model.UserStatusActive}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.AuthSession{ID: id, UserID: id, TokenHash: auth.HashToken("test-token"), ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := service.New(repository.New(db), t.TempDir())
	t.Cleanup(func() { _ = svc.Close() })
	skill, err := svc.CreateSkill("owner", service.SkillMutationRequest{SkillName: "Private Agent Skill", Description: "Installed skill", Instruction: "# Private skill", IsPrivate: true, Tag: "creative"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SystemSetting{Key: "feature_availability", ValueJSON: `{"skillLibraryEnabled":false}`}).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterSkillRoutes(router.Group("/api"), svc)
	send := func(method, path, user, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if user != "" {
			req.AddCookie(&http.Cookie{Name: service.SessionCookieName, Value: user + ".test-token"})
		}
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		return out
	}
	for _, item := range []struct {
		method, path, user, body string
		status                   int
	}{
		{"GET", "/agent/skills/installed", "", "", 401},
		{"GET", "/skills/added", "owner", "", 403},
		{"POST", "/skills/" + skill.SkillID + "/add", "owner", "", 403},
		{"PATCH", "/agent/skills/" + skill.SkillID + "/agent-mode", "owner", `{"mode":"manual"}`, 200},
	} {
		out := send(item.method, item.path, item.user, item.body)
		if out.Code != item.status {
			t.Fatalf("%s %s: %d %s", item.method, item.path, out.Code, out.Body.String())
		}
	}
	for _, user := range []string{"owner", "other"} {
		out := send("GET", "/agent/skills/installed", user, "")
		if out.Code != 200 {
			t.Fatalf("%s: %d %s", user, out.Code, out.Body.String())
		}
		var response struct {
			Data struct {
				Skills []struct {
					SkillID string `json:"skillId"`
				} `json:"skills"`
			} `json:"data"`
		}
		if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, entry := range response.Data.Skills {
			found = found || entry.SkillID == skill.SkillID
		}
		if found != (user == "owner") {
			t.Fatalf("installed skill ownership leaked for %s", user)
		}
	}
}
