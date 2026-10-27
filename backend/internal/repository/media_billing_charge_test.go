package repository

import (
	"errors"
	"math"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestMeasuredMediaChargeUsesOriginalAuthorizationAndReviewDecision(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		duration  int64
		price     int64
		quote     int64
		limit     int64
		limitSet  bool
		reviewed  bool
		quantity  int64
		measured  int64
		charged   int64
		wantError error
	}{
		{name: "shorter output", duration: 2_500, price: 10_000, quote: 50_000, quantity: 3, measured: 30_000, charged: 30_000},
		{name: "automatic over ceiling", duration: 6_000, price: 10_000, quote: 50_000, quantity: 6, measured: 60_000, charged: 60_000, wantError: ErrBillingChargeLimit},
		{name: "review caps platform difference", duration: 6_000, price: 10_000, quote: 50_000, reviewed: true, quantity: 6, measured: 60_000, charged: 50_000},
		{name: "explicit zero cannot be increased", duration: 1_000, price: 10_000, limitSet: true, quantity: 1, measured: 10_000, charged: 10_000, wantError: ErrBillingChargeLimit},
		{name: "review honors zero", duration: 1_000, price: 10_000, limitSet: true, reviewed: true, quantity: 1, measured: 10_000},
		{name: "explicit higher authorization", duration: 6_000, price: 10_000, quote: 50_000, limit: 70_000, limitSet: true, quantity: 6, measured: 60_000, charged: 60_000},
		{name: "small price rounds to quantum", duration: 1_000, price: 1, quote: 10_000, quantity: 1, measured: 10_000, charged: 10_000},
		{name: "unmeasured", quote: 50_000, wantError: ErrMeasuredDurationUnavailable},
		{name: "negative duration", duration: -1, quote: 50_000, wantError: ErrMeasuredDurationUnavailable},
		{name: "negative authorization", duration: 1_000, quote: 50_000, limitSet: true, limit: -1, wantError: ErrBillingChargeLimit},
		{name: "maximum free duration", duration: math.MaxInt64, quantity: math.MaxInt64/1_000 + 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			order := model.BillingOrder{UnitPriceMicrocredits: scenario.price, MultiplierBasisPoints: 10_000, AmountMicrocredits: scenario.quote, ChargeLimitSet: scenario.limitSet, ChargeLimitMicrocredits: scenario.limit}
			charge, err := calculateMeasuredMediaCharge(order, scenario.duration, scenario.reviewed)
			if !errors.Is(err, scenario.wantError) || charge.Quantity != scenario.quantity || charge.MeasuredAmount != scenario.measured || charge.Amount != scenario.charged {
				t.Fatalf("charge=%#v err=%v", charge, err)
			}
			wantLimit := scenario.quote
			if scenario.limitSet {
				wantLimit = scenario.limit
			}
			if charge.Limit != wantLimit {
				t.Fatalf("authorization changed: %d", charge.Limit)
			}
		})
	}
}

func TestMediaChargeArithmeticRejectsOverflowAndRoundsTotalOnce(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		price      int64
		quantity   int64
		multiplier int64
		want       int64
		invalid    bool
	}{
		{name: "combined quantum", price: 10_000, quantity: 3, multiplier: 3_333, want: 10_000},
		{name: "free", quantity: 2, multiplier: 10_000},
		{name: "negative price", price: -1, quantity: 1, multiplier: 10_000, invalid: true},
		{name: "zero multiplier", price: 10_000, quantity: 1, invalid: true},
		{name: "multiplication overflow", price: math.MaxInt64, quantity: 2, multiplier: 10_000, invalid: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			amount, err := audioBillingAmount(scenario.price, scenario.quantity, scenario.multiplier)
			if (err != nil) != scenario.invalid || amount != scenario.want {
				t.Fatalf("amount=%d err=%v", amount, err)
			}
		})
	}
}
