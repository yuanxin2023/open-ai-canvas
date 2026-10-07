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

	result, err := svc.RegisterWithEnvironment(RegisterRequest{Email: " Creator.Name@Example.com ", Password: "strong-password"}, LoginEnvironment{
		IPAddress: "203.0.113.9", UserAgent: "test-agent", DeviceType: "电脑", Browser: "Chrome", BrowserVersion: "140", OS: "Windows", OSVersion: "11",
	})
	if err != nil {
		t.Fatal(err)
	}
	user := result.User.User
	if len(user.ID) != 32 || user.Username != "creato" {
		t.Fatalf("generated identity = %#v", user)
	}
	if user.Email != "creator.name@example.com" || user.DisplayName != user.Username {
		t.Fatalf("normalized profile = %#v", user)
	}
	if user.Role != model.UserRoleAdmin || user.Status != model.UserStatusActive {
		t.Fatalf("first user role/status = %s/%s", user.Role, user.Status)
	}
	if user.AdminLevel != model.AdminLevelFull {
		t.Fatalf("first user admin level = %q, want full", user.AdminLevel)
	}
	if len(host.userIDs) != 1 || host.userIDs[0] != user.ID {
		t.Fatalf("signup bonus user IDs = %#v", host.userIDs)
	}
	var stored model.User
	if err := db.First(&stored, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Username != user.Username || stored.Email != user.Email || stored.DisplayName != user.Username {
		t.Fatalf("stored user = %#v", stored)
	}
	if stored.RegistrationIP != "203.0.113.9" {
		t.Fatalf("registration IP = %q", stored.RegistrationIP)
	}
	if stored.UsernameCustomizedAt != nil || result.User.UsernameChangePolicy.Customized {
		t.Fatalf("generated username was marked customized: stored=%#v policy=%#v", stored.UsernameCustomizedAt, result.User.UsernameChangePolicy)
	}
	for _, account := range []string{user.Username, strings.ToUpper(user.Username), user.Email} {
		if _, err := svc.Login(LoginRequest{Username: account, Password: "strong-password"}); err != nil {
			t.Fatalf("login with generated account %q failed: %v", account, err)
		}
	}
	var loginEvent model.UserLoginEvent
	if err := db.First(&loginEvent, "user_id = ? AND login_method = ?", user.ID, "email_register").Error; err != nil {
		t.Fatal(err)
	}
	if loginEvent.LoginMethod != "email_register" || loginEvent.IPAddress != stored.RegistrationIP || loginEvent.Browser != "Chrome" || loginEvent.OS != "Windows" {
		t.Fatalf("registration login event = %#v", loginEvent)
	}
}

func TestRegisterGeneratesValidFallbackUsernames(t *testing.T) {
	for _, email := range []string{"123456@example.com", "admin@example.com", "a.b+c@example.com", "verylongprefix@example.com"} {
		t.Run(email, func(t *testing.T) {
			svc, _ := newPasswordResetTestService(t)
			result, err := svc.Register(RegisterRequest{Email: email, Password: "strong-password"})
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateUsername(result.User.Username); err != nil {
				t.Fatalf("generated username %q is invalid: %v", result.User.Username, err)
			}
			for _, account := range []string{result.User.Username, email} {
				if _, err := svc.Login(LoginRequest{Username: account, Password: "strong-password"}); err != nil {
					t.Fatalf("login with %q failed: %v", account, err)
				}
			}
		})
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
	if result.User.Role != model.UserRoleUser || result.User.Username != "member" || result.User.DisplayName != "member" {
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

func TestRegisterRetriesConflictingGeneratedUsername(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	admin := model.User{ID: "admin", Username: "member", Email: "admin@example.com", DisplayName: "member", ProfileName: "member", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SystemSetting{Key: registrationSettingKey, ValueJSON: `{"enabled":true}`}).Error; err != nil {
		t.Fatal(err)
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
	if result.User.Username == "member" || !strings.HasPrefix(result.User.Username, "mem") {
		t.Fatalf("conflicting default username was not retried: %q", result.User.Username)
	}
	for _, account := range []string{result.User.Username, result.User.Email} {
		if _, err := svc.Login(LoginRequest{Username: account, Password: "strong-password"}); err != nil {
			t.Fatalf("login with retried account %q failed: %v", account, err)
		}
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
	if _, err := svc.LoginWithEnvironment(LoginRequest{Username: user.Email, Password: "strong-password"}, LoginEnvironment{IPAddress: "2001:db8::8", DeviceType: "手机", Browser: "Safari", OS: "iOS"}); err != nil {
		t.Fatal(err)
	}
	var event model.UserLoginEvent
	if err := db.Where("user_id = ?", user.ID).Order("created_at desc").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.LoginMethod != "password" || event.IPAddress != "2001:db8::8" || event.DeviceType != "手机" {
		t.Fatalf("login event = %#v", event)
	}
}
