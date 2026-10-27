package kernel

import (
	"math"
	"testing"
)

func TestRoundCreditChargeUp(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		amount  int64
		want    int64
		invalid bool
	}{
		{name: "negative", amount: -1, invalid: true},
		{name: "free", amount: 0},
		{name: "one microcredit", amount: 1, want: 10_000},
		{name: "one thousandth credit", amount: 1_000, want: 10_000},
		{name: "below quantum", amount: 9_999, want: 10_000},
		{name: "exact quantum", amount: 10_000, want: 10_000},
		{name: "above quantum", amount: 10_001, want: 20_000},
		{name: "maximum quantum", amount: math.MaxInt64 / 10_000 * 10_000, want: math.MaxInt64 / 10_000 * 10_000},
		{name: "rounding overflow", amount: math.MaxInt64/10_000*10_000 + 1, invalid: true},
		{name: "maximum integer", amount: math.MaxInt64, invalid: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			got, err := RoundCreditChargeUp(scenario.amount)
			if (err != nil) != scenario.invalid || got != scenario.want {
				t.Fatalf("amount=%d got=%d err=%v", scenario.amount, got, err)
			}
		})
	}
}

func TestTokenChargeRoundsCombinedUsageOnce(t *testing.T) {
	amount, err := TokenBillingAmount(10_000, TokenBillingTerm{Tokens: 3_000, PriceMicrocredits: 1_000_000}, TokenBillingTerm{Tokens: 3_000, PriceMicrocredits: 1_000_000})
	if err != nil {
		t.Fatal(err)
	}
	charge, err := RoundCreditChargeUp(amount)
	if err != nil || charge != 10_000 {
		t.Fatalf("combined charge=%d err=%v", charge, err)
	}
}
