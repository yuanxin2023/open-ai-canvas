package app

import (
	"errors"
	"testing"

	"infinite-canvas/backend/internal/protocol"
)

type paymentOrderCounterFunc func(string, string) (int64, error)

func (f paymentOrderCounterFunc) ActivePaymentOrderCountForPlugin(id, version string) (int64, error) {
	return f(id, version)
}

func TestPaymentBootstrapDefersChangedPackagesUntilOrdersSettle(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		currentHash, nextHash string
		payment               bool
		count                 int64
		unavailable           bool
		cause                 error
		deferred              bool
		checked               bool
	}{
		{name: "unresolved payment", currentHash: "old", nextHash: "new", payment: true, count: 1, deferred: true, checked: true},
		{name: "settled payment", currentHash: "old", nextHash: "new", payment: true, checked: true},
		{name: "same package reinstall", currentHash: "same", nextHash: "same", payment: true, count: 3},
		{name: "same version changed bytes", currentHash: "old", nextHash: "new", payment: true, count: 2, deferred: true, checked: true},
		{name: "unknown counter", currentHash: "old", nextHash: "new", payment: true, unavailable: true, deferred: true},
		{name: "database check failed", currentHash: "old", nextHash: "new", payment: true, cause: errors.New("unavailable"), deferred: true, checked: true},
		{name: "invalid count", currentHash: "old", nextHash: "new", payment: true, count: -1, deferred: true, checked: true},
		{name: "declarative model package", currentHash: "old", nextHash: "new", count: 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := protocol.Manifest{Metadata: protocol.Metadata{ID: "plugin", Version: "1.0.0"}}
			if tc.payment {
				manifest.Contributes.PaymentProviders = []protocol.ManifestPaymentProvider{{ID: "pay", Label: "Pay"}}
			}
			checked := false
			var counter paymentPluginOrderCounter = paymentOrderCounterFunc(func(id, version string) (int64, error) {
				checked = true
				if id != "plugin" || version != "1.0.0" {
					t.Fatal("counter did not use installed identity")
				}
				return tc.count, tc.cause
			})
			if tc.unavailable {
				counter = nil
			}
			got, err := deferInstalledPaymentUpdate(manifest, tc.currentHash, tc.nextHash, counter)
			if got != tc.deferred || checked != tc.checked {
				t.Fatalf("deferred=%v checked=%v err=%v", got, checked, err)
			}
			if tc.unavailable || tc.cause != nil || tc.count < 0 {
				if err == nil {
					t.Fatal("missing reason for unavailable check")
				}
			}
		})
	}
}
