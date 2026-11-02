package app

import (
	"infinite-canvas/backend/internal/model"
	"testing"
)

func TestReferralAdminPermissionsAndUserScope(t *testing.T) {
	s, _, admin := quotaTestService(t, "sqlite")
	p := defaultReferralPolicy()
	for _, user := range []*model.User{nil, {ID: "normal", Role: model.UserRoleUser}, {ID: "scoped", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionUsers}}} {
		if _, err := s.UpdateReferralPolicy(user, p); err == nil {
			t.Fatal("unauthorized policy update")
		}
		if _, err := s.AdminSetReferralUserPolicy(user, admin.ID, &p, false); err == nil {
			t.Fatal("unauthorized personal policy update")
		}
		if _, err := s.AdminReferralUser(user, admin.ID); err == nil {
			t.Fatal("unauthorized profile read")
		}
		if _, err := s.AdminReviewReferralReward(user, "reward", true, ""); err == nil {
			t.Fatal("unauthorized reward approval")
		}
	}
	if _, err := s.AdminSetReferralUserPolicy(admin, "missing-user", &p, false); err == nil {
		t.Fatal("policy created for missing user")
	}
	if _, err := s.ReferralDashboard(nil); err == nil {
		t.Fatal("anonymous dashboard accepted")
	}
}

func TestReferralPaymentViewIncludesNewcomerSnapshot(t *testing.T) {
	view := paymentOrderView(model.PaymentOrder{ID: "order", ReferralPolicyVersionID: "pinned-rule", NewcomerBonusMicrocredits: 60_000_000, CreditsMicrocredits: 1_200_000_000})
	if view.ReferralPolicyVersionID != "pinned-rule" || view.NewcomerBonusMicrocredits != 60_000_000 || view.CreditsMicrocredits != 1_200_000_000 {
		t.Fatalf("payment view lost reward snapshot: %+v", view)
	}
}
