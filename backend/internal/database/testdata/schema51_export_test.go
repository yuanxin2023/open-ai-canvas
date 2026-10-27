package database

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

type fixtureTable struct {
	Name    string
	Columns []string
	Rows    json.RawMessage
}

func TestExportOriginalSchema51(t *testing.T) {
	if CurrentSchemaVersion != 51 {
		t.Fatal("export must use the pinned original code")
	}
	root := os.Getenv("CANVAS_FIXTURE_OUTPUT")
	if root != "/results" {
		t.Fatal("fixture output must be the isolated test mount")
	}
	for _, driver := range []string{"sqlite", "postgres"} {
		for _, lineage := range []string{"local", "upstream"} {
			name := driver + "_" + lineage + "_51"
			dsn := filepath.Join(root, name+".db") + "?_foreign_keys=on"
			if driver == "postgres" {
				baseDSN := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
				if !strings.Contains(baseDSN, "@codex-canvas-merge-v1692-postgres:5432/canvas_merge_fixture") {
					t.Fatal("isolated fixture PostgreSQL required")
				}
				base, err := Open(Config{Driver: driver, DSN: baseDSN})
				if err != nil {
					t.Fatal(err)
				}
				if err := base.Exec("CREATE DATABASE " + name).Error; err != nil {
					t.Fatal(err)
				}
				sqlDB, _ := base.DB()
				_ = sqlDB.Close()
				dsn = strings.Replace(baseDSN, "/canvas_merge_fixture?", "/"+name+"?", 1)
			}
			db, err := Open(Config{Driver: driver, DSN: dsn})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&schemaMigration{}); err != nil {
				t.Fatal(err)
			}
			plan := schemaMigrations
			if lineage == "upstream" {
				plan = upstreamFirstMigrationPlan()
			}
			for _, item := range plan {
				if err := db.Transaction(func(tx *gorm.DB) error {
					if err := item.apply(tx); err != nil {
						return err
					}
					return tx.Create(&schemaMigration{Version: item.version, Name: item.name, Checksum: item.checksum, AppliedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}).Error
				}); err != nil {
					t.Fatalf("%s migration %d: %v", name, item.version, err)
				}
			}
			if err := RequireSchemaVersion(db); err != nil {
				t.Fatal(err)
			}
			seeds := map[string][]map[string]any{
				"users":                     {{"id": "owner", "username": "localowner", "email": "owner@example.invalid", "profile_name": "Preserved profile", "admin_remark": "Keep private remark", "role": "user", "status": "active"}, {"id": "scoped", "username": "moduleadmin", "email": "scope@example.invalid", "role": "admin", "admin_level": "scoped", "status": "active"}},
				"admin_permission_grants":   {{"user_id": "scoped", "permission": "admin.commerce.redeem_codes", "granted_by_user_id": "owner"}},
				"user_login_events":         {{"id": "login", "user_id": "owner", "login_method": "password", "browser": "fixture-browser", "os": "fixture-os"}},
				"user_username_changes":     {{"id": "rename", "user_id": "owner", "old_username": "oldname", "new_username": "localowner", "counts_toward_limit": true}},
				"user_resource_quotas":      {{"user_id": "owner", "overrides_json": "{\"uploadBytes\":123456}", "updated_by": "scoped"}},
				"credit_accounts":           {{"user_id": "owner", "available_microcredits": 123456789, "reserved_microcredits": 7654321, "version": 9}},
				"credit_ledger_entries":     {{"id": "ledger", "user_id": "owner", "type": "redeem", "amount_microcredits": 10000, "available_after_microcredits": 123456789, "reserved_after_microcredits": 7654321, "reference_key": "fixture-ledger"}},
				"billing_orders":            {{"id": "bill", "user_id": "owner", "idempotency_key": "bill-key", "amount_microcredits": 50000, "reserved_amount_microcredits": 50000, "actual_amount_microcredits": 30000, "refunded_amount_microcredits": 20000, "status": "refunded"}},
				"topup_products":            {{"id": "product", "name": "Local offer", "amount_fen": 890, "credits_microcredits": 1234567, "ribbon_text": "Keep promotion", "accent_color": "#112233", "enabled": true}},
				"payment_orders":            {{"id": "payment", "user_id": "owner", "idempotency_key": "pay-key", "merchant_order_no": "fixture-payment", "product_id": "product", "amount_fen": 890, "credits_microcredits": 1234567, "status": "closed", "closed_at": time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)}},
				"redeem_batches":            {{"id": "batch", "amount_microcredits": 10000, "count": 1, "funding_source": "module_admin", "created_by": "scoped", "codes_cipher": "fixture-cipher-placeholder"}},
				"redeem_codes":              {{"id": "code", "batch_id": "batch", "code_hash": "fixture-hash-placeholder", "code_suffix": "ABCD", "amount_microcredits": 10000, "status": "redeemed", "redeemed_by": "owner"}},
				"referral_profiles":         {{"user_id": "scoped", "code": "ABC234"}, {"user_id": "owner", "code": "DEF567", "inviter_id": "scoped"}},
				"referral_rewards":          {{"id": "reward", "payment_order_id": "payment", "inviter_id": "scoped", "invitee_id": "owner", "rate_bps": 1250, "reward_microcredits": 10000, "status": "approved"}},
				"resources":                 {{"id": "resource", "user_id": "owner", "kind": "image", "status": "ready", "mime_type": "image/png", "size": 1234, "object_key": "fixture-resource"}},
				"user_prompts":              {{"id": "personal", "user_id": "owner", "title": "Personal prompt", "prompt": "Do not overwrite this text", "cover_resource_id": "resource", "cover_url": "/api/resources/resource/file", "mode": "image"}},
				"canvas_projects":           {{"id": "canvas", "user_id": "owner", "title": "Original canvas", "payload_json": "{\"resourceId\":\"resource\"}", "revision": 3}},
				"canvas_snapshots":          {{"id": "snapshot", "canvas_id": "canvas", "user_id": "owner", "revision": 3, "payload_json": "{\"resourceId\":\"resource\"}", "created_at": time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}},
				"canvas_snapshot_resources": {{"snapshot_id": "snapshot", "resource_id": "resource"}},
			}
			// Respect the resource foreign key, independent of map iteration order.
			tables := []string{"users", "admin_permission_grants", "user_login_events", "user_username_changes", "user_resource_quotas", "credit_accounts", "credit_ledger_entries", "billing_orders", "topup_products", "payment_orders", "redeem_batches", "redeem_codes", "referral_profiles", "referral_rewards", "resources", "user_prompts", "canvas_projects", "canvas_snapshots", "canvas_snapshot_resources"}
			for _, table := range tables {
				for _, row := range seeds[table] {
					if err := db.Table(table).Create(row).Error; err != nil {
						t.Fatalf("seed %s/%s: %v", name, table, err)
					}
				}
			}
			tables = append(tables, "schema_migrations")
			var manifest []fixtureTable
			for _, table := range tables {
				columns, err := db.Migrator().ColumnTypes(table)
				if err != nil {
					t.Fatal(err)
				}
				var names []string
				for _, column := range columns {
					names = append(names, column.Name())
				}
				sort.Strings(names)
				var rows []map[string]any
				if err := db.Table(table).Select(names).Order(strings.Join(names, ",")).Find(&rows).Error; err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(rows)
				if err != nil {
					t.Fatal(err)
				}
				manifest = append(manifest, fixtureTable{Name: table, Columns: names, Rows: encoded})
			}
			encoded, err := json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, name+".json"), encoded, 0600); err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			if err := sqlDB.Close(); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: original Schema 51 with %d protected table snapshots", name, len(manifest))
		}
	}
}
