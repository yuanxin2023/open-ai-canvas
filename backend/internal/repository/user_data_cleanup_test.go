package repository

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
)

func testUserDataCleanupFeed(t *testing.T, db *gorm.DB) {
	t.Helper()
	repo := New(db)
	rollback := errors.New("rollback cleanup")
	if err := repo.Transaction(func(tx *Repository) error {
		if err := tx.RecordUserDataCleanup("cleanup-owner", []model.UserDataCleanupEntry{{Kind: "asset", EntityID: "rolled-back", Action: "delete"}}); err != nil {
			return err
		}
		return rollback
	}); !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	page, err := repo.UserDataCleanupChanges("cleanup-owner", 0)
	if err != nil || len(page.Entries) != 0 {
		t.Fatalf("rollback page=%#v err=%v", page, err)
	}
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results <- repo.Transaction(func(tx *Repository) error {
				return tx.RecordUserDataCleanup("cleanup-owner", []model.UserDataCleanupEntry{{Kind: "asset", EntityID: fmt.Sprintf("concurrent-%d", index), Action: "delete"}})
			})
		}(index)
	}
	wait.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	page, err = repo.UserDataCleanupChanges("cleanup-owner", 0)
	if err != nil || len(page.Entries) != 2 || page.Cursor != 2 || page.Entries[0].Revision != 1 || page.Entries[1].Revision != 2 {
		t.Fatalf("concurrent page=%#v err=%v", page, err)
	}
	for index := 2; index < 102; index++ {
		if err := repo.Transaction(func(tx *Repository) error {
			return tx.RecordUserDataCleanup("cleanup-owner", []model.UserDataCleanupEntry{{Kind: "asset", EntityID: fmt.Sprintf("entry-%d", index), Action: "delete"}})
		}); err != nil {
			t.Fatal(err)
		}
	}
	page, err = repo.UserDataCleanupChanges("cleanup-owner", 0)
	if err != nil || len(page.Entries) != 100 || !page.HasMore || page.Cursor != 100 {
		t.Fatalf("first page=%#v err=%v", page, err)
	}
	last, err := repo.UserDataCleanupChanges("cleanup-owner", page.Cursor)
	if err != nil || len(last.Entries) != 2 || last.HasMore || last.Cursor != 102 {
		t.Fatalf("last=%#v err=%v", last, err)
	}
	other, err := repo.UserDataCleanupChanges("other-cleanup-owner", 0)
	if err != nil || len(other.Entries) != 0 {
		t.Fatalf("other=%#v err=%v", other, err)
	}
}
