package model

import "time"

// A per-user counter serializes cleanup commits so a cursor cannot skip a late commit.
type UserDataCleanupState struct {
	UserID   string `gorm:"primaryKey;size:36"`
	Revision int64
}

// Entries are durable tombstones as well as incremental client invalidations.
type UserDataCleanupEntry struct {
	UserID    string    `json:"-" gorm:"primaryKey;size:36;index:idx_cleanup_cursor,priority:1"`
	Kind      string    `json:"kind" gorm:"primaryKey;size:20"`
	EntityID  string    `json:"id" gorm:"primaryKey;size:320"`
	Revision  int64     `json:"revision" gorm:"index:idx_cleanup_cursor,priority:2"`
	Action    string    `json:"action" gorm:"size:16"`
	CanvasID  string    `json:"canvasId,omitempty" gorm:"size:80"`
	CreatedAt time.Time `json:"createdAt"`
}
