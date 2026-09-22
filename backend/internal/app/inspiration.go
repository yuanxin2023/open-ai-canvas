package app

import (
	"encoding/json"
	"errors"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

const InspirationCoverMaxBytes int64 = 10 << 20
const inspirationCoverDraftTTL = 24 * time.Hour

type InspirationRequest struct {
	Title           string                `json:"title"`
	Description     string                `json:"description"`
	Mode            model.InspirationMode `json:"mode"`
	Prompt          string                `json:"prompt"`
	Tags            []string              `json:"tags"`
	Source          string                `json:"source"`
	CoverResourceID string                `json:"coverResourceId"`
	CoverURL        string                `json:"coverUrl"`
	CoverWidth      int                   `json:"coverWidth"`
	CoverHeight     int                   `json:"coverHeight"`
}

type InspirationPage struct {
	Inspirations []model.Inspiration `json:"inspirations"`
	Total        int64               `json:"total"`
	Page         int                 `json:"page"`
	Limit        int                 `json:"pageSize"`
}

type InspirationOrderItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type InspirationOrderRequest struct {
	IDs         []string `json:"ids"`
	ExpectedIDs []string `json:"expectedIds"`
}

func decorateInspiration(row *model.Inspiration) {
	if row != nil && row.CoverResourceID != "" {
		row.CoverURL = "/api/inspirations/" + row.ID + "/cover"
	}
}

func (s *Service) Inspirations(user *model.User) ([]model.Inspiration, error) {
	if user == nil {
		return nil, Unauthorized("请先登录")
	}
	rows, err := s.repo.ActiveInspirations()
	for index := range rows {
		decorateInspiration(&rows[index])
	}
	return rows, err
}

func (s *Service) AdminInspirationPage(actor *model.User, query AdminListQuery) (*InspirationPage, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	if query.Status != "" && query.Status != string(model.InspirationStatusActive) && query.Status != string(model.InspirationStatusDisabled) {
		return nil, BadAuthRequest("精选灵感状态无效")
	}
	if query.Type != "" && !validInspirationMode(model.InspirationMode(query.Type)) {
		return nil, BadAuthRequest("创作类型无效")
	}
	page, limit := normalizeAdminPage(query.Page, query.Limit)
	rows, total, err := s.repo.AdminInspirations(query.Keyword, model.InspirationMode(query.Type), model.InspirationStatus(query.Status), limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	for index := range rows {
		decorateInspiration(&rows[index])
	}
	return &InspirationPage{Inspirations: rows, Total: total, Page: page, Limit: limit}, nil
}

func normalizeInspirationRequest(req InspirationRequest, allowBuiltInURL bool) (InspirationRequest, string, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.Source = strings.TrimSpace(req.Source)
	req.CoverResourceID = strings.TrimSpace(req.CoverResourceID)
	req.CoverURL = strings.TrimSpace(req.CoverURL)
	if req.Title == "" || utf8.RuneCountInString(req.Title) > 120 {
		return req, "", BadAuthRequest("标题必填且不能超过 120 个字符")
	}
	if req.Description == "" || utf8.RuneCountInString(req.Description) > 240 {
		return req, "", BadAuthRequest("卡片说明必填且不能超过 240 个字符")
	}
	if req.Prompt == "" || utf8.RuneCountInString(req.Prompt) > 20000 {
		return req, "", BadAuthRequest("提示词正文必填且不能超过 20000 个字符")
	}
	if !validInspirationMode(req.Mode) {
		return req, "", BadAuthRequest("创作类型无效")
	}
	if utf8.RuneCountInString(req.Source) > 120 {
		return req, "", BadAuthRequest("来源署名不能超过 120 个字符")
	}
	if req.CoverResourceID != "" && req.CoverURL != "" {
		return req, "", BadAuthRequest("上传封面与外链封面只能选择一种")
	}
	if req.CoverResourceID == "" && req.CoverURL == "" {
		return req, "", BadAuthRequest("请上传封面或填写 HTTPS 封面地址")
	}
	if req.CoverWidth <= 0 || req.CoverHeight <= 0 || req.CoverWidth > 100000 || req.CoverHeight > 100000 {
		return req, "", BadAuthRequest("无法确认封面尺寸，请重新选择图片")
	}
	if req.CoverURL != "" {
		if len(req.CoverURL) > 1000 {
			return req, "", BadAuthRequest("封面地址不能超过 1000 个字符")
		}
		parsed, err := url.Parse(req.CoverURL)
		if err != nil || (parsed.Scheme != "https" && !(allowBuiltInURL && strings.HasPrefix(req.CoverURL, "/"))) || (parsed.Scheme == "https" && parsed.Host == "") {
			return req, "", BadAuthRequest("封面外链必须是有效的 HTTPS 地址")
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
			return req, "", BadAuthRequest("单个标签不能超过 24 个字符")
		}
		seen[key] = true
		tags = append(tags, tag)
		if len(tags) > 8 {
			return req, "", BadAuthRequest("标签不能超过 8 个")
		}
	}
	req.Tags = tags
	encoded, _ := json.Marshal(tags)
	return req, string(encoded), nil
}

func validInspirationMode(mode model.InspirationMode) bool {
	return mode == model.InspirationModeText || mode == model.InspirationModeImage || mode == model.InspirationModeVideo
}

func (s *Service) CreateInspiration(actor *model.User, req InspirationRequest) (*model.Inspiration, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	req, tagsJSON, err := normalizeInspirationRequest(req, false)
	if err != nil {
		return nil, err
	}
	if req.CoverResourceID != "" {
		s.storageMu.Lock()
		defer s.storageMu.Unlock()
	}
	if req.CoverResourceID != "" {
		resource, validateErr := s.validateInspirationCoverDraft(actor, req.CoverResourceID)
		if validateErr != nil {
			return nil, validateErr
		}
		req.CoverResourceID, req.CoverWidth, req.CoverHeight = resource.ID, resource.Width, resource.Height
	}
	sortOrder, err := s.repo.NextInspirationSortOrder()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	row := &model.Inspiration{ID: newID(), Title: req.Title, Description: req.Description, Mode: req.Mode, Prompt: req.Prompt, Tags: req.Tags, TagsJSON: tagsJSON, Source: req.Source, CoverResourceID: req.CoverResourceID, CoverURL: req.CoverURL, CoverWidth: req.CoverWidth, CoverHeight: req.CoverHeight, Status: model.InspirationStatusDisabled, SortOrder: sortOrder, CreatedBy: actor.ID, UpdatedBy: actor.ID, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateInspiration(row); err != nil {
		if errors.Is(err, repository.ErrInspirationCoverDraftUnavailable) {
			return nil, BadAuthRequest("封面草稿已失效，请重新上传")
		}
		return nil, err
	}
	if err := s.appendAdminAudit(actor, "inspiration.create", "inspiration", row.ID, "新增精选灵感（默认停用）", map[string]any{"mode": row.Mode}); err != nil {
		return nil, err
	}
	decorateInspiration(row)
	return row, nil
}

func (s *Service) UpdateInspiration(actor *model.User, id string, req InspirationRequest) (*model.Inspiration, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	row, err := s.repo.Inspiration(strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NotFound("精选灵感不存在")
		}
		return nil, err
	}
	req, tagsJSON, err := normalizeInspirationRequest(req, row.CoverURL != "" && req.CoverURL == row.CoverURL)
	if err != nil {
		return nil, err
	}
	newDraft := ""
	if req.CoverResourceID != "" && req.CoverResourceID != row.CoverResourceID {
		resource, validateErr := s.validateInspirationCoverDraft(actor, req.CoverResourceID)
		if validateErr != nil {
			return nil, validateErr
		}
		req.CoverResourceID, req.CoverWidth, req.CoverHeight = resource.ID, resource.Width, resource.Height
		newDraft = req.CoverResourceID
	} else if req.CoverResourceID != "" && row.CoverWidth > 0 && row.CoverHeight > 0 {
		req.CoverWidth, req.CoverHeight = row.CoverWidth, row.CoverHeight
	}
	var oldResource *model.Resource
	var deletionJob *model.ResourceDeletionJob
	if row.CoverResourceID != "" && row.CoverResourceID != req.CoverResourceID {
		oldResource, err = s.repo.Resource(row.CoverResourceID)
		if err != nil {
			return nil, BadAuthRequest("原封面资源不存在，已停止更新")
		}
		if err := s.ensureResourceHasNoBusinessReferences(oldResource, repository.ResourceDirectReference{Kind: "精选灵感", ID: row.ID}); err != nil {
			return nil, err
		}
		shared, err := s.repo.ResourceStorageReferenceCount(oldResource, []string{oldResource.ID})
		if err != nil {
			return nil, err
		}
		if shared == 0 {
			jobs := resourceDeletionJobs(oldResource.UserID, map[string]*model.Resource{resourceStorageIdentity(oldResource): oldResource})
			if len(jobs) == 1 {
				deletionJob = &jobs[0]
			}
		}
	}
	row.Title, row.Description, row.Mode, row.Prompt = req.Title, req.Description, req.Mode, req.Prompt
	row.Tags, row.TagsJSON, row.Source = req.Tags, tagsJSON, req.Source
	row.CoverResourceID, row.CoverURL, row.CoverWidth, row.CoverHeight, row.UpdatedBy, row.UpdatedAt = req.CoverResourceID, req.CoverURL, req.CoverWidth, req.CoverHeight, actor.ID, time.Now()
	if err := s.repo.UpdateInspiration(row, actor.ID, newDraft, oldResource, deletionJob); err != nil {
		if errors.Is(err, repository.ErrInspirationCoverDraftUnavailable) {
			return nil, BadAuthRequest("封面草稿已失效，请重新上传")
		}
		return nil, err
	}
	if err := s.appendAdminAudit(actor, "inspiration.update", "inspiration", row.ID, "编辑精选灵感", map[string]any{"mode": row.Mode}); err != nil {
		return nil, err
	}
	decorateInspiration(row)
	return row, nil
}

func (s *Service) SetInspirationStatus(actor *model.User, id string, status model.InspirationStatus) (*model.Inspiration, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	if status != model.InspirationStatusActive && status != model.InspirationStatusDisabled {
		return nil, BadAuthRequest("精选灵感状态无效")
	}
	updated, err := s.repo.UpdateInspirationStatus(strings.TrimSpace(id), status, actor.ID, time.Now())
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, NotFound("精选灵感不存在")
	}
	row, err := s.repo.Inspiration(id)
	if err != nil {
		return nil, err
	}
	if err := s.appendAdminAudit(actor, "inspiration.status.update", "inspiration", row.ID, map[bool]string{true: "启用精选灵感", false: "停用精选灵感"}[status == model.InspirationStatusActive], map[string]any{"status": status}); err != nil {
		return nil, err
	}
	decorateInspiration(row)
	return row, nil
}

func (s *Service) InspirationOrder(actor *model.User) ([]InspirationOrderItem, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	rows, err := s.repo.InspirationOrder()
	if err != nil {
		return nil, err
	}
	items := make([]InspirationOrderItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, InspirationOrderItem{ID: row.ID, Name: row.Title, Enabled: row.Status == model.InspirationStatusActive})
	}
	return items, nil
}

func (s *Service) SaveInspirationOrder(actor *model.User, req InspirationOrderRequest) error {
	if err := s.RequireAdmin(actor); err != nil {
		return err
	}
	if len(req.IDs) == 0 || len(req.IDs) > 2000 {
		return BadAuthRequest("排序条目数量无效")
	}
	if err := s.repo.SaveInspirationOrder(req.IDs, req.ExpectedIDs, actor.ID, time.Now()); err != nil {
		if errors.Is(err, repository.ErrInspirationOrderChanged) {
			return BadAuthRequest("列表已发生变化，请重新打开排序后再保存")
		}
		return err
	}
	return s.appendAdminAudit(actor, "inspiration.order.update", "inspiration", "all", "调整精选灵感排序", map[string]any{"count": len(req.IDs)})
}

func (s *Service) DeleteInspirations(actor *model.User, ids []string) error {
	if err := s.RequireAdmin(actor); err != nil {
		return err
	}
	if len(ids) == 0 || len(ids) > 100 {
		return BadAuthRequest("单次删除数量必须在 1 到 100 条之间")
	}
	unique := map[string]bool{}
	clean := make([]string, 0, len(ids))
	rows := make([]*model.Inspiration, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" || unique[id] {
			return BadAuthRequest("删除列表包含无效或重复 ID")
		}
		unique[id] = true
		clean = append(clean, id)
		row, err := s.repo.Inspiration(id)
		if err != nil {
			return NotFound("精选灵感不存在")
		}
		rows = append(rows, row)
	}
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	resources := []model.Resource{}
	jobs := []model.ResourceDeletionJob{}
	for _, row := range rows {
		if row.CoverResourceID == "" {
			continue
		}
		resource, err := s.repo.Resource(row.CoverResourceID)
		if err != nil {
			return BadAuthRequest("精选灵感封面资源不存在，已停止删除")
		}
		if err := s.ensureResourceHasNoBusinessReferences(resource, repository.ResourceDirectReference{Kind: "精选灵感", ID: row.ID}); err != nil {
			return err
		}
		shared, err := s.repo.ResourceStorageReferenceCount(resource, []string{resource.ID})
		if err != nil {
			return err
		}
		resources = append(resources, *resource)
		if shared == 0 {
			jobs = append(jobs, resourceDeletionJobs(resource.UserID, map[string]*model.Resource{resourceStorageIdentity(resource): resource})...)
		}
	}
	if err := s.repo.DeleteInspirations(clean, resources, jobs); err != nil {
		return err
	}
	for _, row := range rows {
		if err := s.appendAdminAudit(actor, "inspiration.delete", "inspiration", row.ID, "永久删除精选灵感", map[string]any{"title": row.Title}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) UploadInspirationCover(actor *model.User, header *multipart.FileHeader, width, height int) (*model.Resource, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	if header == nil {
		return nil, BadAuthRequest("请选择封面图片")
	}
	if header.Size <= 0 || header.Size > InspirationCoverMaxBytes {
		return nil, BadAuthRequest("封面图片大小必须在 10MB 以内")
	}
	if width <= 0 || height <= 0 || width > 100000 || height > 100000 {
		return nil, BadAuthRequest("无法确认封面尺寸，请重新选择图片")
	}
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, 512)
	read, readErr := file.Read(buffer)
	_ = file.Close()
	if readErr != nil && read == 0 {
		return nil, BadAuthRequest("封面内容无法读取")
	}
	detected := http.DetectContentType(buffer[:read])
	if detected != "image/jpeg" && detected != "image/png" && detected != "image/webp" {
		return nil, BadAuthRequest("封面仅支持 JPEG、PNG 或 WebP")
	}
	resource, err := s.UploadResource(actor.ID, header, "image", width, height, 0)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateInspirationCoverDraft(&model.InspirationCoverDraft{ResourceID: resource.ID, UserID: actor.ID, CreatedAt: time.Now()}); err != nil {
		cleanupErr := s.deleteFreshAnnouncementImageResource(resource)
		if cleanupErr != nil {
			return nil, errors.Join(err, cleanupErr)
		}
		return nil, err
	}
	if err := s.appendAdminAudit(actor, "inspiration.cover.upload", "resource", resource.ID, "上传精选灵感封面草稿", map[string]any{"size": resource.Size}); err != nil {
		cleanupErr := s.discardInspirationCoverDraft(actor.ID, resource.ID)
		return nil, errors.Join(err, cleanupErr)
	}
	resource.PublicURL = ""
	return resource, nil
}

func (s *Service) validateInspirationCoverDraft(actor *model.User, id string) (*model.Resource, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}
	if _, err := s.repo.InspirationCoverDraftForUser(actor.ID, id); err != nil {
		return nil, BadAuthRequest("封面草稿不存在或不属于当前管理员")
	}
	resource, err := s.repo.ResourceForUser(actor.ID, id)
	if err != nil || resource.Kind != "image" || resource.Status != model.ResourceStatusReady || !strings.HasPrefix(strings.ToLower(resource.MimeType), "image/") || resource.Width <= 0 || resource.Height <= 0 {
		return nil, BadAuthRequest("封面必须是已确认尺寸并上传完成的图片")
	}
	return resource, nil
}

func (s *Service) DiscardInspirationCover(actor *model.User, id string) error {
	if err := s.RequireAdmin(actor); err != nil {
		return err
	}
	if err := s.discardInspirationCoverDraft(actor.ID, id); err != nil {
		return err
	}
	return s.appendAdminAudit(actor, "inspiration.cover.discard", "resource", strings.TrimSpace(id), "放弃精选灵感封面草稿", nil)
}

func (s *Service) discardInspirationCoverDraft(userID, id string) error {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	if _, err := s.repo.InspirationCoverDraftForUser(userID, id); err != nil {
		return NotFound("封面草稿不存在")
	}
	resource, err := s.repo.ResourceForUser(userID, id)
	if err != nil {
		_ = s.repo.DeleteInspirationCoverDraft(userID, id)
		return nil
	}
	if err := s.ensureResourceHasNoBusinessReferences(resource, repository.ResourceDirectReference{Kind: "精选灵感草稿", ID: id}); err != nil {
		return err
	}
	shared, err := s.repo.ResourceStorageReferenceCount(resource, []string{resource.ID})
	if err != nil {
		return err
	}
	var job *model.ResourceDeletionJob
	if shared == 0 {
		jobs := resourceDeletionJobs(userID, map[string]*model.Resource{resourceStorageIdentity(resource): resource})
		if len(jobs) == 1 {
			job = &jobs[0]
		}
	}
	if err := s.repo.DiscardInspirationCoverDraft(userID, resource, job); errors.Is(err, repository.ErrInspirationCoverReferenced) {
		return BadAuthRequest("封面已经发布，不能按草稿删除")
	} else {
		return err
	}
}

func (s *Service) OpenInspirationCover(actor *model.User, id, rangeHeader string) (*ResourceStream, error) {
	if actor == nil {
		return nil, Unauthorized("请先登录")
	}
	row, err := s.repo.Inspiration(strings.TrimSpace(id))
	if err != nil {
		return nil, NotFound("精选灵感不存在")
	}
	if actor.Role != model.UserRoleAdmin && row.Status != model.InspirationStatusActive {
		return nil, Forbidden("精选灵感不可访问")
	}
	if row.CoverResourceID == "" {
		return nil, NotFound("托管封面不存在")
	}
	resource, err := s.repo.Resource(row.CoverResourceID)
	if err != nil {
		return nil, NotFound("封面资源不存在")
	}
	return s.openResourceRange(resource.UserID, resource, rangeHeader)
}

func (s *Service) OpenInspirationCoverDraft(actor *model.User, id, rangeHeader string) (*ResourceStream, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	if _, err := s.repo.InspirationCoverDraftForUser(actor.ID, id); err != nil {
		return nil, NotFound("封面草稿不存在")
	}
	resource, err := s.repo.ResourceForUser(actor.ID, id)
	if err != nil {
		return nil, NotFound("封面资源不存在")
	}
	return s.openResourceRange(resource.UserID, resource, rangeHeader)
}

func (s *Service) cleanupStaleInspirationCoverDrafts() {
	for {
		drafts, err := s.repo.StaleInspirationCoverDrafts(time.Now().Add(-inspirationCoverDraftTTL), 50)
		if err != nil {
			log.Printf("inspiration cover cleanup query failed: %v", err)
			return
		}
		if len(drafts) == 0 {
			return
		}
		cleaned := 0
		for _, draft := range drafts {
			if err := s.discardInspirationCoverDraft(draft.UserID, draft.ResourceID); err == nil {
				cleaned++
			}
		}
		if len(drafts) < 50 || cleaned == 0 {
			return
		}
	}
}
