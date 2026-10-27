package repository

import (
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// These locks must use the transaction repository. Lock the old billing order
// before reserving the new wallet amount, then lock the source task afterwards.
func (r *Repository) LockWorkbenchRetryBillingOrder(userID, id string) error {
	var order model.BillingOrder
	return r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ? AND user_id = ?", id, userID).Error
}

func (r *Repository) LockWorkbenchRetryTask(userID, id string) (*model.Task, error) {
	result := r.db.Model(&model.Task{}).Where("id = ? AND user_id = ?", id, userID).UpdateColumn("updated_at", gorm.Expr("updated_at"))
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return r.TaskForUser(userID, id)
}
