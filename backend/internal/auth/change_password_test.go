package auth

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"golang.org/x/crypto/bcrypt"
)

func TestChangePasswordUpdatesHashAndKeepsOnlyCurrentSession(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	oldHash, err := HashPassword("old-password")
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{ID: "password-user", Username: "password-user", PasswordHash: oldHash, Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	currentToken := "current-session-token"
	sessions := []model.AuthSession{
		{ID: "current-session", UserID: user.ID, TokenHash: HashToken(currentToken), ExpiresAt: time.Now().Add(time.Hour)},
		{ID: "other-session", UserID: user.ID, TokenHash: HashToken("other-token"), ExpiresAt: time.Now().Add(time.Hour)},
	}
	if err := db.Create(&sessions).Error; err != nil {
		t.Fatal(err)
	}

	if err := svc.ChangePassword(&user, "current-session."+currentToken, ChangePasswordRequest{CurrentPassword: "old-password", NewPassword: "new-password"}); err != nil {
		t.Fatal(err)
	}
	var stored model.User
	if err := db.First(&stored, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("new-password")) != nil {
		t.Fatal("new password hash does not match")
	}
	var remaining []model.AuthSession
	if err := db.Where("user_id = ?", user.ID).Find(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].ID != "current-session" {
		t.Fatalf("remaining sessions = %#v", remaining)
	}
}

func TestChangePasswordRejectsWrongCurrentPasswordAndInvalidNewPassword(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	oldHash, err := HashPassword("old-password")
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{ID: "password-user", Username: "password-user", PasswordHash: oldHash, Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	token := "current-session-token"
	if err := db.Create(&model.AuthSession{ID: "current-session", UserID: user.ID, TokenHash: HashToken(token), ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}

	for _, req := range []ChangePasswordRequest{
		{CurrentPassword: "wrong-password", NewPassword: "new-password"},
		{CurrentPassword: "old-password", NewPassword: "short"},
		{CurrentPassword: "old-password", NewPassword: "old-password"},
	} {
		if err := svc.ChangePassword(&user, "current-session."+token, req); err == nil {
			t.Fatalf("ChangePassword(%#v) error = nil", req)
		}
	}
	var stored model.User
	if err := db.First(&stored, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("old-password")) != nil {
		t.Fatal("password changed after rejected request")
	}
}
