package prompts

import (
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

const customInspirationPrefix = "custom:"

// Generated from the adopted static frontend catalogs. Keep the IDs in sync;
// accepting arbitrary directory keys would bypass the personal content bound.
//
//go:embed inspiration_catalog_ids.json
var inspirationCatalogIDsJSON []byte

var inspirationCatalogIDs = func() map[string]bool {
	var ids []string
	if err := json.Unmarshal(inspirationCatalogIDsJSON, &ids); err != nil {
		panic("invalid embedded inspiration catalog IDs")
	}
	result := make(map[string]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result
}()

type InspirationPromptMetadata struct {
	PromptZh            string `json:"promptZh,omitempty"`
	PromptZhNeedsReview bool   `json:"promptZhNeedsReview,omitempty"`
	Category            string `json:"category,omitempty"`
	Ratio               string `json:"ratio,omitempty"`
	SourceID            string `json:"sourceId,omitempty"`
	Credit              string `json:"credit,omitempty"`
	SourceURL           string `json:"sourceUrl,omitempty"`
}

type InspirationLibraryData struct {
	Favorites      []string                             `json:"favorites"`
	Hidden         []string                             `json:"hidden"`
	Overrides      map[string]string                    `json:"overrides"`
	ImportedCustom map[string]string                    `json:"importedCustom"`
	Metadata       map[string]InspirationPromptMetadata `json:"metadata"`
}

type InspirationLibraryView struct {
	Revision int64                  `json:"revision"`
	State    InspirationLibraryData `json:"state"`
	Prompts  []model.UserPrompt     `json:"prompts"`
	Created  int                    `json:"created,omitempty"`
	Updated  int                    `json:"updated,omitempty"`
}

type InspirationDraft struct {
	Title               string                `json:"title"`
	Description         string                `json:"description"`
	Image               string                `json:"image"`
	CoverResourceID     string                `json:"coverResourceId,omitempty"`
	Mode                model.InspirationMode `json:"mode"`
	Prompt              string                `json:"prompt"`
	PromptZh            string                `json:"promptZh,omitempty"`
	PromptZhNeedsReview bool                  `json:"promptZhNeedsReview,omitempty"`
	Category            string                `json:"category,omitempty"`
	Tags                []string              `json:"tags,omitempty"`
	Ratio               string                `json:"ratio,omitempty"`
}

type InspirationImportEntry struct {
	InspirationDraft
	ID        string `json:"id"`
	SourceID  string `json:"sourceId,omitempty"`
	Credit    string `json:"credit,omitempty"`
	SourceURL string `json:"sourceUrl,omitempty"`
	Featured  bool   `json:"featured,omitempty"`
}

type InspirationImport struct {
	Version    int                         `json:"version"`
	ExportedAt string                      `json:"exportedAt,omitempty"`
	Custom     []InspirationImportEntry    `json:"custom"`
	Overrides  map[string]InspirationDraft `json:"overrides"`
	Favorites  []string                    `json:"favorites"`
	Hidden     []string                    `json:"hidden"`
}

type InspirationLibraryCommand struct {
	ExpectedUserID   string             `json:"expectedUserId"`
	ExpectedRevision *int64             `json:"expectedRevision"`
	Operation        string             `json:"operation"`
	ID               string             `json:"id,omitempty"`
	Draft            *InspirationDraft  `json:"draft,omitempty"`
	Favorited        *bool              `json:"favorited,omitempty"`
	Import           *InspirationImport `json:"payload,omitempty"`
}

func emptyInspirationData() InspirationLibraryData {
	return InspirationLibraryData{Favorites: []string{}, Hidden: []string{}, Overrides: map[string]string{}, ImportedCustom: map[string]string{}, Metadata: map[string]InspirationPromptMetadata{}}
}

func decodeInspirationData(raw string) (InspirationLibraryData, error) {
	data := emptyInspirationData()
	if !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		return data, errors.New("个人灵感状态结构无效")
	}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return data, errors.New("个人灵感状态损坏，已停止写入")
	}
	if data.Overrides == nil || data.ImportedCustom == nil || data.Metadata == nil || data.Favorites == nil || data.Hidden == nil {
		return data, errors.New("个人灵感状态结构无效")
	}
	return data, nil
}

func validBuiltinInspirationCover(value string) bool {
	if strings.ContainsAny(value, "\\?#%") || strings.Contains(value, "..") {
		return false
	}
	if !strings.HasPrefix(value, "/inspiration-thumbs/") && !strings.HasPrefix(value, "/short-drama-styles/") {
		return false
	}
	for _, extension := range []string{".webp", ".jpg", ".jpeg", ".png", ".avif"} {
		if strings.HasSuffix(strings.ToLower(value), extension) {
			return true
		}
	}
	return false
}

func inspirationLibraryChanged() error {
	return &kernel.AppError{Status: http.StatusConflict, Code: http.StatusConflict, Reason: kernel.ReasonInspirationLibraryChanged, Message: "个人灵感已在其他页面更新，请保留草稿，刷新并核对后再保存"}
}

func (s *Service) withInspirationTransaction(userID string, expected *int64, update func(*Service, *InspirationLibraryData) (bool, error)) (int64, error) {
	if strings.TrimSpace(userID) == "" {
		return 0, kernel.Unauthorized("请先登录")
	}
	var revision int64
	err := s.repo.Transaction(func(repo *repository.Repository) error {
		row, err := repo.LockInspirationLibrary(userID)
		if err != nil {
			return err
		}
		if expected != nil && *expected != row.Revision {
			return inspirationLibraryChanged()
		}
		data, err := decodeInspirationData(row.StateJSON)
		if err != nil {
			return err
		}
		txService := &Service{repo: repo, admin: s.admin, libraryState: &data, libraryRevision: row.Revision}
		changed, err := update(txService, &data)
		if err != nil {
			return err
		}
		revision = row.Revision
		if !changed {
			return nil
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			return err
		}
		row.StateJSON, row.Revision, row.UpdatedAt = string(encoded), row.Revision+1, time.Now()
		if err := repo.SaveInspirationLibrary(row, revision); err != nil {
			return err
		}
		revision = row.Revision
		return nil
	})
	return revision, err
}

func (s *Service) InspirationLibrary(userID string) (*InspirationLibraryView, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, kernel.Unauthorized("请先登录")
	}
	view := &InspirationLibraryView{State: emptyInspirationData()}
	err := s.repo.Transaction(func(repo *repository.Repository) error {
		row, err := repo.InspirationLibrary(userID)
		missing := errors.Is(err, gorm.ErrRecordNotFound)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			view.Revision = row.Revision
			view.State, err = decodeInspirationData(row.StateJSON)
			if err != nil {
				return err
			}
		}
		view.Prompts, _, err = repo.UserPrompts(userID, "", "", -1, 0)
		if err == nil && missing {
			// A first write can create the account row between the two reads.
			// Recheck it before returning an unversioned personal snapshot.
			row, readErr := repo.InspirationLibrary(userID)
			if readErr == nil {
				view.Revision = row.Revision
				view.State, err = decodeInspirationData(row.StateJSON)
				if err == nil {
					view.Prompts, _, err = repo.UserPrompts(userID, "", "", -1, 0)
				}
			} else if !errors.Is(readErr, gorm.ErrRecordNotFound) {
				return readErr
			}
		}
		return err
	})
	if err == nil && view.Prompts == nil {
		view.Prompts = []model.UserPrompt{}
	}
	return view, err
}

func validInspirationID(id string) bool {
	if id != strings.TrimSpace(id) || utf8.RuneCountInString(id) > 512 || strings.ContainsAny(id, "\r\n\x00") {
		return false
	}
	if strings.HasPrefix(id, customInspirationPrefix) {
		return strings.TrimSpace(strings.TrimPrefix(id, customInspirationPrefix)) != ""
	}
	return inspirationCatalogIDs[id]
}

func inspirationSet(values []string, id string, enabled bool) []string {
	result := make([]string, 0, len(values)+1)
	for _, value := range values {
		if value != id {
			result = append(result, value)
		}
	}
	if enabled {
		result = append([]string{id}, result...)
	}
	return result
}

func canonicalInspirationID(data *InspirationLibraryData, id string) string {
	if promptID := data.ImportedCustom[id]; promptID != "" {
		return customInspirationPrefix + promptID
	}
	return id
}

func (s *Service) inspirationPromptID(userID, id string, data *InspirationLibraryData) (string, error) {
	if value := data.ImportedCustom[id]; value != "" {
		return value, nil
	}
	if !strings.HasPrefix(id, customInspirationPrefix) {
		return data.Overrides[id], nil
	}
	value := strings.TrimPrefix(id, customInspirationPrefix)
	if _, err := s.repo.UserPromptForUser(userID, value); errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return value, nil
}

func (s *Service) saveInspirationDraft(userID, id string, draft InspirationDraft, data *InspirationLibraryData) (bool, error) {
	if !validInspirationID(id) {
		return false, kernel.BadAuthRequest("灵感 ID 无效")
	}
	if utf8.RuneCountInString(draft.PromptZh) > 20000 || utf8.RuneCountInString(draft.Category) > 64 || utf8.RuneCountInString(draft.Ratio) > 32 {
		return false, kernel.BadAuthRequest("灵感补充信息超出长度限制")
	}
	promptID, err := s.inspirationPromptID(userID, id, data)
	if err != nil {
		return false, err
	}
	req := UserPromptRequest{Title: draft.Title, Description: draft.Description, Mode: draft.Mode, Prompt: draft.Prompt, Tags: draft.Tags, Source: "个人灵感", CoverResourceID: draft.CoverResourceID}
	if draft.CoverResourceID == "" {
		req.CoverURL = draft.Image
	}
	var prompt *model.UserPrompt
	created := promptID == ""
	if created {
		prompt, err = s.CreateUserPrompt(userID, req)
	} else {
		before, readErr := s.repo.UserPromptForUser(userID, promptID)
		if readErr != nil {
			return false, kernel.NotFound("个人副本已删除，请刷新后重试")
		}
		req.Source = before.Source
		prompt, err = s.UpdateUserPrompt(userID, promptID, req)
	}
	if err != nil {
		return false, err
	}
	metadata := data.Metadata[prompt.ID]
	metadata.PromptZh, metadata.Category, metadata.Ratio = draft.PromptZh, draft.Category, draft.Ratio
	metadata.PromptZhNeedsReview = draft.PromptZhNeedsReview
	data.Metadata[prompt.ID] = metadata
	if strings.HasPrefix(id, customInspirationPrefix) {
		if id != customInspirationPrefix+prompt.ID {
			data.ImportedCustom[id] = prompt.ID
		}
	} else {
		data.Overrides[id] = prompt.ID
	}
	return created, nil
}

func (s *Service) ApplyInspirationLibrary(userID string, command InspirationLibraryCommand) (*InspirationLibraryView, error) {
	if strings.TrimSpace(userID) == "" || command.ExpectedUserID != userID {
		return nil, kernel.Unauthorized("账号已变化，请重新打开灵感库")
	}
	if command.ExpectedRevision == nil || *command.ExpectedRevision < 0 {
		return nil, kernel.BadAuthRequest("缺少有效的个人库版本")
	}
	created, updated := 0, 0
	_, err := s.withInspirationTransaction(userID, nil, func(tx *Service, data *InspirationLibraryData) (bool, error) {
		if command.Operation != "import" && !validInspirationID(command.ID) {
			return false, kernel.BadAuthRequest("灵感 ID 无效")
		}
		if command.Operation != "save" {
			command.ID = canonicalInspirationID(data, command.ID)
		}
		if command.Operation != "save" && command.Operation != "import" && strings.HasPrefix(command.ID, customInspirationPrefix) {
			id, err := tx.inspirationPromptID(userID, command.ID, data)
			if err != nil {
				return false, err
			}
			if id == "" {
				return false, kernel.NotFound("个人提示词不存在")
			}
		}
		alreadyApplied := command.Operation == "favorite" && command.Favorited != nil && slices.Contains(data.Favorites, command.ID) == *command.Favorited || command.Operation == "hide" && slices.Contains(data.Hidden, command.ID) || command.Operation == "restore" && !slices.Contains(data.Hidden, command.ID)
		if alreadyApplied {
			return false, nil
		}
		if *command.ExpectedRevision != tx.libraryRevision {
			return false, inspirationLibraryChanged()
		}
		switch command.Operation {
		case "save":
			if command.Draft == nil {
				return false, kernel.BadAuthRequest("缺少灵感内容")
			}
			wasCreated, err := tx.saveInspirationDraft(userID, command.ID, *command.Draft, data)
			if wasCreated {
				created++
			} else {
				updated++
			}
			return true, err
		case "favorite":
			if command.Favorited == nil {
				return false, kernel.BadAuthRequest("缺少收藏状态")
			}
			if strings.HasPrefix(command.ID, customInspirationPrefix) {
				id, err := tx.inspirationPromptID(userID, command.ID, data)
				if err != nil {
					return false, err
				}
				if id == "" {
					return false, kernel.NotFound("个人提示词不存在")
				}
			}
			data.Favorites = inspirationSet(data.Favorites, command.ID, *command.Favorited)
			return true, nil
		case "hide":
			data.Hidden = inspirationSet(data.Hidden, command.ID, true)
			return true, nil
		case "restore":
			data.Hidden = inspirationSet(data.Hidden, command.ID, false)
			return true, nil
		case "delete":
			if !strings.HasPrefix(command.ID, customInspirationPrefix) {
				return false, kernel.BadAuthRequest("目录条目只能隐藏")
			}
			id, err := tx.inspirationPromptID(userID, command.ID, data)
			if err != nil {
				return false, err
			}
			if id == "" {
				return false, kernel.NotFound("个人提示词不存在")
			}
			return true, tx.DeleteUserPrompt(userID, id)
		case "import":
			payload := command.Import
			if payload == nil || payload.Version != 1 {
				return false, kernel.BadAuthRequest("不支持的灵感导入格式")
			}
			for _, entry := range payload.Custom {
				if !strings.HasPrefix(entry.ID, customInspirationPrefix) {
					return false, kernel.BadAuthRequest("导入自建灵感 ID 无效")
				}
				isNew, err := tx.saveInspirationDraft(userID, entry.ID, entry.InspirationDraft, data)
				if err != nil {
					return false, err
				}
				if isNew {
					created++
				} else {
					updated++
				}
				sourceURL, sourceErr := url.Parse(entry.SourceURL)
				if sourceErr != nil || entry.SourceURL != "" && (sourceURL.Scheme != "https" || sourceURL.Host == "" || sourceURL.User != nil) || len(entry.SourceURL) > 2000 || utf8.RuneCountInString(entry.Credit) > 240 {
					return false, kernel.BadAuthRequest("导入来源信息无效")
				}
				if entry.SourceID != "" && !slices.Contains([]string{"original", "chatgpt-prompts", "seedance", "haohaoxue", "youmind"}, entry.SourceID) {
					return false, kernel.BadAuthRequest("导入来源标识无效")
				}
				promptID, err := tx.inspirationPromptID(userID, entry.ID, data)
				if err != nil {
					return false, err
				}
				metadata := data.Metadata[promptID]
				metadata.SourceID, metadata.Credit, metadata.SourceURL = entry.SourceID, entry.Credit, entry.SourceURL
				data.Metadata[promptID] = metadata
			}
			for id, draft := range payload.Overrides {
				if strings.HasPrefix(id, customInspirationPrefix) {
					return false, kernel.BadAuthRequest("导入目录改写 ID 无效")
				}
				isNew, err := tx.saveInspirationDraft(userID, id, draft, data)
				if err != nil {
					return false, err
				}
				if isNew {
					created++
				} else {
					updated++
				}
			}
			for _, id := range payload.Hidden {
				if !validInspirationID(id) {
					return false, kernel.BadAuthRequest("导入隐藏记录 ID 无效")
				}
				id = canonicalInspirationID(data, id)
				if strings.HasPrefix(id, customInspirationPrefix) {
					value, err := tx.inspirationPromptID(userID, id, data)
					if err != nil {
						return false, err
					}
					if value == "" {
						return false, kernel.BadAuthRequest("隐藏记录的个人提示词不存在")
					}
				}
				data.Hidden = inspirationSet(data.Hidden, id, true)
			}
			for _, id := range payload.Favorites {
				if !validInspirationID(id) {
					return false, kernel.BadAuthRequest("导入收藏记录 ID 无效")
				}
				id = canonicalInspirationID(data, id)
				if strings.HasPrefix(id, customInspirationPrefix) {
					value, err := tx.inspirationPromptID(userID, id, data)
					if err != nil {
						return false, err
					}
					if value == "" {
						return false, kernel.BadAuthRequest("收藏记录的个人提示词不存在")
					}
				}
				data.Favorites = inspirationSet(data.Favorites, id, true)
			}
			return true, nil
		default:
			return false, kernel.BadAuthRequest("不支持的个人灵感操作")
		}
	})
	if err != nil {
		return nil, err
	}
	view, err := s.InspirationLibrary(userID)
	if err == nil {
		view.Created, view.Updated = created, updated
	}
	return view, err
}

func removeInspirationPromptLinks(data *InspirationLibraryData, promptID string) {
	ids := []string{customInspirationPrefix + promptID}
	for key, value := range data.ImportedCustom {
		if value == promptID {
			ids = append(ids, key)
			delete(data.ImportedCustom, key)
		}
	}
	for key, value := range data.Overrides {
		if value == promptID {
			delete(data.Overrides, key)
		}
	}
	delete(data.Metadata, promptID)
	for _, id := range ids {
		data.Favorites = inspirationSet(data.Favorites, id, false)
		data.Hidden = inspirationSet(data.Hidden, id, false)
	}
}
