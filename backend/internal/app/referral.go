package app

import (
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

const referralPolicySettingKey = "referral_policy"

type ReferralPolicy struct {
	Enabled                   bool  `json:"enabled"`
	GlobalRateBPS             int64 `json:"globalRateBps"`
	CreditsPerYuanMicro       int64 `json:"creditsPerYuanMicro"`
	PerInviteeCapMicrocredits int64 `json:"perInviteeCapMicrocredits"`
}

type ReferralInviteeView struct {
	UserID   string    `json:"userId"`
	Email    string    `json:"email"`
	JoinedAt time.Time `json:"joinedAt"`
}

type ReferralDashboard struct {
	Enabled          bool                   `json:"enabled"`
	Code             string                 `json:"code"`
	InviterID        string                 `json:"inviterId,omitempty"`
	EffectiveRateBPS int64                  `json:"effectiveRateBps"`
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
	return ReferralPolicy{CreditsPerYuanMicro: CreditScale}
}

func validateReferralPolicy(policy ReferralPolicy) error {
	if policy.GlobalRateBPS < 0 || policy.GlobalRateBPS > 10_000 || policy.CreditsPerYuanMicro <= 0 || policy.CreditsPerYuanMicro > 1_000*CreditScale || policy.PerInviteeCapMicrocredits < 0 || policy.PerInviteeCapMicrocredits > 1_000_000_000*CreditScale {
		return BadAuthRequest("邀请返利规则超出允许范围")
	}
	if policy.Enabled && policy.GlobalRateBPS == 0 {
		return BadAuthRequest("启用邀请返利前请设置全局返利比例")
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
	encoded, err := json.Marshal(policy)
	if err != nil {
		return ReferralPolicy{}, err
	}
	setting := model.SystemSetting{Key: referralPolicySettingKey, ValueJSON: string(encoded), UpdatedBy: actor.ID}
	if current, err := s.repo.SystemSetting(referralPolicySettingKey); err == nil {
		setting.CreatedAt = current.CreatedAt
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ReferralPolicy{}, err
	}
	if err := s.repo.SaveSystemSetting(&setting); err != nil {
		return ReferralPolicy{}, err
	}
	if err := s.appendAdminAudit(actor, "referral.policy.update", "system_setting", referralPolicySettingKey, "更新邀请返利规则", policy); err != nil {
		return ReferralPolicy{}, err
	}
	return policy, nil
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
	rate := policy.GlobalRateBPS
	if profile.RateBPS != nil {
		rate = *profile.RateBPS
	}
	items := make([]ReferralInviteeView, 0, len(invitees))
	for _, invitee := range invitees {
		items = append(items, ReferralInviteeView{UserID: invitee.ID, Email: maskReferralEmail(invitee.Email), JoinedAt: invitee.CreatedAt})
	}
	return &ReferralDashboard{Enabled: policy.Enabled, Code: profile.Code, InviterID: profile.InviterID, EffectiveRateBPS: rate, InviteeCount: inviteeCount, Invitees: items, Rewards: rewards}, nil
}

func maskReferralEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "***"
	}
	return string([]rune(parts[0])[0]) + "***@" + parts[1]
}

func (s *Service) AdminSetReferralRate(actor *model.User, userID string, rate *int64) (*model.ReferralProfile, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionReferrals); err != nil {
		return nil, err
	}
	if rate != nil && (*rate < 0 || *rate > 10_000) {
		return nil, BadAuthRequest("专属比例必须在 0% 到 100% 之间")
	}
	if _, err := s.repo.User(userID); err != nil {
		return nil, err
	}
	if err := s.repo.SetReferralRate(userID, rate); err != nil {
		return nil, err
	}
	if err := s.appendAdminAudit(actor, "referral.rate.update", "user", userID, "更新用户专属返利比例", map[string]any{"rateBps": rate}); err != nil {
		return nil, err
	}
	return s.repo.ReferralProfile(userID)
}

func (s *Service) AdminReferralUser(actor *model.User, userID string) (*model.ReferralProfile, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionReferrals); err != nil {
		return nil, err
	}
	if _, err := s.repo.User(userID); err != nil {
		return nil, err
	}
	return s.repo.ReferralProfile(userID)
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

func (s *Service) referralRewardDraft(order *model.PaymentOrder) (*model.ReferralReward, error) {
	if order == nil || order.AmountFen <= 0 || order.Currency != "CNY" {
		return nil, nil
	}
	policy, err := s.referralPolicy()
	if err != nil || !policy.Enabled {
		return nil, err
	}
	invitee, err := s.repo.ReferralProfileForInvitee(order.UserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if invitee.InviterID == "" {
		return nil, nil
	}
	inviter, err := s.repo.ReferralInviter(invitee.InviterID)
	if err != nil {
		return nil, err
	}
	rate := policy.GlobalRateBPS
	if inviter.RateBPS != nil {
		rate = *inviter.RateBPS
	}
	if rate <= 0 {
		return nil, nil
	}
	amount, err := calculateReferralReward(order.AmountFen, policy.CreditsPerYuanMicro, rate)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, nil
	}
	return &model.ReferralReward{ID: newID(), PaymentOrderID: order.ID, InviterID: invitee.InviterID, InviteeID: order.UserID, AmountFen: order.AmountFen, CreditsPerYuanMicro: policy.CreditsPerYuanMicro, RateBPS: rate, RewardMicrocredits: amount, CapMicrocredits: policy.PerInviteeCapMicrocredits, Status: model.ReferralRewardPending}, nil
}

func calculateReferralReward(amountFen, creditsPerYuanMicro, rateBPS int64) (int64, error) {
	if amountFen <= 0 || creditsPerYuanMicro <= 0 || rateBPS <= 0 {
		return 0, nil
	}
	value := new(big.Int).Mul(big.NewInt(amountFen), big.NewInt(creditsPerYuanMicro))
	value.Mul(value, big.NewInt(rateBPS))
	value.Quo(value, big.NewInt(1_000_000))
	if !value.IsInt64() {
		return 0, errors.New("邀请返利积分溢出")
	}
	return value.Int64() / 10_000 * 10_000, nil
}

func (s *Service) completePaymentOrderWithReferral(providerID, merchantOrderNo string, evidence repository.PaymentEvidence) (*model.PaymentOrder, bool, error) {
	order, err := s.repo.PaymentOrderByMerchant(providerID, merchantOrderNo)
	if err != nil {
		return nil, false, err
	}
	draft, err := s.referralRewardDraft(order)
	if err != nil {
		return nil, false, err
	}
	return s.repo.CompletePaymentOrder(providerID, merchantOrderNo, evidence, draft)
}
