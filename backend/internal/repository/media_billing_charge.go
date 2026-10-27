package repository

import (
	"errors"
	"fmt"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

type measuredMediaCharge struct {
	Quantity       int64
	MeasuredAmount int64
	Amount         int64
	Limit          int64
}

func isMeasuredMediaBilling(order model.BillingOrder, durationMs *int64) bool {
	return order.BillingMode == "per_second" && (order.Capability == "audio" || (order.Capability == "video" && durationMs != nil))
}

func calculateMeasuredMediaCharge(order model.BillingOrder, durationMs int64, reviewed bool) (measuredMediaCharge, error) {
	charge := measuredMediaCharge{Limit: order.AmountMicrocredits}
	if billingChargeLimitApplies(order) {
		charge.Limit = order.ChargeLimitMicrocredits
	}
	if charge.Limit < 0 {
		return charge, ErrBillingChargeLimit
	}
	if durationMs <= 0 {
		return charge, ErrMeasuredDurationUnavailable
	}
	charge.Quantity = durationMs / 1000
	if durationMs%1000 != 0 {
		charge.Quantity++
	}
	amount, err := audioBillingAmount(order.UnitPriceMicrocredits, charge.Quantity, order.MultiplierBasisPoints)
	if err != nil {
		return charge, err
	}
	charge.MeasuredAmount, charge.Amount = amount, amount
	if amount > charge.Limit {
		if !reviewed {
			return charge, ErrBillingChargeLimit
		}
		charge.Amount = charge.Limit
	}
	return charge, nil
}

func audioBillingAmount(unitPrice int64, quantity int64, multiplierBPS int64) (int64, error) {
	if unitPrice < 0 || quantity <= 0 || multiplierBPS <= 0 {
		return 0, errors.New("积分计费参数无效")
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	if unitPrice > maxInt64/quantity || unitPrice*quantity > (maxInt64-9_999)/multiplierBPS {
		return 0, errors.New("积分计费金额溢出")
	}
	amount := (unitPrice*quantity*multiplierBPS + 9_999) / 10_000
	if amount < 0 {
		return 0, fmt.Errorf("积分计费金额无效：%d", amount)
	}
	return kernel.RoundCreditChargeUp(amount)
}
