package app

import (
	"encoding/json"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateAdminUserCreatesActiveUserWithPasswordAndAudit(t *testing.T) {
	db := newBulkUserTestDB(t)
	actor := model.User{ID: "admin-1", Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(&actor).Error; err != nil {
		t.Fatal(err)
	}

	created, err := (&Service{repo: repository.New(db)}).CreateAdminUser(&actor, CreateAdminUserRequest{
		Username:    "new-user",
		DisplayName: "New User",
		Email:       "new-user@example.com",
		Password:    "strong-password",
		Role:        model.UserRoleUser,
		Status:      model.UserStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Username != "new-user" || created.DisplayName != "New User" || created.Email != "new-user@example.com" {
		t.Fatalf("created user = %+v", created)
	}
	if created.Role != model.UserRoleUser || created.Status != model.UserStatusActive {
		t.Fatalf("created user role/status = %q/%q", created.Role, created.Status)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(created.PasswordHash), []byte("strong-password")); err != nil {
		t.Fatalf("password hash does not match: %v", err)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits <= 0 {
		t.Fatalf("available credits = %d, want positive signup bonus", account.AvailableMicrocredits)
	}
	var audit model.AdminAuditEvent
	if err := db.Where("action = ? AND target_id = ?", "user.create", created.ID).First(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if audit.ActorUserID != actor.ID {
		t.Fatalf("audit actor = %q, want %q", audit.ActorUserID, actor.ID)
	}
}

func TestCreateAdminUserRejectsDuplicateUsername(t *testing.T) {
	db := newBulkUserTestDB(t)
	actor := model.User{ID: "admin-1", Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(&actor).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db)}
	input := CreateAdminUserRequest{Username: "duplicate", DisplayName: "Duplicate", Password: "strong-password", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if _, err := svc.CreateAdminUser(&actor, input); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAdminUser(&actor, input); err == nil {
		t.Fatal("CreateAdminUser() duplicate username error = nil")
	}
}

func TestUpdateUserAllowsAdminToResetPasswordAndRevokesTargetSessions(t *testing.T) {
	db := newBulkUserTestDB(t)
	oldHash, err := hashPassword("old-password")
	if err != nil {
		t.Fatal(err)
	}
	actor := model.User{ID: "admin-1", Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	target := model.User{ID: "user-1", Username: "user-one", DisplayName: "User One", PasswordHash: oldHash, Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&[]model.User{actor, target}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]model.AuthSession{{ID: "admin-session", UserID: actor.ID}, {ID: "user-session", UserID: target.ID}}).Error; err != nil {
		t.Fatal(err)
	}

	updated, err := (&Service{repo: repository.New(db)}).UpdateUser(&actor, target.ID, UpdateUserRequest{Password: "new-password"})
	if err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("new-password")) != nil {
		t.Fatal("updated password hash does not match the new password")
	}
	if bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("old-password")) == nil {
		t.Fatal("updated password still matches the old password")
	}
	var targetSessions int64
	if err := db.Model(&model.AuthSession{}).Where("user_id = ?", target.ID).Count(&targetSessions).Error; err != nil {
		t.Fatal(err)
	}
	if targetSessions != 0 {
		t.Fatalf("target sessions = %d, want 0", targetSessions)
	}
	var actorSessions int64
	if err := db.Model(&model.AuthSession{}).Where("user_id = ?", actor.ID).Count(&actorSessions).Error; err != nil {
		t.Fatal(err)
	}
	if actorSessions != 1 {
		t.Fatalf("actor sessions = %d, want 1", actorSessions)
	}
	var audit model.AdminAuditEvent
	if err := db.Where("action = ? AND target_id = ?", "user.update", target.ID).First(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(audit.Summary, "重置密码") {
		t.Fatalf("audit summary = %q", audit.Summary)
	}
	var metadata struct {
		PasswordReset bool `json:"passwordReset"`
	}
	if err := json.Unmarshal([]byte(audit.MetadataJSON), &metadata); err != nil {
		t.Fatal(err)
	}
	if !metadata.PasswordReset {
		t.Fatalf("audit metadata = %s", audit.MetadataJSON)
	}
}

func TestUpdateUserPasswordRequiresAdmin(t *testing.T) {
	db := newBulkUserTestDB(t)
	oldHash, err := hashPassword("old-password")
	if err != nil {
		t.Fatal(err)
	}
	actor := model.User{ID: "user-1", Username: "user-one", Role: model.UserRoleUser, Status: model.UserStatusActive}
	target := model.User{ID: "user-2", Username: "user-two", PasswordHash: oldHash, Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&[]model.User{actor, target}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := (&Service{repo: repository.New(db)}).UpdateUser(&actor, target.ID, UpdateUserRequest{Password: "new-password"}); err == nil {
		t.Fatal("UpdateUser() error = nil, want admin permission error")
	}
	var stored model.User
	if err := db.First(&stored, "id = ?", target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("old-password")) != nil {
		t.Fatal("password changed without admin permission")
	}
}

func TestBulkDisableUsersDisablesUsersSessionsAndWritesAudits(t *testing.T) {
	db := newBulkUserTestDB(t)
	actor := model.User{ID: "admin-1", Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	users := []model.User{
		actor,
		{ID: "user-1", Username: "user-one", Role: model.UserRoleUser, Status: model.UserStatusActive},
		{ID: "user-2", Username: "user-two", Role: model.UserRoleUser, Status: model.UserStatusActive},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	sessions := []model.AuthSession{{ID: "session-1", UserID: "user-1"}, {ID: "session-2", UserID: "user-2"}}
	if err := db.Create(&sessions).Error; err != nil {
		t.Fatal(err)
	}

	result, err := (&Service{repo: repository.New(db)}).BulkDisableUsers(&actor, BulkDisableUsersRequest{UserIDs: []string{"user-1", "user-2"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.DisabledCount != 2 {
		t.Fatalf("DisabledCount = %d, want 2", result.DisabledCount)
	}
	var activeUsers int64
	if err := db.Model(&model.User{}).Where("id IN ? AND status = ?", []string{"user-1", "user-2"}, model.UserStatusActive).Count(&activeUsers).Error; err != nil {
		t.Fatal(err)
	}
	if activeUsers != 0 {
		t.Fatalf("active users = %d, want 0", activeUsers)
	}
	var sessionCount int64
	if err := db.Model(&model.AuthSession{}).Count(&sessionCount).Error; err != nil {
		t.Fatal(err)
	}
	if sessionCount != 0 {
		t.Fatalf("sessions = %d, want 0", sessionCount)
	}
	var auditCount int64
	if err := db.Model(&model.AdminAuditEvent{}).Where("action = ?", "user.bulk_disable").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 2 {
		t.Fatalf("audits = %d, want 2", auditCount)
	}
}

func TestBulkDisableUsersRollsBackWhenAnyUserIsMissing(t *testing.T) {
	db := newBulkUserTestDB(t)
	actor := model.User{ID: "admin-1", Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	user := model.User{ID: "user-1", Username: "user-one", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&[]model.User{actor, user}).Error; err != nil {
		t.Fatal(err)
	}

	_, err := (&Service{repo: repository.New(db)}).BulkDisableUsers(&actor, BulkDisableUsersRequest{UserIDs: []string{"user-1", "missing"}})
	if err == nil {
		t.Fatal("BulkDisableUsers() error = nil")
	}
	var stored model.User
	if err := db.First(&stored, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.UserStatusActive {
		t.Fatalf("user status = %q, want active", stored.Status)
	}
}

func TestBulkDisableUsersRejectsCurrentAdmin(t *testing.T) {
	db := newBulkUserTestDB(t)
	actor := model.User{ID: "admin-1", Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	otherAdmin := model.User{ID: "admin-2", Username: "admin-two", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(&[]model.User{actor, otherAdmin}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := (&Service{repo: repository.New(db)}).BulkDisableUsers(&actor, BulkDisableUsersRequest{UserIDs: []string{actor.ID}}); err == nil {
		t.Fatal("BulkDisableUsers() error = nil")
	}
}

func newBulkUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AuthSession{}, &model.AdminAuditEvent{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.SystemSetting{}, &model.TaskTextDelta{}); err != nil {
		t.Fatal(err)
	}
	return db
}
