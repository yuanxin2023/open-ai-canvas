package repository

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/referrals"
	"testing"
	"time"
)

func referralTestRepository(t *testing.T, p referrals.Policy) (*Repository, *gorm.DB) {
	t.Helper()
	db := openPaymentTestDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.ReferralProfile{}, &model.ReferralReward{}); err != nil {
		t.Fatal(err)
	}
	seedReferralPolicy(t, db, p)
	if err := db.Create(&[]model.User{{ID: "inviter", Username: "inviter", Status: model.UserStatusActive}, {ID: "invitee", Username: "invitee", Status: model.UserStatusActive}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]model.ReferralProfile{{UserID: "inviter", Code: "ABC234"}, {UserID: "invitee", Code: "DEF567", InviterID: "inviter", BoundPolicyVersionID: "test-policy"}}).Error; err != nil {
		t.Fatal(err)
	}
	return New(db), db
}

func referralTestPay(t *testing.T, repo *Repository, db *gorm.DB, n int, evaluator ReferralEvaluator) (*model.PaymentOrder, bool, error) {
	t.Helper()
	id := fmt.Sprintf("order-%d", n)
	order := model.PaymentOrder{ID: id, UserID: "invitee", IdempotencyKey: id, MerchantOrderNo: id, ProviderID: "test", AmountFen: 9900, Currency: "CNY", CreditsMicrocredits: 1200 * referrals.Scale, Status: model.PaymentOrderPending}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	return repo.CompletePaymentOrderWithReferral("test", id, PaymentEvidence{AmountFen: 9900, Currency: "CNY", ProviderTradeNo: "trade-" + id, PaidAt: time.Now()}, evaluator)
}

func TestReferralNewcomerVersionIsolationAndRejectedCap(t *testing.T) {
	p := referrals.Default()
	p.NewcomerEnabled = true
	repo, db := referralTestRepository(t, p)
	order, granted, err := referralTestPay(t, repo, db, 1, referrals.Evaluate)
	if err != nil || !granted || order.NewcomerBonusMicrocredits != 60*referrals.Scale {
		t.Fatalf("first recharge: %+v %v", order, err)
	}
	if _, again, err := repo.CompletePaymentOrderWithReferral("test", order.MerchantOrderNo, PaymentEvidence{AmountFen: 9900, Currency: "CNY", ProviderTradeNo: "trade-" + order.ID}, referrals.Evaluate); err != nil || again {
		t.Fatalf("replay: %v %v", again, err)
	}
	p.Enabled = true
	p.Stages = []referrals.Stage{{RateBPS: 10000}}
	published, err := repo.SaveReferralPolicy(p, "", nil, "admin", &model.AdminAuditEvent{ID: "publish"})
	if err != nil {
		t.Fatal(err)
	}
	if published.VersionID == "test-policy" {
		t.Fatal("version reused")
	}
	if _, _, err := referralTestPay(t, repo, db, 2, referrals.Evaluate); err != nil {
		t.Fatal(err)
	}
	var first, second model.ReferralReward
	if err := db.First(&first, "payment_order_id = ?", "order-1").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&second, "payment_order_id = ?", "order-2").Error; err != nil {
		t.Fatal(err)
	}
	if first.RewardMicrocredits != 240*referrals.Scale || second.RewardMicrocredits != 60*referrals.Scale || second.RateBPS != 2000 || second.OrderOrdinal != 2 || second.PolicyVersionID != "test-policy" {
		t.Fatalf("snapshots: %+v %+v", first, second)
	}
	if _, err := repo.ReviewReferralReward(first.ID, "admin", false, "invalid", &model.AdminAuditEvent{ID: "reject"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := referralTestPay(t, repo, db, 3, referrals.Evaluate); err != nil {
		t.Fatal(err)
	}
	var third model.ReferralReward
	if err := db.First(&third, "payment_order_id = ?", "order-3").Error; err != nil {
		t.Fatal(err)
	}
	if third.RewardMicrocredits != 240*referrals.Scale {
		t.Fatalf("rejection did not release future cap: %+v", third)
	}
	var wallet model.CreditAccount
	if err := db.First(&wallet, "user_id = ?", "invitee").Error; err != nil {
		t.Fatal(err)
	}
	if wallet.AvailableMicrocredits != 3660*referrals.Scale {
		t.Fatalf("newcomer repeated or missing: %+v", wallet)
	}
	var old model.ReferralReward
	if err := db.First(&old, "id = ?", second.ID).Error; err != nil || old.RewardMicrocredits != 60*referrals.Scale {
		t.Fatal("prior truncated reward changed")
	}
}

func TestReferralSettlementRollbackIncludesTopupAndBonus(t *testing.T) {
	p := referrals.Default()
	p.NewcomerEnabled = true
	repo, db := referralTestRepository(t, p)
	if err := db.Create(&model.CreditAccount{UserID: "invitee", AvailableMicrocredits: maxPaymentCreditBalance - 1200*referrals.Scale}).Error; err != nil {
		t.Fatal(err)
	}
	_, granted, err := referralTestPay(t, repo, db, 1, referrals.Evaluate)
	if err == nil || granted {
		t.Fatal("bonus overflow accepted")
	}
	var order model.PaymentOrder
	db.First(&order, "id = ?", "order-1")
	if order.Status != model.PaymentOrderPending {
		t.Fatalf("failed grant committed: %+v", order)
	}
	var count int64
	db.Model(&model.CreditLedgerEntry{}).Count(&count)
	if count != 0 {
		t.Fatalf("rollback left %d ledger rows", count)
	}
	_, _, err = referralTestPay(t, repo, db, 2, func(referrals.Policy, int64, int64, int64, time.Time, time.Time) (referrals.Result, error) {
		return referrals.Result{}, errors.New("injected calculation failure")
	})
	if err == nil {
		t.Fatal("calculation failure swallowed")
	}
}

func TestReferralPauseKeepsSuccessfulOrderCount(t *testing.T) {
	p := referrals.Default()
	p.NewcomerEnabled = true
	p.MaxOrders = 1
	repo, db := referralTestRepository(t, p)
	if err := repo.UpdateReferralProfileControls("inviter", true, false, &model.AdminAuditEvent{ID: "pause"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := referralTestPay(t, repo, db, 1, referrals.Evaluate); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateReferralProfileControls("inviter", false, false, &model.AdminAuditEvent{ID: "resume"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := referralTestPay(t, repo, db, 2, referrals.Evaluate); err != nil {
		t.Fatal(err)
	}
	var rewards, bonus int64
	db.Model(&model.ReferralReward{}).Count(&rewards)
	db.Model(&model.CreditLedgerEntry{}).Where("type = ?", model.CreditLedgerReferral).Count(&bonus)
	if rewards != 0 || bonus != 0 {
		t.Fatalf("pause reset sequence or granted late first bonus: %d %d", rewards, bonus)
	}
}

func TestReferralPersonalPolicyPinsNewBindings(t *testing.T) {
	repo, db := referralTestRepository(t, referrals.Default())
	p := referrals.Default()
	p.Enabled = true
	p.Mode = "time"
	p.Stages = []referrals.Stage{{Until: 30, RateBPS: 3000}}
	if err := repo.SaveReferralUserPolicy("inviter", &p, false, "admin", &model.AdminAuditEvent{ID: "personal"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{ID: "new-user", Username: "new-user", Status: model.UserStatusActive}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return bindReferralTx(tx, "new-user", "ABC234") }); err != nil {
		t.Fatal(err)
	}
	newProfile, err := repo.ReferralProfileForInvitee("new-user")
	if err != nil {
		t.Fatal(err)
	}
	bound, err := repo.ReferralPolicyVersion(newProfile.BoundPolicyVersionID)
	if err != nil || bound.Mode != "time" || bound.Stages[0].RateBPS != 3000 {
		t.Fatalf("personal binding: %+v %v", bound, err)
	}
	if err := repo.SaveReferralUserPolicy("inviter", nil, false, "admin", &model.AdminAuditEvent{ID: "clear"}); err != nil {
		t.Fatal(err)
	}
	after, err := repo.ReferralProfileForInvitee("new-user")
	if err != nil || after.BoundPolicyVersionID != newProfile.BoundPolicyVersionID {
		t.Fatal("clear mutated existing binding")
	}
	if err := repo.UpdateReferralProfileControls("inviter", true, false, &model.AdminAuditEvent{ID: "pause"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return bindReferralTx(tx, "another", "ABC234") }); !errors.Is(err, ErrReferralCodeInvalid) {
		t.Fatalf("paused binding accepted: %v", err)
	}
}
