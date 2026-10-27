package model

import (
	"testing"
	"time"
)

func TestTopupSaleAvailability(t *testing.T) {
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	end := start.Add(time.Hour)
	for _, test := range []struct {
		name        string
		product     TopupProduct
		now         time.Time
		status      string
		canPurchase bool
	}{
		{"existing product defaults unlimited", TopupProduct{Enabled: true}, start, "on_sale", true},
		{"disabled product", TopupProduct{SaleStrategy: TopupSaleStrategyUnlimited}, start, "disabled", false},
		{"unknown strategy fails closed", TopupProduct{Enabled: true, SaleStrategy: "invalid"}, start, "unavailable", false},
		{"periodic configured", TopupProduct{Enabled: true, SaleStrategy: TopupSaleStrategyPeriodic, PeriodDays: 7, PeriodPurchaseLimit: 1}, start, "on_sale", true},
		{"periodic missing limit", TopupProduct{Enabled: true, SaleStrategy: TopupSaleStrategyPeriodic, PeriodDays: 7}, start, "unavailable", false},
		{"periodic invalid days", TopupProduct{Enabled: true, SaleStrategy: TopupSaleStrategyPeriodic, PeriodDays: 3651, PeriodPurchaseLimit: 1}, start, "unavailable", false},
		{"stock available", TopupProduct{Enabled: true, SaleStrategy: TopupSaleStrategyInventory, StockTotal: 3, StockRemaining: 1}, start, "on_sale", true},
		{"stock sold out", TopupProduct{Enabled: true, SaleStrategy: TopupSaleStrategyInventory, StockTotal: 3}, start, "sold_out", false},
		{"negative stock cannot sell", TopupProduct{Enabled: true, SaleStrategy: TopupSaleStrategyInventory, StockTotal: 3, StockRemaining: -1}, start, "sold_out", false},
		{"invalid stock cannot sell", TopupProduct{Enabled: true, SaleStrategy: TopupSaleStrategyInventory, StockTotal: 3, StockRemaining: 4}, start, "unavailable", false},
		{"before timed sale", TopupProduct{Enabled: true, SaleStrategy: TopupSaleStrategyTimed, SaleStartAt: &start, SaleEndAt: &end}, start.Add(-time.Nanosecond), "upcoming", false},
		{"start inclusive", TopupProduct{Enabled: true, SaleStrategy: TopupSaleStrategyTimed, SaleStartAt: &start, SaleEndAt: &end}, start, "on_sale", true},
		{"same instant across timezone", TopupProduct{Enabled: true, SaleStrategy: TopupSaleStrategyTimed, SaleStartAt: &start, SaleEndAt: &end}, start.In(time.FixedZone("UTC+8", 8*3600)), "on_sale", true},
		{"end exclusive", TopupProduct{Enabled: true, SaleStrategy: TopupSaleStrategyTimed, SaleStartAt: &start, SaleEndAt: &end}, end, "ended", false},
		{"missing timed end", TopupProduct{Enabled: true, SaleStrategy: TopupSaleStrategyTimed, SaleStartAt: &start}, start, "unavailable", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, canPurchase := test.product.SaleAvailability(test.now)
			if status != test.status || canPurchase != test.canPurchase {
				t.Fatalf("got %s,%t; want %s,%t", status, canPurchase, test.status, test.canPurchase)
			}
		})
	}
}
