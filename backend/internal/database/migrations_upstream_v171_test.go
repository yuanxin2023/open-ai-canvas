package database

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestDramaSegmentMigrationArchivesMediaWithoutResubmittingJobs(t *testing.T) {
	db := isolatedMigrationDB(t)
	if err := db.AutoMigrate(&model.DramaProduction{}, &model.DramaEpisode{}, &model.DramaShot{}, &model.DramaCast{}, &model.DramaTake{}, &model.DramaJob{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE drama_shots ADD COLUMN \"frame_resource_id\" varchar(36)").Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{
		&model.DramaProduction{ProjectID: "project", UserID: "owner", Status: "running", SettingsJSON: `{"storyboard":{"segmentSeconds":10}}`},
		&model.DramaEpisode{ID: "episode", ProjectID: "project", Stage: "video", MediaReleased: true},
		&model.DramaShot{ID: "shot", ProjectID: "project", EpisodeID: "episode"},
		&model.DramaJob{ID: "job", ProjectID: "project", Kind: "video", Status: "running", TaskID: "paid-task"},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Table("drama_shots").Where("id = ?", "shot").Update("frame_resource_id", "retained-image").Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.Transaction(migrateDramaSegmentProduction); err != nil {
			t.Fatal(err)
		}
		if err := db.Transaction(migrateDramaSegmentRevision); err != nil {
			t.Fatal(err)
		}
		if err := db.Transaction(migrateDramaSegmentMode); err != nil {
			t.Fatal(err)
		}
	}
	var take model.DramaTake
	if err := db.First(&take, "owner_id = ?", "shot").Error; err != nil || take.ResourceID != "retained-image" {
		t.Fatalf("lost historical media: %+v %v", take, err)
	}
	var count int64
	if err := db.Model(&model.DramaTake{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("duplicated migration media: %d %v", count, err)
	}
	var job model.DramaJob
	if err := db.First(&job, "id = ?", "job").Error; err != nil || job.Status != "canceled" || job.TaskID != "paid-task" {
		t.Fatalf("paid job was resubmitted: %+v %v", job, err)
	}
	var episode model.DramaEpisode
	if err := db.First(&episode, "id = ?", "episode").Error; err != nil || episode.Stage != "script" || episode.MediaReleased {
		t.Fatalf("legacy episode still running: %+v %v", episode, err)
	}
	if db.Migrator().HasColumn(&model.DramaShot{}, "frame_resource_id") {
		t.Fatal("legacy shot column survived")
	}
}
