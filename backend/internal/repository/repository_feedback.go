package repository

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas/backend/internal/model"
)

type FeedbackFilter struct {
	Type     string
	Status   string
	Keyword  string
	Archived *bool
	Page     int
	Limit    int
}

func (r *Repository) CreateFeedbackWithResources(fb *model.UserFeedback, resourceIDs []string) (*model.UserFeedback, error) {
	var saved model.UserFeedback
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Lock resource rows before inserting FK references, in the same order as deletion.
		if len(resourceIDs) > 0 {
			var resources []model.Resource
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id IN ?", fb.UserID, resourceIDs).Order("id").Find(&resources).Error; err != nil {
				return err
			}
			if len(resources) != len(resourceIDs) {
				return fmt.Errorf("反馈附件已删除，请重新选择")
			}
			for _, resource := range resources {
				if resource.Status != model.ResourceStatusReady {
					return fmt.Errorf("反馈附件尚未就绪")
				}
			}
		}
		created := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).Create(fb)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected > 0 {
			for _, id := range resourceIDs {
				ref := model.FeedbackResourceReference{FeedbackID: fb.ID, ResourceID: id, UserID: fb.UserID}
				if err := tx.Create(&ref).Error; err != nil {
					return err
				}
			}
		}
		return tx.Where("id = ? AND user_id = ?", fb.ID, fb.UserID).First(&saved).Error
	})
	return &saved, err
}

func (r *Repository) ListFeedbacksByUser(userID string) ([]model.UserFeedback, error) {
	list := []model.UserFeedback{}
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&list).Error
	return list, err
}

func (r *Repository) GetFeedback(id string) (*model.UserFeedback, error) {
	var fb model.UserFeedback
	err := r.db.Where("id = ?", id).First(&fb).Error
	if err != nil {
		return nil, err
	}
	return &fb, nil
}

func (r *Repository) ListAllFeedbacks(filter FeedbackFilter) ([]model.UserFeedback, int64, error) {
	query := r.db.Model(&model.UserFeedback{})

	if filter.Type != "" && filter.Type != "all" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Status == "archived" {
		query = query.Where("archived = ?", true)
	} else if filter.Status != "" && filter.Status != "all" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Archived != nil {
		query = query.Where("archived = ?", *filter.Archived)
	}
	if kw := strings.TrimSpace(filter.Keyword); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("title LIKE ? OR content LIKE ? OR user_name LIKE ? OR contact LIKE ?", like, like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	list := []model.UserFeedback{}
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

func (r *Repository) UpdateFeedbackIfVersion(fb *model.UserFeedback, expected int64) (bool, error) {
	result := r.db.Model(&model.UserFeedback{}).Where("id = ? AND version = ?", fb.ID, expected).Updates(map[string]any{
		"status": fb.Status, "admin_reply": fb.AdminReply, "replied_at": fb.RepliedAt,
		"archived": fb.Archived, "archived_at": fb.ArchivedAt, "updated_at": fb.UpdatedAt, "version": expected + 1,
	})
	return result.RowsAffected == 1, result.Error
}

func (r *Repository) DeleteFeedbackIfVersion(id string, expected int64) (bool, error) {
	result := r.db.Where("id = ? AND version = ?", id, expected).Delete(&model.UserFeedback{})
	if result.Error != nil || result.RowsAffected != 1 {
		return false, result.Error
	}
	if err := r.db.Where("feedback_id = ?", id).Delete(&model.FeedbackResourceReference{}).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) feedbackReferenceDocuments(userID string) ([]ResourceReferenceDocument, error) {
	var list []model.UserFeedback
	if err := r.db.Where("user_id = ?", userID).Find(&list).Error; err != nil {
		return nil, err
	}
	documents := make([]ResourceReferenceDocument, 0, len(list))
	for _, fb := range list {
		documents = append(documents, ResourceReferenceDocument{Kind: "用户反馈", ID: fb.ID, Title: fb.Title, PrimaryJSON: fb.Attachments})
	}
	return documents, nil
}
