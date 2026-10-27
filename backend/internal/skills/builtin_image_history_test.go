package skills

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func builtinImageHistoryService(t *testing.T, driver string) (*Service, *gorm.DB) {
	t.Helper()
	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	var db *gorm.DB
	var err error
	if driver == "postgres" {
		dsn := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("CANVAS_TEST_POSTGRES_DSN is not configured")
		}
		admin, openErr := gorm.Open(postgres.Open(dsn), config)
		if openErr != nil {
			t.Fatal(openErr)
		}
		schema := fmt.Sprintf("legacy_images_%d", time.Now().UnixNano())
		if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
			connection, _ := admin.DB()
			_ = connection.Close()
		})
		parsed, parseErr := url.Parse(dsn)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		db, err = gorm.Open(postgres.Open(parsed.String()), config)
	} else {
		db, err = gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "skills.db")), config)
	}
	if err != nil {
		t.Fatal(err)
	}
	connection, _ := db.DB()
	connection.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = connection.Close() })
	if err := db.AutoMigrate(&model.Skill{}, &model.SkillVersion{}, &model.SkillFile{}, &model.UserSkillState{}, &model.SkillPlatformState{}, &model.SkillCategoryPlatformState{}, &model.BuiltinSkillTombstone{}, &model.SkillCurationAssignment{}, &model.SkillCurationRootAssignment{}); err != nil {
		t.Fatal(err)
	}
	return New(repository.New(db), t.TempDir(), nil), db
}

func addHistoricalImageSkill(t *testing.T, svc *Service, db *gorm.DB, suffix string) model.Skill {
	t.Helper()
	var canonical model.Skill
	if err := db.First(&canonical, "id = ?", "open-ai-canvas-image-"+suffix).Error; err != nil {
		t.Fatal(err)
	}
	legacy := model.Skill{ID: "yingce-image-" + suffix, Name: canonical.Name, OwnerID: "historical-owner", Source: model.SkillSourcePlatform, SourceType: "builtin", Status: model.SkillStatusEnabled, Instruction: "# Historical image workflow\n\nKeep this installed content.\n"}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	archive, err := archiveFromMarkdown([]byte(legacy.Instruction), legacy.Name, "Historical content")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.addSkillArchiveVersion(&legacy, archive, "builtin", "", "", "", "", false); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&legacy, "id = ?", legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	return legacy
}

func TestBuiltinSyncRetainsDeployedOfficialImageIdentities(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			svc, db := builtinImageHistoryService(t, driver)
			if err := svc.EnsureBuiltinSkills(); err != nil {
				t.Fatal(err)
			}
			var legacy []model.Skill
			for _, suffix := range []string{"editing", "annotation", "layer-split"} {
				legacy = append(legacy, addHistoricalImageSkill(t, svc, db, suffix))
			}
			installed := model.UserSkillState{ID: "retained-install", UserID: "reader", SkillID: legacy[2].ID, InstalledVersionID: legacy[2].CurrentVersionID, Added: true, Liked: true, LibraryCategoryID: "personal-category"}
			if err := db.Create(&installed).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.First(&installed, "id = ?", installed.ID).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.SkillPlatformState{SkillID: legacy[1].ID, Available: false}).Error; err != nil {
				t.Fatal(err)
			}
			for run := 0; run < 2; run++ {
				if err := svc.EnsureBuiltinSkills(); err != nil {
					t.Fatal(err)
				}
				if err := svc.EnsureSkillPackages(); err != nil {
					t.Fatal(err)
				}
				for _, before := range legacy {
					var after model.Skill
					if err := db.First(&after, "id = ?", before.ID).Error; err != nil || !reflect.DeepEqual(before, after) {
						t.Fatalf("historical identity changed: %s / %v", before.ID, err)
					}
					var count int64
					if err := db.Model(&model.SkillVersion{}).Where("skill_id = ?", before.ID).Count(&count).Error; err != nil || count != 1 {
						t.Fatalf("historical versions changed: %s / %d / %v", before.ID, count, err)
					}
				}
			}
			var after model.UserSkillState
			if err := db.First(&after, "id = ?", installed.ID).Error; err != nil || !reflect.DeepEqual(installed, after) {
				t.Fatalf("installed relationship changed: %+v / %v", after, err)
			}
			if available, err := svc.repo.PlatformSkillAvailable(legacy[1]); err != nil || available {
				t.Fatalf("historical hidden state changed: %v / %v", available, err)
			}
			if err := svc.repo.DeleteBuiltinSkill(legacy[0].ID, "administrator"); err != nil {
				t.Fatal(err)
			}
			if err := svc.EnsureBuiltinSkills(); err != nil {
				t.Fatal(err)
			}
			var resurrected int64
			if err := db.Model(&model.Skill{}).Where("id = ?", legacy[0].ID).Count(&resurrected).Error; err != nil || resurrected != 0 {
				t.Fatal("deleted historical image skill was recreated")
			}
		})
	}
}

func TestBuiltinImageHistoryRejectsUnverifiedRecords(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			for _, failure := range []string{"private", "missing-local", "wrong-installed-version", "missing-version", "corrupt-archive", "unsafe-package-key"} {
				t.Run(failure, func(t *testing.T) {
					svc, db := builtinImageHistoryService(t, driver)
					if err := svc.EnsureBuiltinSkills(); err != nil {
						t.Fatal(err)
					}
					legacy := addHistoricalImageSkill(t, svc, db, "layer-split")
					var err error
					switch failure {
					case "private":
						err = db.Model(&legacy).Update("is_private", true).Error
					case "missing-local":
						err = db.Delete(&model.Skill{}, "id = ?", "open-ai-canvas-image-layer-split").Error
					case "wrong-installed-version":
						var local model.Skill
						if err := db.First(&local, "id = ?", "open-ai-canvas-image-layer-split").Error; err != nil {
							t.Fatal(err)
						}
						err = db.Create(&model.UserSkillState{ID: "invalid-install", UserID: "reader", SkillID: legacy.ID, InstalledVersionID: local.CurrentVersionID, Added: true}).Error
					case "missing-version":
						err = db.Delete(&model.SkillVersion{}, "id = ?", legacy.CurrentVersionID).Error
					case "corrupt-archive":
						version, versionErr := svc.repo.SkillVersion(legacy.CurrentVersionID)
						if versionErr != nil {
							t.Fatal(versionErr)
						}
						err = os.WriteFile(filepath.Join(svc.dataDir, "skill-packages", filepath.FromSlash(version.PackageKey)), []byte("broken archive"), 0o600)
					case "unsafe-package-key":
						err = db.Model(&model.SkillVersion{}).Where("id = ?", legacy.CurrentVersionID).Update("package_key", legacy.ID+"/../foreign.zip").Error
					}
					if err != nil {
						t.Fatal(err)
					}
					var before, after int64
					if err := db.Model(&model.SkillVersion{}).Count(&before).Error; err != nil {
						t.Fatal(err)
					}
					if err := svc.EnsureBuiltinSkills(); err == nil {
						t.Fatal("unverified historical identity was accepted")
					}
					if err := db.Model(&model.SkillVersion{}).Count(&after).Error; err != nil || before != after {
						t.Fatal("failed verification partially updated the catalog")
					}
				})
			}
		})
	}
}
