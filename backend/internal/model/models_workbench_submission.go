package model

import "time"

// This receipt survives task deletion so an old key can never create a new charge.
type WorkbenchSubmission struct {
	UserID      string `gorm:"primaryKey;size:36"`
	RequestKey  string `gorm:"primaryKey;size:96"`
	RequestHash string `gorm:"not null;size:64"`
	Kind        string `gorm:"not null;size:16"`
	TaskID      string `gorm:"not null;size:36;index"`
	QuoteJSON   string `gorm:"not null;type:text"`
	CreatedAt   time.Time
}

type WorkbenchSubmissionQuote struct {
	BillingEnabled          bool   `json:"billingEnabled"`
	AmountMicrocredits      int64  `json:"amountMicrocredits"`
	BillingMode             string `json:"billingMode"`
	PriceTierVersion        int64  `json:"priceTierVersion"`
	PriceVersion            int64  `json:"priceVersion"`
	Quantity                int64  `json:"quantity"`
	ChargeLimitMicrocredits *int64 `json:"chargeLimitMicrocredits,omitempty"`
}

type WorkbenchSubmissionReceipt struct {
	RequestKey string                   `json:"requestKey"`
	Replayed   bool                     `json:"replayed"`
	Quote      WorkbenchSubmissionQuote `json:"quote"`
}
