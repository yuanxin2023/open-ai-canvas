package repository

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func workbenchSubmissionTestRepo(t *testing.T) *Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.WorkbenchSubmission{}, &model.Task{}, &model.BillingOrder{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: "user", AvailableMicrocredits: 1000}).Error; err != nil {
		t.Fatal(err)
	}
	return New(db)
}

func commitWorkbenchTestTask(repo *Repository, id, hash string, fail bool) (*model.Task, bool, error) {
	record := &model.WorkbenchSubmission{UserID: "user", RequestKey: "same-request-key", RequestHash: hash, Kind: "image", TaskID: id, QuoteJSON: `{"amountMicrocredits":100}`}
	return repo.CommitWorkbenchSubmission(record, func(tx *Repository) error {
		order := &model.BillingOrder{ID: "order-" + id, UserID: "user", TaskID: id, IdempotencyKey: "task:" + id, AmountMicrocredits: 100, BillingMode: "fixed_request"}
		if err := reserveBillingOrder(tx.db, order); err != nil {
			return err
		}
		if err := tx.db.Create(&model.Task{ID: id, UserID: "user", Type: "canvas_image", Status: model.TaskStatusQueued, BillingOrderID: order.ID}).Error; err != nil {
			return err
		}
		if fail {
			return errors.New("injected after reservation and task insertion")
		}
		return nil
	})
}

func TestWorkbenchSubmissionReplayKeepsOriginalTaskAndSingleReservation(t *testing.T) {
	repo := workbenchSubmissionTestRepo(t)
	first, replayed, err := commitWorkbenchTestTask(repo, "task-a", "original", false)
	if err != nil || replayed {
		t.Fatalf("first = %v, %v", replayed, err)
	}
	second, replayed, err := commitWorkbenchTestTask(repo, "task-b", "original", false)
	if err != nil || !replayed || second.ID != first.ID {
		t.Fatalf("replay = %+v, %v, %v", second, replayed, err)
	}
	if _, _, err := commitWorkbenchTestTask(repo, "task-c", "changed", false); !errors.Is(err, ErrWorkbenchSubmissionConflict) {
		t.Fatalf("changed input = %v", err)
	}
	assertWorkbenchReservationCounts(t, repo, 1, 900, 100)
	if _, err := repo.WorkbenchSubmission("other-user", "same-request-key"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("other account = %v", err)
	}
}

func TestWorkbenchSubmissionRollsBackReceiptTaskAndWalletTogether(t *testing.T) {
	repo := workbenchSubmissionTestRepo(t)
	if _, _, err := commitWorkbenchTestTask(repo, "rolled-back", "original", true); err == nil {
		t.Fatal("expected injected failure")
	}
	assertWorkbenchReservationCounts(t, repo, 0, 1000, 0)
	if _, err := repo.WorkbenchSubmission("user", "same-request-key"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("receipt survived rollback: %v", err)
	}
	if _, replayed, err := commitWorkbenchTestTask(repo, "committed", "original", false); err != nil || replayed {
		t.Fatalf("retry = %v, %v", replayed, err)
	}
	assertWorkbenchReservationCounts(t, repo, 1, 900, 100)
}

func TestConcurrentWorkbenchSubmissionsReserveOnlyOnce(t *testing.T) {
	repo := workbenchSubmissionTestRepo(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			task, _, err := commitWorkbenchTestTask(repo, fmt.Sprintf("task-%d", i), "same-hash", false)
			if err != nil || task == nil {
				t.Errorf("concurrent submission = %v", err)
			}
		}(i)
	}
	wg.Wait()
	assertWorkbenchReservationCounts(t, repo, 1, 900, 100)
}

func TestRemovedTaskDoesNotAllowRequestKeyToCreateAgain(t *testing.T) {
	repo := workbenchSubmissionTestRepo(t)
	if _, _, err := commitWorkbenchTestTask(repo, "removed", "original", false); err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Delete(&model.Task{}, "id = ?", "removed").Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := commitWorkbenchTestTask(repo, "replacement", "original", false); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted replay = %v", err)
	}
	var count int64
	repo.db.Model(&model.Task{}).Count(&count)
	if count != 0 {
		t.Fatal("deleted task was recreated")
	}
	repo.db.Model(&model.BillingOrder{}).Count(&count)
	if count != 1 {
		t.Fatal("deleted task was charged again")
	}
}

func assertWorkbenchReservationCounts(t *testing.T, repo *Repository, count, available, reserved int64) {
	t.Helper()
	for _, table := range []any{&model.Task{}, &model.BillingOrder{}, &model.CreditLedgerEntry{}, &model.WorkbenchSubmission{}} {
		var actual int64
		if err := repo.db.Model(table).Count(&actual).Error; err != nil || actual != count {
			t.Fatalf("%T count = %d, %v", table, actual, err)
		}
	}
	var account model.CreditAccount
	if err := repo.db.First(&account, "user_id = ?", "user").Error; err != nil || account.AvailableMicrocredits != available || account.ReservedMicrocredits != reserved {
		t.Fatalf("wallet = %+v, %v", account, err)
	}
}
