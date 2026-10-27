package repository

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Called within the catalog transaction. Only the process that inserts a new
// identity initializes its availability; existing implicit or explicit grants survive.
func (r *Repository) EnsureBuiltinSkillRow(incoming *model.Skill, initiallyAvailable bool) (*model.Skill, bool, error) {
	if incoming == nil || incoming.ID == "" || incoming.Source != model.SkillSourcePlatform || incoming.IsPrivate {
		return nil, false, gorm.ErrInvalidData
	}
	deleted, err := r.BuiltinSkillTombstoned(incoming.ID)
	if err != nil || deleted {
		return nil, false, err
	}
	created := r.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).Create(incoming)
	if created.Error != nil {
		return nil, false, created.Error
	}
	var current model.Skill
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", incoming.ID).Error; err != nil {
		return nil, false, err
	}
	deleted, err = r.BuiltinSkillTombstoned(incoming.ID)
	if err != nil {
		return nil, false, err
	}
	if deleted {
		if created.RowsAffected == 1 {
			if err := r.db.Delete(&model.Skill{}, "id = ?", incoming.ID).Error; err != nil {
				return nil, false, err
			}
		}
		return nil, false, nil
	}
	if current.Source != model.SkillSourcePlatform || current.IsPrivate || (current.SourceType != "" && current.SourceType != "builtin") {
		return nil, false, fmt.Errorf("内置技能 %s 与已有自定义身份冲突，请先核对，未覆盖数据", incoming.ID)
	}
	if created.RowsAffected == 1 && !initiallyAvailable {
		state := model.SkillPlatformState{SkillID: incoming.ID, Available: false}
		if err := r.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "skill_id"}}, DoNothing: true}).Create(&state).Error; err != nil {
			return nil, false, err
		}
	}
	return &current, created.RowsAffected == 1, nil
}

func (r *Repository) CheckBuiltinImageIdentities() ([]model.Skill, error) {
	var retained []model.Skill
	for _, suffix := range []string{"editing", "annotation", "layer-split"} {
		id := "yingce-image-" + suffix
		localID := "open-ai-canvas-image-" + suffix
		conflict := func() error {
			return fmt.Errorf("已有官方图片技能 %s 的身份或版本关系不完整，请先核对；未自动转换数据", id)
		}
		deleted, err := r.BuiltinSkillTombstoned(id)
		if err != nil {
			return nil, err
		}
		var legacy model.Skill
		err = r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&legacy, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			for _, table := range []any{&model.SkillVersion{}, &model.UserSkillState{}, &model.SkillPlatformState{}} {
				if _, platformState := table.(*model.SkillPlatformState); platformState && deleted {
					continue
				}
				var count int64
				if err := r.db.Model(table).Where("skill_id = ?", id).Count(&count).Error; err != nil {
					return nil, err
				}
				if count > 0 {
					return nil, conflict()
				}
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		if deleted || legacy.Source != model.SkillSourcePlatform || legacy.IsPrivate || legacy.SourceType != "builtin" || legacy.CurrentVersionID == "" {
			return nil, conflict()
		}
		var local model.Skill
		err = r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&local, "id = ?", localID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			localDeleted, deleteErr := r.BuiltinSkillTombstoned(localID)
			if deleteErr != nil {
				return nil, deleteErr
			}
			if !localDeleted {
				return nil, conflict()
			}
		} else if err != nil {
			return nil, err
		} else if local.Source != model.SkillSourcePlatform || local.IsPrivate || (local.SourceType != "" && local.SourceType != "builtin") {
			return nil, conflict()
		}
		var versions []model.SkillVersion
		if err := r.db.Where("skill_id = ?", id).Find(&versions).Error; err != nil {
			return nil, err
		}
		knownVersions := make(map[string]model.SkillVersion, len(versions))
		for _, version := range versions {
			if version.ContentHash == "" || version.PackageKey != path.Clean(version.PackageKey) || !strings.HasPrefix(version.PackageKey, id+"/") || strings.Contains(version.PackageKey, "\\") {
				return nil, conflict()
			}
			knownVersions[version.ID] = version
		}
		current, ok := knownVersions[legacy.CurrentVersionID]
		if !ok || current.ContentHash != legacy.ContentHash {
			return nil, conflict()
		}
		var states []model.UserSkillState
		if err := r.db.Where("skill_id = ?", id).Find(&states).Error; err != nil {
			return nil, err
		}
		for _, state := range states {
			_, installed := knownVersions[state.InstalledVersionID]
			if (state.Added || state.InstalledVersionID != "") && !installed {
				return nil, conflict()
			}
		}
		// These deployed identities are retained as history, never synchronized
		// from the new local catalog or merged with another user's installation.
		retained = append(retained, legacy)
	}
	return retained, nil
}
