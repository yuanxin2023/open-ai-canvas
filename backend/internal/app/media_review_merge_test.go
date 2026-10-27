package app

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func mediaReviewServiceFixture(t *testing.T, recovery bool, duration int64) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+newID()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Task{}, &model.Resource{}, &model.BillingOrder{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	order := model.BillingOrder{ID: "review", UserID: "user", TaskID: "task", IdempotencyKey: "task-review", Capability: "audio", BillingMode: "per_second", UnitPriceMicrocredits: 10_000, MultiplierBasisPoints: 10_000, AmountMicrocredits: 50_000, ReservedAmountMicrocredits: 50_000, ChargeLimitSet: true, ChargeLimitMicrocredits: 50_000, Status: model.BillingStatusUncertain}
	reserved := int64(62_000)
	if recovery {
		order.Status = model.BillingStatusRefunded
		order.RefundedAmountMicrocredits = 50_000
		reserved = 12_000
	}
	values := []any{
		&order,
		&model.CreditAccount{UserID: order.UserID, AvailableMicrocredits: 100_000, ReservedMicrocredits: reserved},
		&model.Task{ID: "task", UserID: order.UserID, Type: "canvas_audio", BillingOrderID: order.ID, Status: model.TaskStatusSucceeded, ResultJSON: `{"resourceId":"resource:audio-output"}`},
		&model.Resource{ID: "audio-output", UserID: order.UserID, Status: model.ResourceStatusReady, Kind: "audio", DurationMs: duration},
	}
	for _, value := range values {
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	return &Service{repo: repository.New(db)}, db
}

func TestAuthorizedMediaReviewReadsResourceDurationAndPreservesRefundState(t *testing.T) {
	for _, scenario := range []struct {
		name          string
		recovery      bool
		wantAvailable int64
	}{{"held authorization", false, 100_000}, {"returned authorization", true, 50_000}} {
		t.Run(scenario.name, func(t *testing.T) {
			svc, db := mediaReviewServiceFixture(t, scenario.recovery, 7_000)
			if scenario.recovery {
				task, err := svc.repo.Task("task")
				if err != nil {
					t.Fatal(err)
				}
				if err := svc.settleRecoveredBilling(task, "provider"); !errors.Is(err, repository.ErrBillingChargeLimit) {
					t.Fatalf("automatic recovery cap: %v", err)
				}
			}
			actor := &model.User{ID: "admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull}
			result, err := svc.ResolveBillingOrder(actor, "review", ResolveBillingRequest{Action: "settle", Note: "核对已保存资源的实际时长"})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != model.BillingStatusSettled || result.ActualAmountMicrocredits != 50_000 || result.Quantity != 7 || result.RefundRecoveryPending {
				t.Fatalf("manual review: %#v", result)
			}
			var account model.CreditAccount
			if err := db.First(&account, "user_id = ?", "user").Error; err != nil {
				t.Fatal(err)
			}
			if account.AvailableMicrocredits != scenario.wantAvailable || account.ReservedMicrocredits != 12_000 {
				t.Fatalf("review balances: %#v", account)
			}
			var auditCount int64
			if err := db.Model(&model.AdminAuditEvent{}).Count(&auditCount).Error; err != nil || auditCount != 1 {
				t.Fatalf("audit count=%d err=%v", auditCount, err)
			}
		})
	}
}

func TestMediaReviewRejectsUnauthorizedOrUnverifiedWrites(t *testing.T) {
	for _, scenario := range []struct {
		name            string
		actor           model.User
		note            string
		duration        int64
		foreignResource bool
	}{
		{name: "ordinary user", actor: model.User{ID: "user", Role: model.UserRoleUser}, note: "evidence", duration: 7_000},
		{name: "scoped admin without credits", actor: model.User{ID: "scoped", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped}, note: "evidence", duration: 7_000},
		{name: "no evidence note", actor: model.User{ID: "admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull}, duration: 7_000},
		{name: "unknown duration", actor: model.User{ID: "admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull}, note: "evidence"},
		{name: "foreign resource", actor: model.User{ID: "admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull}, note: "evidence", duration: 7_000, foreignResource: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			svc, db := mediaReviewServiceFixture(t, false, scenario.duration)
			if scenario.foreignResource {
				if err := db.Model(&model.Resource{}).Where("id = ?", "audio-output").Update("user_id", "other").Error; err != nil {
					t.Fatal(err)
				}
			}
			if _, err := svc.ResolveBillingOrder(&scenario.actor, "review", ResolveBillingRequest{Action: "settle", Note: scenario.note}); err == nil {
				t.Fatal("unverified settlement succeeded")
			}
			var account model.CreditAccount
			if err := db.First(&account, "user_id = ?", "user").Error; err != nil {
				t.Fatal(err)
			}
			if account.AvailableMicrocredits != 100_000 || account.ReservedMicrocredits != 62_000 {
				t.Fatal("rejected review changed funds")
			}
			var count int64
			if err := db.Model(&model.AdminAuditEvent{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("rejected review recorded a settlement audit")
			}
		})
	}
}

func TestMediaReviewKeepsEvidenceWithinStoredLength(t *testing.T) {
	svc, _ := mediaReviewServiceFixture(t, false, 2_000)
	actor := &model.User{ID: "admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull}
	order, err := svc.ResolveBillingOrder(actor, "review", ResolveBillingRequest{Action: "settle", Note: strings.Repeat("核", 510)})
	if err != nil || len([]rune(order.ResolutionNote)) > 500 {
		t.Fatalf("evidence length: %#v %v", order, err)
	}
}

func TestUnparseableVideoInputCannotFallBackToSnapshotBilling(t *testing.T) {
	svc := &Service{}
	if _, handled, err := svc.VideoProcessingOutputDurationMs(model.Task{Type: "canvas_video", InputJSON: "{"}); !handled || err == nil {
		t.Fatal("unreadable input treated as ordinary video")
	}
}

func TestMeasuredRefundRecoveryRequiresMeasuredRepositoryCapability(t *testing.T) {
	repo := &taskBillingRepositoryStub{order: &model.BillingOrder{Status: model.BillingStatusRefunded, Capability: "audio", BillingMode: "per_second"}}
	billing := newTaskBillingCoordinator(repo)
	if err := billing.SettleBillingWithAudioDuration("order", "provider", 2_000); err == nil {
		t.Fatal("missing measured recovery implementation accepted")
	}
	if len(repo.restoreCalls) != 0 || len(repo.settleCalls) != 0 {
		t.Fatal("missing measured recovery silently charged a snapshot")
	}
}
