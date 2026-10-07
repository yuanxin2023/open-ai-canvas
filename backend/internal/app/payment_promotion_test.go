package app

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func TestPaymentPromotionActivityBoundariesAndProductPrices(t *testing.T) {
	start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60)).UTC()
	setting := PaymentPromotionSetting{
		CardEnabled: true, ImageResourceID: "image", ActivityEnabled: true,
		ActiveTitle: "限时优惠", StartsAt: &start, DurationDays: 3,
	}
	product := model.TopupProduct{AmountFen: 500, CompareAmountFen: 1000}

	for _, test := range []struct {
		name       string
		now        time.Time
		active     bool
		amountFen  int64
		compareFen int64
	}{
		{name: "before", now: start.Add(-time.Nanosecond), active: false, amountFen: 1000},
		{name: "start", now: start, active: true, amountFen: 500, compareFen: 1000},
		{name: "during", now: start.Add(48 * time.Hour), active: true, amountFen: 500, compareFen: 1000},
		{name: "end", now: start.Add(72 * time.Hour), active: false, amountFen: 1000},
	} {
		t.Run(test.name, func(t *testing.T) {
			active := paymentPromotionActive(setting, test.now)
			if active != test.active {
				t.Fatalf("active = %v, want %v", active, test.active)
			}
			view := publicTopupProduct(product, active)
			if view.AmountFen != test.amountFen || view.CompareAmountFen != test.compareFen {
				t.Fatalf("price = %d/%d, want %d/%d", view.AmountFen, view.CompareAmountFen, test.amountFen, test.compareFen)
			}
		})
	}
}

func TestPaymentPromotionWithoutComparePriceIsUnchanged(t *testing.T) {
	product := model.TopupProduct{AmountFen: 880, CompareAmountFen: 0}
	for _, active := range []bool{false, true} {
		view := publicTopupProduct(product, active)
		if view.AmountFen != product.AmountFen || view.CompareAmountFen != 0 {
			t.Fatalf("active %v changed product: %#v", active, view)
		}
	}
}

func TestValidatePaymentPromotionRequiresEnabledDependencies(t *testing.T) {
	if err := validatePaymentPromotion(PaymentPromotionSetting{CardEnabled: true}); err == nil {
		t.Fatal("enabled card without image was accepted")
	}
	start := time.Now()
	if err := validatePaymentPromotion(PaymentPromotionSetting{ImageResourceID: "image", ActivityEnabled: true, StartsAt: &start, DurationDays: 3}); err == nil {
		t.Fatal("enabled activity without title was accepted")
	}
	if err := validatePaymentPromotion(PaymentPromotionSetting{InactiveCopyEnabled: true}); err == nil {
		t.Fatal("enabled inactive copy without title was accepted")
	}
	valid := PaymentPromotionSetting{CardEnabled: true, ImageResourceID: "image", ActivityEnabled: true, ActiveTitle: "活动", StartsAt: &start, DurationDays: 3, InactiveCopyEnabled: true, InactiveTitle: "日常"}
	if err := validatePaymentPromotion(valid); err != nil {
		t.Fatalf("valid promotion rejected: %v", err)
	}
}
