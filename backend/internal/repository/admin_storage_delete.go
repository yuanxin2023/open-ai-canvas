package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrAdminResourceDeleteChanged = errors.New("admin resource delete set changed")
var ErrAdminResourceProtected = errors.New("admin resource became protected")
var ErrAdminResourceAuditMismatch = errors.New("admin resource delete audit mismatch")

type AdminToolResourceCleanup struct {
	ID                    int64
	ExpectedCover         string
	ExpectedMediaURL      string
	ExpectedExtraInfoJSON string
	Cover                 string
	MediaURL              string
	ExtraInfoJSON         string
}

func (r *Repository) AdminResourcesByIDs(ids []string) ([]model.Resource, error) {
	if len(ids) == 0 {
		return []model.Resource{}, nil
	}
	var resources []model.Resource
	err := r.db.Where("id IN ?", ids).Find(&resources).Error
	return resources, err
}

func (r *Repository) AdminToolsByOwners(ownerIDs []string) ([]model.Tool, error) {
	if len(ownerIDs) == 0 {
		return []model.Tool{}, nil
	}
	var tools []model.Tool
	err := r.db.Where("owner_id IN ?", ownerIDs).Find(&tools).Error
	return tools, err
}

func (r *Repository) UserAvatarResourceReferences(resourceIDs []string) ([]ResourceDirectReference, error) {
	if len(resourceIDs) == 0 {
		return []ResourceDirectReference{}, nil
	}
	var users []model.User
	if err := r.db.Select("id", "username", "avatar_resource_id").Where("avatar_resource_id IN ?", resourceIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	result := make([]ResourceDirectReference, 0, len(users))
	for _, user := range users {
		result = append(result, ResourceDirectReference{Kind: "个人头像", ID: user.ID, Title: user.Username, ResourceID: user.AvatarResourceID})
	}
	return result, nil
}

func (r *Repository) InspirationResourceReferences(resourceIDs []string) ([]ResourceDirectReference, error) {
	if len(resourceIDs) == 0 {
		return []ResourceDirectReference{}, nil
	}
	var inspirations []model.Inspiration
	if err := r.db.Select("id", "title", "cover_resource_id").Where("cover_resource_id IN ?", resourceIDs).Find(&inspirations).Error; err != nil {
		return nil, err
	}
	result := make([]ResourceDirectReference, 0, len(inspirations))
	for _, inspiration := range inspirations {
		result = append(result, ResourceDirectReference{Kind: "首页灵感提示词", ID: inspiration.ID, Title: inspiration.Title, ResourceID: inspiration.CoverResourceID})
	}
	return result, nil
}

func (r *Repository) DeleteAdminResources(resources []model.Resource, toolCleanups []AdminToolResourceCleanup, deletionJobs []model.ResourceDeletionJob, audits []model.AdminAuditEvent, confirmInspirationCovers bool) error {
	if len(resources) == 0 {
		return nil
	}
	if len(audits) != len(resources) {
		return ErrAdminResourceAuditMismatch
	}
	resourceIDs := make([]string, 0, len(resources))
	for _, resource := range resources {
		resourceIDs = append(resourceIDs, resource.ID)
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var current []model.Resource
		query := tx.Where("id IN ?", resourceIDs)
		if r.Dialect() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Find(&current).Error; err != nil {
			return err
		}
		if len(current) != len(resources) {
			return ErrAdminResourceDeleteChanged
		}
		for _, check := range []struct {
			model any
			query string
			args  []any
		}{
			{model: &model.User{}, query: "avatar_resource_id IN ?", args: []any{resourceIDs}},
			{model: &model.CloudAgentResourceLease{}, query: "resource_id IN ? AND expires_at > ?", args: []any{resourceIDs, time.Now()}},
		} {
			var count int64
			if err := tx.Model(check.model).Where(check.query, check.args...).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrAdminResourceProtected
			}
		}
		if !confirmInspirationCovers {
			var count int64
			if err := tx.Model(&model.Inspiration{}).Where("cover_resource_id IN ?", resourceIDs).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrAdminResourceProtected
			}
		}
		for _, cleanup := range toolCleanups {
			updated := tx.Model(&model.Tool{}).
				Where("id = ? AND cover = ? AND media_url = ? AND extra_info_json = ?", cleanup.ID, cleanup.ExpectedCover, cleanup.ExpectedMediaURL, cleanup.ExpectedExtraInfoJSON).
				Updates(map[string]any{"cover": cleanup.Cover, "media_url": cleanup.MediaURL, "extra_info_json": cleanup.ExtraInfoJSON})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ErrAdminResourceDeleteChanged
			}
		}
		for _, value := range []any{&model.CanvasSnapshotResource{}, &model.ArkPrivateAssetBinding{}, &model.AnnouncementImageDraft{}, &model.InspirationCoverDraft{}, &model.AssetRepresentation{}, &model.ShotArtifact{}, &model.CloudAgentResourceLease{}} {
			if err := tx.Where("resource_id IN ?", resourceIDs).Delete(value).Error; err != nil {
				return err
			}
		}
		for _, cleanup := range []struct {
			model   any
			query   string
			updates map[string]any
		}{
			{model: &model.VoiceProfile{}, query: "sample_resource_id IN ?", updates: map[string]any{"sample_resource_id": ""}},
			{model: &model.Project{}, query: "cover_resource_id IN ?", updates: map[string]any{"cover_resource_id": ""}},
			{model: &model.Announcement{}, query: "image_resource_id IN ?", updates: map[string]any{"image_resource_id": ""}},
			{model: &model.Inspiration{}, query: "cover_resource_id IN ?", updates: map[string]any{"cover_resource_id": "", "cover_url": "", "cover_width": 0, "cover_height": 0}},
			{model: &model.UserPrompt{}, query: "cover_resource_id IN ?", updates: map[string]any{"cover_resource_id": "", "cover_url": ""}},
		} {
			if err := tx.Model(cleanup.model).Where(cleanup.query, resourceIDs).Updates(cleanup.updates).Error; err != nil {
				return err
			}
		}
		if len(deletionJobs) > 0 {
			if err := tx.Create(&deletionJobs).Error; err != nil {
				return err
			}
		}
		deleted := tx.Where("id IN ?", resourceIDs).Delete(&model.Resource{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != int64(len(resourceIDs)) {
			return ErrAdminResourceDeleteChanged
		}
		if len(audits) > 0 {
			if err := tx.Create(&audits).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
