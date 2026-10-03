package app

import (
	"errors"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminUserLoginEventsValidatesTimeFilters(t *testing.T) {
	tests := []AdminUserLoginEventQuery{
		{StartAt: "not-a-time"},
		{EndAt: "2026-10-03"},
		{StartAt: "2026-10-03T12:00:01Z", EndAt: "2026-10-03T12:00:00Z"},
	}
	for _, query := range tests {
		if _, err := normalizeAdminUserLoginEventFilter(query); err == nil {
			t.Fatalf("normalizeAdminUserLoginEventFilter(%+v) error = nil", query)
		} else {
			var appErr *AppError
			if !errors.As(err, &appErr) || appErr.Status != 400 || appErr.Reason != ReasonInvalidArgument {
				t.Fatalf("normalizeAdminUserLoginEventFilter(%+v) error = %#v", query, err)
			}
		}
	}
}

func TestAdminUserLoginEventsRequiresUserManagementPermission(t *testing.T) {
	actor := &model.User{ID: "ordinary-actor", Role: model.UserRoleUser, Status: model.UserStatusActive}
	_, err := (&Service{}).AdminUserLoginEvents(actor, "user-1", AdminUserLoginEventQuery{})
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Status != 403 {
		t.Fatalf("AdminUserLoginEvents() error = %#v, want forbidden", err)
	}
}

func TestAdminUserLoginEventsReturnsFilteredTotal(t *testing.T) {
	svc, actor, target := newAdminLoginEventTestService(t)
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	events := []model.UserLoginEvent{
		{ID: "event-1", UserID: target.ID, LoginMethod: "password", IPAddress: "10.0.0.1", CreatedAt: now.Add(-time.Hour)},
		{ID: "event-2", UserID: target.ID, LoginMethod: "linuxdo", IPAddress: "10.0.0.2", CreatedAt: now},
	}
	if err := svc.repo.Create(&events); err != nil {
		t.Fatal(err)
	}
	result, err := svc.AdminUserLoginEvents(actor, target.ID, AdminUserLoginEventQuery{
		Page: 1, Limit: 20, StartAt: now.Add(-2 * time.Hour).Format(time.RFC3339), EndAt: now.Format(time.RFC3339), LoginMethod: "password", IP: "10.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Events) != 1 || result.Events[0].ID != "event-1" {
		t.Fatalf("result = %+v", result)
	}
}

func newAdminLoginEventTestService(t *testing.T) (*Service, *model.User, *model.User) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AdminPermissionGrant{}, &model.UserLoginEvent{}); err != nil {
		t.Fatal(err)
	}
	actor := &model.User{ID: "admin-1", Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	target := &model.User{ID: "user-1", Username: "user-one", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(target).Error; err != nil {
		t.Fatal(err)
	}
	return &Service{repo: repository.New(db)}, actor, target
}
