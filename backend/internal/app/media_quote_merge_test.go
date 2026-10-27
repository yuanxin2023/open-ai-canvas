package app

import (
	"encoding/json"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestQuoteExposesMeasuredSettlementAndZeroAuthorization(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		capability string
		mode       string
		operation  string
		amount     int64
		measured   bool
	}{
		{name: "audio measured", capability: "audio", mode: "per_second", amount: 50_000, measured: true},
		{name: "audio zero authorization", capability: "audio", mode: "per_second", measured: true},
		{name: "video processing measured", capability: "video", mode: "per_second", operation: VideoProcessingOperationEnhance, amount: 50_000, measured: true},
		{name: "ordinary video snapshot", capability: "video", mode: "per_second", amount: 50_000},
		{name: "fixed audio snapshot", capability: "audio", mode: "fixed_request", amount: 50_000},
		{name: "token usage separate", capability: "audio", mode: "token", amount: 50_000},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			order := &model.BillingOrder{Capability: scenario.capability, BillingMode: scenario.mode, AmountMicrocredits: scenario.amount, Quantity: 5}
			quote := quoteFromOrder("logical", order, tokenBillingEstimate{}, scenario.operation)
			encoded, err := json.Marshal(quote)
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatal(err)
			}
			if scenario.measured {
				if !quote.Estimated || quote.SettlementBasis != "measured_duration" || quote.ChargeLimitMicrocredits == nil || *quote.ChargeLimitMicrocredits != scenario.amount {
					t.Fatalf("measured quote: %s", encoded)
				}
				if _, exists := payload["chargeLimitMicrocredits"]; !exists {
					t.Fatalf("zero authorization disappeared: %s", encoded)
				}
			} else {
				if quote.SettlementBasis != "" || quote.ChargeLimitMicrocredits != nil || quote.Estimated != (scenario.mode == "token") {
					t.Fatalf("ordinary quote changed: %s", encoded)
				}
				if _, exists := payload["chargeLimitMicrocredits"]; exists {
					t.Fatalf("ordinary quote received measured authorization: %s", encoded)
				}
			}
		})
	}
}

func TestMeasuredMediaAuthorizationPreservesOriginalAndZeroLimits(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		capability string
		mode       string
		operation  string
		amount     int64
		oldLimit   int64
		oldSet     bool
		wantSet    bool
		wantLimit  int64
	}{
		{name: "audio quote pinned", capability: "audio", mode: "per_second", amount: 50_000, wantSet: true, wantLimit: 50_000},
		{name: "video processing pinned", capability: "video", mode: "per_second", operation: VideoProcessingOperationEnhance, amount: 50_000, wantSet: true, wantLimit: 50_000},
		{name: "ordinary video unchanged", capability: "video", mode: "per_second", amount: 50_000},
		{name: "fixed audio unchanged", capability: "audio", mode: "fixed_request", amount: 50_000},
		{name: "token unchanged", capability: "audio", mode: "token", amount: 50_000},
		{name: "zero quote pinned", capability: "audio", mode: "per_second", wantSet: true},
		{name: "explicit zero preserved", capability: "audio", mode: "per_second", amount: 50_000, oldSet: true, wantSet: true},
		{name: "existing limit preserved", capability: "audio", mode: "per_second", amount: 50_000, oldSet: true, oldLimit: 70_000, wantSet: true, wantLimit: 70_000},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			order := &model.BillingOrder{Capability: scenario.capability, BillingMode: scenario.mode, AmountMicrocredits: scenario.amount, ChargeLimitSet: scenario.oldSet, ChargeLimitMicrocredits: scenario.oldLimit}
			authorizeMeasuredMediaOrder(order, scenario.operation)
			if order.ChargeLimitSet != scenario.wantSet || order.ChargeLimitMicrocredits != scenario.wantLimit || order.AmountMicrocredits != scenario.amount {
				t.Fatalf("authorization: %#v", order)
			}
			order.AmountMicrocredits = 1_000_000
			authorizeMeasuredMediaOrder(order, scenario.operation)
			if order.ChargeLimitSet != scenario.wantSet || order.ChargeLimitMicrocredits != scenario.wantLimit {
				t.Fatal("later quote expanded authorization")
			}
		})
	}
}
