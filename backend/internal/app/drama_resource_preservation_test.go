package app

import (
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestDramaMediaReferencesBlockDeletionWithoutGenerationHistory(t *testing.T) {
	for _, source := range []struct {
		name string
		row  any
	}{
		{"cast", &model.DramaCast{ID: "cast", ProjectID: "drama", ImageResourceID: "retained"}},
		{"final", &model.DramaEpisode{ID: "episode", ProjectID: "drama", FinalResourceID: "retained"}},
		{"video", &model.DramaSegment{ID: "video", ProjectID: "drama", VideoResourceID: "retained"}},
		{"tail", &model.DramaSegment{ID: "tail", ProjectID: "drama", TailFrameResourceID: "retained"}},
		{"history", &model.DramaTake{ID: "take", ProjectID: "drama", OwnerID: "old-segment", ResourceID: "retained"}},
	} {
		t.Run(source.name, func(t *testing.T) {
			s, db, _ := newResourceDeletionTestService(t)
			for _, row := range []any{&model.DramaProduction{ProjectID: "drama", UserID: "user-1"}, &model.Resource{ID: "retained", UserID: "user-1", Status: model.ResourceStatusReady}, &model.Asset{ID: "asset", UserID: "user-1", PayloadJSON: `{"data":{"storageKey":"resource:retained"}}`}, source.row} {
				if err := db.Create(row).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := s.DeleteUserAsset("user-1", "asset"); err == nil || !strings.Contains(err.Error(), "短剧") {
				t.Fatalf("drama media was not retained: %v", err)
			}
			var resource model.Resource
			if err := db.First(&resource, "id = ?", "retained").Error; err != nil {
				t.Fatal("referenced resource record was deleted", err)
			}
		})
	}
}
