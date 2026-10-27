package app

import (
	"errors"
	"net/http"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func (s *Service) measuredBillingDuration(order model.BillingOrder, task model.Task) (int64, bool, error) {
	if order.BillingMode != "per_second" || (order.Capability != "audio" && order.Capability != "video") {
		return 0, false, nil
	}
	if order.UserID == "" || order.TaskID == "" || order.UserID != task.UserID || order.TaskID != task.ID || task.BillingOrderID != order.ID {
		return 0, true, repository.ErrBillingStateConflict
	}
	if order.Capability == "audio" {
		duration, err := s.AudioOutputDurationMs(task)
		if err != nil || duration <= 0 {
			return 0, true, errors.Join(repository.ErrMeasuredDurationUnavailable, err)
		}
		return duration, true, nil
	}
	duration, handled, err := s.VideoProcessingOutputDurationMs(task)
	if err != nil || (handled && duration <= 0) || (!handled && order.Scene == VideoProcessingOperationEnhance) {
		return 0, true, errors.Join(repository.ErrMeasuredDurationUnavailable, err)
	}
	return duration, handled, nil
}

func (s *Service) settleMediaBillingReview(actor *model.User, order model.BillingOrder, note string) (bool, error) {
	if order.BillingMode != "per_second" || (order.Capability != "audio" && order.Capability != "video") {
		return false, nil
	}
	if order.TaskID == "" && order.Capability == "video" && order.Scene != VideoProcessingOperationEnhance {
		return false, nil
	}
	task, err := s.repo.TaskForUser(order.UserID, order.TaskID)
	if err != nil {
		return true, WrapAppError(http.StatusConflict, "无法核实任务产物，请补齐依据后再结算", err)
	}
	duration, handled, err := s.measuredBillingDuration(order, *task)
	if !handled {
		return false, err
	}
	if err != nil {
		return true, WrapAppError(http.StatusConflict, "无法核实实际时长，请补齐依据或退回预授权", err)
	}
	review := repository.BillingMediaReview{ActorUserID: actor.ID, Note: note}
	if order.RefundRecoveryPending {
		err = s.repo.RestoreRefundedBillingOrderAfterMediaReview(order.ID, order.ProviderRequestID, duration, review)
	} else {
		err = s.repo.SettleBillingOrderAfterMediaReview(order.ID, order.ProviderRequestID, duration, review)
	}
	if errors.Is(err, repository.ErrInsufficientCredits) {
		return true, InsufficientCredits()
	}
	if errors.Is(err, repository.ErrBillingStateConflict) || errors.Is(err, repository.ErrBillingChargeLimit) || errors.Is(err, repository.ErrMeasuredDurationUnavailable) {
		return true, WrapAppError(http.StatusConflict, "计费状态或实测依据发生变化，请刷新后核对", err)
	}
	return true, err
}
