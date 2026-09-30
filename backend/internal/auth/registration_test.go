package auth

import (
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

type signupBonusRecordingHost struct {
	nopHost
	userIDs []string
}

func (h *signupBonusRecordingHost) EnsureSignupBonus(userID string) error {
	h.userIDs = append(h.userIDs, userID)
	return nil
}

func TestRegisterFirstUserUsesEmailIdentityWithoutVerification(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	host := &signupBonusRecordingHost{}
	svc.host = host
	settings, err := svc.PublicAuthSettings()
	if err != nil || !settings.FirstUser || !settings.EmailFirstRegistration {
		t.Fatalf("public auth settings = %#v, err = %v", settings, err)
	}

	result, err := svc.Register(RegisterRequest{Email: " Creator.Name@Example.com ", Password: "strong-password"})
	if err != nil {
		t.Fatal(err)
	}
	user := result.User.User
	if len(user.ID) != 32 || user.Username != user.ID {
		t.Fatalf("generated identity = %#v", user)
	}
	if user.Email != "creator.name@example.com" || user.DisplayName != "creator.name" {
		t.Fatalf("normalized profile = %#v", user)
	}
	if user.Role != model.UserRoleAdmin || user.Status != model.UserStatusActive {
		t.Fatalf("first user role/status = %s/%s", user.Role, user.Status)
	}
	if len(host.userIDs) != 1 || host.userIDs[0] != user.ID {
		t.Fatalf("signup bonus user IDs = %#v", host.userIDs)
	}
	var stored model.User
	if err := db.First(&stored, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Username != user.ID || stored.Email != user.Email || stored.DisplayName != user.DisplayName {
		t.Fatalf("stored user = %#v", stored)
	}
}

func TestRegisterRequiresEmailForFirstUser(t *testing.T) {
	svc, _ := newPasswordResetTestService(t)
	if _, err := svc.Register(RegisterRequest{Password: "strong-password"}); err == nil || !strings.Contains(err.Error(), "请输入邮箱") {
		t.Fatalf("missing email error = %v", err)
	}
	if _, err := svc.Register(RegisterRequest{Email: "invalid", Password: "strong-password"}); err == nil || !strings.Contains(err.Error(), "邮箱格式不正确") {
		t.Fatalf("invalid email error = %v", err)
	}
}

func TestRegisterOrdinaryUserConsumesEmailCode(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	admin := model.User{ID: "admin", Username: "admin", Email: "admin@example.com", DisplayName: "Admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SystemSetting{Key: registrationSettingKey, ValueJSON: `{"enabled":true}`}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(RegisterRequest{Email: "member@example.com", Password: "strong-password"}); err == nil || !strings.Contains(err.Error(), "验证码") {
		t.Fatalf("missing registration code error = %v", err)
	}
	var deliveredCode string
	svc.SetMailSender(func(_ EmailSettingValue, _, _ string, body string) error {
		deliveredCode = codeFromEmailBody(body)
		return nil
	})
	if err := svc.SendRegistrationEmailCode("member@example.com"); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Register(RegisterRequest{Email: "member@example.com", EmailCode: deliveredCode, Password: "strong-password"})
	if err != nil {
		t.Fatal(err)
	}
	if result.User.Role != model.UserRoleUser || result.User.Username != result.User.ID || result.User.DisplayName != "member" {
		t.Fatalf("registered user = %#v", result.User)
	}
	var code model.EmailVerificationCode
	if err := db.Where("email = ? AND purpose = ?", "member@example.com", registrationEmailPurpose).First(&code).Error; err != nil {
		t.Fatal(err)
	}
	if code.UsedAt == nil {
		t.Fatal("registration code was not consumed")
	}
}

func TestRegisterRejectsOrdinaryUserWhenRegistrationIsClosed(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	admin := model.User{ID: "admin", Username: "admin", Email: "admin@example.com", DisplayName: "Admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(RegisterRequest{Email: "member@example.com", EmailCode: "123456", Password: "strong-password"}); err == nil || !strings.Contains(err.Error(), "未开放") {
		t.Fatalf("closed registration error = %v", err)
	}
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("closed registration created a user: count=%d err=%v", count, err)
	}
}

func TestRegisterRejectsDuplicateEmailAfterVerification(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	existing := model.User{ID: "existing", Username: "existing", Email: "member@example.com", DisplayName: "Member", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SystemSetting{Key: registrationSettingKey, ValueJSON: `{"enabled":true}`}).Error; err != nil {
		t.Fatal(err)
	}
	hash, err := svc.emailVerificationCodeHash(registrationEmailPurpose, existing.Email, "123456")
	if err != nil {
		t.Fatal(err)
	}
	code := model.EmailVerificationCode{ID: "duplicate-email-code", Email: existing.Email, Purpose: registrationEmailPurpose, CodeHash: hash, ExpiresAt: time.Now().Add(time.Minute), CreatedAt: time.Now()}
	if err := db.Create(&code).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(RegisterRequest{Email: "MEMBER@example.com", EmailCode: "123456", Password: "strong-password"}); err == nil || !strings.Contains(err.Error(), "邮箱已被注册") {
		t.Fatalf("duplicate email error = %v", err)
	}
}

func TestLoginKeepsUsernameAndEmailCompatibility(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	hash, err := HashPassword("strong-password")
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{ID: "existing-user", Username: "legacy-name", Email: "legacy@example.com", DisplayName: "Legacy", PasswordHash: hash, Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"legacy-name", "LEGACY@example.com"} {
		if _, err := svc.Login(LoginRequest{Username: account, Password: "strong-password"}); err != nil {
			t.Fatalf("login with %q failed: %v", account, err)
		}
	}
}
