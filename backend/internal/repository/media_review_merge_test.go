package repository

import (
	"encoding/json"
	"errors"
	"testing"

	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
)

func mediaReviewFixture(t *testing.T, refunded bool, available int64) (*Repository, *gorm.DB, model.BillingOrder) {
	t.Helper()
	r, db := newRefundedBillingRecoveryRepository(t)
	order := model.BillingOrder{ID: "media-review", UserID: "user", TaskID: "task", IdempotencyKey: "media", Capability: "audio", BillingMode: "per_second", UnitPriceMicrocredits: 10_000, MultiplierBasisPoints: 10_000, Quantity: 5, AmountMicrocredits: 50_000, ReservedAmountMicrocredits: 50_000, ChargeLimitSet: true, ChargeLimitMicrocredits: 50_000, Status: model.BillingStatusUncertain, BillingCostSnapshot: model.BillingCostSnapshot{CostBillingMode: "per_second", CostQuantity: 5}}
	reserved := int64(62_000)
	if refunded {
		order.Status = model.BillingStatusRefunded
		order.RefundedAmountMicrocredits = 50_000
		reserved = 12_000
	}
	if err := db.Create(&model.CreditAccount{UserID: order.UserID, AvailableMicrocredits: available, ReservedMicrocredits: reserved}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	return r, db, order
}

func TestManualMediaReviewCapsChargeAndAuditsPlatformDifferenceOnce(t *testing.T) {
	r, db, order := mediaReviewFixture(t, false, 100_000)
	if err := r.SettleBillingOrderWithAudioDuration(order.ID, "provider", 7_000); !errors.Is(err, ErrBillingChargeLimit) {
		t.Fatalf("automatic overspend: %v", err)
	}
	review := BillingMediaReview{ActorUserID: "admin", Note: "已核实交付时长，按原授权结算"}
	for attempt := 0; attempt < 2; attempt++ {
		if err := r.SettleBillingOrderAfterMediaReview(order.ID, "provider", 7_000, review); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := r.BillingOrder(order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ActualAmountMicrocredits != 50_000 || stored.Quantity != 7 || stored.CostQuantity != 7 || stored.ResolvedBy != review.ActorUserID || stored.ResolutionNote != review.Note {
		t.Fatalf("review result: %#v", stored)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", order.UserID).Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits != 100_000 || account.ReservedMicrocredits != 12_000 {
		t.Fatalf("review funds: %#v", account)
	}
	var events []model.AdminAuditEvent
	if err := db.Where("action = ?", "billing.media_review.settle").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ActorUserID != review.ActorUserID {
		t.Fatalf("audit count=%d", len(events))
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(events[0].MetadataJSON), &metadata); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]float64{"durationMs": 7_000, "measuredMicrocredits": 70_000, "chargedMicrocredits": 50_000, "authorizationMicrocredits": 50_000, "platformCoveredMicrocredits": 20_000} {
		if metadata[key] != value {
			t.Fatalf("audit %s=%v", key, metadata[key])
		}
	}
	var consumes int64
	if err := db.Model(&model.CreditLedgerEntry{}).Where("billing_order_id = ? AND type = ?", order.ID, model.CreditLedgerConsume).Count(&consumes).Error; err != nil || consumes != 1 {
		t.Fatalf("consumes=%d err=%v", consumes, err)
	}
}

func TestMediaReviewRejectsMissingEvidenceAndRollsBackAuditFailure(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		review    BillingMediaReview
		duration  int64
		dropAudit bool
	}{
		{name: "missing actor", review: BillingMediaReview{Note: "evidence"}, duration: 7_000},
		{name: "missing note", review: BillingMediaReview{ActorUserID: "admin"}, duration: 7_000},
		{name: "missing duration", review: BillingMediaReview{ActorUserID: "admin", Note: "evidence"}},
		{name: "audit write failed", review: BillingMediaReview{ActorUserID: "admin", Note: "evidence"}, duration: 7_000, dropAudit: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			r, db, order := mediaReviewFixture(t, false, 100_000)
			if scenario.dropAudit {
				if err := db.Migrator().DropTable(&model.AdminAuditEvent{}); err != nil {
					t.Fatal(err)
				}
			}
			if err := r.SettleBillingOrderAfterMediaReview(order.ID, "provider", scenario.duration, scenario.review); err == nil {
				t.Fatal("invalid review succeeded")
			}
			stored, err := r.BillingOrder(order.ID)
			if err != nil {
				t.Fatal(err)
			}
			var account model.CreditAccount
			if err := db.First(&account, "user_id = ?", order.UserID).Error; err != nil {
				t.Fatal(err)
			}
			var entries int64
			if err := db.Model(&model.CreditLedgerEntry{}).Count(&entries).Error; err != nil {
				t.Fatal(err)
			}
			if stored.Status != model.BillingStatusUncertain || account.AvailableMicrocredits != 100_000 || account.ReservedMicrocredits != 62_000 || entries != 0 {
				t.Fatalf("failed review changed funds: %#v %#v ledger=%d", stored, account, entries)
			}
		})
	}
}

func TestRefundedMediaRecoveryPreservesReturnedFundsUntilValid(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		duration  int64
		balance   int64
		wantError error
	}{
		{name: "shorter output", duration: 2_500, balance: 100_000},
		{name: "over authorization", duration: 6_000, balance: 100_000, wantError: ErrBillingChargeLimit},
		{name: "insufficient available", duration: 3_000, balance: 20_000, wantError: ErrInsufficientCredits},
		{name: "unknown duration", balance: 100_000, wantError: ErrMeasuredDurationUnavailable},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			r, db, order := mediaReviewFixture(t, true, scenario.balance)
			err := r.RestoreRefundedBillingOrderWithAudioDuration(order.ID, "provider", scenario.duration)
			if !errors.Is(err, scenario.wantError) {
				t.Fatalf("recovery error=%v expected=%v", err, scenario.wantError)
			}
			stored, err := r.BillingOrder(order.ID)
			if err != nil {
				t.Fatal(err)
			}
			var account model.CreditAccount
			if err := db.First(&account, "user_id = ?", order.UserID).Error; err != nil {
				t.Fatal(err)
			}
			if account.ReservedMicrocredits != 12_000 {
				t.Fatal("recovery consumed another task's reservation")
			}
			if scenario.wantError != nil {
				if stored.Status != model.BillingStatusUncertain || !stored.RefundRecoveryPending || account.AvailableMicrocredits != scenario.balance {
					t.Fatalf("failed recovery: %#v %#v", stored, account)
				}
				for repeat := 0; repeat < 2; repeat++ {
					if err := r.RefundBillingOrder(order.ID, "取消恢复扣费"); err != nil {
						t.Fatal(err)
					}
				}
				if err := db.First(&account, "user_id = ?", order.UserID).Error; err != nil {
					t.Fatal(err)
				}
				if account.AvailableMicrocredits != scenario.balance || account.ReservedMicrocredits != 12_000 {
					t.Fatal("cancelled recovery refunded a second time")
				}
			} else {
				if stored.Status != model.BillingStatusSettled || stored.RefundRecoveryPending || account.AvailableMicrocredits != 70_000 || stored.ActualAmountMicrocredits != 30_000 {
					t.Fatalf("valid recovery: %#v %#v", stored, account)
				}
				if err := r.RestoreRefundedBillingOrderWithAudioDuration(order.ID, "provider", 99_000); err != nil {
					t.Fatal(err)
				}
				if err := db.First(&account, "user_id = ?", order.UserID).Error; err != nil || account.AvailableMicrocredits != 70_000 {
					t.Fatal("duplicate recovery changed funds")
				}
			}
		})
	}
}

func TestManualReviewOfRefundRecoveryDebitsAvailableAndNeverRefundsAgain(t *testing.T) {
	r, db, order := mediaReviewFixture(t, true, 100_000)
	if err := r.RestoreRefundedBillingOrderWithAudioDuration(order.ID, "provider", 7_000); !errors.Is(err, ErrBillingChargeLimit) {
		t.Fatal(err)
	}
	review := BillingMediaReview{ActorUserID: "admin", Note: "核实实测与原授权差额"}
	if err := r.SettleBillingOrderAfterMediaReview(order.ID, "provider", 7_000, review); !errors.Is(err, ErrBillingStateConflict) {
		t.Fatal("returned funds were treated as a held reservation")
	}
	if err := r.RestoreRefundedBillingOrderAfterMediaReview(order.ID, "provider", 7_000, review); err != nil {
		t.Fatal(err)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", order.UserID).Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits != 50_000 || account.ReservedMicrocredits != 12_000 {
		t.Fatalf("manual recovery funds: %#v", account)
	}
	var refunds int64
	if err := db.Model(&model.CreditLedgerEntry{}).Where("type = ?", model.CreditLedgerRefund).Count(&refunds).Error; err != nil || refunds != 0 {
		t.Fatal("manual recovery created another refund")
	}
}

func TestFixedRefundRecoveryAlsoRequiresAvailableBalance(t *testing.T) {
	r, db, order := mediaReviewFixture(t, true, 20_000)
	if err := db.Model(&order).Update("billing_mode", "fixed_request").Error; err != nil {
		t.Fatal(err)
	}
	if err := r.RestoreRefundedBillingOrder(order.ID, "provider"); !errors.Is(err, ErrInsufficientCredits) {
		t.Fatalf("fixed recovery error=%v", err)
	}
	stored, err := r.BillingOrder(order.ID)
	if err != nil || !stored.RefundRecoveryPending || stored.Status != model.BillingStatusUncertain {
		t.Fatalf("fixed recovery review: %#v %v", stored, err)
	}
}
