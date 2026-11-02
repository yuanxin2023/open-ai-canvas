package database

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/referrals"
)

func migrateReferralPolicies(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&model.SystemSetting{}, &model.ReferralPolicyVersion{}, &model.ReferralProfile{}, &model.ReferralReward{}, &model.PaymentOrder{}); err != nil {
		return err
	}
	p := referrals.Default()
	p.VersionID = "referral-policy-initial"
	var setting model.SystemSetting
	err := tx.First(&setting, "key = ?", referrals.SettingKey).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil {
		var current referrals.Policy
		if err := json.Unmarshal([]byte(setting.ValueJSON), &current); err != nil {
			return err
		}
		if current.VersionID != "" {
			if err := current.Validate(); err != nil {
				return err
			}
			var existing model.ReferralPolicyVersion
			return tx.First(&existing, "id = ?", current.VersionID).Error
		}
		var old struct {
			Enabled                   bool  `json:"enabled"`
			GlobalRateBPS             int64 `json:"globalRateBps"`
			PerInviteeCapMicrocredits int64 `json:"perInviteeCapMicrocredits"`
		}
		if err := json.Unmarshal([]byte(setting.ValueJSON), &old); err != nil {
			return err
		}
		p.Enabled, p.PerInviteeCapMicrocredits = old.Enabled, old.PerInviteeCapMicrocredits
		p.Stages = []referrals.Stage{{RateBPS: old.GlobalRateBPS}}
	}
	if err := p.Validate(); err != nil {
		return err
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		return err
	}
	// A disabled global switch pauses existing relationships; it must not pin
	// them to a permanently disabled benefit snapshot when the switch resumes.
	bound := p
	bound.Enabled = true
	boundJSON, err := json.Marshal(bound)
	if err != nil {
		return err
	}
	if err := tx.Create(&model.ReferralPolicyVersion{ID: p.VersionID, PolicyJSON: string(boundJSON), CreatedBy: "migration"}).Error; err != nil {
		return err
	}
	setting.Key, setting.ValueJSON = referrals.SettingKey, string(encoded)
	if err := tx.Save(&setting).Error; err != nil {
		return err
	}
	return tx.Model(&model.ReferralProfile{}).Where("inviter_id <> ''").Updates(map[string]any{"bound_policy_version_id": p.VersionID, "rate_bps": nil, "policy_version_id": ""}).Error
}
