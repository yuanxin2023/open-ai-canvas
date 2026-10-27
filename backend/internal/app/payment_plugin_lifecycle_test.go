package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/payment"
	"infinite-canvas/backend/internal/protocol"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPaymentLifecycleService(t *testing.T, status model.PaymentOrderStatus, configure ...func(*model.PaymentOrder)) (*Service, protocol.Manifest) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+newID()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.PaymentOrder{}, &model.TopupProduct{}, &model.PaymentProviderConfig{}, &model.PluginPlatformState{}, &model.UserPluginState{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	manifest := bundledPaymentPluginManifests()[0]
	manifest.Metadata.ID = "uploaded-payment"
	manifest.Metadata.Version = "1.0.0"
	manifest.Metadata.Enabled = true
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	center := &pluginRuntime{plugins: map[string]pluginRecord{
		manifest.Metadata.ID: {Raw: raw, Metadata: manifest.Metadata, Source: PluginOriginUploaded, Status: "enabled", PackageSHA256: "installed-package"},
	}}
	order := model.PaymentOrder{ID: "active-order", UserID: "user", IdempotencyKey: "key", MerchantOrderNo: "merchant", ProviderID: manifest.Contributes.PaymentProviders[0].ID, PluginID: manifest.Metadata.ID, PluginVersion: manifest.Metadata.Version, Status: status}
	for _, apply := range configure {
		apply(&order)
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	return &Service{repo: repository.New(db), pluginRuntime: center}, manifest
}

func TestPaymentPluginUpgradeBlockedByActiveOrders(t *testing.T) {
	svc, next := newPaymentLifecycleService(t, model.PaymentOrderPending)
	next.Metadata.Version = "2.0.0"
	if err := svc.ensurePaymentPluginLifecycle(next, "replacement-package"); err == nil || !strings.Contains(err.Error(), "未完成订单") {
		t.Fatalf("upgrade guard error = %v", err)
	}
}

func TestPaymentPluginRemovalAllowedAfterOrdersFinish(t *testing.T) {
	svc, manifest := newPaymentLifecycleService(t, model.PaymentOrderCredited)
	if err := svc.ensurePaymentPluginCanBeRemoved(manifest.Metadata.ID); err != nil {
		t.Fatalf("removal guard rejected completed order: %v", err)
	}
}

func TestPaymentPluginChangeProtectsUnconfirmedPurchaseOccupancy(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		stock     bool
		periodic  bool
		age       time.Duration
		confirmed bool
		blocked   bool
	}{
		{name: "inventory held", stock: true, blocked: true},
		{name: "period quota held", periodic: true, age: time.Hour, blocked: true},
		{name: "period window expired", periodic: true, age: 8 * 24 * time.Hour},
		{name: "inventory released", confirmed: true},
		{name: "period confirmed closed", periodic: true, confirmed: true},
		{name: "unlimited local close"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			svc, next := newPaymentLifecycleService(t, model.PaymentOrderClosed, func(order *model.PaymentOrder) {
				order.ProductID = "product"
				order.StockReserved = scenario.stock
				order.CreatedAt = time.Now().Add(-scenario.age)
				if scenario.periodic {
					order.SaleStrategySnapshot = model.TopupSaleStrategyPeriodic
				}
				if scenario.confirmed {
					now := time.Now()
					order.ProviderClosedAt = &now
				}
			})
			product := model.TopupProduct{ID: "product", SaleStrategy: model.TopupSaleStrategyUnlimited}
			if scenario.periodic {
				product.SaleStrategy = model.TopupSaleStrategyPeriodic
				product.PeriodDays = 7
				product.PeriodPurchaseLimit = 1
			}
			if err := svc.repo.CreateTopupProduct(&product); err != nil {
				t.Fatal(err)
			}
			next.Metadata.Version = "2.0.0"
			if err := svc.ensurePaymentPluginLifecycle(next, "replacement-package"); (err != nil) != scenario.blocked {
				t.Fatalf("upgrade guard: %v", err)
			}
			if err := svc.ensurePaymentPluginCanBeRemoved(next.Metadata.ID); (err != nil) != scenario.blocked {
				t.Fatalf("removal guard: %v", err)
			}
		})
	}
}

func TestPaymentPluginReplacementCannotBypassGuardWithSameVersionOrNoProviders(t *testing.T) {
	svc, next := newPaymentLifecycleService(t, model.PaymentOrderPending)
	if err := svc.ensurePaymentPluginLifecycle(next, "installed-package"); err != nil {
		t.Fatalf("identical reinstall rejected: %v", err)
	}
	if err := svc.ensurePaymentPluginLifecycle(next, "different-package"); err == nil {
		t.Fatal("same-version replacement bypassed active order guard")
	}
	next.Contributes.PaymentProviders = nil
	if err := svc.ensurePaymentPluginLifecycle(next, "different-package"); err == nil {
		t.Fatal("dropping payment contribution bypassed active order guard")
	}
}

type paymentPauseRuntime struct {
	payment.Provider
	descriptor payment.Descriptor
	queries    int
}

func (p *paymentPauseRuntime) Descriptor() payment.Descriptor { return p.descriptor }

func (p *paymentPauseRuntime) QueryOrder(_ context.Context, _ payment.Config, request payment.QueryRequest) (payment.Result, error) {
	p.queries++
	return payment.Result{MerchantOrderNo: request.MerchantOrderNo, ProviderStatus: "WAIT"}, nil
}

func TestPausedPaymentPluginRetainsHistoricalRuntime(t *testing.T) {
	svc, manifest := newPaymentLifecycleService(t, model.PaymentOrderPending, func(order *model.PaymentOrder) {
		order.ProviderConfigID = "pause-config"
		order.ProviderConfigVersion = 1
	})
	provider := &paymentPauseRuntime{descriptor: payment.Descriptor{ID: manifest.Contributes.PaymentProviders[0].ID, PluginID: manifest.Metadata.ID, PluginVersion: manifest.Metadata.Version}}
	registry, err := payment.NewRegistry(provider)
	if err != nil {
		t.Fatal(err)
	}
	svc.paymentRegistry = registry
	config := &model.PaymentProviderConfig{ID: "pause-config", ProviderID: provider.descriptor.ID, PluginID: manifest.Metadata.ID, PluginVersion: manifest.Metadata.Version, ConfigCipher: "{}", Enabled: true}
	if err := svc.repo.CreatePaymentProviderConfig(config); err != nil {
		t.Fatal(err)
	}
	actor := &model.User{ID: "full-admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull}
	paused, err := svc.SetPluginPlatformAvailability(actor, manifest.Metadata.ID, false)
	if err != nil || paused.PlatformAvailable || paused.EffectiveEnabled {
		t.Fatalf("pause state=%#v err=%v", paused, err)
	}
	view, _, err := svc.paymentProviderView(provider.descriptor)
	if err != nil || view.Enabled {
		t.Fatalf("paused provider accepted for new orders: %#v %v", view, err)
	}
	order, err := svc.repo.PaymentOrder("active-order")
	if err != nil {
		t.Fatal(err)
	}
	runtime, _, values, err := svc.paymentRuntimeForOrder(order)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.QueryOrder(context.Background(), values, payment.QueryRequest{MerchantOrderNo: order.MerchantOrderNo}); err != nil || provider.queries != 1 {
		t.Fatalf("historical query unavailable: %v queries=%d", err, provider.queries)
	}
	svc.pluginRuntime.mu.RLock()
	record := svc.pluginRuntime.plugins[manifest.Metadata.ID]
	svc.pluginRuntime.mu.RUnlock()
	if record.Status != "enabled" || !record.Metadata.Enabled {
		t.Fatal("pause disabled persisted runtime metadata")
	}
	if _, err := svc.SetPluginPlatformAvailability(actor, manifest.Metadata.ID, true); err != nil {
		t.Fatal(err)
	}
	view, _, err = svc.paymentProviderView(provider.descriptor)
	if err != nil || !view.Enabled {
		t.Fatalf("provider did not resume: %#v %v", view, err)
	}
}
