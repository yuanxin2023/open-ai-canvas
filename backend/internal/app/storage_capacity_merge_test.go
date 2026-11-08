package app

import (
	"encoding/json"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/platform"
	"infinite-canvas/backend/internal/repository"
)

func TestStorageCapacityUsesOnlyAccountQuota(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			s, db, actor := quotaTestService(t, driver)
			base := int64(1)
			if _, err := s.PatchAdminUserResourceQuota(actor, "one", UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"storedFileGB": &base}}); err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.UserStorageGrant{ID: "legacy", UserID: "one", Source: "admin", Bytes: 10 << 30, StartsAt: time.Now().Add(-time.Hour)}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.Resource{ID: "stored", UserID: "one", Status: model.ResourceStatusReady, Size: 2 << 30}).Error; err != nil {
				t.Fatal(err)
			}
			usage, err := s.AccountFileStorageUsage("one")
			if err != nil || usage.TotalBytes != 1<<30 || !usage.OverLimit {
				t.Fatalf("usage: %+v %v", usage, err)
			}
			encoded, _ := json.Marshal(usage)
			var fields map[string]any
			_ = json.Unmarshal(encoded, &fields)
			if len(fields) != 3 {
				t.Fatalf("unexpected usage fields: %s", encoded)
			}
			policy, err := s.RuntimePolicy()
			if err != nil {
				t.Fatal(err)
			}
			if err := s.requireStoredFileCapacityForTask("one", "canvas_image", policy); err == nil {
				t.Fatal("media task bypassed personal capacity")
			}
			base = 3
			if _, err := s.PatchAdminUserResourceQuota(actor, "one", UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"storedFileGB": &base}}); err != nil {
				t.Fatal(err)
			}
			usage, err = s.AccountFileStorageUsage("one")
			if err != nil || usage.TotalBytes != 3<<30 || usage.OverLimit {
				t.Fatalf("updated quota: %+v %v", usage, err)
			}
		})
	}
}

func TestIndependentStorageProductsAreRejected(t *testing.T) {
	s, db, actor := quotaTestService(t, "sqlite")
	if _, err := s.CreateTopupProduct(actor, TopupProductRequest{Name: "storage", Kind: "storage", AmountFen: 500, StorageBytes: 2 << 30}); err == nil {
		t.Fatal("storage product accepted")
	}
	if topupProductKind("invalid") == model.TopupProductKindCredits {
		t.Fatal("unknown kind became credits")
	}
	product := model.TopupProduct{ID: "storage", Name: "storage", Kind: "storage", StorageBytes: 2 << 30, AmountFen: 500, Enabled: true}
	if err := db.Create(&product).Error; err != nil {
		t.Fatal(err)
	}
	items, err := s.AdminTopupProducts(actor)
	if err != nil || len(items) != 0 {
		t.Fatalf("storage product visible: %+v %v", items, err)
	}
	order := &model.PaymentOrder{ID: "old-order", UserID: "one", ProductID: product.ID, ProductKind: "storage", StorageBytes: 2 << 30, AmountFen: 500, Currency: "CNY", ProviderID: "test", MerchantOrderNo: "merchant", IdempotencyKey: "key", Status: model.PaymentOrderCreated, ExpiresAt: time.Now().Add(time.Hour)}
	if _, _, err := s.repo.CreatePaymentOrderWithProductReservation(order); err == nil {
		t.Fatal("storage order admitted")
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.repo.CompletePaymentOrder("test", "merchant", repository.PaymentEvidence{AmountFen: 500, Currency: "CNY", ProviderTradeNo: "trade"}); err == nil {
		t.Fatal("storage callback granted")
	}
	var grants int64
	if err := db.Model(&model.UserStorageGrant{}).Count(&grants).Error; err != nil || grants != 0 {
		t.Fatal("unexpected grant", grants, err)
	}
}
