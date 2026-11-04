package database

import (
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// 保留本地 1–96 的迁移身份；官方推广中心不接入，分组只增加渠道权限。
func upstreamV1735Migrations() []migration {
	return []migration{
		{version: 97, name: "drama_episode_source_format", checksum: "sha256:drama-episode-source-format-v73-20261027", apply: func(tx *gorm.DB) error {
			if tx.Migrator().HasColumn(&model.DramaEpisode{}, "SourceFormat") {
				return nil
			}
			return tx.Migrator().AddColumn(&model.DramaEpisode{}, "SourceFormat")
		}},
		{version: 98, name: "drama_segment_references", checksum: "sha256:drama-segment-references-v74-20261027", apply: func(tx *gorm.DB) error {
			if tx.Migrator().HasColumn(&model.DramaSegment{}, "ReferenceCastIDsJSON") {
				return nil
			}
			return tx.Migrator().AddColumn(&model.DramaSegment{}, "ReferenceCastIDsJSON")
		}},
		{version: 99, name: "user_groups_channel_access", checksum: "sha256:user-groups-channel-access-v99", apply: migrateUserGroupChannelAccess},
		{version: 100, name: "agent_memory_document_skill_discovery", checksum: "sha256:agent-memory-document-skill-discovery-v76-20261028", apply: migrateAgentMemoryDocumentSkillDiscovery},
		{version: 101, name: "agent_memory_document_columns", checksum: "sha256:agent-memory-document-columns-v77-20261028", apply: migrateAgentMemoryDocumentColumns},
		{version: 102, name: "drama_cast_voice_looks", checksum: "sha256:drama-cast-voice-looks-v78-20261101", apply: migrateDramaCastVoiceLooks},
		{version: 103, name: "ecommerce_agent_and_knowledge", checksum: "sha256:ecommerce-agent-and-knowledge-v79-20261101", apply: migrateEcommerceFeature},
	}
}

func migrateUserGroupChannelAccess(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&model.UserGroup{}, &model.UserGroupChannel{}); err != nil {
		return err
	}
	for _, item := range []any{&model.User{}, &model.BillingOrder{}} {
		if tx.Migrator().HasColumn(item, "UserGroup") {
			continue
		}
		if err := tx.Migrator().AddColumn(item, "UserGroup"); err != nil {
			return err
		}
	}
	if !tx.Migrator().HasIndex(&model.User{}, "UserGroup") {
		return tx.Migrator().CreateIndex(&model.User{}, "UserGroup")
	}
	return nil
}

func migrateDramaCastVoiceLooks(tx *gorm.DB) error {
	for _, field := range []string{"VoiceProfileID", "VoiceSampleResourceID", "ParentID", "LookName", "LookFromChapter", "LookToChapter"} {
		if tx.Migrator().HasColumn(&model.DramaCast{}, field) {
			continue
		}
		if err := tx.Migrator().AddColumn(&model.DramaCast{}, field); err != nil {
			return err
		}
	}
	for column, value := range map[string]any{"voice_profile_id": "", "voice_sample_resource_id": "", "parent_id": "", "look_name": "", "look_from_chapter": 0, "look_to_chapter": 0} {
		if err := tx.Model(&model.DramaCast{}).Where(column+" IS NULL").Update(column, value).Error; err != nil {
			return err
		}
	}
	if !tx.Migrator().HasColumn(&model.DramaShot{}, "LookCastIDsJSON") {
		if err := tx.Migrator().AddColumn(&model.DramaShot{}, "LookCastIDsJSON"); err != nil {
			return err
		}
	}
	if err := tx.Model(&model.DramaShot{}).Where("look_cast_ids_json IS NULL").Update("look_cast_ids_json", "").Error; err != nil {
		return err
	}
	if tx.Migrator().HasIndex(&model.DramaCast{}, "ParentID") {
		return nil
	}
	return tx.Migrator().CreateIndex(&model.DramaCast{}, "ParentID")
}

func migrateAgentMemoryDocumentColumns(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&model.AgentMemoryDocument{}, &model.AgentMemoryDocumentRevision{}); err != nil {
		return err
	}
	// 新增列对旧行是 NULL；写成零值，与新建行一致。
	if err := tx.Model(&model.AgentMemoryDocument{}).Where("next_entry_seq IS NULL").Update("next_entry_seq", 0).Error; err != nil {
		return err
	}
	return tx.Model(&model.AgentMemoryDocumentRevision{}).Where("call_id IS NULL").Update("call_id", "").Error
}

func migrateAgentMemoryDocumentSkillDiscovery(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&model.AgentMemoryDocument{}, &model.AgentMemoryDocumentRevision{}); err != nil {
		return err
	}
	for _, field := range []string{"AgentMode", "AgentUseCount", "AgentLastUsedAt"} {
		if tx.Migrator().HasColumn(&model.UserSkillState{}, field) {
			continue
		}
		if err := tx.Migrator().AddColumn(&model.UserSkillState{}, field); err != nil {
			return err
		}
	}
	return nil
}
