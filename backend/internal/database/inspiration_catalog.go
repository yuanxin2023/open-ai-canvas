package database

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

//go:embed seed/inspiration-catalog.json
var inspirationCatalogJSON []byte

type bundledInspirationCatalog struct {
	Entries []bundledInspiration `json:"entries"`
}

type bundledInspiration struct {
	CatalogKey  string                `json:"catalogKey"`
	LegacyID    string                `json:"legacyId,omitempty"`
	Revision    string                `json:"revision,omitempty"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	Mode        model.InspirationMode `json:"mode"`
	Prompt      string                `json:"prompt"`
	Tags        []string              `json:"tags"`
	Source      string                `json:"source,omitempty"`
	SourceID    string                `json:"sourceId,omitempty"`
	SourceURL   string                `json:"sourceUrl,omitempty"`
	Category    string                `json:"category,omitempty"`
	Ratio       string                `json:"ratio,omitempty"`
	CoverURL    string                `json:"coverUrl"`
}

func migrateInspirationCatalog(tx *gorm.DB) error {
	return tx.AutoMigrate(&model.Inspiration{}, &model.InspirationCatalogRecord{})
}

func inspirationCatalogID(catalogKey string) string {
	sum := sha256.Sum256([]byte(catalogKey))
	return "INS-" + hex.EncodeToString(sum[:16])
}

// syncInspirationCatalog only appends catalog keys that have never been seen.
// Existing rows are never overwritten, and a catalog record with no matching
// Inspiration row is an intentional deletion tombstone.
func syncInspirationCatalog(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&model.InspirationCatalogRecord{}) {
		return nil
	}
	var catalog bundledInspirationCatalog
	if err := json.Unmarshal(inspirationCatalogJSON, &catalog); err != nil {
		return fmt.Errorf("解析官方灵感目录：%w", err)
	}
	if len(catalog.Entries) == 0 {
		return fmt.Errorf("官方灵感目录为空")
	}

	var known []string
	if err := tx.Model(&model.InspirationCatalogRecord{}).Pluck("catalog_key", &known).Error; err != nil {
		return fmt.Errorf("读取官方灵感导入记录：%w", err)
	}
	seen := make(map[string]struct{}, len(known)+len(catalog.Entries))
	for _, key := range known {
		seen[key] = struct{}{}
	}
	var existingIDs []string
	if err := tx.Model(&model.Inspiration{}).Pluck("id", &existingIDs).Error; err != nil {
		return fmt.Errorf("读取已有灵感 ID：%w", err)
	}
	ids := make(map[string]struct{}, len(existingIDs)+len(catalog.Entries))
	for _, id := range existingIDs {
		ids[id] = struct{}{}
	}

	var maxSort int64
	if err := tx.Model(&model.Inspiration{}).Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSort).Error; err != nil {
		return fmt.Errorf("读取灵感排序：%w", err)
	}
	now := time.Now().UTC()
	rows := make([]model.Inspiration, 0, len(catalog.Entries))
	records := make([]model.InspirationCatalogRecord, 0, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		if entry.CatalogKey == "" {
			return fmt.Errorf("官方灵感目录包含空 catalogKey")
		}
		if _, exists := seen[entry.CatalogKey]; exists {
			continue
		}
		seen[entry.CatalogKey] = struct{}{}

		inspirationID := entry.LegacyID
		if inspirationID == "" {
			inspirationID = inspirationCatalogID(entry.CatalogKey)
		}
		_, exists := ids[inspirationID]
		if !exists && entry.LegacyID == "" {
			tagsJSON, marshalErr := json.Marshal(entry.Tags)
			if marshalErr != nil {
				return marshalErr
			}
			maxSort++
			rows = append(rows, model.Inspiration{
				ID: inspirationID, Title: entry.Title, Description: entry.Description, Mode: entry.Mode,
				Prompt: entry.Prompt, Tags: entry.Tags, TagsJSON: string(tagsJSON), Source: entry.Source,
				SourceID: entry.SourceID, SourceURL: entry.SourceURL, Category: entry.Category, Ratio: entry.Ratio,
				CoverURL: entry.CoverURL, Status: model.InspirationStatusActive, SortOrder: maxSort,
				CreatedBy: "system", UpdatedBy: "system", CreatedAt: now, UpdatedAt: now,
			})
			ids[inspirationID] = struct{}{}
		} else if !exists && entry.LegacyID != "" {
			// A missing legacy seed ID means an administrator deleted it before
			// the catalog ledger existed. Register only the tombstone.
		} else if exists && entry.LegacyID != "" {
			// These columns did not exist before this migration. Populate only
			// provenance metadata; all administrator-editable content stays intact.
			if err := tx.Model(&model.Inspiration{}).Where("id = ?", inspirationID).UpdateColumns(map[string]any{
				"source_id": entry.SourceID, "source_url": entry.SourceURL,
			}).Error; err != nil {
				return fmt.Errorf("补充内置灵感来源 %s：%w", inspirationID, err)
			}
		} else if exists && entry.LegacyID == "" {
			return fmt.Errorf("官方灵感稳定 ID 与已有记录冲突：%s", inspirationID)
		}
		records = append(records, model.InspirationCatalogRecord{
			CatalogKey: entry.CatalogKey, InspirationID: inspirationID, SourceID: entry.SourceID,
			Revision: entry.Revision, ImportedAt: now,
		})
	}
	if len(rows) > 0 {
		if err := tx.CreateInBatches(&rows, 40).Error; err != nil {
			return fmt.Errorf("导入官方灵感：%w", err)
		}
	}
	if len(records) > 0 {
		if err := tx.CreateInBatches(&records, 40).Error; err != nil {
			return fmt.Errorf("记录官方灵感：%w", err)
		}
	}
	return nil
}
