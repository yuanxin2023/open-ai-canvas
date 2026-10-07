package model

import "time"

type ReferralProfile struct {
	UserID    string    `json:"userId" gorm:"primaryKey;size:36"`
	Code      string    `json:"code" gorm:"size:16;uniqueIndex"`
	InviterID string    `json:"inviterId,omitempty" gorm:"size:36;index"`
	RateBPS   *int64    `json:"rateBps,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ReferralRewardStatus string

const (
	ReferralRewardPending  ReferralRewardStatus = "pending"
	ReferralRewardApproved ReferralRewardStatus = "approved"
	ReferralRewardRejected ReferralRewardStatus = "rejected"
)

type ReferralReward struct {
	ID                  string               `json:"id" gorm:"primaryKey;size:36"`
	PaymentOrderID      string               `json:"paymentOrderId" gorm:"size:36;uniqueIndex"`
	InviterID           string               `json:"inviterId" gorm:"size:36;index"`
	InviteeID           string               `json:"inviteeId" gorm:"size:36;index"`
	AmountFen           int64                `json:"amountFen"`
	CreditsPerYuanMicro int64                `json:"creditsPerYuanMicro"`
	RateBPS             int64                `json:"rateBps"`
	RewardMicrocredits  int64                `json:"rewardMicrocredits"`
	CapMicrocredits     int64                `json:"-" gorm:"-"`
	Status              ReferralRewardStatus `json:"status" gorm:"size:16;index"`
	ReviewedBy          string               `json:"reviewedBy,omitempty" gorm:"size:36"`
	ReviewNote          string               `json:"reviewNote,omitempty" gorm:"size:500"`
	ReviewedAt          *time.Time           `json:"reviewedAt,omitempty"`
	CreatedAt           time.Time            `json:"createdAt" gorm:"index"`
	UpdatedAt           time.Time            `json:"updatedAt"`
}
