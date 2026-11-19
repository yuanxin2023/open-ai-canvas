package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"time"

	"infinite-canvas/backend/internal/assets"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

// A retained version can still point at the Asset whose primary was removed.
// Rebind it to another owning Asset, or materialize the surviving immutable Resource.
func (s *Service) rebindAdminCleanedCanvas(repo *repository.Repository, userID, raw string) (string, []string, error) {
	owned, err := repo.Assets(userID)
	if err != nil {
		return "", nil, err
	}
	var document any
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		return "", nil, err
	}
	bindings := map[string]string{}
	assetResources := map[string]map[string]struct{}{}
	for _, asset := range owned {
		assetResources[asset.ID] = map[string]struct{}{}
		refs, err := assets.CollectDocumentResourceReferences(asset.PayloadJSON)
		if err != nil {
			continue
		}
		for _, ref := range refs {
			assetResources[asset.ID][ref.ResourceID] = struct{}{}
			if bindings[ref.ResourceID] == "" {
				bindings[ref.ResourceID] = asset.ID
			}
		}
	}
	created := []string{}
	var walk func(any, string) error
	walk = func(value any, key string) error {
		switch item := value.(type) {
		case map[string]any:
			if key == "metadata" || key == "media" || key == "directMedia" {
				id := ""
				for _, field := range []string{"storageKey", "content", "url", "dataUrl"} {
					text, _ := item[field].(string)
					if candidate := assets.ResourceID(text); candidate != "" {
						id = candidate
						break
					}
				}
				if id != "" {
					binding := bindings[id]
					currentID, _ := item["assetId"].(string)
					if _, valid := assetResources[currentID][id]; valid {
						binding = currentID
					}
					if binding == "" {
						rows, err := repo.ResourcesForUserIDs(userID, []string{id})
						if err != nil {
							return err
						}
						if len(rows) == 1 && rows[0].Status == model.ResourceStatusReady {
							resource := rows[0]
							digest := sha256.Sum256([]byte(userID + "\x00" + id))
							binding = "cleanup_" + hex.EncodeToString(digest[:16])
							if err := repo.RequireNotAdminDeleted(userID, "asset", binding); err != nil {
								return err
							}
							kind := resource.Kind
							if kind != "image" && kind != "video" && kind != "audio" {
								kind = "model"
							}
							url := "/api/resources/" + id + "/file"
							data := map[string]any{"storageKey": "resource:" + id, "bytes": resource.Size, "mimeType": resource.MimeType}
							if kind == "model" {
								data["fileName"] = filepath.Base(resource.ObjectKey)
							}
							if kind == "image" {
								data["dataUrl"] = url
								data["width"] = max(1, resource.Width)
								data["height"] = max(1, resource.Height)
							} else {
								data["url"] = url
								data["durationMs"] = resource.DurationMs
								data["width"] = resource.Width
								data["height"] = resource.Height
							}
							now := time.Now().UTC()
							category := model.NormalizeAssetCategory("", kind)
							payload, err := json.Marshal(map[string]any{"id": binding, "kind": kind, "category": category, "title": "画布保留版本", "source": "画布", "coverUrl": "", "tags": []string{}, "createdAt": now, "updatedAt": now, "data": data})
							if err != nil {
								return err
							}
							if err := repo.UpsertAsset(&model.Asset{ID: binding, UserID: userID, Kind: kind, Category: category, Status: model.AssetVersionStatusConfirmed, Title: "画布保留版本", PayloadJSON: string(payload), CreatedAt: now, UpdatedAt: now}); err != nil {
								return err
							}
							created = append(created, binding)
							bindings[id] = binding
						}
					}
					if binding != "" {
						item["assetId"] = binding
					}
				}
			}
			for childKey, child := range item {
				if childKey == "chatSessions" || childKey == "generation" || childKey == "generationSpec" || childKey == "references" {
					continue
				}
				if err := walk(child, childKey); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range item {
				if err := walk(child, key); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(document, ""); err != nil {
		return "", nil, err
	}
	encoded, err := json.Marshal(document)
	return string(encoded), created, err
}
