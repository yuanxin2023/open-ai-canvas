package repository

import (
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) InspirationLibrary(userID string) (*model.UserInspirationLibrary, error) {
	var row model.UserInspirationLibrary
	err := r.db.Clauses(clause.Locking{Strength: "SHARE"}).First(&row, "user_id = ?", userID).Error
	return &row, err
}

func (r *Repository) LockInspirationLibrary(userID string) (*model.UserInspirationLibrary, error) {
	row := model.UserInspirationLibrary{UserID: userID, StateJSON: "{}"}
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return nil, err
	}
	// The first write also serializes SQLite transactions; PostgreSQL retains
	// the row lock until the enclosing transaction commits or rolls back.
	if err := r.db.Model(&model.UserInspirationLibrary{}).Where("user_id = ?", userID).
		UpdateColumn("revision", gorm.Expr("revision")).Error; err != nil {
		return nil, err
	}
	return r.InspirationLibrary(userID)
}

func (r *Repository) SaveInspirationLibrary(row *model.UserInspirationLibrary, previous int64) error {
	result := r.db.Model(&model.UserInspirationLibrary{}).Where("user_id = ? AND revision = ?", row.UserID, previous).
		Updates(map[string]any{"revision": row.Revision, "state_json": row.StateJSON, "updated_at": row.UpdatedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
