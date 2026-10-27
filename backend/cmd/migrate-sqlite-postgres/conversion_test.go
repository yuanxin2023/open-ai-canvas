package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func conversionSource(t *testing.T, lineage string) string {
	t.Helper()
	root := os.Getenv("CANVAS_TEST_MIGRATION_FIXTURES")
	if root == "" {
		t.Skip("CANVAS_TEST_MIGRATION_FIXTURES is not configured")
	}
	path := filepath.Join(t.TempDir(), "source # & copy.db")
	raw, err := os.ReadFile(filepath.Join(root, "sqlite_"+lineage+"_51.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	db := writableSource(t, path)
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`UPDATE inspirations SET title = 'Keep operator content' WHERE created_by = 'system'`,
		`UPDATE skill_curation_roots SET name = 'Keep edited root', enabled = false WHERE id = 'drama'`,
		`INSERT INTO tasks (id, user_id, status, operation, created_at_epoch) VALUES ('historic-task','owner','failed','image_generate',0)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	stamp := time.Date(2026, 10, 2, 3, 4, 5, 123456000, time.UTC)
	for _, entity := range []any{
		&model.Tool{ID: 42, Label: "Keep numeric ID", Desc: "Keep quoted column", OwnerID: "owner"},
		&model.UserAvatar{UserID: "owner", Data: []byte{0, 1, 2, 255}},
		&model.SMSChannel{ID: "deleted-channel", Name: "Keep deleted channel", DeletedAt: gorm.DeletedAt{Time: stamp, Valid: true}},
		&model.ChannelModel{ID: "MODEL_000123", ChannelID: "CHANNEL_000123", ModelKey: "keep", Tags: []model.ChannelModelTag{{Text: "tag", Color: "blue"}}, DeletedAt: gorm.DeletedAt{Time: stamp, Valid: true}},
		&model.CloudAgentExecution{ID: "run", UserID: "owner", Status: "completed", StateJSON: "{}"},
		&model.CloudAgentEventRecord{RunID: "run", Sequence: 1, UserID: "owner", EventJSON: "{}"},
		&model.CloudAgentMessageRecord{RunID: "run", Kind: "message", Sequence: 1, UserID: "owner", MessageJSON: "{}"},
		&model.UserFeedback{ID: "feedback", UserID: "owner", Version: 7, Archived: true, Attachments: `[{"id":"resource","resourceId":"resource","url":"/api/feedbacks/feedback/attachments/resource"}]`},
		&model.FeedbackResourceReference{FeedbackID: "feedback", ResourceID: "resource", UserID: "owner"},
		&model.WorkbenchSubmission{UserID: "owner", RequestKey: "fixture-request-01", RequestHash: "digest", Kind: "image", TaskID: "historic-task", QuoteJSON: `{"amountMicrocredits":12345}`},
		&model.UserInspirationLibrary{UserID: "owner", Revision: 9, StateJSON: `{"favorites":["fixture"]}`},
	} {
		if err := db.Create(entity).Error; err != nil {
			t.Fatalf("seed %T: %v", entity, err)
		}
	}
	// More than one batch, with mixed-case keys and explicit NULL timestamps.
	for i := 0; i < 105; i++ {
		if err := db.Table("user_prompts").Create(map[string]any{"id": fmt.Sprintf("p_%03d", i), "user_id": "owner", "title": "Keep batch", "prompt": "Keep text"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"Case-A", "case-a"} {
		if err := db.Table("user_prompts").Create(map[string]any{"id": id, "user_id": "owner", "title": "Keep ordering"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	sqlDB, _ := db.DB()
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func writableSource(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := database.Open(database.Config{Driver: "sqlite", DSN: path + "?_foreign_keys=on&_journal_mode=DELETE"})
	if err != nil {
		t.Fatal(err)
	}
	return db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}

func fixtureTarget(t *testing.T) (string, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("CANVAS_TEST_POSTGRES_DSN is not configured")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Host != "codex-canvas-merge-v1692-postgres:5432" || parsed.Path != "/canvas_merge_fixture" {
		t.Fatal("conversion tests require the isolated merge fixture server")
	}
	base, err := database.Open(database.Config{Driver: "postgres", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("conversion_fixture_%d", time.Now().UnixNano())
	if err := base.Exec("CREATE DATABASE " + name).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := base.Exec("DROP DATABASE " + name).Error; err != nil {
			t.Error(err)
		}
		sqlDB, _ := base.DB()
		_ = sqlDB.Close()
	})
	parsed.Path = "/" + name
	targetDSN := parsed.String()
	target, err := database.Open(database.Config{Driver: "postgres", DSN: targetDSN})
	if err != nil {
		t.Fatal(err)
	}
	target = target.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	t.Cleanup(func() { sqlDB, _ := target.DB(); _ = sqlDB.Close() })
	return targetDSN, target
}

func assertEmptyTarget(t *testing.T, target *gorm.DB) {
	t.Helper()
	tables, err := targetTableNames(target)
	if err != nil || len(tables) != 0 {
		t.Fatalf("failed conversion left target objects: %v %v", tables, err)
	}
}

func TestConversionPreservesBothHistoriesAndAllPersistedValues(t *testing.T) {
	for _, lineage := range []string{"local", "upstream"} {
		t.Run(lineage, func(t *testing.T) {
			sourcePath := conversionSource(t, lineage)
			before, err := fileDigest(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			dsn, target := fixtureTarget(t)
			result, err := runConversion(sourcePath, dsn)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Copied || result.Tables != len(migrations()) || result.Rows < 200 {
				t.Fatalf("incomplete copy: %#v", result)
			}
			var v24 database.MigrationLedgerRecord
			if err := target.First(&v24, "version = ?", 24).Error; err != nil {
				t.Fatal(err)
			}
			want := "topup_product_benefits"
			if lineage == "upstream" {
				want = "channel_model_label"
			}
			if v24.Name != want || !v24.AppliedAt.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
				t.Fatalf("history rewritten: %#v", v24)
			}
			var count int64
			if err := target.Model(&model.Inspiration{}).Where("title = ?", "Keep operator content").Count(&count).Error; err != nil || count != 22 {
				t.Fatalf("custom seed overwritten: %d %v", count, err)
			}
			if err := target.Unscoped().Model(&model.SMSChannel{}).Where("id = ? AND deleted_at IS NOT NULL", "deleted-channel").Count(&count).Error; err != nil || count != 1 {
				t.Fatal("soft delete lost", err)
			}
			if err := target.Table("tasks").Where("id = ? AND created_at IS NULL AND created_at_epoch = 0", "historic-task").Count(&count).Error; err != nil || count != 1 {
				t.Fatal("NULL or epoch rewritten by hooks", err)
			}
			var blob model.UserAvatar
			if err := target.First(&blob, "user_id = ?", "owner").Error; err != nil || string(blob.Data) != string([]byte{0, 1, 2, 255}) {
				t.Fatal("binary avatar changed", err)
			}
			var fkCount int64
			if err := target.Model(&model.FeedbackResourceReference{}).Count(&fkCount).Error; err != nil || fkCount != 1 {
				t.Fatal("feedback reference lost", err)
			}
			// Rechecking must succeed with a server-enforced read-only transaction.
			parsed, _ := url.Parse(dsn)
			query := parsed.Query()
			query.Set("default_transaction_read_only", "on")
			parsed.RawQuery = query.Encode()
			result, err = runConversion(sourcePath, parsed.String())
			if err != nil || result.Copied {
				t.Fatalf("read-only repeat failed: %#v %v", result, err)
			}
			tool := model.Tool{Label: "New after copy"}
			if err := target.Create(&tool).Error; err != nil || tool.ID <= 42 {
				t.Fatalf("new serial sequence not advanced: %d %v", tool.ID, err)
			}
			after, err := fileDigest(sourcePath)
			if err != nil || after != before {
				t.Fatal("source bytes changed", err)
			}
		})
	}
}

func TestConversionRejectsTargetDriftWithoutRepair(t *testing.T) {
	sourcePath := conversionSource(t, "local")
	dsn, target := fixtureTarget(t)
	if _, err := runConversion(sourcePath, dsn); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct{ name, sql string }{
		{"data", `UPDATE user_prompts SET title='Target edit' WHERE id='personal'`},
		{"ledger", `UPDATE schema_migrations SET checksum='unknown' WHERE version=83`},
		{"applied_time", `UPDATE schema_migrations SET applied_at=applied_at+interval '1 second' WHERE version=83`},
		{"extra_table", `CREATE TABLE must_not_remove (id text)`},
		{"extra_view", `CREATE VIEW must_not_remove AS SELECT id FROM users`},
		{"extra_sequence", `CREATE SEQUENCE must_not_remove`},
		{"missing_table", `DROP TABLE user_resource_quotas`},
		{"column_type", `ALTER TABLE users ALTER COLUMN admin_remark TYPE text`},
		{"column_default", `ALTER TABLE user_feedbacks ALTER COLUMN version SET DEFAULT 999`},
		{"column_nullability", `ALTER TABLE user_feedbacks ALTER COLUMN version DROP NOT NULL`},
		{"missing_index", `DROP INDEX idx_users_username_ci`},
		{"wrong_index_unique", `DROP INDEX idx_users_username_ci; CREATE INDEX idx_users_username_ci ON users(lower(username))`},
		{"wrong_index_order", `DROP INDEX idx_user_login_events_user_created; CREATE INDEX idx_user_login_events_user_created ON user_login_events(user_id,created_at ASC)`},
		{"wrong_predicate", `DROP INDEX idx_logical_models_code; CREATE UNIQUE INDEX idx_logical_models_code ON logical_models(code) WHERE archived_at IS NOT NULL`},
		{"missing_fk", `ALTER TABLE feedback_resource_references DROP CONSTRAINT fk_feedback_resource_references_resource`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			tx := target.Begin()
			if tx.Error != nil {
				t.Fatal(tx.Error)
			}
			defer tx.Rollback()
			if err := tx.Exec(scenario.sql).Error; err != nil {
				t.Fatal(err)
			}
			uri, err := readOnlySnapshotDSN(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			source, err := database.Open(database.Config{Driver: "sqlite", DSN: uri})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := source.DB()
			defer sqlDB.Close()
			if _, err := convertSnapshot(source, tx, nil); err == nil {
				t.Fatal("drift accepted or silently repaired")
			}
			if scenario.name == "data" {
				var title string
				if err := tx.Table("user_prompts").Select("title").Where("id = ?", "personal").Scan(&title).Error; err != nil || title != "Target edit" {
					t.Fatal("target edit overwritten", err)
				}
			}
			if scenario.name == "extra_table" && !tx.Migrator().HasTable("must_not_remove") {
				t.Fatal("extra table removed")
			}
		})
	}
}

func TestConversionRollsBackCopiedDataAndDDL(t *testing.T) {
	sourcePath := conversionSource(t, "local")
	for _, scenario := range []string{"final_file_check", "late_length_error"} {
		t.Run(scenario, func(t *testing.T) {
			_, target := fixtureTarget(t)
			if scenario == "late_length_error" {
				db := writableSource(t, sourcePath)
				if err := db.Table("user_prompts").Where("id = ?", "personal").UpdateColumn("title", strings.Repeat("private-content-", 20)).Error; err != nil {
					t.Fatal(err)
				}
				sqlDB, _ := db.DB()
				_ = sqlDB.Close()
			}
			uri, err := readOnlySnapshotDSN(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			source, err := database.Open(database.Config{Driver: "sqlite", DSN: uri})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := source.DB()
			defer sqlDB.Close()
			var verify func() error
			if scenario == "final_file_check" {
				verify = func() error { return errors.New("fixture source changed") }
			}
			if _, err := convertSnapshot(source, target, verify); err == nil || strings.Contains(err.Error(), "private-content") {
				t.Fatalf("rollback or safe error failed: %v", err)
			}
			assertEmptyTarget(t, target)
		})
	}
}

func TestConversionRejectsOldAndInvalidSources(t *testing.T) {
	root := os.Getenv("CANVAS_TEST_MIGRATION_FIXTURES")
	if root == "" {
		t.Skip("CANVAS_TEST_MIGRATION_FIXTURES is not configured")
	}
	if _, err := runConversion(filepath.Join(root, "sqlite_local_51.db"), "postgres://must-not-connect.invalid/unreachable"); err == nil || !strings.Contains(err.Error(), "83") {
		t.Fatalf("old source was not rejected first: %v", err)
	}
	for _, scenario := range []string{"checksum", "extra_column", "foreign_key", "empty_source", "extra_view", "extra_trigger"} {
		t.Run(scenario, func(t *testing.T) {
			path := conversionSource(t, "local")
			db := writableSource(t, path)
			statement := `UPDATE schema_migrations SET checksum='unknown' WHERE version=83`
			switch scenario {
			case "extra_column":
				statement = `ALTER TABLE user_prompts ADD COLUMN unknown_private_data text`
			case "foreign_key":
				if err := db.Exec("PRAGMA foreign_keys=OFF").Error; err != nil {
					t.Fatal(err)
				}
				statement = `INSERT INTO feedback_resource_references (feedback_id,resource_id,user_id) VALUES ('feedback','missing','owner')`
			case "empty_source":
				statement = `DROP TABLE schema_migrations`
			case "extra_view":
				statement = `CREATE VIEW unknown_view AS SELECT id FROM users`
			case "extra_trigger":
				statement = `CREATE TRIGGER unknown_trigger AFTER UPDATE ON users BEGIN SELECT 1; END`
			}
			if err := db.Exec(statement).Error; err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			_ = sqlDB.Close()
			dsn, target := fixtureTarget(t)
			if _, err := runConversion(path, dsn); err == nil {
				t.Fatal("invalid source accepted")
			}
			assertEmptyTarget(t, target)
		})
	}
}

func TestConversionDoesNotRestoreDeletedDefaults(t *testing.T) {
	path := conversionSource(t, "local")
	source := writableSource(t, path)
	if err := source.Exec("DELETE FROM inspirations").Error; err != nil {
		t.Fatal(err)
	}
	if err := source.Exec("DELETE FROM skill_curation_roots WHERE id = ?", "drama").Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := source.DB()
	_ = sqlDB.Close()
	dsn, target := fixtureTarget(t)
	if _, err := runConversion(path, dsn); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := target.Model(&model.Inspiration{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("deleted content restored: %d %v", count, err)
	}
	if err := target.Model(&model.SkillCurationRoot{}).Count(&count).Error; err != nil || count != 4 {
		t.Fatalf("deleted root restored: %d %v", count, err)
	}
}

func TestSnapshotDSNRejectsPendingWALAndEncodesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot # &.db")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	uri, err := readOnlySnapshotDSN(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Fragment != "" || parsed.Query().Get("mode") != "ro" || parsed.Query().Get("immutable") != "1" {
		t.Fatalf("unsafe source URI: %s %v", uri, err)
	}
	if err := os.WriteFile(path+"-wal", []byte("pending"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readOnlySnapshotDSN(path); err == nil {
		t.Fatal("pending WAL was ignored")
	}
}

func TestIndexContractNormalizationPreservesLiteralCaseAndOperators(t *testing.T) {
	if normalizedIndexSQL("lower((username)::text)") != normalizedIndexSQL("lower(username)") {
		t.Fatal("implicit cast not handled")
	}
	if normalizedIndexSQL("operation = 'cloud_agent'") == normalizedIndexSQL("operation = 'CLOUD_AGENT'") {
		t.Fatal("literal case lost")
	}
	if normalizedIndexSQL("archived_at IS NULL") == normalizedIndexSQL("archived_at IS NOT NULL") {
		t.Fatal("predicate changed")
	}
}

func TestConversionExecutable(t *testing.T) {
	path := conversionSource(t, "local")
	dsn, _ := fixtureTarget(t)
	binary := filepath.Join(t.TempDir(), "migrate-sqlite-postgres")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build executable: %v %s", err, output)
	}
	before, err := fileDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	run := func(sourcePath, targetDSN string) ([]byte, error) {
		command := exec.Command(binary)
		command.Env = append(os.Environ(), "SQLITE_SOURCE_PATH="+sourcePath, "DATABASE_URL="+targetDSN)
		return command.CombinedOutput()
	}
	old := filepath.Join(os.Getenv("CANVAS_TEST_MIGRATION_FIXTURES"), "sqlite_local_51.db")
	if output, err := run(old, dsn); err == nil || !strings.Contains(string(output), "83") {
		t.Fatalf("old source command accepted: %v %s", err, output)
	}
	if output, err := run(path, dsn); err != nil || !strings.Contains(string(output), "转库事务已提交") {
		t.Fatalf("copy command: %v %s", err, output)
	}
	if output, err := run(path, dsn); err != nil || !strings.Contains(string(output), "未重复写入") {
		t.Fatalf("repeat command: %v %s", err, output)
	}
	after, err := fileDigest(path)
	if err != nil || after != before {
		t.Fatal("executable changed the source", err)
	}
}
