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
