package app

import (
	"testing"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

func TestRequireAdminPermissionUsesScopedGrant(t *testing.T) {
	service := &Service{}
	actor := &model.User{
		Role:             model.UserRoleAdmin,
		AdminLevel:       model.AdminLevelScoped,
		AdminPermissions: []model.AdminPermission{model.AdminPermissionUsers},
	}

	if err := service.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		t.Fatalf("RequireAdminPermission(granted) error = %v", err)
	}
	err := service.RequireAdminPermission(actor, model.AdminPermissionChannels)
	appErr, ok := err.(*AppError)
	if !ok || appErr.Reason != kernel.ReasonAdminPermission {
		t.Fatalf("RequireAdminPermission(missing) error = %#v", err)
	}
}

func TestRequireAdminPermissionAllowsFullAdmin(t *testing.T) {
	service := &Service{}
	actor := &model.User{Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull}
	for _, permission := range model.AllAdminPermissions {
		if err := service.RequireAdminPermission(actor, permission); err != nil {
			t.Fatalf("RequireAdminPermission(%q) error = %v", permission, err)
		}
	}
}

func TestNewAdminModulesRejectOtherScopedGrants(t *testing.T) {
	svc := &Service{}
	checks := []struct {
		name       string
		permission model.AdminPermission
		call       func(*model.User) error
	}{
		{"group list", model.AdminPermissionUserGroups, func(user *model.User) error { _, err := svc.AdminUserGroups(user); return err }},
		{"group create", model.AdminPermissionUserGroups, func(user *model.User) error { _, err := svc.CreateUserGroup(user, UserGroupRequest{}); return err }},
		{"stage settings", model.AdminPermissionEcommerceConfig, func(user *model.User) error { _, err := svc.EcommerceOrchestrationSetting(user); return err }},
		{"knowledge stats", model.AdminPermissionEcommerceKnowledge, func(user *model.User) error { _, err := svc.AdminEcommerceKnowledgeStats(user); return err }},
		{"managed skills", model.AdminPermissionEcommerceKnowledge, func(user *model.User) error { _, err := svc.EcommerceManagedSkills(user); return err }},
	}
	permissions := []model.AdminPermission{model.AdminPermissionUsers, model.AdminPermissionUserGroups, model.AdminPermissionEcommerceConfig, model.AdminPermissionEcommerceKnowledge}
	for _, check := range checks {
		for _, granted := range permissions {
			if granted == check.permission {
				continue
			}
			t.Run(check.name+"/"+string(granted), func(t *testing.T) {
				actor := &model.User{ID: "scoped-admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{granted}}
				err := check.call(actor)
				appErr, ok := err.(*AppError)
				if !ok || appErr.Reason != kernel.ReasonAdminPermission {
					t.Fatalf("expected module permission denial before accessing data, got %#v", err)
				}
			})
		}
	}
}

func TestNormalizeAdminAccessValidatesScopedPermissions(t *testing.T) {
	level, permissions, err := normalizeAdminAccess(&AdminAccessInput{
		Level: model.AdminLevelScoped,
		Permissions: []model.AdminPermission{
			model.AdminPermissionUsers,
			model.AdminPermissionUsers,
			model.AdminPermissionCredits,
		},
	})
	if err != nil {
		t.Fatalf("normalizeAdminAccess() error = %v", err)
	}
	if level != model.AdminLevelScoped || len(permissions) != 2 {
		t.Fatalf("normalizeAdminAccess() = %q, %#v", level, permissions)
	}

	if _, _, err := normalizeAdminAccess(&AdminAccessInput{Level: model.AdminLevelScoped}); err == nil {
		t.Fatal("normalizeAdminAccess(empty scoped permissions) error = nil")
	}
	if _, _, err := normalizeAdminAccess(&AdminAccessInput{Level: model.AdminLevelScoped, Permissions: []model.AdminPermission{"admin.unknown"}}); err == nil {
		t.Fatal("normalizeAdminAccess(unknown permission) error = nil")
	}
}
