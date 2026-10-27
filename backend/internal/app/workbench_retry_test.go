package app

import (
	"errors"
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func workbenchRetryTestService(t *testing.T) (*Service, *model.Task) {
	t.Helper()
	s, db := newFeatureAvailabilityTestService(t)
	if err := db.AutoMigrate(&model.Task{}, &model.TaskTextDelta{}, &model.WorkbenchSubmission{}, &model.BillingOrder{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	for _, row := range []any{
		&model.SystemSetting{Key: featureAvailabilitySettingKey, ValueJSON: `{"imageWorkbenchEnabled":true,"videoWorkbenchEnabled":true}`},
		&model.CreditAccount{UserID: "owner", AvailableMicrocredits: 1000},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	source := &model.Task{ID: "history-source", UserID: "owner", Type: "canvas_image", WorkbenchKind: "image", Status: model.TaskStatusFailed, Model: "original-model", InputJSON: `{"mode":"image","metadata":{"source":"workbench","workbenchKind":"image","workbench":{"kind":"image","parameters":{"size":"16:9","quality":"high","imageResolution":"2k","count":2},"references":[]}}}`}
	if err := db.Create(source).Error; err != nil {
		t.Fatal(err)
	}
	return s, source
}

func TestWorkbenchRetryIntentIsReadOnlyAndOwnerScoped(t *testing.T) {
	s, source := workbenchRetryTestService(t)
	before := *source
	intent, err := s.WorkbenchRetryIntent("owner", source.ID)
	if err != nil || intent.Task.ID != source.ID || intent.RetryOf != source.ID || len(intent.RetryVersion) != 64 {
		t.Fatalf("intent = %+v, %v", intent, err)
	}
	if _, err := s.WorkbenchRetryIntent("other-owner", source.ID); err == nil {
		t.Fatal("other owner received retry parameters")
	}
	after, err := s.repo.TaskForUser("owner", source.ID)
	if err != nil || after.Status != before.Status || after.BillingOrderID != before.BillingOrderID || !after.UpdatedAt.Equal(before.UpdatedAt) || after.InputJSON != before.InputJSON {
		t.Fatalf("read mutated source: %+v, %v", after, err)
	}
	assertWorkbenchRetryWallet(t, s, 1000, 0)
}

func TestWorkbenchRetryEligibilityProtectsExistingRestrictions(t *testing.T) {
	base := model.Task{ID: "source", Status: model.TaskStatusFailed}
	creationID := "creation-submission"
	for _, tc := range []struct {
		name   string
		change func(*model.Task)
		order  *model.BillingOrder
	}{
		{"media recovery", func(task *model.Task) { task.MediaRecoveryJSON = "checkpoint" }, nil},
		{"creation approval", func(task *model.Task) { task.CreationSubmissionID = &creationID }, nil},
		{"agent run", func(task *model.Task) { task.AgentRunID = "run" }, nil},
		{"running task", func(task *model.Task) { task.Status = model.TaskStatusRunning }, nil},
		{"pending cancel", func(task *model.Task) { task.ProviderCancelStatus = model.ProviderCancelStatusRequested }, nil},
		{"uncertain fee", func(task *model.Task) { task.BillingOrderID = "old-order" }, &model.BillingOrder{Status: model.BillingStatusUncertain}},
		{"refund review", func(task *model.Task) { task.BillingOrderID = "old-order" }, &model.BillingOrder{Status: model.BillingStatusRefunded, RefundRecoveryPending: true}},
		{"moderation", func(task *model.Task) { task.Error = "provider: " + contentModerationErrorCode }, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := base
			tc.change(&task)
			billing := newTaskBillingCoordinator(&taskBillingRepositoryStub{order: tc.order})
			if err := validateTaskRetryEligibility(&task, billing); err == nil {
				t.Fatal("restricted retry was allowed")
			}
		})
	}
}

func TestWorkbenchRetrySourceChangeRollsBackNewTaskOrderAndReceipt(t *testing.T) {
	s, source := workbenchRetryTestService(t)
	intent, err := s.WorkbenchRetryIntent("owner", source.ID)
	if err != nil {
		t.Fatal(err)
	}
	ref := workbenchRetryReference{TaskID: source.ID, Version: intent.RetryVersion}
	for _, changed := range []bool{true, false} {
		record := &model.WorkbenchSubmission{UserID: "owner", RequestKey: "manual-retry-request-key", RequestHash: "request-digest", Kind: "image", TaskID: "new-task", QuoteJSON: `{"billingEnabled":true,"amountMicrocredits":100,"billingMode":"fixed_request","quantity":1}`}
		created, _, err := s.repo.CommitWorkbenchSubmission(record, func(tx *repository.Repository) error {
			return saveWorkbenchRetry(tx, "owner", "image", ref, func() error {
				fresh := &model.Task{ID: "new-task", UserID: "owner", Type: "canvas_image", WorkbenchKind: "image", Status: model.TaskStatusQueued, BillingOrderID: "new-order"}
				order := &model.BillingOrder{ID: "new-order", UserID: "owner", TaskID: fresh.ID, IdempotencyKey: "new-order-key", UnitPriceMicrocredits: 100, MultiplierBasisPoints: 10000, Quantity: 1, AmountMicrocredits: 100, BillingMode: "fixed_request", Status: model.BillingStatusReserved}
				if err := tx.CreateTaskWithCreditReservation(fresh, order, 100); err != nil {
					return err
				}
				if changed {
					_, err := tx.RetryTaskWithBilling("owner", source, nil, 100)
					return err
				}
				return nil
			})
		})
		if changed {
			var conflict *AppError
			if !errors.As(err, &conflict) || conflict.Status != 409 {
				t.Fatalf("changed source = %v", err)
			}
			assertWorkbenchRetryWallet(t, s, 1000, 0)
			if _, err := s.repo.WorkbenchSubmission("owner", record.RequestKey); err == nil {
				t.Fatal("rejected request consumed its key")
			}
		} else {
			if err != nil || created.ID != "new-task" || created.ID == source.ID || created.BillingOrderID != "new-order" {
				t.Fatalf("manual creation = %+v, %v", created, err)
			}
			assertWorkbenchRetryWallet(t, s, 900, 100)
		}
		original, err := s.repo.TaskForUser("owner", source.ID)
		if err != nil || original.Status != model.TaskStatusFailed || original.BillingOrderID != source.BillingOrderID || !original.UpdatedAt.Equal(source.UpdatedAt) {
			t.Fatalf("original task changed: %+v, %v", original, err)
		}
	}
}

func assertWorkbenchRetryWallet(t *testing.T, s *Service, available, reserved int64) {
	t.Helper()
	account, err := s.repo.CreditAccount("owner")
	if err != nil || account.AvailableMicrocredits != available || account.ReservedMicrocredits != reserved {
		t.Fatalf("wallet = %+v, %v", account, err)
	}
}

func TestWorkbenchRetryModelSelectorDoesNotInferSystemChannelFromName(t *testing.T) {
	for _, tc := range []struct{ name, model, logical, input, want string }{
		{"custom selector", "owned::vendor::image", "", `{"config":{"model":"vendor::image","apiKey":"private-key"}}`, "owned::vendor::image"},
		{"custom raw model", "vendor::image", "", `{"config":{"model":"vendor::image"}}`, ""},
		{"unscoped alias", "public-alias", "", `{"config":{"model":"provider-model"}}`, ""},
		{"system namespaced model", "channel::image", "", `{"config":{"channelId":"system-channel","model":"channel::image"}}`, ""},
		{"managed model", "public-model", "logical", `{"config":{"channelId":"private-route","model":"supplier-model"}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := workbenchRetryModelSelector(&model.Task{Model: tc.model, LogicalModelID: tc.logical, InputJSON: tc.input}); got != tc.want {
				t.Fatalf("selector = %q, want %q", got, tc.want)
			}
		})
	}
}
