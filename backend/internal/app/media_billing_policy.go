package app

import "infinite-canvas/backend/internal/model"

func usesMeasuredMediaSettlement(mode, capability, operation string) bool {
	return mode == "per_second" && (capability == "audio" || (capability == "video" && operation == VideoProcessingOperationEnhance))
}

func authorizeMeasuredMediaOrder(order *model.BillingOrder, operation string) {
	if !usesMeasuredMediaSettlement(order.BillingMode, order.Capability, operation) || order.ChargeLimitSet || order.ChargeLimitMicrocredits != 0 {
		return
	}
	// Pin authorization when quoting the order, before a later route can change
	// its price snapshot. A zero quote must remain an explicit zero authorization.
	order.ChargeLimitSet = true
	order.ChargeLimitMicrocredits = order.AmountMicrocredits
}
