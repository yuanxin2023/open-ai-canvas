package model

import "time"

type ReferralProfile struct {
	UserID               string    `json:"userId" gorm:"primaryKey;size:36"`
	Code                 string    `json:"code" gorm:"size:16;uniqueIndex"`
	InviterID            string    `json:"inviterId,omitempty" gorm:"size:36;index"`
	RateBPS              *int64    `json:"rateBps,omitempty"`
	PolicyVersionID      string    `json:"policyVersionId,omitempty" gorm:"size:36"`
	BoundPolicyVersionID string    `json:"boundPolicyVersionId,omitempty" gorm:"size:36;index"`
	Paused               bool      `json:"paused" gorm:"not null;default:false"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type ReferralRewardStatus string

const (
	ReferralRewardPending  ReferralRewardStatus = "pending"
	ReferralRewardApproved ReferralRewardStatus = "approved"
	ReferralRewardRejected ReferralRewardStatus = "rejected"
)

type ReferralReward struct {
	ID                   string               `json:"id" gorm:"primaryKey;size:36"`
	PaymentOrderID       string               `json:"paymentOrderId" gorm:"size:36;uniqueIndex"`
	InviterID            string               `json:"inviterId" gorm:"size:36;index"`
	InviteeID            string               `json:"inviteeId" gorm:"size:36;index"`
	AmountFen            int64                `json:"amountFen"`
	PolicyVersionID      string               `json:"policyVersionId,omitempty" gorm:"size:36;index"`
	BaseMicrocredits     int64                `json:"baseMicrocredits"`
	OrderOrdinal         int64                `json:"orderOrdinal"`
	StageIndex           int                  `json:"stageIndex"`
	UncappedMicrocredits int64                `json:"uncappedMicrocredits"`
	CreditsPerYuanMicro  int64                `json:"creditsPerYuanMicro"`
	RateBPS              int64                `json:"rateBps"`
	RewardMicrocredits   int64                `json:"rewardMicrocredits"`
	Status               ReferralRewardStatus `json:"status" gorm:"size:16;index"`
	ReviewedBy           string               `json:"reviewedBy,omitempty" gorm:"size:36"`
	ReviewNote           string               `json:"reviewNote,omitempty" gorm:"size:500"`
	ReviewedAt           *time.Time           `json:"reviewedAt,omitempty"`
	CreatedAt            time.Time            `json:"createdAt" gorm:"index"`
	UpdatedAt            time.Time            `json:"updatedAt"`
}

type ReferralPolicyVersion struct {
	ID         string    `json:"id" gorm:"primaryKey;size:36"`
	PolicyJSON string    `json:"-" gorm:"type:text;not null"`
	CreatedBy  string    `json:"createdBy" gorm:"size:36"`
	CreatedAt  time.Time `json:"createdAt"`
}
