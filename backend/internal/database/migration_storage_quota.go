package database

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 历史扩容只在迁移中读取；审计 ID 固定，事务重试不会再次增加个人配额。
func migrateStorageGrantsToQuota(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.PaymentOrder{}).Where("product_kind <> ? OR storage_bytes <> 0 OR storage_duration_days <> 0", "credits").Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return errors.New("存在存储或未知权益订单，停止升级，请先单独核实权益")
		}
		if err := tx.Model(&model.UserStorageGrant{}).Where("source <> ? OR payment_order_id <> '' OR product_id <> ''", "admin").Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return errors.New("存在非管理员赠送或商品关联扩容，停止升级，请先核实权益")
		}
		// 固定迁移发生前的默认值，后续默认值变化不应改变历史迁移。
		policy := struct {
			Resource struct {
				StoredFileGB int64 `json:"storedFileGB"`
			} `json:"resource"`
		}{}
		policy.Resource.StoredFileGB = 20
		var setting model.SystemSetting
		err := tx.First(&setting, "key = ?", "runtime_policy").Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			if setting.ValueJSON == "" {
				return errors.New("资源全局配置为空，停止扩容迁移")
			}
			if err := json.Unmarshal([]byte(setting.ValueJSON), &policy); err != nil {
				return err
			}
		}
		if policy.Resource.StoredFileGB < 1 || policy.Resource.StoredFileGB > 999 {
			return errors.New("全局文件配额不合法，停止扩容迁移")
		}
		now := time.Now()
		var grants []model.UserStorageGrant
		if err := tx.Where("revoked_at IS NULL AND starts_at <= ? AND (expires_at IS NULL OR expires_at > ?)", now, now).Order("user_id, id").Find(&grants).Error; err != nil {
			return err
		}
		byUser := make(map[string][]model.UserStorageGrant)
		for _, grant := range grants {
			byUser[grant.UserID] = append(byUser[grant.UserID], grant)
		}
		userIDs := make([]string, 0, len(byUser))
		for userID := range byUser {
			userIDs = append(userIDs, userID)
		}
		sort.Strings(userIDs)
		for _, userID := range userIDs {
			items := byUser[userID]
			digest := sha256.Sum256([]byte("storage-quota-v113:" + userID))
			auditID := fmt.Sprintf("%x-%x-%x-%x-%x", digest[:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
			if err := tx.Model(&model.AdminAuditEvent{}).Where("id = ?", auditID).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				continue
			}
			var user model.User
			if err := tx.First(&user, "id = ?", userID).Error; err != nil {
				return fmt.Errorf("赠送所属用户不存在：%w", err)
			}
			var quota model.UserResourceQuota
			err := tx.First(&quota, "user_id = ?", userID).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			overrides := map[string]int64{}
			if err == nil {
				if err := json.Unmarshal([]byte(quota.OverridesJSON), &overrides); err != nil || overrides == nil {
					return errors.New("个人资源配额格式无效，停止扩容迁移")
				}
			}
			baseGB := policy.Resource.StoredFileGB
			if value, ok := overrides["storedFileGB"]; ok {
				baseGB = value
			}
			if baseGB < 1 || baseGB > 999 {
				return errors.New("个人文件配额不合法，停止扩容迁移")
			}
			before, err := json.Marshal(overrides)
			if err != nil {
				return err
			}
			totalGB := baseGB
			ids := make([]string, 0, len(items))
			for _, grant := range items {
				if grant.Bytes <= 0 || grant.Bytes%(1<<30) != 0 || grant.Bytes/(1<<30) > 999-totalGB {
					return errors.New("赠送折入后超出个人配额合法范围或不是整 GB，停止升级")
				}
				totalGB += grant.Bytes / (1 << 30)
				ids = append(ids, grant.ID)
			}
			overrides["storedFileGB"] = totalGB
			after, err := json.Marshal(overrides)
			if err != nil {
				return err
			}
			quota.UserID, quota.OverridesJSON, quota.UpdatedAt = userID, string(after), now
			if quota.CreatedAt.IsZero() {
				quota.CreatedAt = now
			}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"overrides_json", "updated_at"})}).Create(&quota).Error; err != nil {
				return err
			}
			metadata, err := json.Marshal(map[string]any{"before": json.RawMessage(before), "after": overrides, "grantIds": ids, "baseGB": baseGB, "totalGB": totalGB, "migrationVersion": 113})
			if err != nil {
				return err
			}
			if err := tx.Create(&model.AdminAuditEvent{ID: auditID, Action: "storage_grants.migrate_to_quota", TargetType: "user", TargetID: userID, Summary: "历史管理员赠送折入用户文件配额（不再到期）", MetadataJSON: string(metadata), CreatedAt: now}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.TopupProduct{}).Where("kind = ?", "storage").Update("enabled", false).Error
	})
}
