package app

import (
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"strings"
)

func (s *Service) RequireFullAdmin(user *model.User) error {
	if err := s.RequireAdmin(user); err != nil {
		return err
	}
	if user.AdminLevel != model.AdminLevelFull && user.AdminLevel != "" {
		return adminPermissionDenied("需要全权限管理员权限")
	}
	return nil
}

func (s *Service) RequireOrdinaryUserTarget(actor *model.User, userID string) (*model.User, error) {
	user, err := s.repo.User(strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	if user.Role == model.UserRoleAdmin && actor.AdminLevel != model.AdminLevelFull && actor.AdminLevel != "" {
		return nil, adminPermissionDenied("只有全权限管理员可以操作管理员账号")
	}
	return user, nil
}

func (s *Service) RequireAdminPermission(user *model.User, permission model.AdminPermission) error {
	if err := s.RequireAdmin(user); err != nil {
		return err
	}
	// 空级别只可能来自未迁移的内存测试对象；持久化管理员会在 v45 迁移后拥有明确级别。
	if user.AdminLevel == "" || user.HasAdminPermission(permission) {
		return nil
	}
	return adminPermissionDenied("缺少该后台模块权限")
}

func (s *Service) RequireAnyAdminPermission(user *model.User, permissions ...model.AdminPermission) error {
	if err := s.RequireAdmin(user); err != nil {
		return err
	}
	if user.AdminLevel == "" || user.AdminLevel == model.AdminLevelFull {
		return nil
	}
	for _, permission := range permissions {
		if user.HasAdminPermission(permission) {
			return nil
		}
	}
	return adminPermissionDenied("缺少该后台模块权限")
}

func adminPermissionDenied(message string) error {
	err := Forbidden(message)
	err.Reason = kernel.ReasonAdminPermission
	return err
}

func normalizeAdminAccess(input *AdminAccessInput) (model.AdminLevel, []model.AdminPermission, error) {
	if input == nil {
		return "", nil, BadAuthRequest("管理员权限配置不能为空")
	}
	if input.Level == model.AdminLevelFull {
		return model.AdminLevelFull, nil, nil
	}
	if input.Level != model.AdminLevelScoped {
		return "", nil, BadAuthRequest("管理员级别无效")
	}
	seen := make(map[model.AdminPermission]struct{}, len(input.Permissions))
	permissions := make([]model.AdminPermission, 0, len(input.Permissions))
	for _, permission := range input.Permissions {
		if !model.IsAdminPermission(permission) {
			return "", nil, BadAuthRequest("管理员权限项无效")
		}
		if _, exists := seen[permission]; exists {
			continue
		}
		seen[permission] = struct{}{}
		permissions = append(permissions, permission)
	}
	if len(permissions) == 0 {
		return "", nil, BadAuthRequest("模块管理员至少需要一个模块权限")
	}
	return model.AdminLevelScoped, permissions, nil
}

func adminAccessView(user *model.User) *AdminAccessView {
	if user == nil || user.Role != model.UserRoleAdmin {
		return nil
	}
	permissions := user.AdminPermissions
	level := user.AdminLevel
	if level == model.AdminLevelFull || level == "" {
		level = model.AdminLevelFull
		permissions = model.AllAdminPermissions
	}
	return &AdminAccessView{Level: level, Permissions: append([]model.AdminPermission(nil), permissions...)}
}

func adminPermissionsEqual(left, right []model.AdminPermission) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[model.AdminPermission]int, len(left))
	for _, permission := range left {
		counts[permission]++
	}
	for _, permission := range right {
		counts[permission]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func normalizeAdminRemark(value string) (string, error) {
	remark := strings.TrimSpace(value)
	if len([]rune(remark)) > 500 {
		return "", BadAuthRequest("用户备注不能超过 500 个字符")
	}
	return remark, nil
}

func (s *Service) requireTargetRole(userID string, role model.UserRole) (*model.User, error) {
	user, err := s.repo.User(strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	if user.Role != role {
		if role == model.UserRoleAdmin {
			return nil, BadAuthRequest("目标账号不是管理员")
		}
		return nil, BadAuthRequest("目标账号不是普通用户")
	}
	return user, nil
}
