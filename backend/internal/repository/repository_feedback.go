package repository

import (
	"strings"

	"infinite-canvas/backend/internal/model"
)

// FeedbackResourceReferenceKind 标记反馈附件对资源的引用；孤儿清理和管理员删除都保留这些资源。
const FeedbackResourceReferenceKind = "用户反馈"

// Both orphan cleanup and explicit deletion must scan the official attachment JSON contract.
func (r *Repository) feedbackReferenceDocuments(userID string) ([]ResourceReferenceDocument, error) {
	var feedbacks []model.UserFeedback
	query := r.db.Select("id", "title", "attachments").Where("attachments <> ? AND attachments <> ?", "", "[]")
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Find(&feedbacks).Error; err != nil {
		return nil, err
	}
	documents := make([]ResourceReferenceDocument, 0, len(feedbacks))
	for _, feedback := range feedbacks {
		documents = append(documents, ResourceReferenceDocument{Kind: FeedbackResourceReferenceKind, ID: feedback.ID, Title: feedback.Title, PrimaryJSON: feedback.Attachments})
	}
	return documents, nil
}

type FeedbackFilter struct {
	Type     string
	Status   string
	Keyword  string
	Archived *bool
	Page     int
	Limit    int
}

func (r *Repository) CreateFeedback(fb *model.UserFeedback) error {
	return r.db.Create(fb).Error
}

func (r *Repository) ListFeedbacksByUser(userID string) ([]model.UserFeedback, error) {
	var list []model.UserFeedback
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

	var list []model.UserFeedback
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

func (r *Repository) UpdateFeedback(fb *model.UserFeedback) error {
	return r.db.Save(fb).Error
}

func (r *Repository) DeleteFeedback(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.UserFeedback{}).Error
}
