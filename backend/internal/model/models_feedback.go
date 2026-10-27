package model

import "time"

type FeedbackType string

const (
	FeedbackTypeBug         FeedbackType = "bug"
	FeedbackTypeFeature     FeedbackType = "feature"
	FeedbackTypeImprovement FeedbackType = "improvement"
	FeedbackTypeOther       FeedbackType = "other"
)

type FeedbackStatus string

const (
	FeedbackStatusPending    FeedbackStatus = "pending"    // 待处理
	FeedbackStatusProcessing FeedbackStatus = "processing" // 处理中/评估中
	FeedbackStatusResolved   FeedbackStatus = "resolved"   // 已解决/已采纳
	FeedbackStatusClosed     FeedbackStatus = "closed"     // 已关闭
	FeedbackStatusArchived   FeedbackStatus = "archived"   // 已归档
)

type FeedbackAttachment struct {
	ID         string `json:"id"`
	ResourceID string `json:"resourceId"`
	Name       string `json:"name"`
	Type       string `json:"type"` // "image" | "video"
	URL        string `json:"url"`
	Size       int64  `json:"size"`
	MimeType   string `json:"mimeType"`
}

type UserFeedback struct {
	ID          string         `gorm:"primaryKey;size:64" json:"id"`
	Version     int64          `gorm:"not null;default:1" json:"version"`
	UserID      string         `gorm:"index;size:64" json:"userId"`
	UserName    string         `gorm:"size:128" json:"userName"`
	UserEmail   string         `gorm:"size:128" json:"userEmail"`
	Contact     string         `gorm:"size:256" json:"contact"`
	Type        FeedbackType   `gorm:"size:32;index" json:"type"`
	Title       string         `gorm:"size:256" json:"title"`
	Content     string         `gorm:"type:text" json:"content"`
	Attachments string         `gorm:"type:text" json:"attachments"` // JSON array of FeedbackAttachment
	Environment string         `gorm:"type:text" json:"environment"` // JSON object of browser/device info
	Status      FeedbackStatus `gorm:"size:32;index;default:'pending'" json:"status"`
	AdminReply  string         `gorm:"type:text" json:"adminReply"`
	RepliedAt   *time.Time     `json:"repliedAt,omitempty"`
	Archived    bool           `gorm:"index;default:false" json:"archived"`
	ArchivedAt  *time.Time     `json:"archivedAt,omitempty"`
	CreatedAt   time.Time      `gorm:"index" json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// Foreign keys keep a concurrent resource deletion from breaking a committed
// feedback attachment. Ordinary deletion checks still report the reference.
type FeedbackResourceReference struct {
	FeedbackID string        `gorm:"primaryKey;size:64"`
	ResourceID string        `gorm:"primaryKey;size:36;index"`
	UserID     string        `gorm:"size:36;index;not null"`
	Resource   *Resource     `gorm:"foreignKey:ResourceID;references:ID;constraint:OnDelete:RESTRICT"`
	Feedback   *UserFeedback `gorm:"foreignKey:FeedbackID;references:ID;constraint:OnDelete:CASCADE"`
}
