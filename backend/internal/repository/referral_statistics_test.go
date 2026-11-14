package repository

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestReferralStatisticsSQLite(t *testing.T) {
	db := openPaymentTestDB(t)
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	testReferralStatistics(t, db)
}

func TestReferralStatisticsPostgres(t *testing.T) {
	dsn := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("CANVAS_TEST_POSTGRES_DSN is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("referral_statistics_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE"); sqlDB, _ := admin.DB(); sqlDB.Close() })
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	testReferralStatistics(t, db)
}

func testReferralStatistics(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&model.User{}, &model.ReferralProfile{}, &model.PaymentOrder{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repo := New(db)
	empty, err := repo.AdminReferralStatistics(ctx)
	if err != nil || *empty != (ReferralStatistics{}) {
		t.Fatalf("empty statistics: %+v %v", empty, err)
	}
	joined := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ids := []string{"a", "b", "c", "a1", "a2", "a3", "b1", "b2", "c1", "a11", "empty"}
	for i, id := range ids {
		user := model.User{ID: id, Username: id, Status: model.UserStatusActive, CreatedAt: joined}
		if id == "b" {
			user.Username = "B_100%"
		}
		if id == "a2" {
			user.Status = model.UserStatusDisabled
		}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		if id == "empty" {
			continue
		}
		inviter := map[string]string{"a1": "a", "a2": "a", "a3": "a", "b1": "b", "b2": "b", "c1": "c", "a11": "a1"}[id]
		profile := model.ReferralProfile{UserID: id, Code: fmt.Sprintf("CODE%02d", i), InviterID: inviter, Paused: id == "a"}
		if err := db.Create(&profile).Error; err != nil {
			t.Fatal(err)
		}
	}
	addOrder := func(id, user string, amount int64, status model.PaymentOrderStatus, currency string, kind model.TopupProductKind) {
		t.Helper()
		order := model.PaymentOrder{ID: id, UserID: user, IdempotencyKey: id, MerchantOrderNo: id, AmountFen: amount, Status: status, Currency: currency, ProductKind: kind}
		if err := db.Create(&order).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i, user := range []string{"a1", "a1", "a2", "b1", "b2", "a11"} {
		addOrder(fmt.Sprintf("paid-%d", i), user, []int64{100, 200, 400, 900, 100, 500}[i], model.PaymentOrderCredited, "CNY", model.TopupProductKindCredits)
	}
	addOrder("own", "a", 90000, model.PaymentOrderCredited, "CNY", model.TopupProductKindCredits)
	addOrder("pending", "a3", 9999, model.PaymentOrderPending, "CNY", model.TopupProductKindCredits)
	addOrder("closed", "a3", 9999, model.PaymentOrderClosed, "CNY", model.TopupProductKindCredits)
	addOrder("failed", "a3", 9999, model.PaymentOrderCreateFailed, "CNY", model.TopupProductKindCredits)
	addOrder("zero", "a3", 0, model.PaymentOrderCredited, "CNY", model.TopupProductKindCredits)
	addOrder("usd", "a3", 9999, model.PaymentOrderCredited, "USD", model.TopupProductKindCredits)
	addOrder("storage", "a3", 9999, model.PaymentOrderCredited, "CNY", model.TopupProductKindStorage)
	stats, err := repo.AdminReferralStatistics(ctx)
	want := ReferralStatistics{InviterCount: 4, ReferralMetrics: ReferralMetrics{InviteeCount: 7, PaidInviteeCount: 5, PaymentOrderCount: 6, PaymentAmountFen: 2200}}
	if err != nil || *stats != want {
		t.Fatalf("statistics: %+v want %+v err %v", stats, want, err)
	}
	for sort, wantIDs := range map[string][]string{
		"inviteeCount": {"a", "b", "a1", "c"}, "paidInviteeCount": {"a", "b", "a1", "c"},
		"paymentOrderCount": {"a", "b", "a1", "c"}, "paymentAmountFen": {"b", "a", "a1", "c"},
	} {
		page, err := repo.AdminReferralStatisticsUsers(ctx, ReferralStatisticsQuery{SortBy: sort, Page: 1, PageSize: 20})
		if err != nil || page.Total != 4 || len(page.Users) != 4 {
			t.Fatalf("ranking %s: %+v %v", sort, page, err)
		}
		for i, id := range wantIDs {
			if page.Users[i].UserID != id || page.Users[i].Rank != int64(i+1) {
				t.Fatalf("ranking %s: %+v", sort, page.Users)
			}
		}
	}
	for _, keyword := range []string{"b_100%", "_", "%", "code01"} {
		page, err := repo.AdminReferralStatisticsUsers(ctx, ReferralStatisticsQuery{Keyword: keyword, SortBy: "paidInviteeCount", Page: 1, PageSize: 1})
		if err != nil || page.Total != 1 || len(page.Users) != 1 || page.Users[0].UserID != "b" || page.Users[0].Rank != 2 {
			t.Fatalf("search %s lost literal matching/global rank: %+v %v", keyword, page, err)
		}
	}
	for _, keyword := range []string{"no-match", `\`, "' OR 1=1 --"} {
		page, err := repo.AdminReferralStatisticsUsers(ctx, ReferralStatisticsQuery{Keyword: keyword, SortBy: "paidInviteeCount", Page: 1, PageSize: 20})
		if err != nil || page.Total != 0 || len(page.Users) != 0 || page.Users == nil {
			t.Fatalf("empty search: %+v %v", page, err)
		}
	}
	page, err := repo.AdminReferralStatisticsUsers(ctx, ReferralStatisticsQuery{SortBy: "paidInviteeCount", Page: 2, PageSize: 1})
	if err != nil || len(page.Users) != 1 || page.Users[0].UserID != "b" || page.Users[0].Rank != 2 {
		t.Fatalf("page 2: %+v %v", page, err)
	}
	detail, err := repo.AdminReferralInvitees(ctx, "a", 1, 20)
	if err != nil || detail.Total != 3 || len(detail.Invitees) != 3 || detail.Inviter.ReferralMetrics != (ReferralMetrics{3, 2, 3, 700}) || detail.Invitees[2].PaymentOrderCount != 0 {
		t.Fatalf("direct invitee detail: %+v %v", detail, err)
	}
	if !detail.Invitees[0].JoinedAt.Equal(joined) || detail.Invitees[0].UserID != "a1" {
		t.Fatalf("invitee timestamps/order: %+v", detail.Invitees)
	}
	zero, err := repo.AdminReferralInvitees(ctx, "empty", 1, 20)
	if err != nil || zero.Total != 0 || zero.Inviter.UserID != "empty" || zero.Invitees == nil {
		t.Fatalf("zero inviter: %+v %v", zero, err)
	}
	var profileCount int64
	if err := db.Model(&model.ReferralProfile{}).Where("user_id = ?", "empty").Count(&profileCount).Error; err != nil || profileCount != 0 {
		t.Fatal("statistics created referral profile")
	}
	if _, err := repo.AdminReferralInvitees(ctx, "missing", 1, 20); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing inviter: %v", err)
	}
	for i := 0; i < 120; i++ {
		id := fmt.Sprintf("extra-%03d", i)
		if err := db.Create(&model.User{ID: id, Username: id, CreatedAt: joined}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.ReferralProfile{UserID: id, Code: id, InviterID: "a"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for page := 1; page <= 7; page++ {
		detail, err := repo.AdminReferralInvitees(ctx, "a", page, 20)
		if err != nil || detail.Total != 123 {
			t.Fatalf("large invitee page: %+v %v", detail, err)
		}
		for _, invitee := range detail.Invitees {
			if seen[invitee.UserID] {
				t.Fatalf("duplicate page entry: %s", invitee.UserID)
			}
			seen[invitee.UserID] = true
		}
	}
	if len(seen) != 123 {
		t.Fatalf("truncated invitees: %d", len(seen))
	}
	if err := db.Delete(&model.User{}, "id IN ?", []string{"b", "a2"}).Error; err != nil {
		t.Fatal(err)
	}
	stats, err = repo.AdminReferralStatistics(ctx)
	if err != nil || stats.InviterCount != 3 || stats.InviteeCount != 124 || stats.PaidInviteeCount != 2 || stats.PaymentAmountFen != 800 {
		t.Fatalf("removed users/orphan relationships: %+v %v", stats, err)
	}
}
