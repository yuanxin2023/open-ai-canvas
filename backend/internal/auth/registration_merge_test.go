package auth

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
)

type registrationReferralHost struct{ profileTestHost }

func (registrationReferralHost) ReferralEnabled() (bool, error) { return true, nil }

func combinedRegistrationFixture(t *testing.T) (*Service, *gorm.DB, RegisterRequest) {
	t.Helper()
	s, db, inviter, _ := newProfileTestService(t)
	s.host = registrationReferralHost{}
	if err := db.AutoMigrate(&model.ReferralProfile{}); err != nil {
		t.Fatal(err)
	}
	policy := VerificationPolicy{SMSAndEmailRegistration: true}
	raw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{
		&model.SystemSetting{Key: registrationSettingKey, ValueJSON: `{"enabled":true}`},
		&model.SystemSetting{Key: authPolicyKey, ValueJSON: string(raw)},
		&model.ReferralProfile{UserID: inviter.ID, Code: "ABC234"},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	secret := strings.Repeat("s", 64)
	v := model.AuthVerification{ID: "combined-register", TokenHash: HashToken(secret), Purpose: "register", Method: "sms_email", Email: "member@example.com", Phone: "+8613812345678", PolicyHash: policy.hash(), Ready: true, ExpiresAt: time.Now().Add(time.Hour)}
	v.EmailHash, err = s.verificationHash(v.ID + ":email:" + v.Email + ":654321")
	if err != nil {
		t.Fatal(err)
	}
	v.PhoneHash, err = s.verificationHash(v.ID + ":sms:" + v.Phone + ":123456")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&v).Error; err != nil {
		t.Fatal(err)
	}
	return s, db, RegisterRequest{Ticket: v.ID + "." + secret, Email: v.Email, Phone: "13812345678", SMSCode: "123456", EmailCode: "654321", Password: "test-password", ReferralCode: "ABC234", AcceptedTerms: true}
}

func TestCombinedRegistrationPreservesGeneratedIdentityAndReferral(t *testing.T) {
	s, db, req := combinedRegistrationFixture(t)
	result, err := s.Register(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateUsername(result.User.Username); err != nil || result.User.UsernameCustomizedAt != nil || result.User.UsernameChangePolicy.Customized {
		t.Fatalf("generated identity contract changed: %#v %v", result.User, err)
	}
	if result.User.EmailVerifiedAt == nil || result.User.PhoneVerifiedAt == nil || result.User.Phone != "+8613812345678" {
		t.Fatal("verified contacts lost")
	}
	var profile model.ReferralProfile
	if err := db.First(&profile, "user_id = ?", result.User.ID).Error; err != nil || profile.InviterID != "profile-user" {
		t.Fatalf("referral binding: %#v %v", profile, err)
	}
	if _, err := s.Register(req); err == nil {
		t.Fatal("consumed registration ticket reused")
	}
}

func TestCombinedRegistrationRejectsPartialOrChangedProofsWithoutCreatingAccount(t *testing.T) {
	for _, scenario := range []string{"no-consent", "wrong-sms", "wrong-email", "no-ticket", "changed-phone", "changed-domain-policy", "invalid-referral"} {
		t.Run(scenario, func(t *testing.T) {
			s, db, req := combinedRegistrationFixture(t)
			switch scenario {
			case "no-consent":
				req.AcceptedTerms = false
			case "wrong-sms":
				req.SMSCode = "000000"
			case "wrong-email":
				req.EmailCode = "000000"
			case "no-ticket":
				req.Ticket = ""
			case "changed-phone":
				req.Phone = "13812345679"
			case "changed-domain-policy":
				if err := db.Model(&model.SystemSetting{}).Where("key = ?", emailSettingKey).Update("value_json", `{"enabled":true,"host":"smtp.example.com","port":587,"fromEmail":"sender@gmail.com","registrationAllowedDomains":["gmail.com"]}`).Error; err != nil {
					t.Fatal(err)
				}
			case "invalid-referral":
				req.ReferralCode = "ZZZZZZ"
			}
			if _, err := s.Register(req); err == nil {
				t.Fatal("invalid registration accepted")
			}
			var count int64
			if err := db.Model(&model.User{}).Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("failed registration created account: count=%d err=%v", count, err)
			}
			var v model.AuthVerification
			if err := db.First(&v, "id = ?", "combined-register").Error; err != nil || v.UsedAt != nil {
				t.Fatalf("failure consumed ticket: %#v %v", v, err)
			}
			if scenario == "no-consent" && v.Attempts != 0 {
				t.Fatal("unaccepted terms attempted verification")
			}
		})
	}
}

func TestDefaultVerificationPolicyKeepsSMSAndCombinedDisabled(t *testing.T) {
	s, _ := newPasswordResetTestService(t)
	policy, err := s.verificationPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if policy.SMSLogin || policy.SMSRegistration || policy.SMSAndEmailRegistration || !policy.EmailRegistration {
		t.Fatalf("unexpected default policy: %#v", policy)
	}
}
