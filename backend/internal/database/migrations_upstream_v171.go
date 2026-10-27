package database

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"time"
)

func upstreamV171Migrations() []migration {
	return []migration{
		{version: 84, name: "drama_production", checksum: "sha256:drama-production-v61-20261024", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.DramaProduction{}, &model.DramaCast{}, &model.DramaEpisode{}, &model.DramaShot{}, &model.DramaTake{}, &model.DramaJob{})
		}},
		{version: 85, name: "drama_shot_segments", checksum: "sha256:drama-shot-segments-v62-20261024", apply: func(tx *gorm.DB) error {
			// 镜头级视频模型已随 v67 移到剧情段，这里只补仍存在的列。
			for _, field := range []string{"SegmentIndex", "SegmentTitle"} {
				if !tx.Migrator().HasColumn(&model.DramaShot{}, field) {
					if err := tx.Migrator().AddColumn(&model.DramaShot{}, field); err != nil {
						return err
					}
				}
			}
			return nil
		}},
		{version: 86, name: "drama_job_attempts", checksum: "sha256:drama-job-attempts-v63-20261024", apply: func(tx *gorm.DB) error {
			if tx.Migrator().HasColumn(&model.DramaJob{}, "AttemptsJSON") {
				return nil
			}
			return tx.Migrator().AddColumn(&model.DramaJob{}, "AttemptsJSON")
		}},
		{version: 87, name: "drama_media_release", checksum: "sha256:drama-media-release-v64-20261024", apply: func(tx *gorm.DB) error {
			if !tx.Migrator().HasColumn(&model.DramaProduction{}, "AssetsReleased") {
				if err := tx.Migrator().AddColumn(&model.DramaProduction{}, "AssetsReleased"); err != nil {
					return err
				}
			}
			if !tx.Migrator().HasColumn(&model.DramaEpisode{}, "MediaReleased") {
				return tx.Migrator().AddColumn(&model.DramaEpisode{}, "MediaReleased")
			}
			return nil
		}},
		{version: 88, name: "drama_shot_transition", checksum: "sha256:drama-shot-transition-v65-20261025", apply: func(tx *gorm.DB) error {
			if tx.Migrator().HasColumn(&model.DramaShot{}, "Transition") {
				return nil
			}
			return tx.Migrator().AddColumn(&model.DramaShot{}, "Transition")
		}},
		{version: 89, name: "drama_prompt_overrides", checksum: "sha256:drama-prompt-overrides-v66-20261025", apply: func(tx *gorm.DB) error {
			// 镜头级提示词改写已在 v67 移除，这里只建剧情段表。
			return tx.AutoMigrate(&model.DramaSegment{})
		}},
		{version: 90, name: "drama_segment_production", checksum: "sha256:drama-segment-production-v67-20261026", apply: migrateDramaSegmentProduction},
		{version: 91, name: "drama_segment_revision", checksum: "sha256:drama-segment-revision-v68-20261026", apply: migrateDramaSegmentRevision},
		{version: 92, name: "storage_products", checksum: "sha256:storage-products-v69-20261026", apply: func(tx *gorm.DB) error {
			// 已有商品和订单通过列默认值成为 credits 类型。
			return tx.AutoMigrate(&model.TopupProduct{}, &model.PaymentOrder{}, &model.UserStorageGrant{}, &model.UserStorageState{})
		}},
		{version: 93, name: "resource_offload_jobs", checksum: "sha256:resource-offload-jobs-v70-20261026", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.ResourceOffloadJob{})
		}},
		{version: 94, name: "drama_segment_mode", checksum: "sha256:drama-segment-mode-v71-20261026", apply: migrateDramaSegmentMode},
	}
}

func migrateDramaSegmentMode(tx *gorm.DB) error {
	var rows []model.DramaProduction
	if err := tx.Select("project_id", "settings_json").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		var settings map[string]any
		if err := json.Unmarshal([]byte(row.SettingsJSON), &settings); err != nil {
			continue
		}
		storyboard, ok := settings["storyboard"].(map[string]any)
		if !ok {
			continue
		}
		if _, done := storyboard["segmentMode"]; done {
			continue
		}
		storyboard["segmentMode"] = "auto"
		if seconds, _ := storyboard["segmentSeconds"].(float64); seconds > 0 {
			storyboard["segmentMode"] = "fixed"
		} else {
			delete(storyboard, "segmentSeconds")
		}
		encoded, err := json.Marshal(settings)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.DramaProduction{}).Where("project_id = ?", row.ProjectID).Update("settings_json", string(encoded)).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateDramaSegmentProduction(tx *gorm.DB) error {
	// Archive current legacy slots before removing their columns. Takes and resources are never deleted.
	legacy := false
	for _, slot := range []struct{ column, kind string }{{"audio_resource_id", "audio"}, {"frame_resource_id", "frame"}, {"video_resource_id", "video"}} {
		if !tx.Migrator().HasColumn(&model.DramaShot{}, slot.column) {
			continue
		}
		legacy = true
		var rows []struct {
			ID, ProjectID, ResourceID string
			DurationMs                int64
		}
		if err := tx.Model(&model.DramaShot{}).Select("id,project_id,duration_ms," + slot.column + " AS resource_id").Where(slot.column + " IS NOT NULL AND " + slot.column + " <> ''").Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			var count int64
			if err := tx.Model(&model.DramaTake{}).Where("owner_id = ? AND kind = ? AND resource_id = ?", row.ID, slot.kind, row.ResourceID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			var version int
			if err := tx.Model(&model.DramaTake{}).Where("owner_id = ? AND kind = ?", row.ID, slot.kind).Select("COALESCE(MAX(version),0)").Scan(&version).Error; err != nil {
				return err
			}
			take := model.DramaTake{ID: kernel.NewID(), ProjectID: row.ProjectID, OwnerID: row.ID, Kind: slot.kind, Version: version + 1, ResourceID: row.ResourceID, DurationMs: row.DurationMs, Prompt: "逐镜流程迁移保留的历史产物", CreatedAt: time.Now()}
			if err := tx.Create(&take).Error; err != nil {
				return err
			}
		}
	}
	if err := tx.AutoMigrate(&model.DramaSegment{}); err != nil {
		return err
	}
	if !tx.Migrator().HasColumn(&model.DramaShot{}, "PropIDsJSON") {
		if err := tx.Migrator().AddColumn(&model.DramaShot{}, "PropIDsJSON"); err != nil {
			return err
		}
	}
	if !tx.Migrator().HasColumn(&model.DramaEpisode{}, "Approved") {
		if err := tx.Migrator().AddColumn(&model.DramaEpisode{}, "Approved"); err != nil {
			return err
		}
	}
	if legacy {
		if err := tx.Model(&model.DramaProduction{}).Where("project_id IN (?)", tx.Model(&model.DramaEpisode{}).Select("project_id").Where("stage <> ?", "done")).Updates(map[string]any{"status": "paused", "pause_reason": "已切换剧情段制作，请核对剧本、视频模型并重新改编未完成的集；历史产物已保留", "revision": gorm.Expr("revision + 1")}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.DramaEpisode{}).Where("stage <> ?", "done").Updates(map[string]any{"stage": "script", "media_released": false, "approved": false, "error": "旧逐镜流程已停用，请确认设置后点击重新改编", "revision": gorm.Expr("revision + 1")}).Error; err != nil {
			return err
		}
	}
	if err := tx.Model(&model.DramaJob{}).Where("kind IN ? AND status IN ?", []string{"voice", "frame", "video"}, []string{"queued", "running"}).Updates(map[string]any{"status": "canceled", "error": "已切换剧情段制作，原任务保留在任务记录中", "finished_at": time.Now()}).Error; err != nil {
		return err
	}
	if legacy {
		if err := tx.Model(&model.DramaJob{}).Where("status IN ?", []string{"queued", "running"}).Updates(map[string]any{"status": "canceled", "error": "旧制作流程已暂停，禁止自动重发", "finished_at": time.Now()}).Error; err != nil {
			return err
		}
	}
	for _, column := range []string{"video_model_json", "speaker_cast_id", "frame_prompt_override", "video_prompt_override", "audio_resource_id", "audio_duration_ms", "audio_status", "frame_resource_id", "frame_status", "video_resource_id", "video_status", "approved", "locked", "error"} {
		if tx.Migrator().HasColumn(&model.DramaShot{}, column) {
			if err := tx.Migrator().DropColumn(&model.DramaShot{}, column); err != nil {
				return fmt.Errorf("删除旧镜头列 %s: %w", column, err)
			}
		}
	}
	if tx.Migrator().HasColumn(&model.DramaCast{}, "voice_key") {
		if err := tx.Migrator().DropColumn(&model.DramaCast{}, "voice_key"); err != nil {
			return err
		}
	}
	if tx.Migrator().HasColumn(&model.DramaEpisode{}, "frames_approved") {
		if err := tx.Migrator().DropColumn(&model.DramaEpisode{}, "frames_approved"); err != nil {
			return err
		}
	}
	return nil
}

func migrateDramaSegmentRevision(tx *gorm.DB) error {
	if err := migrateDramaSegmentProduction(tx); err != nil {
		return err
	}
	if err := tx.AutoMigrate(&model.DramaCast{}, &model.DramaSegment{}); err != nil {
		return err
	}
	// v67 installations may already have segment work in flight without revision snapshots.
	if err := tx.Model(&model.DramaProduction{}).Where("project_id IN (?)", tx.Model(&model.DramaJob{}).Select("project_id").Where("kind IN ? AND status IN ?", []string{"segment_prompt", "segment_video", "render", "script"}, []string{"queued", "running"})).Updates(map[string]any{"status": "paused", "pause_reason": "制作合同已升级，请核对任务记录后重试或重新改编"}).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.DramaSegment{}).Where("prompt_status = ?", "running").Updates(map[string]any{"prompt_status": "failed", "prompt_error": "任务合同升级，请核对原任务后重试"}).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.DramaSegment{}).Where("video_status IN ?", []string{"running", "queued"}).Updates(map[string]any{"video_status": "failed", "video_error": "任务合同升级，请核对原任务后重试"}).Error; err != nil {
		return err
	}
	return tx.Model(&model.DramaJob{}).Where("status IN ?", []string{"queued", "running"}).Updates(map[string]any{"status": "canceled", "error": "任务合同升级，原任务保留，需人工核对", "finished_at": time.Now()}).Error
}
