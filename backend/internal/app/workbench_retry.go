package app

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	"infinite-canvas/backend/internal/workbench/submission"

	"gorm.io/gorm"
)

type workbenchRetryReference struct{ TaskID, Version string }

type WorkbenchRetryIntentView struct {
	Task          *model.Task `json:"task"`
	RetryOf       string      `json:"retryOf"`
	RetryVersion  string      `json:"retryVersion"`
	ModelSelector string      `json:"modelSelector,omitempty"`
}

var workbenchRetryVersionPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (ref workbenchRetryReference) validate() error {
	if ref.TaskID == "" && ref.Version == "" {
		return nil
	}
	if strings.TrimSpace(ref.TaskID) == "" || len(ref.TaskID) > 160 || !workbenchRetryVersionPattern.MatchString(ref.Version) {
		return BadAuthRequest("历史重做需要完整的原任务与版本，请重新载入参数")
	}
	return nil
}

func taskWorkbenchKind(task *model.Task) string {
	if task.WorkbenchKind != "" {
		return task.WorkbenchKind
	}
	var input map[string]any
	if json.Unmarshal([]byte(task.InputJSON), &input) != nil {
		return ""
	}
	return workbenchKindFromTaskInput(input, task.Type)
}

func (s *Service) requireTaskWorkbenchFeature(task *model.Task) error {
	switch taskWorkbenchKind(task) {
	case model.WorkbenchKindImage:
		return s.RequireFeature(FeatureImageWorkbench)
	case model.WorkbenchKindVideo:
		return s.RequireFeature(FeatureVideoWorkbench)
	default:
		return nil
	}
}

func validateTaskRetryEligibility(task *model.Task, billing *taskBillingCoordinator) error {
	if task.MediaRecoveryJSON != "" {
		return BadAuthRequest("作品已生成，请使用重试保存，不要重复生成")
	}
	if task.CreationSubmissionID != nil {
		return creationConflict("智能创作重做需要新的报价批准，请回到创作会话继续")
	}
	if task.AgentRunID != "" || task.GenerationID != "" || strings.HasPrefix(task.Operation, "cloud_agent") || taskHasAgentOrigin(task.InputJSON) {
		return BadAuthRequest("Agent 重试需要新的幂等键和预算校验，请回到 Agent 对话重新发送")
	}
	if task.Status != model.TaskStatusFailed && task.Status != model.TaskStatusCancelled {
		return BadAuthRequest("只有失败或已取消的任务可以重试")
	}
	if task.ProviderCancelStatus == model.ProviderCancelStatusRequested {
		return BadAuthRequest("上游取消状态仍在确认中，请确认费用结果后再重试")
	}
	if err := billing.CheckRetryEligibility(task.BillingOrderID); err != nil {
		if errors.Is(err, errTaskBillingReview) {
			return BadAuthRequest("上一次调用费用仍在核对中，处理完成前不能重复提交")
		}
		return err
	}
	if isContentModerationFailure(task.Error) {
		return BadAuthRequest(contentModerationRetryMessage)
	}
	return nil
}

func (s *Service) WorkbenchRetryIntent(userID, id string) (*WorkbenchRetryIntentView, error) {
	if userID == "" {
		return nil, &AppError{Status: 401, Code: 401, Message: "请先登录"}
	}
	task, err := s.repo.TaskForUser(userID, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("任务不存在或无权访问")
	}
	if err != nil {
		return nil, err
	}
	kind := taskWorkbenchKind(task)
	if kind != model.WorkbenchKindImage && kind != model.WorkbenchKindVideo || task.ProjectID != "" {
		return nil, BadAuthRequest("请在原工作区处理该任务")
	}
	if err := s.requireTaskWorkbenchFeature(task); err != nil {
		return nil, err
	}
	if err := validateTaskRetryEligibility(task, s.taskBilling()); err != nil {
		return nil, err
	}
	version, err := submission.RetryVersion(task)
	if err != nil {
		return nil, err
	}
	return &WorkbenchRetryIntentView{Task: taskForOutput(*task), RetryOf: task.ID, RetryVersion: version, ModelSelector: workbenchRetryModelSelector(task)}, nil
}

// System model names can contain the same separators as browser selectors.
// Only an original custom-channel selector is a safe restoration hint; managed
// models use their public logical ID and never expose the supplying channel.
func workbenchRetryModelSelector(task *model.Task) string {
	if task.LogicalModelID != "" {
		return ""
	}
	var input map[string]any
	if json.Unmarshal([]byte(task.InputJSON), &input) != nil {
		return ""
	}
	config, ok := input["config"].(map[string]any)
	if !ok || strings.TrimSpace(stringValue(config["channelId"])) != "" {
		return ""
	}
	rawModel := strings.TrimSpace(stringValue(config["model"]))
	if rawModel == "" || rawModel == task.Model || !strings.HasSuffix(task.Model, "::"+rawModel) || len(task.Model) <= len(rawModel)+2 {
		return ""
	}
	return task.Model
}

func validateWorkbenchRetrySource(repo *repository.Repository, userID, kind string, ref workbenchRetryReference, task *model.Task) error {
	if task == nil || task.UserID != userID || task.ID != ref.TaskID || task.ProjectID != "" || taskWorkbenchKind(task) != kind {
		return workbenchRetryChanged()
	}
	version, err := submission.RetryVersion(task)
	if err != nil {
		return err
	}
	if version != ref.Version {
		return workbenchRetryChanged()
	}
	return validateTaskRetryEligibility(task, newTaskBillingCoordinator(repo))
}

func workbenchRetryChanged() error {
	return &AppError{Status: 409, Code: 409, Reason: kernel.ReasonWorkbenchRetryChanged, Message: "原任务状态已变化，请重新载入并核对后再生成"}
}

func saveWorkbenchRetry(repo *repository.Repository, userID, kind string, ref workbenchRetryReference, save func() error) error {
	source, err := repo.TaskForUser(userID, ref.TaskID)
	if err != nil {
		return err
	}
	if source.BillingOrderID != "" {
		if err := repo.LockWorkbenchRetryBillingOrder(userID, source.BillingOrderID); err != nil {
			return err
		}
	}
	if err := validateWorkbenchRetrySource(repo, userID, kind, ref, source); err != nil {
		return err
	}
	if err := save(); err != nil {
		return err
	}
	source, err = repo.LockWorkbenchRetryTask(userID, ref.TaskID)
	if err != nil {
		return err
	}
	return validateWorkbenchRetrySource(repo, userID, kind, ref, source)
}
