package database

import (
	"encoding/json"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/referrals"
	"testing"
	"time"
)

func TestReferralPolicyMigrationPreservesMoneyAndUnifiesBindings(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:referral-policy-migration?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.ReferralProfile{}, &model.ReferralReward{}, &model.PaymentOrder{}, &model.CreditAccount{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SystemSetting{Key: referrals.SettingKey, ValueJSON: `{"enabled":true,"globalRateBps":2000,"creditsPerYuanMicro":1000000,"perInviteeCapMicrocredits":300000000}`}).Error; err != nil {
		t.Fatal(err)
	}
	rate := int64(5000)
	stamp := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := db.Create(&[]model.ReferralProfile{{UserID: "inviter", Code: "ABC234", RateBPS: &rate}, {UserID: "invitee", Code: "DEF567", InviterID: "inviter", CreatedAt: stamp, UpdatedAt: stamp}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ReferralReward{ID: "old-reward", PaymentOrderID: "old-order", InviterID: "inviter", InviteeID: "invitee", RewardMicrocredits: 12_340_000, Status: model.ReferralRewardPending}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: "inviter", AvailableMicrocredits: 17_000_000}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateReferralPolicies(db); err != nil {
		t.Fatal(err)
	}
	var profile model.ReferralProfile
	db.First(&profile, "user_id = ?", "invitee")
	if !profile.CreatedAt.Equal(stamp) || !profile.UpdatedAt.Equal(stamp) {
		t.Fatal("migration changed relationship timestamps")
	}
	var version model.ReferralPolicyVersion
	if err := db.First(&version, "id = ?", profile.BoundPolicyVersionID).Error; err != nil {
		t.Fatal(err)
	}
	var policy referrals.Policy
	if err := json.Unmarshal([]byte(version.PolicyJSON), &policy); err != nil {
		t.Fatal(err)
	}
	if !policy.Enabled || policy.Stages[0].RateBPS != 2000 || policy.NewcomerEnabled || policy.PerInviteeCapMicrocredits != 300_000_000 {
		t.Fatalf("migration policy: %+v", policy)
	}
	var reward model.ReferralReward
	db.First(&reward, "id = ?", "old-reward")
	var account model.CreditAccount
	db.First(&account, "user_id = ?", "inviter")
	if reward.RewardMicrocredits != 12_340_000 || reward.Status != model.ReferralRewardPending || account.AvailableMicrocredits != 17_000_000 {
		t.Fatal("migration changed historical money")
	}
}
