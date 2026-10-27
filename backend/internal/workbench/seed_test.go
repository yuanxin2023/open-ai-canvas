package workbench

import (
	"reflect"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestBuiltinCatalogStartsUnpublishedWithStableIdentities(t *testing.T) {
	tools, presets, err := builtinCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 118 || len(presets) != 354 {
		t.Fatalf("catalog counts = %d tools / %d presets", len(tools), len(presets))
	}
	kinds := map[string]int{}
	toolIDs := map[string]bool{}
	presetCounts := map[string]int{}
	for _, tool := range tools {
		if tool.Enabled || tool.ID == "" || toolIDs[tool.ID] || tool.IconResourceID != "" || tool.CoverResourceID != "" {
			t.Fatalf("new builtin tool must be unpublished with a unique resource-free identity: %#v", tool)
		}
		toolIDs[tool.ID] = true
		kinds[tool.Kind]++
	}
	for _, preset := range presets {
		if preset.Enabled || !toolIDs[preset.ToolID] || preset.PromptTemplate == "" {
			t.Fatalf("new preset must be unpublished and belong to a builtin tool: %#v", preset)
		}
		presetCounts[preset.ToolID]++
	}
	if kinds[KindImage] != 69 || kinds[KindVideo] != 49 {
		t.Fatalf("kind counts = %#v", kinds)
	}
	againTools, againPresets, err := builtinCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for index, tool := range tools {
		if presetCounts[tool.ID] != 3 || againTools[index].ID != tool.ID {
			t.Fatalf("tool identity or preset count changed: %s", tool.ID)
		}
	}
	for index, preset := range presets {
		if againPresets[index].ID != preset.ID {
			t.Fatalf("preset identity changed: %s", preset.ID)
		}
	}
}

func TestEnsureBuiltinCatalogRequiresExplicitPublication(t *testing.T) {
	svc, _, _ := newWorkbenchTest(t)
	if err := svc.EnsureBuiltinCatalog(); err != nil {
		t.Fatal(err)
	}
	admin := &model.User{ID: "admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull}
	for _, kind := range []string{KindImage, KindVideo} {
		tools, err := svc.PublicTools(kind)
		if err != nil {
			t.Fatal(err)
		}
		if len(tools) != 0 {
			t.Fatalf("unreviewed %s tools were published", kind)
		}
		all, err := svc.AdminTools(admin, kind)
		if err != nil || len(all) == 0 || len(all[0].Presets) != 3 {
			t.Fatalf("administrator cannot review %s catalog: %v", kind, err)
		}
		tool := all[0]
		if _, err := svc.ResolveTask("user", TaskInput{Kind: kind, ToolID: tool.ID}); err == nil {
			t.Fatal("unpublished tool accepted a task")
		}
		if err := svc.SetToolEnabled(admin, tool.ID, true); err != nil {
			t.Fatal(err)
		}
		tools, err = svc.PublicTools(kind)
		if err != nil || len(tools) != 1 || len(tools[0].Presets) != 0 {
			t.Fatalf("tool publication must not publish its presets: %#v / %v", tools, err)
		}
		preset := tool.Presets[0]
		if _, err := svc.ResolveTask("user", TaskInput{Kind: kind, ToolID: tool.ID, PresetID: preset.ID}); err == nil {
			t.Fatal("unpublished preset accepted a task")
		}
		if err := svc.SetPresetEnabled(admin, "wrong-tool", preset.ID, true); err == nil {
			t.Fatal("preset publication accepted the wrong parent tool")
		}
		if err := svc.SetPresetEnabled(admin, tool.ID, preset.ID, true); err != nil {
			t.Fatal(err)
		}
		tools, err = svc.PublicTools(kind)
		if err != nil || len(tools) != 1 || len(tools[0].Presets) != 1 || tools[0].Presets[0].ID != preset.ID {
			t.Fatalf("explicit preset publication not reflected: %#v / %v", tools, err)
		}
		if err := svc.SetToolEnabled(admin, tool.ID, false); err != nil {
			t.Fatal(err)
		}
		tools, err = svc.PublicTools(kind)
		if err != nil || len(tools) != 0 {
			t.Fatalf("unpublished tool remained visible: %v", err)
		}
	}
}

func TestEnsureBuiltinCatalogIsRepeatableAndPreservesAdminState(t *testing.T) {
	svc, db, _ := newWorkbenchTest(t)
	seedTools, seedPresets, err := builtinCatalog()
	if err != nil {
		t.Fatal(err)
	}
	editedTool := seedTools[0]
	editedTool.Name = "管理员自定义名称"
	editedTool.Enabled = false
	editedTool.Version = 9
	editedPreset := seedPresets[0]
	editedPreset.Name = "管理员自定义预设"
	editedPreset.PromptTemplate = "管理员自己的提示词"
	editedPreset.Enabled = false
	editedPreset.Version = 6
	publishedTool := seedTools[1]
	publishedTool.Enabled, publishedTool.Version = true, 4
	publishedPreset := seedPresets[3]
	publishedPreset.Enabled, publishedPreset.Version = true, 3
	if err := db.Create(&editedTool).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&editedPreset).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&publishedTool).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&publishedPreset).Error; err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range []struct {
		row any
		id  string
	}{
		{&editedTool, editedTool.ID}, {&editedPreset, editedPreset.ID},
		{&publishedTool, publishedTool.ID}, {&publishedPreset, publishedPreset.ID},
	} {
		if err := db.First(snapshot.row, "id = ?", snapshot.id).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.EnsureBuiltinCatalog(); err != nil {
		t.Fatal(err)
	}
	var firstTools, firstPresets int64
	if err := db.Model(&model.WorkbenchTool{}).Count(&firstTools).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.WorkbenchPreset{}).Count(&firstPresets).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureBuiltinCatalog(); err != nil {
		t.Fatal(err)
	}
	var secondTools, secondPresets int64
	if err := db.Model(&model.WorkbenchTool{}).Count(&secondTools).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.WorkbenchPreset{}).Count(&secondPresets).Error; err != nil {
		t.Fatal(err)
	}
	if firstTools != 118 || firstPresets != 354 || firstTools != secondTools || firstPresets != secondPresets {
		t.Fatalf("repeat seed changed counts: tools %d/%d presets %d/%d", firstTools, secondTools, firstPresets, secondPresets)
	}
	var originalTool model.WorkbenchTool
	var originalPreset model.WorkbenchPreset
	if err := db.First(&originalTool, "id = ?", publishedTool.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&originalPreset, "id = ?", publishedPreset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(originalTool, publishedTool) || !reflect.DeepEqual(originalPreset, publishedPreset) {
		t.Fatal("repeat initialization changed published content, identity, versions or timestamps")
	}
	var gotTool model.WorkbenchTool
	if err := db.First(&gotTool, "id = ?", editedTool.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotTool, editedTool) {
		t.Fatalf("admin tool changed: %#v", gotTool)
	}
	var gotPreset model.WorkbenchPreset
	if err := db.First(&gotPreset, "id = ?", editedPreset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotPreset, editedPreset) {
		t.Fatalf("admin preset changed: %#v", gotPreset)
	}
}
