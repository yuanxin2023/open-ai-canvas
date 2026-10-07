package model

import "time"

type User struct {
	ID                   string            `json:"id" gorm:"primaryKey;size:36"`
	Username             string            `json:"username" gorm:"uniqueIndex;size:80"`
	Email                string            `json:"email,omitempty" gorm:"size:160"`
	DisplayName          string            `json:"displayName" gorm:"size:80"`
	ProfileName          string            `json:"profileName" gorm:"size:80"`
	UsernameCustomizedAt *time.Time        `json:"-" gorm:"index"`
	AvatarResourceID     string            `json:"-" gorm:"index;size:36"`
	Role                 UserRole          `json:"role" gorm:"index;size:24"`
	AdminLevel           AdminLevel        `json:"-" gorm:"index;size:24"`
	AdminPermissions     []AdminPermission `json:"-" gorm:"-"`
	Status               UserStatus        `json:"status" gorm:"index;size:24"`
	PasswordHash         string            `json:"-"`
	AdminRemark          string            `json:"-" gorm:"size:500"`
	RegistrationIP       string            `json:"-" gorm:"size:64"`
	LastLoginAt          *time.Time        `json:"lastLoginAt"`
	CreatedAt            time.Time         `json:"createdAt"`
	UpdatedAt            time.Time         `json:"updatedAt"`
}

type AdminPermissionGrant struct {
	UserID          string          `json:"userId" gorm:"primaryKey;size:36"`
	Permission      AdminPermission `json:"permission" gorm:"primaryKey;size:80"`
	GrantedByUserID string          `json:"grantedByUserId" gorm:"index;size:36"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// UserUsernameChange 记录每次成功的语义改名；旧用户名不会被保留占用。
type UserUsernameChange struct {
	ID                string    `json:"id" gorm:"primaryKey;size:36"`
	UserID            string    `json:"-" gorm:"index:idx_user_username_changes_user_created,priority:1;index;size:36"`
	OldUsername       string    `json:"oldUsername" gorm:"size:80"`
	NewUsername       string    `json:"newUsername" gorm:"size:80"`
	CountsTowardLimit bool      `json:"countsTowardLimit" gorm:"index"`
	CreatedAt         time.Time `json:"createdAt" gorm:"index:idx_user_username_changes_user_created,priority:2,sort:desc"`
}

// UserLoginEvent 保留成功登录时的排障环境；它独立于可撤销的 AuthSession，退出后仍可审计。
type UserLoginEvent struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	UserID         string    `json:"-" gorm:"index:idx_user_login_events_user_created,priority:1;index;size:36"`
	SessionID      string    `json:"-" gorm:"index;size:36"`
	LoginMethod    string    `json:"loginMethod" gorm:"index;size:32"`
	IPAddress      string    `json:"ipAddress" gorm:"index;size:64"`
	UserAgent      string    `json:"userAgent" gorm:"size:1024"`
	DeviceType     string    `json:"deviceType" gorm:"size:32"`
	Browser        string    `json:"browser" gorm:"size:80"`
	BrowserVersion string    `json:"browserVersion" gorm:"size:40"`
	OS             string    `json:"os" gorm:"size:80"`
	OSVersion      string    `json:"osVersion" gorm:"size:40"`
	CreatedAt      time.Time `json:"createdAt" gorm:"index:idx_user_login_events_user_created,priority:2,sort:desc"`
}

type AuthSession struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	UserID    string    `json:"userId" gorm:"index;size:36"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expiresAt" gorm:"index"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type UserIdentity struct {
	ID               string    `json:"id" gorm:"primaryKey;size:36"`
	UserID           string    `json:"userId" gorm:"index;size:36"`
	Provider         string    `json:"provider" gorm:"size:32;uniqueIndex:idx_user_identity_provider_subject,priority:1"`
	Subject          string    `json:"subject" gorm:"size:160;uniqueIndex:idx_user_identity_provider_subject,priority:2"`
	ProviderUsername string    `json:"providerUsername" gorm:"size:160"`
	AvatarURL        string    `json:"avatarUrl"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type OAuthState struct {
	ID           string     `json:"id" gorm:"primaryKey;size:36"`
	Provider     string     `json:"provider" gorm:"index;size:32"`
	StateHash    string     `json:"-" gorm:"uniqueIndex;size:64"`
	CodeVerifier string     `json:"-" gorm:"size:160"`
	NextPath     string     `json:"nextPath"`
	ExpiresAt    time.Time  `json:"expiresAt" gorm:"index"`
	UsedAt       *time.Time `json:"usedAt" gorm:"index"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type EmailVerificationCode struct {
	ID        string     `json:"id" gorm:"primaryKey;size:36"`
	Email     string     `json:"email" gorm:"index;size:160"`
	CodeHash  string     `json:"-" gorm:"size:64"`
	Purpose   string     `json:"purpose" gorm:"index;size:32"`
	ExpiresAt time.Time  `json:"expiresAt" gorm:"index"`
	UsedAt    *time.Time `json:"usedAt" gorm:"index"`
	CreatedAt time.Time  `json:"createdAt" gorm:"index"`
}
