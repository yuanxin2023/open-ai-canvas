package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

type tableMigration struct {
	name string
	run  func(source *gorm.DB, target *gorm.DB, copyRows bool) (int, error)
}

func main() {
	sourcePath := strings.TrimSpace(os.Getenv("SQLITE_SOURCE_PATH"))
	targetDSN := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if sourcePath == "" || targetDSN == "" {
		log.Fatal("必须配置 SQLITE_SOURCE_PATH 和 DATABASE_URL")
	}
	result, err := runConversion(sourcePath, targetDSN)
	if err != nil {
		log.Fatal(safeConversionError(err))
	}
	if result.Copied {
		log.Printf("转库事务已提交：%d 张表（含原迁移账本），%d 行", result.Tables, result.Rows)
	} else {
		log.Printf("目标库与源快照一致，未重复写入：%d 张表，%d 行", result.Tables, result.Rows)
	}
}

func verifyMigrationCoverage(db *gorm.DB) error {
	expected := map[string]struct{}{"schema_migrations": {}}
	for _, value := range database.Models() {
		statement := &gorm.Statement{DB: db}
		if err := statement.Parse(value); err != nil {
			return err
		}
		expected[statement.Schema.Table] = struct{}{}
	}
	for _, migration := range migrations() {
		if _, exists := expected[migration.name]; !exists {
			return fmt.Errorf("迁移清单包含未知表 %s", migration.name)
		}
		delete(expected, migration.name)
	}
	if len(expected) > 0 {
		missing := make([]string, 0, len(expected))
		for name := range expected {
			missing = append(missing, name)
		}
		return fmt.Errorf("迁移清单缺少表：%s", strings.Join(missing, ", "))
	}
	return nil
}

func migrations() []tableMigration {
	return []tableMigration{
		migrateTable[database.MigrationLedgerRecord]("schema_migrations"),
		migrateTable[model.Tool]("tools"),
		migrateTable[model.ToolFavorite]("tool_favorites"),
		migrateTable[model.User]("users"),
		migrateTable[model.AdminPermissionGrant]("admin_permission_grants"),
		migrateTable[model.UserAvatar]("user_avatars"),
		migrateTable[model.UserFeedback]("user_feedbacks"),
		migrateTable[model.DramaProduction]("drama_productions"),
		migrateTable[model.DramaCast]("drama_casts"),
		migrateTable[model.DramaEpisode]("drama_episodes"),
		migrateTable[model.DramaShot]("drama_shots"),
		migrateTable[model.DramaTake]("drama_takes"),
		migrateTable[model.DramaSegment]("drama_segments"),
		migrateTable[model.DramaJob]("drama_jobs"),
		migrateTable[model.AuthSession]("auth_sessions"),
		migrateTable[model.LoginLog]("login_logs"),
		migrateTable[model.UserIdentity]("user_identities"),
		migrateTable[model.UserLoginEvent]("user_login_events"),
		migrateTable[model.UserUsernameChange]("user_username_changes"),
		migrateTable[model.OAuthState]("o_auth_states"),
		migrateTable[model.EmailVerificationCode]("email_verification_codes"),
		migrateTable[model.AuthVerification]("auth_verifications"),
		migrateTable[model.NotificationQuota]("notification_quota"),
		migrateTable[model.SMSChannel]("sms_channels"),
		migrateTable[model.SMSRecord]("sms_records"),
		migrateTable[model.ModelChannel]("model_channels"),
		migrateTable[model.ChannelModel]("channel_models"),
		migrateTable[model.ChannelModelPriceTier]("channel_model_price_tiers"),
		migrateTable[model.IDSequence]("id_sequences"),
		migrateTable[model.LogicalModel]("logical_models"),
		migrateTable[model.LogicalModelRevision]("logical_model_revisions"),
		migrateTable[model.LogicalModelRoute]("logical_model_routes"),
		migrateTable[model.RouteAttempt]("route_attempts"),
		migrateTable[model.ApiCallLog]("api_call_logs"),
		migrateTable[model.ModelPricing]("model_pricings"),
		migrateTable[model.CreditAccount]("credit_accounts"),
		migrateTable[model.CreditLedgerEntry]("credit_ledger_entries"),
		migrateTable[model.ReferralProfile]("referral_profiles"),
		migrateTable[model.ReferralReward]("referral_rewards"),
		migrateTable[model.BillingOrder]("billing_orders"),
		migrateTable[model.TopupProduct]("topup_products"),
		migrateTable[model.PaymentProviderConfig]("payment_provider_configs"),
		migrateTable[model.PaymentOrder]("payment_orders"),
		migrateTable[model.PaymentNotification]("payment_notifications"),
		migrateTable[model.PaymentReconciliationRun]("payment_reconciliation_runs"),
		migrateTable[model.PaymentReconciliationItem]("payment_reconciliation_items"),
		migrateTable[model.UserStorageGrant]("user_storage_grants"),
		migrateTable[model.UserStorageState]("user_storage_states"),
		migrateTable[model.RedeemBatch]("redeem_batches"),
		migrateTable[model.RedeemCode]("redeem_codes"),
		migrateTable[model.AdminAuditEvent]("admin_audit_events"),
		migrateTable[model.UserDailyActivity]("user_daily_activities"),
		migrateTable[model.SystemSetting]("system_settings"),
		migrateTable[model.PluginPlatformState]("plugin_platform_states"),
		migrateTable[model.UserPluginState]("user_plugin_states"),
		migrateTable[model.ArkPrivateAssetBinding]("ark_private_asset_bindings"),
		migrateTable[model.UserOSSSetting]("user_oss_settings"),
		migrateTable[model.StorageLocation]("storage_locations"),
		migrateTable[model.UserDailyUploadUsage]("user_daily_upload_usages"),
		migrateTable[model.UserResourceQuota]("user_resource_quotas"),
		migrateTable[model.UploadReservation]("upload_reservations"),
		migrateTable[model.Skill]("skills"),
		migrateTable[model.SkillPlatformState]("skill_platform_states"),
		migrateTable[model.SkillCategoryPlatformState]("skill_category_platform_states"),
		migrateTable[model.SkillLibraryCategory]("skill_library_categories"),
		migrateTable[model.SkillCurationSetting]("skill_curation_settings"),
		migrateTable[model.SkillCurationCategory]("skill_curation_categories"),
		migrateTable[model.SkillCurationAssignment]("skill_curation_assignments"),
		migrateTable[model.SkillCurationRoot]("skill_curation_roots"),
		migrateTable[model.SkillCurationRootAssignment]("skill_curation_root_assignments"),
		migrateTable[model.SkillVersion]("skill_versions"),
		migrateTable[model.SkillFile]("skill_files"),
		migrateTable[model.UserSkillState]("user_skill_states"),
		migrateTable[model.BuiltinSkillTombstone]("builtin_skill_tombstones"),
		migrateTable[model.Resource]("resources"),
		migrateTable[model.FeedbackResourceReference]("feedback_resource_references"),
		migrateTable[model.ResourceDeletionJob]("resource_deletion_jobs"),
		migrateTable[model.PaymentPromotionImageDraft]("payment_promotion_image_drafts"),
		migrateTable[model.ResourceOffloadJob]("resource_offload_jobs"),
		migrateTable[model.AnnouncementImageDraft]("announcement_image_drafts"),
		migrateTable[model.InspirationCoverDraft]("inspiration_cover_drafts"),
		migrateTable[model.Asset]("assets"),
		migrateTable[model.AssetFolder]("asset_folders"),
		migrateTable[model.ProjectAssetLink]("project_asset_links"),
		migrateTable[model.ProjectAssetFolder]("project_asset_folders"),
		migrateTable[model.ProjectAssetCandidate]("project_asset_candidates"),
		migrateTable[model.AssetVersion]("asset_versions"),
		migrateTable[model.AssetRepresentation]("asset_representations"),
		migrateTable[model.VoiceProfile]("voice_profiles"),
		migrateTable[model.CharacterVoiceBinding]("character_voice_bindings"),
		migrateTable[model.Project]("projects"),
		migrateTable[model.StyleProfile]("style_profiles"),
		migrateTable[model.ProjectUnit]("project_units"),
		migrateTable[model.CanvasUnitLink]("canvas_unit_links"),
		migrateTable[model.Shot]("shots"),
		migrateTable[model.ShotRevision]("shot_revisions"),
		migrateTable[model.ShotArtifact]("shot_artifacts"),
		migrateTable[model.ShotAssetReference]("shot_asset_references"),
		migrateTable[model.WorkflowTemplateVersion]("workflow_template_versions"),
		migrateTable[model.WorkflowInstance]("workflow_instances"),
		migrateTable[model.WorkflowStepInstance]("workflow_step_instances"),
		migrateTable[model.WorkflowStepTask]("workflow_step_tasks"),
		migrateTable[model.ProductionTaskLink]("production_task_links"),
		migrateTable[model.CanvasProject]("canvas_projects"),
		migrateTable[model.CanvasMutationReceipt]("canvas_mutation_receipts"),
		migrateTable[model.CanvasEditLease]("canvas_edit_leases"),
		migrateTable[model.CanvasSnapshot]("canvas_snapshots"),
		migrateTable[model.CanvasSnapshotResource]("canvas_snapshot_resources"),
		migrateTable[model.CanvasShare]("canvas_shares"),
		migrateTable[model.PromptTemplate]("prompt_templates"),
		migrateTable[model.UserPromptCustomization]("user_prompt_customizations"),
		migrateTable[model.UserPrompt]("user_prompts"),
		migrateTable[model.UserInspirationLibrary]("user_inspiration_libraries"),
		migrateTable[model.Announcement]("announcements"),
		migrateTable[model.Inspiration]("inspirations"),
		migrateTable[model.UserAnnouncementRead]("user_announcement_reads"),
		migrateTable[model.BannerAnnouncement]("banner_announcements"),
		migrateTable[model.CreationRun]("creation_runs"),
		migrateTable[model.CreationSubmission]("creation_submissions"),
		migrateTable[model.Task]("tasks"),
		migrateTable[model.WorkbenchSubmission]("workbench_submissions"),
		migrateTable[model.WorkbenchTool]("workbench_tools"),
		migrateTable[model.WorkbenchPreset]("workbench_presets"),
		migrateTable[model.CloudAgentExecution]("cloud_agent_executions"),
		migrateTable[model.CloudAgentEventRecord]("cloud_agent_event_records"),
		migrateTable[model.CloudAgentMessageRecord]("cloud_agent_message_records"),
		migrateTable[model.CloudAgentCanvasMutation]("cloud_agent_canvas_mutations"),
		migrateTable[model.CloudAgentResourceLease]("cloud_agent_resource_leases"),
		migrateTable[model.CloudAgentPiSession]("cloud_agent_pi_sessions"),
		migrateTable[model.CloudAgentGeminiCache]("cloud_agent_gemini_caches"),
		migrateTable[model.AgentProfile]("agent_profiles"),
		migrateTable[model.AgentLesson]("agent_lessons"),
		migrateTable[model.AgentMemorySetting]("agent_memory_settings"),
		migrateTable[model.AgentMemoryProposal]("agent_memory_proposals"),
		migrateTable[model.AgentMemoryRevision]("agent_memory_revisions"),
		migrateTable[model.TaskTextDelta]("task_text_delta"),
		migrateTable[model.TaskLog]("task_logs"),
		migrateTable[model.Result]("results"),
	}
}
