package app

import (
	"testing"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func TestNewModulePermissionsRejectExistingScopedGrantsBeforeAccessingData(t *testing.T) {
	var oldGrants []model.AdminPermission
	for _, permission := range model.AllAdminPermissions {
		switch permission {
		case model.AdminPermissionSMS, model.AdminPermissionFeedbacks, model.AdminPermissionWorkbenchTools:
		default:
			oldGrants = append(oldGrants, permission)
		}
	}
	actor := &model.User{ID: "scoped-admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: oldGrants}
	svc := &Service{}
	checks := map[string]func() error{
		"sms": func() error {
			_, err := svc.AdminSMSChannels(actor)
			return err
		},
		"feedbacks": func() error {
			_, err := svc.AdminListFeedbacks(actor, repository.FeedbackFilter{})
			return err
		},
		"workbench tools": func() error {
			_, err := svc.workbenchDomain().AdminTools(actor, "")
			return err
		},
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			err, ok := check().(*AppError)
			if !ok || err.Reason != kernel.ReasonAdminPermission {
				t.Fatalf("missing independent permission must fail before accessing the nil repository: %#v", err)
			}
		})
	}
}
