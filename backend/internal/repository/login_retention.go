package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func loginAuditTimeExpression(tx *gorm.DB) string {
	if tx.Dialector.Name() == "sqlite" {
		// Historical SQLite timestamps can carry different UTC offsets.
		return "julianday(created_at)"
	}
	return "created_at"
}

func expireLoginAudit(tx *gorm.DB, cutoff time.Time, userID *string) error {
	condition := "created_at < ?"
	if tx.Dialector.Name() == "sqlite" {
		condition = "julianday(created_at) < julianday(?)"
	}
	for _, row := range []any{&model.UserLoginEvent{}, &model.LoginLog{}} {
		query := tx.Where(condition, cutoff.UTC())
		if userID != nil {
			query = query.Where("user_id = ?", *userID)
		}
		if err := query.Delete(row).Error; err != nil {
			return err
		}
	}
	return nil
}

func successfulLoginAudit(tx *gorm.DB, userID string) *gorm.DB {
	timestamp := loginAuditTimeExpression(tx)
	current := tx.Model(&model.LoginLog{}).
		Select("id, 'current' AS storage, "+timestamp+" AS retention_time, id AS audit_id").
		Where("user_id = ? AND event = ? AND result = ?", userID, "login", "success")
	legacy := tx.Model(&model.UserLoginEvent{}).
		Select("id, 'legacy' AS storage, "+timestamp+" AS retention_time, 'legacy:' || id AS audit_id").
		Where("user_id = ?", userID)
	return tx.Table("(?) AS successful_login_audit", tx.Raw("? UNION ALL ?", current, legacy))
}

// The caller holds the account lock also used by session issuance. Keep the
// newest 200 across both stores; deleting surplus rows never revokes sessions.
func pruneUserLoginAudit(tx *gorm.DB, userID string, now time.Time) error {
	if err := expireLoginAudit(tx, now.Add(-userLoginEventRetention), &userID); err != nil {
		return err
	}
	for _, storage := range []string{"legacy", "current"} {
		kept := successfulLoginAudit(tx, userID).
			Order("retention_time DESC, audit_id DESC, storage DESC").Limit(userLoginEventLimit)
		ids := tx.Table("(?) AS retained_login_audit", kept).Select("id").Where("storage = ?", storage)
		query := tx.Where("user_id = ? AND id NOT IN (?)", userID, ids)
		var row any = &model.UserLoginEvent{}
		if storage == "current" {
			row = &model.LoginLog{}
			query = query.Where("event = ? AND result = ?", "login", "success")
		}
		if err := query.Delete(row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) PruneLoginAudit(now time.Time) error {
	// Commit age cleanup before taking account locks, avoiding a lock inversion
	// with a login that holds its account lock while pruning the same rows.
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		return expireLoginAudit(tx, now.Add(-userLoginEventRetention), nil)
	}); err != nil {
		return err
	}
	lastID := ""
	for {
		current := r.db.Model(&model.LoginLog{}).Select("user_id").Where("event = ? AND result = ?", "login", "success")
		legacy := r.db.Model(&model.UserLoginEvent{}).Select("user_id")
		var userIDs []string
		if err := r.db.Table("(?) AS audit_accounts", r.db.Raw("? UNION ?", current, legacy)).
			Where("user_id > ?", lastID).Order("user_id").Limit(100).Pluck("user_id", &userIDs).Error; err != nil {
			return err
		}
		if len(userIDs) == 0 {
			return nil
		}
		for _, userID := range userIDs {
			if err := r.db.Transaction(func(tx *gorm.DB) error {
				if err := lockProfileUser(tx, userID); err != nil {
					return err
				}
				var user model.User
				err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Take(&user, "id = ?", userID).Error
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				return pruneUserLoginAudit(tx, userID, now)
			}); err != nil {
				return err
			}
		}
		lastID = userIDs[len(userIDs)-1]
	}
}
