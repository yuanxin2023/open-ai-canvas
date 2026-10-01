package repository

import (
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrUsernameChangeLimit = errors.New("username change limit reached")

type UsernameChangeUsage struct {
	Used            int64
	NextAvailableAt *time.Time
}

type UpdateUserProfileInput struct {
	UserID           string
	Username         string
	AvatarResourceID *string
	ChangeID         string
	Now              time.Time
	Window           time.Duration
	Limit            int64
}

func (r *Repository) UsernameChangeUsage(userID string, since time.Time, limit int64) (UsernameChangeUsage, error) {
	return usernameChangeUsage(r.db, userID, since, limit)
}

func usernameChangeUsage(db *gorm.DB, userID string, since time.Time, limit int64) (UsernameChangeUsage, error) {
	var usage UsernameChangeUsage
	query := db.Model(&model.UserUsernameChange{}).
		Where("user_id = ? AND counts_toward_limit = ? AND created_at > ?", userID, true, since)
	if err := query.Count(&usage.Used).Error; err != nil {
		return usage, err
	}
	if usage.Used >= limit {
		var oldest model.UserUsernameChange
		if err := db.Where("user_id = ? AND counts_toward_limit = ? AND created_at > ?", userID, true, since).Order("created_at asc").First(&oldest).Error; err != nil {
			return usage, err
		}
		next := oldest.CreatedAt.Add(30 * 24 * time.Hour)
		usage.NextAvailableAt = &next
	}
	return usage, nil
}

func (r *Repository) UpdateUserProfile(input UpdateUserProfileInput) (*model.User, UsernameChangeUsage, error) {
	var stored model.User
	var usage UsernameChangeUsage
	err := r.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("id = ?", input.UserID)
		if r.Dialect() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&stored).Error; err != nil {
			return err
		}
		changed := !strings.EqualFold(stored.Username, input.Username)
		counts := changed && stored.Role != model.UserRoleAdmin && stored.UsernameCustomizedAt != nil
		if counts {
			current, err := usernameChangeUsage(tx, stored.ID, input.Now.Add(-input.Window), input.Limit)
			if err != nil {
				return err
			}
			if current.Used >= input.Limit {
				usage = current
				return ErrUsernameChangeLimit
			}
		}

		updates := map[string]any{"updated_at": input.Now}
		if input.AvatarResourceID != nil {
			updates["avatar_resource_id"] = *input.AvatarResourceID
		}
		if changed {
			oldUsername := stored.Username
			updates["username"] = input.Username
			updates["display_name"] = input.Username
			updates["profile_name"] = input.Username
			updates["username_customized_at"] = input.Now
			if err := tx.Model(&model.User{}).Where("id = ?", stored.ID).Updates(updates).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.UserUsernameChange{
				ID: input.ChangeID, UserID: stored.ID, OldUsername: oldUsername, NewUsername: input.Username,
				CountsTowardLimit: counts, CreatedAt: input.Now,
			}).Error; err != nil {
				return err
			}
		} else if len(updates) > 1 {
			if err := tx.Model(&model.User{}).Where("id = ?", stored.ID).Updates(updates).Error; err != nil {
				return err
			}
		}
		if err := tx.First(&stored, "id = ?", stored.ID).Error; err != nil {
			return err
		}
		var err error
		usage, err = usernameChangeUsage(tx, stored.ID, input.Now.Add(-input.Window), input.Limit)
		return err
	})
	return &stored, usage, err
}
