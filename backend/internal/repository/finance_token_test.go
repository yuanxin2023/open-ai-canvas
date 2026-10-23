package repository

import (
	"errors"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTokenUsageAmountSettlesArkVideoCompletionTokens(t *testing.T) {
	amount, err := tokenUsageAmount(model.BillingOrder{
		Capability:                   "video",
		OutputTokenPriceMicrocredits: 16_000_000,
		MultiplierBasisPoints:        10_000,
	}, &BillingUsage{OutputTokens: 108900})
	if err != nil {
		t.Fatalf("tokenUsageAmount() error = %v", err)
	}
	if amount != 1_750_000 {
		t.Fatalf("tokenUsageAmount() = %d", amount)
	}
}

func TestTokenUsageAmountChargesMinimumCent(t *testing.T) {
	amount, err := tokenUsageAmount(model.BillingOrder{Capability: "video", OutputTokenPriceMicrocredits: 10_000, MultiplierBasisPoints: 10_000}, &BillingUsage{OutputTokens: 1})
	if err != nil {
		t.Fatalf("tokenUsageAmount() error = %v", err)
	}
	if amount != 10_000 {
		t.Fatalf("tokenUsageAmount() = %d, want 10000", amount)
	}
}

func TestBillingUsageReadsAsyncVideoPollUsage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:finance-token-poll?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ApiCallLog{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ApiCallLog{
		ID: "poll-log-1", BillingOrderID: "order-1", RequestKind: "poll", Billable: false,
		Status: model.ApiCallStatusSucceeded, UsageAvailable: true, OutputTokens: 108900,
	}).Error; err != nil {
		t.Fatal(err)
	}
	usage, err := billingUsage(db, "order-1")
	if err != nil {
		t.Fatalf("billingUsage() error = %v", err)
	}
	if usage.OutputTokens != 108900 {
		t.Fatalf("billingUsage() = %#v", usage)
	}
}

func TestTokenUsageAmountRejectsVideoWithoutOutputUsage(t *testing.T) {
	_, err := tokenUsageAmount(model.BillingOrder{Capability: "video", OutputTokenPriceMicrocredits: 16_000_000, MultiplierBasisPoints: 10_000}, &BillingUsage{})
	if !errors.Is(err, ErrBillingUsageUnavailable) {
		t.Fatalf("tokenUsageAmount() error = %v", err)
	}
}

func TestSettleArkVideoTokenOrderFromPollUsage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:finance-token-settle?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CreditAccount{}, &model.BillingOrder{}, &model.ApiCallLog{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	const reserved = int64(1_920_000)
	if err := db.Create(&model.CreditAccount{UserID: "user-1", ReservedMicrocredits: reserved}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.BillingOrder{
		ID: "order-1", UserID: "user-1", IdempotencyKey: "task:task-1", Capability: "video", BillingMode: "token",
		AmountMicrocredits: reserved, ReservedAmountMicrocredits: reserved, OutputTokenPriceMicrocredits: 16_000_000,
		MultiplierBasisPoints: 10_000, Status: model.BillingStatusRunning,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ApiCallLog{
		ID: "poll-log-1", BillingOrderID: "order-1", RequestKind: "poll", Billable: false,
		Status: model.ApiCallStatusSucceeded, UsageAvailable: true, OutputTokens: 108900,
	}).Error; err != nil {
		t.Fatal(err)
	}

	repo := &Repository{db: db}
	if err := repo.SettleBillingOrder("order-1", "ark-task-1"); err != nil {
		t.Fatalf("SettleBillingOrder() error = %v", err)
	}
	var order model.BillingOrder
	if err := db.First(&order, "id = ?", "order-1").Error; err != nil {
		t.Fatal(err)
	}
	if order.Status != model.BillingStatusSettled || order.ActualAmountMicrocredits != 1_750_000 || order.RefundedAmountMicrocredits != 170_000 {
		t.Fatalf("settled order = %#v", order)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", "user-1").Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits != 170_000 || account.ReservedMicrocredits != 0 {
		t.Fatalf("settled account = %#v", account)
	}
}

func TestSettleArkVideoTokenOrderSupplementsUnderreservation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:finance-token-supplement?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CreditAccount{}, &model.BillingOrder{}, &model.ApiCallLog{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	const (
		reserved   = int64(3_050_000)
		actual     = int64(3_120_000)
		supplement = actual - reserved
	)
	if err := db.Create(&model.CreditAccount{UserID: "user-1", AvailableMicrocredits: 1_000_000, ReservedMicrocredits: reserved}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.BillingOrder{
		ID: "order-1", UserID: "user-1", IdempotencyKey: "task:task-1", Capability: "video", BillingMode: "token",
		AmountMicrocredits: reserved, ReservedAmountMicrocredits: reserved, OutputTokenPriceMicrocredits: 18_200_000,
		MultiplierBasisPoints: 10_000, Status: model.BillingStatusRunning,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ApiCallLog{
		ID: "poll-log-1", BillingOrderID: "order-1", RequestKind: "poll", Billable: false,
		Status: model.ApiCallStatusSucceeded, UsageAvailable: true, OutputTokens: 171_166,
	}).Error; err != nil {
		t.Fatal(err)
	}

	repo := &Repository{db: db}
	if err := repo.SettleBillingOrder("order-1", "ark-task-1"); err != nil {
		t.Fatalf("SettleBillingOrder() error = %v", err)
	}
	var order model.BillingOrder
	if err := db.First(&order, "id = ?", "order-1").Error; err != nil {
		t.Fatal(err)
	}
	if order.Status != model.BillingStatusSettled || order.ActualAmountMicrocredits != actual || order.OutputTokens != 171_166 || !order.UsageAvailable {
		t.Fatalf("settled order = %#v", order)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", "user-1").Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits != 1_000_000-supplement || account.ReservedMicrocredits != 0 {
		t.Fatalf("settled account = %#v", account)
	}
	var entry model.CreditLedgerEntry
	if err := db.First(&entry, "billing_order_id = ? AND type = ?", "order-1", model.CreditLedgerConsume).Error; err != nil {
		t.Fatal(err)
	}
	if entry.AmountMicrocredits != -actual || entry.AvailableDeltaMicrocredits != -supplement || entry.ReservedDeltaMicrocredits != -reserved {
		t.Fatalf("consume entry = %#v", entry)
	}
}

func TestSettleAgentTokenOrderCapsChargeAtQuotedLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:finance-token-agent-cap?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CreditAccount{}, &model.BillingOrder{}, &model.ApiCallLog{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: "user-1", AvailableMicrocredits: 500, ReservedMicrocredits: 100}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.BillingOrder{
		ID: "agent-order", UserID: "user-1", IdempotencyKey: "task:agent-task", Capability: "text", BillingMode: "token",
		AmountMicrocredits: 100, ReservedAmountMicrocredits: 100, ChargeLimitMicrocredits: 100,
		OutputTokenPriceMicrocredits: 1_000_000, MultiplierBasisPoints: 10_000, Status: model.BillingStatusRunning,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ApiCallLog{
		ID: "agent-log", BillingOrderID: "agent-order", RequestKind: "generate", Billable: true,
		Status: model.ApiCallStatusSucceeded, UsageAvailable: true, OutputTokens: 200,
	}).Error; err != nil {
		t.Fatal(err)
	}

	repo := &Repository{db: db}
	if err := repo.SettleBillingOrder("agent-order", "provider-request"); err != nil {
		t.Fatalf("SettleBillingOrder() error = %v", err)
	}
	var order model.BillingOrder
	if err := db.First(&order, "id = ?", "agent-order").Error; err != nil {
		t.Fatal(err)
	}
	if order.ActualAmountMicrocredits != 100 || order.OutputTokens != 200 || !order.UsageAvailable {
		t.Fatalf("capped Agent order = %#v", order)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", "user-1").Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits != 500 || account.ReservedMicrocredits != 0 {
		t.Fatalf("capped Agent account = %#v", account)
	}
	var entry model.CreditLedgerEntry
	if err := db.First(&entry, "billing_order_id = ? AND type = ?", "agent-order", model.CreditLedgerConsume).Error; err != nil {
		t.Fatal(err)
	}
	if entry.AmountMicrocredits != -100 || !strings.Contains(entry.Note, "硬上限") {
		t.Fatalf("capped Agent ledger entry = %#v", entry)
	}
}

func TestSettleArkVideoTokenOrderAllowsNegativeBalance(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:finance-token-negative-balance?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CreditAccount{}, &model.BillingOrder{}, &model.ApiCallLog{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	const reserved = int64(3_050_000)
	if err := db.Create(&model.CreditAccount{UserID: "user-1", AvailableMicrocredits: 10_000, ReservedMicrocredits: reserved}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.BillingOrder{
		ID: "order-1", UserID: "user-1", IdempotencyKey: "task:task-1", Capability: "video", BillingMode: "token",
		AmountMicrocredits: reserved, ReservedAmountMicrocredits: reserved, OutputTokenPriceMicrocredits: 18_200_000,
		MultiplierBasisPoints: 10_000, Status: model.BillingStatusRunning,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ApiCallLog{
		ID: "poll-log-1", BillingOrderID: "order-1", RequestKind: "poll", Billable: false,
		Status: model.ApiCallStatusSucceeded, UsageAvailable: true, OutputTokens: 171_166,
	}).Error; err != nil {
		t.Fatal(err)
	}

	repo := &Repository{db: db}
	err = repo.SettleBillingOrder("order-1", "ark-task-1")
	if err != nil {
		t.Fatalf("SettleBillingOrder() error = %v", err)
	}
	var order model.BillingOrder
	if err := db.First(&order, "id = ?", "order-1").Error; err != nil {
		t.Fatal(err)
	}
	if order.Status != model.BillingStatusSettled || order.ActualAmountMicrocredits != 3_120_000 || order.OutputTokens != 171_166 || !order.UsageAvailable {
		t.Fatalf("settled order = %#v", order)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", "user-1").Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits != -60_000 || account.ReservedMicrocredits != 0 {
		t.Fatalf("settled account = %#v", account)
	}
	var entry model.CreditLedgerEntry
	if err := db.First(&entry, "billing_order_id = ? AND type = ?", "order-1", model.CreditLedgerConsume).Error; err != nil {
		t.Fatal(err)
	}
	if entry.AvailableAfterMicrocredits != -60_000 || entry.AvailableDeltaMicrocredits != -70_000 {
		t.Fatalf("consume entry = %#v", entry)
	}
}

func TestNegativeBalanceBlocksNewReservationsAndAcceptsRepayment(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:finance-negative-balance-repayment?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CreditAccount{}, &model.BillingOrder{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: "user-1", AvailableMicrocredits: -1_000_000}).Error; err != nil {
		t.Fatal(err)
	}

	repo := &Repository{db: db}
	order := model.BillingOrder{
		ID: "order-1", UserID: "user-1", IdempotencyKey: "task:task-1",
		AmountMicrocredits: 1, ReservedAmountMicrocredits: 1, Status: model.BillingStatusReserved,
	}
	if err := repo.ReserveBillingOrder(&order); !errors.Is(err, ErrInsufficientCredits) {
		t.Fatalf("ReserveBillingOrder() error = %v", err)
	}

	account, err := repo.AdjustCredits("user-1", "admin-1", 100_000_000, "充值入账")
	if err != nil {
		t.Fatalf("AdjustCredits() error = %v", err)
	}
	if account.AvailableMicrocredits != 99_000_000 {
		t.Fatalf("available balance = %d", account.AvailableMicrocredits)
	}
}

func TestInsufficientCreditsDoesNotCreateTaskOrReservation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:task-insufficient-credits?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CreditAccount{}, &model.BillingOrder{}, &model.CreditLedgerEntry{}, &model.Task{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: "user-1", AvailableMicrocredits: 0}).Error; err != nil {
		t.Fatal(err)
	}
	repo := &Repository{db: db}
	task := &model.Task{ID: "task-1", UserID: "user-1", Type: "canvas_image", Status: model.TaskStatusQueued}
	order := &model.BillingOrder{ID: "order-1", UserID: "user-1", IdempotencyKey: "task:task-1", AmountMicrocredits: 10_000, ReservedAmountMicrocredits: 10_000, Status: model.BillingStatusReserved}
	if err := repo.CreateTaskWithCreditReservation(task, order, 10); !errors.Is(err, ErrInsufficientCredits) {
		t.Fatalf("CreateTaskWithCreditReservation() error = %v", err)
	}
	for _, value := range []any{&model.Task{}, &model.BillingOrder{}, &model.CreditLedgerEntry{}} {
		var count int64
		if err := db.Model(value).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("failed reservation persisted %T: count=%d error=%v", value, count, err)
		}
	}
	freeTask := &model.Task{ID: "free-task", UserID: "user-1", Type: "canvas_image", Status: model.TaskStatusQueued}
	if err := repo.CreateTaskWithActiveLimit(freeTask, 10); err != nil {
		t.Fatalf("free task was blocked by zero balance: %v", err)
	}
	var count int64
	if err := db.Model(&model.Task{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("free task count=%d error=%v", count, err)
	}
}
