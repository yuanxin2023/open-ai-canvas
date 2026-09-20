package repository

import (
	"encoding/json"
	"strings"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func decodeUserPrompt(row *model.UserPrompt) {
	row.Tags = []string{}
	_ = json.Unmarshal([]byte(row.TagsJSON), &row.Tags)
}

func (r *Repository) UserPrompts(userID, keyword string, mode model.InspirationMode, limit, offset int) ([]model.UserPrompt, int64, error) {
	query := r.db.Model(&model.UserPrompt{}).Where("user_id = ?", userID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR description LIKE ? OR prompt LIKE ? OR tags_json LIKE ?", like, like, like, like)
	}
	if mode != "" {
		query = query.Where("mode = ?", mode)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.UserPrompt
	if err := query.Order("updated_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	for index := range rows {
		decodeUserPrompt(&rows[index])
	}
	return rows, total, nil
}

func (r *Repository) UserPromptCount(userID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.UserPrompt{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *Repository) UserPromptForUser(userID, id string) (*model.UserPrompt, error) {
	var row model.UserPrompt
	if err := r.db.First(&row, "id = ? AND user_id = ?", id, userID).Error; err != nil {
		return nil, err
	}
	decodeUserPrompt(&row)
	return &row, nil
}

func (r *Repository) CreateUserPrompt(row *model.UserPrompt) error {
	return r.db.Create(row).Error
}

func (r *Repository) UpdateUserPrompt(row *model.UserPrompt) error {
	result := r.db.Model(&model.UserPrompt{}).Where("id = ? AND user_id = ?", row.ID, row.UserID).Updates(map[string]any{
		"title": row.Title, "description": row.Description, "mode": row.Mode, "prompt": row.Prompt,
		"tags_json": row.TagsJSON, "source": row.Source, "cover_resource_id": row.CoverResourceID,
		"cover_url": row.CoverURL, "updated_at": row.UpdatedAt,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) DeleteUserPrompt(userID, id string) (bool, error) {
	result := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.UserPrompt{})
	return result.RowsAffected == 1, result.Error
}
