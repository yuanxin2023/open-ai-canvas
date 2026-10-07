package app

import "testing"

func TestCalculateReferralRewardUsesPaidFenAndRoundsDown(t *testing.T) {
	tests := []struct {
		name       string
		amountFen  int64
		conversion int64
		rateBPS    int64
		want       int64
	}{
		{name: "ten percent of one yuan", amountFen: 100, conversion: 1_000_000, rateBPS: 1_000, want: 100_000},
		{name: "custom conversion", amountFen: 1_999, conversion: 2_500_000, rateBPS: 1_500, want: 7_490_000},
		{name: "round to hundredth credit", amountFen: 1, conversion: 1_000_000, rateBPS: 333, want: 0},
		{name: "zero rate", amountFen: 100, conversion: 1_000_000, rateBPS: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculateReferralReward(tt.amountFen, tt.conversion, tt.rateBPS)
			if err != nil || got != tt.want {
				t.Fatalf("calculateReferralReward() = %d, %v; want %d", got, err, tt.want)
			}
		})
	}
}

func TestCalculateReferralRewardRejectsOverflow(t *testing.T) {
	if _, err := calculateReferralReward(1_000_000_000_000, 1_000_000_000, 10_000); err == nil {
		t.Fatal("expected overflow to be rejected")
	}
}
