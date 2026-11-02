package referrals

import (
	"testing"
	"time"
)

func TestOrderStagesAndNewcomer(t *testing.T) {
	p := Default()
	p.Enabled = true
	p.NewcomerEnabled = true
	start := time.Now()
	for _, tc := range []struct{ order, used, reward, bonus int64 }{
		{1, 0, 240 * Scale, 60 * Scale}, {2, 240 * Scale, 60 * Scale, 0},
		{3, 300 * Scale, 0, 0}, {4, 0, 60 * Scale, 0},
	} {
		r, err := Evaluate(p, 1200*Scale, tc.order, tc.used, start, start)
		if err != nil || r.Reward != tc.reward || r.Newcomer != tc.bonus {
			t.Fatalf("order %d: %+v %v", tc.order, r, err)
		}
	}
}

func TestTimeBoundariesAndIndependentLimits(t *testing.T) {
	p := Default()
	p.Enabled = true
	p.Mode = "time"
	p.Stages = []Stage{{30, 2000}, {90, 500}}
	p.PerInviteeCapMicrocredits = 0
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		elapsed time.Duration
		want    int64
	}{
		{0, 240 * Scale}, {30*24*time.Hour - time.Nanosecond, 240 * Scale}, {30 * 24 * time.Hour, 60 * Scale}, {90 * 24 * time.Hour, 0},
	} {
		r, err := Evaluate(p, 1200*Scale, 2, 0, start, start.Add(tc.elapsed))
		if err != nil || r.Reward != tc.want {
			t.Fatalf("%v: %+v %v", tc.elapsed, r, err)
		}
	}
	p.MaxOrders = 1
	r, _ := Evaluate(p, 1200*Scale, 2, 0, start, start)
	if r.Reward != 0 {
		t.Fatal("order limit ignored")
	}
	p.MaxOrders = 0
	p.MaxDays = 10
	r, _ = Evaluate(p, 1200*Scale, 2, 0, start, start.Add(10*24*time.Hour))
	if r.Reward != 0 {
		t.Fatal("day limit ignored")
	}
}

func TestPolicyValidationAndRounding(t *testing.T) {
	p := Default()
	for _, stages := range [][]Stage{{{0, 500}, {3, 2000}}, {{3, 2000}, {2, 500}}, {{1, 10001}}, {{-1, 10}}, {}} {
		p.Stages = stages
		if p.Validate() == nil {
			t.Fatalf("accepted %+v", stages)
		}
	}
	got, err := Credits(1_234_567, 333)
	if err != nil || got != 40000 {
		t.Fatalf("rounding: %d %v", got, err)
	}
	p = Default()
	p.Enabled = true
	p.NewcomerEnabled = true
	p.NewcomerCapMicrocredits = 5 * Scale
	r, err := Evaluate(p, 1200*Scale, 1, 299*Scale, time.Now().Add(-time.Hour), time.Now())
	if err != nil || r.Reward != Scale || r.Newcomer != 5*Scale || r.Uncapped != 240*Scale {
		t.Fatalf("caps: %+v %v", r, err)
	}
}
