package repository

import (
	"errors"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrUserPurgeChanged = errors.New("user purge data changed")
var ErrUserPurgePendingReferral = errors.New("user has pending referral reward")

type UserPurgeSnapshot struct {
	Resources []model.Resource
	TaskIDs   []string
}

func (r *Repository) UserPurgeSnapshot(userID string) (*UserPurgeSnapshot, error) {
	var resources []model.Resource
	if err := r.db.Where("user_id = ?", userID).Find(&resources).Error; err != nil {
		return nil, err
	}
	var taskIDs []string
	if err := r.db.Model(&model.Task{}).Where("user_id = ?", userID).Pluck("id", &taskIDs).Error; err != nil {
		return nil, err
	}
	return &UserPurgeSnapshot{Resources: resources, TaskIDs: taskIDs}, nil
}

// PurgeUserData removes one user's account and all user-scoped business data in
// a single transaction. ResourceDeletionJob rows intentionally survive until
// the physical objects have been removed by the outbox worker.
func (r *Repository) PurgeUserData(userID string, expectedResourceIDs []string, deletionJobs []model.ResourceDeletionJob, audit model.AdminAuditEvent, protectLastFull bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		query := tx.Where("id = ?", userID)
		if r.Dialect() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&user).Error; err != nil {
			return err
		}
		var pendingReferralCount int64
		if err := tx.Model(&model.ReferralReward{}).Where("status = ? AND (inviter_id = ? OR invitee_id = ?)", model.ReferralRewardPending, userID, userID).Count(&pendingReferralCount).Error; err != nil {
			return err
		}
		if pendingReferralCount > 0 {
			return ErrUserPurgePendingReferral
		}
		if protectLastFull {
			var fullAdminIDs []string
			fullQuery := tx.Model(&model.User{}).
				Where("role = ? AND status = ? AND (admin_level = ? OR admin_level = '')", model.UserRoleAdmin, model.UserStatusActive, model.AdminLevelFull).
				Order("id")
			if r.Dialect() == "postgres" {
				fullQuery = fullQuery.Clauses(clause.Locking{Strength: "UPDATE"})
			}
			if err := fullQuery.Pluck("id", &fullAdminIDs).Error; err != nil {
				return err
			}
			remaining := 0
			for _, id := range fullAdminIDs {
				if id != userID {
					remaining++
				}
			}
			if remaining == 0 {
				return ErrLastActiveFullAdmin
			}
		}

		var currentResourceIDs []string
		if err := tx.Model(&model.Resource{}).Where("user_id = ?", userID).Pluck("id", &currentResourceIDs).Error; err != nil {
			return err
		}
		if !sameStringSet(currentResourceIDs, expectedResourceIDs) {
			return ErrUserPurgeChanged
		}

		var channelIDs []string
		if err := tx.Unscoped().Model(&model.ModelChannel{}).Where("user_id = ? AND scope = ?", userID, model.ChannelScopeUser).Pluck("id", &channelIDs).Error; err != nil {
			return err
		}
		var channelModelIDs []string
		if len(channelIDs) > 0 {
			if err := tx.Unscoped().Model(&model.ChannelModel{}).Where("channel_id IN ?", channelIDs).Pluck("id", &channelModelIDs).Error; err != nil {
				return err
			}
		}
		skillIDs, err := pluckStringIDs(tx, &model.Skill{}, "owner_id = ?", userID)
		if err != nil {
			return err
		}
		skillVersionIDs, err := pluckStringIDs(tx, &model.SkillVersion{}, "skill_id IN ?", skillIDs)
		if err != nil {
			return err
		}
		var toolIDs []int64
		if err := tx.Model(&model.Tool{}).Where("owner_id = ?", userID).Pluck("id", &toolIDs).Error; err != nil {
			return err
		}
		assetIDs, err := pluckStringIDs(tx, &model.Asset{}, "user_id = ?", userID)
		if err != nil {
			return err
		}
		assetVersionIDs, err := pluckStringIDs(tx, &model.AssetVersion{}, "asset_id IN ?", assetIDs)
		if err != nil {
			return err
		}
		voiceIDs, err := pluckStringIDs(tx, &model.VoiceProfile{}, "user_id = ?", userID)
		if err != nil {
			return err
		}
		projectIDs, err := pluckStringIDs(tx, &model.Project{}, "user_id = ?", userID)
		if err != nil {
			return err
		}
		shotIDs, err := pluckStringIDs(tx, &model.Shot{}, "project_id IN ?", projectIDs)
		if err != nil {
			return err
		}
		workflowIDs, err := pluckStringIDs(tx, &model.WorkflowInstance{}, "project_id IN ?", projectIDs)
		if err != nil {
			return err
		}
		workflowStepIDs, err := pluckStringIDs(tx, &model.WorkflowStepInstance{}, "workflow_instance_id IN ?", workflowIDs)
		if err != nil {
			return err
		}
		canvasIDs, err := pluckStringIDs(tx, &model.CanvasProject{}, "user_id = ?", userID)
		if err != nil {
			return err
		}
		snapshotIDs, err := pluckStringIDs(tx, &model.CanvasSnapshot{}, "user_id = ?", userID)
		if err != nil {
			return err
		}
		taskIDs, err := pluckStringIDs(tx, &model.Task{}, "user_id = ?", userID)
		if err != nil {
			return err
		}
		paymentOrderIDs, err := pluckStringIDs(tx, &model.PaymentOrder{}, "user_id = ?", userID)
		if err != nil {
			return err
		}

		// Remove or anonymize references held by records that belong to other users.
		if err := tx.Model(&model.CreditLedgerEntry{}).Where("actor_user_id = ? AND user_id <> ?", userID, userID).Update("actor_user_id", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BillingOrder{}).Where("resolved_by = ? AND user_id <> ?", userID, userID).Update("resolved_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.RedeemBatch{}).Where("created_by = ?", userID).Update("created_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.RedeemCode{}).Where("redeemed_by = ?", userID).Update("redeemed_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.PromptTemplate{}).Where("created_by = ?", userID).Update("created_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Announcement{}).Where("created_by = ?", userID).Update("created_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Inspiration{}).Where("created_by = ?", userID).Update("created_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Inspiration{}).Where("updated_by = ?", userID).Update("updated_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ShotRevision{}).Where("created_by = ?", userID).Update("created_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Task{}).Where("cancellation_actor_id = ? AND user_id <> ?", userID, userID).Update("cancellation_actor_id", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.LogicalModelRevision{}).Where("created_by = ?", userID).Update("created_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.SystemSetting{}).Where("updated_by = ?", userID).Update("updated_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.PluginPlatformState{}).Where("updated_by = ?", userID).Update("updated_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.TopupProduct{}).Where("created_by = ?", userID).Update("created_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.TopupProduct{}).Where("updated_by = ?", userID).Update("updated_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.PaymentProviderConfig{}).Where("created_by = ?", userID).Update("created_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.PaymentReconciliationRun{}).Where("started_by = ?", userID).Update("started_by", "").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BannerAnnouncement{}).Where("created_by = ?", userID).Update("created_by", "").Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Model(&model.ModelChannel{}).Where("user_id = ? AND scope = ?", userID, model.ChannelScopeSystem).Update("user_id", "").Error; err != nil {
			return err
		}

		if err := deleteWhereIn(tx, &model.ChannelModelPriceTier{}, "channel_model_id", channelModelIDs, true); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.LogicalModelRoute{}, "channel_model_id", channelModelIDs, false); err != nil {
			return err
		}
		if len(channelModelIDs) > 0 {
			if err := tx.Model(&model.LogicalModel{}).Where("source_channel_model_id IN ?", channelModelIDs).Update("source_channel_model_id", "").Error; err != nil {
				return err
			}
		}
		if err := deleteWhereIn(tx, &model.ModelPricing{}, "channel_id", channelIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ChannelModel{}, "channel_id", channelIDs, true); err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ? AND scope = ?", userID, model.ChannelScopeUser).Delete(&model.ModelChannel{}).Error; err != nil {
			return err
		}

		if err := deleteWhereIn(tx, &model.SkillFile{}, "skill_version_id", skillVersionIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.SkillVersion{}, "skill_id", skillIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.UserSkillState{}, "skill_id", skillIDs, false); err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserSkillState{}).Error; err != nil {
			return err
		}
		if err := tx.Where("owner_id = ?", userID).Delete(&model.Skill{}).Error; err != nil {
			return err
		}
		if len(toolIDs) > 0 {
			if err := tx.Where("tool_id IN ?", toolIDs).Delete(&model.ToolFavorite{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.ToolFavorite{}).Error; err != nil {
			return err
		}
		if err := tx.Where("owner_id = ?", userID).Delete(&model.Tool{}).Error; err != nil {
			return err
		}

		if err := deleteWhereIn(tx, &model.CharacterVoiceBinding{}, "asset_version_id", assetVersionIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.CharacterVoiceBinding{}, "voice_profile_id", voiceIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ShotAssetReference{}, "asset_version_id", assetVersionIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.AssetRepresentation{}, "asset_version_id", assetVersionIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ProjectAssetLink{}, "asset_id", assetIDs, false); err != nil {
			return err
		}
		if len(assetIDs) > 0 {
			if err := tx.Model(&model.ProjectAssetCandidate{}).Where("resolved_asset_id IN ?", assetIDs).Update("resolved_asset_id", "").Error; err != nil {
				return err
			}
		}
		if err := deleteWhereIn(tx, &model.AssetVersion{}, "asset_id", assetIDs, false); err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.Asset{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.AssetFolder{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.VoiceProfile{}).Error; err != nil {
			return err
		}

		if err := deleteWhereIn(tx, &model.WorkflowStepTask{}, "workflow_step_id", workflowStepIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.WorkflowStepInstance{}, "workflow_instance_id", workflowIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.WorkflowInstance{}, "project_id", projectIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ShotAssetReference{}, "shot_id", shotIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ShotArtifact{}, "project_id", projectIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ShotRevision{}, "shot_id", shotIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.Shot{}, "project_id", projectIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.CanvasUnitLink{}, "project_id", projectIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ProjectAssetCandidate{}, "project_id", projectIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ProjectAssetLink{}, "project_id", projectIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ProjectAssetFolder{}, "project_id", projectIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ProjectUnit{}, "project_id", projectIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ProductionTaskLink{}, "project_id", projectIDs, false); err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.Project{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.StyleProfile{}).Error; err != nil {
			return err
		}

		if err := deleteWhereIn(tx, &model.CanvasSnapshotResource{}, "snapshot_id", snapshotIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.CanvasUnitLink{}, "canvas_id", canvasIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ProductionTaskLink{}, "canvas_id", canvasIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.CanvasShare{}, "project_id", canvasIDs, false); err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.CanvasShare{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.CanvasSnapshot{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.CanvasProject{}).Error; err != nil {
			return err
		}

		if err := deleteWhereIn(tx, &model.RouteAttempt{}, "task_id", taskIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.WorkflowStepTask{}, "task_id", taskIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.ProductionTaskLink{}, "task_id", taskIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.AssetRepresentation{}, "task_id", taskIDs, false); err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.TaskTextDelta{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.TaskLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.Result{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.ApiCallLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.Task{}).Error; err != nil {
			return err
		}

		if err := deleteWhereIn(tx, &model.PaymentNotification{}, "payment_order_id", paymentOrderIDs, false); err != nil {
			return err
		}
		if err := deleteWhereIn(tx, &model.PaymentReconciliationItem{}, "payment_order_id", paymentOrderIDs, false); err != nil {
			return err
		}
		if err := tx.Model(&model.ReferralProfile{}).Where("inviter_id = ?", userID).Update("inviter_id", "").Error; err != nil {
			return err
		}
		if err := tx.Where("inviter_id = ? OR invitee_id = ?", userID, userID).Delete(&model.ReferralReward{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.ReferralProfile{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.PaymentOrder{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.CreditLedgerEntry{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.BillingOrder{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.CreditAccount{}).Error; err != nil {
			return err
		}

		for _, owned := range []struct {
			model any
			query string
		}{
			{&model.CloudAgentResourceLease{}, "user_id = ?"}, {&model.CloudAgentCanvasMutation{}, "user_id = ?"},
			{&model.CloudAgentEventRecord{}, "user_id = ?"}, {&model.CloudAgentMessageRecord{}, "user_id = ?"},
			{&model.CloudAgentExecution{}, "user_id = ?"}, {&model.AgentProfile{}, "user_id = ?"},
			{&model.AgentLesson{}, "author_user_id = ?"}, {&model.AgentMemorySetting{}, "user_id = ?"},
			{&model.CreationSubmission{}, "user_id = ?"}, {&model.CreationRun{}, "user_id = ?"},
			{&model.UserPromptCustomization{}, "user_id = ?"}, {&model.UserPrompt{}, "user_id = ?"},
			{&model.UserAnnouncementRead{}, "user_id = ?"}, {&model.UserPluginState{}, "user_id = ?"},
			{&model.UserDailyActivity{}, "user_id = ?"}, {&model.UserDailyUploadUsage{}, "user_id = ?"},
			{&model.ArkPrivateAssetBinding{}, "user_id = ?"}, {&model.UserOSSSetting{}, "user_id = ?"},
			{&model.AnnouncementImageDraft{}, "user_id = ?"},
			{&model.InspirationCoverDraft{}, "user_id = ?"}, {&model.PaymentPromotionImageDraft{}, "user_id = ?"}, {&model.UserIdentity{}, "user_id = ?"},
			{&model.UserLoginEvent{}, "user_id = ?"},
			{&model.UserUsernameChange{}, "user_id = ?"},
			{&model.AuthSession{}, "user_id = ?"},
			{&model.AdminPermissionGrant{}, "user_id = ?"},
		} {
			if err := tx.Where(owned.query, userID).Delete(owned.model).Error; err != nil {
				return err
			}
		}

		if len(expectedResourceIDs) > 0 {
			if err := tx.Where("resource_id IN ?", expectedResourceIDs).Delete(&model.CanvasSnapshotResource{}).Error; err != nil {
				return err
			}
			if err := tx.Where("resource_id IN ?", expectedResourceIDs).Delete(&model.ArkPrivateAssetBinding{}).Error; err != nil {
				return err
			}
			if err := tx.Where("resource_id IN ?", expectedResourceIDs).Delete(&model.AssetRepresentation{}).Error; err != nil {
				return err
			}
			if err := tx.Where("resource_id IN ?", expectedResourceIDs).Delete(&model.ShotArtifact{}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.VoiceProfile{}).Where("sample_resource_id IN ?", expectedResourceIDs).Update("sample_resource_id", "").Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Project{}).Where("cover_resource_id IN ?", expectedResourceIDs).Update("cover_resource_id", "").Error; err != nil {
				return err
			}
			if err := tx.Model(&model.UserPrompt{}).Where("cover_resource_id IN ?", expectedResourceIDs).Updates(map[string]any{"cover_resource_id": "", "cover_url": ""}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Announcement{}).Where("image_resource_id IN ?", expectedResourceIDs).Update("image_resource_id", "").Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Inspiration{}).Where("cover_resource_id IN ?", expectedResourceIDs).Updates(map[string]any{"cover_resource_id": "", "cover_url": "", "cover_width": 0, "cover_height": 0}).Error; err != nil {
				return err
			}
		}
		if len(deletionJobs) > 0 {
			if err := tx.Create(&deletionJobs).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.Resource{}).Error; err != nil {
			return err
		}
		if err := tx.Where("owner_id = ? AND NOT EXISTS (SELECT 1 FROM resources WHERE resources.storage_setting_id = storage_locations.id) AND NOT EXISTS (SELECT 1 FROM resource_deletion_jobs WHERE resource_deletion_jobs.storage_setting_id = storage_locations.id)", userID).Delete(&model.StorageLocation{}).Error; err != nil {
			return err
		}

		if err := tx.Where("actor_user_id = ? OR (target_type = ? AND target_id = ?)", userID, "user", userID).Delete(&model.AdminAuditEvent{}).Error; err != nil {
			return err
		}
		if user.Email != "" {
			if err := tx.Where("lower(email) = lower(?)", user.Email).Delete(&model.EmailVerificationCode{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Delete(&model.User{}, "id = ?", userID).Error; err != nil {
			return err
		}
		return tx.Create(&audit).Error
	})
}

func pluckStringIDs(tx *gorm.DB, modelValue any, query string, args ...any) ([]string, error) {
	if len(args) == 1 {
		if ids, ok := args[0].([]string); ok && len(ids) == 0 {
			return []string{}, nil
		}
	}
	var ids []string
	err := tx.Model(modelValue).Where(query, args...).Pluck("id", &ids).Error
	return ids, err
}

func deleteWhereIn(tx *gorm.DB, modelValue any, column string, ids []string, unscoped bool) error {
	if len(ids) == 0 {
		return nil
	}
	query := tx.Where(column+" IN ?", ids)
	if unscoped {
		query = query.Unscoped()
	}
	return query.Delete(modelValue).Error
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]int, len(left))
	for _, value := range left {
		seen[value]++
	}
	for _, value := range right {
		seen[value]--
		if seen[value] < 0 {
			return false
		}
	}
	return true
}
