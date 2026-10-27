package app

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/platform"
)

func TestChunkUploadReservationsUseAccountQuotaAndReleaseCapacity(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			svc, db, actor := quotaTestService(t, driver)
			upload, daily := int64(2), int64(3)
			if _, err := svc.PatchAdminUserResourceQuota(actor, "one", UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"resourceUploadMB": &upload, "dailyUploadMB": &daily}}); err != nil {
				t.Fatal(err)
			}
			expires := time.Now().Add(time.Hour)
			if err := svc.ReserveChunkUploadSession("one", "too-large", 2<<20, expires, 3); err == nil {
				t.Fatal("chunk admission used the global single-file limit")
			}
			for _, id := range []string{"first", "second"} {
				if err := svc.ReserveChunkUploadSession("one", id, 1<<20, expires, 3); err != nil {
					t.Fatal(err)
				}
			}
			if err := svc.ReserveChunkUploadSession("one", "daily-denied", 1<<20, expires, 3); err == nil {
				t.Fatal("chunk admission ignored pending daily reservations")
			}
			if err := svc.ReserveChunkUploadSession("two", "other-user", 2<<20, expires, 3); err != nil {
				t.Fatalf("account override affected another account: %v", err)
			}
			if err := svc.ReleaseChunkUploadSession("one", "first"); err != nil {
				t.Fatal(err)
			}
			if err := svc.ReserveChunkUploadSession("one", "replacement", 1<<20, expires, 3); err != nil {
				t.Fatalf("released daily capacity was not restored: %v", err)
			}
			var count int64
			if err := db.Model(&model.UploadReservation{}).Where("user_id = ?", "one").Count(&count).Error; err != nil || count != 2 {
				t.Fatalf("unexpected account reservations: %d, %v", count, err)
			}
		})
	}
}

func TestPublishResourceUsesAccountStorageLimitAndRollsBackOnDenial(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			svc, db, actor := quotaTestService(t, driver)
			stored := int64(1)
			if _, err := svc.PatchAdminUserResourceQuota(actor, "one", UserResourceQuotaRequest{Overrides: platform.ResourceQuotaPatch{"storedFileGB": &stored}}); err != nil {
				t.Fatal(err)
			}
			existing := model.Resource{ID: "existing", UserID: "one", Provider: "local", ObjectKey: "existing-file", Status: model.ResourceStatusReady, Size: (1 << 30) - (1 << 20)}
			pending := model.Resource{ID: "pending", UserID: "one", Provider: "local", ObjectKey: "new-file", Status: model.ResourceStatusPending, Size: 2 << 20}
			if err := db.Create(&[]model.Resource{existing, pending}).Error; err != nil {
				t.Fatal(err)
			}
			pending.Status = model.ResourceStatusReady
			if err := svc.saveResourceWithinStorageLimit(&pending, ""); err == nil {
				t.Fatal("publishing used the global storage limit")
			}
			var persisted model.Resource
			if err := db.First(&persisted, "id = ?", pending.ID).Error; err != nil {
				t.Fatal(err)
			}
			if persisted.Status != model.ResourceStatusPending {
				t.Fatal("denied resource publication committed ready bytes")
			}
			other := model.Resource{ID: "other-resource", UserID: "two", Provider: "local", ObjectKey: "other-file", Status: model.ResourceStatusReady, Size: 2 << 20}
			if err := svc.saveResourceWithinStorageLimit(&other, ""); err != nil {
				t.Fatalf("account storage override affected another account: %v", err)
			}
		})
	}
}
