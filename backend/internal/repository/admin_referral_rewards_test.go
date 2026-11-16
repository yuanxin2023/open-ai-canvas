package repository

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func TestAdminReferralRewardsJoinsAndPagination(t *testing.T) {
	db := openPaymentTestDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.ReferralReward{}); err != nil {
		t.Fatal(err)
	}
	users := []model.User{{ID: "inviter", Username: "alice", Email: "alice@example.com"}, {ID: "invitee", Username: "bob"}}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	orders := []model.PaymentOrder{
		{ID: "original", UserID: "invitee", IdempotencyKey: "one", MerchantOrderNo: "MERCHANT-ONE", AmountFen: 1234, Currency: "CNY", Status: model.PaymentOrderCredited, CreditedAt: &stamp},
		{ID: "newer", UserID: "invitee", IdempotencyKey: "two", MerchantOrderNo: "MERCHANT-TWO", AmountFen: 9999, Currency: "CNY", Status: model.PaymentOrderPending},
		{ID: "foreign", UserID: "inviter", IdempotencyKey: "three", MerchantOrderNo: "MERCHANT-FOREIGN"},
	}
	if err := db.Create(&orders).Error; err != nil {
		t.Fatal(err)
	}
	rewards := []model.ReferralReward{
		{ID: "a", InviterID: "inviter", InviteeID: "invitee", PaymentOrderID: "original", Status: model.ReferralRewardPending, CreatedAt: stamp},
		{ID: "b", InviterID: "inviter", InviteeID: "invitee", PaymentOrderID: "newer", Status: model.ReferralRewardApproved, CreatedAt: stamp},
		{ID: "c", InviterID: "inviter", InviteeID: "invitee", PaymentOrderID: "foreign", Status: model.ReferralRewardRejected, CreatedAt: stamp},
		{ID: "d", InviterID: "missing-inviter", InviteeID: "missing-invitee", PaymentOrderID: "missing-order", Status: model.ReferralRewardPending, CreatedAt: stamp},
	}
	if err := db.Create(&rewards).Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	rows, total, err := repo.AdminReferralRewards("all", 20, 0)
	if err != nil || total != 4 || len(rows) != 4 {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, err)
	}
	first := rows[0]
	if first.ID != "a" || first.PaymentOrderID != "original" || first.OrderID == nil || *first.OrderID != "original" || first.OrderMerchantOrderNo != "MERCHANT-ONE" || first.OrderUserID != "invitee" || first.OrderAmountFen != 1234 || first.OrderCurrency != "CNY" || first.OrderStatus != model.PaymentOrderCredited || first.OrderCreditedAt == nil || !first.OrderCreditedAt.Equal(stamp) {
		t.Fatalf("wrong payment join: %+v", first)
	}
	if first.InviterUserID == nil || *first.InviterUserID != "inviter" || first.InviterUsername != "alice" || first.InviterEmail != "alice@example.com" || first.InviteeUserID == nil || *first.InviteeUserID != "invitee" || first.InviteeUsername != "bob" || first.InviteeEmail != "" {
		t.Fatalf("wrong identities: %+v", first)
	}
	if rows[1].OrderID == nil || *rows[1].OrderID != "newer" || rows[1].OrderCreditedAt != nil || rows[1].OrderStatus != model.PaymentOrderPending {
		t.Fatalf("wrong second payment: %+v", rows[1])
	}
	if rows[2].OrderID != nil || rows[3].OrderID != nil || rows[3].InviterUserID != nil || rows[3].InviteeUserID != nil {
		t.Fatalf("missing or foreign relations leaked: %+v", rows[2:])
	}
	for _, tc := range []struct {
		status string
		total  int64
		id     string
	}{{"pending", 2, "a"}, {"approved", 1, "b"}, {"rejected", 1, "c"}} {
		rows, total, err := repo.AdminReferralRewards(tc.status, 1, 0)
		if err != nil || total != tc.total || len(rows) != 1 || rows[0].ID != tc.id {
			t.Fatalf("filter %s: %+v %d %v", tc.status, rows, total, err)
		}
	}
	rows, total, err = repo.AdminReferralRewards("pending", 1, 1)
	if err != nil || total != 2 || len(rows) != 1 || rows[0].ID != "d" {
		t.Fatalf("second page: %+v %d %v", rows, total, err)
	}
	rows, total, err = repo.AdminReferralRewards("all", 20, 20)
	if err != nil || total != 4 || len(rows) != 0 {
		t.Fatalf("empty page: %+v %d %v", rows, total, err)
	}
	if err := db.Migrator().DropTable(&model.PaymentOrder{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.AdminReferralRewards("all", 20, 0); err == nil {
		t.Fatal("join failure was swallowed")
	}
}
