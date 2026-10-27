package app

import (
	"errors"
	"testing"

	"infinite-canvas/backend/internal/model"
)

type measuredTaskBillingStub struct {
	taskTerminalBillingStub
	order         *model.BillingOrder
	orderError    error
	measuredCalls []int64
	measuredError error
}

func (b *measuredTaskBillingStub) BillingOrder(string) (*model.BillingOrder, error) {
	return b.order, b.orderError
}

func (b *measuredTaskBillingStub) SettleBillingWithAudioDuration(_, _ string, duration int64) error {
	b.measuredCalls = append(b.measuredCalls, duration)
	return b.measuredError
}

type measuredTaskOutputStub struct {
	taskTerminalOutputStub
	duration int64
	err      error
}

func (o *measuredTaskOutputStub) AudioOutputDurationMs(model.Task) (int64, error) {
	return o.duration, o.err
}

func TestAudioTaskCompletionUsesMeasuredBillingOrReview(t *testing.T) {
	failure := errors.New("measured billing unavailable")
	for _, scenario := range []struct {
		name            string
		mode            string
		missingOrder    bool
		missingOutput   bool
		outputError     error
		settlementError error
		wantMeasured    int
		wantOrdinary    int
		wantReview      bool
	}{
		{name: "per second measured", mode: "per_second", wantMeasured: 1},
		{name: "fixed audio unchanged", mode: "fixed_request", wantOrdinary: 1},
		{name: "unknown duration needs review", mode: "per_second", outputError: failure, wantReview: true},
		{name: "authorization failure needs review", mode: "per_second", settlementError: failure, wantMeasured: 1, wantReview: true},
		{name: "missing output capability needs review", mode: "per_second", missingOutput: true, wantReview: true},
		{name: "missing order needs review", mode: "per_second", missingOrder: true, wantReview: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			task := &model.Task{ID: "audio", UserID: "user", Type: "canvas_audio", BillingOrderID: "order", Status: model.TaskStatusSucceeded}
			repo := &taskTerminalRepositoryStub{task: task}
			billing := &measuredTaskBillingStub{order: &model.BillingOrder{Capability: "audio", BillingMode: scenario.mode}, measuredError: scenario.settlementError}
			if scenario.missingOrder {
				billing.order = nil
			}
			outputs := &measuredTaskOutputStub{duration: 2_500, err: scenario.outputError}
			coordinator := newTaskTerminalCoordinatorForTest(repo, &billing.taskTerminalBillingStub, &taskTerminalReplayStub{}, &taskTerminalLoggerStub{}, &outputs.taskTerminalOutputStub)
			coordinator.billing = billing
			if !scenario.missingOutput {
				coordinator.outputs = outputs
			}
			err := coordinator.handleSuccess(task)
			wantReviewCount := 0
			if scenario.wantReview {
				wantReviewCount = 1
			}
			if (err != nil) != scenario.wantReview || len(billing.uncertain) != wantReviewCount {
				t.Fatalf("completion err=%v review=%v", err, billing.uncertain)
			}
			if len(billing.measuredCalls) != scenario.wantMeasured || billing.settleCalls != scenario.wantOrdinary || len(billing.refund) != 0 {
				t.Fatalf("wrong billing path: measured=%v ordinary=%d refunds=%v", billing.measuredCalls, billing.settleCalls, billing.refund)
			}
			if len(billing.measuredCalls) > 0 && billing.measuredCalls[0] != 2_500 {
				t.Fatal("measured milliseconds changed before settlement")
			}
		})
	}
}

func TestMeasuredBillingCannotFallBackToUnmeasuredSettlement(t *testing.T) {
	repo := &taskBillingRepositoryStub{order: &model.BillingOrder{Status: model.BillingStatusRunning, Capability: "audio", BillingMode: "per_second"}}
	coordinator := newTaskBillingCoordinator(repo)
	if err := coordinator.SettleBillingWithAudioDuration("order", "provider", 1_000); err == nil {
		t.Fatal("missing measured billing implementation was accepted")
	}
	if len(repo.settleCalls) != 0 {
		t.Fatal("missing measured implementation charged the original quote")
	}
	if err := coordinator.SettleBillingWithAudioDuration("", "provider", 1_000); err != nil {
		t.Fatalf("free task needs no settlement: %v", err)
	}
}
