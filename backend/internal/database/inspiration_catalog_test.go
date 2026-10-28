package database

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestOfficialInspirationCatalogIsAppendOnlyAndKeepsDeletionTombstones(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: "file:official-inspiration-catalog?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}

	var inspirationCount, recordCount int64
	if err := db.Model(&model.Inspiration{}).Count(&inspirationCount).Error; err != nil || inspirationCount != 1341 {
		t.Fatalf("inspirations = %d, err = %v, want 1341", inspirationCount, err)
	}
	if err := db.Model(&model.InspirationCatalogRecord{}).Count(&recordCount).Error; err != nil || recordCount != 1341 {
		t.Fatalf("catalog records = %d, err = %v, want 1341", recordCount, err)
	}

	const preservedID = "INS-000000000000000000000000000001"
	if err := db.Model(&model.Inspiration{}).Where("id = ?", preservedID).Updates(map[string]any{"title": "管理员已改标题", "status": model.InspirationStatusDisabled}).Error; err != nil {
		t.Fatal(err)
	}
	var imported model.InspirationCatalogRecord
	if err := db.Where("source_id = ?", "seedance").First(&imported).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&model.Inspiration{}, "id = ?", imported.InspirationID).Error; err != nil {
		t.Fatal(err)
	}
	const preLedgerDeletedID = "INS-000000000000000000000000000022"
	if err := db.Delete(&model.Inspiration{}, "id = ?", preLedgerDeletedID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&model.InspirationCatalogRecord{}, "inspiration_id = ?", preLedgerDeletedID).Error; err != nil {
		t.Fatal(err)
	}

	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	var preserved model.Inspiration
	if err := db.First(&preserved, "id = ?", preservedID).Error; err != nil {
		t.Fatal(err)
	}
	if preserved.Title != "管理员已改标题" || preserved.Status != model.InspirationStatusDisabled {
		t.Fatalf("catalog sync overwrote admin changes: %#v", preserved)
	}
	if err := db.Model(&model.Inspiration{}).Where("id = ?", imported.InspirationID).Count(&inspirationCount).Error; err != nil || inspirationCount != 0 {
		t.Fatalf("deleted catalog item was resurrected: count=%d err=%v", inspirationCount, err)
	}
	if err := db.Model(&model.Inspiration{}).Where("id = ?", preLedgerDeletedID).Count(&inspirationCount).Error; err != nil || inspirationCount != 0 {
		t.Fatalf("legacy item deleted before ledger was resurrected: count=%d err=%v", inspirationCount, err)
	}
	if err := db.Model(&model.InspirationCatalogRecord{}).Count(&recordCount).Error; err != nil || recordCount != 1341 {
		t.Fatalf("catalog records after resync = %d, err = %v", recordCount, err)
	}
}
