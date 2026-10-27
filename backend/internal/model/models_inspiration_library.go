package model

import "time"

// UserInspirationLibrary stores directory preferences and links to UserPrompt,
// rather than another copy of personal prompt bodies.
type UserInspirationLibrary struct {
	UserID    string    `json:"-" gorm:"primaryKey;size:36"`
	Revision  int64     `json:"revision" gorm:"not null;default:0"`
	StateJSON string    `json:"-" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
