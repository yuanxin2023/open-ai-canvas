package repository

import (
	"errors"
	"math"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestMeasuredMediaSettlementRespectsQuoteAndExplicitAuthorization(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		capability string
		duration   int64
		quote      int64
		limit      int64
		limitSet   bool
		blocked    bool
	}{
		{name: "audio default quote exceeded", capability: "audio", duration: 6_000, quote: 50_000, blocked: true},
		{name: "video processing default quote exceeded", capability: "video", duration: 6_000, quote: 50_000, blocked: true},
		{name: "audio exactly quoted", capability: "audio", duration: 5_000, quote: 50_000},
		{name: "video processing shorter", capability: "video", duration: 2_100, quote: 50_000},
		{name: "explicit larger authorization", capability: "audio", duration: 6_000, quote: 50_000, limit: 60_000, limitSet: true},
		{name: "explicit authorization exceeded", capability: "audio", duration: 6_001, quote: 50_000, limit: 60_000, limitSet: true, blocked: true},
		{name: "explicit zero authorization", capability: "audio", duration: 1, limitSet: true, blocked: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			repo, db := newRefundedBillingRecoveryRepository(t)
			order := model.BillingOrder{ID: "media-order", UserID: "media-user", IdempotencyKey: "media-task", Capability: scenario.capability, BillingMode: "per_second", UnitPriceMicrocredits: 10_000, MultiplierBasisPoints: 10_000, Quantity: 5, AmountMicrocredits: scenario.quote, ReservedAmountMicrocredits: scenario.quote, ChargeLimitSet: scenario.limitSet, ChargeLimitMicrocredits: scenario.limit, Status: model.BillingStatusRunning}
			account := model.CreditAccount{UserID: order.UserID, AvailableMicrocredits: 100_000, ReservedMicrocredits: scenario.quote}
			if err := db.Create(&account).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&order).Error; err != nil {
				t.Fatal(err)
			}
			err := repo.SettleBillingOrderWithAudioDuration(order.ID, "provider", scenario.duration)
			if scenario.blocked {
				if !errors.Is(err, ErrBillingChargeLimit) {
					t.Fatalf("authorization error: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			stored, err := repo.BillingOrder(order.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.First(&account, "user_id = ?", order.UserID).Error; err != nil {
				t.Fatal(err)
			}
			var ledgerCount int64
			if err := db.Model(&model.CreditLedgerEntry{}).Where("billing_order_id = ?", order.ID).Count(&ledgerCount).Error; err != nil {
				t.Fatal(err)
			}
			seconds := scenario.duration / 1_000
			if scenario.duration%1_000 != 0 {
				seconds++
			}
			actual := seconds * order.UnitPriceMicrocredits
			if stored.AmountMicrocredits != order.AmountMicrocredits {
				t.Fatal("original quote was overwritten")
			}
			if scenario.blocked {
				if stored.Status != model.BillingStatusRunning || stored.ActualAmountMicrocredits != actual || account.AvailableMicrocredits != 100_000 || account.ReservedMicrocredits != scenario.quote || ledgerCount != 0 {
					t.Fatalf("blocked settlement changed funds or lost evidence: order=%#v account=%#v ledger=%d", stored, account, ledgerCount)
				}
			} else {
				if stored.Status != model.BillingStatusSettled || stored.ActualAmountMicrocredits != actual || stored.Quantity != seconds || account.AvailableMicrocredits != 100_000+scenario.quote-actual || account.ReservedMicrocredits != 0 {
					t.Fatalf("wrong measured settlement: order=%#v account=%#v", stored, account)
				}
				if err := repo.SettleBillingOrderWithAudioDuration(order.ID, "retry", 99_000); err != nil {
					t.Fatal(err)
				}
				var after int64
				if err := db.Model(&model.CreditLedgerEntry{}).Where("billing_order_id = ?", order.ID).Count(&after).Error; err != nil || after != ledgerCount {
					t.Fatalf("retry created another ledger entry: before=%d after=%d err=%v", ledgerCount, after, err)
				}
				var retriedAccount model.CreditAccount
				if err := db.First(&retriedAccount, "user_id = ?", order.UserID).Error; err != nil {
					t.Fatal(err)
				}
				if retriedAccount.AvailableMicrocredits != account.AvailableMicrocredits || retriedAccount.ReservedMicrocredits != account.ReservedMicrocredits {
					t.Fatal("settled retry changed funds")
				}
			}
		})
	}
}

func TestMeasuredMediaDurationBoundaries(t *testing.T) {
	for _, scenario := range []struct {
		name         string
		duration     int64
		price        int64
		quote        int64
		wantQuantity int64
		wantAmount   int64
		invalid      bool
	}{
		{name: "missing duration", duration: 0, price: 10_000, quote: 50_000, invalid: true},
		{name: "negative duration", duration: -1, price: 10_000, quote: 50_000, invalid: true},
		{name: "fractional second", duration: 999, price: 10_000, quote: 50_000, wantQuantity: 1, wantAmount: 10_000},
		{name: "exact second", duration: 1_000, price: 10_000, quote: 50_000, wantQuantity: 1, wantAmount: 10_000},
		{name: "above second", duration: 1_001, price: 10_000, quote: 50_000, wantQuantity: 2, wantAmount: 20_000},
		{name: "small price uses local quantum", duration: 1_000, price: 1, quote: 10_000, wantQuantity: 1, wantAmount: 10_000},
		{name: "free duration cannot overflow seconds", duration: math.MaxInt64, wantQuantity: math.MaxInt64/1_000 + 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			repo, db := newRefundedBillingRecoveryRepository(t)
			order := model.BillingOrder{ID: "duration", UserID: "user", IdempotencyKey: "duration-task", Capability: "audio", BillingMode: "per_second", UnitPriceMicrocredits: scenario.price, MultiplierBasisPoints: 10_000, AmountMicrocredits: scenario.quote, ReservedAmountMicrocredits: scenario.quote, Status: model.BillingStatusRunning}
			if err := db.Create(&model.CreditAccount{UserID: order.UserID, ReservedMicrocredits: scenario.quote}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&order).Error; err != nil {
				t.Fatal(err)
			}
			err := repo.SettleBillingOrderWithAudioDuration(order.ID, "", scenario.duration)
			if (err != nil) != scenario.invalid {
				t.Fatalf("duration result: %v", err)
			}
			stored, err := repo.BillingOrder(order.ID)
			if err != nil {
				t.Fatal(err)
			}
			if scenario.invalid {
				var account model.CreditAccount
				if err := db.First(&account, "user_id = ?", order.UserID).Error; err != nil {
					t.Fatal(err)
				}
				if stored.Status != model.BillingStatusRunning || account.ReservedMicrocredits != scenario.quote || account.AvailableMicrocredits != 0 {
					t.Fatalf("missing duration changed billing: %#v %#v", stored, account)
				}
			} else if stored.Quantity != scenario.wantQuantity || stored.ActualAmountMicrocredits != scenario.wantAmount {
				t.Fatalf("duration=%d quantity=%d amount=%d", scenario.duration, stored.Quantity, stored.ActualAmountMicrocredits)
			}
		})
	}
}

func TestOrdinaryVideoAndFixedAudioKeepOriginalQuote(t *testing.T) {
	for _, scenario := range []struct {
		capability string
		mode       string
	}{{"video", "per_second"}, {"audio", "fixed_request"}} {
		t.Run(scenario.capability, func(t *testing.T) {
			repo, db := newRefundedBillingRecoveryRepository(t)
			order := model.BillingOrder{ID: "ordinary", UserID: "user", IdempotencyKey: "ordinary-task", Capability: scenario.capability, BillingMode: scenario.mode, UnitPriceMicrocredits: 10_000, MultiplierBasisPoints: 10_000, Quantity: 5, AmountMicrocredits: 50_000, ReservedAmountMicrocredits: 50_000, Status: model.BillingStatusRunning}
			if err := db.Create(&model.CreditAccount{UserID: order.UserID, ReservedMicrocredits: 50_000}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&order).Error; err != nil {
				t.Fatal(err)
			}
			if err := repo.SettleBillingOrder(order.ID, ""); err != nil {
				t.Fatal(err)
			}
			stored, err := repo.BillingOrder(order.ID)
			if err != nil || stored.ActualAmountMicrocredits != 50_000 || stored.Quantity != 5 {
				t.Fatalf("ordinary quote changed: %#v %v", stored, err)
			}
		})
	}
}

func TestMeasuredRouteFallbackCannotIncreaseOriginalAuthorization(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		explicit bool
	}{{"existing explicit cap", true}, {"older audio quote", false}} {
		t.Run(scenario.name, func(t *testing.T) {
			repo, db := newRefundedBillingRecoveryRepository(t)
			if err := db.AutoMigrate(&model.Task{}); err != nil {
				t.Fatal(err)
			}
			task := model.Task{ID: "task", UserID: "user", Status: model.TaskStatusRunning, RouteID: "original", BillingOrderID: "order"}
			order := model.BillingOrder{ID: "order", UserID: task.UserID, TaskID: task.ID, IdempotencyKey: "fallback", Capability: "audio", BillingMode: "per_second", UnitPriceMicrocredits: 10_000, Quantity: 5, MultiplierBasisPoints: 10_000, AmountMicrocredits: 50_000, ReservedAmountMicrocredits: 50_000, ChargeLimitSet: scenario.explicit, ChargeLimitMicrocredits: 50_000, Status: model.BillingStatusRunning}
			if !scenario.explicit {
				order.ChargeLimitMicrocredits = 0
			}
			for _, value := range []any{&task, &order, &model.CreditAccount{UserID: task.UserID, AvailableMicrocredits: 100_000, ReservedMicrocredits: 50_000}} {
				if err := db.Create(value).Error; err != nil {
					t.Fatal(err)
				}
			}
			replacement := order
			replacement.AmountMicrocredits = 60_000
			if err := repo.SwitchTaskLogicalRoute(task.ID, "original", "expensive", "{}", order.ID, "channel", "model", &replacement, model.BillingCostSnapshot{}); !errors.Is(err, ErrBillingChargeLimit) {
				t.Fatalf("expanded authorization: %v", err)
			}
			replacement.AmountMicrocredits = 40_000
			replacement.UnitPriceMicrocredits = 8_000
			if err := repo.SwitchTaskLogicalRoute(task.ID, "original", "cheaper", "{}", order.ID, "channel", "model", &replacement, model.BillingCostSnapshot{}); err != nil {
				t.Fatal(err)
			}
			if err := repo.SettleBillingOrderWithAudioDuration(order.ID, "provider", 6_000); err != nil {
				t.Fatal(err)
			}
			stored, err := repo.BillingOrder(order.ID)
			if err != nil || stored.ChargeLimitMicrocredits != 50_000 || !stored.ChargeLimitSet || stored.AmountMicrocredits != 40_000 || stored.ActualAmountMicrocredits != 50_000 {
				t.Fatalf("original authorization lost: %#v %v", stored, err)
			}
			var account model.CreditAccount
			if err := db.First(&account, "user_id = ?", task.UserID).Error; err != nil {
				t.Fatal(err)
			}
			if account.AvailableMicrocredits != 100_000 || account.ReservedMicrocredits != 0 {
				t.Fatalf("fallback settlement funds: %#v", account)
			}
		})
	}
}
