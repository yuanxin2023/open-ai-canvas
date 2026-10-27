package database

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

type historicalTableSnapshot struct {
	Name    string
	Columns []string
	Rows    json.RawMessage
}

// These fixtures must be created by the pinned Schema 51 source, rather than
// today's Models() registry with an old version number written into its ledger.
func TestUpgradeOriginalSchema51Fixtures(t *testing.T) {
	root := os.Getenv("CANVAS_TEST_MIGRATION_FIXTURES")
	if root == "" {
		t.Skip("CANVAS_TEST_MIGRATION_FIXTURES is not configured")
	}
	for _, driver := range []string{"sqlite", "postgres"} {
		for _, lineage := range []string{"local", "upstream"} {
			t.Run(driver+"_"+lineage, func(t *testing.T) {
				name := driver + "_" + lineage + "_51"
				raw, err := os.ReadFile(filepath.Join(root, name+".json"))
				if err != nil {
					t.Fatal(err)
				}
				var snapshots []historicalTableSnapshot
				if err := json.Unmarshal(raw, &snapshots); err != nil || len(snapshots) < 20 {
					t.Fatalf("incomplete original data manifest: %v", err)
				}
				db := openHistoricalFixtureCopy(t, root, name, driver)
				status, err := ReadSchemaStatus(db)
				if err != nil || status.Current != 51 || status.Ready {
					t.Fatalf("fixture must be the original Schema 51: %#v %v", status, err)
				}
				for run := 0; run < 2; run++ {
					if err := MigrateSchema(db); err != nil {
						t.Fatal(err)
					}
					if err := RequireSchemaVersion(db); err != nil {
						t.Fatal(err)
					}
					for _, snapshot := range snapshots {
						var rows []map[string]any
						query := db.Table(snapshot.Name).Select(snapshot.Columns).Order(strings.Join(snapshot.Columns, ","))
						if snapshot.Name == "schema_migrations" {
							query = query.Where("version <= ?", 51)
						}
						if err := query.Find(&rows).Error; err != nil {
							t.Fatalf("read original columns in %s: %v", snapshot.Name, err)
						}
						encoded, err := json.Marshal(rows)
						if err != nil {
							t.Fatal(err)
						}
						var before, after any
						beforeDecoder := json.NewDecoder(bytes.NewReader(snapshot.Rows))
						beforeDecoder.UseNumber()
						if err := beforeDecoder.Decode(&before); err != nil {
							t.Fatal(err)
						}
						afterDecoder := json.NewDecoder(bytes.NewReader(encoded))
						afterDecoder.UseNumber()
						if err := afterDecoder.Decode(&after); err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(before, after) {
							t.Errorf("%s original data changed on upgrade run %d", snapshot.Name, run+1)
						}
					}
					for _, table := range []string{"workbench_submissions", "user_inspiration_libraries", "user_feedbacks", "feedback_resource_references"} {
						var count int64
						if err := db.Table(table).Count(&count).Error; err != nil || count != 0 {
							t.Errorf("%s should be newly created and empty: %d %v", table, count, err)
						}
					}
				}
			})
		}
	}
}

func openHistoricalFixtureCopy(t *testing.T, root, name, driver string) *gorm.DB {
	t.Helper()
	var config Config
	if driver == "sqlite" {
		raw, err := os.ReadFile(filepath.Join(root, name+".db"))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "fixture-copy.db")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		config = Config{Driver: driver, DSN: path + "?_foreign_keys=on"}
	} else {
		baseDSN := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
		if !strings.Contains(baseDSN, "@codex-canvas-merge-v1692-postgres:5432/canvas_merge_fixture?") {
			t.Fatal("historical PostgreSQL fixtures require the isolated merge test server")
		}
		base, err := Open(Config{Driver: driver, DSN: baseDSN})
		if err != nil {
			t.Fatal(err)
		}
		copyName := fmt.Sprintf("merge_fixture_%d", time.Now().UnixNano())
		if err := base.Exec("CREATE DATABASE " + copyName + " TEMPLATE " + name).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := base.Exec("DROP DATABASE " + copyName).Error; err != nil {
				t.Error(err)
			}
			sqlDB, _ := base.DB()
			_ = sqlDB.Close()
		})
		config = Config{Driver: driver, DSN: strings.Replace(baseDSN, "/canvas_merge_fixture?", "/"+copyName+"?", 1)}
	}
	db, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
