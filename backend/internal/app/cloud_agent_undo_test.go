package app

import (
	"encoding/json"
	"errors"
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func TestUndoCloudAgentCanvasRestoresLatestMutationAndIsIdempotent(t *testing.T) {
	s, db, _, _ := creationTestService(t)
	canvas := model.CanvasProject{ID: "agent-canvas", UserID: "user", PayloadJSON: `{"nodes":[]}`}
	if err := db.Create(&canvas).Error; err != nil {
		t.Fatal(err)
	}
	run, err := s.CreateCloudAgentRun("user", agentTestRequest(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CloudAgentRun("user", run.ID); err != nil {
		t.Fatal(err)
	}
	execution, err := s.repo.CloudAgent("user", run.ID)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := s.RuntimePolicy()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := creationDocument(canvas.PayloadJSON)
	if err != nil {
		t.Fatal(err)
	}
	beforeHash := cloudAgentCanvasHash(doc)
	call := cloudAgentCall{ID: "canvas-call-1"}
	args, _ := json.Marshal(map[string]any{
		"snapshotHash": beforeHash,
		"ops":          []map[string]any{{"type": "add_node", "id": "note-1", "nodeType": "text", "title": "临时节点", "content": "待撤销", "x": 10, "y": 20}},
	})
	call.Function.Arguments = string(args)
	if err := s.repo.MutateCloudAgent("user", run.ID, execution.Revision, func(current *model.CloudAgentExecution, repo *repository.Repository) error {
		_, err := applyCloudAgentCanvas(repo, "user", "agent-canvas", call, policy, cloudAgentMutationRecorderForRun(run.ID))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	mutated, _ := s.repo.CanvasProjectForUser("user", "agent-canvas")
	mutatedDoc, _ := creationDocument(mutated.PayloadJSON)
	afterHash := cloudAgentCanvasHash(mutatedDoc)
	if afterHash == beforeHash {
		t.Fatal("canvas mutation did not change snapshot")
	}
	for _, rejected := range []struct {
		name, userID, stepID, hash string
	}{
		{"missing step receipt", "user", "", afterHash},
		{"different step", "user", "other-call", afterHash},
		{"different owner", "other-user", call.ID, afterHash},
		{"missing hash", "user", call.ID, ""},
	} {
		t.Run(rejected.name, func(t *testing.T) {
			if _, err := s.UndoCloudAgentCanvas(rejected.userID, run.ID, rejected.stepID, rejected.hash, "拒绝路径"); err == nil {
				t.Fatal("invalid receipt or foreign owner was accepted")
			}
		})
	}
	mutationBefore, err := s.repo.LatestCloudAgentCanvasMutation("user", run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(mutationBefore).Update("has_submitted_task", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.UndoCloudAgentCanvas("user", run.ID, call.ID, afterHash, "收费任务拒绝"); err == nil {
		t.Fatal("a submitted generation task must prevent canvas undo")
	}
	unchanged, err := s.repo.CanvasProjectForUser("user", "agent-canvas")
	if err != nil || unchanged.PayloadJSON != mutated.PayloadJSON {
		t.Fatal("rejected undo changed the canvas")
	}
	if err := db.Model(mutationBefore).Update("has_submitted_task", false).Error; err != nil {
		t.Fatal(err)
	}
	result, err := s.UndoCloudAgentCanvas("user", run.ID, call.ID, afterHash, "用户撤销")
	if err != nil {
		t.Fatal(err)
	}
	if result["accepted"] != true || result["snapshotHash"] != beforeHash {
		t.Fatalf("unexpected undo result: %+v", result)
	}
	restored, _ := s.repo.CanvasProjectForUser("user", "agent-canvas")
	restoredDoc, err := creationDocument(restored.PayloadJSON)
	if err != nil || cloudAgentCanvasHash(restoredDoc) != beforeHash {
		t.Fatal("undo did not restore the semantic before snapshot")
	}
	result, err = s.UndoCloudAgentCanvas("user", run.ID, call.ID, afterHash, "重复原请求")
	if err != nil || result["accepted"] != true {
		t.Fatalf("undo should be idempotent: result=%+v err=%v", result, err)
	}
	mutation, err := s.repo.LatestCloudAgentCanvasMutation("user", run.ID)
	if err != nil || mutation.Status != "undone" {
		t.Fatalf("mutation was not marked undone: %+v %v", mutation, err)
	}
}

func TestUndoCloudAgentCanvasRejectsChangedCanvasAndSubmittedTask(t *testing.T) {
	s, db, _, _ := creationTestService(t)
	canvas := model.CanvasProject{ID: "agent-canvas", UserID: "user", PayloadJSON: `{"nodes":[]}`}
	if err := db.Create(&canvas).Error; err != nil {
		t.Fatal(err)
	}
	run, err := s.CreateCloudAgentRun("user", agentTestRequest(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CloudAgentRun("user", run.ID); err != nil {
		t.Fatal(err)
	}
	execution, err := s.repo.CloudAgent("user", run.ID)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := s.RuntimePolicy()
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := creationDocument(canvas.PayloadJSON)
	beforeHash := cloudAgentCanvasHash(doc)
	call := cloudAgentCall{ID: "canvas-call-2"}
	args, _ := json.Marshal(map[string]any{"snapshotHash": beforeHash, "ops": []map[string]any{{"type": "add_node", "id": "note-2", "nodeType": "text", "title": "节点", "content": "内容"}}})
	call.Function.Arguments = string(args)
	if err := s.repo.MutateCloudAgent("user", run.ID, execution.Revision, func(current *model.CloudAgentExecution, repo *repository.Repository) error {
		_, err := applyCloudAgentCanvas(repo, "user", "agent-canvas", call, policy, cloudAgentMutationRecorderForRun(run.ID))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	changed, _ := s.repo.CanvasProjectForUser("user", "agent-canvas")
	changedDoc, _ := creationDocument(changed.PayloadJSON)
	afterHash := cloudAgentCanvasHash(changedDoc)
	// A user edit after the Agent mutation must make the compare-and-restore fail.
	changed.PayloadJSON = `{"nodes":[{"id":"user-node","type":"text","title":"用户编辑","metadata":{"content":"后来修改"}}]}`
	if err := saveCreationCanvasWithHistory(s.repo, changed, changed.PayloadJSON); err == nil {
		t.Fatal("test setup unexpectedly saved with the new value as previous snapshot")
	}
	latest, _ := s.repo.CanvasProjectForUser("user", "agent-canvas")
	previous := latest.PayloadJSON
	latest.PayloadJSON = `{"nodes":[{"id":"user-node","type":"text","title":"用户编辑","metadata":{"content":"后来修改"}}]}`
	if err := saveCreationCanvasWithHistory(s.repo, latest, previous); err != nil {
		t.Fatal(err)
	}
	_, err = s.UndoCloudAgentCanvas("user", run.ID, call.ID, afterHash, "冲突")
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Status != 409 {
		t.Fatalf("changed canvas should return conflict, got %v", err)
	}
}
