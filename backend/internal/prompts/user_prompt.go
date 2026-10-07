package prompts

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const maxUserPrompts = 500

type UserPromptRequest struct {
	Title           string                `json:"title"`
	Description     string                `json:"description"`
	Mode            model.InspirationMode `json:"mode"`
	Prompt          string                `json:"prompt"`
	Tags            []string              `json:"tags"`
	Source          string                `json:"source"`
	CoverResourceID string                `json:"coverResourceId"`
	CoverURL        string                `json:"coverUrl"`
}

type UserPromptPage struct {
	Prompts  []model.UserPrompt `json:"prompts"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

func (s *Service) ListUserPrompts(userID, keyword string, mode model.InspirationMode, page, pageSize int) (*UserPromptPage, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, kernel.Unauthorized("请先登录")
	}
	if mode != "" && !validUserPromptMode(mode) {
		return nil, kernel.BadAuthRequest("创作类型无效")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	rows, total, err := s.repo.UserPrompts(userID, keyword, mode, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	return &UserPromptPage{Prompts: rows, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Service) CreateUserPrompt(userID string, req UserPromptRequest) (*model.UserPrompt, error) {
	count, err := s.repo.UserPromptCount(userID)
	if err != nil {
		return nil, err
	}
	if count >= maxUserPrompts {
		return nil, kernel.BadAuthRequest("个人提示词最多保存 500 条")
	}
	req, tagsJSON, err := s.normalizeUserPromptRequest(userID, req)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	row := &model.UserPrompt{
		ID: kernel.NewID(), UserID: userID, Title: req.Title, Description: req.Description, Mode: req.Mode,
		Prompt: req.Prompt, Tags: req.Tags, TagsJSON: tagsJSON, Source: req.Source,
		CoverResourceID: req.CoverResourceID, CoverURL: req.CoverURL, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreateUserPrompt(row); err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) UpdateUserPrompt(userID, id string, req UserPromptRequest) (*model.UserPrompt, error) {
	row, err := s.repo.UserPromptForUser(userID, strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, kernel.NotFound("提示词不存在")
		}
		return nil, err
	}
	req, tagsJSON, err := s.normalizeUserPromptRequest(userID, req)
	if err != nil {
		return nil, err
	}
	row.Title, row.Description, row.Mode, row.Prompt = req.Title, req.Description, req.Mode, req.Prompt
	row.Tags, row.TagsJSON, row.Source = req.Tags, tagsJSON, req.Source
	row.CoverResourceID, row.CoverURL, row.UpdatedAt = req.CoverResourceID, req.CoverURL, time.Now()
	if err := s.repo.UpdateUserPrompt(row); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, kernel.NotFound("提示词不存在")
		}
		return nil, err
	}
	return row, nil
}

func (s *Service) DeleteUserPrompt(userID, id string) error {
	deleted, err := s.repo.DeleteUserPrompt(userID, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if !deleted {
		return kernel.NotFound("提示词不存在")
	}
	return nil
}

func (s *Service) normalizeUserPromptRequest(userID string, req UserPromptRequest) (UserPromptRequest, string, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.Source = strings.TrimSpace(req.Source)
	req.CoverResourceID = strings.TrimSpace(req.CoverResourceID)
	req.CoverURL = strings.TrimSpace(req.CoverURL)
	if req.Title == "" || utf8.RuneCountInString(req.Title) > 120 {
		return req, "", kernel.BadAuthRequest("标题必填且不能超过 120 个字符")
	}
	if utf8.RuneCountInString(req.Description) > 500 {
		return req, "", kernel.BadAuthRequest("卡片说明不能超过 500 个字符")
	}
	if req.Prompt == "" || utf8.RuneCountInString(req.Prompt) > 20000 {
		return req, "", kernel.BadAuthRequest("提示词正文必填且不能超过 20000 个字符")
	}
	if !validUserPromptMode(req.Mode) {
		return req, "", kernel.BadAuthRequest("创作类型无效")
	}
	if utf8.RuneCountInString(req.Source) > 120 {
		return req, "", kernel.BadAuthRequest("来源署名不能超过 120 个字符")
	}
	if req.CoverResourceID != "" && req.CoverURL != "" {
		return req, "", kernel.BadAuthRequest("上传封面与外链封面只能选择一种")
	}
	if req.CoverResourceID != "" {
		resource, err := s.repo.ResourceForUser(userID, req.CoverResourceID)
		if err != nil || resource.Kind != "image" || resource.Status != model.ResourceStatusReady || !strings.HasPrefix(strings.ToLower(resource.MimeType), "image/") {
			return req, "", kernel.BadAuthRequest("封面必须是当前用户上传完成的图片")
		}
	}
	if req.CoverURL != "" {
		if len(req.CoverURL) > 1000 {
			return req, "", kernel.BadAuthRequest("封面地址不能超过 1000 个字符")
		}
		parsed, err := url.Parse(req.CoverURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return req, "", kernel.BadAuthRequest("封面外链必须是有效的 HTTPS 地址")
		}
	}
	seen := map[string]bool{}
	tags := make([]string, 0, len(req.Tags))
	for _, raw := range req.Tags {
		tag := strings.TrimSpace(raw)
		key := strings.ToLower(tag)
		if tag == "" || seen[key] {
			continue
		}
		if utf8.RuneCountInString(tag) > 24 {
			return req, "", kernel.BadAuthRequest("单个标签不能超过 24 个字符")
		}
		seen[key] = true
		tags = append(tags, tag)
		if len(tags) > 8 {
			return req, "", kernel.BadAuthRequest("标签不能超过 8 个")
		}
	}
	req.Tags = tags
	encoded, _ := json.Marshal(tags)
	return req, string(encoded), nil
}

func validUserPromptMode(mode model.InspirationMode) bool {
	return mode == model.InspirationModeText || mode == model.InspirationModeImage || mode == model.InspirationModeVideo
}
