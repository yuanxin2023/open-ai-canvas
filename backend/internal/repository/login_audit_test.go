package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
)

func TestLoginAuditUnifiesHistoricalAndCurrentEvents(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.User{}, &model.UserLoginEvent{}, &model.LoginLog{}); err != nil {
		t.Fatal(err)
	}
	owner := model.User{ID: "ordinary", Username: "creator", DisplayName: "Creator", Role: model.UserRoleUser}
	admin := model.User{ID: "administrator", Username: "admin", Role: model.UserRoleAdmin}
	if err := db.Create(&[]model.User{owner, admin}).Error; err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	legacy := []model.UserLoginEvent{
		{ID: "same-id", UserID: owner.ID, SessionID: "private-legacy-session", LoginMethod: "email_register", IPAddress: "192.0.2.8", DeviceType: "desktop", Browser: "Chrome", BrowserVersion: "140", OS: "Windows", OSVersion: "11", CreatedAt: at},
		{ID: "admin-event", UserID: admin.ID, LoginMethod: "password", CreatedAt: at},
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	current := []model.LoginLog{
		{ID: "same-id", UserID: &owner.ID, Username: owner.Username, Event: "login", Method: "password", Source: "login", Result: "failed", CreatedAt: at.Add(time.Minute)},
		{ID: "logout", UserID: &owner.ID, Username: owner.Username, Event: "logout", Method: "password", Source: "logout", Result: "success", CreatedAt: at.Add(2 * time.Minute)},
	}
	if err := db.Create(&current).Error; err != nil {
		t.Fatal(err)
	}
	r := New(db)
	logs, total, err := r.LoginLogs(LoginLogFilter{UserRole: model.UserRoleUser}, 2, 0)
	if err != nil || total != 3 || len(logs) != 2 || logs[0].ID != "logout" || logs[1].ID != "same-id" {
		t.Fatalf("merged page: %#v total=%d err=%v", logs, total, err)
	}
	logs, total, err = r.LoginLogs(LoginLogFilter{UserRole: model.UserRoleUser}, 2, 2)
	if err != nil || total != 3 || len(logs) != 1 || logs[0].ID != "legacy:same-id" {
		t.Fatalf("historical page: %#v total=%d err=%v", logs, total, err)
	}
	archived, err := r.LoginLog("legacy:same-id")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(legacy[0].SessionID))
	if archived.Source != "legacy" || archived.Result != "success" || archived.SessionRef != hex.EncodeToString(digest[:]) || archived.BrowserVersion != "140" || archived.OSVersion != "11" {
		t.Fatalf("historical metadata changed: %#v", archived)
	}
	encoded, err := json.Marshal(archived)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), legacy[0].SessionID) {
		t.Fatal("raw legacy session exposed")
	}
	logs, total, err = r.LoginLogs(LoginLogFilter{Method: "email_register", IP: "192.0.2.8"}, 20, 0)
	if err != nil || total != 1 || len(logs) != 1 || logs[0].ID != archived.ID {
		t.Fatalf("legacy filter: %#v %d %v", logs, total, err)
	}
	logs, total, err = r.LoginLogs(LoginLogFilter{Result: "failed"}, 20, 0)
	if err != nil || total != 1 || len(logs) != 1 || logs[0].ID != "same-id" {
		t.Fatalf("failure filter: %#v %d %v", logs, total, err)
	}
	var stored model.UserLoginEvent
	if err := db.First(&stored, "id = ?", legacy[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SessionID != legacy[0].SessionID || stored.BrowserVersion != "140" {
		t.Fatal("query modified historical row")
	}
}
