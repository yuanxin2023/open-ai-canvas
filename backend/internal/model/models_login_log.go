package model

import "time"

// LoginLog 保存事件时的用户快照，不随账号改名或会话清理改写历史。
type LoginLog struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	UserID         *string   `json:"userId" gorm:"index;size:36"`
	Username       string    `json:"username" gorm:"size:80"`
	DisplayName    string    `json:"displayName" gorm:"size:80"`
	Account        string    `json:"account" gorm:"size:160"`
	Event          string    `json:"event" gorm:"index;size:16"`
	Method         string    `json:"method" gorm:"index;size:24"`
	Source         string    `json:"source" gorm:"size:24"`
	Result         string    `json:"result" gorm:"index;size:16"`
	IP             string    `json:"ip" gorm:"index;size:64"`
	UserAgent      string    `json:"userAgent" gorm:"size:1024"`
	DeviceType     string    `json:"deviceType" gorm:"size:32"`
	Browser        string    `json:"browser" gorm:"size:80"`
	BrowserVersion string    `json:"browserVersion" gorm:"size:40"`
	OS             string    `json:"os" gorm:"size:80"`
	OSVersion      string    `json:"osVersion" gorm:"size:40"`
	SessionRef     string    `json:"sessionRef" gorm:"index;size:64"`
	Reason         string    `json:"reason" gorm:"size:64"`
	Message        string    `json:"message" gorm:"size:500"`
	CreatedAt      time.Time `json:"createdAt" gorm:"index"`
}
