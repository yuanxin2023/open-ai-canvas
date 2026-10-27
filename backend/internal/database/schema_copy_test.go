package database

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestCopySchemaHasNoBootstrapRowsOrLedger(t *testing.T) {
	db := isolatedMigrationDB(t)
	if err := InitializeSchemaForCopy(db); err != nil {
		t.Fatal(err)
	}
	for _, entity := range append(Models(), &MigrationLedgerRecord{}) {
		var count int64
		if err := db.Unscoped().Model(entity).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("bootstrap rows remain in %T: %d %v", entity, count, err)
		}
	}
	if !db.Migrator().HasIndex(&model.User{}, "idx_users_username_ci") || !db.Migrator().HasIndex(&model.CanvasProject{}, "idx_canvas_projects_user_updated") {
		t.Fatal("copy initialization lost real migration DDL")
	}
}

func TestCopySchemaRefusesExistingDatabase(t *testing.T) {
	db := isolatedMigrationDB(t)
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Inspiration{}).Where("created_by = ?", "system").UpdateColumn("title", "Preserved edit").Error; err != nil {
		t.Fatal(err)
	}
	if err := InitializeSchemaForCopy(db); err == nil {
		t.Fatal("existing target accepted")
	}
	if err := RequireSchemaVersion(db); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.Inspiration{}).Where("title = ?", "Preserved edit").Count(&count).Error; err != nil || count != 22 {
		t.Fatalf("existing seed edits changed: %d %v", count, err)
	}
}
