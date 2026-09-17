package app

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestPaymentOrderStatusesForUserFilter(t *testing.T) {
	tests := []struct {
		filter string
		want   []model.PaymentOrderStatus
	}{
		{filter: "all"},
		{filter: "unpaid", want: []model.PaymentOrderStatus{model.PaymentOrderCreated, model.PaymentOrderPending, model.PaymentOrderClosing}},
		{filter: "completed", want: []model.PaymentOrderStatus{model.PaymentOrderCredited}},
		{filter: "failed", want: []model.PaymentOrderStatus{model.PaymentOrderCreateFailed}},
		{filter: "closed", want: []model.PaymentOrderStatus{model.PaymentOrderClosed}},
	}
	for _, test := range tests {
		t.Run(test.filter, func(t *testing.T) {
			got, err := paymentOrderStatusesForUserFilter(test.filter)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(test.want) {
				t.Fatalf("statuses = %v, want %v", got, test.want)
			}
			for index := range got {
				if got[index] != test.want[index] {
					t.Fatalf("statuses = %v, want %v", got, test.want)
				}
			}
		})
	}
	if _, err := paymentOrderStatusesForUserFilter("credited"); err == nil {
		t.Fatal("raw payment status unexpectedly accepted as a user filter")
	}
}
