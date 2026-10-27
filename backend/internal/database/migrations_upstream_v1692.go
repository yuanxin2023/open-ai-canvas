package database

import (
	"fmt"
	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
)

const authNotificationsChecksum = "sha256:auth-notifications-v35-20260924"
const builtinSkillTombstonesChecksum = "sha256:builtin-skill-tombstones-v40-20260927"
const cloudAgentGeminiCacheChecksum = "sha256:cloud-agent-gemini-cache-v36-20260924"
const cloudAgentGeminiCacheIdentityChecksum = "sha256:cloud-agent-gemini-cache-identity-v37-20260925"
const legacyWorkbenchToolsMigrationChecksum = "sha256:workbench-tools-presets-v51-20261014"
const prefixedIDSequenceReconcileChecksum = "sha256:prefixed-id-sequence-reconcile-v38-20260926"
const resourceThumbnailChecksum = "sha256:resource-thumbnail-v41-20260927"
const skillLibraryCategoriesChecksum = "sha256:skill-library-categories-v39-20260926"
const workbenchToolsMigrationChecksum = "sha256:workbench-tools-presets-v54-20261014"
const workbenchToolsMigrationName = "workbench_tools_and_presets"

// Imported identities retain upstream signatures; only the unshipped local positions differ.
func upstreamV1692Migrations() []migration {
	return []migration{
		{version: 52, name: "oauth_state_accepted_terms", checksum: "sha256:oauth-state-accepted-terms-v33", apply: migrateOAuthStateAcceptedTerms},
		{version: 53, name: "task_media_recovery", checksum: "sha256:task-media-recovery-v34", apply: func(tx *gorm.DB) error {
			for _, field := range []string{"MediaRecoveryJSON", "MediaStage"} {
				if !tx.Migrator().HasColumn(&model.Task{}, field) {
					if err := tx.Migrator().AddColumn(&model.Task{}, field); err != nil {
						return err
					}
				}
			}
			return nil
		}},
		{version: 54, name: "auth_notifications", checksum: authNotificationsChecksum, apply: migrateSchemaV35},
		{version: 55, name: "cloud_agent_gemini_cache", checksum: cloudAgentGeminiCacheChecksum, apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.CloudAgentGeminiCache{})
		}},
		{version: 56, name: "cloud_agent_gemini_cache_identity", checksum: cloudAgentGeminiCacheIdentityChecksum, apply: migrateCloudAgentGeminiCacheIdentity},
		{version: 57, name: "prefixed_id_sequence_reconcile", checksum: prefixedIDSequenceReconcileChecksum, apply: migratePrefixedIDSequenceReconcile},
		{version: 58, name: "skill_library_categories", checksum: skillLibraryCategoriesChecksum, apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.SkillLibraryCategory{}, &model.UserSkillState{})
		}},
		{version: 59, name: "builtin_skill_tombstones", checksum: builtinSkillTombstonesChecksum, apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.BuiltinSkillTombstone{})
		}},
		{version: 60, name: "resource_thumbnail", checksum: resourceThumbnailChecksum, apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.Resource{})
		}},
		{version: 61, name: "cloud_agent_pi_sessions", checksum: "sha256:cloud-agent-pi-sessions-v42-20260928", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.CloudAgentPiSession{})
		}},
		{version: 62, name: "topup_sale_strategies", checksum: "sha256:topup-sale-strategies-v43-20260929", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.TopupProduct{}, &model.PaymentOrder{})
		}},
		{version: 63, name: "upload_reservations", checksum: "sha256:upload-reservations-v45-20261005", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.UploadReservation{})
		}},
		{version: 64, name: "skill_curation", checksum: "sha256:skill-curation-v46-20261005", apply: migrateSkillCuration},
		{version: 65, name: "skill_curation_roots", checksum: "sha256:skill-curation-roots-v47-20261005", apply: migrateSkillCurationRoots},
		{version: 66, name: "channel_proxy_url", checksum: "sha256:channel-proxy-url-v48-20261009", apply: func(tx *gorm.DB) error {
			if !tx.Migrator().HasColumn(&model.ModelChannel{}, "ProxyURL") {
				return tx.Migrator().AddColumn(&model.ModelChannel{}, "ProxyURL")
			}
			return nil
		}},
		{version: 67, name: "banner_sort_order", checksum: "sha256:banner-sort-order-v49-20261010", apply: func(tx *gorm.DB) error {
			if !tx.Migrator().HasColumn(&model.BannerAnnouncement{}, "SortOrder") {
				return tx.Migrator().AddColumn(&model.BannerAnnouncement{}, "SortOrder")
			}
			return nil
		}},
		{version: 68, name: "canvas_mutation_receipts", checksum: "sha256:canvas-mutation-receipts-v50-20261011", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.CanvasMutationReceipt{})
		}},
		{version: 69, name: "personal_profile_security", checksum: "sha256:personal-profile-security-v51-20261012", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.User{}, &model.AuthSession{}, &model.OAuthState{}, &model.AuthVerification{}, &model.UserAvatar{})
		}},
		{version: 70, name: "user_avatar_presets", checksum: "sha256:user-avatar-presets-v52-20261012", apply: func(tx *gorm.DB) error {
			if tx.Migrator().HasColumn(&model.User{}, "AvatarPreset") {
				return nil
			}
			return tx.Migrator().AddColumn(&model.User{}, "AvatarPreset")
		}},
		{version: 71, name: "user_login_logs", checksum: "sha256:user-login-logs-v53", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.LoginLog{})
		}},
		{version: 72, name: workbenchToolsMigrationName, checksum: workbenchToolsMigrationChecksum, apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.WorkbenchTool{}, &model.WorkbenchPreset{}, &model.Task{})
		}},
		{version: 73, name: "sql_perf_indexes", checksum: "sha256:sql-perf-indexes-v56-20261016", apply: migrateSchemaV56},
		{version: 74, name: "canvas_multi_session_sync", checksum: "sha256:canvas-multi-session-sync-v57", apply: migrateCanvasCollaboration},
		{version: 75, name: "agent_memory_proposals", checksum: "sha256:agent-memory-proposals-v58", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.AgentLesson{}, &model.AgentMemorySetting{}, &model.AgentMemoryProposal{}, &model.AgentMemoryRevision{})
		}},
		{version: 76, name: "channel_model_protocol_length", checksum: "sha256:channel-model-protocol-length-v59", apply: migrateChannelModelProtocolLength},
		{version: 77, name: "user_feedbacks", checksum: "sha256:user-feedbacks-v60-20261020", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.UserFeedback{})
		}},
	}
}

func migrateChannelModelProtocolLength(tx *gorm.DB) error {
	if tx.Dialector.Name() == "postgres" {
		return tx.Exec("ALTER TABLE channel_models ALTER COLUMN protocol TYPE varchar(64)").Error
	}
	return tx.Migrator().AlterColumn(&model.ChannelModel{}, "Protocol")
}

func migrateCloudAgentGeminiCacheIdentity(tx *gorm.DB) error {
	// v36 accidentally made cache_key globally unique while repository reads and
	// writes are user-scoped. Remove that index before creating the explicit
	// (user_id, cache_key) identity used by the model tags.
	for _, name := range []string{"idx_cloud_agent_gemini_caches_cache_key", "idx_cloud_agent_gemini_cache_cache_key"} {
		if tx.Migrator().HasIndex(&model.CloudAgentGeminiCache{}, name) {
			if err := tx.Migrator().DropIndex(&model.CloudAgentGeminiCache{}, name); err != nil {
				return fmt.Errorf("删除 Gemini 缓存旧唯一索引 %s：%w", name, err)
			}
		}
	}
	return tx.AutoMigrate(&model.CloudAgentGeminiCache{})
}

func migrateOAuthStateAcceptedTerms(tx *gorm.DB) error {
	if tx.Migrator().HasColumn(&model.OAuthState{}, "AcceptedTerms") {
		return nil
	}
	return tx.Migrator().AddColumn(&model.OAuthState{}, "AcceptedTerms")
}

func migratePrefixedIDSequenceReconcile(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&model.IDSequence{}); err != nil {
		return fmt.Errorf("创建可读 ID 序列表：%w", err)
	}
	return reconcilePrefixedIDSequences(tx)
}

func migrateSchemaV56(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&model.ApiCallLog{}, &model.Task{}, &model.CreditLedgerEntry{}); err != nil {
		return fmt.Errorf("迁移 SQL 性能索引：%w", err)
	}
	if tx.Dialector.Name() == "sqlite" {
		if err := tx.Exec("UPDATE api_call_logs SET created_at_epoch = unixepoch(created_at) WHERE created_at_epoch = 0 OR created_at_epoch IS NULL").Error; err != nil {
			return fmt.Errorf("回填 api_call_logs 时间戳：%w", err)
		}
		if err := tx.Exec("UPDATE tasks SET created_at_epoch = unixepoch(created_at) WHERE created_at_epoch = 0 OR created_at_epoch IS NULL").Error; err != nil {
			return fmt.Errorf("回填 tasks 时间戳：%w", err)
		}
	}
	return nil
}
