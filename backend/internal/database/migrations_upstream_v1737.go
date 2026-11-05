package database

import (
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func upstreamV1737Migrations() []migration {
	return []migration{
		{version: 104, name: "ark_portrait_assets", checksum: "sha256:ark-portrait-assets-v104", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.ArkPortraitSession{}, &model.ArkPortraitGroup{}, &model.ArkPortraitAsset{})
		}},
		{version: 105, name: "token_cache_write_price", checksum: "sha256:token-cache-write-price-v105", apply: migrateTokenCacheWritePrice},
	}
}

// Append columns without changing existing migration identities or recalculating historical bills.
func migrateTokenCacheWritePrice(tx *gorm.DB) error {
	for _, target := range []struct {
		entity  any
		columns []string
	}{
		{&model.ChannelModel{}, []string{"cache_write_token_price_microcredits"}},
		{&model.ChannelModelPriceTier{}, []string{"cache_write_token_price_microcredits", "cost_cache_write_token_price_microcredits"}},
		{&model.BillingOrder{}, []string{"cache_write_token_price_microcredits", "cost_cache_write_token_price_microcredits", "cache_write_tokens"}},
		{&model.LogicalModel{}, []string{"cache_write_price_microcredits"}},
		{&model.ApiCallLog{}, []string{"cache_write_tokens"}},
	} {
		if !tx.Migrator().HasTable(target.entity) {
			continue
		}
		for _, column := range target.columns {
			if tx.Migrator().HasColumn(target.entity, column) {
				continue
			}
			if err := tx.Migrator().AddColumn(target.entity, column); err != nil {
				return err
			}
		}
	}
	return nil
}
