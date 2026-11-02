package app

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/referrals"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

const referralPolicySettingKey = "referral_policy"

type ReferralPolicy = referrals.Policy

type ReferralInviteeView struct {
	UserID   string         `json:"userId"`
	Email    string         `json:"email"`
	JoinedAt time.Time      `json:"joinedAt"`
	Policy   ReferralPolicy `json:"policy"`
}

type ReferralDashboard struct {
	Enabled          bool                   `json:"enabled"`
	Code             string                 `json:"code"`
	InviterID        string                 `json:"inviterId,omitempty"`
	Policy           ReferralPolicy         `json:"policy"`
	BoundPolicy      *ReferralPolicy        `json:"boundPolicy,omitempty"`
	NewcomerEligible bool                   `json:"newcomerEligible"`
	InviteeCount     int64                  `json:"inviteeCount"`
	Invitees         []ReferralInviteeView  `json:"invitees"`
	Rewards          []model.ReferralReward `json:"rewards"`
}

type ReferralRewardPage struct {
	Rewards  []model.ReferralReward `json:"rewards"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
}

func defaultReferralPolicy() ReferralPolicy {
	return referrals.Default()
}

func validateReferralPolicy(policy ReferralPolicy) error {
	if err := policy.Validate(); err != nil {
		return BadAuthRequest(err.Error())
	}
	return nil
}

func (s *Service) referralPolicy() (ReferralPolicy, error) {
	setting, err := s.repo.SystemSetting(referralPolicySettingKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultReferralPolicy(), nil
	}
	if err != nil {
		return ReferralPolicy{}, err
	}
	var policy ReferralPolicy
	if err := json.Unmarshal([]byte(setting.ValueJSON), &policy); err != nil {
		return ReferralPolicy{}, errors.New("邀请返利规则配置格式无效")
	}
	if err := validateReferralPolicy(policy); err != nil {
		return ReferralPolicy{}, err
	}
	return policy, nil
}

func (s *Service) AdminReferralPolicy(actor *model.User) (ReferralPolicy, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionReferrals); err != nil {
		return ReferralPolicy{}, err
	}
	return s.referralPolicy()
}

func (s *Service) UpdateReferralPolicy(actor *model.User, policy ReferralPolicy) (ReferralPolicy, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionReferrals); err != nil {
		return ReferralPolicy{}, err
	}
	if err := validateReferralPolicy(policy); err != nil {
		return ReferralPolicy{}, err
	}
	audit, err := newAdminAuditEvent(actor, "referral.policy.update", "system_setting", referralPolicySettingKey, "发布邀请返利规则版本", policy)
	if err != nil {
		return ReferralPolicy{}, err
	}
	return s.repo.SaveReferralPolicy(policy, "", nil, actor.ID, audit)
}

func (s *Service) ReferralDashboard(user *model.User) (*ReferralDashboard, error) {
	if user == nil {
		return nil, Unauthorized("请先登录")
	}
	policy, err := s.referralPolicy()
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.ReferralProfile(user.ID)
	if err != nil {
		return nil, err
	}
	invitees, err := s.repo.ReferralInvitees(user.ID)
	if err != nil {
		return nil, err
	}
	inviteeCount, err := s.repo.ReferralInviteeCount(user.ID)
	if err != nil {
		return nil, err
	}
	rewards, _, err := s.repo.ReferralRewards(user.ID, "all", 100, 0)
	if err != nil {
		return nil, err
	}
	enabled := policy.Enabled && !profile.Paused
	if profile.PolicyVersionID != "" {
		policy, err = s.repo.ReferralPolicyVersion(profile.PolicyVersionID)
		if err != nil {
			return nil, err
		}
	}
	enabled = enabled && policy.Enabled
	items := make([]ReferralInviteeView, 0, len(invitees))
	for _, invitee := range invitees {
		var boundPolicy ReferralPolicy
		if err := json.Unmarshal([]byte(invitee.PolicyJSON), &boundPolicy); err != nil {
			return nil, err
		}
		if err := validateReferralPolicy(boundPolicy); err != nil {
			return nil, err
		}
		items = append(items, ReferralInviteeView{UserID: invitee.ID, Email: maskReferralEmail(invitee.Email), JoinedAt: invitee.CreatedAt, Policy: boundPolicy})
	}
	result := &ReferralDashboard{Enabled: enabled, Code: profile.Code, InviterID: profile.InviterID, Policy: policy, InviteeCount: inviteeCount, Invitees: items, Rewards: rewards}
	if profile.InviterID != "" {
		bound, err := s.repo.ReferralPolicyVersion(profile.BoundPolicyVersionID)
		if err != nil {
			return nil, err
		}
		result.BoundPolicy = &bound
		if bound.Enabled && bound.NewcomerEnabled {
			result.NewcomerEligible, err = s.repo.ReferralNewcomerEligible(user.ID, profile.InviterID)
			if err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func maskReferralEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "***"
	}
	return string([]rune(parts[0])[0]) + "***@" + parts[1]
}

type ReferralUserPolicy struct {
	UserID string          `json:"userId"`
	Code   string          `json:"code"`
	Paused bool            `json:"paused"`
	Policy *ReferralPolicy `json:"policy"`
}

func (s *Service) AdminSetReferralUserPolicy(actor *model.User, userID string, policy *ReferralPolicy, paused bool) (*ReferralUserPolicy, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionReferrals); err != nil {
		return nil, err
	}
	if policy != nil {
		if err := validateReferralPolicy(*policy); err != nil {
			return nil, err
		}
	}
	if _, err := s.repo.User(userID); err != nil {
		return nil, err
	}
	audit, err := newAdminAuditEvent(actor, "referral.policy.user.update", "user", userID, "更新专属推广规则", map[string]any{"policy": policy, "paused": paused})
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveReferralUserPolicy(userID, policy, paused, actor.ID, audit); err != nil {
		return nil, err
	}
	return s.AdminReferralUser(actor, userID)
}

func (s *Service) AdminReferralUser(actor *model.User, userID string) (*ReferralUserPolicy, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionReferrals); err != nil {
		return nil, err
	}
	if _, err := s.repo.User(userID); err != nil {
		return nil, err
	}
	profile, err := s.repo.ReferralProfile(userID)
	if err != nil {
		return nil, err
	}
	result := &ReferralUserPolicy{UserID: userID, Code: profile.Code, Paused: profile.Paused}
	if profile.PolicyVersionID != "" {
		policy, err := s.repo.ReferralPolicyVersion(profile.PolicyVersionID)
		if err != nil {
			return nil, err
		}
		result.Policy = &policy
	}
	return result, nil
}

func (s *Service) AdminReferralRewards(actor *model.User, status string, page, pageSize int) (*ReferralRewardPage, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionReferrals); err != nil {
		return nil, err
	}
	if status != "" && status != "all" && status != string(model.ReferralRewardPending) && status != string(model.ReferralRewardApproved) && status != string(model.ReferralRewardRejected) {
		return nil, BadAuthRequest("返利状态无效")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 30
	}
	rewards, total, err := s.repo.ReferralRewards("", status, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	return &ReferralRewardPage{Rewards: rewards, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Service) AdminReviewReferralReward(actor *model.User, id string, approve bool, note string) (*model.ReferralReward, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionReferrals); err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, BadAuthRequest("返利记录不存在")
	}
	note = truncateRunes(strings.TrimSpace(note), 500)
	if !approve && note == "" {
		return nil, BadAuthRequest("拒绝返利时请填写原因")
	}
	action := "referral.reward.reject"
	if approve {
		action = "referral.reward.approve"
	}
	audit, err := newAdminAuditEvent(actor, action, "referral_reward", id, "审核邀请返利", map[string]any{"approved": approve, "note": note})
	if err != nil {
		return nil, err
	}
	reward, err := s.repo.ReviewReferralReward(id, actor.ID, approve, note, audit)
	if errors.Is(err, repository.ErrReferralReviewConflict) {
		return nil, BadAuthRequest("这笔返利已审核")
	}
	return reward, err
}

func (s *Service) completePaymentOrderWithReferral(providerID, merchantOrderNo string, evidence repository.PaymentEvidence) (*model.PaymentOrder, bool, error) {
	return s.repo.CompletePaymentOrderWithReferral(providerID, merchantOrderNo, evidence, referrals.Evaluate)
}
