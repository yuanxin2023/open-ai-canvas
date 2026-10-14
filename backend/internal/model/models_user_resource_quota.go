package model

import "time"

type UserResourceQuota struct {
	UserID        string `gorm:"primaryKey;size:36"`
	OverridesJSON string `gorm:"type:text;not null"`
	UpdatedBy     string `gorm:"size:36;not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (UserResourceQuota) TableName() string { return "user_resource_quotas" }
