package model

import "time"

// DefaultUserGroupCode 是未分配分组用户的归属；该分组不能删除。
const DefaultUserGroupCode = "default"

// 用户端分组徽章外观；none 表示不展示徽章。
const (
	UserGroupBadgeNone = "none"
	UserGroupBadgeVIP  = "vip"
	UserGroupBadgeSVIP = "svip"
)

// UserGroup 保存展示信息和可见渠道范围，不包含计价配置。
type UserGroup struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	Code        string    `json:"code" gorm:"size:40;uniqueIndex"`
	Name        string    `json:"name" gorm:"size:80"`
	Description string    `json:"description" gorm:"size:500;not null;default:''"`
	BadgeLabel  string    `json:"badgeLabel" gorm:"size:16;not null;default:''"`
	BadgeStyle  string    `json:"badgeStyle" gorm:"size:16;not null;default:none"`
	SortOrder   int       `json:"sortOrder" gorm:"not null;default:0"`
	AllChannels bool      `json:"allChannels" gorm:"not null;default:false"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// UserGroupChannel 绑定分组可使用的系统渠道；AllChannels 为 true 时不读取本表。
type UserGroupChannel struct {
	GroupID   string    `json:"groupId" gorm:"primaryKey;size:36"`
	ChannelID string    `json:"channelId" gorm:"primaryKey;size:36;index"`
	CreatedAt time.Time `json:"createdAt"`
}
