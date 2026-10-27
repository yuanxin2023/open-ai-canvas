package auth

import (
	"context"
	"testing"

	"infinite-canvas/backend/internal/model"
)

type bindingTestSMS struct{ code string }

func (*bindingTestSMS) Available(string) (bool, error) { return true, nil }
func (sms *bindingTestSMS) SendCode(_ context.Context, _, _, code string) error {
	sms.code = code
	return nil
}

func TestBindingProofRequiresSameActiveSessionAndCredentials(t *testing.T) {
	for _, scenario := range []string{"success", "other-session", "password-changed", "session-revoked"} {
		t.Run(scenario, func(t *testing.T) {
			s, db, user, cookie := newProfileTestService(t)
			delivery := &bindingTestSMS{}
			s.SetSMSDelivery(delivery)
			ticket, err := s.StartVerification(context.Background(), user, VerificationRequest{Purpose: "bind", Method: "sms", Phone: "13812345678", Password: "old-password"}, cookie)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "other-session":
				cookie = "session-two.token"
			case "password-changed":
				if err := s.ChangeProfilePassword(user, cookie, ProfilePasswordRequest{CurrentPassword: "old-password", NewPassword: "new-password"}); err != nil {
					t.Fatal(err)
				}
			case "session-revoked":
				if err := db.Delete(&model.AuthSession{}, "id = ?", "session-one").Error; err != nil {
					t.Fatal(err)
				}
			}
			_, err = s.BindVerification(user, VerificationConfirm{Ticket: ticket.Ticket, SMSCode: delivery.code}, cookie)
			if (err == nil) != (scenario == "success") {
				t.Fatalf("binding result for %s: %v", scenario, err)
			}
			stored, err := s.repo.User(user.ID)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "success" {
				if stored.Phone != "+8613812345678" || stored.PhoneVerifiedAt == nil {
					t.Fatal("valid binding lost verified phone")
				}
			} else if stored.Phone != "" {
				t.Fatal("invalid proof changed phone")
			}
		})
	}
}
