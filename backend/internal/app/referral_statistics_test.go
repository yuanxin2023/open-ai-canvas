package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestReferralStatisticsPermissionsAndValidation(t *testing.T) {
	s, _, admin := quotaTestService(t, "sqlite")
	ctx := context.Background()
	for _, actor := range []*model.User{nil, {ID: "normal", Role: model.UserRoleUser}, {ID: "scoped", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionUsers}}} {
		if _, err := s.AdminReferralStatistics(ctx, actor); err == nil {
			t.Fatal("unauthorized overview accepted")
		}
		if _, err := s.AdminReferralStatisticsUsers(ctx, actor, ReferralStatisticsQuery{}); err == nil {
			t.Fatal("unauthorized ranking accepted")
		}
		if _, err := s.AdminReferralInvitees(ctx, actor, admin.ID, ReferralStatisticsQuery{}); err == nil {
			t.Fatal("unauthorized invitees accepted")
		}
	}
	for _, q := range []ReferralStatisticsQuery{{SortBy: "username DESC; DROP TABLE users"}, {Page: -1}, {PageSize: 101}, {Page: 1_000_001}, {Keyword: strings.Repeat("序", 201)}} {
		if _, err := s.AdminReferralStatisticsUsers(ctx, admin, q); err == nil {
			t.Fatalf("invalid query accepted: %+v", q)
		}
	}
	if _, err := s.AdminReferralInvitees(ctx, admin, "", ReferralStatisticsQuery{}); err == nil {
		t.Fatal("empty inviter accepted")
	}
	_, err := s.AdminReferralInvitees(ctx, admin, "missing-user", ReferralStatisticsQuery{})
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Status != 404 {
		t.Fatalf("missing user should return 404: %v", err)
	}
	scoped := &model.User{ID: admin.ID, Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionReferrals}}
	if _, err := s.AdminReferralStatistics(ctx, scoped); err != nil {
		t.Fatalf("referrals-only admin overview denied: %v", err)
	}
	if _, err := s.AdminReferralStatisticsUsers(ctx, scoped, ReferralStatisticsQuery{}); err != nil {
		t.Fatalf("referrals-only admin ranking denied: %v", err)
	}
	if _, err := s.AdminReferralInvitees(ctx, scoped, admin.ID, ReferralStatisticsQuery{}); err != nil {
		t.Fatalf("referrals-only admin invitees denied: %v", err)
	}
}

func TestReferralStatisticsQueryDefaults(t *testing.T) {
	q, err := normalizeReferralStatisticsQuery(ReferralStatisticsQuery{Keyword: "  " + strings.Repeat("序", 200) + "  "})
	if err != nil || q.Page != 1 || q.PageSize != 20 || q.SortBy != "paidInviteeCount" || q.Keyword != strings.Repeat("序", 200) {
		t.Fatalf("query defaults/unicode keyword: %+v %v", q, err)
	}
}
