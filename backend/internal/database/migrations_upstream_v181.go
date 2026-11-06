package database

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func upstreamV181Migrations() []migration {
	return []migration{
		{version: 106, name: "skill_admin_edits", checksum: "sha256:skill-admin-edits-v106-20261105", apply: migrateSkillAdminEdits},
		{version: 107, name: "token_price_rules", checksum: "sha256:token-price-rules-v107-20261105", apply: migrateTokenPriceRules},
		{version: 108, name: "pose_library", checksum: "sha256:pose-library-v108-20261105", apply: func(tx *gorm.DB) error {
			// 站点级姿势库：来源署名与参考图条目，只建新表；图片文件在数据目录 pose-library/ 下。
			return tx.AutoMigrate(&model.PoseLibrarySource{}, &model.PoseLibraryItem{})
		}},
		{version: 109, name: "ark_asset_groups", checksum: "sha256:ark-asset-groups-v109-20261105", apply: migrateArkAssetGroups},
		{version: 110, name: "inspiration_feed", checksum: "sha256:inspiration-feed-v110-20261105", apply: func(tx *gorm.DB) error {
			// 灵感上新：定时同步的上游灵感条目，只建新表；开关与同步记录存 system_settings。
			return tx.AutoMigrate(&model.InspirationFeedEntry{})
		}},
		{version: 111, name: "reference_video_billing", checksum: "sha256:reference-video-billing-v111-20261106", apply: migrateReferenceVideoBilling},
		{version: 112, name: "reference_video_cost_price", checksum: "sha256:reference-video-cost-price-v112-20261106", apply: migrateReferenceVideoCostPrice},
	}
}

// migrateReferenceVideoCostPrice 为价格档成本和账单成本快照增加带参考视频的成本价。
// 新列 NOT NULL DEFAULT 0，旧订单的成本口径不变。版本 111 自动开启参考视频计费的全 Token 视频模型，
// 挂参考视频的订单从此按带参考视频成本价记成本：这些 Token 档把带参考视频成本价回填为现有成本价，
// 否则升级后这类订单的成本会记成 0，直到管理员重新保存模型。
func migrateReferenceVideoCostPrice(tx *gorm.DB) error {
	columns := []string{"cost_reference_video_unit_price_microcredits", "cost_reference_video_output_token_price_microcredits"}
	for _, entity := range []any{&model.ChannelModelPriceTier{}, &model.BillingOrder{}} {
		if !tx.Migrator().HasTable(entity) {
			continue
		}
		for _, column := range columns {
			if tx.Migrator().HasColumn(entity, column) {
				continue
			}
			if err := tx.Migrator().AddColumn(entity, column); err != nil {
				return err
			}
		}
	}
	if !tx.Migrator().HasTable(&model.ChannelModel{}) || !tx.Migrator().HasTable(&model.ChannelModelPriceTier{}) {
		return nil
	}
	enabled := tx.Model(&model.ChannelModel{}).Select("id").Where("capability = ? AND reference_video_billing = ?", "video", true)
	return tx.Model(&model.ChannelModelPriceTier{}).
		Where("billing_mode = ? AND channel_model_id IN (?) AND cost_reference_video_output_token_price_microcredits = ?", "token", enabled, 0).
		Update("cost_reference_video_output_token_price_microcredits", gorm.Expr("cost_output_token_price_microcredits")).Error
}

// migrateReferenceVideoBilling 增加视频模型的“参考视频计费”开关、价格档的带参考视频价格和账单的参考视频快照。
// 开关默认关闭；只有已经全部按 Token 计费的视频模型开启开关，并把带参考视频的 Token 价格填成现价，
// 这些模型原本就把参考视频计入公式用量，迁移后扣费结果不变。含按秒价格档的模型保持关闭，
// 避免按秒订单因计入参考视频秒数而悄悄涨价。
func migrateReferenceVideoBilling(tx *gorm.DB) error {
	for _, target := range []struct {
		entity  any
		columns []string
	}{
		{&model.ChannelModel{}, []string{"reference_video_billing"}},
		{&model.ChannelModelPriceTier{}, []string{"reference_video_unit_price_microcredits", "reference_video_output_token_price_microcredits"}},
		{&model.BillingOrder{}, []string{"reference_video_count", "reference_video_millis", "reference_video_billed"}},
	} {
		if !tx.Migrator().HasTable(target.entity) {
			continue
		}
		for _, column := range target.columns {
			if tx.Migrator().HasColumn(target.entity, column) {
				continue
			}
			if err := tx.Migrator().AddColumn(target.entity, column); err != nil {
				return err
			}
		}
	}
	if !tx.Migrator().HasTable(&model.ChannelModel{}) || !tx.Migrator().HasTable(&model.ChannelModelPriceTier{}) {
		return nil
	}
	var tokenModelIDs, perSecondModelIDs []string
	if err := tx.Model(&model.ChannelModelPriceTier{}).Where("billing_mode = ?", "token").Distinct().Pluck("channel_model_id", &tokenModelIDs).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.ChannelModelPriceTier{}).Where("billing_mode = ?", "per_second").Distinct().Pluck("channel_model_id", &perSecondModelIDs).Error; err != nil {
		return err
	}
	excluded := make(map[string]bool, len(perSecondModelIDs))
	for _, id := range perSecondModelIDs {
		excluded[id] = true
	}
	candidates := make([]string, 0, len(tokenModelIDs))
	for _, id := range tokenModelIDs {
		if !excluded[id] {
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	var videoModelIDs []string
	if err := tx.Model(&model.ChannelModel{}).Where("capability = ? AND id IN ?", "video", candidates).Pluck("id", &videoModelIDs).Error; err != nil {
		return err
	}
	if len(videoModelIDs) == 0 {
		return nil
	}
	if err := tx.Model(&model.ChannelModel{}).Where("id IN ?", videoModelIDs).Update("reference_video_billing", true).Error; err != nil {
		return err
	}
	return tx.Model(&model.ChannelModelPriceTier{}).Where("billing_mode = ? AND channel_model_id IN ?", "token", videoModelIDs).
		Update("reference_video_output_token_price_microcredits", gorm.Expr("output_token_price_microcredits")).Error
}

// migrateArkAssetGroups 把方舟真人素材库扩展为真人 / 虚拟 / 版权 IP 三类素材组：
//   - 会话、素材组、素材补列；旧素材组均为用户自建的真人组，旧素材无需审核。
//   - 新建平台素材组授权表。
//   - 设置 JSON 的 portraitEnabled 改名为 realEnabled，保留管理员已开放的状态。
//   - 由旧版「加入素材库」写入的真人素材（metadata.source=ark-portrait）转为方舟认证分类，
//     并删除已废弃的手动 portraitCertified 标记。
func migrateArkAssetGroups(tx *gorm.DB) error {
	for _, target := range []struct {
		entity  any
		columns []string
	}{
		{&model.ArkPortraitSession{}, []string{"scope", "description", "billing_order_id"}},
		{&model.ArkPortraitGroup{}, []string{"kind", "scope", "description", "grant_all", "authorized_at", "expires_at", "billing_order_id"}},
		{&model.ArkPortraitAsset{}, []string{"library_asset_id", "reviewed_by", "reviewed_at"}},
	} {
		if !tx.Migrator().HasTable(target.entity) {
			continue
		}
		for _, column := range target.columns {
			if tx.Migrator().HasColumn(target.entity, column) {
				continue
			}
			if err := tx.Migrator().AddColumn(target.entity, column); err != nil {
				return fmt.Errorf("补充方舟素材列 %s：%w", column, err)
			}
		}
	}
	for _, index := range []struct {
		entity any
		field  string
	}{
		{&model.ArkPortraitGroup{}, "Kind"},
		{&model.ArkPortraitGroup{}, "Scope"},
		{&model.ArkPortraitAsset{}, "LibraryAssetID"},
	} {
		if !tx.Migrator().HasTable(index.entity) || tx.Migrator().HasIndex(index.entity, index.field) {
			continue
		}
		if err := tx.Migrator().CreateIndex(index.entity, index.field); err != nil {
			return fmt.Errorf("创建方舟素材索引 %s：%w", index.field, err)
		}
	}
	if err := tx.AutoMigrate(&model.ArkPortraitGroupGrant{}); err != nil {
		return fmt.Errorf("创建方舟素材组授权表：%w", err)
	}
	if err := migrateArkPortraitSettingKey(tx); err != nil {
		return err
	}
	return migrateArkPortraitLibraryAssets(tx)
}

func migrateArkPortraitSettingKey(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&model.SystemSetting{}) {
		return nil
	}
	var setting model.SystemSetting
	err := tx.Where("key = ?", "ark_private_assets").Limit(1).Find(&setting).Error
	if err != nil {
		return fmt.Errorf("读取方舟素材库配置：%w", err)
	}
	if setting.Key == "" || strings.TrimSpace(setting.ValueJSON) == "" {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(setting.ValueJSON), &value); err != nil {
		return fmt.Errorf("解析方舟素材库配置：%w", err)
	}
	legacy, ok := value["portraitEnabled"]
	if !ok {
		return nil
	}
	if _, exists := value["realEnabled"]; !exists {
		value["realEnabled"] = legacy
	}
	delete(value, "portraitEnabled")
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return tx.Model(&model.SystemSetting{}).Where("key = ?", setting.Key).Update("value_json", string(encoded)).Error
}

func migrateArkPortraitLibraryAssets(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&model.Asset{}) {
		return nil
	}
	var assets []model.Asset
	if err := tx.Where("payload_json LIKE ?", `%portraitCertified%`).Find(&assets).Error; err != nil {
		return fmt.Errorf("读取旧人像认证素材：%w", err)
	}
	now := time.Now().UTC()
	for _, asset := range assets {
		var payload map[string]any
		if err := json.Unmarshal([]byte(asset.PayloadJSON), &payload); err != nil {
			// 无法解析的历史记录保持原样，交给现有坏数据隔离逻辑。
			continue
		}
		if _, ok := payload["portraitCertified"]; !ok {
			continue
		}
		delete(payload, "portraitCertified")
		category := asset.Category
		metadata, _ := payload["metadata"].(map[string]any)
		arkAssetID, _ := payload["arkAssetId"].(string)
		if source, _ := metadata["source"].(string); source == "ark-portrait" && strings.TrimSpace(arkAssetID) != "" && asset.Kind == "image" {
			certifiedAt := asset.UpdatedAt
			if certifiedAt.IsZero() {
				certifiedAt = now
			}
			certification := map[string]any{"kind": "real", "scope": "user", "certifiedAt": certifiedAt.UTC().Format(time.RFC3339Nano), "previousCategory": string(asset.Category)}
			if arkGroupID, _ := metadata["arkPortraitGroupId"].(string); arkGroupID != "" {
				certification["arkGroupId"] = arkGroupID
			}
			var portrait model.ArkPortraitAsset
			if tx.Migrator().HasTable(&model.ArkPortraitAsset{}) {
				if err := tx.Where("user_id = ? AND ark_asset_id = ?", asset.UserID, strings.TrimSpace(arkAssetID)).Limit(1).Find(&portrait).Error; err != nil {
					return fmt.Errorf("读取方舟真人素材：%w", err)
				}
			}
			if portrait.ID != "" {
				certification["portraitAssetId"] = portrait.ID
				certification["groupId"] = portrait.GroupID
				if portrait.LibraryAssetID == "" {
					if err := tx.Model(&model.ArkPortraitAsset{}).Where("id = ?", portrait.ID).Update("library_asset_id", asset.ID).Error; err != nil {
						return fmt.Errorf("关联方舟真人素材：%w", err)
					}
				}
			}
			payload["arkCertification"] = certification
			payload["category"] = string(model.AssetCategoryArk)
			category = model.AssetCategoryArk
		}
		payload["updatedAt"] = now.Format(time.RFC3339Nano)
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.Asset{}).Where("id = ? AND user_id = ?", asset.ID, asset.UserID).Updates(map[string]any{"payload_json": string(encoded), "category": category, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("更新旧人像认证素材：%w", err)
		}
	}
	return nil
}

// migrateTokenPriceRules 为 Token 价格档增加分时与上下文规则，为订单增加规则快照与命中序号。
// 旧价格档规则为空且关闭，旧订单快照为空串，结算仍只用基础 Token 单价。
func migrateTokenPriceRules(tx *gorm.DB) error {
	for _, target := range []struct {
		entity  any
		columns []string
	}{
		{&model.ChannelModelPriceTier{}, []string{"token_price_rules_enabled", "token_price_rules"}},
		{&model.BillingOrder{}, []string{"token_price_rules_json", "token_price_rule"}},
	} {
		if !tx.Migrator().HasTable(target.entity) {
			continue
		}
		for _, column := range target.columns {
			if tx.Migrator().HasColumn(target.entity, column) {
				continue
			}
			if err := tx.Migrator().AddColumn(target.entity, column); err != nil {
				return err
			}
		}
	}
	return nil
}

// migrateSkillAdminEdits 为技能增加管理员修改时间。旧技能两列为空，内置技能启动同步和 GitHub 同步行为不变。
func migrateSkillAdminEdits(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&model.Skill{}) {
		return nil
	}
	for _, column := range []string{"admin_info_edited_at", "admin_content_edited_at"} {
		if tx.Migrator().HasColumn(&model.Skill{}, column) {
			continue
		}
		if err := tx.Migrator().AddColumn(&model.Skill{}, column); err != nil {
			return err
		}
	}
	return nil
}
