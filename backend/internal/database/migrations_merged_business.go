package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func mergedBusinessMigrations() []migration {
	return []migration{
		{version: 78, name: "merged_login_environment_contract", checksum: "sha256:merged-login-environment-contract-v1", apply: func(tx *gorm.DB) error { return tx.AutoMigrate(&model.LoginLog{}) }},
		{version: 79, name: "payment_order_channel_confirmation", checksum: "sha256:payment-order-channel-confirmation-v1", apply: migratePaymentOrderConfirmation},
		{version: 80, name: "billing_refund_recovery_state", checksum: "sha256:billing-refund-recovery-state-v1", apply: func(tx *gorm.DB) error { return tx.AutoMigrate(&model.BillingOrder{}) }},
		{version: 81, name: "workbench_submission_receipts", checksum: "sha256:workbench-submission-receipts-v1", apply: func(tx *gorm.DB) error { return tx.AutoMigrate(&model.WorkbenchSubmission{}) }},
		{version: 82, name: "user_inspiration_library_state", checksum: "sha256:user-inspiration-library-state-v1", apply: func(tx *gorm.DB) error { return tx.AutoMigrate(&model.UserInspirationLibrary{}) }},
		{version: 83, name: "feedback_versions_and_resource_references", checksum: "sha256:feedback-versions-resource-references-v1", apply: migrateFeedbackResourceReferences},
	}
}

func migratePaymentOrderConfirmation(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&model.PaymentOrder{}); err != nil {
		return err
	}
	// The deployed local line had unlimited products. An existing inventory
	// reservation without its own snapshot cannot be inferred from today's product.
	var ambiguous int64
	if err := tx.Model(&model.PaymentOrder{}).Where("(sale_strategy_snapshot IS NULL OR sale_strategy_snapshot = '') AND stock_reserved = ?", true).Count(&ambiguous).Error; err != nil {
		return err
	}
	if ambiguous != 0 {
		return errors.New("旧库存订单缺少售卖策略快照，停止升级；请基于副本核对，不推定历史策略或渠道关单")
	}
	return tx.Model(&model.PaymentOrder{}).Where("sale_strategy_snapshot IS NULL OR sale_strategy_snapshot = ''").UpdateColumn("sale_strategy_snapshot", model.TopupSaleStrategyUnlimited).Error
}

func migrateFeedbackResourceReferences(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&model.UserFeedback{}, &model.FeedbackResourceReference{}); err != nil {
		return err
	}
	var invalid int64
	if err := tx.Model(&model.UserFeedback{}).Where("id IS NULL OR id = '' OR user_id IS NULL OR user_id = '' OR version IS NULL OR version < 1").Count(&invalid).Error; err != nil {
		return err
	}
	if invalid != 0 {
		return errors.New("反馈存在无效身份、归属或版本，停止迁移并核对")
	}
	after := ""
	for {
		var rows []model.UserFeedback
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id > ?", after).Order("id ASC").Limit(100).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			if row.Version < 1 {
				return fmt.Errorf("反馈 %s 版本无效，停止迁移并核对", row.ID)
			}
			encoded, resources, err := migratedFeedbackAttachments(tx, row)
			if err != nil {
				// Do not put historical URLs, contact details or content in errors.
				return fmt.Errorf("反馈 %s 附件无法安全转换，停止迁移：%w", row.ID, err)
			}
			for _, resourceID := range resources {
				ref := model.FeedbackResourceReference{FeedbackID: row.ID, ResourceID: resourceID, UserID: row.UserID}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ref).Error; err != nil {
					return err
				}
				var stored model.FeedbackResourceReference
				if err := tx.First(&stored, "feedback_id = ? AND resource_id = ?", row.ID, resourceID).Error; err != nil {
					return err
				}
				if stored.UserID != row.UserID {
					return fmt.Errorf("反馈 %s 的资源引用归属不一致，停止迁移", row.ID)
				}
			}
			if encoded != row.Attachments {
				result := tx.Model(&model.UserFeedback{}).Where("id = ? AND version = ? AND COALESCE(attachments, '') = ?", row.ID, row.Version, row.Attachments).UpdateColumn("attachments", encoded)
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return fmt.Errorf("反馈 %s 在迁移期间发生变化，停止迁移", row.ID)
				}
			}
			after = row.ID
		}
	}
}

func migratedFeedbackAttachments(tx *gorm.DB, row model.UserFeedback) (string, []string, error) {
	raw := strings.TrimSpace(row.Attachments)
	if raw == "" || raw == "null" {
		return "[]", nil, nil
	}
	// Oversized or damaged historical data is reported, never silently truncated.
	if len(raw) > 1024*1024 || !strings.HasPrefix(raw, "[") {
		return "", nil, errors.New("附件列表格式或大小异常")
	}
	var attachments []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &attachments); err != nil {
		return "", nil, errors.New("附件列表不是有效 JSON")
	}
	seen := map[string]bool{}
	resourceIDs := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		if attachment == nil {
			return "", nil, errors.New("存在空附件")
		}
		id, err := historicalFeedbackResourceID(attachment, row.ID)
		if err != nil {
			return "", nil, err
		}
		var resource model.Resource
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&resource, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "", nil, errors.New("附件资源不存在")
			}
			return "", nil, errors.New("无法核对附件资源")
		}
		if row.UserID == "" || resource.UserID != row.UserID {
			return "", nil, errors.New("附件资源归属不一致")
		}
		if resource.Status != model.ResourceStatusReady || (resource.Kind != "image" && resource.Kind != "video") || !strings.HasPrefix(resource.MimeType, resource.Kind+"/") || resource.Size <= 0 || resource.Size > 50*1024*1024 {
			return "", nil, errors.New("附件资源状态、类型或大小异常")
		}
		for key, value := range map[string]any{
			"id": id, "resourceId": id, "type": resource.Kind,
			"url":  "/api/feedbacks/" + row.ID + "/attachments/" + id,
			"size": resource.Size, "mimeType": resource.MimeType,
		} {
			encoded, err := json.Marshal(value)
			if err != nil {
				return "", nil, err
			}
			attachment[key] = encoded
		}
		if !seen[id] {
			resourceIDs = append(resourceIDs, id)
			seen[id] = true
		}
	}
	encoded, err := json.Marshal(attachments)
	return string(encoded), resourceIDs, err
}

func historicalFeedbackResourceID(attachment map[string]json.RawMessage, feedbackID string) (string, error) {
	id := ""
	for _, key := range []string{"resourceId", "id"} {
		var candidate string
		if value, exists := attachment[key]; exists {
			if err := json.Unmarshal(value, &candidate); err != nil {
				return "", errors.New("附件标识类型无效")
			}
		}
		if candidate == "" {
			continue
		}
		if strings.ContainsAny(candidate, "/\\?#% \t\r\n") || len(candidate) > 36 || (id != "" && candidate != id) {
			return "", errors.New("附件标识无效或互相冲突")
		}
		id = candidate
	}
	var rawURL string
	if value, exists := attachment["url"]; exists {
		if err := json.Unmarshal(value, &rawURL); err != nil {
			return "", errors.New("附件地址类型无效")
		}
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.New("附件地址格式无效")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	candidate := ""
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "resources" && parts[3] == "file" {
		candidate = parts[2]
	} else if len(parts) == 5 && parts[0] == "api" && parts[1] == "feedbacks" && parts[2] == feedbackID && parts[3] == "attachments" {
		candidate = parts[4]
	}
	if candidate != "" {
		if strings.ContainsAny(candidate, "/\\?#% \t\r\n") || len(candidate) > 36 || (id != "" && id != candidate) {
			return "", errors.New("附件地址与资源标识不一致")
		}
		id = candidate
	}
	if id == "" {
		return "", errors.New("无法识别附件对应的资源")
	}
	return id, nil
}
