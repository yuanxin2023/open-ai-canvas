package repository

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/referrals"
)

func readReferralPolicyTx(tx *gorm.DB) (referrals.Policy, error) {
	var setting model.SystemSetting
	err := tx.First(&setting, "key = ?", referrals.SettingKey).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return referrals.Default(), nil
	}
	if err != nil {
		return referrals.Policy{}, err
	}
	var p referrals.Policy
	if err := json.Unmarshal([]byte(setting.ValueJSON), &p); err != nil {
		return p, err
	}
	return p, p.Validate()
}

func policyVersionTx(tx *gorm.DB, id string) (referrals.Policy, error) {
	var version model.ReferralPolicyVersion
	if id == "" {
		return referrals.Policy{}, errors.New("邀请关系缺少规则版本")
	}
	if err := tx.First(&version, "id = ?", id).Error; err != nil {
		return referrals.Policy{}, err
	}
	var p referrals.Policy
	if err := json.Unmarshal([]byte(version.PolicyJSON), &p); err != nil {
		return p, err
	}
	return p, p.Validate()
}

func (r *Repository) ReferralPolicyVersion(id string) (referrals.Policy, error) {
	return policyVersionTx(r.db, id)
}

// Policy publishing and registration lock the same setting row to pin a complete version.
func lockReferralSettingTx(tx *gorm.DB) error {
	query := tx.Model(&model.SystemSetting{}).Where("key = ?", referrals.SettingKey)
	if tx.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var setting model.SystemSetting
	return query.First(&setting).Error
}

func (r *Repository) SaveReferralPolicy(p referrals.Policy, userID string, paused *bool, actor string, audit *model.AdminAuditEvent) (referrals.Policy, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockReferralSettingTx(tx); err != nil {
			return err
		}
		if userID != "" {
			if _, err := ensureReferralProfileTx(tx, userID); err != nil {
				return err
			}
		}
		if paused != nil {
			if err := tx.Model(&model.ReferralProfile{}).Where("user_id = ?", userID).Update("paused", *paused).Error; err != nil {
				return err
			}
		}
		p.VersionID = newRepositoryID()
		metadata, err := json.Marshal(map[string]any{"policy": p, "userId": userID, "paused": paused})
		if err != nil {
			return err
		}
		audit.MetadataJSON = string(metadata)
		encoded, err := json.Marshal(p)
		if err != nil {
			return err
		}
		if err := tx.Create(&model.ReferralPolicyVersion{ID: p.VersionID, PolicyJSON: string(encoded), CreatedBy: actor}).Error; err != nil {
			return err
		}
		if userID == "" {
			if err := tx.Model(&model.SystemSetting{}).Where("key = ?", referrals.SettingKey).Updates(map[string]any{"value_json": string(encoded), "updated_by": actor, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&model.ReferralProfile{}).Where("user_id = ?", userID).Updates(map[string]any{"policy_version_id": p.VersionID, "rate_bps": nil}).Error; err != nil {
			return err
		}
		return tx.Create(audit).Error
	})
	return p, err
}

func (r *Repository) SaveReferralUserPolicy(userID string, policy *referrals.Policy, paused bool, actor string, audit *model.AdminAuditEvent) error {
	if policy == nil {
		return r.UpdateReferralProfileControls(userID, paused, true, audit)
	}
	_, err := r.SaveReferralPolicy(*policy, userID, &paused, actor, audit)
	return err
}

func (r *Repository) UpdateReferralProfileControls(userID string, paused bool, clearPolicy bool, audit *model.AdminAuditEvent) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockReferralSettingTx(tx); err != nil {
			return err
		}
		if _, err := ensureReferralProfileTx(tx, userID); err != nil {
			return err
		}
		updates := map[string]any{"paused": paused}
		if clearPolicy {
			updates["policy_version_id"] = ""
			updates["rate_bps"] = nil
		}
		if err := tx.Model(&model.ReferralProfile{}).Where("user_id = ?", userID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(audit).Error
	})
}

// The caller holds the payment transaction. Serializing on the inviter protects
// both per-invitee caps and the first successful recharge across different orders.
func settleVersionedReferralTx(tx *gorm.DB, order *model.PaymentOrder, now time.Time, evaluate ReferralEvaluator) error {
	var invitee model.ReferralProfile
	err := tx.First(&invitee, "user_id = ?", order.UserID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if invitee.InviterID == "" {
		return nil
	}
	if invitee.InviterID == order.UserID {
		return errors.New("邀请关系不能指向本人")
	}
	var inviter model.ReferralProfile
	query := tx.Where("user_id = ?", invitee.InviterID)
	if tx.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&inviter).Error; err != nil {
		return err
	}
	global, err := readReferralPolicyTx(tx)
	if err != nil {
		return err
	}
	if !global.Enabled || inviter.Paused {
		return nil
	}
	var active int64
	if err := tx.Model(&model.User{}).Where("id = ? AND status = ?", inviter.UserID, model.UserStatusActive).Count(&active).Error; err != nil {
		return err
	}
	if active != 1 {
		return nil
	}
	p, err := policyVersionTx(tx, invitee.BoundPolicyVersionID)
	if err != nil {
		return err
	}
	orders := tx.Model(&model.PaymentOrder{}).Where("user_id = ? AND status = ? AND product_kind <> ? AND amount_fen > 0 AND currency = ?", order.UserID, model.PaymentOrderCredited, model.TopupProductKindStorage, "CNY")
	var ordinal int64
	if err := orders.Count(&ordinal).Error; err != nil {
		return err
	}
	var firstOrder model.PaymentOrder
	if err := orders.Select("id", "credited_at").Order("credited_at asc, id asc").First(&firstOrder).Error; err != nil {
		return err
	}
	if firstOrder.CreditedAt == nil {
		return errors.New("成功充值订单缺少入账时间")
	}
	first := *firstOrder.CreditedAt
	var used int64
	if err := tx.Model(&model.ReferralReward{}).Where("inviter_id = ? AND invitee_id = ? AND status <> ?", inviter.UserID, invitee.UserID, model.ReferralRewardRejected).Select("COALESCE(SUM(reward_microcredits),0)").Row().Scan(&used); err != nil {
		return err
	}
	result, err := evaluate(p, order.CreditsMicrocredits, ordinal, used, first, now)
	if err != nil {
		return err
	}
	if err := tx.Model(&model.PaymentOrder{}).Where("id = ?", order.ID).Updates(map[string]any{"referral_policy_version_id": p.VersionID, "newcomer_bonus_microcredits": result.Newcomer}).Error; err != nil {
		return err
	}
	if result.Newcomer > 0 {
		bonusOrder := *order
		bonusOrder.CreditsMicrocredits = result.Newcomer
		if err := creditPaymentTopup(tx, &bonusOrder, "referral-newcomer:"+order.ID, now); err != nil {
			return err
		}
		if err := tx.Model(&model.CreditLedgerEntry{}).Where("reference_key = ?", "referral-newcomer:"+order.ID).Updates(map[string]any{"type": model.CreditLedgerReferral, "note": "邀请新人首充赠送"}).Error; err != nil {
			return err
		}
	}
	if result.Reward <= 0 {
		return nil
	}
	return tx.Create(&model.ReferralReward{ID: newRepositoryID(), PaymentOrderID: order.ID, InviterID: inviter.UserID, InviteeID: order.UserID, AmountFen: order.AmountFen, PolicyVersionID: p.VersionID, BaseMicrocredits: order.CreditsMicrocredits, OrderOrdinal: ordinal, StageIndex: result.Stage, RateBPS: result.RateBPS, UncappedMicrocredits: result.Uncapped, RewardMicrocredits: result.Reward, Status: model.ReferralRewardPending}).Error
}

type ReferralEvaluator func(referrals.Policy, int64, int64, int64, time.Time, time.Time) (referrals.Result, error)

func (r *Repository) ReferralNewcomerEligible(userID, inviterID string) (bool, error) {
	p, err := readReferralPolicyTx(r.db)
	if err != nil || !p.Enabled {
		return false, err
	}
	var count int64
	if err := r.db.Model(&model.ReferralProfile{}).Joins("JOIN users ON users.id = referral_profiles.user_id").Where("referral_profiles.user_id = ? AND referral_profiles.paused = ? AND users.status = ?", inviterID, false, model.UserStatusActive).Count(&count).Error; err != nil {
		return false, err
	}
	if count != 1 {
		return false, nil
	}
	if err := r.db.Model(&model.PaymentOrder{}).Where("user_id = ? AND status = ? AND product_kind <> ? AND amount_fen > 0 AND currency = ?", userID, model.PaymentOrderCredited, model.TopupProductKindStorage, "CNY").Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func (r *Repository) CompletePaymentOrderWithReferral(providerID, merchantOrderNo string, evidence PaymentEvidence, evaluate ReferralEvaluator) (*model.PaymentOrder, bool, error) {
	return r.completePaymentOrder(providerID, merchantOrderNo, evidence, evaluate)
}
