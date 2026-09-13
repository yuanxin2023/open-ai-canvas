package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const (
	customerServiceSettingKey         = "customer_service"
	customerServiceSchemaVersion      = 2
	customerServiceButtonImageMaxSize = int64(2 << 20)
)

var customerServiceColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type CustomerServiceSetting struct {
	SchemaVersion   int    `json:"schemaVersion"`
	Enabled         bool   `json:"enabled"`
	Position        string `json:"position"`
	DisplayType     string `json:"displayType"`
	Color           string `json:"color"`
	Label           string `json:"label"`
	ImageResourceID string `json:"imageResourceId"`
	Draggable       bool   `json:"draggable"`
	DesktopEnabled  bool   `json:"desktopEnabled"`
	MobileEnabled   bool   `json:"mobileEnabled"`
	ButtonSize      int    `json:"buttonSize"`
	OffsetX         int    `json:"offsetX"`
	OffsetY         int    `json:"offsetY"`
}

type PublicCustomerServiceSetting struct {
	SchemaVersion   int       `json:"schemaVersion"`
	Enabled         bool      `json:"enabled"`
	Position        string    `json:"position"`
	DisplayType     string    `json:"displayType"`
	Color           string    `json:"color"`
	Label           string    `json:"label"`
	ImageURL        string    `json:"imageUrl"`
	ImageConfigured bool      `json:"imageConfigured"`
	Draggable       bool      `json:"draggable"`
	DesktopEnabled  bool      `json:"desktopEnabled"`
	MobileEnabled   bool      `json:"mobileEnabled"`
	ButtonSize      int       `json:"buttonSize"`
	OffsetX         int       `json:"offsetX"`
	OffsetY         int       `json:"offsetY"`
	Configured      bool      `json:"configured"`
	Revision        string    `json:"revision"`
	UpdatedAt       time.Time `json:"updatedAt,omitempty"`
}

type AdminCustomerServiceSetting struct {
	CustomerServiceSetting
	Public     PublicCustomerServiceSetting `json:"public"`
	Configured bool                         `json:"configured"`
	UpdatedBy  string                       `json:"updatedBy,omitempty"`
	CreatedAt  time.Time                    `json:"createdAt,omitempty"`
	UpdatedAt  time.Time                    `json:"updatedAt,omitempty"`
}

func defaultCustomerServiceSetting() CustomerServiceSetting {
	return CustomerServiceSetting{
		SchemaVersion:  customerServiceSchemaVersion,
		Enabled:        true,
		Position:       "bottom-right",
		DisplayType:    "circle",
		Color:          "#2563EB",
		Label:          "联系客服",
		Draggable:      false,
		DesktopEnabled: true,
		MobileEnabled:  true,
		ButtonSize:     56,
		OffsetX:        24,
		OffsetY:        24,
	}
}

func CustomerServiceButtonImageMaxBytes() int64 {
	return customerServiceButtonImageMaxSize
}

func (s *Service) CustomerService() (*PublicCustomerServiceSetting, error) {
	setting, value, err := s.readCustomerService()
	if err != nil {
		return nil, err
	}
	value = s.resolveAvailableCustomerServiceImage(value)
	return publicCustomerServiceSetting(setting, value), nil
}

func (s *Service) AdminCustomerService(actor *model.User) (*AdminCustomerServiceSetting, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	setting, value, err := s.readCustomerService()
	if err != nil {
		return nil, err
	}
	value = s.resolveAvailableCustomerServiceImage(value)
	result := &AdminCustomerServiceSetting{
		CustomerServiceSetting: value,
		Public:                 *publicCustomerServiceSetting(setting, value),
		Configured:             setting != nil,
	}
	if setting != nil {
		result.UpdatedBy = setting.UpdatedBy
		result.CreatedAt = setting.CreatedAt
		result.UpdatedAt = setting.UpdatedAt
	}
	return result, nil
}

func (s *Service) UpdateCustomerService(actor *model.User, value CustomerServiceSetting) (*AdminCustomerServiceSetting, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	value.SchemaVersion = customerServiceSchemaVersion
	value.Position = strings.TrimSpace(value.Position)
	value.DisplayType = strings.TrimSpace(value.DisplayType)
	value.Color = strings.ToUpper(strings.TrimSpace(value.Color))
	value.Label = strings.TrimSpace(value.Label)
	value.ImageResourceID = strings.TrimSpace(value.ImageResourceID)
	if err := validateCustomerServiceSetting(value); err != nil {
		return nil, err
	}

	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	current, before, err := s.readCustomerService()
	if err != nil {
		return nil, err
	}
	if err := s.validateCustomerServiceImage(actor, value.ImageResourceID, before.ImageResourceID); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	setting := model.SystemSetting{Key: customerServiceSettingKey, ValueJSON: string(encoded), UpdatedBy: actor.ID}
	if current != nil {
		setting.CreatedAt = current.CreatedAt
	}
	if err := s.repo.SaveSystemSetting(&setting); err != nil {
		return nil, err
	}
	if err := s.appendAdminAudit(actor, "customer_service.update", "system_setting", customerServiceSettingKey, "更新客服展示配置", map[string]any{"before": before, "after": value}); err != nil {
		return nil, err
	}
	return s.AdminCustomerService(actor)
}

func (s *Service) UploadCustomerServiceButtonImage(actor *model.User, header *multipart.FileHeader) (*model.Resource, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	if err := validateCustomerServiceButtonImageUpload(header); err != nil {
		return nil, err
	}
	header.Header.Set("Content-Type", "image/png")
	resource, err := s.uploadLocalResource(actor.ID, header, "image", 0, 0, 0)
	if err != nil {
		return nil, err
	}
	resource.PublicURL = ""
	return resource, nil
}

func (s *Service) OpenCustomerServiceButtonImage(rangeHeader string) (*ResourceStream, error) {
	_, value, err := s.readCustomerService()
	if err != nil {
		return nil, err
	}
	if value.ImageResourceID == "" {
		return nil, NotFound("未配置客服按钮图片")
	}
	resource, err := s.repo.Resource(value.ImageResourceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NotFound("客服按钮图片不存在")
		}
		return nil, err
	}
	if err := validateCustomerServiceImageResource(resource); err != nil {
		return nil, err
	}
	return s.openResourceRange(resource.UserID, resource, rangeHeader)
}

func (s *Service) readCustomerService() (*model.SystemSetting, CustomerServiceSetting, error) {
	setting, err := s.repo.SystemSetting(customerServiceSettingKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, defaultCustomerServiceSetting(), nil
	}
	if err != nil {
		return nil, CustomerServiceSetting{}, err
	}
	value := defaultCustomerServiceSetting()
	if strings.TrimSpace(setting.ValueJSON) == "" || json.Unmarshal([]byte(setting.ValueJSON), &value) != nil {
		return nil, CustomerServiceSetting{}, errors.New("客服配置格式无效")
	}
	value.SchemaVersion = customerServiceSchemaVersion
	value.Position = strings.TrimSpace(value.Position)
	value.DisplayType = strings.TrimSpace(value.DisplayType)
	value.Color = strings.ToUpper(strings.TrimSpace(value.Color))
	value.Label = strings.TrimSpace(value.Label)
	value.ImageResourceID = strings.TrimSpace(value.ImageResourceID)
	return setting, value, nil
}

func validateCustomerServiceSetting(value CustomerServiceSetting) error {
	if !oneOf(value.Position, "bottom-right", "bottom-left", "top-right", "top-left") {
		return BadAuthRequest("客服按钮位置无效")
	}
	if !oneOf(value.DisplayType, "circle", "pill", "icon-text", "custom-image") {
		return BadAuthRequest("客服按钮显示形态无效")
	}
	if !customerServiceColorPattern.MatchString(value.Color) {
		return BadAuthRequest("客服按钮颜色必须是六位十六进制颜色")
	}
	if utf8.RuneCountInString(value.Label) == 0 || utf8.RuneCountInString(value.Label) > 20 {
		return BadAuthRequest("客服按钮文字必须为 1 到 20 个字符")
	}
	for _, char := range value.Label {
		if unicode.IsControl(char) {
			return BadAuthRequest("客服按钮文字不能包含控制字符")
		}
	}
	if value.DisplayType == "custom-image" && value.ImageResourceID == "" {
		return BadAuthRequest("使用自定义图片前请先上传 PNG 按钮图片")
	}
	if len(value.ImageResourceID) > 80 {
		return BadAuthRequest("客服按钮图片资源 ID 无效")
	}
	if value.OffsetX < 8 || value.OffsetX > 200 || value.OffsetY < 8 || value.OffsetY > 200 {
		return BadAuthRequest("客服按钮边缘距离必须在 8 到 200 像素之间")
	}
	if value.ButtonSize < 20 || value.ButtonSize > 96 {
		return BadAuthRequest("客服按钮大小必须在 20 到 96 像素之间")
	}
	return nil
}

func validateCustomerServiceButtonImageUpload(header *multipart.FileHeader) error {
	if header == nil || header.Size <= 0 || header.Size > customerServiceButtonImageMaxSize {
		return BadAuthRequest(fmt.Sprintf("客服按钮 PNG 图片大小必须在 %dMB 以内", customerServiceButtonImageMaxSize>>20))
	}
	file, err := header.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	buffer := make([]byte, 512)
	read, readErr := file.Read(buffer)
	if readErr != nil && read == 0 {
		return BadAuthRequest("客服按钮图片内容无法读取")
	}
	mimeType := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(buffer[:read]), ";")[0]))
	if mimeType != "image/png" {
		return BadAuthRequest("客服按钮图片仅支持 PNG 格式")
	}
	return nil
}

func (s *Service) validateCustomerServiceImage(actor *model.User, resourceID string, currentID string) error {
	if resourceID == "" {
		return nil
	}
	resource, err := s.repo.Resource(resourceID)
	if err != nil {
		return BadAuthRequest("选择的客服按钮图片不存在")
	}
	if resourceID != currentID && resource.UserID != actor.ID {
		return Forbidden("只能使用当前管理员上传的客服按钮图片")
	}
	return validateCustomerServiceImageResource(resource)
}

func validateCustomerServiceImageResource(resource *model.Resource) error {
	if resource == nil || resource.Status != model.ResourceStatusReady || resource.Kind != "image" {
		return BadAuthRequest("客服按钮图片尚未上传完成")
	}
	mimeType := strings.ToLower(strings.TrimSpace(strings.Split(resource.MimeType, ";")[0]))
	if mimeType != "image/png" {
		return BadAuthRequest("客服按钮图片必须是 PNG 格式")
	}
	return nil
}

func (s *Service) resolveAvailableCustomerServiceImage(value CustomerServiceSetting) CustomerServiceSetting {
	if value.ImageResourceID == "" {
		return value
	}
	resource, err := s.repo.Resource(value.ImageResourceID)
	if err != nil || validateCustomerServiceImageResource(resource) != nil {
		value.ImageResourceID = ""
		if value.DisplayType == "custom-image" {
			value.DisplayType = "circle"
		}
		return value
	}
	if resource.Provider == "local" {
		info, statErr := os.Stat(filepath.Join(s.dataDir, "resources", filepath.FromSlash(resource.ObjectKey)))
		if statErr != nil || info.IsDir() {
			value.ImageResourceID = ""
			if value.DisplayType == "custom-image" {
				value.DisplayType = "circle"
			}
		}
	}
	return value
}

func publicCustomerServiceSetting(setting *model.SystemSetting, value CustomerServiceSetting) *PublicCustomerServiceSetting {
	revision := "builtin"
	if setting != nil {
		revision = strconv.FormatInt(setting.UpdatedAt.UTC().UnixNano(), 36)
	}
	result := &PublicCustomerServiceSetting{
		SchemaVersion:  customerServiceSchemaVersion,
		Enabled:        value.Enabled,
		Position:       value.Position,
		DisplayType:    value.DisplayType,
		Color:          value.Color,
		Label:          value.Label,
		Draggable:      value.Draggable,
		DesktopEnabled: value.DesktopEnabled,
		MobileEnabled:  value.MobileEnabled,
		ButtonSize:     value.ButtonSize,
		OffsetX:        value.OffsetX,
		OffsetY:        value.OffsetY,
		Configured:     setting != nil,
		Revision:       revision,
	}
	if setting != nil {
		result.UpdatedAt = setting.UpdatedAt
	}
	if value.ImageResourceID != "" {
		result.ImageConfigured = true
		result.ImageURL = "/api/public/customer-service/button-image?v=" + revision
	}
	return result
}

func (s *Service) customerServiceResourceReferences(resourceIDs []string) map[string][]AdminResourceReferenceView {
	result := make(map[string][]AdminResourceReferenceView)
	_, value, err := s.readCustomerService()
	if err != nil {
		for _, resourceID := range resourceIDs {
			result[resourceID] = []AdminResourceReferenceView{{Kind: "客服", ID: customerServiceSettingKey, Title: "客服配置无法读取"}}
		}
		return result
	}
	wanted := make(map[string]struct{}, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		wanted[resourceID] = struct{}{}
	}
	if _, exists := wanted[value.ImageResourceID]; exists && value.ImageResourceID != "" {
		result[value.ImageResourceID] = []AdminResourceReferenceView{{Kind: "客服", ID: customerServiceSettingKey, Title: "用户端客服按钮图片"}}
	}
	return result
}
