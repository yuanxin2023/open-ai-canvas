package repository

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/referrals"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

func TestReferralPostgresConcurrentSettlement(t *testing.T) {
	dsn := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("CANVAS_TEST_POSTGRES_DSN is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("referral_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE"); sqlDB, _ := admin.DB(); sqlDB.Close() })
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&model.User{}, &model.SystemSetting{}, &model.ReferralProfile{}, &model.ReferralReward{}, &model.ReferralPolicyVersion{}, &model.PaymentOrder{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	p := referrals.Default()
	p.NewcomerEnabled = true
	p.PerInviteeCapMicrocredits = 1000 * referrals.Scale
	seedReferralPolicy(t, db, p)
	if err := db.Create(&[]model.User{{ID: "inviter", Username: "inviter", Status: model.UserStatusActive}, {ID: "invitee", Username: "invitee", Status: model.UserStatusActive}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]model.ReferralProfile{{UserID: "inviter", Code: "ABC234"}, {UserID: "invitee", Code: "DEF567", InviterID: "inviter", BoundPolicyVersionID: "test-policy"}}).Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("order-%d", i)
		if err := db.Create(&model.PaymentOrder{ID: id, UserID: "invitee", IdempotencyKey: id, MerchantOrderNo: id, ProviderID: "test", AmountFen: 9900, Currency: "CNY", CreditsMicrocredits: 1200 * referrals.Scale, Status: model.PaymentOrderPending}).Error; err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("order-%d", n%8)
			_, _, err := repo.CompletePaymentOrderWithReferral("test", id, PaymentEvidence{AmountFen: 9900, Currency: "CNY", ProviderTradeNo: "trade-" + id}, referrals.Evaluate)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var rewards []model.ReferralReward
	if err := db.Order("order_ordinal asc").Find(&rewards).Error; err != nil {
		t.Fatal(err)
	}
	var sum int64
	for i, r := range rewards {
		sum += r.RewardMicrocredits
		if r.OrderOrdinal != int64(i+1) {
			t.Fatalf("ordinal gap/duplicate: %+v", rewards)
		}
	}
	if len(rewards) != 8 || sum != 1000*referrals.Scale {
		t.Fatalf("concurrent cap: count=%d sum=%d", len(rewards), sum)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", "invitee").Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits != 9660*referrals.Scale {
		t.Fatalf("duplicate recharge or first bonus: %d", account.AvailableMicrocredits)
	}
}
