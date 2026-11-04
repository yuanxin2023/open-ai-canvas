package app

import (
	"errors"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestClosedWorkbenchRejectsBothCreationPathsAndCatalog(t *testing.T) {
	s, _ := newFeatureAvailabilityTestService(t)
	for _, kind := range []string{model.WorkbenchKindImage, model.WorkbenchKindVideo} {
		t.Run(kind, func(t *testing.T) {
			_, err := s.CreateWorkbenchTask("user", WorkbenchTaskInput{Kind: kind, Prompt: "test"}, "", "")
			assertClosedWorkbench(t, err)
			_, err = s.CreateTask("user", CreateTaskRequest{Type: "canvas_" + kind, Prompt: "test", Input: map[string]any{
				"mode": kind, "metadata": map[string]any{"source": "workbench", "workbenchKind": kind},
			}})
			assertClosedWorkbench(t, err)
			_, err = s.PublicWorkbenchTools(kind)
			assertClosedWorkbench(t, err)
		})
	}
}

func TestClosedWorkbenchKeepsOwnedHistoryReadable(t *testing.T) {
	s, db, _, _ := ecommerceCreationTestService(t)
	saveAdmissionFeatures(t, s, defaultFeatureAvailability())
	for _, kind := range []string{WorkbenchKindImage, WorkbenchKindVideo} {
		for _, userID := range []string{"user", "other"} {
			if err := db.Create(&model.Task{ID: userID + "-" + kind, UserID: userID, Type: "canvas_" + kind, WorkbenchKind: kind, Status: model.TaskStatusSucceeded, InputJSON: `{}`}).Error; err != nil {
				t.Fatal(err)
			}
		}
		items, err := s.WorkbenchTasks("user", kind, false, 10)
		if err != nil || len(items) != 1 || items[0].ID != "user-"+kind {
			t.Fatalf("closed %s history ownership: %+v, %v", kind, items, err)
		}
	}
}

func assertClosedWorkbench(t *testing.T, err error) {
	t.Helper()
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Status != 403 {
		t.Fatalf("closed workbench error = %v, want forbidden", err)
	}
}

func TestClosedWorkbenchAlsoRejectsNewAttemptsFromTaskCenter(t *testing.T) {
	s, db := newFeatureAvailabilityTestService(t)
	if err := db.AutoMigrate(&model.Task{}); err != nil {
		t.Fatal(err)
	}
	for _, task := range []model.Task{
		{ID: "image", UserID: "user", Type: "canvas_image", WorkbenchKind: "image", Status: model.TaskStatusFailed},
		{ID: "video", UserID: "user", Type: "canvas_video", WorkbenchKind: "video", Status: model.TaskStatusFailed},
		{ID: "legacy-image", UserID: "user", Type: "canvas_image", Status: model.TaskStatusFailed, InputJSON: `{"mode":"image","metadata":{"source":"workbench","workbenchKind":"image"}}`},
	} {
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
		_, err := s.RetryTask("user", task.ID)
		assertClosedWorkbench(t, err)
	}
}
