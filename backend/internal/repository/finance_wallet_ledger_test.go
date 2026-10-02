package repository

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWalletCreditLedgerCollapsesSettledRefundAndKeepsAuditRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:wallet-ledger-collapse?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.BillingOrder{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	order := model.BillingOrder{ID: "order-1", UserID: "user-1", IdempotencyKey: "task:1", BillingMode: "token", Status: model.BillingStatusSettled, ReservedAmountMicrocredits: 60_000, ActualAmountMicrocredits: 30_000, RefundedAmountMicrocredits: 30_000, InputTokens: 2_182, OutputTokens: 350, UsageAvailable: true}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	entries := []model.CreditLedgerEntry{{ID: "consume", UserID: "user-1", Type: model.CreditLedgerConsume, AmountMicrocredits: -30_000, BillingOrderID: order.ID, CreatedAt: now}, {ID: "refund", UserID: "user-1", Type: model.CreditLedgerRefund, AmountMicrocredits: 30_000, BillingOrderID: order.ID, CreatedAt: now}}
	if err := db.Create(&entries).Error; err != nil {
		t.Fatal(err)
	}
	repo := &Repository{db: db}
	walletEntries, walletTotal, err := repo.WalletCreditLedger("user-1", "all", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if walletTotal != 1 || len(walletEntries) != 1 || walletEntries[0].ID != "consume" {
		t.Fatalf("wallet ledger = %#v, total = %d", walletEntries, walletTotal)
	}
	item := walletEntries[0]
	if item.ReservedAmountMicrocredits != 60_000 || item.ActualAmountMicrocredits != 30_000 || item.RefundedAmountMicrocredits != 30_000 || !item.UsageAvailable || item.InputTokens != 2_182 || item.OutputTokens != 350 {
		t.Fatalf("wallet billing details = %#v", item)
	}
	auditEntries, auditTotal, err := repo.CreditLedger("user-1", "all", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if auditTotal != 2 || len(auditEntries) != 2 {
		t.Fatalf("audit ledger = %#v, total = %d", auditEntries, auditTotal)
	}
}

func TestWalletCreditLedgerKeepsWholeOrderRefund(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:wallet-ledger-whole-refund?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.BillingOrder{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	order := model.BillingOrder{ID: "order-1", UserID: "user-1", IdempotencyKey: "task:1", BillingMode: "token", Status: model.BillingStatusRefunded, ReservedAmountMicrocredits: 60_000, RefundedAmountMicrocredits: 60_000}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	entry := model.CreditLedgerEntry{ID: "refund", UserID: "user-1", Type: model.CreditLedgerRefund, AmountMicrocredits: 60_000, BillingOrderID: order.ID, CreatedAt: time.Now()}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatal(err)
	}
	repo := &Repository{db: db}
	items, total, err := repo.WalletCreditLedger("user-1", "refund", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != entry.ID {
		t.Fatalf("refund ledger = %#v, total = %d", items, total)
	}
}

func TestWalletCreditLedgerIncomeIncludesPaymentTopup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:wallet-ledger-income?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.BillingOrder{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	entries := []model.CreditLedgerEntry{
		{ID: "topup", UserID: "user-1", Type: model.CreditLedgerPaymentTopup, AmountMicrocredits: 10_000_000, CreatedAt: time.Now()},
		{ID: "consume", UserID: "user-1", Type: model.CreditLedgerConsume, AmountMicrocredits: -1_000_000, CreatedAt: time.Now()},
	}
	if err := db.Create(&entries).Error; err != nil {
		t.Fatal(err)
	}
	repo := &Repository{db: db}
	items, total, err := repo.WalletCreditLedger("user-1", "income", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != "topup" {
		t.Fatalf("income wallet ledger = %#v, total = %d", items, total)
	}
}

func TestCreditLedgerAdminFilterGroups(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-ledger-filter-groups?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	entries := []model.CreditLedgerEntry{
		{ID: "redeem", UserID: "user-1", Type: model.CreditLedgerRedeem, AmountMicrocredits: 1, CreatedAt: now.Add(-9 * time.Minute)},
		{ID: "payment", UserID: "user-1", Type: model.CreditLedgerPaymentTopup, AmountMicrocredits: 1, CreatedAt: now.Add(-8 * time.Minute)},
		{ID: "refund", UserID: "user-1", Type: model.CreditLedgerRefund, AmountMicrocredits: 1, CreatedAt: now.Add(-7 * time.Minute)},
		{ID: "signup", UserID: "user-1", Type: model.CreditLedgerSignupBonus, AmountMicrocredits: 1, CreatedAt: now.Add(-6 * time.Minute)},
		{ID: "checkin", UserID: "user-1", Type: model.CreditLedgerCheckinBonus, AmountMicrocredits: 1, CreatedAt: now.Add(-5 * time.Minute)},
		{ID: "consume", UserID: "user-1", Type: model.CreditLedgerConsume, AmountMicrocredits: -1, CreatedAt: now.Add(-4 * time.Minute)},
		{ID: "admin-grant", UserID: "user-1", Type: model.CreditLedgerAdminGrant, AmountMicrocredits: 1, CreatedAt: now.Add(-3 * time.Minute)},
		{ID: "admin-adjust", UserID: "user-1", Type: model.CreditLedgerAdminAdjust, AmountMicrocredits: -1, CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "reserve", UserID: "user-1", Type: model.CreditLedgerReserve, AmountMicrocredits: -1, CreatedAt: now.Add(-time.Minute)},
		{ID: "other-user", UserID: "user-2", Type: model.CreditLedgerPaymentTopup, AmountMicrocredits: 1, CreatedAt: now},
	}
	if err := db.Create(&entries).Error; err != nil {
		t.Fatal(err)
	}
	repo := &Repository{db: db}

	increasePage, increaseTotal, err := repo.CreditLedger("user-1", "increase", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if increaseTotal != 5 || len(increasePage) != 2 {
		t.Fatalf("increase page length = %d, total = %d, want 2/5", len(increasePage), increaseTotal)
	}
	increase, _, err := repo.CreditLedger("user-1", "increase", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertLedgerTypes(t, increase, map[model.CreditLedgerType]bool{
		model.CreditLedgerRedeem: true, model.CreditLedgerPaymentTopup: true, model.CreditLedgerRefund: true,
		model.CreditLedgerSignupBonus: true, model.CreditLedgerCheckinBonus: true,
	})

	consume, consumeTotal, err := repo.CreditLedger("user-1", "consume", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if consumeTotal != 1 {
		t.Fatalf("consume total = %d, want 1", consumeTotal)
	}
	assertLedgerTypes(t, consume, map[model.CreditLedgerType]bool{model.CreditLedgerConsume: true})

	admin, adminTotal, err := repo.CreditLedger("user-1", "admin", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if adminTotal != 2 {
		t.Fatalf("admin total = %d, want 2", adminTotal)
	}
	assertLedgerTypes(t, admin, map[model.CreditLedgerType]bool{model.CreditLedgerAdminGrant: true, model.CreditLedgerAdminAdjust: true})

	all, allTotal, err := repo.CreditLedger("user-1", "all", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if allTotal != 8 || len(all) != 8 {
		t.Fatalf("all ledger length = %d, total = %d, want 8/8", len(all), allTotal)
	}
}

func assertLedgerTypes(t *testing.T, entries []model.CreditLedgerEntry, want map[model.CreditLedgerType]bool) {
	t.Helper()
	if len(entries) != len(want) {
		t.Fatalf("ledger length = %d, want %d: %#v", len(entries), len(want), entries)
	}
	for _, entry := range entries {
		if !want[entry.Type] {
			t.Fatalf("unexpected ledger type %q in %#v", entry.Type, entries)
		}
	}
}
