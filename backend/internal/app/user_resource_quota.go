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
	return s.adminAccountResourceQuota(actor, userID, model.UserRoleUser)
}

func (s *Service) AdminAdministratorResourceQuota(actor *model.User, userID string) (*UserResourceQuotaView, error) {
	return s.adminAccountResourceQuota(actor, userID, model.UserRoleAdmin)
}

func (s *Service) requireResourceQuotaTarget(actor *model.User, userID string, role model.UserRole) (*model.User, error) {
	var err error
	if role == model.UserRoleAdmin {
		err = s.RequireFullAdmin(actor)
	} else {
		err = s.RequireAdminPermission(actor, model.AdminPermissionUsers)
	}
	if err != nil {
		return nil, err
	}
	return s.requireTargetRole(userID, role)
}

func (s *Service) adminAccountResourceQuota(actor *model.User, userID string, role model.UserRole) (*UserResourceQuotaView, error) {
	user, err := s.requireResourceQuotaTarget(actor, userID, role)
	if err != nil {
		return nil, err
	}
	userID = user.ID
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
	return s.patchAdminAccountResourceQuota(actor, userID, model.UserRoleUser, request)
}

func (s *Service) PatchAdminAdministratorResourceQuota(actor *model.User, userID string, request UserResourceQuotaRequest) (*UserResourceQuotaView, error) {
	return s.patchAdminAccountResourceQuota(actor, userID, model.UserRoleAdmin, request)
}

func (s *Service) patchAdminAccountResourceQuota(actor *model.User, userID string, role model.UserRole, request UserResourceQuotaRequest) (*UserResourceQuotaView, error) {
	user, err := s.requireResourceQuotaTarget(actor, userID, role)
	if err != nil {
		return nil, err
	}
	userID = user.ID
	if len(request.Overrides) == 0 {
		return nil, BadAuthRequest("请选择需要更新的配额项目")
	}
	defaults, err := s.RuntimePolicy()
	if err != nil {
		return nil, err
	}
	err = s.repo.PatchUserResourceQuota(userID, role, func(repo *repository.Repository) (*model.UserResourceQuota, *model.AdminAuditEvent, error) {
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
		action, summary := "user.resource_quota.update", "更新用户资源配额"
		if role == model.UserRoleAdmin {
			action, summary = "administrator.resource_quota.update", "更新管理员资源配额"
		}
		audit := &model.AdminAuditEvent{ID: newID(), ActorUserID: actor.ID, Action: action, TargetType: "user", TargetID: userID, Summary: summary, MetadataJSON: string(metadata), CreatedAt: time.Now()}
		return row, audit, nil
	})
	if err != nil {
		return nil, err
	}
	s.invalidateStorageStatus(userID)
	return s.adminAccountResourceQuota(actor, userID, role)
}
