package model

import "time"

func (product TopupProduct) SaleAvailability(now time.Time) (string, bool) {
	if !product.Enabled {
		return "disabled", false
	}
	switch product.SaleStrategy {
	case "", TopupSaleStrategyUnlimited:
		return "on_sale", true
	case TopupSaleStrategyPeriodic:
		if product.PeriodDays < 1 || product.PeriodDays > 3650 || product.PeriodPurchaseLimit < 1 || product.PeriodPurchaseLimit > 100000 {
			return "unavailable", false
		}
		return "on_sale", true
	case TopupSaleStrategyInventory:
		if product.StockTotal <= 0 || product.StockRemaining > product.StockTotal {
			return "unavailable", false
		}
		if product.StockRemaining <= 0 {
			return "sold_out", false
		}
		return "on_sale", true
	case TopupSaleStrategyTimed:
		if product.SaleStartAt == nil || product.SaleEndAt == nil || !product.SaleEndAt.After(*product.SaleStartAt) {
			return "unavailable", false
		}
		if now.Before(*product.SaleStartAt) {
			return "upcoming", false
		}
		if !now.Before(*product.SaleEndAt) {
			return "ended", false
		}
		return "on_sale", true
	default:
		return "unavailable", false
	}
}
