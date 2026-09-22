package repository

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInspirationOrderChanged = errors.New("inspiration order changed")
var ErrInspirationCoverDraftUnavailable = errors.New("inspiration cover draft unavailable")
var ErrInspirationCoverReferenced = errors.New("inspiration cover is still referenced")

func decodeInspiration(row *model.Inspiration) {
	row.Tags = []string{}
	_ = json.Unmarshal([]byte(row.TagsJSON), &row.Tags)
}

func (r *Repository) ActiveInspirations() ([]model.Inspiration, error) {
	var rows []model.Inspiration
	err := r.db.Where("status = ?", model.InspirationStatusActive).Order("sort_order ASC, id ASC").Find(&rows).Error
	for index := range rows {
		decodeInspiration(&rows[index])
	}
	return rows, err
}

func (r *Repository) AdminInspirations(keyword string, mode model.InspirationMode, status model.InspirationStatus, limit, offset int) ([]model.Inspiration, int64, error) {
	query := r.db.Model(&model.Inspiration{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR description LIKE ? OR prompt LIKE ? OR tags_json LIKE ?", like, like, like, like)
	}
	if mode != "" {
		query = query.Where("mode = ?", mode)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.Inspiration
	if err := query.Order("sort_order ASC, id ASC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	for index := range rows {
		decodeInspiration(&rows[index])
	}
	return rows, total, nil
}

func (r *Repository) Inspiration(id string) (*model.Inspiration, error) {
	var row model.Inspiration
	if err := r.db.First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	decodeInspiration(&row)
	return &row, nil
}

func (r *Repository) InspirationOrder() ([]model.Inspiration, error) {
	var rows []model.Inspiration
	err := r.db.Select("id", "title", "status", "sort_order").Order("sort_order ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *Repository) NextInspirationSortOrder() (int64, error) {
	var value int64
	err := r.db.Model(&model.Inspiration{}).Select("COALESCE(MAX(sort_order), 0)").Scan(&value).Error
	return value + 1, err
}

func (r *Repository) CreateInspirationCoverDraft(draft *model.InspirationCoverDraft) error {
	return r.db.Create(draft).Error
}

func (r *Repository) InspirationCoverDraftForUser(userID, resourceID string) (*model.InspirationCoverDraft, error) {
	var draft model.InspirationCoverDraft
	if err := r.db.First(&draft, "resource_id = ? AND user_id = ?", resourceID, userID).Error; err != nil {
		return nil, err
	}
	return &draft, nil
}

func (r *Repository) DeleteInspirationCoverDraft(userID, resourceID string) error {
	return r.db.Where("resource_id = ? AND user_id = ?", resourceID, userID).Delete(&model.InspirationCoverDraft{}).Error
}

func (r *Repository) StaleInspirationCoverDrafts(before time.Time, limit int) ([]model.InspirationCoverDraft, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var drafts []model.InspirationCoverDraft
	err := r.db.Where("created_at < ?", before).Order("created_at ASC").Limit(limit).Find(&drafts).Error
	return drafts, err
}

func consumeInspirationCoverDraft(tx *gorm.DB, userID, resourceID string) error {
	if resourceID == "" {
		return nil
	}
	result := tx.Where("resource_id = ? AND user_id = ?", resourceID, userID).Delete(&model.InspirationCoverDraft{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrInspirationCoverDraftUnavailable
	}
	return nil
}

func (r *Repository) CreateInspiration(row *model.Inspiration) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := consumeInspirationCoverDraft(tx, row.CreatedBy, row.CoverResourceID); err != nil {
			return err
		}
		return tx.Create(row).Error
	})
}

func (r *Repository) UpdateInspiration(row *model.Inspiration, draftUserID, newDraftResourceID string, oldResource *model.Resource, deletionJob *model.ResourceDeletionJob) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := consumeInspirationCoverDraft(tx, draftUserID, newDraftResourceID); err != nil {
			return err
		}
		result := tx.Model(&model.Inspiration{}).Where("id = ?", row.ID).Updates(map[string]any{
			"title": row.Title, "description": row.Description, "mode": row.Mode, "prompt": row.Prompt,
			"tags_json": row.TagsJSON, "source": row.Source, "cover_resource_id": row.CoverResourceID,
			"cover_url": row.CoverURL, "cover_width": row.CoverWidth, "cover_height": row.CoverHeight,
			"updated_by": row.UpdatedBy, "updated_at": row.UpdatedAt,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if oldResource == nil {
			return nil
		}
		var count int64
		if err := tx.Model(&model.Inspiration{}).Where("cover_resource_id = ?", oldResource.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrInspirationCoverReferenced
		}
		if err := tx.Where("resource_id = ?", oldResource.ID).Delete(&model.InspirationCoverDraft{}).Error; err != nil {
			return err
		}
		deleted := tx.Where("id = ? AND user_id = ?", oldResource.ID, oldResource.UserID).Delete(&model.Resource{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if deletionJob != nil {
			return tx.Create(deletionJob).Error
		}
		return nil
	})
}

func (r *Repository) UpdateInspirationStatus(id string, status model.InspirationStatus, updatedBy string, at time.Time) (bool, error) {
	result := r.db.Model(&model.Inspiration{}).Where("id = ?", id).Updates(map[string]any{"status": status, "updated_by": updatedBy, "updated_at": at})
	return result.RowsAffected == 1, result.Error
}

func (r *Repository) SaveInspirationOrder(ids, expected []string, updatedBy string, at time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var current []model.Inspiration
		query := tx.Select("id").Order("sort_order ASC, id ASC")
		if r.Dialect() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Find(&current).Error; err != nil {
			return err
		}
		if len(current) != len(expected) || len(ids) != len(expected) {
			return ErrInspirationOrderChanged
		}
		for index := range current {
			if current[index].ID != expected[index] {
				return ErrInspirationOrderChanged
			}
		}
		seen := map[string]bool{}
		for index, id := range ids {
			if seen[id] {
				return ErrInspirationOrderChanged
			}
			seen[id] = true
			result := tx.Model(&model.Inspiration{}).Where("id = ?", id).Updates(map[string]any{"sort_order": index + 1, "updated_by": updatedBy, "updated_at": at})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrInspirationOrderChanged
			}
		}
		return nil
	})
}

func (r *Repository) DeleteInspirations(ids []string, resources []model.Resource, deletionJobs []model.ResourceDeletionJob) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var rows []model.Inspiration
		query := tx.Where("id IN ?", ids)
		if r.Dialect() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return gorm.ErrRecordNotFound
		}
		if result := tx.Where("id IN ?", ids).Delete(&model.Inspiration{}); result.Error != nil {
			return result.Error
		} else if result.RowsAffected != int64(len(ids)) {
			return gorm.ErrInvalidData
		}
		for index := range resources {
			resource := resources[index]
			var count int64
			if err := tx.Model(&model.Inspiration{}).Where("cover_resource_id = ?", resource.ID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrInspirationCoverReferenced
			}
			if err := tx.Where("resource_id = ?", resource.ID).Delete(&model.InspirationCoverDraft{}).Error; err != nil {
				return err
			}
			deleted := tx.Where("id = ? AND user_id = ?", resource.ID, resource.UserID).Delete(&model.Resource{})
			if deleted.Error != nil {
				return deleted.Error
			}
			if deleted.RowsAffected != 1 {
				return gorm.ErrInvalidData
			}
		}
		if len(deletionJobs) > 0 {
			return tx.Create(&deletionJobs).Error
		}
		return nil
	})
}

func (r *Repository) DiscardInspirationCoverDraft(userID string, resource *model.Resource, deletionJob *model.ResourceDeletionJob) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var draft model.InspirationCoverDraft
		if err := tx.First(&draft, "resource_id = ? AND user_id = ?", resource.ID, userID).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.Inspiration{}).Where("cover_resource_id = ?", resource.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrInspirationCoverReferenced
		}
		if err := tx.Where("id = ? AND user_id = ?", resource.ID, userID).Delete(&model.Resource{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&draft).Error; err != nil {
			return err
		}
		if deletionJob != nil {
			return tx.Create(deletionJob).Error
		}
		return nil
	})
}
