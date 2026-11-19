package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"infinite-canvas/backend/internal/assets"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

type AdminResourceCleanupImpact struct {
	Assets          int `json:"assets"`
	Nodes           int `json:"nodes"`
	Canvases        int `json:"canvases"`
	UpdatedCanvases int `json:"updatedCanvases"`
}

func (impact *AdminResourceCleanupImpact) add(other AdminResourceCleanupImpact) {
	impact.Assets += other.Assets
	impact.Nodes += other.Nodes
	impact.Canvases += other.Canvases
	impact.UpdatedCanvases += other.UpdatedCanvases
}

func (s *Service) cleanupAdminResourceDocuments(repo *repository.Repository, resources []model.Resource, apply bool) (AdminResourceCleanupImpact, error) {
	impact := AdminResourceCleanupImpact{}
	byUser := map[string]map[string]struct{}{}
	for _, resource := range resources {
		if byUser[resource.UserID] == nil {
			byUser[resource.UserID] = map[string]struct{}{}
		}
		byUser[resource.UserID][resource.ID] = struct{}{}
	}
	users := make([]string, 0, len(byUser))
	for id := range byUser {
		users = append(users, id)
	}
	sort.Strings(users)
	for _, userID := range users {
		deleted := byUser[userID]
		assetRows, canvases, err := repo.AdminCleanupDocuments(userID, apply)
		if err != nil {
			return impact, err
		}
		entries := []model.UserDataCleanupEntry{}
		for _, id := range assets.SortedIDs(deleted) {
			if err := repo.RequireNotAdminDeleted(userID, "resource", id); err == nil {
				entries = append(entries, model.UserDataCleanupEntry{Kind: "resource", EntityID: id, Action: "delete"})
			} else if !errors.Is(err, repository.ErrAdminDeletedEntity) {
				return impact, err
			}
		}
		for _, asset := range assetRows {
			versionChanged, representationPrimaryDeleted, err := s.cleanupAdminAssetVersions(repo, asset, deleted, apply)
			if err != nil {
				return impact, err
			}
			affected, err := adminCleanupDocumentAffected(asset.PayloadJSON, deleted)
			if err != nil {
				return impact, fmt.Errorf("素材 %s: %w", asset.ID, err)
			}
			if !affected && !versionChanged {
				continue
			}
			cleaned, err := assets.CleanMediaDocument(asset.PayloadJSON, deleted, nil)
			if err != nil {
				return impact, fmt.Errorf("素材 %s 无法解析，已停止清理: %w", asset.ID, err)
			}
			if !cleaned.Changed && !versionChanged {
				continue
			}
			var document map[string]any
			if err := json.Unmarshal([]byte(asset.PayloadJSON), &document); err != nil {
				return impact, err
			}
			data, _ := document["data"].(map[string]any)
			if data == nil {
				data = document
			}
			primaryDeleted := representationPrimaryDeleted
			for _, key := range []string{"storageKey", "dataUrl", "url", "resourceId"} {
				value, _ := data[key].(string)
				id := assets.ResourceID(value)
				if key == "resourceId" {
					id = assets.ValidID(value)
				}
				if _, gone := deleted[id]; gone && id != "" {
					primaryDeleted = true
				}
			}
			media := asset.Kind == "image" || asset.Kind == "video" || asset.Kind == "audio" || asset.Kind == "model"
			if media && primaryDeleted {
				impact.Assets++
				entries = append(entries, model.UserDataCleanupEntry{Kind: "asset", EntityID: asset.ID, Action: "delete"})
				if apply {
					if err := repo.DeleteAssetsAndResources(userID, []string{asset.ID}, nil, nil, true); err != nil {
						return impact, err
					}
				}
			} else {
				entries = append(entries, model.UserDataCleanupEntry{Kind: "asset", EntityID: asset.ID, Action: "update"})
				if apply {
					asset.PayloadJSON = cleaned.Payload
					if err := repo.UpdateAdminCleanedAsset(asset); err != nil {
						return impact, err
					}
				}
			}
		}
		for _, canvas := range canvases {
			affected, err := adminCleanupDocumentAffected(canvas.PayloadJSON, deleted)
			if err != nil {
				return impact, fmt.Errorf("画布 %s: %w", canvas.ID, err)
			}
			if !affected {
				continue
			}
			cleaned, err := assets.CleanMediaDocument(canvas.PayloadJSON, deleted, nil)
			if err != nil {
				return impact, fmt.Errorf("画布 %s 无法解析，已停止清理: %w", canvas.ID, err)
			}
			if !cleaned.Changed {
				continue
			}
			deleteCanvas := cleaned.HadMedia && !cleaned.HasMedia
			if apply {
				active, err := repo.CanvasHasActiveWork(userID, canvas.ID)
				if err != nil {
					return impact, err
				}
				if active {
					return impact, BadAuthRequest("受影响画布存在活动任务或 Agent，请稍后重试")
				}
			}
			impact.Nodes += len(cleaned.RemovedNodeIDs)
			if deleteCanvas {
				impact.Canvases++
			} else {
				impact.UpdatedCanvases++
			}
			action := "update"
			if deleteCanvas {
				action = "delete"
			}
			entries = append(entries, model.UserDataCleanupEntry{Kind: "canvas", EntityID: canvas.ID, Action: action})
			for _, id := range cleaned.RemovedNodeIDs {
				entries = append(entries, model.UserDataCleanupEntry{Kind: "node", EntityID: canvas.ID + ":" + id, CanvasID: canvas.ID, Action: "delete"})
			}
			if apply {
				if !deleteCanvas {
					payload, created, err := s.rebindAdminCleanedCanvas(repo, userID, cleaned.Payload)
					if err != nil {
						return impact, err
					}
					cleaned.Payload = payload
					for _, id := range created {
						entries = append(entries, model.UserDataCleanupEntry{Kind: "asset", EntityID: id, Action: "update"})
					}
				}
				if err := repo.ApplyAdminCanvasCleanup(canvas, cleaned.Payload, cleaned.ChangedNodeIDs, deleteCanvas); err != nil {
					return impact, err
				}
			}
		}
		if apply {
			if err := repo.RecordUserDataCleanup(userID, entries); err != nil {
				return impact, err
			}
		}
	}
	return impact, nil
}

func adminCleanupDocumentAffected(raw string, deleted map[string]struct{}) (bool, error) {
	if json.Valid([]byte(raw)) {
		return assets.DocumentReferences(raw, deleted), nil
	}
	for id := range deleted {
		if strings.Contains(raw, "resource:"+id) || strings.Contains(raw, "/resources/"+id+"/") || strings.Contains(raw, `"`+id+`"`) {
			return false, fmt.Errorf("正文损坏，无法安全判断资源影响")
		}
	}
	return false, nil
}

func (s *Service) cleanupAdminAssetVersions(repo *repository.Repository, asset model.Asset, deleted map[string]struct{}, apply bool) (bool, bool, error) {
	versions, reps, err := repo.AssetResourceRecords(asset.ID)
	if err != nil {
		return false, false, err
	}
	changed, primary := false, false
	for _, version := range versions {
		affected, err := adminCleanupDocumentAffected(version.DefinitionJSON, deleted)
		if err != nil {
			return false, false, err
		}
		if !affected {
			continue
		}
		cleaned, err := assets.CleanMediaDocument(version.DefinitionJSON, deleted, nil)
		if err != nil {
			return false, false, err
		}
		changed = changed || cleaned.Changed
		if apply && cleaned.Changed {
			version.DefinitionJSON = cleaned.Payload
			if err := repo.UpdateAdminCleanedAssetVersion(version); err != nil {
				return false, false, err
			}
		}
	}
	for _, rep := range reps {
		if _, gone := deleted[rep.ResourceID]; gone {
			changed = true
			if rep.Role == "primary" {
				primary = true
			}
		}
		affected, err := adminCleanupDocumentAffected(rep.MetadataJSON, deleted)
		if err != nil {
			return false, false, err
		}
		if !affected {
			continue
		}
		cleaned, err := assets.CleanMediaDocument(rep.MetadataJSON, deleted, nil)
		if err != nil {
			return false, false, err
		}
		changed = changed || cleaned.Changed
		if apply && cleaned.Changed {
			rep.MetadataJSON = cleaned.Payload
			if err := repo.UpdateAdminCleanedRepresentation(rep); err != nil {
				return false, false, err
			}
		}
	}
	return changed, primary, nil
}

func (s *Service) UserDataCleanupChanges(userID string, after int64) (repository.UserDataCleanupPage, error) {
	if userID == "" || after < 0 || after > 9007199254740991 {
		return repository.UserDataCleanupPage{}, BadAuthRequest("清理游标无效")
	}
	return s.repo.UserDataCleanupChanges(userID, after)
}
