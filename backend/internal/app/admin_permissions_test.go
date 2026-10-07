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
