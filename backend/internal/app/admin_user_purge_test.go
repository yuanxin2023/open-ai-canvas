package app

import (
	"testing"

	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeleteUserStillOnlyDisablesAccount(t *testing.T) {
	db := newBulkUserTestDB(t)
	actor := model.User{ID: "admin-1", Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	target := model.User{ID: "user-1", Username: "target", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&[]model.User{actor, target}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: target.ID, AvailableMicrocredits: 100}).Error; err != nil {
		t.Fatal(err)
	}

	if err := (&Service{repo: repository.New(db)}).DeleteUser(&actor, target.ID); err != nil {
		t.Fatal(err)
	}
	var stored model.User
	if err := db.First(&stored, "id = ?", target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.UserStatusDisabled {
		t.Fatalf("status = %q, want disabled", stored.Status)
	}
	assertUserPurgeCount(t, db, &model.CreditAccount{}, "user_id = ?", target.ID, 1)
}

func TestPurgeUserDeletesAccountAndUserScopedData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	actor := model.User{ID: "admin-1", Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	target := model.User{ID: "user-1", Username: "target", Email: "target@example.com", Role: model.UserRoleUser, Status: model.UserStatusActive}
	other := model.User{ID: "user-2", Username: "other", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&[]model.User{actor, target, other}).Error; err != nil {
		t.Fatal(err)
	}
	rows := []any{
		&model.AuthSession{ID: "session-1", UserID: target.ID},
		&model.UserLoginEvent{ID: "login-event-1", UserID: target.ID, SessionID: "session-1", LoginMethod: "password"},
		&model.UserUsernameChange{ID: "username-change-1", UserID: target.ID, OldUsername: "oldone", NewUsername: target.Username, CountsTowardLimit: true},
		&model.CreditAccount{UserID: target.ID, AvailableMicrocredits: 100},
		&model.Project{ID: "project-1", UserID: target.ID, Name: "Project"},
		&model.CanvasProject{ID: "canvas-1", UserID: target.ID, Title: "Canvas"},
		&model.Task{ID: "task-1", UserID: target.ID, Status: model.TaskStatusSucceeded},
		&model.TaskLog{ID: "log-1", UserID: target.ID, TaskID: "task-1"},
		&model.Resource{ID: "resource-1", UserID: target.ID, Kind: "image", Status: model.ResourceStatusReady},
		&model.Asset{ID: "asset-1", UserID: target.ID, Kind: "image", Title: "Asset"},
		&model.UserPrompt{ID: "prompt-1", UserID: target.ID, Title: "Prompt", Mode: model.InspirationModeText, Prompt: "text"},
		&model.CreditAccount{UserID: other.ID, AvailableMicrocredits: 200},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("create %T: %v", row, err)
		}
	}

	if err := (&Service{repo: repository.New(db)}).PurgeUser(&actor, target.ID); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		model any
		query string
	}{
		{&model.User{}, "id = ?"}, {&model.AuthSession{}, "user_id = ?"}, {&model.UserLoginEvent{}, "user_id = ?"}, {&model.UserUsernameChange{}, "user_id = ?"},
		{&model.CreditAccount{}, "user_id = ?"}, {&model.Project{}, "user_id = ?"},
		{&model.CanvasProject{}, "user_id = ?"}, {&model.Task{}, "user_id = ?"},
		{&model.TaskLog{}, "user_id = ?"}, {&model.Resource{}, "user_id = ?"},
		{&model.Asset{}, "user_id = ?"}, {&model.UserPrompt{}, "user_id = ?"},
	} {
		assertUserPurgeCount(t, db, check.model, check.query, target.ID, 0)
	}
	assertUserPurgeCount(t, db, &model.CreditAccount{}, "user_id = ?", other.ID, 1)
	assertUserPurgeCount(t, db, &model.AdminAuditEvent{}, "action = ? AND target_id = ?", []any{"user.purge", target.ID}, 1)
}

func assertUserPurgeCount(t *testing.T, db *gorm.DB, modelValue any, query string, arg any, want int64) {
	t.Helper()
	args := []any{arg}
	if values, ok := arg.([]any); ok {
		args = values
	}
	var count int64
	if err := db.Model(modelValue).Where(query, args...).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("%T count = %d, want %d", modelValue, count, want)
	}
}
