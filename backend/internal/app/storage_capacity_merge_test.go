package app

import (
	"errors"
	"testing"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/platform"
	"infinite-canvas/backend/internal/repository"
)

func TestStorageExpansionRetainsAccountQuotaAndReadOnlyRecovery(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			s, db, actor := quotaTestService(t, driver)
			base := int64(1)
			if _, err := s.PatchAdminUserResourceQuota(actor, "one", UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"storedFileGB": &base}}); err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.Resource{ID: "stored", UserID: "one", Status: model.ResourceStatusReady, Size: 2 << 30}).Error; err != nil {
				t.Fatal(err)
			}
			global, err := s.RuntimePolicy()
			if err != nil {
				t.Fatal(err)
			}
			if err := s.requireStoredFileCapacityForTask("one", "canvas_image", global); err == nil {
				t.Fatal("task admission ignored the account override")
			}
			grant, err := s.AdminGrantUserStorage(actor, "one", AdminStorageGrantRequest{Bytes: 2 << 30, DurationDays: 30})
			if err != nil {
				t.Fatal(err)
			}
			usage, err := s.AccountFileStorageUsage("one")
			if err != nil || usage.BaseBytes != 1<<30 || usage.PurchasedBytes != 2<<30 || usage.TotalBytes != 3<<30 || usage.OverLimit {
				t.Fatalf("expanded quota: %+v %v", usage, err)
			}
			policy, err := s.UserRuntimePolicy("one")
			if err != nil {
				t.Fatal(err)
			}
			limit, err := s.storedFileLimit("one", policy)
			if err != nil || limit != usage.TotalBytes {
				t.Fatalf("admission differs from displayed capacity: %d %v", limit, err)
			}
			other, err := s.AccountFileStorageUsage("two")
			if err != nil || other.PurchasedBytes != 0 {
				t.Fatalf("cross-account grant: %+v %v", other, err)
			}
			if _, err := s.AdminRevokeUserStorageGrant(actor, grant.ID, RevokeStorageGrantRequest{Reason: "test revoke"}); err != nil {
				t.Fatal(err)
			}
			usage, err = s.AccountFileStorageUsage("one")
			if err != nil || !usage.OverLimit || usage.ReadOnly || usage.ReadOnlyAt == nil {
				t.Fatalf("grace period: %+v %v", usage, err)
			}
			since := time.Now().Add(-8 * 24 * time.Hour)
			if err := db.Model(&model.UserStorageState{}).Where("user_id = ?", "one").Update("over_limit_since", since).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := s.AccountFileStorageUsage("one"); err != nil {
				t.Fatal(err)
			}
			var appErr *kernel.AppError
			if err := s.RequireStorageWritable("one"); !errors.As(err, &appErr) || appErr.Reason != kernel.ReasonStorageReadOnly {
				t.Fatalf("expected read-only: %v", err)
			}
			base = 3
			if _, err := s.PatchAdminUserResourceQuota(actor, "one", UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"storedFileGB": &base}}); err != nil {
				t.Fatal(err)
			}
			if err := s.RequireStorageWritable("one"); err != nil {
				t.Fatalf("quota increase did not invalidate read-only: %v", err)
			}
		})
	}
}

func TestStorageGrantsRespectScopedAdministratorBoundaries(t *testing.T) {
	s, db, full := quotaTestService(t, "sqlite")
	scoped := &model.User{ID: "scoped", Username: "scoped", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionUsers}, Status: model.UserStatusActive}
	if err := db.Create(scoped).Error; err != nil {
		t.Fatal(err)
	}
	for _, denied := range []*model.User{nil, {ID: "one", Role: model.UserRoleUser}, {ID: "other-admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped}} {
		if _, err := s.AdminUserStorage(denied, "one"); err == nil {
			t.Fatal("unauthorized storage read")
		}
		if _, err := s.AdminGrantUserStorage(denied, "one", AdminStorageGrantRequest{Bytes: 1 << 30}); err == nil {
			t.Fatal("unauthorized storage grant")
		}
	}
	if _, err := s.AdminGrantUserStorage(scoped, "one", AdminStorageGrantRequest{Bytes: 1 << 30}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdminUserStorage(scoped, full.ID); err == nil {
		t.Fatal("scoped administrator read another administrator")
	}
	grant, err := s.AdminGrantUserStorage(full, full.ID, AdminStorageGrantRequest{Bytes: 1 << 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdminRevokeUserStorageGrant(scoped, grant.ID, RevokeStorageGrantRequest{Reason: "denied"}); err == nil {
		t.Fatal("scoped administrator revoked administrator capacity")
	}
	if _, err := s.referralRewardDraft(&model.PaymentOrder{ProductKind: model.TopupProductKindStorage, AmountFen: 100, Currency: "CNY"}); err != nil {
		t.Fatal(err)
	}
}

func TestStoragePaymentPreservesSnapshotAndGrantsOnlyOnce(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			s, db, _ := quotaTestService(t, driver)
			product := model.TopupProduct{ID: "storage", Name: "Storage", Kind: model.TopupProductKindStorage, StorageBytes: 2 << 30, StorageDurationDays: 30, AmountFen: 500, Enabled: true}
			if err := db.Create(&product).Error; err != nil {
				t.Fatal(err)
			}
			order := &model.PaymentOrder{ID: "storage-order", UserID: "one", IdempotencyKey: "storage-key", MerchantOrderNo: "storage-merchant", ProductID: product.ID, ProductKind: product.Kind, StorageBytes: product.StorageBytes, StorageDurationDays: product.StorageDurationDays, AmountFen: product.AmountFen, ProviderID: "test-provider", Currency: "CNY", Status: model.PaymentOrderCreated, ExpiresAt: time.Now().Add(time.Hour)}
			quote := func(p model.TopupProduct, _ time.Time) int64 { return p.AmountFen }
			if _, created, err := s.repo.CreatePaymentOrderWithProductReservation(order, quote); err != nil || !created {
				t.Fatalf("storage order admission: %t %v", created, err)
			}
			if err := db.Model(&product).Updates(map[string]any{"storage_bytes": 9 << 30, "amount_fen": 900}).Error; err != nil {
				t.Fatal(err)
			}
			for attempt := range 2 {
				paid, granted, err := s.completePaymentOrderWithReferral(order.ProviderID, order.MerchantOrderNo, repository.PaymentEvidence{ProviderTradeNo: "trade", AmountFen: 500, Currency: "CNY", PaidAt: time.Now()})
				if err != nil || paid.StorageBytes != 2<<30 || granted != (attempt == 0) {
					t.Fatalf("payment replay changed snapshot or granted twice: %+v %t %v", paid, granted, err)
				}
			}
			var grants []model.UserStorageGrant
			if err := db.Where("user_id = ?", "one").Find(&grants).Error; err != nil || len(grants) != 1 || grants[0].Bytes != 2<<30 {
				t.Fatalf("invalid storage grants: %+v %v", grants, err)
			}
			for _, table := range []any{&model.CreditLedgerEntry{}, &model.ReferralReward{}} {
				var count int64
				if err := db.Model(table).Where("payment_order_id = ?", order.ID).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("storage order entered credit or referral ledger: %d %v", count, err)
				}
			}
		})
	}
}
