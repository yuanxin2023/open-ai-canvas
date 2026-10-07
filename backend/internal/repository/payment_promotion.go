package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

var ErrPaymentPromotionImageDraftUnavailable = errors.New("payment promotion image draft unavailable")

func (r *Repository) CreatePaymentPromotionImageDraft(draft *model.PaymentPromotionImageDraft) error {
	return r.db.Create(draft).Error
}

func (r *Repository) PaymentPromotionImageDraftForUser(userID string, resourceID string) (*model.PaymentPromotionImageDraft, error) {
	var draft model.PaymentPromotionImageDraft
	if err := r.db.First(&draft, "resource_id = ? AND user_id = ?", resourceID, userID).Error; err != nil {
		return nil, err
	}
	return &draft, nil
}

func (r *Repository) DeletePaymentPromotionImageDraft(userID string, resourceID string) error {
	return r.db.Where("resource_id = ? AND user_id = ?", resourceID, userID).Delete(&model.PaymentPromotionImageDraft{}).Error
}

func (r *Repository) StalePaymentPromotionImageDrafts(before time.Time, limit int) ([]model.PaymentPromotionImageDraft, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var drafts []model.PaymentPromotionImageDraft
	err := r.db.Where("created_at < ?", before).Order("created_at asc").Limit(limit).Find(&drafts).Error
	return drafts, err
}

func (r *Repository) SavePaymentPromotionSetting(setting *model.SystemSetting, draftUserID string, newDraftResourceID string, oldResource *model.Resource, deletionJob *model.ResourceDeletionJob) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if newDraftResourceID != "" {
			result := tx.Where("resource_id = ? AND user_id = ?", newDraftResourceID, draftUserID).Delete(&model.PaymentPromotionImageDraft{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrPaymentPromotionImageDraftUnavailable
			}
		}
		if err := tx.Save(setting).Error; err != nil {
			return err
		}
		if oldResource == nil {
			return nil
		}
		if err := tx.Where("resource_id = ?", oldResource.ID).Delete(&model.ArkPrivateAssetBinding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("resource_id = ?", oldResource.ID).Delete(&model.PaymentPromotionImageDraft{}).Error; err != nil {
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

func (r *Repository) DiscardPaymentPromotionImageDraft(userID string, resource *model.Resource, deletionJob *model.ResourceDeletionJob) error {
	if resource == nil {
		return gorm.ErrRecordNotFound
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var draft model.PaymentPromotionImageDraft
		if err := tx.First(&draft, "resource_id = ? AND user_id = ?", resource.ID, userID).Error; err != nil {
			return err
		}
		if err := tx.Where("resource_id = ?", resource.ID).Delete(&model.ArkPrivateAssetBinding{}).Error; err != nil {
			return err
		}
		deleted := tx.Where("id = ? AND user_id = ?", resource.ID, userID).Delete(&model.Resource{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
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
