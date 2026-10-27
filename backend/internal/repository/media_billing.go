package repository

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
)

type BillingMediaReview struct {
	ActorUserID string
	Note        string
}

func validBillingMediaReview(review *BillingMediaReview) bool {
	return review != nil && strings.TrimSpace(review.ActorUserID) != "" && strings.TrimSpace(review.Note) != "" && len([]rune(review.Note)) <= 500
}

func recordMediaBillingReview(tx *gorm.DB, order model.BillingOrder, durationMs int64, charge measuredMediaCharge, review *BillingMediaReview) error {
	if !validBillingMediaReview(review) {
		return ErrBillingStateConflict
	}
	metadata, err := json.Marshal(map[string]any{
		"billingOrderId": order.ID, "taskId": order.TaskID, "note": review.Note,
		"durationMs": durationMs, "quantity": charge.Quantity,
		"measuredMicrocredits": charge.MeasuredAmount, "chargedMicrocredits": charge.Amount,
		"authorizationMicrocredits": charge.Limit, "platformCoveredMicrocredits": charge.MeasuredAmount - charge.Amount,
		"refundRecovery": order.RefundRecoveryPending || order.Status == model.BillingStatusRefunded,
	})
	if err != nil {
		return err
	}
	if err := tx.Create(&model.AdminAuditEvent{ID: newRepositoryID(), ActorUserID: review.ActorUserID, Action: "billing.media_review.settle", TargetType: "user", TargetID: order.UserID, Summary: "人工核对实测媒体计费", MetadataJSON: string(metadata), CreatedAt: time.Now()}).Error; err != nil {
		return err
	}
	return tx.Model(&model.BillingOrder{}).Where("id = ?", order.ID).Updates(map[string]any{"resolved_by": review.ActorUserID, "resolution_note": review.Note, "error": ""}).Error
}

func (r *Repository) SettleBillingOrderAfterMediaReview(id, providerRequestID string, durationMs int64, review BillingMediaReview) error {
	if !validBillingMediaReview(&review) {
		return ErrBillingStateConflict
	}
	return r.settleBillingOrder(id, providerRequestID, &durationMs, &review)
}

func (r *Repository) RestoreRefundedBillingOrderWithAudioDuration(id, providerRequestID string, durationMs int64) error {
	return r.restoreRefundedBillingOrder(id, providerRequestID, &durationMs, nil)
}

func (r *Repository) RestoreRefundedBillingOrderAfterMediaReview(id, providerRequestID string, durationMs int64, review BillingMediaReview) error {
	if !validBillingMediaReview(&review) {
		return ErrBillingStateConflict
	}
	return r.restoreRefundedBillingOrder(id, providerRequestID, &durationMs, &review)
}

func (r *Repository) MarkRefundedBillingRecoveryReview(id, providerRequestID string, cause error, measuredAmount *int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		order, err := lockBillingOrder(tx, id)
		if err != nil {
			return err
		}
		if order.Status == model.BillingStatusSettled {
			return nil
		}
		if order.Status != model.BillingStatusRefunded && !(order.Status == model.BillingStatusUncertain && order.RefundRecoveryPending) {
			return ErrBillingStateConflict
		}
		reason := "退款后恢复扣费尚未完成，需人工核对"
		if errors.Is(cause, ErrBillingChargeLimit) {
			reason = "实测费用超过原授权，退款后恢复扣费待核对"
		}
		if errors.Is(cause, ErrInsufficientCredits) {
			reason = "可用积分不足，退款后恢复扣费待核对"
		}
		if errors.Is(cause, ErrMeasuredDurationUnavailable) {
			reason = "实测时长不可用，退款后恢复扣费待核对"
		}
		updates := map[string]any{"status": model.BillingStatusUncertain, "refund_recovery_pending": true, "error": reason, "updated_at": time.Now()}
		if providerRequestID != "" {
			updates["provider_request_id"] = providerRequestID
		}
		if measuredAmount != nil && *measuredAmount >= 0 {
			updates["actual_amount_microcredits"] = *measuredAmount
		}
		return tx.Model(&model.BillingOrder{}).Where("id = ?", order.ID).Updates(updates).Error
	})
}
