package app

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

type ReferralStatisticsQuery = repository.ReferralStatisticsQuery

func normalizeReferralStatisticsQuery(q ReferralStatisticsQuery) (ReferralStatisticsQuery, error) {
	q.Keyword = strings.TrimSpace(q.Keyword)
	if utf8.RuneCountInString(q.Keyword) > 200 {
		return q, BadAuthRequest("搜索关键词不能超过 200 个字符")
	}
	if q.SortBy == "" {
		q.SortBy = "paidInviteeCount"
	}
	if !repository.ValidReferralStatisticsSort(q.SortBy) {
		return q, BadAuthRequest("邀请统计排序字段无效")
	}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.PageSize == 0 {
		q.PageSize = 20
	}
	if q.Page < 1 || q.Page > 1_000_000 || q.PageSize < 1 || q.PageSize > 100 {
		return q, BadAuthRequest("分页参数无效，单页最多 100 条")
	}
	return q, nil
}

func (s *Service) AdminReferralStatistics(ctx context.Context, actor *model.User) (*repository.ReferralStatistics, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionReferrals); err != nil {
		return nil, err
	}
	return s.repo.AdminReferralStatistics(ctx)
}

func (s *Service) AdminReferralStatisticsUsers(ctx context.Context, actor *model.User, q ReferralStatisticsQuery) (*repository.ReferralUserStatisticsPage, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionReferrals); err != nil {
		return nil, err
	}
	q, err := normalizeReferralStatisticsQuery(q)
	if err != nil {
		return nil, err
	}
	return s.repo.AdminReferralStatisticsUsers(ctx, q)
}

func (s *Service) AdminReferralInvitees(ctx context.Context, actor *model.User, userID string, q ReferralStatisticsQuery) (*repository.ReferralInviteeStatisticsPage, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionReferrals); err != nil {
		return nil, err
	}
	q, err := normalizeReferralStatisticsQuery(q)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(userID) == "" {
		return nil, BadAuthRequest("请选择邀请人")
	}
	result, err := s.repo.AdminReferralInvitees(ctx, userID, q.Page, q.PageSize)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("邀请人不存在")
	}
	return result, err
}
