package database

import (
	"encoding/json"
	"errors"
	"fmt"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// MigrationLedgerRecord shares the exact persisted ledger contract with the
// read-only Schema 83 copy tool. It is not an alternative migration plan.
type MigrationLedgerRecord = schemaMigration

// LockSchemaForCopy shares the normal migration lock so a copy cannot overlap
// another schema migration in the same PostgreSQL database.
func LockSchemaForCopy(db *gorm.DB) error {
	return db.Exec("SELECT pg_advisory_xact_lock(?)", postgresSchemaMigrationLockID).Error
}

// InitializeSchemaForCopy creates the real migration DDL in an empty target.
// Only bootstrap rows created in this transaction are removed before copying
// the source's complete data and original ledger. Existing targets are refused.
func InitializeSchemaForCopy(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		tables, err := tx.Migrator().GetTables()
		if err != nil {
			return err
		}
		if len(tables) != 0 {
			return errors.New("转库结构初始化只允许空目标，拒绝清理已有数据")
		}
		if err := MigrateSchema(tx); err != nil {
			return err
		}
		var catalog bundledInspirationCatalog
		if err := json.Unmarshal(inspirationCatalogJSON, &catalog); err != nil {
			return err
		}
		inspirationIDs := make([]any, 0, len(catalog.Entries))
		catalogKeys := make([]any, 0, len(catalog.Entries))
		for _, item := range catalog.Entries {
			id := item.LegacyID
			if id == "" {
				id = inspirationCatalogID(item.CatalogKey)
			}
			inspirationIDs = append(inspirationIDs, id)
			catalogKeys = append(catalogKeys, item.CatalogKey)
		}
		rootIDs := make([]any, 0)
		for _, item := range model.DefaultSkillCurationRoots() {
			rootIDs = append(rootIDs, item.ID)
		}
		sequenceNames := make([]any, 0, len(prefixedIDDefinitions))
		for _, item := range prefixedIDDefinitions {
			sequenceNames = append(sequenceNames, "id:"+item.prefix)
		}
		seeds := []struct {
			entity any
			column string
			keys   []any
		}{
			{&model.Inspiration{}, "id", inspirationIDs},
			{&model.InspirationCatalogRecord{}, "catalog_key", catalogKeys},
			{&model.SkillCurationSetting{}, "id", []any{int64(1)}},
			{&model.SkillCurationRoot{}, "id", rootIDs},
			{&model.IDSequence{}, "name", sequenceNames},
		}
		seedTables := map[string]bool{}
		for _, seed := range seeds {
			statement := &gorm.Statement{DB: tx}
			if err := statement.Parse(seed.entity); err != nil {
				return err
			}
			seedTables[statement.Schema.Table] = true
			var count, matched int64
			if err := tx.Model(seed.entity).Count(&count).Error; err != nil {
				return err
			}
			if err := tx.Model(seed.entity).Where(seed.column+" IN ?", seed.keys).Count(&matched).Error; err != nil {
				return err
			}
			if count != int64(len(seed.keys)) || matched != count {
				return fmt.Errorf("新目标 %s 的初始化身份不匹配，停止转库", statement.Schema.Table)
			}
		}
		for _, entity := range Models() {
			statement := &gorm.Statement{DB: tx}
			if err := statement.Parse(entity); err != nil {
				return err
			}
			if seedTables[statement.Schema.Table] {
				continue
			}
			var count int64
			if err := tx.Unscoped().Model(entity).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				return fmt.Errorf("新目标 %s 存在非预期初始化数据，停止转库", statement.Schema.Table)
			}
		}
		for _, seed := range seeds {
			result := tx.Unscoped().Where(seed.column+" IN ?", seed.keys).Delete(seed.entity)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != int64(len(seed.keys)) {
				return errors.New("初始化数据在转库期间发生变化，停止转库")
			}
		}
		// MigrateSchema validated these uncommitted bootstrap records. The copy
		// now replaces them with the source lineage and original applied times.
		result := tx.Where("version BETWEEN ? AND ?", 1, CurrentSchemaVersion).Delete(&schemaMigration{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != CurrentSchemaVersion {
			return errors.New("新目标迁移账本数量不匹配，停止转库")
		}
		return nil
	})
}
