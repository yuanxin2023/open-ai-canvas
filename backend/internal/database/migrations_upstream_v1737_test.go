package database

import (
	"reflect"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestUpstreamV1737AppendOnlyUpgradePreservesHistory(t *testing.T) {
	db := isolatedMigrationDB(t)
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	account := model.CreditAccount{UserID: "migration-1737-user", AvailableMicrocredits: 123456789, ReservedMicrocredits: 7654321, Version: 7}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	var ledger []schemaMigration
	if err := db.Where("version <= ?", 103).Order("version").Find(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	if len(ledger) != 103 {
		t.Fatalf("base ledger length = %d", len(ledger))
	}
	if err := db.Where("version > ?", 103).Delete(&schemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, entity := range []any{&model.ArkPortraitAsset{}, &model.ArkPortraitGroup{}, &model.ArkPortraitSession{}} {
		if err := db.Migrator().DropTable(entity); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []struct {
		entity any
		column string
	}{
		{&model.ChannelModel{}, "cache_write_token_price_microcredits"},
		{&model.ChannelModelPriceTier{}, "cache_write_token_price_microcredits"},
		{&model.ChannelModelPriceTier{}, "cost_cache_write_token_price_microcredits"},
		{&model.BillingOrder{}, "cache_write_token_price_microcredits"},
		{&model.BillingOrder{}, "cost_cache_write_token_price_microcredits"},
		{&model.BillingOrder{}, "cache_write_tokens"},
		{&model.LogicalModel{}, "cache_write_price_microcredits"},
		{&model.ApiCallLog{}, "cache_write_tokens"},
	} {
		if err := db.Migrator().DropColumn(target.entity, target.column); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := MigrateSchema(db); err != nil {
			t.Fatal(err)
		}
	}
	var after []schemaMigration
	if err := db.Where("version <= ?", 103).Order("version").Find(&after).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ledger, after) {
		t.Fatal("upgrade rewrote the existing migration ledger")
	}
	var restored model.CreditAccount
	if err := db.First(&restored, "user_id = ?", account.UserID).Error; err != nil {
		t.Fatal(err)
	}
	if restored.AvailableMicrocredits != account.AvailableMicrocredits || restored.ReservedMicrocredits != account.ReservedMicrocredits || restored.Version != account.Version {
		t.Fatal("upgrade changed credit account balances or version")
	}
	status, err := ReadSchemaStatus(db)
	if err != nil || !status.Ready || status.Current != 105 {
		t.Fatalf("schema status = %+v, error = %v", status, err)
	}
	for _, entity := range []any{&model.ArkPortraitAsset{}, &model.ArkPortraitGroup{}, &model.ArkPortraitSession{}} {
		if !db.Migrator().HasTable(entity) {
			t.Fatalf("portrait table is missing for %T", entity)
		}
	}
	if !db.Migrator().HasColumn(&model.BillingOrder{}, "cache_write_tokens") {
		t.Fatal("cache write usage column is missing")
	}
	if db.Migrator().HasColumn(&model.PaymentOrder{}, "exchange_rate_micros") {
		t.Fatal("upgrade enabled the declined exchange-rate schema")
	}
}
