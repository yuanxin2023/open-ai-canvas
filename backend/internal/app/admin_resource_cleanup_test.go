package app

import (
	"encoding/json"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func TestAdminResourceCleanupDeletesAssetAndLastMediaCanvas(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	items := []any{
		&model.Resource{ID: "gone", UserID: admin.ID, Kind: "image", Status: model.ResourceStatusReady},
		&model.Asset{ID: "card", UserID: admin.ID, Kind: "image", PayloadJSON: `{"kind":"image","data":{"storageKey":"resource:gone"}}`},
		&model.Asset{ID: "other-card", UserID: "other", Kind: "image", PayloadJSON: `{"data":{"storageKey":"resource:gone"}}`},
		&model.CanvasProject{ID: "canvas", UserID: admin.ID, Revision: 1, PayloadJSON: `{"nodes":[{"id":"n","type":"image","metadata":{"storageKey":"resource:gone"}},{"id":"t","type":"text","metadata":{"content":"script"}}],"connections":[]}`},
		&model.CanvasProject{ID: "text", UserID: admin.ID, Revision: 1, PayloadJSON: `{"nodes":[{"id":"t","type":"text"}]}`},
	}
	for _, item := range items {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	preview, err := svc.PreviewAdminResourceDelete(admin, AdminResourceDeleteRequest{ResourceIDs: []string{"gone"}})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Impact.Assets != 1 || preview.Impact.Canvases != 1 || preview.Impact.Nodes != 1 {
		t.Fatalf("preview=%#v", preview)
	}
	result, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{"gone"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Impact != preview.Impact {
		t.Fatalf("impact=%#v preview=%#v", result.Impact, preview.Impact)
	}
	assertModelCount(t, db, &model.Asset{}, "id = 'card'", 0)
	assertModelCount(t, db, &model.Asset{}, "id = 'other-card'", 1)
	assertModelCount(t, db, &model.CanvasProject{}, "id = 'canvas'", 0)
	assertModelCount(t, db, &model.CanvasProject{}, "id = 'text'", 1)
	if err := svc.repo.UpsertAsset(&model.Asset{ID: "card", UserID: admin.ID}); err != repository.ErrAdminDeletedEntity {
		t.Fatalf("recreate asset err=%v", err)
	}
	if err := svc.repo.UpsertCanvasProject(&model.CanvasProject{ID: "canvas", UserID: admin.ID, PayloadJSON: `{"nodes":[]}`}); err != repository.ErrAdminDeletedEntity {
		t.Fatalf("recreate canvas err=%v", err)
	}
	page, err := svc.UserDataCleanupChanges(admin.ID, 0)
	if err != nil || len(page.Entries) != 4 || page.Cursor != 1 {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	other, err := svc.UserDataCleanupChanges("other", 0)
	if err != nil || len(other.Entries) != 0 {
		t.Fatalf("other=%#v err=%v", other, err)
	}
}

func TestAdminResourceCleanupPartialThenFinal(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	for _, id := range []string{"a", "b"} {
		if err := db.Create(&model.Resource{ID: id, UserID: admin.ID, Kind: "image", Status: model.ResourceStatusReady}).Error; err != nil {
			t.Fatal(err)
		}
	}
	payload := `{"nodes":[{"id":"a-node","type":"image","metadata":{"storageKey":"resource:a"}},{"id":"b-node","type":"image","metadata":{"storageKey":"resource:b"}}],"connections":[{"id":"e","fromNodeId":"a-node","toNodeId":"b-node"}]}`
	if err := db.Create(&model.CanvasProject{ID: "canvas", UserID: admin.ID, Revision: 1, PayloadJSON: payload}).Error; err != nil {
		t.Fatal(err)
	}
	first, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Impact.UpdatedCanvases != 1 || first.Impact.Canvases != 0 {
		t.Fatalf("first=%#v", first)
	}
	canvas, err := svc.repo.CanvasProjectForUser(admin.ID, "canvas")
	if err != nil {
		t.Fatal(err)
	}
	if canvas.Revision != 2 || strings.Contains(canvas.PayloadJSON, "a-node") {
		t.Fatalf("canvas=%#v", canvas)
	}
	cleaned, err := svc.repo.SanitizeAdminCanvasPayload(admin.ID, "canvas", payload)
	if err != nil || strings.Contains(cleaned, "a-node") {
		t.Fatalf("history=%s err=%v", cleaned, err)
	}
	last, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{"b"}})
	if err != nil {
		t.Fatal(err)
	}
	if last.Impact.Canvases != 1 {
		t.Fatalf("last=%#v", last)
	}
}

func TestAdminResourceCleanupRollbackAndProtection(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	if err := db.Create(&model.Resource{ID: "r", UserID: admin.ID, Kind: "image", Status: model.ResourceStatusReady}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CanvasProject{ID: "c", UserID: admin.ID, Revision: 1, PayloadJSON: `{"nodes":[{"id":"n","type":"image","metadata":{"storageKey":"resource:r"}}]}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Task{ID: "active", UserID: admin.ID, ProjectID: "c", Status: model.TaskStatusRunning}).Error; err != nil {
		t.Fatal(err)
	}
	result, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{"r"}})
	if err != nil || len(result.Blocked) != 1 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if err := db.Model(&model.Task{}).Where("id = 'active'").Update("status", model.TaskStatusSucceeded).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TRIGGER fail_cleanup_audit BEFORE INSERT ON admin_audit_events BEGIN SELECT RAISE(ABORT, 'fail'); END;").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{"r"}}); err == nil {
		t.Fatal("expected rollback")
	}
	assertModelCount(t, db, &model.Resource{}, "id = 'r'", 1)
	assertModelCount(t, db, &model.CanvasProject{}, "id = 'c'", 1)
	assertModelCount(t, db, &model.UserDataCleanupEntry{}, "1=1", 0)
}

func TestAdminResourceCleanupHistoryPreviewAndIdempotency(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	for _, id := range []string{"a", "b"} {
		metadata, _ := json.Marshal(map[string]any{"userId": admin.ID, "deleteMode": "admin_force"})
		if err := db.Create(&model.AdminAuditEvent{ID: id, ActorUserID: admin.ID, Action: "resource.delete", TargetType: "resource", TargetID: id, MetadataJSON: string(metadata)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.CanvasProject{ID: "c", UserID: admin.ID, Revision: 1, PayloadJSON: `{"nodes":[{"id":"a","metadata":{"storageKey":"resource:a"}},{"id":"b","metadata":{"storageKey":"resource:b"}}]}`}).Error; err != nil {
		t.Fatal(err)
	}
	preview, err := svc.CleanupAdminResourceHistory(false)
	if err != nil || preview.Impact.Canvases != 1 {
		t.Fatalf("preview=%#v err=%v", preview, err)
	}
	assertModelCount(t, db, &model.CanvasProject{}, "id='c'", 1)
	assertModelCount(t, db, &model.UserDataCleanupEntry{}, "1=1", 0)
	applied, err := svc.CleanupAdminResourceHistory(true)
	if err != nil || applied.Impact != preview.Impact {
		t.Fatalf("apply=%#v err=%v", applied, err)
	}
	again, err := svc.CleanupAdminResourceHistory(true)
	if err != nil || again.Impact != (AdminResourceCleanupImpact{}) {
		t.Fatalf("again=%#v err=%v", again, err)
	}
}

func TestAdminResourceCleanupAllMediaKindsAndComposite(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	ids := []string{}
	for _, kind := range []string{"image", "video", "audio", "model"} {
		id := "delete-" + kind
		ids = append(ids, id)
		for _, row := range []any{&model.Resource{ID: id, UserID: admin.ID, Kind: kind, Status: model.ResourceStatusReady}, &model.Asset{ID: "card-" + kind, UserID: admin.ID, Kind: kind, Status: model.AssetVersionStatusArchived, PayloadJSON: `{"data":{"storageKey":"resource:` + id + `"}}`}} {
			if err := db.Create(row).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, row := range []any{
		&model.Asset{ID: "character", UserID: admin.ID, Kind: "entity", PayloadJSON: `{"data":{"definition":{"name":"主角","imageUrl":"resource:delete-image"}}}`},
		&model.AssetVersion{ID: "character-v1", AssetID: "character", Version: 1, DefinitionJSON: `{"name":"主角","storageKey":"resource:delete-image"}`},
		&model.AssetRepresentation{ID: "portrait", AssetVersionID: "character-v1", Role: "primary", ResourceID: "delete-image"},
		&model.Asset{ID: "cover-only", UserID: admin.ID, Kind: "image", PayloadJSON: `{"coverUrl":"resource:delete-image","data":{"storageKey":"resource:keep"}}`},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	result, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	if result.Impact.Assets != 4 || len(result.Deleted) != 4 {
		t.Fatalf("result=%#v", result)
	}
	assertModelCount(t, db, &model.Asset{}, "id = 'character'", 1)
	assertModelCount(t, db, &model.Asset{}, "id = 'cover-only'", 1)
	var character model.Asset
	if err := db.First(&character, "id = 'character'").Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(character.PayloadJSON, "主角") || strings.Contains(character.PayloadJSON, "delete-image") {
		t.Fatalf("character=%s", character.PayloadJSON)
	}
	var version model.AssetVersion
	if err := db.First(&version, "id = 'character-v1'").Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(version.DefinitionJSON, "delete-image") {
		t.Fatalf("version=%#v", version)
	}
}

func TestAdminResourceCleanupHistorySkipsUnprovenAndBrokenDocuments(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	for _, row := range []any{
		&model.Asset{ID: "unproven", UserID: admin.ID, Kind: "image", PayloadJSON: `{"data":{"storageKey":"resource:unproven"}}`},
		&model.Asset{ID: "broken", UserID: admin.ID, Kind: "image", PayloadJSON: `{"data":{"storageKey":"resource:proven"`},
		&model.AdminAuditEvent{ID: "audit", ActorUserID: admin.ID, Action: "resource.delete", TargetType: "resource", TargetID: "proven", MetadataJSON: `{"userId":"admin","deleteMode":"admin_force"}`},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	report, err := svc.CleanupAdminResourceHistory(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Skipped) != 1 || report.Impact.Assets != 0 {
		t.Fatalf("report=%#v", report)
	}
	assertModelCount(t, db, &model.Asset{}, "1=1", 2)
	assertModelCount(t, db, &model.UserDataCleanupEntry{}, "1=1", 0)
}
