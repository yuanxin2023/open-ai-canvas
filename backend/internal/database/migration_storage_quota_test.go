package database

import (
	"encoding/json"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func TestStorageGrantsToQuotaMigration(t *testing.T) {
	db := isolatedMigrationDB(t)
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	for _, user := range []model.User{{ID: "one", Username: "quotaone", Role: "user"}, {ID: "two", Username: "quotatwo", Role: "admin"}, {ID: "untouched", Username: "untouched", Role: "user"}} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.SystemSetting{Key: "runtime_policy", ValueJSON: `{"resource":{"storedFileGB":30}}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.UserResourceQuota{UserID: "one", OverridesJSON: `{"storedFileGB":40,"canvasCount":9}`, UpdatedBy: "original-admin"}).Error; err != nil {
		t.Fatal(err)
	}
	grants := []model.UserStorageGrant{
		{ID: "active", UserID: "one", Source: "admin", Bytes: 2 << 30, StartsAt: past, ExpiresAt: &future},
		{ID: "permanent", UserID: "one", Source: "admin", Bytes: 3 << 30, StartsAt: past},
		{ID: "expired", UserID: "one", Source: "admin", Bytes: 7 << 30, StartsAt: past, ExpiresAt: &past},
		{ID: "revoked", UserID: "one", Source: "admin", Bytes: 8 << 30, StartsAt: past, RevokedAt: &past},
		{ID: "future", UserID: "one", Source: "admin", Bytes: 9 << 30, StartsAt: future},
		{ID: "admin-gift", UserID: "two", Source: "admin", Bytes: 6 << 30, StartsAt: past},
	}
	if err := db.Create(&grants).Error; err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := migrateStorageGrantsToQuota(db); err != nil {
			t.Fatal(err)
		}
		for userID, want := range map[string]int64{"one": 45, "two": 36} {
			var quota model.UserResourceQuota
			if err := db.First(&quota, "user_id = ?", userID).Error; err != nil {
				t.Fatal(err)
			}
			var fields map[string]int64
			if err := json.Unmarshal([]byte(quota.OverridesJSON), &fields); err != nil {
				t.Fatal(err)
			}
			if fields["storedFileGB"] != want {
				t.Fatalf("quota %s: %+v", userID, fields)
			}
			if userID == "one" && (fields["canvasCount"] != 9 || quota.UpdatedBy != "original-admin") {
				t.Fatal("unrelated quota/actor changed", quota)
			}
		}
	}
	var count int64
	if err := db.Model(&model.UserStorageGrant{}).Count(&count).Error; err != nil || count != 6 {
		t.Fatal("history changed", count, err)
	}
	if err := db.Model(&model.UserResourceQuota{}).Where("user_id = ?", "untouched").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("unaffected account changed", count, err)
	}
	var audits []model.AdminAuditEvent
	if err := db.Where("action = ?", "storage_grants.migrate_to_quota").Find(&audits).Error; err != nil || len(audits) != 2 {
		t.Fatal("audit/idempotence", audits, err)
	}
	for _, audit := range audits {
		var metadata map[string]json.RawMessage
		if err := json.Unmarshal([]byte(audit.MetadataJSON), &metadata); err != nil || metadata["grantIds"] == nil || metadata["before"] == nil || metadata["after"] == nil {
			t.Fatal("missing conversion audit", audit, err)
		}
		if audit.ActorUserID != "" {
			t.Fatal("migration forged an administrator")
		}
	}
}

func TestStorageGrantsMigrationStopsAndRollsBack(t *testing.T) {
	for _, scenario := range []string{"overflow", "fractional", "payment_order", "unknown_grant", "missing_user"} {
		t.Run(scenario, func(t *testing.T) {
			db := isolatedMigrationDB(t)
			if err := MigrateSchema(db); err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.User{ID: "one", Username: "quotaone", Role: "user"}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.User{ID: "a-first", Username: "firstquota", Role: "user"}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.UserStorageGrant{ID: "valid-first", UserID: "a-first", Source: "admin", Bytes: 1 << 30, StartsAt: time.Now().Add(-time.Hour)}).Error; err != nil {
				t.Fatal(err)
			}
			grant := model.UserStorageGrant{ID: "gift", UserID: "one", Source: "admin", Bytes: 1 << 30, StartsAt: time.Now().Add(-time.Hour)}
			switch scenario {
			case "overflow":
				grant.Bytes = 1000 << 30
			case "fractional":
				grant.Bytes = 1
			case "payment_order":
				if err := db.Create(&model.PaymentOrder{ID: "paid", UserID: "one", ProductKind: "storage", MerchantOrderNo: "merchant", IdempotencyKey: "key"}).Error; err != nil {
					t.Fatal(err)
				}
			case "unknown_grant":
				grant.Source = "payment"
			case "missing_user":
				grant.UserID = "missing"
			}
			if err := db.Create(&grant).Error; err != nil {
				t.Fatal(err)
			}
			if err := migrateStorageGrantsToQuota(db); err == nil {
				t.Fatal("unsafe migration accepted")
			}
			var count int64
			if err := db.Model(&model.UserResourceQuota{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("quota persisted after failure", count, err)
			}
			if err := db.Model(&model.AdminAuditEvent{}).Where("action = ?", "storage_grants.migrate_to_quota").Count(&count).Error; err != nil || count != 0 {
				t.Fatal("audit persisted after failure", count, err)
			}
		})
	}
}
