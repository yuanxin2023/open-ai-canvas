package repository

import (
	"time"

	"infinite-canvas/backend/internal/model"
)

type AdminReferralRewardRecord struct {
	model.ReferralReward
	InviterUserID        *string
	InviterUsername      string
	InviterEmail         string
	InviteeUserID        *string
	InviteeUsername      string
	InviteeEmail         string
	OrderID              *string
	OrderUserID          string
	OrderMerchantOrderNo string
	OrderAmountFen       int64
	OrderCurrency        string
	OrderStatus          model.PaymentOrderStatus
	OrderCreditedAt      *time.Time
}

func (r *Repository) AdminReferralRewards(status string, limit, offset int) ([]AdminReferralRewardRecord, int64, error) {
	query := r.db.Model(&model.ReferralReward{})
	if status != "" && status != "all" {
		query = query.Where("referral_rewards.status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []AdminReferralRewardRecord
	err := query.Select(`referral_rewards.*,
		inviter.id AS inviter_user_id, inviter.username AS inviter_username, inviter.email AS inviter_email,
		invitee.id AS invitee_user_id, invitee.username AS invitee_username, invitee.email AS invitee_email,
		payment.id AS order_id, payment.user_id AS order_user_id, payment.merchant_order_no AS order_merchant_order_no,
		payment.amount_fen AS order_amount_fen, payment.currency AS order_currency,
		payment.status AS order_status, payment.credited_at AS order_credited_at`).
		Joins("LEFT JOIN users inviter ON inviter.id = referral_rewards.inviter_id").
		Joins("LEFT JOIN users invitee ON invitee.id = referral_rewards.invitee_id").
		Joins("LEFT JOIN payment_orders payment ON payment.id = referral_rewards.payment_order_id AND payment.user_id = referral_rewards.invitee_id").
		Order("referral_rewards.created_at DESC, referral_rewards.id ASC").Limit(limit).Offset(offset).Scan(&records).Error
	return records, total, err
}
