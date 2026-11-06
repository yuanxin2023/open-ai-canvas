package app

import (
	"context"
	"testing"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

func TestUpstreamV181ManagementRejectsUnrelatedScopedAdmin(t *testing.T) {
	svc := &Service{}
	actor := &model.User{ID: "unrelated-admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionChannels}}
	ctx := context.Background()
	calls := map[string]func() error{
		"pose list":   func() error { _, err := svc.AdminPoseLibraryItems(actor, PoseLibraryAdminQuery{}); return err },
		"pose import": func() error { _, err := svc.ImportPoseLibrary(actor, PoseLibraryImportRequest{}); return err },
		"pose sync":   func() error { _, err := svc.SyncPoseLibraryImages(ctx, actor, PoseLibrarySyncRequest{}); return err },
		"pose update": func() error {
			_, err := svc.UpdatePoseLibraryItem(actor, "pose", PoseLibraryUpdateItemRequest{})
			return err
		},
		"pose delete": func() error { return svc.DeletePoseLibraryItem(actor, "pose") },
		"pose bulk":   func() error { _, err := svc.BulkPoseLibraryItems(actor, PoseLibraryBulkRequest{}); return err },
		"pose create": func() error { _, err := svc.CreatePoseLibraryItem(actor, PoseLibraryCreateItemRequest{}); return err },
		"feed read":   func() error { _, err := svc.AdminInspirationFeed(actor); return err },
		"feed settings": func() error {
			_, err := svc.UpdateInspirationFeedSettings(actor, InspirationFeedSettingsUpdate{})
			return err
		},
		"feed sync":  func() error { _, err := svc.SyncInspirationFeed(ctx, actor); return err },
		"ark list":   func() error { _, err := svc.AdminArkGroups(actor, AdminArkGroupQuery{}); return err },
		"ark read":   func() error { _, err := svc.AdminArkGroup(actor, "group"); return err },
		"ark create": func() error { _, err := svc.AdminCreateArkGroup(ctx, actor, AdminArkGroupCreateRequest{}); return err },
		"ark update": func() error {
			_, err := svc.AdminUpdateArkGroup(actor, "group", AdminArkGroupUpdateRequest{})
			return err
		},
		"ark delete": func() error { return svc.AdminDeleteArkGroup(ctx, actor, "group") },
		"ark upload": func() error {
			_, err := svc.AdminCreateArkPortraitAsset(ctx, actor, "group", ArkAssetUploadInput{})
			return err
		},
		"ark asset delete": func() error { return svc.AdminDeleteArkPortraitAsset(ctx, actor, "asset") },
		"ark review":       func() error { _, err := svc.AdminArkReviewQueue(actor, "", 1, 20); return err },
		"ark approve":      func() error { _, err := svc.AdminApproveArkAssets(ctx, actor, []string{"asset"}); return err },
		"ark reject":       func() error { _, err := svc.AdminRejectArkAssets(actor, []string{"asset"}, "fixture"); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err, ok := call().(*AppError)
			if !ok || err.Reason != kernel.ReasonAdminPermission {
				t.Fatalf("operation did not deny before accessing data or upstream: %#v", err)
			}
		})
	}
}
