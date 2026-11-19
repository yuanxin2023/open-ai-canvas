package repository

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas/backend/internal/assets"
	"infinite-canvas/backend/internal/model"
)

var ErrAdminDeletedEntity = errors.New("内容已被管理员永久删除")

func (r *Repository) LockUserDataCleanup(userID string) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).UpdateColumn("updated_at", gorm.Expr("updated_at")).Error
}

type UserDataCleanupPage struct {
	Entries []model.UserDataCleanupEntry `json:"entries"`
	Cursor  int64                        `json:"cursor"`
	HasMore bool                         `json:"hasMore"`
}

func (r *Repository) UserDataCleanupChanges(userID string, after int64) (UserDataCleanupPage, error) {
	result := UserDataCleanupPage{Entries: []model.UserDataCleanupEntry{}, Cursor: after}
	var revisions []int64
	if err := r.db.Model(&model.UserDataCleanupEntry{}).Where("user_id = ? AND revision > ?", userID, after).Distinct("revision").Order("revision").Limit(101).Pluck("revision", &revisions).Error; err != nil {
		return result, err
	}
	if len(revisions) == 0 {
		return result, nil
	}
	result.HasMore = len(revisions) > 100
	if result.HasMore {
		revisions = revisions[:100]
	}
	result.Cursor = revisions[len(revisions)-1]
	err := r.db.Where("user_id = ? AND revision > ? AND revision <= ?", userID, after, result.Cursor).Order("revision, kind, entity_id").Find(&result.Entries).Error
	return result, err
}

func (r *Repository) RequireNotAdminDeleted(userID, kind, id string) error {
	var count int64
	if err := r.db.Model(&model.UserDataCleanupEntry{}).Where("user_id = ? AND kind = ? AND entity_id = ? AND action = 'delete'", userID, kind, id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrAdminDeletedEntity
	}
	return nil
}

func (r *Repository) RequireNoAdminDeletedResources(userID, raw string) error {
	deleted, err := r.AdminDeletedResourceIDs(userID)
	if err != nil {
		return err
	}
	if assets.DocumentReferences(raw, deleted) {
		return ErrAdminDeletedEntity
	}
	return nil
}

func (r *Repository) RecordUserDataCleanup(userID string, entries []model.UserDataCleanupEntry) error {
	if len(entries) == 0 {
		return nil
	}
	state := model.UserDataCleanupState{UserID: userID}
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; err != nil {
		return err
	}
	// Updating the counter takes a row lock through the outer transaction commit.
	if err := r.db.Model(&state).UpdateColumn("revision", gorm.Expr("revision + 1")).Error; err != nil {
		return err
	}
	if err := r.db.First(&state, "user_id = ?", userID).Error; err != nil {
		return err
	}
	for _, entry := range entries {
		entry.UserID, entry.Revision, entry.CreatedAt = userID, state.Revision, time.Now()
		if err := r.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "kind"}, {Name: "entity_id"}}, DoUpdates: clause.AssignmentColumns([]string{"revision", "action", "canvas_id", "created_at"})}).Create(&entry).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) AdminCleanupDocuments(userID string, lock bool) ([]model.Asset, []model.CanvasProject, error) {
	var assetRows []model.Asset
	var canvases []model.CanvasProject
	if lock {
		// Take the SQLite writer lock before reads and serialize new entity writes.
		if err := r.LockUserDataCleanup(userID); err != nil {
			return nil, nil, err
		}
	}
	query := func() *gorm.DB {
		q := r.db.Where("user_id = ?", userID).Order("id")
		if lock && r.Dialect() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		return q
	}
	if err := query().Find(&assetRows).Error; err != nil {
		return nil, nil, err
	}
	if err := query().Find(&canvases).Error; err != nil {
		return nil, nil, err
	}
	return assetRows, canvases, nil
}

func (r *Repository) UpdateAdminCleanedAsset(asset model.Asset) error {
	return r.db.Model(&model.Asset{}).Where("user_id = ? AND id = ?", asset.UserID, asset.ID).Updates(map[string]any{"payload_json": asset.PayloadJSON, "updated_at": time.Now()}).Error
}

func (r *Repository) UpdateAdminCleanedAssetVersion(version model.AssetVersion) error {
	return r.db.Model(&model.AssetVersion{}).Where("id = ? AND asset_id = ?", version.ID, version.AssetID).Update("definition_json", version.DefinitionJSON).Error
}

func (r *Repository) UpdateAdminCleanedRepresentation(rep model.AssetRepresentation) error {
	return r.db.Model(&model.AssetRepresentation{}).Where("id = ? AND asset_version_id = ?", rep.ID, rep.AssetVersionID).Update("metadata_json", rep.MetadataJSON).Error
}

func (r *Repository) ApplyAdminCanvasCleanup(before model.CanvasProject, payload string, removed []string, deleteCanvas bool) error {
	if deleteCanvas {
		if err := r.db.Where("user_id = ? AND canvas_id = ?", before.UserID, before.ID).Delete(&model.CanvasEditLease{}).Error; err != nil {
			return err
		}
		return r.DeleteCanvasProject(before.UserID, before.ID)
	}
	if len(removed) > 0 {
		if err := r.db.Where("user_id = ? AND canvas_id = ? AND node_id IN ?", before.UserID, before.ID, removed).Delete(&model.CanvasEditLease{}).Error; err != nil {
			return err
		}
	}
	updated := r.db.Model(&model.CanvasProject{}).Where("user_id = ? AND id = ? AND revision = ?", before.UserID, before.ID, before.Revision).Updates(map[string]any{"payload_json": payload, "revision": before.Revision + 1, "updated_at": time.Now()})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrCanvasRevisionConflict
	}
	r.canvasChanged(before.UserID, before.ID)
	return nil
}

func (r *Repository) CanvasHasActiveWork(userID, canvasID string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.Task{}).Where("user_id = ? AND project_id = ? AND status IN ?", userID, canvasID, []model.TaskStatus{model.TaskStatusQueued, model.TaskStatusRunning, model.TaskStatusTextReplay}).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	if err := r.db.Model(&model.CloudAgentExecution{}).Where("user_id = ? AND canvas_id = ? AND status IN ?", userID, canvasID, []string{"queued", "running", "waiting_answer", "waiting_proposal", "waiting_canvas", "waiting_payment", "waiting_task"}).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) AdminDeletedResourceIDs(userID string) (map[string]struct{}, error) {
	var ids []string
	err := r.db.Model(&model.UserDataCleanupEntry{}).Where("user_id = ? AND kind = 'resource' AND action = 'delete'", userID).Pluck("entity_id", &ids).Error
	result := map[string]struct{}{}
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result, err
}

// SanitizeAdminCanvasPayload is also used by history restore and create operations.
func (r *Repository) SanitizeAdminCanvasPayload(userID, canvasID, raw string) (string, error) {
	if err := r.RequireNotAdminDeleted(userID, "canvas", canvasID); err != nil {
		return "", err
	}
	deleted, err := r.AdminDeletedResourceIDs(userID)
	if err != nil {
		return "", err
	}
	var nodes []string
	if err := r.db.Model(&model.UserDataCleanupEntry{}).Where("user_id = ? AND kind = 'node' AND canvas_id = ? AND action = 'delete'", userID, canvasID).Pluck("entity_id", &nodes).Error; err != nil {
		return "", err
	}
	forced := map[string]struct{}{}
	for _, id := range nodes {
		forced[strings.TrimPrefix(id, canvasID+":")] = struct{}{}
	}
	if len(deleted) == 0 && len(forced) == 0 {
		return raw, nil
	}
	cleaned, err := assets.CleanMediaDocument(raw, deleted, forced)
	return cleaned.Payload, err
}
