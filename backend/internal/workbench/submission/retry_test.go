package submission

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func TestRetryVersionBindsSourceAttempt(t *testing.T) {
	source := model.Task{ID: "source", UserID: "owner", Type: "canvas_image", WorkbenchKind: "image", Status: model.TaskStatusFailed, BillingOrderID: "original-order", InputJSON: `{"prompt":"original"}`, UpdatedAt: time.Unix(100, 123)}
	original, err := RetryVersion(&source)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*model.Task){
		func(task *model.Task) { task.UserID = "other" },
		func(task *model.Task) { task.Status = model.TaskStatusRunning },
		func(task *model.Task) { task.BillingOrderID = "new-order" },
		func(task *model.Task) { task.InputJSON = `{"prompt":"changed"}` },
		func(task *model.Task) { task.Prompt = "changed top-level prompt" },
		func(task *model.Task) { task.Model = "different-model" },
		func(task *model.Task) { task.LogicalModelID = "different-public-model" },
		func(task *model.Task) { task.MediaRecoveryJSON = "private-checkpoint" },
		func(task *model.Task) { task.ProviderCancelStatus = model.ProviderCancelStatusRequested },
		func(task *model.Task) { task.UpdatedAt = task.UpdatedAt.Add(time.Nanosecond) },
	} {
		changed := source
		mutate(&changed)
		version, err := RetryVersion(&changed)
		if err != nil || version == original {
			t.Fatalf("source change did not invalidate version: %v", err)
		}
	}
	unchanged, _ := RetryVersion(&source)
	if original != unchanged || len(original) != 64 {
		t.Fatal("source version is not stable")
	}
}
