package app

import (
	"errors"
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/workbench/submission"
)

func TestWorkbenchSubmissionReplaysOriginalQuoteWhenFeatureIsClosed(t *testing.T) {
	s, db := newFeatureAvailabilityTestService(t)
	if err := db.AutoMigrate(&model.Task{}, &model.WorkbenchSubmission{}); err != nil {
		t.Fatal(err)
	}
	req := WorkbenchTaskInput{Kind: "image", Prompt: "original draft", Model: "original-model"}
	hash, err := submission.Fingerprint("workbench", req)
	if err != nil {
		t.Fatal(err)
	}
	key := "original-request-key"
	for _, row := range []any{
		&model.Task{ID: "original-task", UserID: "owner", Type: "canvas_image", Status: model.TaskStatusSucceeded, WorkbenchKind: "image"},
		&model.WorkbenchSubmission{UserID: "owner", RequestKey: key, RequestHash: hash, Kind: "image", TaskID: "original-task", QuoteJSON: `{"billingEnabled":true,"amountMicrocredits":10000,"billingMode":"fixed_request","priceVersion":7}`},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	req.RequestKey = key
	task, err := s.CreateWorkbenchTask("owner", req, "new-trace", "new-http-request")
	if err != nil || task.ID != "original-task" || task.WorkbenchSubmission == nil || !task.WorkbenchSubmission.Replayed || task.WorkbenchSubmission.Quote.AmountMicrocredits != 10000 || task.WorkbenchSubmission.Quote.PriceVersion != 7 {
		t.Fatalf("replay = %+v, %v", task, err)
	}
	if _, err := s.WorkbenchSubmission("other-owner", key); err == nil {
		t.Fatal("other account read receipt")
	}
	req.Prompt = "changed draft"
	_, err = s.CreateWorkbenchTask("owner", req, "", "")
	var conflict *AppError
	if !errors.As(err, &conflict) || conflict.Status != 409 {
		t.Fatalf("changed = %v", err)
	}
	req.Prompt = "original draft"
	if err := db.Delete(&model.Task{}, "id = ?", "original-task").Error; err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateWorkbenchTask("owner", req, "", "")
	if !errors.As(err, &conflict) || conflict.Status != 410 {
		t.Fatalf("deleted = %v", err)
	}
	var count int64
	if err := db.Model(&model.WorkbenchSubmission{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("receipt count = %d, %v", count, err)
	}
}

func TestWorkbenchSubmissionRejectsChangedSessionBeforeStorageOrBilling(t *testing.T) {
	s := &Service{}
	_, err := s.CreateWorkbenchTask("new-account", WorkbenchTaskInput{Kind: "image", RequestKey: "original-request-key", ExpectedUserID: "old-account"}, "", "")
	var denied *AppError
	if !errors.As(err, &denied) || denied.Status != 403 {
		t.Fatalf("managed actor mismatch = %v", err)
	}
	_, err = s.CreateTask("new-account", CreateTaskRequest{WorkbenchRequestKey: "original-request-key", WorkbenchExpectedUserID: "old-account"})
	if !errors.As(err, &denied) || denied.Status != 403 {
		t.Fatalf("custom actor mismatch = %v", err)
	}
	_, err = s.CreateTask("owner", CreateTaskRequest{WorkbenchRequestKey: "original-request-key", ProjectID: "canvas-project", Type: "canvas_image", Input: map[string]any{"mode": "image", "metadata": map[string]any{"source": "workbench", "workbenchKind": "image"}}})
	if !errors.As(err, &denied) || denied.Status != 400 {
		t.Fatalf("canvas request outside standalone workbench = %v", err)
	}
}

func TestDirectWorkbenchSubmissionReplaysWithoutRepricing(t *testing.T) {
	s, db := newFeatureAvailabilityTestService(t)
	if err := db.AutoMigrate(&model.Task{}, &model.WorkbenchSubmission{}); err != nil {
		t.Fatal(err)
	}
	req := CreateTaskRequest{Type: "canvas_image", Prompt: "original", WorkbenchExpectedUserID: "owner", Input: map[string]any{"mode": "image", "metadata": map[string]any{"source": "workbench", "workbenchKind": "image"}}}
	hash, err := submission.Fingerprint("workbench-direct", req)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{
		&model.Task{ID: "direct-task", UserID: "owner", Type: "canvas_image", Status: model.TaskStatusSucceeded, WorkbenchKind: "image"},
		&model.WorkbenchSubmission{UserID: "owner", RequestKey: "direct-request-key", RequestHash: hash, Kind: "image", TaskID: "direct-task", QuoteJSON: `{"billingEnabled":false,"amountMicrocredits":0,"billingMode":"","quantity":0}`},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	req.WorkbenchRequestKey = "direct-request-key"
	task, err := s.CreateTask("owner", req)
	if err != nil || task.ID != "direct-task" || task.WorkbenchSubmission == nil || !task.WorkbenchSubmission.Replayed || task.WorkbenchSubmission.Quote.BillingEnabled {
		t.Fatalf("direct replay = %+v, %v", task, err)
	}
}
