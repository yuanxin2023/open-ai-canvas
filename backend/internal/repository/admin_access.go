package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas/backend/internal/model"
	"time"
)

func (r *Repository) HydrateAdminAccess(user *model.User) error {
	if user == nil || user.Role != model.UserRoleAdmin {
		return nil
	}
	if user.AdminLevel == model.AdminLevelFull || user.AdminLevel == "" {
		user.AdminLevel = model.AdminLevelFull
		user.AdminPermissions = append([]model.AdminPermission(nil), model.AllAdminPermissions...)
		return nil
	}
	var permissions []model.AdminPermission
	if err := r.db.Model(&model.AdminPermissionGrant{}).
		Where("user_id = ?", user.ID).Order("permission asc").Pluck("permission", &permissions).Error; err != nil {
		return err
	}
	user.AdminPermissions = permissions
	return nil
}

func (r *Repository) AdminPermissions(userID string) ([]model.AdminPermission, error) {
	var permissions []model.AdminPermission
	err := r.db.Model(&model.AdminPermissionGrant{}).
		Where("user_id = ?", userID).Order("permission asc").Pluck("permission", &permissions).Error
	return permissions, err
}

func (r *Repository) ReplaceAdminPermissions(userID string, permissions []model.AdminPermission, grantedBy string, now time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return replaceAdminPermissions(tx, userID, permissions, grantedBy, now)
	})
}

func replaceAdminPermissions(tx *gorm.DB, userID string, permissions []model.AdminPermission, grantedBy string, now time.Time) error {
	if err := tx.Delete(&model.AdminPermissionGrant{}, "user_id = ?", userID).Error; err != nil {
		return err
	}
	if len(permissions) == 0 {
		return nil
	}
	grants := make([]model.AdminPermissionGrant, 0, len(permissions))
	for _, permission := range permissions {
		grants = append(grants, model.AdminPermissionGrant{UserID: userID, Permission: permission, GrantedByUserID: grantedBy, CreatedAt: now, UpdatedAt: now})
	}
	return tx.Create(&grants).Error
}

func (r *Repository) ActiveFullAdminCountExcluding(userID string) (int64, error) {
	var count int64
	query := r.db.Model(&model.User{}).Where("role = ? AND (admin_level = ? OR admin_level = '') AND status = ?", model.UserRoleAdmin, model.AdminLevelFull, model.UserStatusActive)
	if userID != "" {
		query = query.Where("id <> ?", userID)
	}
	err := query.Count(&count).Error
	return count, err
}

func (r *Repository) CreateAdminManagedUser(user *model.User, permissions []model.AdminPermission, grantedBy string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		return replaceAdminPermissions(tx, user.ID, permissions, grantedBy, user.CreatedAt)
	})
}

func (r *Repository) SaveAdminManagedUser(user *model.User, permissions []model.AdminPermission, grantedBy string, revokeSessions bool, protectLastFull bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if protectLastFull {
			var fullAdminIDs []string
			query := tx.Model(&model.User{}).
				Where("role = ? AND status = ? AND (admin_level = ? OR admin_level = '')", model.UserRoleAdmin, model.UserStatusActive, model.AdminLevelFull).
				Order("id")
			if r.Dialect() == "postgres" {
				query = query.Clauses(clause.Locking{Strength: "UPDATE"})
			}
			if err := query.Pluck("id", &fullAdminIDs).Error; err != nil {
				return err
			}
			remaining := 0
			for _, id := range fullAdminIDs {
				if id != user.ID {
					remaining++
				}
			}
			if remaining == 0 {
				return ErrLastActiveFullAdmin
			}
		}
		if err := tx.Save(user).Error; err != nil {
			return err
		}
		if err := replaceAdminPermissions(tx, user.ID, permissions, grantedBy, user.UpdatedAt); err != nil {
			return err
		}
		if revokeSessions {
			if err := tx.Delete(&model.AuthSession{}, "user_id = ?", user.ID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
