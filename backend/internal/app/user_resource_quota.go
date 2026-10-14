package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/platform"
	"infinite-canvas/backend/internal/repository"
)

type UserResourceQuotaRequest struct {
	Overrides platform.ResourceQuotaPatch `json:"overrides"`
}
type UserResourceQuotaView struct {
	Overrides        platform.ResourceQuotaOverrides `json:"overrides"`
	DefaultQuota     RuntimeResourcePolicy           `json:"defaultQuota"`
	EffectiveQuota   RuntimeResourcePolicy           `json:"effectiveQuota"`
	StorageUsage     repository.UserStorageUsage     `json:"storageUsage"`
	StoredFileBytes  int64                           `json:"storedFileBytes"`
	DailyUploadBytes int64                           `json:"dailyUploadBytes"`
}

func readUserResourceOverrides(repo *repository.Repository, userID string) (platform.ResourceQuotaOverrides, error) {
	row, err := repo.UserResourceQuota(userID)
	if err != nil {
		return nil, err
	}
	overrides := platform.ResourceQuotaOverrides{}
	if row != nil {
		if err := json.Unmarshal([]byte(row.OverridesJSON), &overrides); err != nil {
			return nil, fmt.Errorf("用户资源配额配置无效：%w", err)
		}
		if overrides == nil {
			return nil, fmt.Errorf("用户资源配额配置必须是 JSON 对象")
		}
	}
	return overrides, nil
}

func userResourcePolicyWithRepository(repo *repository.Repository, defaults RuntimeResourcePolicy, userID string) (RuntimeResourcePolicy, error) {
	if strings.TrimSpace(userID) == "" {
		return RuntimeResourcePolicy{}, Unauthorized("请先登录")
	}
	overrides, err := readUserResourceOverrides(repo, userID)
	if err != nil {
		return RuntimeResourcePolicy{}, err
	}
	return platform.MergeResourceQuota(defaults, overrides)
}

func (s *Service) UserRuntimePolicy(userID string) (RuntimePolicySetting, error) {
	policy, err := s.RuntimePolicy()
	if err != nil {
		return RuntimePolicySetting{}, err
	}
	policy.Resource, err = userResourcePolicyWithRepository(s.repo, policy.Resource, userID)
	return policy, err
}

func (s *Service) ValidateUserUploadSize(userID string, size int64) error {
	policy, err := s.UserRuntimePolicy(userID)
	if err != nil {
		return err
	}
	if size <= 0 {
		return BadAuthRequest("上传文件不能为空")
	}
	if size >= megabytes(policy.Resource.ResourceUploadMB) {
		return BadAuthRequest(fmt.Sprintf("单个上传文件必须小于 %dMB", policy.Resource.ResourceUploadMB))
	}
	return nil
}

func (s *Service) AdminUserResourceQuota(actor *model.User, userID string) (*UserResourceQuotaView, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	if _, err := s.requireTargetRole(userID, model.UserRoleUser); err != nil {
		return nil, err
	}
	defaults, err := s.RuntimePolicy()
	if err != nil {
		return nil, err
	}
	overrides, err := readUserResourceOverrides(s.repo, userID)
	if err != nil {
		return nil, err
	}
	effective, err := platform.MergeResourceQuota(defaults.Resource, overrides)
	if err != nil {
		return nil, err
	}
	usage, err := s.repo.UserStorageUsage(userID)
	if err != nil {
		return nil, err
	}
	stored, err := s.repo.UserStoredFileBytes(userID)
	if err != nil {
		return nil, err
	}
	daily, err := s.repo.DailyUploadBytes(userID, time.Now().UTC().Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	return &UserResourceQuotaView{Overrides: overrides, DefaultQuota: defaults.Resource, EffectiveQuota: effective, StorageUsage: usage, StoredFileBytes: stored, DailyUploadBytes: daily}, nil
}

func (s *Service) PatchAdminUserResourceQuota(actor *model.User, userID string, request UserResourceQuotaRequest) (*UserResourceQuotaView, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	if _, err := s.requireTargetRole(userID, model.UserRoleUser); err != nil {
		return nil, err
	}
	if len(request.Overrides) == 0 {
		return nil, BadAuthRequest("请选择需要更新的配额项目")
	}
	defaults, err := s.RuntimePolicy()
	if err != nil {
		return nil, err
	}
	err = s.repo.PatchUserResourceQuota(userID, func(repo *repository.Repository) (*model.UserResourceQuota, *model.AdminAuditEvent, error) {
		current, err := readUserResourceOverrides(repo, userID)
		if err != nil {
			return nil, nil, err
		}
		next, _, err := platform.PatchResourceQuota(defaults.Resource, current, request.Overrides)
		if err != nil {
			return nil, nil, err
		}
		encoded, err := json.Marshal(next)
		if err != nil {
			return nil, nil, err
		}
		var row *model.UserResourceQuota
		if len(next) > 0 {
			row = &model.UserResourceQuota{UserID: userID, OverridesJSON: string(encoded), UpdatedBy: actor.ID, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		}
		metadata, err := json.Marshal(map[string]any{"before": current, "after": next})
		if err != nil {
			return nil, nil, err
		}
		audit := &model.AdminAuditEvent{ID: newID(), ActorUserID: actor.ID, Action: "user.resource_quota.update", TargetType: "user", TargetID: userID, Summary: "更新用户资源配额", MetadataJSON: string(metadata), CreatedAt: time.Now()}
		return row, audit, nil
	})
	if err != nil {
		return nil, err
	}
	return s.AdminUserResourceQuota(actor, userID)
}
