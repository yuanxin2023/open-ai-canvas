package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

func TestDeleteAdminResourcesSupportsPartialSuccessAndBatchedReferences(t *testing.T) {
	svc, db, dataDir, admin := newAdminStorageDeleteTestService(t)
	user := model.User{ID: "user-1", Username: "user-1", AvatarResourceID: "resource-blocked", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	resources := []model.Resource{
		{ID: "resource-free", UserID: "user-1", Kind: "image", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "users/user-1/image/delete-directory"},
		{ID: "resource-blocked", UserID: "user-1", Kind: "image", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "users/user-1/image/blocked.png"},
	}
	if err := db.Create(&resources).Error; err != nil {
		t.Fatal(err)
	}
	directoryPath := filepath.Join(dataDir, "resources", filepath.FromSlash(resources[0].ObjectKey))
	if err := os.MkdirAll(directoryPath, 0o750); err != nil {
		t.Fatal(err)
	}

	result, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{"resource-free", "resource-blocked", "resource-free"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "resource-free" {
		t.Fatalf("deleted = %#v", result.Deleted)
	}
	if len(result.Blocked) != 1 || result.Blocked[0].ID != "resource-blocked" || len(result.Blocked[0].References) != 1 || result.Blocked[0].References[0].Kind != "个人头像" {
		t.Fatalf("blocked = %#v", result.Blocked)
	}
	assertModelCount(t, db, &model.Resource{}, "id = ?", 0, "resource-free")
	assertModelCount(t, db, &model.Resource{}, "id = ?", 1, "resource-blocked")
	assertModelCount(t, db, &model.ResourceDeletionJob{}, "resource_id = ?", 1, "resource-free")
	assertModelCount(t, db, &model.AdminAuditEvent{}, "action = ? AND target_id = ?", 1, "resource.delete", "resource-free")
}

func TestDeleteAdminResourcesClearsAnnouncementImage(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	resource := model.Resource{ID: "announcement-image", UserID: admin.ID, Kind: "image", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "announcements/cover.png"}
	announcement := model.Announcement{ID: "announcement-1", Title: "维护通知", Content: "content", ImageResourceID: resource.ID, CreatedBy: admin.ID, Status: model.AnnouncementStatusActive}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&announcement).Error; err != nil {
		t.Fatal(err)
	}

	result, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{resource.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 1 || len(result.Blocked) != 0 {
		t.Fatalf("result = %#v", result)
	}
	assertModelCount(t, db, &model.Resource{}, "id = ?", 0, resource.ID)
	var stored model.Announcement
	if err := db.First(&stored, "id = ?", announcement.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ImageResourceID != "" {
		t.Fatalf("announcement image resource = %q", stored.ImageResourceID)
	}
}

func TestDeleteAdminResourcesBlocksDarkAppearanceLogo(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	resource := model.Resource{ID: "appearance-logo", UserID: admin.ID, Kind: "image", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "appearance/logo.png", MimeType: "image/png"}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateAppearance(admin, AppearanceSetting{BrandName: "HIMA Studio", BrandSlug: "hima-studio", AuthHeroTitle: defaultAppearanceHeroTitle, DarkLogoResourceID: resource.ID, LogoFrameEnabled: true}); err != nil {
		t.Fatal(err)
	}

	result, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{resource.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 0 || len(result.Blocked) != 1 || len(result.Blocked[0].References) != 1 || result.Blocked[0].References[0].Kind != "外观" {
		t.Fatalf("result = %#v", result)
	}
	assertModelCount(t, db, &model.Resource{}, "id = ?", 1, resource.ID)
}

func TestDeleteAdminResourcesBlocksCustomerServiceImage(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	resource := model.Resource{ID: "customer-image", UserID: admin.ID, Kind: "image", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "customer/button.png", MimeType: "image/png"}
	setting := model.SystemSetting{Key: customerServiceSettingKey, ValueJSON: `{"imageResourceId":"customer-image"}`, UpdatedBy: admin.ID}
	for _, item := range []any{&resource, &setting} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}

	result, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{resource.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 0 || len(result.Blocked) != 1 || len(result.Blocked[0].References) != 1 || result.Blocked[0].References[0].Kind != "客服" {
		t.Fatalf("result = %#v", result)
	}
	assertModelCount(t, db, &model.Resource{}, "id = ?", 1, resource.ID)
}

func TestDeleteAdminResourcesIgnoresTerminalTaskOutputHistory(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	resources := []model.Resource{
		{ID: "terminal-output", UserID: "user-1", Kind: "image", Status: model.ResourceStatusReady, Provider: "local"},
		{ID: "running-output", UserID: "user-1", Kind: "image", Status: model.ResourceStatusReady, Provider: "local"},
		{ID: "terminal-input", UserID: "user-1", Kind: "image", Status: model.ResourceStatusReady, Provider: "local"},
	}
	for _, item := range []any{
		&resources,
		&model.Task{ID: "terminal-task", UserID: "user-1", Status: model.TaskStatusSucceeded, InputJSON: `{"storageKey":"resource:terminal-input"}`, ResultJSON: `{"storageKey":"resource:terminal-output"}`},
		&model.Task{ID: "running-task", UserID: "user-1", Status: model.TaskStatusRunning, ResultJSON: `{"storageKey":"resource:running-output"}`},
		&model.TaskLog{ID: "terminal-log", UserID: "user-1", TaskID: "terminal-task", Payload: `{"resourceId":"terminal-output"}`},
		&model.Result{ID: "terminal-result", UserID: "user-1", TaskID: "terminal-task", URL: "/api/resources/terminal-output/file"},
	} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}

	result, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{"terminal-output", "running-output", "terminal-input"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 2 || result.Deleted[0] != "terminal-output" || result.Deleted[1] != "terminal-input" {
		t.Fatalf("deleted = %#v", result.Deleted)
	}
	if len(result.Blocked) != 1 || result.Blocked[0].ID != "running-output" || result.Blocked[0].References[0].Kind != "活动任务" {
		t.Fatalf("blocked = %#v", result.Blocked)
	}
	assertModelCount(t, db, &model.Resource{}, "id = ?", 0, "terminal-output")
	assertModelCount(t, db, &model.Resource{}, "id = ?", 0, "terminal-input")
	assertModelCount(t, db, &model.Resource{}, "id = ?", 1, "running-output")
}

func TestAdminResourceReferencesKeepsOnlyDeletionBlockingTaskReferences(t *testing.T) {
	resources := []model.Resource{{ID: "terminal-output"}, {ID: "running-output"}, {ID: "terminal-input"}, {ID: "avatar"}, {ID: "agent-input"}, {ID: "creation-input"}}
	snapshot := repository.ResourceReferenceSnapshot{Direct: []repository.ResourceDirectReference{
		{Kind: "个人头像", ID: "user", ResourceID: "avatar"},
		{Kind: "Agent 待执行引用", ID: "approval", ResourceID: "agent-input"},
	}, Documents: []repository.ResourceReferenceDocument{
		{Kind: "任务", ID: "terminal-task", TaskStatus: model.TaskStatusSucceeded, PrimaryJSON: `{"storageKey":"resource:terminal-input"}`, SecondaryJSON: `{"storageKey":"resource:terminal-output"}`},
		{Kind: "任务日志", ID: "terminal-log", TaskStatus: model.TaskStatusSucceeded, PrimaryJSON: `{"resourceId":"terminal-output"}`},
		{Kind: "任务结果", ID: "terminal-result", TaskStatus: model.TaskStatusFailed, PrimaryJSON: `/api/resources/terminal-output/file`},
		{Kind: "任务", ID: "running-task", TaskStatus: model.TaskStatusRunning, SecondaryJSON: `{"storageKey":"resource:running-output"}`},
		{Kind: "创作会话", ID: "creation", ExecutionStatus: "waiting_task", PrimaryJSON: `{"storageKey":"resource:creation-input"}`},
	}}

	references := adminResourceReferences(snapshot, resources)
	if _, exists := references["terminal-output"]; exists {
		t.Fatalf("terminal output history must not block deletion: %#v", references["terminal-output"])
	}
	if len(references["running-output"]) != 1 || references["running-output"][0].ID != "running-task" {
		t.Fatalf("running output references = %#v", references["running-output"])
	}
	if _, exists := references["terminal-input"]; exists {
		t.Fatalf("terminal input history must not block deletion: %#v", references["terminal-input"])
	}
	for resourceID, kind := range map[string]string{"avatar": "个人头像", "agent-input": "活动 Agent", "creation-input": "活动创作"} {
		if len(references[resourceID]) != 1 || references[resourceID][0].Kind != kind {
			t.Fatalf("%s references = %#v", resourceID, references[resourceID])
		}
	}
}

func TestDeleteAdminResourcesCleansStructuredReferencesAndKeepsHistory(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	resource := model.Resource{ID: "force-delete", UserID: admin.ID, Kind: "image", Status: model.ResourceStatusReady, Provider: "local"}
	asset := model.Asset{ID: "asset", UserID: admin.ID, Title: "素材", PayloadJSON: `{"storageKey":"resource:force-delete"}`}
	version := model.AssetVersion{ID: "version", AssetID: asset.ID, Version: 1, DefinitionJSON: asset.PayloadJSON}
	snapshot := model.CanvasSnapshot{ID: "snapshot", CanvasID: "canvas", UserID: admin.ID, Revision: 1, Title: "历史", PayloadJSON: asset.PayloadJSON, CreatedAt: time.Now()}
	items := []any{
		&resource,
		&model.CanvasProject{ID: "canvas", UserID: admin.ID, Title: "画布", PayloadJSON: asset.PayloadJSON},
		&snapshot,
		&model.CanvasSnapshotResource{SnapshotID: snapshot.ID, ResourceID: resource.ID},
		&asset,
		&version,
		&model.AssetRepresentation{ID: "representation", AssetVersionID: version.ID, ResourceID: resource.ID, Role: "primary"},
		&model.VoiceProfile{ID: "voice", UserID: admin.ID, Name: "声音", Provider: "test", VoiceKey: "voice", SampleResourceID: resource.ID},
		&model.Project{ID: "project", UserID: admin.ID, Name: "项目", CoverResourceID: resource.ID},
		&model.Announcement{ID: "announcement", Title: "公告", ImageResourceID: resource.ID, CreatedBy: admin.ID},
		&model.Inspiration{ID: "inspiration", Title: "灵感", CoverResourceID: resource.ID, CoverURL: "/api/resources/force-delete/file", CoverWidth: 100, CoverHeight: 100},
		&model.UserPrompt{ID: "prompt", UserID: admin.ID, Title: "提示词", CoverResourceID: resource.ID, CoverURL: "/api/resources/force-delete/file"},
		&model.ShotArtifact{ID: "artifact", ProjectID: "project", ShotID: "shot", Type: "image", Version: 1, ResourceID: resource.ID},
		&model.Tool{ID: 100, OwnerID: admin.ID, Label: "工具", Cover: "/api/resources/force-delete/file", MediaURL: "resource:force-delete", ExtraInfoJSON: `["resource:force-delete","https://example.com/keep.png"]`},
	}
	for _, item := range items {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}

	result, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{resource.ID}, ConfirmInspirationCovers: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 1 || len(result.Blocked) != 0 || len(result.Warnings) != 0 {
		t.Fatalf("result = %#v", result)
	}
	for _, check := range []struct {
		model any
		query string
	}{
		{&model.Resource{}, "id = 'force-delete'"},
		{&model.CanvasSnapshotResource{}, "resource_id = 'force-delete'"},
		{&model.AssetRepresentation{}, "resource_id = 'force-delete'"},
		{&model.ShotArtifact{}, "resource_id = 'force-delete'"},
	} {
		assertModelCount(t, db, check.model, check.query, 0)
	}
	for _, check := range []struct {
		model any
		query string
	}{
		{&model.CanvasProject{}, "id = 'canvas'"},
		{&model.CanvasSnapshot{}, "id = 'snapshot'"},
		{&model.Asset{}, "id = 'asset'"},
		{&model.Project{}, "id = 'project'"},
		{&model.Announcement{}, "id = 'announcement'"},
		{&model.Inspiration{}, "id = 'inspiration'"},
		{&model.UserPrompt{}, "id = 'prompt'"},
		{&model.VoiceProfile{}, "id = 'voice'"},
	} {
		assertModelCount(t, db, check.model, check.query, 1)
	}
	var storedSnapshot model.CanvasSnapshot
	if err := db.First(&storedSnapshot, "id = ?", snapshot.ID).Error; err != nil || storedSnapshot.PayloadJSON != asset.PayloadJSON {
		t.Fatalf("snapshot = %#v, error = %v", storedSnapshot, err)
	}
	var storedTool model.Tool
	if err := db.First(&storedTool, "id = ?", 100).Error; err != nil {
		t.Fatal(err)
	}
	if storedTool.Cover != "" || storedTool.MediaURL != "" || storedTool.ExtraInfoJSON != `["https://example.com/keep.png"]` {
		t.Fatalf("tool cleanup = %#v", storedTool)
	}
	for _, value := range []struct {
		model any
		field string
	}{
		{&model.Project{}, "cover_resource_id"}, {&model.Announcement{}, "image_resource_id"},
		{&model.Inspiration{}, "cover_resource_id"}, {&model.UserPrompt{}, "cover_resource_id"}, {&model.VoiceProfile{}, "sample_resource_id"},
	} {
		assertModelCount(t, db, value.model, value.field+" <> ''", 0)
	}
}

func TestDeleteAdminResourcesRequiresInspirationCoverConfirmation(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	resources := []model.Resource{
		{ID: "inspiration-cover", UserID: admin.ID, Kind: "image", Status: model.ResourceStatusReady, Provider: "local"},
		{ID: "ordinary-image", UserID: admin.ID, Kind: "image", Status: model.ResourceStatusReady, Provider: "local"},
	}
	inspiration := model.Inspiration{ID: "inspiration-1", Title: "电影感雨夜", CoverResourceID: resources[0].ID, CoverURL: "/api/resources/inspiration-cover/file", CoverWidth: 1200, CoverHeight: 800}
	if err := db.Create(&resources).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&inspiration).Error; err != nil {
		t.Fatal(err)
	}

	preview, err := svc.PreviewAdminResourceDelete(admin, AdminResourceDeleteRequest{ResourceIDs: []string{resources[1].ID, resources[0].ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.InspirationCovers) != 1 || preview.InspirationCovers[0].ID != resources[0].ID || len(preview.InspirationCovers[0].References) != 1 || preview.InspirationCovers[0].References[0].Title != inspiration.Title {
		t.Fatalf("preview = %#v", preview)
	}

	if _, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{resources[0].ID, resources[1].ID}}); err == nil {
		t.Fatal("expected inspiration cover confirmation rejection")
	} else if appErr, ok := err.(*AppError); !ok || appErr.Reason != reasonInspirationCoverConfirmationRequired {
		t.Fatalf("error = %#v", err)
	}
	assertModelCount(t, db, &model.Resource{}, "id IN ?", 2, []string{resources[0].ID, resources[1].ID})

	result, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{resources[0].ID, resources[1].ID}, ConfirmInspirationCovers: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 2 || len(result.Blocked) != 0 {
		t.Fatalf("result = %#v", result)
	}
	var stored model.Inspiration
	if err := db.First(&stored, "id = ?", inspiration.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CoverResourceID != "" || stored.CoverURL != "" || stored.CoverWidth != 0 || stored.CoverHeight != 0 {
		t.Fatalf("inspiration cover was not cleared: %#v", stored)
	}
}

func TestDeleteAdminResourcesWarnsWhenPhysicalProviderIsUnsupported(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	resource := model.Resource{ID: "legacy-resource", UserID: admin.ID, Kind: "image", Status: model.ResourceStatusReady, Provider: "legacy", ObjectKey: "legacy/image.png"}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}

	result, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{resource.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 1 || len(result.Warnings) != 1 || result.Warnings[0].ID != resource.ID {
		t.Fatalf("result = %#v", result)
	}
	assertModelCount(t, db, &model.Resource{}, "id = ?", 0, resource.ID)
	assertModelCount(t, db, &model.ResourceDeletionJob{}, "resource_id = ?", 0, resource.ID)
	var audit model.AdminAuditEvent
	if err := db.First(&audit, "target_id = ?", resource.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(audit.MetadataJSON, `"physicalDeleteSkippedReason":"unsupported_provider"`) || !strings.Contains(audit.MetadataJSON, `"deleteMode":"admin_force"`) {
		t.Fatalf("audit metadata = %s", audit.MetadataJSON)
	}
}

func TestDeleteAdminResourcesKeepsSharedPhysicalObject(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	resources := []model.Resource{
		{ID: "resource-delete", UserID: "user-1", Kind: "image", Status: model.ResourceStatusReady, Provider: "", ObjectKey: "shared/object.png"},
		{ID: "resource-keep", UserID: "user-2", Kind: "image", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "shared/object.png"},
	}
	if err := db.Create(&resources).Error; err != nil {
		t.Fatal(err)
	}

	result, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{"resource-delete"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 1 {
		t.Fatalf("result = %#v", result)
	}
	assertModelCount(t, db, &model.Resource{}, "id = ?", 0, "resource-delete")
	assertModelCount(t, db, &model.Resource{}, "id = ?", 1, "resource-keep")
	assertModelCount(t, db, &model.ResourceDeletionJob{}, "1 = 1", 0)
	var audit model.AdminAuditEvent
	if err := db.First(&audit, "target_id = ?", "resource-delete").Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(audit.MetadataJSON, `"physicalDeleteQueued":false`) {
		t.Fatalf("audit metadata = %s", audit.MetadataJSON)
	}
}

func TestDeleteAdminResourcesRollsBackWhenAuditInsertFails(t *testing.T) {
	svc, db, _, admin := newAdminStorageDeleteTestService(t)
	resource := model.Resource{ID: "resource-rollback", UserID: admin.ID, Kind: "image", Status: model.ResourceStatusReady, Provider: "local"}
	binding := model.ArkPrivateAssetBinding{ID: "binding-1", UserID: admin.ID, ResourceID: resource.ID, ProjectName: "project", Status: "active"}
	draft := model.AnnouncementImageDraft{ResourceID: resource.ID, UserID: admin.ID}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TRIGGER fail_resource_audit BEFORE INSERT ON admin_audit_events WHEN NEW.action = 'resource.delete' BEGIN SELECT RAISE(ABORT, 'forced audit failure'); END;").Error; err != nil {
		t.Fatal(err)
	}

	if _, err := svc.DeleteAdminResources(admin, AdminResourceDeleteRequest{ResourceIDs: []string{resource.ID}}); err == nil {
		t.Fatal("expected audit failure")
	}
	assertModelCount(t, db, &model.Resource{}, "id = ?", 1, resource.ID)
	assertModelCount(t, db, &model.ArkPrivateAssetBinding{}, "resource_id = ?", 1, resource.ID)
	assertModelCount(t, db, &model.AnnouncementImageDraft{}, "resource_id = ?", 1, resource.ID)
	assertModelCount(t, db, &model.ResourceDeletionJob{}, "1 = 1", 0)
}

func TestDeleteAdminResourcesRejectsNonAdminAndOversizedBatch(t *testing.T) {
	svc := &Service{}
	user := &model.User{ID: "user", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if _, err := svc.PreviewAdminResourceDelete(user, AdminResourceDeleteRequest{ResourceIDs: []string{"resource-1"}}); err == nil {
		t.Fatal("expected non-admin preview rejection")
	}
	if _, err := svc.DeleteAdminResources(user, AdminResourceDeleteRequest{ResourceIDs: []string{"resource-1"}}); err == nil {
		t.Fatal("expected non-admin rejection")
	}
	ids := make([]string, maxAdminResourceDeleteCount+1)
	for index := range ids {
		ids[index] = newID()
	}
	if _, err := normalizeAdminResourceDeleteIDs(ids); err == nil {
		t.Fatal("expected oversized batch rejection")
	}
}

func newAdminStorageDeleteTestService(t *testing.T) (*Service, *gorm.DB, string, *model.User) {
	t.Helper()
	db := newSQLiteTestDB(t)
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	admin := &model.User{ID: "admin", Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	svc := New(repository.New(db), dataDir)
	startDeletionTestWorkers(t, svc)
	return svc, db, dataDir, admin
}

func assertModelCount(t *testing.T, db *gorm.DB, value any, query string, expected int64, args ...any) {
	t.Helper()
	var count int64
	dbQuery := db.Model(value).Where(query, args...)
	if err := dbQuery.Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("count = %d, want %d", count, expected)
	}
}
