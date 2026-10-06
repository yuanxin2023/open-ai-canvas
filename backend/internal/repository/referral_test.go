package repository

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func TestReferralRewardPaymentCapAndApprovalAreAtomic(t *testing.T) {
	db := openPaymentTestDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.ReferralProfile{}, &model.ReferralReward{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	users := []model.User{
		{ID: "inviter", Username: "inviter", Status: model.UserStatusActive},
		{ID: "invitee", Username: "invitee", Status: model.UserStatusActive},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	profiles := []model.ReferralProfile{
		{UserID: "inviter", Code: "INVITERCODE1"},
		{UserID: "invitee", Code: "INVITEECODE1", InviterID: "inviter"},
	}
	if err := db.Create(&profiles).Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	for i, want := range []int64{1_000_000, 500_000, 0} {
		id := fmt.Sprintf("referral-order-%d", i)
		order := model.PaymentOrder{
			ID: id, UserID: "invitee", IdempotencyKey: id, MerchantOrderNo: id,
			ProviderID: "wechat-native", AmountFen: 100, Currency: "CNY",
			CreditsMicrocredits: 10_000_000, Status: model.PaymentOrderPending,
		}
		if err := db.Create(&order).Error; err != nil {
			t.Fatal(err)
		}
		draft := &model.ReferralReward{
			ID: "reward-" + id, PaymentOrderID: id, InviterID: "inviter", InviteeID: "invitee",
			AmountFen: 100, RewardMicrocredits: 1_000_000, CapMicrocredits: 1_500_000,
			RateBPS: 1_000, Status: model.ReferralRewardPending,
		}
		evidence := PaymentEvidence{ProviderTradeNo: "trade-" + id, ProviderStatus: "SUCCESS", AmountFen: 100, Currency: "CNY", PaidAt: time.Now()}
		if _, granted, err := repo.CompletePaymentOrder(order.ProviderID, id, evidence, draft); err != nil || !granted {
			t.Fatalf("complete order %s: granted=%v err=%v", id, granted, err)
		}
		if _, granted, err := repo.CompletePaymentOrder(order.ProviderID, id, evidence, draft); err != nil || granted {
			t.Fatalf("duplicate order %s: granted=%v err=%v", id, granted, err)
		}
		var reward model.ReferralReward
		err := db.First(&reward, "payment_order_id = ?", id).Error
		if want == 0 {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("third order reward = %#v, err=%v", reward, err)
			}
			continue
		}
		if err != nil || reward.RewardMicrocredits != want || reward.Status != model.ReferralRewardPending {
			t.Fatalf("reward for %s = %#v, err=%v, want %d pending", id, reward, err, want)
		}
		audit := &model.AdminAuditEvent{ID: "audit-" + id, ActorUserID: "admin", Action: "referral.reward.approve"}
		if _, err := repo.ReviewReferralReward(reward.ID, "admin", true, "approved", audit); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.ReviewReferralReward(reward.ID, "admin", true, "again", audit); !errors.Is(err, ErrReferralReviewConflict) {
			t.Fatalf("duplicate approval error = %v", err)
		}
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", "inviter").Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits != 1_500_000 {
		t.Fatalf("inviter balance = %d, want 1500000", account.AvailableMicrocredits)
	}
	var count int64
	if err := db.Model(&model.CreditLedgerEntry{}).Where("user_id = ? AND type = ?", "inviter", model.CreditLedgerReferral).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("referral ledger count = %d, err=%v", count, err)
	}
}
