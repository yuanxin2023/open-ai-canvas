package database

import (
	"errors"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/referralcode"

	"gorm.io/gorm"
)

func TestCurrentSchemaVersionMatchesMigrationPlan(t *testing.T) {
	for name, plan := range map[string][]migration{"local": schemaMigrations, "upstream": upstreamFirstMigrationPlan()} {
		if len(plan) == 0 {
			t.Fatalf("%s migration plan is empty", name)
		}
		for index, item := range plan {
			if item.version != int64(index+1) {
				t.Fatalf("%s migration %q has version %d at position %d", name, item.name, item.version, index+1)
			}
		}
		latest := plan[len(plan)-1]
		if CurrentSchemaVersion != latest.version || latest.name != "six_character_referral_codes" {
			t.Fatalf("%s latest migration = %d/%q, want %d/six_character_referral_codes", name, latest.version, latest.name, CurrentSchemaVersion)
		}
	}
}

func TestTopupProductAccentColorMigrationIsSharedTail(t *testing.T) {
	for name, plan := range map[string][]migration{"local": schemaMigrations, "upstream": upstreamFirstMigrationPlan()} {
		firstShared := plan[len(plan)-14]
		if firstShared.version != 37 || firstShared.name != "user_profiles" {
			t.Fatalf("%s shared migration tail starts at %d/%s, want 37/user_profiles", name, firstShared.version, firstShared.name)
		}
		item := plan[len(plan)-7]
		if item.version != 44 || item.name != "topup_product_accent_color" {
			t.Fatalf("%s migration 44 = %d/%s", name, item.version, item.name)
		}
	}
}

func TestSixCharacterReferralCodeMigrationPreservesReferralData(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:referral-code-migration?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ReferralProfile{}, &model.ReferralReward{}); err != nil {
		t.Fatal(err)
	}
	rate := int64(1_250)
	profiles := []model.ReferralProfile{
		{UserID: "inviter", Code: "INVITERCODE1", RateBPS: &rate},
		{UserID: "invitee", Code: "INVITEECODE1", InviterID: "inviter"},
		{UserID: "current", Code: "ABC234"},
	}
	if err := db.Create(&profiles).Error; err != nil {
		t.Fatal(err)
	}
	reward := model.ReferralReward{ID: "reward", PaymentOrderID: "order", InviterID: "inviter", InviteeID: "invitee", RewardMicrocredits: 100_000, Status: model.ReferralRewardApproved}
	if err := db.Create(&reward).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(migrateSixCharacterReferralCodes); err != nil {
		t.Fatal(err)
	}
	var migrated []model.ReferralProfile
	if err := db.Order("user_id").Find(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	if len(migrated) != 3 {
		t.Fatalf("profile count = %d", len(migrated))
	}
	seen := map[string]bool{}
	for _, profile := range migrated {
		if !referralcode.Valid(profile.Code) || seen[profile.Code] {
			t.Fatalf("invalid or duplicate migrated code %q", profile.Code)
		}
		seen[profile.Code] = true
		switch profile.UserID {
		case "inviter":
			if profile.Code == "INVITERCODE1" || profile.RateBPS == nil || *profile.RateBPS != rate {
				t.Fatalf("inviter profile changed unexpectedly: %#v", profile)
			}
		case "invitee":
			if profile.Code == "INVITEECODE1" || profile.InviterID != "inviter" {
				t.Fatalf("invitee profile changed unexpectedly: %#v", profile)
			}
		case "current":
			if profile.Code != "ABC234" {
				t.Fatalf("existing six-character code was changed: %q", profile.Code)
			}
		}
	}
	var count int64
	if err := db.Model(&model.ReferralProfile{}).Where("code IN ?", []string{"INVITERCODE1", "INVITEECODE1"}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("old codes still exist: count=%d err=%v", count, err)
	}
	var storedReward model.ReferralReward
	if err := db.First(&storedReward, "id = ?", reward.ID).Error; err != nil || storedReward.InviterID != reward.InviterID || storedReward.InviteeID != reward.InviteeID {
		t.Fatalf("reward changed: %#v err=%v", storedReward, err)
	}
}

func TestRedeemBatchLifecycleMigrationBackfillsTerminalBatches(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-redeem-lifecycle-v48?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE redeem_batches (id text PRIMARY KEY, amount_microcredits integer, count integer, note text, created_by text, funding_source text, codes_cipher text, expires_at datetime, created_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE redeem_codes (id text PRIMARY KEY, batch_id text, status text, updated_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO redeem_batches (id, count, funding_source, created_at) VALUES ('completed', 1, 'platform', '2026-01-01'), ('ongoing', 1, 'platform', '2026-01-01')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO redeem_codes (id, batch_id, status, updated_at) VALUES ('completed-code', 'completed', 'redeemed', '2026-02-01'), ('ongoing-code', 'ongoing', 'unused', '2026-02-01')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := schemaMigrations[47].apply(db); err != nil {
		t.Fatal(err)
	}
	var completed model.RedeemBatch
	if err := db.First(&completed, "id = ?", "completed").Error; err != nil {
		t.Fatal(err)
	}
	if completed.TerminalAt == nil {
		t.Fatal("completed batch terminal_at was not backfilled")
	}
	var ongoing model.RedeemBatch
	if err := db.First(&ongoing, "id = ?", "ongoing").Error; err != nil {
		t.Fatal(err)
	}
	if ongoing.TerminalAt != nil {
		t.Fatalf("ongoing batch terminal_at = %v", ongoing.TerminalAt)
	}
}

func TestPaymentPromotionImageDraftMigrationAddsTable(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-payment-promotion-v46?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := schemaMigrations[45].apply(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&model.PaymentPromotionImageDraft{}) {
		t.Fatal("migration v46 did not add payment promotion image drafts")
	}
}

func TestScopedRedeemFundingMigrationBackfillsPlatformSource(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-redeem-funding-v47?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE redeem_batches (id text PRIMARY KEY, amount_microcredits integer, count integer, note text, created_by text, codes_cipher text, expires_at datetime, created_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO redeem_batches (id, amount_microcredits, count, created_by) VALUES ('legacy', 1000000, 1, 'admin')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := schemaMigrations[46].apply(db); err != nil {
		t.Fatal(err)
	}
	var batch model.RedeemBatch
	if err := db.First(&batch, "id = ?", "legacy").Error; err != nil {
		t.Fatal(err)
	}
	if batch.FundingSource != model.RedeemBatchFundingPlatform {
		t.Fatalf("legacy funding source = %q", batch.FundingSource)
	}
	if !db.Migrator().HasColumn(&model.CreditLedgerEntry{}, "RedeemBatchID") {
		t.Fatal("migration v47 did not add redeem batch ledger reference")
	}
}

func TestTopupProductCardFieldsMigrationAddsColumns(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-topup-card-v43?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE topup_products (id text PRIMARY KEY, name text, amount_fen integer, credits_microcredits integer)").Error; err != nil {
		t.Fatal(err)
	}
	if err := schemaMigrations[42].apply(db); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"RibbonText", "BadgeText", "CompareAmountFen", "PriceCaption", "QuotaCaption", "QuotaDetail", "ActionText", "Featured"} {
		if !db.Migrator().HasColumn(&model.TopupProduct{}, column) {
			t.Fatalf("migration v43 did not add %s", column)
		}
	}
}

func TestTopupProductAccentColorMigrationAddsColumn(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-topup-accent-v44?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE topup_products (id text PRIMARY KEY, name text, amount_fen integer, credits_microcredits integer)").Error; err != nil {
		t.Fatal(err)
	}
	if err := schemaMigrations[43].apply(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(&model.TopupProduct{}, "AccentColor") {
		t.Fatal("migration v44 did not add AccentColor")
	}
}

func TestScopedAdminPermissionMigrationBackfillsExistingAdmins(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-admin-permissions-v45?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	admin := model.User{ID: "admin", Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	user := model.User{ID: "user", Username: "user", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&[]model.User{admin, user}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateScopedAdminPermissions(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&admin, "id = ?", admin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if admin.AdminLevel != model.AdminLevelFull {
		t.Fatalf("admin level = %q, want full", admin.AdminLevel)
	}
	if !db.Migrator().HasTable(&model.AdminPermissionGrant{}) {
		t.Fatal("admin_permission_grants table was not created")
	}
}

func TestMigrateUserAdminRemarksAddsPrivateRemarkColumn(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-user-admin-remarks-v42?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE users (id text PRIMARY KEY, username text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateUserAdminRemarks(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(&model.User{}, "AdminRemark") {
		t.Fatal("migration v42 did not add users.admin_remark")
	}
}

func TestMigrateSchemaSupportsLocalAndUpstreamPost23Lineages(t *testing.T) {
	for _, scenario := range []struct {
		name             string
		plan             []migration
		appliedThrough   int64
		expectedV24Name  string
		expectedTailName string
	}{
		{name: "local", plan: schemaMigrations, appliedThrough: 27, expectedV24Name: "topup_product_benefits", expectedTailName: "six_character_referral_codes"},
		{name: "upstream", plan: upstreamFirstMigrationPlan(), appliedThrough: 32, expectedV24Name: "channel_model_label", expectedTailName: "six_character_referral_codes"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, err := Open(Config{Driver: "sqlite", DSN: "file:" + t.Name() + "?mode=memory&cache=shared"})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&schemaMigration{}); err != nil {
				t.Fatal(err)
			}
			for _, item := range scenario.plan {
				if item.version > scenario.appliedThrough {
					break
				}
				if err := item.apply(db); err != nil {
					t.Fatalf("apply migration %d: %v", item.version, err)
				}
				if err := db.Create(&schemaMigration{Version: item.version, Name: item.name, Checksum: item.checksum, AppliedAt: time.Now().UTC()}).Error; err != nil {
					t.Fatalf("record migration %d: %v", item.version, err)
				}
			}
			if err := MigrateSchema(db); err != nil {
				t.Fatal(err)
			}
			status, err := ReadSchemaStatus(db)
			if err != nil || !status.Ready || status.Current != CurrentSchemaVersion {
				t.Fatalf("status = %+v, err = %v", status, err)
			}
			for version, expectedName := range map[int64]string{24: scenario.expectedV24Name, CurrentSchemaVersion: scenario.expectedTailName} {
				var record schemaMigration
				if err := db.First(&record, "version = ?", version).Error; err != nil {
					t.Fatal(err)
				}
				if record.Name != expectedName {
					t.Fatalf("migration %d name = %q, want %q", version, record.Name, expectedName)
				}
			}
		})
	}
}

func TestMigrateSchemaRecordsAndValidatesVersion(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-version?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	status, err := ReadSchemaStatus(db)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("unexpected schema status: %#v", status)
	}
	if !db.Migrator().HasIndex(&schemaMigration{}, "idx_schema_migrations_applied_at") {
		t.Fatal("schema migration v2 did not create the applied_at index")
	}
	if !db.Migrator().HasIndex(&model.ProjectAssetCandidate{}, "idx_project_asset_candidates_pending_identity") {
		t.Fatal("schema migration v3 did not create candidate identity index")
	}
	if !db.Migrator().HasTable(&model.AgentProfile{}) || !db.Migrator().HasIndex(&model.AgentProfile{}, "idx_agent_profiles_scope") {
		t.Fatal("schema migration v15 did not create scoped Agent profiles")
	}
	if !db.Migrator().HasTable(&model.AgentLesson{}) || !db.Migrator().HasIndex(&model.AgentLesson{}, "idx_agent_lessons_status") {
		t.Fatal("schema migration v16 did not create Agent lessons")
	}
	if !db.Migrator().HasIndex(&model.AgentLesson{}, "idx_agent_lessons_author_status") {
		t.Fatal("schema migration v17 did not create owner status index")
	}
	if !db.Migrator().HasTable(&model.AgentMemorySetting{}) {
		t.Fatal("schema migration v18 did not create agent memory settings")
	}
	if !db.Migrator().HasColumn(&model.PaymentProviderConfig{}, "plugin_version") || !db.Migrator().HasColumn(&model.PaymentOrder{}, "plugin_version") {
		t.Fatal("schema migration v19 did not add payment plugin version columns")
	}
	if !db.Migrator().HasTable(&model.SkillPlatformState{}) || !db.Migrator().HasTable(&model.SkillCategoryPlatformState{}) {
		t.Fatal("schema migration v40 did not create platform skill availability tables")
	}
	if !db.Migrator().HasTable(&model.BannerAnnouncement{}) {
		t.Fatal("schema migration v20 did not create banner announcements")
	}
	if !db.Migrator().HasColumn(&model.BannerAnnouncement{}, "title_runs") {
		t.Fatal("schema migration v21 did not create banner announcements title_runs")
	}
	if !db.Migrator().HasColumn(&model.BannerAnnouncement{}, "notice_type") {
		t.Fatal("schema migration v22 did not create banner announcements notice_type")
	}
	if !db.Migrator().HasColumn(&model.TopupProduct{}, "Benefits") {
		t.Fatal("schema migration v24 did not create top-up product benefits")
	}
	if !db.Migrator().HasColumn(&model.TopupProduct{}, "AccentColor") {
		t.Fatal("schema migration v44 did not create top-up product accent color")
	}
	if !db.Migrator().HasTable(&model.Inspiration{}) || !db.Migrator().HasTable(&model.InspirationCoverDraft{}) {
		t.Fatal("schema migration v25 did not create inspiration tables")
	}
	var inspirationCount int64
	if err := db.Model(&model.Inspiration{}).Count(&inspirationCount).Error; err != nil || inspirationCount != 22 {
		t.Fatalf("schema migration v25 inspiration seed count = %d, error = %v", inspirationCount, err)
	}
	if !db.Migrator().HasTable(&model.UserPrompt{}) {
		t.Fatal("schema migration v26 did not create user prompts")
	}
	if !db.Migrator().HasColumn(&model.Inspiration{}, "CoverWidth") || !db.Migrator().HasColumn(&model.Inspiration{}, "CoverHeight") {
		t.Fatal("schema migration v27 did not create inspiration cover dimensions")
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("migration should be idempotent: %v", err)
	}
}

func TestMigrateSchemaV24UpgradesExistingDatabase(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-topup-product-benefits-v24?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&model.TopupProduct{}, "Benefits"); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("version = ?", 24).Delete(&schemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("upgrade from v23: %v", err)
	}
	if !db.Migrator().HasColumn(&model.TopupProduct{}, "Benefits") {
		t.Fatal("v24 upgrade did not install top-up product benefits column")
	}
	status, err := ReadSchemaStatus(db)
	if err != nil || !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("unexpected upgraded schema status: %+v, %v", status, err)
	}
}

func TestMigrateSchemaRemapsLegacyTopupProductBenefitsV16(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-topup-product-benefits-legacy-v16?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.AgentLesson{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("version = ?", 16).Delete(&schemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&schemaMigration{}).Where("version = ?", 24).Update("version", 16).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("upgrade legacy v16 record: %v", err)
	}
	if !db.Migrator().HasTable(&model.AgentLesson{}) {
		t.Fatal("official v16 migration was not applied after legacy record remap")
	}
	for _, version := range []int64{16, 24} {
		var count int64
		if err := db.Model(&schemaMigration{}).Where("version = ?", version).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("migration version %d count = %d, want 1", version, count)
		}
	}
}

func TestMigrateSchemaV15UpgradesExistingDatabase(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-agent-profiles-v15?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.AgentProfile{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("version = ?", 15).Delete(&schemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("upgrade from v14: %v", err)
	}
	if !db.Migrator().HasTable(&model.AgentProfile{}) || !db.Migrator().HasIndex(&model.AgentProfile{}, "idx_agent_profiles_scope") {
		t.Fatal("v15 upgrade did not install Agent profile table and scope index")
	}
	status, err := ReadSchemaStatus(db)
	if err != nil || !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("unexpected upgraded schema status: %+v, %v", status, err)
	}
}

func TestMigrateSchemaV16UpgradesExistingDatabase(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-agent-lessons-v16?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.AgentLesson{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("version = ?", 16).Delete(&schemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("upgrade from v15: %v", err)
	}
	if !db.Migrator().HasTable(&model.AgentLesson{}) || !db.Migrator().HasIndex(&model.AgentLesson{}, "idx_agent_lessons_status") {
		t.Fatal("v16 upgrade did not install Agent lesson table and status index")
	}
	status, err := ReadSchemaStatus(db)
	if err != nil || !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("unexpected upgraded schema status: %+v, %v", status, err)
	}
}

func TestMigrateSchemaV17UpgradesExistingDatabase(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-agent-lessons-v17?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropIndex(&model.AgentLesson{}, "idx_agent_lessons_author_status"); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("version = ?", 17).Delete(&schemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("upgrade from v16: %v", err)
	}
	if !db.Migrator().HasIndex(&model.AgentLesson{}, "idx_agent_lessons_author_status") {
		t.Fatal("v17 upgrade did not install owner status index")
	}
	status, err := ReadSchemaStatus(db)
	if err != nil || !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("unexpected upgraded schema status: %+v, %v", status, err)
	}
}

func TestMigrateSchemaV18UpgradesExistingDatabase(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-agent-memory-settings-v18?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.AgentMemorySetting{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("version = ?", 18).Delete(&schemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("upgrade from v17: %v", err)
	}
	if !db.Migrator().HasTable(&model.AgentMemorySetting{}) {
		t.Fatal("v18 upgrade did not install agent memory settings")
	}
	status, err := ReadSchemaStatus(db)
	if err != nil || !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("unexpected upgraded schema status: %+v, %v", status, err)
	}
}

func TestMigrateSchemaV19AddsPaymentPluginVersion(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-payment-plugin-version-v19?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&model.PaymentProviderConfig{}, "PluginVersion"); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&model.PaymentOrder{}, "PluginVersion"); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("version = ?", 19).Delete(&schemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("upgrade from v18: %v", err)
	}
	if !db.Migrator().HasColumn(&model.PaymentProviderConfig{}, "plugin_version") {
		t.Fatal("v19 upgrade did not add payment_provider_configs.plugin_version")
	}
	if !db.Migrator().HasColumn(&model.PaymentOrder{}, "plugin_version") {
		t.Fatal("v19 upgrade did not add payment_orders.plugin_version")
	}
	status, err := ReadSchemaStatus(db)
	if err != nil || !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("unexpected upgraded schema status: %+v, %v", status, err)
	}
}

func TestMigrateSchemaV20UpgradesExistingDatabaseWithBannerAnnouncements(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-banner-announcements-v20?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&model.BannerAnnouncement{}) {
		t.Fatal("v20 migration did not create banner_announcements table")
	}
	// 模拟旧库升级：删表 + 删除 v20 记录，重跑迁移应能重建。
	if err := db.Migrator().DropTable(&model.BannerAnnouncement{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("version = ?", 20).Delete(&schemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("upgrade from v19: %v", err)
	}
	if !db.Migrator().HasTable(&model.BannerAnnouncement{}) {
		t.Fatal("v20 upgrade did not reinstall banner_announcements table")
	}
	status, err := ReadSchemaStatus(db)
	if err != nil || !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("unexpected upgraded schema status: %+v, %v", status, err)
	}
}

func TestMigrateSchemaV21AddsBannerAnnouncementTitleRuns(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-banner-title-runs-v21?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(&model.BannerAnnouncement{}, "title_runs") {
		t.Fatal("v21 migration did not add banner_announcements.title_runs")
	}
	legacy := &model.BannerAnnouncement{ID: "legacy-banner", Title: "旧库通知", Status: "active"}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&model.BannerAnnouncement{}, "title_runs"); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("version = ?", 21).Delete(&schemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("upgrade from v20: %v", err)
	}
	if !db.Migrator().HasColumn(&model.BannerAnnouncement{}, "title_runs") {
		t.Fatal("v21 upgrade did not restore banner_announcements.title_runs")
	}
	var stored model.BannerAnnouncement
	if err := db.First(&stored, "id = ?", "legacy-banner").Error; err != nil {
		t.Fatal(err)
	}
	if stored.Title != "旧库通知" {
		t.Fatalf("legacy banner lost during upgrade: %+v", stored)
	}
	status, err := ReadSchemaStatus(db)
	if err != nil || !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("unexpected upgraded schema status: %+v, %v", status, err)
	}
}

func TestMigrateSchemaV22AddsBannerAnnouncementNoticeType(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-banner-notice-type-v22?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(&model.BannerAnnouncement{}, "notice_type") {
		t.Fatal("v22 migration did not add banner_announcements.notice_type")
	}
	legacy := &model.BannerAnnouncement{ID: "legacy-banner-v21", Title: "旧库通知", TitleRunsJSON: `[{"text":"旧库通知"}]`, Status: "active"}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&model.BannerAnnouncement{}, "notice_type"); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("version = ?", 22).Delete(&schemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("upgrade from v21: %v", err)
	}
	if !db.Migrator().HasColumn(&model.BannerAnnouncement{}, "notice_type") {
		t.Fatal("v22 upgrade did not restore banner_announcements.notice_type")
	}
	var stored model.BannerAnnouncement
	if err := db.First(&stored, "id = ?", "legacy-banner-v21").Error; err != nil {
		t.Fatal(err)
	}
	if stored.Title != "旧库通知" || stored.TitleRunsJSON != `[{"text":"旧库通知"}]` {
		t.Fatalf("legacy banner lost during upgrade: %+v", stored)
	}
	status, err := ReadSchemaStatus(db)
	if err != nil || !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("unexpected upgraded schema status: %+v, %v", status, err)
	}
}

func TestMigrateSchemaV23BackfillsCanvasRevisions(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-canvas-revisions-v23?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.CanvasSnapshotResource{}, &model.CanvasSnapshot{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&model.CanvasProject{}, "Revision"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO canvas_projects (id, user_id, title, payload_json) VALUES ('legacy', 'owner', 'Existing canvas', '{"nodes":[]}')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("version = ?", 23).Delete(&schemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	var project model.CanvasProject
	if err := db.First(&project, "id = ?", "legacy").Error; err != nil {
		t.Fatal(err)
	}
	if project.Revision != 1 || project.PayloadJSON != `{"nodes":[]}` {
		t.Fatalf("legacy canvas changed: %+v", project)
	}
	if !db.Migrator().HasTable(&model.CanvasSnapshot{}) || !db.Migrator().HasTable(&model.CanvasSnapshotResource{}) {
		t.Fatal("history tables missing")
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("migration not idempotent: %v", err)
	}
}

func TestMigrateSchemaV8AllowsReusingArchivedLogicalModelCode(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-logical-model-active-code?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE logical_models (id text PRIMARY KEY, code text NOT NULL, archived_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX idx_logical_models_code ON logical_models(code)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO logical_models(id, code, archived_at) VALUES ('archived', 'gpt-image-2', CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateSchemaV8(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO logical_models(id, code, archived_at) VALUES ('active', 'gpt-image-2', NULL)`).Error; err != nil {
		t.Fatalf("reusing archived code after migration: %v", err)
	}
	if err := db.Exec(`INSERT INTO logical_models(id, code, archived_at) VALUES ('duplicate', 'gpt-image-2', NULL)`).Error; err == nil {
		t.Fatal("active logical model code must remain unique")
	}
}

func TestMigrateSchemaV12AddsAgentTokenChargeLimit(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-agent-token-charge-limit?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE billing_orders (id text PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateSchemaV12(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(&model.BillingOrder{}, "ChargeLimitMicrocredits") {
		t.Fatal("migration v12 did not add Agent token charge limit")
	}
}

func TestMigrateSchemaRejectsChecksumMismatch(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-checksum?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&schemaMigration{}).Where("version = ?", CurrentSchemaVersion).Update("checksum", "changed").Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err == nil || !strings.Contains(err.Error(), "校验和不一致") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	if err := RequireSchemaVersion(db); err == nil || !strings.Contains(err.Error(), "校验和不一致") {
		t.Fatalf("schema verification must reject checksum mismatch, got %v", err)
	}
}

func TestMigrateSchemaV3NormalizesLegacyAccessoryCategory(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-asset-taxonomy?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Asset{}, &model.ProjectAssetCandidate{}); err != nil {
		t.Fatal(err)
	}
	asset := model.Asset{ID: "asset-1", UserID: "user-1", Kind: "image", Category: model.AssetCategory("accessory"), Title: "旧配饰"}
	candidate := model.ProjectAssetCandidate{ID: "candidate-1", ProjectID: "project-1", Name: "旧配饰候选", Category: model.AssetCategory("accessory"), Status: "pending_confirmation"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&candidate).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateSchemaV3(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&asset, "id = ?", asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&candidate, "id = ?", candidate.ID).Error; err != nil {
		t.Fatal(err)
	}
	if asset.Category != model.AssetCategoryProp || candidate.Category != model.AssetCategoryProp {
		t.Fatalf("legacy accessory categories = %q/%q, want prop/prop", asset.Category, candidate.Category)
	}
	if candidate.NameKey != model.AssetCandidateNameKey(candidate.Name) {
		t.Fatalf("candidate name key = %q", candidate.NameKey)
	}
}

func TestMigrateSchemaV4AddsResourceUploadKeyToExistingSchema(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-resource-upload-key?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE resources (id TEXT PRIMARY KEY, user_id TEXT NOT NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ModelChannel{}, &model.ChannelModel{}, &model.ChannelModelPriceTier{}, &model.BillingOrder{}); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatal(err)
	}
	for _, item := range schemaMigrations[:3] {
		if err := db.Create(&schemaMigration{Version: item.version, Name: item.name, Checksum: item.checksum, AppliedAt: time.Now().UTC()}).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := MigrateSchema(db); err != nil {
		t.Fatalf("migrate existing schema: %v", err)
	}
	if !db.Migrator().HasColumn(&model.Resource{}, "upload_key") {
		t.Fatal("resource upload_key column was not added")
	}
	if !db.Migrator().HasIndex(&model.Resource{}, "idx_resources_user_upload_key") {
		t.Fatal("resource upload key index was not added")
	}
	var status SchemaStatus
	status, err = ReadSchemaStatus(db)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("unexpected schema status: %#v", status)
	}

	firstKey := "same-upload"
	if err := db.Exec(`INSERT INTO resources (id, user_id, upload_key) VALUES (?, ?, ?)`, "resource-1", "user-1", firstKey).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO resources (id, user_id, upload_key) VALUES (?, ?, ?)`, "resource-2", "user-1", firstKey).Error; err == nil {
		t.Fatal("duplicate resource upload key should be rejected")
	}
}

func TestMigrateSchemaRepairsLegacyAssetFoldersMigrationOrder(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-legacy-v6-order?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Resource{}, &model.Asset{}, &model.AssetFolder{}, &model.ModelChannel{}, &model.ChannelModel{}, &model.ChannelModelPriceTier{}, &model.BillingOrder{}); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatal(err)
	}
	for _, item := range schemaMigrations[:5] {
		if err := db.Create(&schemaMigration{Version: item.version, Name: item.name, Checksum: item.checksum, AppliedAt: time.Now().UTC()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&schemaMigration{Version: 6, Name: "asset_library_folders", Checksum: assetLibraryFoldersChecksum, AppliedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}

	if err := MigrateSchema(db); err != nil {
		t.Fatalf("repair legacy migration order: %v", err)
	}
	if !db.Migrator().HasColumn(&model.Resource{}, "playback_status") || !db.Migrator().HasColumn(&model.Resource{}, "playback_object_key") || !db.Migrator().HasColumn(&model.Resource{}, "playback_error") {
		t.Fatal("legacy database did not receive resource playback columns")
	}
	status, err := ReadSchemaStatus(db)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("unexpected repaired schema status: %#v", status)
	}
	var applied schemaMigration
	if err := db.First(&applied, "version = ?", 6).Error; err != nil {
		t.Fatal(err)
	}
	if applied.Name != "asset_library_folders" || applied.Checksum != assetLibraryFoldersChecksum {
		t.Fatalf("historical migration 6 must be preserved: %#v", applied)
	}
	var playback schemaMigration
	if err := db.First(&playback, "version = ?", 7).Error; err != nil {
		t.Fatal(err)
	}
	if playback.Name != "resource_playback_variant" || playback.Checksum != resourcePlaybackChecksum {
		t.Fatalf("migration 7 must supply playback schema: %#v", playback)
	}
}

func TestMigrateSchemaRollsBackFailedMigration(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-rollback?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}

	original := schemaMigrations
	schemaMigrations = append(append([]migration(nil), original...), migration{
		version:  CurrentSchemaVersion + 1,
		name:     "rollback_probe",
		checksum: "sha256:rollback-probe",
		apply: func(tx *gorm.DB) error {
			if err := tx.Exec("CREATE TABLE migration_rollback_probe (id INTEGER PRIMARY KEY)").Error; err != nil {
				return err
			}
			return errors.New("forced migration failure")
		},
	})
	t.Cleanup(func() { schemaMigrations = original })

	if err := MigrateSchema(db); err == nil || !strings.Contains(err.Error(), "forced migration failure") {
		t.Fatalf("expected forced migration failure, got %v", err)
	}
	if db.Migrator().HasTable("migration_rollback_probe") {
		t.Fatal("failed migration left a partial table behind")
	}
	var count int64
	if err := db.Model(&schemaMigration{}).Where("version = ?", CurrentSchemaVersion+1).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed migration was recorded: %d", count)
	}
}

func TestRequireSchemaVersionRejectsUninitializedDatabase(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-uninitialized?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := RequireSchemaVersion(db); err == nil || !strings.Contains(err.Error(), "请先执行 migrate-schema up") {
		t.Fatalf("expected missing migration error, got %v", err)
	}
}

func TestMigrateSchemaV13AddsCloudAgentCanvasMutation(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-cloud-agent-canvas-mutation?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&model.CloudAgentCanvasMutation{}) {
		t.Fatal("migration v13 did not create cloud agent canvas mutation table")
	}
	for _, field := range []string{"RunID", "BeforeSnapshotHash", "AfterSnapshotHash", "BeforeJSON", "HasSubmittedTask", "Status"} {
		if !db.Migrator().HasColumn(&model.CloudAgentCanvasMutation{}, field) {
			t.Fatalf("migration v13 did not add %s", field)
		}
	}
}

func TestMigrateSchemaV30AddsBuiltinTools(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-builtin-tools-v30?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&model.Tool{}) {
		t.Fatal("migration v30 did not create tools table")
	}
}

func TestMigrateSchemaV31AddsToolUserActions(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-tool-user-actions-v31?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&model.ToolFavorite{}) {
		t.Fatal("migration v31 did not create tool_favorites table")
	}
}

func TestMigrateUserProfilesBackfillsOnlyLegacyUsernames(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-user-profiles-v37?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	users := []model.User{
		{ID: "legacy-id", Username: "legacy_name", DisplayName: "已有显示名称", Role: model.UserRoleUser, Status: model.UserStatusActive},
		{ID: "generated-id", Username: "generated-id", DisplayName: "mail-prefix", Role: model.UserRoleUser, Status: model.UserStatusActive},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateUserProfiles(db); err != nil {
		t.Fatal(err)
	}
	var migrated []model.User
	if err := db.Order("id").Find(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	byID := map[string]model.User{}
	for _, user := range migrated {
		byID[user.ID] = user
	}
	if byID["legacy-id"].ProfileName != "legacy_name" {
		t.Fatalf("legacy profile name = %q", byID["legacy-id"].ProfileName)
	}
	if byID["generated-id"].ProfileName != "" {
		t.Fatalf("generated username leaked into profile name = %q", byID["generated-id"].ProfileName)
	}
}

func TestMigrateUserLoginNamesAddsCaseInsensitiveUniqueness(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-user-login-names-v38?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{ID: "first-user", Username: "Creator", Role: model.UserRoleUser, Status: model.UserStatusActive}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateUserLoginNames(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{ID: "second-user", Username: "creator", Role: model.UserRoleUser, Status: model.UserStatusActive}).Error; err == nil {
		t.Fatal("case-insensitive duplicate username was accepted")
	}
}

func TestMigrateUserLoginNamesRejectsExistingCaseConflict(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-user-login-conflict-v38?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	users := []model.User{
		{ID: "first-user", Username: "Creator", Role: model.UserRoleUser, Status: model.UserStatusActive},
		{ID: "second-user", Username: "creator", Role: model.UserRoleUser, Status: model.UserStatusActive},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateUserLoginNames(db); err == nil {
		t.Fatal("migration accepted existing case-insensitive username conflict")
	}
}

func TestMigrateUserLoginEnvironmentAddsRegistrationIPAndEventTable(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-user-login-environment-v39?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE users (id text PRIMARY KEY, username text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateUserLoginEnvironment(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(&model.User{}, "RegistrationIP") {
		t.Fatal("users.registration_ip was not added")
	}
	if !db.Migrator().HasTable(&model.UserLoginEvent{}) {
		t.Fatal("user_login_events table was not created")
	}
}

func TestMigrateShortLoginUsernamesBackfillsGeneratedNames(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:migration-short-login-usernames-v41?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-24 * time.Hour)
	users := []model.User{
		{ID: "generated-one", Username: "generated-one", Email: "Creator.Name+tag@example.com", DisplayName: "old", Role: model.UserRoleUser, Status: model.UserStatusActive, CreatedAt: now},
		{ID: "generated-two", Username: "generated-two", Email: "Creator.Name@example.com", DisplayName: "old", Role: model.UserRoleUser, Status: model.UserStatusActive, CreatedAt: now},
		{ID: "generated-empty", Username: "generated-empty", Email: "", DisplayName: "old", Role: model.UserRoleUser, Status: model.UserStatusActive, CreatedAt: now},
		{ID: "legacy-long", Username: "Legacy_User_Name", Email: "legacy@example.com", DisplayName: "Old display", ProfileName: "Old profile", Role: model.UserRoleUser, Status: model.UserStatusActive, CreatedAt: now},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateUserLoginNames(db); err != nil {
		t.Fatal(err)
	}
	if err := migrateShortLoginUsernames(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&model.UserUsernameChange{}) || !db.Migrator().HasColumn(&model.User{}, "UsernameCustomizedAt") {
		t.Fatal("short username migration did not create its schema")
	}
	var migrated []model.User
	if err := db.Order("id").Find(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]model.User, len(migrated))
	seen := map[string]struct{}{}
	for _, user := range migrated {
		byID[user.ID] = user
		key := strings.ToLower(user.Username)
		if _, duplicate := seen[key]; duplicate {
			t.Fatalf("duplicate migrated username %q", user.Username)
		}
		seen[key] = struct{}{}
	}
	for _, id := range []string{"generated-one", "generated-two"} {
		user := byID[id]
		if user.Username == id || user.UsernameCustomizedAt != nil || user.DisplayName != user.Username || user.ProfileName != user.Username {
			t.Fatalf("generated user %q migration = %#v", id, user)
		}
	}
	noEmail := byID["generated-empty"]
	if noEmail.Username != noEmail.ID || noEmail.UsernameCustomizedAt == nil || noEmail.DisplayName != noEmail.Username || noEmail.ProfileName != noEmail.Username {
		t.Fatalf("no-email legacy user migration = %#v", noEmail)
	}
	legacy := byID["legacy-long"]
	if legacy.Username != "Legacy_User_Name" || legacy.UsernameCustomizedAt == nil || legacy.DisplayName != legacy.Username || legacy.ProfileName != legacy.Username {
		t.Fatalf("legacy user migration = %#v", legacy)
	}
	if err := db.Create(&model.User{ID: "case-conflict", Username: strings.ToLower(legacy.Username), Role: model.UserRoleUser, Status: model.UserStatusActive}).Error; err == nil {
		t.Fatal("case-insensitive username index was not preserved")
	}
}

func TestToolsUpgradeFromMain29PreservesMigrationChecksums(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:tools-main29-upgrade?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatal(err)
	}
	for _, item := range schemaMigrations {
		if item.version > 29 {
			break
		}
		if err := item.apply(db); err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&schemaMigration{Version: item.version, Name: item.name, Checksum: item.checksum, AppliedAt: time.Now()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	// The initial migration uses today's model registry; restore the actual v29
	// boundary so this test proves that v30/v31 create the new tables.
	if err := db.Migrator().DropTable(&model.ToolFavorite{}, &model.Tool{}); err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasTable(&model.Tool{}) || db.Migrator().HasTable(&model.ToolFavorite{}) {
		t.Fatal("tool tables must not exist before upgrading v29")
	}
	for range 2 {
		if err := MigrateSchema(db); err != nil {
			t.Fatal(err)
		}
	}
	for _, version := range []int64{28, 29} {
		var record schemaMigration
		if err := db.First(&record, version).Error; err != nil {
			t.Fatal(err)
		}
		expected := map[int64]string{28: "sha256:agent-execution-journal-v28", 29: "sha256:agent-resource-leases-v29-20260919"}
		if record.Checksum != expected[version] {
			t.Fatal("main checksum changed")
		}
	}
	if !db.Migrator().HasTable(&model.Tool{}) || !db.Migrator().HasTable(&model.ToolFavorite{}) {
		t.Fatal("tools tables missing")
	}
}
