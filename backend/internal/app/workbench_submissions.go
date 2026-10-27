package app

import (
	"encoding/json"
	"errors"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	"infinite-canvas/backend/internal/workbench/submission"

	"gorm.io/gorm"
)

type workbenchTaskPreparation struct {
	Order  *model.BillingOrder
	Policy RuntimePolicySetting
	Retry  workbenchRetryReference
}

func (s *Service) createIdempotentWorkbenchTask(userID string, req WorkbenchTaskInput, traceID, requestID string) (*model.Task, error) {
	key := req.RequestKey
	req.RequestKey = ""
	hash, err := submission.Fingerprint("workbench", req)
	if err != nil {
		return nil, BadAuthRequest("工作台请求无法编码")
	}
	return s.commitWorkbenchTask(userID, key, hash, req.Kind, workbenchRetryReference{TaskID: req.RetryOf, Version: req.RetryVersion}, func(prepared *workbenchTaskPreparation) (*model.Task, error) {
		return s.createWorkbenchTask(userID, req, traceID, requestID, prepared)
	})
}

func (s *Service) createIdempotentWorkbenchDirectTask(userID string, req CreateTaskRequest) (*model.Task, error) {
	if req.WorkbenchExpectedUserID != "" && req.WorkbenchExpectedUserID != userID {
		return nil, Forbidden("登录账号已变化，请核对当前账号后重新操作")
	}
	kind := workbenchKindFromTaskInput(req.Input, req.Type)
	if kind == "" || req.ProjectID != "" || req.admission != nil || req.creationPrepare != nil || req.workbenchPreparation != nil {
		return nil, BadAuthRequest("请求标识仅用于独立工作台任务")
	}
	key := req.WorkbenchRequestKey
	req.WorkbenchRequestKey = ""
	hash, err := submission.Fingerprint("workbench-direct", req)
	if err != nil {
		return nil, BadAuthRequest("工作台请求无法编码")
	}
	return s.commitWorkbenchTask(userID, key, hash, kind, workbenchRetryReference{TaskID: req.WorkbenchRetryOf, Version: req.WorkbenchRetryVersion}, func(prepared *workbenchTaskPreparation) (*model.Task, error) {
		req.workbenchPreparation = prepared
		return s.CreateTask(userID, req)
	})
}

func (s *Service) commitWorkbenchTask(userID, key, hash, kind string, retry workbenchRetryReference, prepare func(*workbenchTaskPreparation) (*model.Task, error)) (*model.Task, error) {
	if userID == "" {
		return nil, &AppError{Status: 401, Code: 401, Message: "请先登录"}
	}
	if err := submission.ValidateKey(key); err != nil {
		return nil, BadAuthRequest(err.Error())
	}
	if err := retry.validate(); err != nil {
		return nil, err
	}
	if old, err := s.repo.WorkbenchSubmission(userID, key); err == nil {
		if old.RequestHash != hash || old.Kind != kind {
			return nil, workbenchSubmissionConflict()
		}
		return s.workbenchSubmissionTask(old, true)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	prepared := &workbenchTaskPreparation{Retry: retry}
	task, err := func() (*model.Task, error) {
		if retry.TaskID != "" {
			source, err := s.repo.TaskForUser(userID, retry.TaskID)
			if err != nil {
				return nil, err
			}
			if err := validateWorkbenchRetrySource(s.repo, userID, kind, retry, source); err != nil {
				return nil, err
			}
		}
		return prepare(prepared)
	}()
	if err != nil {
		// A concurrent winner may have consumed the remaining quota or credits.
		if old, lookupErr := s.repo.WorkbenchSubmission(userID, key); lookupErr == nil {
			if old.RequestHash != hash || old.Kind != kind {
				return nil, workbenchSubmissionConflict()
			}
			return s.workbenchSubmissionTask(old, true)
		}
		return nil, err
	}
	quote := model.WorkbenchSubmissionQuote{}
	if order := prepared.Order; order != nil {
		quote = model.WorkbenchSubmissionQuote{BillingEnabled: true, AmountMicrocredits: order.AmountMicrocredits, BillingMode: order.BillingMode, PriceTierVersion: order.PriceTierVersion, PriceVersion: order.PriceVersion, Quantity: order.Quantity}
		if order.ChargeLimitSet {
			limit := order.ChargeLimitMicrocredits
			quote.ChargeLimitMicrocredits = &limit
		}
	}
	encoded, err := json.Marshal(quote)
	if err != nil {
		return nil, err
	}
	record := &model.WorkbenchSubmission{UserID: userID, RequestKey: key, RequestHash: hash, Kind: kind, TaskID: task.ID, QuoteJSON: string(encoded)}
	s.storageMu.Lock()
	stored, replayed, err := s.repo.CommitWorkbenchSubmission(record, func(repo *repository.Repository) error {
		if retry.TaskID != "" {
			return saveWorkbenchRetry(repo, userID, kind, retry, func() error { return createTaskWithStorageQuotaRepository(repo, task, prepared.Order, prepared.Policy) })
		}
		return createTaskWithStorageQuotaRepository(repo, task, prepared.Order, prepared.Policy)
	})
	s.storageMu.Unlock()
	if errors.Is(err, repository.ErrWorkbenchSubmissionConflict) {
		return nil, workbenchSubmissionConflict()
	}
	if replayed && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, workbenchSubmissionTaskRemoved()
	}
	if errors.Is(err, repository.ErrActiveTaskLimit) {
		return nil, BadAuthRequest("同时排队或运行的任务已达上限，请等待已有任务完成")
	}
	if errors.Is(err, repository.ErrInsufficientCredits) {
		return nil, InsufficientCredits()
	}
	if errors.Is(err, repository.ErrLogicalModelUnavailable) {
		return nil, BadAuthRequest("所选模型已停用、归档或配置已更新，请重新选择")
	}
	if err != nil {
		return nil, err
	}
	if !replayed {
		s.wakeTaskDispatcher()
		s.recordActivity(userID, "task", 1)
		_ = s.log(userID, stored.ID, "info", "工作台任务已进入队列", "")
	}
	return workbenchTaskWithReceipt(stored, record, replayed)
}

func (s *Service) WorkbenchSubmission(userID, key string) (*model.Task, error) {
	if userID == "" {
		return nil, &AppError{Status: 401, Code: 401, Message: "请先登录"}
	}
	if err := submission.ValidateKey(key); err != nil {
		return nil, BadAuthRequest(err.Error())
	}
	record, err := s.repo.WorkbenchSubmission(userID, key)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &AppError{Status: 404, Code: 404, Reason: kernel.ReasonWorkbenchSubmissionMissing, Message: "尚未查到该请求的接单回执，请保留原请求标识"}
	}
	if err != nil {
		return nil, err
	}
	return s.workbenchSubmissionTask(record, true)
}

func (s *Service) workbenchSubmissionTask(record *model.WorkbenchSubmission, replayed bool) (*model.Task, error) {
	task, err := s.repo.TaskForUser(record.UserID, record.TaskID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, workbenchSubmissionTaskRemoved()
	}
	if err != nil {
		return nil, err
	}
	return workbenchTaskWithReceipt(task, record, replayed)
}

func workbenchTaskWithReceipt(task *model.Task, record *model.WorkbenchSubmission, replayed bool) (*model.Task, error) {
	var quote model.WorkbenchSubmissionQuote
	if err := json.Unmarshal([]byte(record.QuoteJSON), &quote); err != nil {
		return nil, err
	}
	out := taskForOutput(*task)
	out.WorkbenchSubmission = &model.WorkbenchSubmissionReceipt{RequestKey: record.RequestKey, Replayed: replayed, Quote: quote}
	return out, nil
}

func workbenchSubmissionConflict() error {
	return &AppError{Status: 409, Code: 409, Reason: kernel.ReasonWorkbenchSubmissionConflict, Message: "该请求标识已用于不同参数，请先核对原任务"}
}

func workbenchSubmissionTaskRemoved() error {
	return &AppError{Status: 410, Code: 410, Reason: kernel.ReasonWorkbenchSubmissionTaskRemoved, Message: "原请求已经接单，任务记录已移除，不能复用请求标识创建新任务"}
}
