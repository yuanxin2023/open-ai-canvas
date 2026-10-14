package app

import (
	"bytes"
	"sync"
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/platform"
	"infinite-canvas/backend/internal/repository"
)

func TestAdministratorResourceQuotaLifecycle(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			s, db, actor := quotaTestService(t, driver)
			module := &model.User{ID: "module", Username: "module", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionUsers}, Status: model.UserStatusActive}
			if err := db.Create(module).Error; err != nil {
				t.Fatal(err)
			}
			view, err := s.AdminAdministratorResourceQuota(actor, module.ID)
			if err != nil || len(view.Overrides) != 0 || view.EffectiveQuota != view.DefaultQuota {
				t.Fatal("default inheritance", view, err)
			}
			upload, stored := int64(3), int64(5)
			patch := UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"resourceUploadMB": &upload, "storedFileGB": &stored}}
			view, err = s.PatchAdminAdministratorResourceQuota(actor, module.ID, patch)
			if err != nil || view.EffectiveQuota.ResourceUploadMB != 3 || view.EffectiveQuota.StoredFileGB != 5 {
				t.Fatal(view, err)
			}
			if _, err := s.PatchAdminAdministratorResourceQuota(actor, actor.ID, patch); err != nil {
				t.Fatal("full admin self quota", err)
			}
			for _, denied := range []*model.User{module, {ID: "one", Role: model.UserRoleUser}} {
				if _, err := s.AdminAdministratorResourceQuota(denied, module.ID); err == nil {
					t.Fatal("unauthorized admin quota read")
				}
				if _, err := s.PatchAdminAdministratorResourceQuota(denied, module.ID, patch); err == nil {
					t.Fatal("unauthorized admin quota patch")
				}
			}
			if _, err := s.AdminUserResourceQuota(actor, module.ID); err == nil {
				t.Fatal("ordinary endpoint read admin")
			}
			if _, err := s.PatchAdminUserResourceQuota(actor, module.ID, patch); err == nil {
				t.Fatal("ordinary endpoint patched admin")
			}
			if _, err := s.AdminAdministratorResourceQuota(actor, "one"); err == nil {
				t.Fatal("administrator endpoint read ordinary user")
			}
			if _, err := s.PatchAdminAdministratorResourceQuota(actor, "one", patch); err == nil {
				t.Fatal("administrator endpoint patched ordinary user")
			}
			for _, invalid := range []platform.ResourceQuotaPatch{{"unknown": nil}, {"storedFileGB": new(int64)}} {
				if _, err := s.PatchAdminAdministratorResourceQuota(actor, module.ID, UserResourceQuotaRequest{Overrides: invalid}); err == nil {
					t.Fatal("invalid admin patch accepted")
				}
			}
			policy, err := s.UserRuntimePolicy("two")
			if err != nil || policy.Resource.ResourceUploadMB != 50 {
				t.Fatal("ordinary user quota changed", policy, err)
			}
			resource, err := s.UploadResourceFile(module.ID, "file.bin", 2<<20, "file", 0, 0, 0, bytes.NewReader(bytes.Repeat([]byte("x"), 2<<20)))
			if err != nil || resource.Size != 2<<20 {
				t.Fatal("administrator upload", resource, err)
			}
			usage, err := s.AccountFileStorageUsage(module.ID)
			if err != nil || usage.UsedBytes != 2<<20 || usage.TotalBytes != 5<<30 {
				t.Fatal("admin storage usage", usage, err)
			}
			one := int64(1)
			if _, err := s.PatchAdminAdministratorResourceQuota(actor, module.ID, UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"resourceUploadMB": &one, "generatedFileMB": &one}}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.UploadResourceFile(module.ID, "another.bin", 2<<20, "file", 0, 0, 0, bytes.NewReader(bytes.Repeat([]byte("x"), 2<<20))); err == nil {
				t.Fatal("admin upload bypassed reduced quota")
			}
			if _, err := s.persistGeneratedMediaResult(module.ID, map[string]interface{}{"url": dataURL("image/png", bytes.Repeat([]byte("x"), 2<<20))}); err == nil {
				t.Fatal("admin generation bypassed quota")
			}
			if _, err := s.PatchAdminAdministratorResourceQuota(actor, module.ID, UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"resourceUploadMB": nil}}); err != nil {
				t.Fatal(err)
			}
			global := defaultRuntimePolicy()
			global.Resource.ResourceUploadMB = 40
			if _, err := s.UpdateRuntimePolicySetting(actor, global); err != nil {
				t.Fatal(err)
			}
			view, err = s.AdminAdministratorResourceQuota(actor, module.ID)
			if err != nil || view.EffectiveQuota.ResourceUploadMB != 40 || view.EffectiveQuota.StoredFileGB != 5 {
				t.Fatal("admin dynamic inheritance", view, err)
			}
			called := false
			err = s.repo.PatchUserResourceQuota(module.ID, model.UserRoleUser, func(*repository.Repository) (*model.UserResourceQuota, *model.AdminAuditEvent, error) {
				called = true
				return nil, nil, nil
			})
			if err == nil || called {
				t.Fatal("transaction did not enforce expected role")
			}
			stored = 7
			if _, err := s.PatchAdminUserResourceQuota(actor, "one", UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"storedFileGB": &stored}}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PromoteAdministrator(actor, PromoteAdministratorRequest{UserID: "one", AdminAccess: &AdminAccessInput{Level: model.AdminLevelScoped, Permissions: []model.AdminPermission{model.AdminPermissionUsers}}}); err != nil {
				t.Fatal(err)
			}
			view, err = s.AdminAdministratorResourceQuota(actor, "one")
			if err != nil || view.EffectiveQuota.StoredFileGB != 7 {
				t.Fatal("promotion lost quota", view, err)
			}
			if _, err := s.DemoteAdministrator(actor, "one"); err != nil {
				t.Fatal(err)
			}
			view, err = s.AdminUserResourceQuota(actor, "one")
			if err != nil || view.EffectiveQuota.StoredFileGB != 7 {
				t.Fatal("demotion lost quota", view, err)
			}
			if driver == "postgres" {
				assets, canvases := int64(10), int64(5)
				var wg sync.WaitGroup
				for _, patch := range []platform.ResourceQuotaPatch{{"assetCount": &assets}, {"canvasCount": &canvases}} {
					wg.Add(1)
					go func(patch platform.ResourceQuotaPatch) {
						defer wg.Done()
						if _, err := s.PatchAdminAdministratorResourceQuota(actor, module.ID, UserResourceQuotaRequest{Overrides: patch}); err != nil {
							t.Error(err)
						}
					}(patch)
				}
				wg.Wait()
				view, err = s.AdminAdministratorResourceQuota(actor, module.ID)
				if err != nil || view.EffectiveQuota.AssetCount != 10 || view.EffectiveQuota.CanvasCount != 5 {
					t.Fatal("concurrent admin patch lost fields", view, err)
				}
			}
			var audits int64
			if err := db.Model(&model.AdminAuditEvent{}).Where("action = ? AND target_id = ?", "administrator.resource_quota.update", module.ID).Count(&audits).Error; err != nil || audits < 3 {
				t.Fatal("administrator audit missing", audits, err)
			}
			if err := s.PurgeAdministrator(actor, module.ID); err != nil {
				t.Fatal(err)
			}
			row, err := s.repo.UserResourceQuota(module.ID)
			if err != nil || row != nil {
				t.Fatal("administrator purge retained quota", row, err)
			}
		})
	}
}
