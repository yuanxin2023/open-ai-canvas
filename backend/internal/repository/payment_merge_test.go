package repository

import (
	"errors"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func stockProduct(id string, total int64) model.TopupProduct {
	return model.TopupProduct{ID: id, Name: "Stock product", Enabled: true, SaleStrategy: model.TopupSaleStrategyInventory, StockTotal: total, StockRemaining: total, AmountFen: 100, CreditsMicrocredits: 100_000_000}
}

func TestLocalClosureRetainsStockUntilProviderConfirmation(t *testing.T) {
	db := openPaymentTestDB(t)
	r := New(db)
	p := stockProduct("stock", 1)
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	order := paymentLimitTestOrder("first", p.ID)
	if _, _, err := r.CreatePaymentOrderWithProductReservation(order); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkPaymentOrderClosed(order.ID, "LOCAL", false); err != nil {
		t.Fatal(err)
	}
	stored, err := r.PaymentOrder(order.ID)
	if err != nil || !stored.StockReserved || stored.ProviderClosedAt != nil {
		t.Fatalf("unconfirmed closure released reservation: %#v %v", stored, err)
	}
	if _, _, err := r.CreatePaymentOrderWithProductReservation(paymentLimitTestOrder("second", p.ID)); !errors.Is(err, ErrTopupUnavailable) {
		t.Fatalf("stock reused before confirmation: %v", err)
	}
	if err := r.MarkPaymentOrderClosed(order.ID, "CLOSED", true); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkPaymentOrderClosed(order.ID, "CLOSED", true); err != nil {
		t.Fatal(err)
	}
	persisted, err := r.TopupProduct(p.ID)
	if err != nil || persisted.StockRemaining != 1 {
		t.Fatalf("confirmation did not release exactly once: %#v %v", persisted, err)
	}
}

func TestLatePaidOrderCreditsOnceAndRecordsOversoldStock(t *testing.T) {
	db := openPaymentTestDB(t)
	r := New(db)
	p := stockProduct("limited", 1)
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	first := paymentLimitTestOrder("first", p.ID)
	if _, _, err := r.CreatePaymentOrderWithProductReservation(first); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkPaymentOrderClosed(first.ID, "CLOSED", true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.CreatePaymentOrderWithProductReservation(paymentLimitTestOrder("second", p.ID)); err != nil {
		t.Fatal(err)
	}
	evidence := PaymentEvidence{ProviderTradeNo: "late-trade", ProviderStatus: "SUCCESS", AmountFen: first.AmountFen, Currency: first.Currency, PaidAt: time.Now()}
	for index := 0; index < 2; index++ {
		order, granted, err := r.CompletePaymentOrder(first.ProviderID, first.MerchantOrderNo, evidence)
		if err != nil || granted != (index == 0) || !order.InventoryOversold {
			t.Fatalf("late completion: %#v granted=%t err=%v", order, granted, err)
		}
	}
	product, err := r.TopupProduct(p.ID)
	if err != nil || product.StockRemaining != -1 || product.Enabled {
		t.Fatalf("oversold product still available: %#v %v", product, err)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", first.UserID).Error; err != nil || account.AvailableMicrocredits != first.CreditsMicrocredits {
		t.Fatalf("wrong credit grant: %#v %v", account, err)
	}
	var audits int64
	if err := db.Model(&model.AdminAuditEvent{}).Where("action = ? AND target_id = ?", "payment.inventory_oversold", first.ID).Count(&audits).Error; err != nil || audits != 1 {
		t.Fatalf("oversale audit count=%d err=%v", audits, err)
	}
}

func TestProductStrategyCannotChangeWhileStockIsUnresolved(t *testing.T) {
	db := openPaymentTestDB(t)
	r := New(db)
	p := stockProduct("stock", 2)
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	order := paymentLimitTestOrder("first", p.ID)
	if _, _, err := r.CreatePaymentOrderWithProductReservation(order); err != nil {
		t.Fatal(err)
	}
	change := func(product *model.TopupProduct) (*model.TopupProduct, error) {
		product.SaleStrategy = model.TopupSaleStrategyUnlimited
		return product, nil
	}
	if err := r.UpdateTopupProduct(p.ID, change); !errors.Is(err, ErrTopupReservationsPending) {
		t.Fatalf("pending strategy change allowed: %v", err)
	}
	if err := r.MarkPaymentOrderClosed(order.ID, "LOCAL", false); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateTopupProduct(p.ID, change); !errors.Is(err, ErrTopupReservationsPending) {
		t.Fatalf("unconfirmed strategy change allowed: %v", err)
	}
	if err := r.MarkPaymentOrderClosed(order.ID, "CLOSED", true); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateTopupProduct(p.ID, change); err != nil {
		t.Fatal(err)
	}
}

func TestProductBuilderCannotChangeLockedIdentity(t *testing.T) {
	db := openPaymentTestDB(t)
	r := New(db)
	product := stockProduct("locked-product", 2)
	if err := db.Create(&product).Error; err != nil {
		t.Fatal(err)
	}
	err := r.UpdateTopupProduct(product.ID, func(draft *model.TopupProduct) (*model.TopupProduct, error) {
		draft.ID = "other-product"
		draft.Name = "must not save"
		return draft, nil
	})
	if !errors.Is(err, ErrPaymentOrderStateConflict) {
		t.Fatalf("identity change accepted: %v", err)
	}
	stored, err := r.TopupProduct(product.ID)
	if err != nil || stored.Name != product.Name || stored.StockRemaining != product.StockRemaining {
		t.Fatalf("locked product changed: %#v %v", stored, err)
	}
}

func TestOrderUsesLockedProductSnapshotAndKeepsLocalCardFields(t *testing.T) {
	db := openPaymentTestDB(t)
	r := New(db)
	p := model.TopupProduct{ID: "product", Name: "Before", Enabled: true, AmountFen: 100, CreditsMicrocredits: 100_000_000, Benefits: "Local benefits", RibbonText: "Local ribbon", CompareAmountFen: 400, AccentColor: "#123456"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	stale := paymentLimitTestOrder("snapshot", p.ID)
	if err := r.UpdateTopupProduct(p.ID, func(current *model.TopupProduct) (*model.TopupProduct, error) {
		current.Name = "After"
		current.AmountFen = 200
		current.CreditsMicrocredits = 300_000_000
		return current, nil
	}); err != nil {
		t.Fatal(err)
	}
	order, _, err := r.CreatePaymentOrderWithProductReservation(stale, func(current model.TopupProduct, _ time.Time) int64 { return current.AmountFen })
	if err != nil || order.AmountFen != 200 || order.CreditsMicrocredits != 300_000_000 || order.ProductName != "After" {
		t.Fatalf("stale product snapshot used: %#v %v", order, err)
	}
	updated, err := r.TopupProduct(p.ID)
	if err != nil || updated.Benefits != p.Benefits || updated.RibbonText != p.RibbonText || updated.CompareAmountFen != p.CompareAmountFen || updated.AccentColor != p.AccentColor {
		t.Fatalf("card fields lost: %#v %v", updated, err)
	}
	if err := db.Model(updated).Updates(map[string]any{"amount_fen": 900, "credits_microcredits": 900_000_000}).Error; err != nil {
		t.Fatal(err)
	}
	credited, _, err := r.CompletePaymentOrder(order.ProviderID, order.MerchantOrderNo, PaymentEvidence{ProviderTradeNo: "snapshot-trade", AmountFen: 200, Currency: "CNY", PaidAt: time.Now()})
	if err != nil || credited.CreditsMicrocredits != 300_000_000 {
		t.Fatalf("settlement repriced order: %#v %v", credited, err)
	}
}

func TestIdempotencyMismatchDoesNotReserveAnotherProduct(t *testing.T) {
	db := openPaymentTestDB(t)
	r := New(db)
	products := []model.TopupProduct{stockProduct("first", 2), stockProduct("second", 2)}
	if err := db.Create(&products).Error; err != nil {
		t.Fatal(err)
	}
	first := paymentLimitTestOrder("first", "first")
	if _, _, err := r.CreatePaymentOrderWithProductReservation(first); err != nil {
		t.Fatal(err)
	}
	second := paymentLimitTestOrder("second", "second")
	second.IdempotencyKey = first.IdempotencyKey
	if _, _, err := r.CreatePaymentOrderWithProductReservation(second); !errors.Is(err, ErrPaymentOrderStateConflict) {
		t.Fatalf("idempotency crossed products: %v", err)
	}
	stored, err := r.TopupProduct("second")
	if err != nil || stored.StockRemaining != 2 {
		t.Fatalf("failed idempotency leaked stock: %#v %v", stored, err)
	}
}

func TestUnconfirmedClosureRecheckIsClaimedOnce(t *testing.T) {
	db := openPaymentTestDB(t)
	r := New(db)
	p := stockProduct("stock", 1)
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	order := paymentLimitTestOrder("first", p.ID)
	if _, _, err := r.CreatePaymentOrderWithProductReservation(order); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkPaymentOrderClosed(order.ID, "LOCAL", false); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().Add(-time.Minute)
	orders, err := r.UnconfirmedClosedPaymentOrdersNeedingQuery(cutoff, 20)
	if err != nil || len(orders) != 1 {
		t.Fatalf("held stock not scheduled: %#v %v", orders, err)
	}
	for index := 0; index < 2; index++ {
		claimed, err := r.ClaimUnconfirmedPaymentOrderQuery(order.ID, cutoff)
		if err != nil || claimed != (index == 0) {
			t.Fatalf("claim %d: %t %v", index, claimed, err)
		}
	}
	if err := r.MarkPaymentOrderClosed(order.ID, "CLOSED", true); err != nil {
		t.Fatal(err)
	}
	orders, err = r.UnconfirmedClosedPaymentOrdersNeedingQuery(time.Now().Add(time.Hour), 20)
	if err != nil || len(orders) != 0 {
		t.Fatalf("confirmed closure rescheduled: %#v %v", orders, err)
	}
}
