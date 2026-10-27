package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/platform"
	"infinite-canvas/backend/internal/repository"
)

func quotaTestService(t *testing.T, driver string) (*Service, *gorm.DB, *model.User) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "quota.db") + "?_journal_mode=WAL&_busy_timeout=5000"
	if driver == "postgres" {
		dsn = os.Getenv("CANVAS_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("CANVAS_TEST_POSTGRES_DSN is not configured")
		}
		base, err := database.Open(database.Config{Driver: driver, DSN: dsn})
		if err != nil {
			t.Fatal(err)
		}
		schema := fmt.Sprintf("user_quota_%d", time.Now().UnixNano())
		if err := base.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = base.Exec(`DROP SCHEMA "` + schema + `" CASCADE`).Error
			sqlDB, _ := base.DB()
			_ = sqlDB.Close()
		})
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		dsn = parsed.String()
	}
	db, err := database.Open(database.Config{Driver: driver, DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	actor := &model.User{ID: "admin", Username: "admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull, Status: model.UserStatusActive}
	for _, user := range []*model.User{actor, {ID: "one", Username: "usrone", Role: model.UserRoleUser, Status: model.UserStatusActive}, {ID: "two", Username: "usrtwo", Role: model.UserRoleUser, Status: model.UserStatusActive}} {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	return &Service{repo: repository.New(db), dataDir: t.TempDir()}, db, actor
}

func TestUserResourceQuotaLifecycle(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			s, db, actor := quotaTestService(t, driver)
			set := func(userID string, patch platform.ResourceQuotaPatch) *UserResourceQuotaView {
				view, err := s.PatchAdminUserResourceQuota(actor, userID, UserResourceQuotaRequest{Overrides: patch})
				if err != nil {
					t.Fatal(err)
				}
				return view
			}
			upload, stored := int64(10), int64(5)
			view := set("one", platform.ResourceQuotaPatch{"resourceUploadMB": &upload, "storedFileGB": &stored})
			if view.EffectiveQuota.ResourceUploadMB != 10 || view.EffectiveQuota.StoredFileGB != 5 {
				t.Fatal(view)
			}
			other, err := s.UserRuntimePolicy("two")
			if err != nil || other.Resource.ResourceUploadMB != 50 {
				t.Fatalf("other user: %#v %v", other, err)
			}
			if err := s.ValidateUserUploadSize("one", 10<<20); err == nil {
				t.Fatal("single file limit bypassed")
			}
			if _, err := s.reserveChunkedUploadQuota("one", 10<<20); err == nil {
				t.Fatal("chunk limit bypassed")
			}
			if err := s.ValidateUserUploadSize("two", 10<<20); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PatchAdminUserResourceQuota(&model.User{ID: "one", Role: model.UserRoleUser}, "two", UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"storedFileGB": &stored}}); err == nil {
				t.Fatal("user modified quota")
			}
			if _, err := s.PatchAdminUserResourceQuota(actor, "admin", UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"storedFileGB": &stored}}); err == nil {
				t.Fatal("ordinary user endpoint accepted admin target")
			}
			if _, err := s.PatchAdminUserResourceQuota(actor, "one", UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"unknown": nil}}); err == nil {
				t.Fatal("unknown field accepted")
			}
			view = set("one", platform.ResourceQuotaPatch{"resourceUploadMB": nil})
			if view.EffectiveQuota.ResourceUploadMB != 50 || view.EffectiveQuota.StoredFileGB != 5 {
				t.Fatal("partial restore", view)
			}
			policy := defaultRuntimePolicy()
			policy.Resource.ResourceUploadMB = 40
			if _, err := s.UpdateRuntimePolicySetting(actor, policy); err != nil {
				t.Fatal(err)
			}
			updated, err := s.UserRuntimePolicy("one")
			if err != nil || updated.Resource.ResourceUploadMB != 40 || updated.Resource.StoredFileGB != 5 {
				t.Fatal("dynamic inheritance", updated, err)
			}
			generated := int64(1)
			set("one", platform.ResourceQuotaPatch{"generatedFileMB": &generated})
			if _, _, err := s.decodeDataURL("one", dataURL("image/png", make([]byte, 2<<20))); err == nil {
				t.Fatal("generation file quota bypassed")
			}
			for _, id := range []string{"resource", "duplicate"} {
				if err := db.Create(&model.Resource{ID: id, UserID: "one", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "same-file", Size: 6 << 30}).Error; err != nil {
					t.Fatal(err)
				}
			}
			usage, err := s.AccountFileStorageUsage("one")
			if err != nil || usage.UsedBytes != 6<<30 || usage.TotalBytes != 5<<30 {
				t.Fatal("dedup/over limit usage", usage, err)
			}
			if _, err := s.reserveUserUploadQuota("one", 1); err == nil {
				t.Fatal("over capacity upload accepted")
			}
			limit := int64(1)
			set("one", platform.ResourceQuotaPatch{"structuredDataMB": &limit, "assetCount": &limit})
			dataUsage := repository.UserStorageUsage{AssetBytes: 2 << 20, AssetCount: 2}
			if err := validateUserStructuredStorageQuota(s.repo, "one", dataUsage, "asset", false, -1, policy.Resource); err != nil {
				t.Fatal("shrinking over quota blocked", err)
			}
			if err := validateUserStructuredStorageQuota(s.repo, "one", dataUsage, "asset", false, 1, policy.Resource); err == nil {
				t.Fatal("growth accepted")
			}
			if err := validateUserStructuredStorageQuota(s.repo, "one", dataUsage, "asset", true, 0, policy.Resource); err == nil {
				t.Fatal("count quota bypassed")
			}
			set("one", platform.ResourceQuotaPatch{"resourceUploadMB": nil, "storedFileGB": nil, "generatedFileMB": nil, "structuredDataMB": nil, "assetCount": nil})
			row, err := s.repo.UserResourceQuota("one")
			if err != nil || row != nil {
				t.Fatal("full restore left overrides", row, err)
			}
			var auditCount int64
			if err := db.Model(&model.AdminAuditEvent{}).Where("action = ?", "user.resource_quota.update").Count(&auditCount).Error; err != nil || auditCount < 5 {
				t.Fatal("missing audit", auditCount, err)
			}
		})
	}
}

func TestUserResourceQuotaConcurrentReservationAndRelease(t *testing.T) {
	s, _, actor := quotaTestService(t, "sqlite")
	upload, daily := int64(2), int64(2)
	if _, err := s.PatchAdminUserResourceQuota(actor, "one", UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"resourceUploadMB": &upload, "dailyUploadMB": &daily}}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan string, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			day, err := s.reserveUserUploadQuota("one", 1<<20)
			if err == nil {
				results <- day
			}
		}()
	}
	wg.Wait()
	close(results)
	days := []string{}
	for day := range results {
		days = append(days, day)
	}
	if len(days) != 1 {
		t.Fatalf("reserved %d files against exclusive 2MB daily limit", len(days))
	}
	s.releaseUserUploadQuota("one", days[0], 1<<20)
	if _, err := s.reserveUserUploadQuota("one", 1<<20); err != nil {
		t.Fatal("failed upload did not release quota", err)
	}
}

func TestUserResourceQuotaUpgradeFrom50(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			s, db, actor := quotaTestService(t, driver)
			if err := db.Migrator().DropTable(&model.UserResourceQuota{}); err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("DELETE FROM schema_migrations WHERE version >= 51").Error; err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := database.MigrateSchema(db); err != nil {
					t.Fatal(err)
				}
			}
			if !db.Migrator().HasTable("user_resource_quotas") {
				t.Fatal("v51 table missing")
			}
			view, err := s.AdminUserResourceQuota(actor, "one")
			if err != nil || len(view.Overrides) != 0 || view.EffectiveQuota != view.DefaultQuota {
				t.Fatal("existing user did not inherit", view, err)
			}
		})
	}
}

func TestUserResourceQuotaPostgresConcurrentPatches(t *testing.T) {
	s, _, actor := quotaTestService(t, "postgres")
	upload, stored := int64(10), int64(5)
	patches := []platform.ResourceQuotaPatch{{"resourceUploadMB": &upload}, {"storedFileGB": &stored}}
	var wg sync.WaitGroup
	errors := make(chan error, len(patches))
	for _, patch := range patches {
		wg.Add(1)
		go func(patch platform.ResourceQuotaPatch) {
			defer wg.Done()
			_, err := s.PatchAdminUserResourceQuota(actor, "one", UserResourceQuotaRequest{Overrides: patch})
			errors <- err
		}(patch)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	view, err := s.AdminUserResourceQuota(actor, "one")
	if err != nil || len(view.Overrides) != 2 || view.EffectiveQuota.ResourceUploadMB != 10 || view.EffectiveQuota.StoredFileGB != 5 {
		t.Fatal("concurrent patch lost a field", view, err)
	}
}

func TestUserResourceQuotaRecycleBinRetention(t *testing.T) {
	s, db, actor := quotaTestService(t, "sqlite")
	zero, day := int64(0), int64(1)
	for id, retention := range map[string]*int64{"one": &zero, "two": &day} {
		if _, err := s.PatchAdminUserResourceQuota(actor, id, UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"recycleBinRetentionDays": retention}}); err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.Asset{ID: id, UserID: id, Kind: "image", Status: model.AssetVersionStatusArchived, PayloadJSON: `{"kind":"image"}`, UpdatedAt: time.Now().Add(-72 * time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	s.cleanupExpiredArchivedAssets()
	var count int64
	if err := db.Model(&model.Asset{}).Where("user_id = ?", "one").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("zero retention did not preserve assets", count, err)
	}
	if err := db.Model(&model.Asset{}).Where("user_id = ?", "two").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("expired assets not cleaned", count, err)
	}
}
