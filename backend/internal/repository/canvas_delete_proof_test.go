package repository

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/model"
)

func TestCanvasDeletionPreservesEditProofAndRejectsOtherSessions(t *testing.T) {
	db, err := database.Open(database.Config{Driver: "sqlite", DSN: "file:canvas-delete-proof?mode=memory&cache=shared&_foreign_keys=on"})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	project := model.CanvasProject{ID: "canvas", UserID: "owner", PayloadJSON: `{"nodes":[{"id":"text","type":"text","title":"Keep text"}]}`, Revision: 1}
	if err := db.Create(&project).Error; err != nil {
		t.Fatal(err)
	}
	lease := model.CanvasEditLease{UserID: "owner", CanvasID: "canvas", NodeID: "text", SessionID: "tab", Token: "held-proof", ExpiresAt: time.Now().Add(time.Minute)}
	if err := db.Create(&lease).Error; err != nil {
		t.Fatal(err)
	}
	var notifications int
	repo := New(db).WithCanvasChangeNotifier(func(string, string) { notifications++ })
	if err := repo.DeleteCanvasProject("owner", "canvas"); !errors.Is(err, ErrCanvasEditLease) {
		t.Fatalf("unproven delete accepted: %v", err)
	}
	if _, err := repo.CanvasProjectForUser("owner", "canvas"); err != nil || notifications != 0 {
		t.Fatalf("rejected delete changed canvas: %v %d", err, notifications)
	}
	proof := model.CanvasEditProof{NodeID: "text", SessionID: "tab", Token: "held-proof"}
	if err := repo.WithCanvasEditProofs([]model.CanvasEditProof{proof}).DeleteCanvasProject("owner", "canvas"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CanvasProjectForUser("owner", "canvas"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("proved delete did not remove canvas: %v", err)
	}
	if notifications != 1 {
		t.Fatalf("delete notifications = %d", notifications)
	}
}
