package database

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func migrationNamed(t *testing.T, name string) migration {
	t.Helper()
	for _, item := range schemaMigrations {
		if item.name == name {
			return item
		}
	}
	t.Fatalf("migration %q is absent", name)
	return migration{}
}

func isolatedMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	config := Config{Driver: "sqlite", DSN: fmt.Sprintf("file:%s?mode=memory&cache=shared&_foreign_keys=on", t.Name())}
	if os.Getenv("CANVAS_TEST_MIGRATION_DRIVER") == "postgres" {
		dsn := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Fatal("PostgreSQL migration validation requires CANVAS_TEST_POSTGRES_DSN")
		}
		base, err := Open(Config{Driver: "postgres", DSN: dsn})
		if err != nil {
			t.Fatal(err)
		}
		schema := fmt.Sprintf("merged_migration_%d", time.Now().UnixNano())
		if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
				t.Error(err)
			}
			sqlDB, _ := base.DB()
			_ = sqlDB.Close()
		})
		fixtureDSN, err := postgresDSNWithSearchPath(dsn, schema)
		if err != nil {
			t.Fatal(err)
		}
		config = Config{Driver: "postgres", DSN: fixtureDSN}
	}
	db, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestLedgerPreflightStopsBeforeApplyingBusinessMigrations(t *testing.T) {
	for _, scenario := range []string{"checksum", "name", "hole", "future", "legacy_topup", "legacy_workbench"} {
		t.Run(scenario, func(t *testing.T) {
			db := isolatedMigrationDB(t)
			if err := db.AutoMigrate(&schemaMigration{}); err != nil {
				t.Fatal(err)
			}
			item := schemaMigrations[0]
			record := schemaMigration{Version: item.version, Name: item.name, Checksum: item.checksum, AppliedAt: time.Now().UTC()}
			switch scenario {
			case "checksum":
				record.Checksum = "unknown"
			case "name":
				record.Name = "unrecognized"
			case "hole":
				item = schemaMigrations[1]
				record.Version, record.Name, record.Checksum = item.version, item.name, item.checksum
			case "future":
				record.Version = CurrentSchemaVersion + 1
			case "legacy_topup":
				record.Version, record.Name, record.Checksum = 16, "topup_product_benefits", legacyTopupProductBenefitsChecksum
			case "legacy_workbench":
				record.Version, record.Name, record.Checksum = 51, workbenchToolsMigrationName, legacyWorkbenchToolsMigrationChecksum
			}
			if err := db.Create(&record).Error; err != nil {
				t.Fatal(err)
			}
			// Compare persisted values; PostgreSQL stores timestamps at microsecond
			// precision rather than the nanoseconds in the in-memory insert value.
			if err := db.First(&record, "version = ?", record.Version).Error; err != nil {
				t.Fatal(err)
			}
			if err := MigrateSchema(db); err == nil {
				t.Fatal("invalid ledger accepted")
			}
			if _, err := ReadSchemaStatus(db); err == nil {
				t.Fatal("verification accepted invalid ledger")
			}
			if db.Migrator().HasTable(&model.User{}) || db.Migrator().HasTable(&model.Resource{}) {
				t.Fatal("business migration ran before preflight")
			}
			var after []schemaMigration
			if err := db.Find(&after).Error; err != nil || len(after) != 1 || !reflect.DeepEqual(after[0], record) {
				t.Fatalf("ledger changed: %#v %v", after, err)
			}
		})
	}
}

func TestPaymentConfirmationMigrationPreservesHistoricalFinancialSnapshot(t *testing.T) {
	db := isolatedMigrationDB(t)
	for _, sql := range []string{
		`CREATE TABLE payment_orders (id TEXT PRIMARY KEY, user_id TEXT, idempotency_key TEXT, merchant_order_no TEXT, amount_fen INTEGER, credits_microcredits INTEGER, status TEXT, closed_at TIMESTAMP, updated_at TIMESTAMP)`,
		`INSERT INTO payment_orders VALUES ('old','owner','old-key','old-no',890,123456789,'closed','2026-09-01 00:00:00','2026-09-01 01:00:00')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := db.Transaction(migratePaymentOrderConfirmation); err != nil {
			t.Fatal(err)
		}
	}
	var order model.PaymentOrder
	if err := db.First(&order, "id = ?", "old").Error; err != nil {
		t.Fatal(err)
	}
	if order.AmountFen != 890 || order.CreditsMicrocredits != 123456789 || order.Status != model.PaymentOrderClosed || order.ClosedAt == nil || order.ProviderClosedAt != nil || order.InventoryOversold || order.StockReserved || order.SaleStrategySnapshot != model.TopupSaleStrategyUnlimited || order.UpdatedAt.Hour() != 1 {
		t.Fatalf("historical snapshot changed: %#v", order)
	}
	if err := db.Model(&order).Updates(map[string]any{"sale_strategy_snapshot": "", "stock_reserved": true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(migratePaymentOrderConfirmation); err == nil || !strings.Contains(err.Error(), "缺少售卖策略快照") {
		t.Fatalf("ambiguous historical inventory accepted: %v", err)
	}
	if err := db.First(&order, "id = ?", "old").Error; err != nil || order.SaleStrategySnapshot != "" || !order.StockReserved {
		t.Fatalf("ambiguous order was guessed: %#v %v", order, err)
	}
}

func TestRefundRecoveryMigrationDoesNotRecalculateHistoricalBill(t *testing.T) {
	db := isolatedMigrationDB(t)
	for _, sql := range []string{
		`CREATE TABLE billing_orders (id TEXT PRIMARY KEY, user_id TEXT, idempotency_key TEXT, amount_microcredits INTEGER, actual_amount_microcredits INTEGER, refunded_amount_microcredits INTEGER, status TEXT)`,
		`INSERT INTO billing_orders VALUES ('old','owner','old-key',50000,30000,20000,'refunded')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := db.Transaction(migrationNamed(t, "billing_refund_recovery_state").apply); err != nil {
			t.Fatal(err)
		}
	}
	var order model.BillingOrder
	if err := db.First(&order, "id = ?", "old").Error; err != nil || order.AmountMicrocredits != 50000 || order.ActualAmountMicrocredits != 30000 || order.RefundedAmountMicrocredits != 20000 || string(order.Status) != "refunded" || order.RefundRecoveryPending {
		t.Fatalf("historical bill changed: %#v %v", order, err)
	}
}

func TestFeedbackMigrationPreservesArchivedAttachmentsAndProtectsResources(t *testing.T) {
	db := isolatedMigrationDB(t)
	if err := db.AutoMigrate(&model.Resource{}, &model.UserFeedback{}); err != nil {
		t.Fatal(err)
	}
	resource := model.Resource{ID: "resource", UserID: "owner", Kind: "image", Status: model.ResourceStatusReady, MimeType: "image/png", Size: 123}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	updated := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	row := model.UserFeedback{ID: "archived", UserID: "owner", Version: 7, Archived: true, UpdatedAt: updated, Attachments: `[{"url":"https://old.invalid/api/resources/resource/file?old=signature","name":"original.png","legacyExtension":{"epoch":9007199254740993}}]`}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.Transaction(migrateFeedbackResourceReferences); err != nil {
			t.Fatal(err)
		}
	}
	var stored model.UserFeedback
	if err := db.First(&stored, "id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Version != 7 || !stored.Archived || !stored.UpdatedAt.Equal(updated) || !strings.Contains(stored.Attachments, "9007199254740993") || !strings.Contains(stored.Attachments, "original.png") {
		t.Fatalf("historical feedback changed: %#v", stored)
	}
	var attachments []model.FeedbackAttachment
	if err := json.Unmarshal([]byte(stored.Attachments), &attachments); err != nil || len(attachments) != 1 || attachments[0].ResourceID != resource.ID || attachments[0].URL != "/api/feedbacks/archived/attachments/resource" || attachments[0].Size != resource.Size {
		t.Fatalf("unsafe attachment projection: %#v %v", attachments, err)
	}
	var count int64
	if err := db.Model(&model.FeedbackResourceReference{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("reference count = %d, %v", count, err)
	}
	if err := db.Delete(&resource).Error; err == nil {
		t.Fatal("referenced resource was deleted")
	}
	if err := db.Delete(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.FeedbackResourceReference{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("feedback reference did not cascade: %d %v", count, err)
	}
	if err := db.First(&model.Resource{}, "id = ?", resource.ID).Error; err != nil {
		t.Fatal("feedback deletion deleted resource", err)
	}
}

func TestFeedbackMigrationRollsBackWholeBatchOnInvalidResource(t *testing.T) {
	for _, scenario := range []string{"missing", "foreign", "conflicting_identity", "empty_feedback_id", "empty_owner"} {
		t.Run(scenario, func(t *testing.T) {
			db := isolatedMigrationDB(t)
			if err := db.AutoMigrate(&model.Resource{}, &model.UserFeedback{}); err != nil {
				t.Fatal(err)
			}
			for _, resource := range []model.Resource{
				{ID: "owned", UserID: "owner", Kind: "image", Status: model.ResourceStatusReady, MimeType: "image/png", Size: 100},
				{ID: "foreign", UserID: "other", Kind: "image", Status: model.ResourceStatusReady, MimeType: "image/png", Size: 100},
			} {
				if err := db.Create(&resource).Error; err != nil {
					t.Fatal(err)
				}
			}
			bad := `[{"id":"missing"}]`
			if scenario == "foreign" {
				bad = `[{"id":"foreign"}]`
			} else if scenario == "conflicting_identity" {
				bad = `[{"id":"owned","resourceId":"foreign"}]`
			}
			rows := []model.UserFeedback{
				{ID: "a_valid", UserID: "owner", Attachments: `[{"id":"owned","name":"preserved"}]`},
				{ID: "z_invalid", UserID: "owner", Attachments: bad},
			}
			if scenario == "empty_feedback_id" {
				rows[1].ID = ""
			} else if scenario == "empty_owner" {
				rows[1].UserID = ""
			}
			if err := db.Create(&rows).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Transaction(migrateFeedbackResourceReferences); err == nil {
				t.Fatal("invalid historical attachment accepted")
			}
			var after []model.UserFeedback
			if err := db.Order("id").Find(&after).Error; err != nil || len(after) != len(rows) {
				t.Fatalf("feedback rows lost: %v", err)
			}
			original := map[string]model.UserFeedback{}
			for _, row := range rows {
				original[row.ID] = row
			}
			for _, row := range after {
				if row.Attachments != original[row.ID].Attachments || row.Version != original[row.ID].Version {
					t.Fatal("failed conversion left partial writes")
				}
			}
			if db.Migrator().HasTable(&model.FeedbackResourceReference{}) {
				t.Fatal("failed conversion left reference table")
			}
		})
	}
}

func TestNewPersonalStoresDoNotImportHistoricalData(t *testing.T) {
	db := isolatedMigrationDB(t)
	for range 2 {
		for _, name := range []string{"workbench_submission_receipts", "user_inspiration_library_state"} {
			if err := db.Transaction(migrationNamed(t, name).apply); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, entity := range []any{&model.WorkbenchSubmission{}, &model.UserInspirationLibrary{}} {
		var count int64
		if err := db.Model(entity).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("new store auto-imported data: %T %d %v", entity, count, err)
		}
	}
	row := model.WorkbenchSubmission{UserID: "owner", RequestKey: "key", RequestHash: "hash", Kind: "image", TaskID: "task", QuoteJSON: "{}"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&row).Error; err == nil {
		t.Fatal("duplicate account request key accepted")
	}
	row.UserID = "other"
	if err := db.Create(&row).Error; err != nil {
		t.Fatal("different accounts must have isolated keys", err)
	}
}
