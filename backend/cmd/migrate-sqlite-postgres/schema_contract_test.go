package main

import (
	"testing"

	"infinite-canvas/backend/internal/database"
)

func TestNewSchemaSatisfiesCopyStructureContract(t *testing.T) {
	source, err := database.Open(database.Config{Driver: "sqlite", DSN: "file:copy-structure-current-ddl?mode=memory&cache=shared&_foreign_keys=on"})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := source.DB()
	defer sqlDB.Close()
	if err := database.MigrateSchema(source); err != nil {
		t.Fatal(err)
	}
	_, target := fixtureTarget(t)
	if err := database.MigrateSchema(target); err != nil {
		t.Fatal(err)
	}
	if err := verifyTargetStructure(source, target); err != nil {
		t.Fatal(err)
	}
}
