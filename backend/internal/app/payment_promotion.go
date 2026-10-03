package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

const (
	paymentPromotionSettingKey     = "payment_promotion"
	paymentPromotionSchemaVersion  = 1
	paymentPromotionImageDraftTTL  = 24 * time.Hour
	PaymentPromotionImageMaxBytes  = int64(10 << 20)
	maxPaymentPromotionDurationDay = 365
)

type PaymentPromotionSetting struct {
	SchemaVersion       int        `json:"schemaVersion"`
	CardEnabled         bool       `json:"cardEnabled"`
	ImageResourceID     string     `json:"imageResourceId"`
	ActivityEnabled     bool       `json:"activityEnabled"`
	ActiveTitle         string     `json:"activeTitle"`
	ActiveSubtitle      string     `json:"activeSubtitle"`
	StartsAt            *time.Time `json:"startsAt,omitempty"`
	DurationDays        int        `json:"durationDays"`
	InactiveCopyEnabled bool       `json:"inactiveCopyEnabled"`
	InactiveTitle       string     `json:"inactiveTitle"`
	InactiveSubtitle    string     `json:"inactiveSubtitle"`
}

type PublicPaymentPromotion struct {
	Visible             bool       `json:"visible"`
	Phase               string     `json:"phase"`
	ImageURL            string     `json:"imageUrl,omitempty"`
	ActiveTitle         string     `json:"activeTitle,omitempty"`
	ActiveSubtitle      string     `json:"activeSubtitle,omitempty"`
	InactiveCopyEnabled bool       `json:"inactiveCopyEnabled"`
	InactiveTitle       string     `json:"inactiveTitle,omitempty"`
	InactiveSubtitle    string     `json:"inactiveSubtitle,omitempty"`
	StartsAt            *time.Time `json:"startsAt,omitempty"`
	EndsAt              *time.Time `json:"endsAt,omitempty"`
	Revision            string     `json:"revision,omitempty"`
}

type AdminPaymentPromotion struct {
	PaymentPromotionSetting
	Public     PublicPaymentPromotion `json:"public"`
	ImageURL   string                 `json:"imageUrl,omitempty"`
	Configured bool                   `json:"configured"`
	UpdatedBy  string                 `json:"updatedBy,omitempty"`
	CreatedAt  time.Time              `json:"createdAt,omitempty"`
	UpdatedAt  time.Time              `json:"updatedAt,omitempty"`
}

func defaultPaymentPromotionSetting() PaymentPromotionSetting {
	return PaymentPromotionSetting{SchemaVersion: paymentPromotionSchemaVersion, DurationDays: 3}
}

func (s *Service) AdminPaymentPromotion(actor *model.User) (*AdminPaymentPromotion, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionProducts); err != nil {
		return nil, err
	}
	setting, value, err := s.readPaymentPromotion()
	if err != nil {
		return nil, err
	}
	public := s.publicPaymentPromotion(setting, value, time.Now())
	result := &AdminPaymentPromotion{PaymentPromotionSetting: value, Public: public, Configured: setting != nil}
	if value.ImageResourceID != "" {
		if _, imageErr := s.paymentPromotionImageResource(value.ImageResourceID); imageErr == nil {
			result.ImageURL = "/api/admin/payments/promotion/image"
		}
	}
	if setting != nil {
		result.UpdatedBy, result.CreatedAt, result.UpdatedAt = setting.UpdatedBy, setting.CreatedAt, setting.UpdatedAt
	}
	return result, nil
}

func (s *Service) UpdatePaymentPromotion(actor *model.User, value PaymentPromotionSetting) (*AdminPaymentPromotion, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionProducts); err != nil {
		return nil, err
	}
	value = normalizePaymentPromotion(value)
	if err := validatePaymentPromotion(value); err != nil {
		return nil, err
	}

	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	current, before, err := s.readPaymentPromotion()
	if err != nil {
		return nil, err
	}
	newDraftResourceID := ""
	if value.ImageResourceID != "" && value.ImageResourceID != before.ImageResourceID {
		if err := s.validatePaymentPromotionImageDraft(actor, value.ImageResourceID); err != nil {
			return nil, err
		}
		newDraftResourceID = value.ImageResourceID
	} else if value.ImageResourceID != "" {
		if _, err := s.paymentPromotionImageResource(value.ImageResourceID); err != nil {
			return nil, err
		}
	}

	var oldResource *model.Resource
	var deletionJob *model.ResourceDeletionJob
	if before.ImageResourceID != "" && before.ImageResourceID != value.ImageResourceID {
		oldResource, err = s.repo.Resource(before.ImageResourceID)
		if err != nil {
			return nil, BadAuthRequest("原促销背景图不存在，已停止更新")
		}
		if err := s.ensurePaymentPromotionImageReplaceable(oldResource); err != nil {
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

	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	setting := &model.SystemSetting{Key: paymentPromotionSettingKey, ValueJSON: string(encoded), UpdatedBy: actor.ID}
	if current != nil {
		setting.CreatedAt = current.CreatedAt
	}
	if err := s.repo.SavePaymentPromotionSetting(setting, actor.ID, newDraftResourceID, oldResource, deletionJob); err != nil {
		if errors.Is(err, repository.ErrPaymentPromotionImageDraftUnavailable) {
			return nil, BadAuthRequest("促销背景图草稿已失效，请重新上传")
		}
		return nil, err
	}
	if deletionJob != nil {
		s.runWorkerTask(func() { s.drainResourceDeletionJobs(1) })
	}
	if err := s.appendAdminAudit(actor, "payment_promotion.update", "system_setting", paymentPromotionSettingKey, "更新充值套餐促销配置", map[string]any{"before": before, "after": value}); err != nil {
		return nil, err
	}
	return s.AdminPaymentPromotion(actor)
}

func (s *Service) UploadPaymentPromotionImage(actor *model.User, header *multipart.FileHeader) (*model.Resource, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionProducts); err != nil {
		return nil, err
	}
	if err := validatePaymentPromotionImageUpload(header); err != nil {
		return nil, err
	}
	resource, err := s.UploadResource(actor.ID, header, "image", 0, 0, 0)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreatePaymentPromotionImageDraft(&model.PaymentPromotionImageDraft{ResourceID: resource.ID, UserID: actor.ID, CreatedAt: time.Now()}); err != nil {
		cleanupErr := s.deleteFreshAnnouncementImageResource(resource)
		if cleanupErr != nil {
			return nil, errors.Join(err, cleanupErr)
		}
		return nil, err
	}
	resource.PublicURL = ""
	return resource, nil
}

func (s *Service) DiscardPaymentPromotionImage(actor *model.User, resourceID string) error {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionProducts); err != nil {
		return err
	}
	return s.discardPaymentPromotionImageDraft(actor.ID, resourceID)
}

func (s *Service) OpenPaymentPromotionImage(actor *model.User, rangeHeader string) (*ResourceStream, error) {
	if actor == nil {
		return nil, Unauthorized("请先登录")
	}
	if err := s.RequireFeature(FeatureCredits); err != nil {
		return nil, err
	}
	_, value, err := s.readPaymentPromotion()
	if err != nil {
		return nil, err
	}
	if !value.CardEnabled || value.ImageResourceID == "" {
		return nil, NotFound("促销背景图未启用")
	}
	resource, err := s.paymentPromotionImageResource(value.ImageResourceID)
	if err != nil {
		return nil, err
	}
	return s.openResourceRange(resource.UserID, resource, rangeHeader)
}

func (s *Service) OpenAdminPaymentPromotionImage(actor *model.User, rangeHeader string) (*ResourceStream, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionProducts); err != nil {
		return nil, err
	}
	_, value, err := s.readPaymentPromotion()
	if err != nil {
		return nil, err
	}
	if value.ImageResourceID == "" {
		return nil, NotFound("促销背景图未配置")
	}
	resource, err := s.paymentPromotionImageResource(value.ImageResourceID)
	if err != nil {
		return nil, err
	}
	return s.openResourceRange(resource.UserID, resource, rangeHeader)
}

func (s *Service) readPaymentPromotion() (*model.SystemSetting, PaymentPromotionSetting, error) {
	setting, err := s.repo.SystemSetting(paymentPromotionSettingKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, defaultPaymentPromotionSetting(), nil
	}
	if err != nil {
		return nil, PaymentPromotionSetting{}, err
	}
	value := defaultPaymentPromotionSetting()
	if err := json.Unmarshal([]byte(setting.ValueJSON), &value); err != nil {
		return nil, PaymentPromotionSetting{}, fmt.Errorf("促销优惠配置损坏：%w", err)
	}
	value = normalizePaymentPromotion(value)
	return setting, value, nil
}

func normalizePaymentPromotion(value PaymentPromotionSetting) PaymentPromotionSetting {
	value.SchemaVersion = paymentPromotionSchemaVersion
	value.ImageResourceID = strings.TrimSpace(value.ImageResourceID)
	value.ActiveTitle = strings.TrimSpace(value.ActiveTitle)
	value.ActiveSubtitle = strings.TrimSpace(value.ActiveSubtitle)
	value.InactiveTitle = strings.TrimSpace(value.InactiveTitle)
	value.InactiveSubtitle = strings.TrimSpace(value.InactiveSubtitle)
	if value.StartsAt != nil {
		utc := value.StartsAt.UTC()
		value.StartsAt = &utc
	}
	return value
}

func validatePaymentPromotion(value PaymentPromotionSetting) error {
	if len(value.ImageResourceID) > 64 {
		return BadAuthRequest("促销背景图资源 ID 无效")
	}
	if value.CardEnabled && value.ImageResourceID == "" {
		return BadAuthRequest("启用促销卡片前请上传背景图")
	}
	if len([]rune(value.ActiveTitle)) > 120 || len([]rune(value.InactiveTitle)) > 120 || len([]rune(value.ActiveSubtitle)) > 240 || len([]rune(value.InactiveSubtitle)) > 240 {
		return BadAuthRequest("促销卡片文案超过长度限制")
	}
	if value.ActivityEnabled {
		if value.StartsAt == nil || value.StartsAt.IsZero() {
			return BadAuthRequest("启用促销活动前请设置开始时间")
		}
		if value.DurationDays < 1 || value.DurationDays > maxPaymentPromotionDurationDay {
			return BadAuthRequest("活动时长必须为 1 至 365 天")
		}
		if value.ActiveTitle == "" {
			return BadAuthRequest("启用促销活动前请填写活动主标题")
		}
	}
	if value.InactiveCopyEnabled && value.InactiveTitle == "" {
		return BadAuthRequest("显示非活动文案时必须填写日常主标题")
	}
	return nil
}

func paymentPromotionEndsAt(value PaymentPromotionSetting) *time.Time {
	if value.StartsAt == nil || value.DurationDays <= 0 {
		return nil
	}
	end := value.StartsAt.Add(time.Duration(value.DurationDays) * 24 * time.Hour)
	return &end
}

func paymentPromotionActive(value PaymentPromotionSetting, now time.Time) bool {
	if !value.CardEnabled || !value.ActivityEnabled || value.ImageResourceID == "" || value.StartsAt == nil {
		return false
	}
	endsAt := paymentPromotionEndsAt(value)
	return endsAt != nil && !now.Before(*value.StartsAt) && now.Before(*endsAt)
}

func paymentPromotionPhase(value PaymentPromotionSetting, now time.Time) string {
	if !value.ActivityEnabled || value.StartsAt == nil {
		return "inactive"
	}
	if now.Before(*value.StartsAt) {
		return "scheduled"
	}
	if paymentPromotionActive(value, now) {
		return "active"
	}
	return "expired"
}

func (s *Service) publicPaymentPromotion(setting *model.SystemSetting, value PaymentPromotionSetting, now time.Time) PublicPaymentPromotion {
	visible := value.CardEnabled && value.ImageResourceID != ""
	if visible {
		if _, err := s.paymentPromotionImageResource(value.ImageResourceID); err != nil {
			visible = false
		}
	}
	result := PublicPaymentPromotion{
		Visible: visible, Phase: paymentPromotionPhase(value, now), ActiveTitle: value.ActiveTitle, ActiveSubtitle: value.ActiveSubtitle,
		InactiveCopyEnabled: value.InactiveCopyEnabled, InactiveTitle: value.InactiveTitle, InactiveSubtitle: value.InactiveSubtitle,
		StartsAt: value.StartsAt, EndsAt: paymentPromotionEndsAt(value),
	}
	if visible {
		result.ImageURL = "/api/payments/promotion/image"
	}
	if setting != nil {
		result.Revision = fmt.Sprintf("%d", setting.UpdatedAt.UnixNano())
	}
	return result
}

func (s *Service) paymentPromotionImageResource(resourceID string) (*model.Resource, error) {
	resource, err := s.repo.Resource(strings.TrimSpace(resourceID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NotFound("促销背景图不存在")
		}
		return nil, err
	}
	if resource.Kind != "image" || resource.Status != model.ResourceStatusReady || !strings.HasPrefix(strings.ToLower(resource.MimeType), "image/") {
		return nil, BadAuthRequest("促销背景图资源无效")
	}
	return resource, nil
}

func (s *Service) validatePaymentPromotionImageDraft(actor *model.User, resourceID string) error {
	if _, err := s.repo.PaymentPromotionImageDraftForUser(actor.ID, resourceID); err != nil {
		return BadAuthRequest("促销背景图草稿不存在或不属于当前管理员")
	}
	resource, err := s.repo.ResourceForUser(actor.ID, resourceID)
	if err != nil {
		return BadAuthRequest("促销背景图资源不存在或不属于当前管理员")
	}
	if resource.Kind != "image" || resource.Status != model.ResourceStatusReady || !strings.HasPrefix(strings.ToLower(resource.MimeType), "image/") {
		return BadAuthRequest("促销背景图必须是上传完成的图片")
	}
	return nil
}

func validatePaymentPromotionImageUpload(header *multipart.FileHeader) error {
	if header == nil {
		return BadAuthRequest("请选择促销背景图")
	}
	if header.Size <= 0 || header.Size > PaymentPromotionImageMaxBytes {
		return BadAuthRequest("促销背景图大小必须在 10MB 以内")
	}
	file, err := header.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	buffer := make([]byte, 512)
	read, readErr := file.Read(buffer)
	if readErr != nil && read == 0 {
		return BadAuthRequest("促销背景图内容无法读取")
	}
	detected := http.DetectContentType(buffer[:read])
	if detected != "image/jpeg" && detected != "image/png" && detected != "image/webp" {
		return BadAuthRequest("促销背景图仅支持 JPEG、PNG 或 WebP")
	}
	return nil
}

func (s *Service) ensurePaymentPromotionImageReplaceable(resource *model.Resource) error {
	snapshot, err := s.repo.ResourceReferenceSnapshot(resource.UserID, "", []string{resource.ID})
	if err != nil {
		return err
	}
	for _, direct := range snapshot.Direct {
		if direct.Kind != "促销优惠草稿" {
			return BadAuthRequest("原促销背景图仍被其他业务数据引用，已停止替换")
		}
	}
	resourceIDs := map[string]struct{}{resource.ID: {}}
	for _, document := range snapshot.Documents {
		if document.Kind == "促销优惠" {
			continue
		}
		if documentReferencesResources(document.PrimaryJSON, resourceIDs) || documentReferencesResources(document.SecondaryJSON, resourceIDs) {
			return BadAuthRequest("原促销背景图仍被其他业务数据引用，已停止替换")
		}
	}
	return nil
}

func (s *Service) discardPaymentPromotionImageDraft(userID, resourceID string) error {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" || len(resourceID) > 64 {
		return BadAuthRequest("促销背景图资源 ID 无效")
	}
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	if _, err := s.repo.PaymentPromotionImageDraftForUser(userID, resourceID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NotFound("促销背景图草稿不存在")
		}
		return err
	}
	resource, err := s.repo.ResourceForUser(userID, resourceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.repo.DeletePaymentPromotionImageDraft(userID, resourceID)
		}
		return err
	}
	if err := s.ensureResourceHasNoBusinessReferences(resource, repository.ResourceDirectReference{Kind: "促销优惠草稿", ID: resource.ID}); err != nil {
		return err
	}
	shared, err := s.repo.ResourceStorageReferenceCount(resource, []string{resource.ID})
	if err != nil {
		return err
	}
	var deletionJob *model.ResourceDeletionJob
	if shared == 0 {
		jobs := resourceDeletionJobs(userID, map[string]*model.Resource{resourceStorageIdentity(resource): resource})
		if len(jobs) != 1 {
			return errors.New("无法创建促销背景图删除任务")
		}
		deletionJob = &jobs[0]
	}
	if err := s.repo.DiscardPaymentPromotionImageDraft(userID, resource, deletionJob); err != nil {
		return err
	}
	if deletionJob != nil {
		s.runWorkerTask(func() { s.drainResourceDeletionJobs(1) })
	}
	return nil
}

func (s *Service) cleanupStalePaymentPromotionImageDrafts() {
	for {
		drafts, err := s.repo.StalePaymentPromotionImageDrafts(time.Now().Add(-paymentPromotionImageDraftTTL), 50)
		if err != nil || len(drafts) == 0 {
			return
		}
		cleaned := 0
		for _, draft := range drafts {
			if err := s.discardPaymentPromotionImageDraft(draft.UserID, draft.ResourceID); err == nil {
				cleaned++
			}
		}
		if len(drafts) < 50 || cleaned == 0 {
			return
		}
	}
}

func (s *Service) paymentPromotionResourceReferences(resourceIDs []string) map[string]struct{} {
	references := map[string]struct{}{}
	_, value, err := s.readPaymentPromotion()
	if err != nil || value.ImageResourceID == "" {
		return references
	}
	for _, resourceID := range resourceIDs {
		if resourceID == value.ImageResourceID {
			references[resourceID] = struct{}{}
		}
	}
	return references
}
