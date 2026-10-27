package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"infinite-canvas/backend/internal/assets"
	feedbackcontract "infinite-canvas/backend/internal/feedback"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

type SubmitFeedbackRequest struct {
	RequestID   string             `json:"requestId"`
	Type        model.FeedbackType `json:"type"`
	Title       string             `json:"title"`
	Content     string             `json:"content"`
	Attachments string             `json:"attachments"`
	Environment string             `json:"environment"`
	Contact     string             `json:"contact"`
}

type AdminUpdateFeedbackRequest struct {
	ExpectedVersion int64                `json:"-"`
	Status          model.FeedbackStatus `json:"status"`
	AdminReply      *string              `json:"adminReply"`
	Archived        *bool                `json:"archived"`
}

type FeedbackListResult struct {
	List  []model.UserFeedback `json:"list"`
	Total int64                `json:"total"`
	Page  int                  `json:"page"`
	Limit int                  `json:"limit"`
}

func feedbackChanged() error {
	return &kernel.AppError{Status: http.StatusConflict, Code: http.StatusConflict, Reason: kernel.ReasonFeedbackChanged, Message: "反馈已更新，请保留回复草稿，刷新核对后再操作"}
}

func (s *Service) SubmitFeedback(actor *model.User, req SubmitFeedbackRequest) (*model.UserFeedback, error) {
	if actor == nil || actor.ID == "" {
		return nil, Unauthorized("请先登录后提交反馈")
	}
	if err := s.RequireFeature(FeatureUserFeedback); err != nil {
		return nil, err
	}
	if !feedbackcontract.ValidRequestID(req.RequestID) {
		return nil, kernel.BadAuthRequest("反馈请求标识无效")
	}
	title, content, contact := strings.TrimSpace(req.Title), strings.TrimSpace(req.Content), strings.TrimSpace(req.Contact)
	if title == "" || utf8.RuneCountInString(title) > 200 {
		return nil, kernel.BadAuthRequest("反馈标题须为 1–200 个字")
	}
	if content == "" || utf8.RuneCountInString(content) > 10000 {
		return nil, kernel.BadAuthRequest("反馈内容须为 1–10000 个字")
	}
	if utf8.RuneCountInString(contact) > 256 {
		return nil, kernel.BadAuthRequest("联系方式不能超过 256 个字")
	}
	switch req.Type {
	case model.FeedbackTypeBug, model.FeedbackTypeFeature, model.FeedbackTypeImprovement, model.FeedbackTypeOther:
	default:
		return nil, kernel.BadAuthRequest("反馈类型无效")
	}
	env, err := feedbackcontract.NormalizeEnvironment(req.Environment)
	if err != nil {
		return nil, kernel.BadAuthRequest(err.Error())
	}
	var attachments []model.FeedbackAttachment
	raw := strings.TrimSpace(req.Attachments)
	if raw == "" {
		raw = "[]"
	}
	if len(raw) > 32*1024 || !strings.HasPrefix(raw, "[") || json.Unmarshal([]byte(raw), &attachments) != nil || len(attachments) > 5 {
		return nil, kernel.BadAuthRequest("附件列表无效，最多选择 5 个附件")
	}
	id := feedbackcontract.SubmissionID(actor.ID, req.RequestID)
	seen := make(map[string]bool)
	for i := range attachments {
		item := &attachments[i]
		if item.ID == "" || seen[item.ID] || (item.ResourceID != "" && item.ResourceID != item.ID) {
			return nil, kernel.BadAuthRequest("附件标识无效或重复")
		}
		seen[item.ID] = true
		resource, err := s.repo.ResourceForUser(actor.ID, item.ID)
		if err != nil {
			return nil, kernel.BadAuthRequest("附件不存在或不属于当前账号")
		}
		if err := validateFeedbackResource(resource); err != nil {
			return nil, err
		}
		name := filepath.Base(strings.ReplaceAll(strings.TrimSpace(item.Name), "\\", "/"))
		if name == "." || name == "" {
			name = "附件"
		}
		if utf8.RuneCountInString(name) > 240 {
			return nil, kernel.BadAuthRequest("附件名称过长")
		}
		*item = model.FeedbackAttachment{ID: resource.ID, ResourceID: resource.ID, Name: name, Type: resource.Kind, URL: "/api/feedbacks/" + id + "/attachments/" + resource.ID, Size: resource.Size, MimeType: resource.MimeType}
	}
	if attachments == nil {
		attachments = []model.FeedbackAttachment{}
	}
	encoded, err := json.Marshal(attachments)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	fb := &model.UserFeedback{ID: id, Version: 1, UserID: actor.ID, UserName: actor.Username, UserEmail: actor.Email, Contact: contact, Type: req.Type, Title: title, Content: content, Attachments: string(encoded), Environment: env, Status: model.FeedbackStatusPending, CreatedAt: now, UpdatedAt: now}
	resourceIDs := make([]string, 0, len(attachments))
	for _, item := range attachments {
		resourceIDs = append(resourceIDs, item.ID)
	}
	saved, err := s.repo.CreateFeedbackWithResources(fb, resourceIDs)
	if err != nil {
		return nil, fmt.Errorf("保存反馈失败：%w", err)
	}
	// Replays compare immutable submission fields, independently of admin changes.
	if saved.UserID != fb.UserID || saved.Type != fb.Type || saved.Title != fb.Title || saved.Content != fb.Content || saved.Contact != fb.Contact || saved.Attachments != fb.Attachments || saved.Environment != fb.Environment {
		return nil, &kernel.AppError{Status: http.StatusConflict, Code: http.StatusConflict, Reason: kernel.ReasonFeedbackSubmissionConflict, Message: "此请求已提交其他内容，请先检查我的反馈，确认后再新建"}
	}
	return saved, nil
}

func validateFeedbackResource(resource *model.Resource) error {
	if resource == nil || resource.Status != model.ResourceStatusReady || resource.Size <= 0 || resource.Size > 50*1024*1024 {
		return kernel.BadAuthRequest("附件未就绪或超过 50MB")
	}
	mime := strings.ToLower(strings.TrimSpace(strings.Split(resource.MimeType, ";")[0]))
	if resource.Kind != "image" && resource.Kind != "video" || !strings.HasPrefix(mime, resource.Kind+"/") {
		return kernel.BadAuthRequest("附件必须是图片或视频")
	}
	return nil
}

func (s *Service) MyFeedbacks(actor *model.User) ([]model.UserFeedback, error) {
	if actor == nil || actor.ID == "" {
		return nil, Unauthorized("请先登录")
	}
	return s.repo.ListFeedbacksByUser(actor.ID)
}

func (s *Service) UploadFeedbackAttachment(actor *model.User, header *multipart.FileHeader) (*model.FeedbackAttachment, error) {
	if actor == nil || actor.ID == "" {
		return nil, Unauthorized("请先登录")
	}
	if err := s.RequireFeature(FeatureUserFeedback); err != nil {
		return nil, err
	}
	if header == nil || header.Size <= 0 || header.Size > 50*1024*1024 {
		return nil, kernel.BadAuthRequest("请选择不超过 50MB 的图片或视频")
	}
	var kind string
	switch strings.ToLower(filepath.Ext(header.Filename)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp":
		kind = "image"
	case ".mp4", ".webm", ".mov", ".avi", ".mkv":
		kind = "video"
	default:
		return nil, kernel.BadAuthRequest("仅支持图片或视频附件")
	}
	resource, err := s.UploadResource(actor.ID, header, kind, 0, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("上传附件失败：%w", err)
	}
	if err := validateFeedbackResource(resource); err != nil {
		return nil, err
	}
	return &model.FeedbackAttachment{ID: resource.ID, ResourceID: resource.ID, Name: filepath.Base(header.Filename), Type: resource.Kind, URL: "/api/resources/" + resource.ID + "/file", Size: resource.Size, MimeType: resource.MimeType}, nil
}

func (s *Service) OpenFeedbackAttachment(actor *model.User, feedbackID, resourceID, rangeHeader string) (*ResourceDelivery, error) {
	if actor == nil || actor.ID == "" {
		return nil, Unauthorized("请先登录")
	}
	fb, err := s.repo.GetFeedback(feedbackID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, kernel.NewAppError(http.StatusNotFound, "反馈不存在")
		}
		return nil, err
	}
	if fb.UserID != actor.ID {
		if err := s.RequireAdminPermission(actor, model.AdminPermissionFeedbacks); err != nil {
			return nil, err
		}
	}
	var items []model.FeedbackAttachment
	if err := json.Unmarshal([]byte(fb.Attachments), &items); err != nil {
		return nil, kernel.BadAuthRequest("反馈附件记录无效")
	}
	found := false
	for _, item := range items {
		if item.ID == resourceID {
			found = true
			break
		}
	}
	if !found || resourceID == "" {
		return nil, kernel.NewAppError(http.StatusNotFound, "反馈附件不存在")
	}
	resource, err := s.repo.ResourceForUser(fb.UserID, resourceID)
	if err != nil {
		return nil, err
	}
	if err := validateFeedbackResource(resource); err != nil {
		return nil, err
	}
	stream, err := s.openResourceRange(fb.UserID, resource, rangeHeader)
	if err != nil {
		return nil, err
	}
	return &ResourceDelivery{Resource: resource, Stream: stream, Access: &assets.ResourceAccess{Delivery: assets.DeliveryLocal}}, nil
}

func (s *Service) AdminListFeedbacks(actor *model.User, filter repository.FeedbackFilter) (*FeedbackListResult, error) {
	if actor == nil || actor.ID == "" {
		return nil, Unauthorized("请先登录")
	}
	if err := s.RequireAdminPermission(actor, model.AdminPermissionFeedbacks); err != nil {
		return nil, err
	}
	list, total, err := s.repo.ListAllFeedbacks(filter)
	if err != nil {
		return nil, fmt.Errorf("查询反馈列表失败：%w", err)
	}
	page, limit := filter.Page, filter.Limit
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return &FeedbackListResult{List: list, Total: total, Page: page, Limit: limit}, nil
}

func (s *Service) AdminUpdateFeedback(actor *model.User, id string, req AdminUpdateFeedbackRequest) (*model.UserFeedback, error) {
	if actor == nil || actor.ID == "" {
		return nil, Unauthorized("请先登录")
	}
	if err := s.RequireAdminPermission(actor, model.AdminPermissionFeedbacks); err != nil {
		return nil, err
	}
	if req.ExpectedVersion < 1 {
		return nil, kernel.BadAuthRequest("缺少反馈版本，请刷新列表")
	}
	fb, err := s.repo.GetFeedback(strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, kernel.NewAppError(http.StatusNotFound, "反馈不存在或已被删除")
		}
		return nil, err
	}
	if fb.Version != req.ExpectedVersion {
		return nil, feedbackChanged()
	}
	now := time.Now().UTC()
	if req.Status != "" {
		switch req.Status {
		case model.FeedbackStatusPending, model.FeedbackStatusProcessing, model.FeedbackStatusResolved, model.FeedbackStatusClosed:
			fb.Status = req.Status
		default:
			return nil, kernel.BadAuthRequest("反馈状态无效")
		}
	}
	if req.AdminReply != nil {
		reply := strings.TrimSpace(*req.AdminReply)
		if utf8.RuneCountInString(reply) > 10000 {
			return nil, kernel.BadAuthRequest("回复不能超过 10000 个字")
		}
		fb.AdminReply, fb.RepliedAt = reply, &now
	}
	if req.Archived != nil {
		fb.Archived = *req.Archived
		fb.ArchivedAt = nil
		if fb.Archived {
			fb.ArchivedAt = &now
		}
	}
	fb.UpdatedAt = now
	audit, err := newAdminAuditEvent(actor, "feedback.update", "user_feedback", fb.ID, "更新用户反馈处理记录", map[string]any{"fromVersion": req.ExpectedVersion, "status": fb.Status, "archived": fb.Archived, "replyChanged": req.AdminReply != nil})
	if err != nil {
		return nil, err
	}
	err = s.repo.Transaction(func(tx *repository.Repository) error {
		updated, err := tx.UpdateFeedbackIfVersion(fb, req.ExpectedVersion)
		if err != nil {
			return err
		}
		if !updated {
			return feedbackChanged()
		}
		return tx.AppendAdminAudit(audit)
	})
	if err != nil {
		return nil, err
	}
	fb.Version++
	return fb, nil
}

func (s *Service) AdminArchiveFeedback(actor *model.User, id string, archive bool, expectedVersion int64) (*model.UserFeedback, error) {
	return s.AdminUpdateFeedback(actor, id, AdminUpdateFeedbackRequest{ExpectedVersion: expectedVersion, Archived: &archive})
}

func (s *Service) AdminDeleteFeedback(actor *model.User, id string, expectedVersion int64) error {
	if actor == nil || actor.ID == "" {
		return Unauthorized("请先登录")
	}
	if err := s.RequireAdminPermission(actor, model.AdminPermissionFeedbacks); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" || expectedVersion < 1 {
		return kernel.BadAuthRequest("缺少反馈标识或版本")
	}
	audit, err := newAdminAuditEvent(actor, "feedback.delete", "user_feedback", id, "删除用户反馈记录", map[string]any{"fromVersion": expectedVersion})
	if err != nil {
		return err
	}
	return s.repo.Transaction(func(tx *repository.Repository) error {
		deleted, err := tx.DeleteFeedbackIfVersion(id, expectedVersion)
		if err != nil {
			return err
		}
		if !deleted {
			return feedbackChanged()
		}
		return tx.AppendAdminAudit(audit)
	})
}
