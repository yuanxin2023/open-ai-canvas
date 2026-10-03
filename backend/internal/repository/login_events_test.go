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

func TestAdminUserLoginEventsAppliesFiltersAndPagination(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.UserLoginEvent{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	events := []model.UserLoginEvent{
		{ID: "event-1", UserID: "user-1", LoginMethod: "password", IPAddress: "192.168.0.10", CreatedAt: now.Add(-3 * time.Hour)},
		{ID: "event-2", UserID: "user-1", LoginMethod: "linuxdo", IPAddress: "192.168.0.11", CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "event-3", UserID: "user-1", LoginMethod: "password", IPAddress: "10.0.0.8", CreatedAt: now.Add(-time.Hour)},
		{ID: "event-4", UserID: "user-1", LoginMethod: "password", IPAddress: "10.0.0.9", CreatedAt: now},
		{ID: "other-user", UserID: "user-2", LoginMethod: "password", IPAddress: "10.0.0.9", CreatedAt: now},
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)

	page, total, err := repo.AdminUserLoginEvents("user-1", AdminUserLoginEventFilter{}, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(page) != 2 || page[0].ID != "event-4" || page[1].ID != "event-3" {
		t.Fatalf("unfiltered page = %+v, total = %d", page, total)
	}

	startAt := now.Add(-2 * time.Hour)
	endAt := now.Add(-time.Hour)
	tests := []struct {
		name   string
		filter AdminUserLoginEventFilter
		want   []string
	}{
		{name: "closed time range", filter: AdminUserLoginEventFilter{StartAt: &startAt, EndAt: &endAt}, want: []string{"event-3", "event-2"}},
		{name: "login method", filter: AdminUserLoginEventFilter{LoginMethod: "linuxdo"}, want: []string{"event-2"}},
		{name: "partial ip", filter: AdminUserLoginEventFilter{IP: "168.0"}, want: []string{"event-2", "event-1"}},
		{name: "combined", filter: AdminUserLoginEventFilter{StartAt: &startAt, LoginMethod: "password", IP: "10.0"}, want: []string{"event-4", "event-3"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, count, err := repo.AdminUserLoginEvents("user-1", test.filter, 20, 0)
			if err != nil {
				t.Fatal(err)
			}
			if count != int64(len(test.want)) || len(got) != len(test.want) {
				t.Fatalf("events = %+v, total = %d, want IDs %v", got, count, test.want)
			}
			for index, id := range test.want {
				if got[index].ID != id {
					t.Fatalf("event[%d].ID = %q, want %q", index, got[index].ID, id)
				}
			}
		})
	}
}
