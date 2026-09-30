package repository

import (
	"fmt"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateAuthSessionWithLoginEventAppliesRetention(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AuthSession{}, &model.UserLoginEvent{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	rows := make([]model.UserLoginEvent, 0, 205)
	rows = append(rows, model.UserLoginEvent{ID: "expired", UserID: "user-1", CreatedAt: now.Add(-91 * 24 * time.Hour)})
	for index := 0; index < 204; index++ {
		rows = append(rows, model.UserLoginEvent{ID: fmt.Sprintf("existing-%03d", index), UserID: "user-1", CreatedAt: now.Add(-time.Duration(index+1) * time.Minute)})
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	session := model.AuthSession{ID: "session-new", UserID: "user-1", ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now}
	event := model.UserLoginEvent{ID: "event-new", UserID: "user-1", SessionID: session.ID, LoginMethod: "password", CreatedAt: now}
	if err := New(db).CreateAuthSessionWithLoginEvent(&session, &event); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.UserLoginEvent{}).Where("user_id = ?", "user-1").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != userLoginEventLimit {
		t.Fatalf("login event count = %d, want %d", count, userLoginEventLimit)
	}
	var expiredCount int64
	if err := db.Model(&model.UserLoginEvent{}).Where("id = ?", "expired").Count(&expiredCount).Error; err != nil {
		t.Fatal(err)
	}
	if expiredCount != 0 {
		t.Fatal("expired login event was retained")
	}
}
