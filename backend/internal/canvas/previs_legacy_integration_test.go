package canvas

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

func legacyStoredCanvas(t *testing.T, svc *Service, ambiguous bool) *model.CanvasProject {
	t.Helper()
	raw, err := os.ReadFile("previslegacy/testdata/director.json")
	if err != nil {
		t.Fatal(err)
	}
	// This fixture exercises document ownership/transactions; resource ownership
	// is covered separately with the unchanged resource collector and media guard.
	raw = []byte(strings.ReplaceAll(string(raw), "/api/resources/", "https://media.example.test/"))
	if ambiguous {
		var doc map[string]json.RawMessage
		_ = json.Unmarshal(raw, &doc)
		doc["previsScenes"] = json.RawMessage(`[]`)
		raw, _ = json.Marshal(doc)
	}
	project := &model.CanvasProject{ID: "legacy-canvas", UserID: "owner", Title: "旧场景", PayloadJSON: string(raw)}
	if err := svc.repo.UpsertCanvasProject(project); err != nil {
		t.Fatal(err)
	}
	return project
}

func TestLegacyCanvasReadProjectionDoesNotRewriteRowsAndProtectsOwner(t *testing.T) {
	svc := newCanvasOperationTestService(t)
	stored := legacyStoredCanvas(t, svc, false)
	raw, err := svc.UserCanvasProject("owner", stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	_ = json.Unmarshal(raw, &doc)
	if doc["directorScenes"] != nil || doc["previsScenes"] == nil {
		t.Fatal("legacy projection not normalized")
	}
	row, err := svc.repo.CanvasProjectForUser("owner", stored.ID)
	if err != nil || row.PayloadJSON != stored.PayloadJSON || row.Revision != stored.Revision {
		t.Fatal("GET rewrote row", err)
	}
	if _, err := svc.UserCanvasProject("another-owner", stored.ID); err == nil {
		t.Fatal("foreign canvas exposed")
	}
}

func TestLegacyCanvasOriginalOperationReceiptRemainsReplayable(t *testing.T) {
	svc := newCanvasOperationTestService(t)
	stored := legacyStoredCanvas(t, svc, false)
	var source map[string]json.RawMessage
	_ = json.Unmarshal([]byte(stored.PayloadJSON), &source)
	var scenes []map[string]any
	_ = json.Unmarshal(source["directorScenes"], &scenes)
	scenes[0]["title"] = "确认编辑"
	value, _ := json.Marshal(scenes)
	req := CanvasOperationsRequest{MutationID: "legacy-request", Operations: []CanvasOperation{{Type: "set_field", Field: "directorScenes", BeforeValue: source["directorScenes"], Value: value}}}
	first, err := svc.ApplyUserCanvasOperations("owner", stored.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.ApplyUserCanvasOperations("owner", stored.ID, req)
	if err != nil || first.Revision != second.Revision {
		t.Fatal("retry applied twice", err)
	}
	receipt, err := svc.repo.CanvasMutationReceiptForUser("owner", stored.ID, req.MutationID)
	if err != nil || receipt.PayloadDigest != CanvasOperationsDigest(req) {
		t.Fatal("original digest changed", err)
	}
	req.Operations[0].Value = source["directorScenes"]
	if _, err := svc.ApplyUserCanvasOperations("owner", stored.ID, req); err == nil {
		t.Fatal("mutation identity reuse accepted")
	}
}

func TestLegacyCanvasConflictRejectsReadsAndWritesWithoutChangingLedger(t *testing.T) {
	svc := newCanvasOperationTestService(t)
	stored := legacyStoredCanvas(t, svc, true)
	_, err := svc.UserCanvasProject("owner", stored.ID)
	var appError *kernel.AppError
	if !errors.As(err, &appError) || appError.Reason != "canvas_previs_migration_conflict" {
		t.Fatalf("missing explicit migration error: %v", err)
	}
	req := CanvasOperationsRequest{MutationID: "must-roll-back", Operations: []CanvasOperation{{Type: "set_field", Field: "title", BeforeValue: json.RawMessage(`"旧场景"`), Value: json.RawMessage(`"不能覆盖"`)}}}
	if _, err := svc.ApplyUserCanvasOperations("owner", stored.ID, req); err == nil {
		t.Fatal("ambiguous stored data overwritten")
	}
	row, err := svc.repo.CanvasProjectForUser("owner", stored.ID)
	if err != nil || row.PayloadJSON != stored.PayloadJSON || row.Revision != stored.Revision {
		t.Fatal("row/ledger changed on rejection", err)
	}
	if _, err := svc.repo.CanvasMutationReceiptForUser("owner", stored.ID, req.MutationID); err == nil {
		t.Fatal("rejected mutation committed a receipt")
	}
}
