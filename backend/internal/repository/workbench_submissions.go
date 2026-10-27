package repository

import (
	"errors"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm/clause"
)

var ErrWorkbenchSubmissionConflict = errors.New("workbench request key has different input")

func (r *Repository) WorkbenchSubmission(userID, key string) (*model.WorkbenchSubmission, error) {
	var record model.WorkbenchSubmission
	err := r.db.First(&record, "user_id = ? AND request_key = ?", userID, key).Error
	return &record, err
}

// Insert and task/billing reservation share one transaction. OnConflict waits
// for a concurrent winner to commit; a rollback leaves the key unconsumed.
func (r *Repository) CommitWorkbenchSubmission(record *model.WorkbenchSubmission, save func(*Repository) error) (*model.Task, bool, error) {
	var task *model.Task
	replayed := false
	err := r.Transaction(func(repo *Repository) error {
		result := repo.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "request_key"}}, DoNothing: true,
		}).Create(record)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			old, err := repo.WorkbenchSubmission(record.UserID, record.RequestKey)
			if err != nil {
				return err
			}
			if old.RequestHash != record.RequestHash || old.Kind != record.Kind {
				return ErrWorkbenchSubmissionConflict
			}
			*record = *old
			replayed = true
		} else if err := save(repo); err != nil {
			return err
		}
		var err error
		task, err = repo.TaskForUser(record.UserID, record.TaskID)
		return err
	})
	return task, replayed, err
}
