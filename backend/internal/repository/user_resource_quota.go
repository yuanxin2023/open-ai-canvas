package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas/backend/internal/model"
)

func (r *Repository) UserResourceQuota(userID string) (*model.UserResourceQuota, error) {
	var row model.UserResourceQuota
	err := r.db.First(&row, "user_id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

// Serialize patches against the user row, including the first override write.
func (r *Repository) PatchUserResourceQuota(userID string, apply func(*Repository) (*model.UserResourceQuota, *model.AdminAuditEvent, error)) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		query := tx.Where("id = ? AND role = ?", userID, model.UserRoleUser)
		if r.Dialect() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&user).Error; err != nil {
			return err
		}
		if r.Dialect() == "sqlite" {
			if err := tx.Model(&model.User{}).Where("id = ?", userID).UpdateColumn("updated_at", user.UpdatedAt).Error; err != nil {
				return err
			}
		}
		row, audit, err := apply(&Repository{db: tx})
		if err != nil {
			return err
		}
		if row == nil {
			if err := tx.Delete(&model.UserResourceQuota{}, "user_id = ?", userID).Error; err != nil {
				return err
			}
		} else if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"overrides_json", "updated_by", "updated_at"})}).Create(row).Error; err != nil {
			return err
		}
		return tx.Create(audit).Error
	})
}

func (r *Repository) ArchivedAssetUserIDs() ([]string, error) {
	var ids []string
	err := r.db.Model(&model.Asset{}).Where("status = ?", model.AssetVersionStatusArchived).Distinct("user_id").Pluck("user_id", &ids).Error
	return ids, err
}

func (r *Repository) ExpiredArchivedAssetsForUser(userID string, cutoff time.Time, limit int) ([]model.Asset, error) {
	var rows []model.Asset
	err := r.db.Where("user_id = ? AND status = ? AND updated_at <= ?", userID, model.AssetVersionStatusArchived, cutoff).Order("updated_at asc, id asc").Limit(limit).Find(&rows).Error
	return rows, err
}
