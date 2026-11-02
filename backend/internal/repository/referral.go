package repository

import (
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/referralcode"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrReferralCodeInvalid = errors.New("referral code invalid")
var ErrReferralReviewConflict = errors.New("referral reward already reviewed")

func ensureReferralProfileTx(tx *gorm.DB, userID string) (*model.ReferralProfile, error) {
	var profile model.ReferralProfile
	err := tx.First(&profile, "user_id = ?", userID).Error
	if err == nil {
		return &profile, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	for attempt := 0; attempt < 5; attempt++ {
		code, err := referralcode.New()
		if err != nil {
			return nil, err
		}
		profile = model.ReferralProfile{UserID: userID, Code: code}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&profile)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 1 {
			return &profile, nil
		}
		if err := tx.First(&profile, "user_id = ?", userID).Error; err == nil {
			return &profile, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return nil, errors.New("could not allocate referral code")
}

func (r *Repository) ReferralProfile(userID string) (*model.ReferralProfile, error) {
	var profile *model.ReferralProfile
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var err error
		profile, err = ensureReferralProfileTx(tx, userID)
		return err
	})
	return profile, err
}

func (r *Repository) ReferralProfileForInvitee(userID string) (*model.ReferralProfile, error) {
	var profile model.ReferralProfile
	err := r.db.First(&profile, "user_id = ?", userID).Error
	return &profile, err
}

func bindReferralTx(tx *gorm.DB, userID, code string) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil
	}
	if !referralcode.Valid(code) {
		return ErrReferralCodeInvalid
	}
	if err := lockReferralSettingTx(tx); err != nil {
		return err
	}
	policy, err := readReferralPolicyTx(tx)
	if err != nil {
		return err
	}
	if !policy.Enabled {
		return ErrReferralCodeInvalid
	}
	var inviter model.ReferralProfile
	if err := tx.First(&inviter, "code = ?", code).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReferralCodeInvalid
		}
		return err
	}
	if inviter.UserID == userID {
		return ErrReferralCodeInvalid
	}
	if inviter.Paused {
		return ErrReferralCodeInvalid
	}
	version := policy.VersionID
	if inviter.PolicyVersionID != "" {
		version = inviter.PolicyVersionID
	}
	boundPolicy, err := policyVersionTx(tx, version)
	if err != nil {
		return err
	}
	if !boundPolicy.Enabled {
		return ErrReferralCodeInvalid
	}
	var user model.User
	if err := tx.First(&user, "id = ? AND status = ?", inviter.UserID, model.UserStatusActive).Error; err != nil {
		return ErrReferralCodeInvalid
	}
	profile, err := ensureReferralProfileTx(tx, userID)
	if err != nil {
		return err
	}
	if profile.InviterID != "" {
		return ErrReferralCodeInvalid
	}
	updated := tx.Model(&model.ReferralProfile{}).Where("user_id = ? AND inviter_id = ''", userID).Updates(map[string]any{"inviter_id": inviter.UserID, "bound_policy_version_id": version, "updated_at": time.Now()})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrReferralCodeInvalid
	}
	return nil
}

type ReferralInviteeRecord struct {
	ID         string
	Email      string
	CreatedAt  time.Time
	PolicyJSON string
}

func (r *Repository) ReferralInvitees(inviterID string) ([]ReferralInviteeRecord, error) {
	var users []ReferralInviteeRecord
	err := r.db.Model(&model.User{}).Select("users.id, users.email, users.created_at, referral_policy_versions.policy_json").Joins("JOIN referral_profiles ON referral_profiles.user_id = users.id").Joins("LEFT JOIN referral_policy_versions ON referral_policy_versions.id = referral_profiles.bound_policy_version_id").
		Where("referral_profiles.inviter_id = ?", inviterID).Order("users.created_at desc").Limit(100).
		Scan(&users).Error
	return users, err
}

func (r *Repository) ReferralInviteeCount(inviterID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.ReferralProfile{}).Where("inviter_id = ?", inviterID).Count(&count).Error
	return count, err
}

func (r *Repository) ReferralRewards(inviterID, status string, limit, offset int) ([]model.ReferralReward, int64, error) {
	var rewards []model.ReferralReward
	var total int64
	query := r.db.Model(&model.ReferralReward{})
	if inviterID != "" {
		query = query.Where("inviter_id = ?", inviterID)
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&rewards).Error
	return rewards, total, err
}

func (r *Repository) HasPendingReferralReward(userID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.ReferralReward{}).Where("status = ? AND (inviter_id = ? OR invitee_id = ?)", model.ReferralRewardPending, userID, userID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) SetReferralRate(userID string, rate *int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if _, err := ensureReferralProfileTx(tx, userID); err != nil {
			return err
		}
		return tx.Model(&model.ReferralProfile{}).Where("user_id = ?", userID).Updates(map[string]any{"rate_bps": rate, "updated_at": time.Now()}).Error
	})
}

func (r *Repository) ReviewReferralReward(id, reviewerID string, approve bool, note string, audit *model.AdminAuditEvent) (*model.ReferralReward, error) {
	var reward model.ReferralReward
	err := r.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("id = ?", id)
		if r.Dialect() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&reward).Error; err != nil {
			return err
		}
		if reward.Status != model.ReferralRewardPending {
			return ErrReferralReviewConflict
		}
		status := model.ReferralRewardRejected
		if approve {
			status = model.ReferralRewardApproved
			var inviter model.User
			if err := tx.First(&inviter, "id = ? AND status = ?", reward.InviterID, model.UserStatusActive).Error; err != nil {
				return err
			}
			account := model.CreditAccount{UserID: reward.InviterID}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&account).Error; err != nil {
				return err
			}
			key := "referral:" + reward.PaymentOrderID
			entry := model.CreditLedgerEntry{ID: newRepositoryID(), UserID: reward.InviterID, Type: model.CreditLedgerReferral, AmountMicrocredits: reward.RewardMicrocredits, PaymentOrderID: reward.PaymentOrderID, ReferenceKey: &key, ActorUserID: reviewerID, Note: "邀请返利"}
			if err := tx.Create(&entry).Error; err != nil {
				return err
			}
			now := time.Now()
			updatedAccount := tx.Model(&model.CreditAccount{}).Where("user_id = ? AND available_microcredits <= ?", reward.InviterID, int64(9_000_000_000_000_000)-reward.RewardMicrocredits).
				Updates(map[string]any{"available_microcredits": gorm.Expr("available_microcredits + ?", reward.RewardMicrocredits), "version": gorm.Expr("version + 1"), "updated_at": now})
			if updatedAccount.Error != nil {
				return updatedAccount.Error
			}
			if updatedAccount.RowsAffected != 1 {
				return errors.New("referral credit balance overflow")
			}
			if err := tx.First(&account, "user_id = ?", reward.InviterID).Error; err != nil {
				return err
			}
			if err := tx.Model(&entry).Updates(map[string]any{"available_delta_microcredits": reward.RewardMicrocredits, "available_after_microcredits": account.AvailableMicrocredits, "reserved_after_microcredits": account.ReservedMicrocredits}).Error; err != nil {
				return err
			}
		}
		now := time.Now()
		updated := tx.Model(&model.ReferralReward{}).Where("id = ? AND status = ?", id, model.ReferralRewardPending).
			Updates(map[string]any{"status": status, "reviewed_by": reviewerID, "review_note": note, "reviewed_at": &now, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrReferralReviewConflict
		}
		if audit != nil {
			if err := tx.Create(audit).Error; err != nil {
				return err
			}
		}
		return tx.First(&reward, "id = ?", id).Error
	})
	return &reward, err
}
