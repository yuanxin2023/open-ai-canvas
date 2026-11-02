package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestControlPreviewPeriods(t *testing.T) {
	for _, test := range []struct{ period, now, from string }{
		{"day", "2026-10-24T00:01:00+08:00", "2026-10-24T00:00:00+08:00"},
		{"week", "2026-10-26T00:01:00+08:00", "2026-10-26T00:00:00+08:00"},
		{"week", "2026-11-01T23:00:00+08:00", "2026-10-26T00:00:00+08:00"},
		{"month", "2026-11-01T00:01:00+08:00", "2026-11-01T00:00:00+08:00"},
	} {
		now, _ := time.Parse(time.RFC3339, test.now)
		want, _ := time.Parse(time.RFC3339, test.from)
		meta, filter, err := controlPreviewRange(ControlPreviewQuery{Period: test.period}, now)
		if err != nil || !filter.From.Equal(want) || !filter.To.Equal(now) || meta.Timezone != "Asia/Shanghai" {
			t.Fatalf("range %s %s: %+v %v", test.period, test.now, meta, err)
		}
		trend := controlPreviewTrend(meta, []repository.ControlPreviewBucket{{Bucket: want.Format("2006-01-02"), InflowMicrocredits: 100}})
		if len(trend) == 0 || !trend[0].Time.Equal(want) || !trend[len(trend)-1].Time.Before(now) {
			t.Fatalf("trend range: %+v", trend)
		}
	}
	for _, query := range []ControlPreviewQuery{{Period: "invalid"}, {Page: -1}, {PageSize: 101}, {Capability: "bad"}} {
		if _, _, err := controlPreviewRange(query, time.Now()); err == nil {
			t.Fatalf("accepted invalid query: %+v", query)
		}
	}
	meta, _, _ := controlPreviewRange(ControlPreviewQuery{}, time.Date(2026, 10, 24, 3, 10, 0, 0, controlPreviewLocation))
	trend := controlPreviewTrend(meta, []repository.ControlPreviewBucket{{Bucket: "2026-10-24 02:00", InflowMicrocredits: 123}})
	if len(trend) != 4 || trend[0].InflowMicrocredits != 0 || trend[2].InflowMicrocredits != 123 {
		t.Fatalf("missing zero buckets: %+v", trend)
	}
}

func TestControlPreviewCustomRange(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, controlPreviewLocation)
	for _, test := range []struct {
		from, to string
		buckets  int
	}{
		{"2026-10-04", "2026-11-02", 30},
		{"2026-11-01", "2026-11-01", 24},
		{"2024-01-01", "2024-12-31", 366},
	} {
		meta, filter, err := controlPreviewRange(ControlPreviewQuery{Period: "custom", From: test.from, To: test.to}, now)
		if err != nil {
			t.Fatal(err)
		}
		end, _ := time.ParseInLocation("2006-01-02", test.to, controlPreviewLocation)
		if filter.From.In(controlPreviewLocation).Format("2006-01-02 15:04") != test.from+" 00:00" || !filter.To.Equal(end.AddDate(0, 0, 1)) || !meta.AsOf.Equal(now) {
			t.Fatalf("incorrect inclusive Beijing dates: %+v", meta)
		}
		if trend := controlPreviewTrend(meta, nil); len(trend) != test.buckets || !trend[len(trend)-1].Time.Before(filter.To) {
			t.Fatalf("incorrect trend: %+v", trend)
		}
	}
	for _, query := range []ControlPreviewQuery{
		{Period: "custom"},
		{Period: "custom", From: "2026-11-01"},
		{Period: "custom", From: "2026-02-30", To: "2026-03-01"},
		{Period: "custom", From: "2026-11-02", To: "2026-11-01"},
		{Period: "custom", From: "2024-01-01", To: "2025-01-01"},
		{Period: "day", From: "2026-11-01", To: "2026-11-02"},
	} {
		if _, _, err := controlPreviewRange(query, now); err == nil {
			t.Fatalf("accepted invalid range: %+v", query)
		}
	}
}

func TestControlPreviewPermissionAndCache(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "preview.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.AutoMigrate(&model.User{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.BillingOrder{}, &model.PaymentOrder{}, &model.Task{}, &model.ApiCallLog{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{ID: "user", Username: "user", Role: model.UserRoleUser}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: "user", AvailableMicrocredits: 100}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db)}
	ctx := context.Background()
	full := &model.User{ID: "admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull}
	first, err := svc.AdminControlPreview(ctx, full, ControlPreviewQuery{})
	if err != nil || first.KPI.AvailableMicrocredits != 100 {
		t.Fatalf("overview: %+v %v", first, err)
	}
	if err := db.Model(&model.CreditAccount{}).Where("user_id = ?", "user").Update("available_microcredits", 200).Error; err != nil {
		t.Fatal(err)
	}
	second, err := svc.AdminControlPreview(ctx, full, ControlPreviewQuery{Model: "ignored", Capability: "text"})
	if err != nil || !second.AsOf.Equal(first.AsOf) || second.KPI.AvailableMicrocredits != 100 {
		t.Fatalf("cache did not preserve snapshot: %+v %v", second, err)
	}
	denied := &model.User{Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionAnalyticsOverview}}
	custom := ControlPreviewQuery{Period: "custom", From: "2026-10-01", To: "2026-10-02"}
	for index, at := range []string{"2026-09-30T23:59:59+08:00", "2026-10-01T00:00:00+08:00", "2026-10-02T23:59:59+08:00", "2026-10-03T00:00:00+08:00"} {
		created, _ := time.Parse(time.RFC3339, at)
		entry := model.CreditLedgerEntry{ID: at, UserID: "user", Type: model.CreditLedgerCheckinBonus, AmountMicrocredits: int64((index + 1) * 10), CreatedAt: created}
		if err := db.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}
	}
	short, err := svc.AdminControlPreview(ctx, full, custom)
	if err != nil {
		t.Fatal(err)
	}
	if short.KPI.InflowMicrocredits != 50 {
		t.Fatalf("inclusive date boundary: %+v", short.KPI)
	}
	custom.To = "2026-10-03"
	long, err := svc.AdminControlPreview(ctx, full, custom)
	if err != nil || long.To.Equal(short.To) || len(long.Trend) != 3 || long.KPI.AvailableMicrocredits != 200 || long.KPI.InflowMicrocredits != 90 {
		t.Fatalf("custom range cache collision: %+v %v", long, err)
	}
	if _, err := svc.AdminControlPreview(ctx, denied, ControlPreviewQuery{}); err == nil {
		t.Fatal("existing overview permission granted new financial permission")
	}
	if _, err := svc.AdminControlPreviewUsers(ctx, denied, ControlPreviewQuery{}, false); err == nil {
		t.Fatal("users bypassed permission")
	}
	if _, err := svc.AdminControlPreviewUser(ctx, denied, "user", ControlPreviewQuery{}); err == nil {
		t.Fatal("detail bypassed permission")
	}
	granted := &model.User{Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionControlPreview}}
	if _, err := svc.AdminControlPreview(ctx, granted, ControlPreviewQuery{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdminControlPreviewUser(ctx, full, "admin", ControlPreviewQuery{}); err == nil {
		t.Fatal("administrator exposed")
	} else {
		var appErr *AppError
		if !errors.As(err, &appErr) || appErr.Status != 404 {
			t.Fatalf("missing user error: %v", err)
		}
	}
	if _, err := svc.AdminControlPreviewUsers(ctx, full, ControlPreviewQuery{}, true); err == nil {
		t.Fatal("model query missing identifier accepted")
	}
	svc.controlPreviewReadCache.Clear()
	third, err := svc.AdminControlPreview(ctx, full, ControlPreviewQuery{})
	if err != nil || third.KPI.AvailableMicrocredits != 200 {
		t.Fatalf("refreshed cache: %+v %v", third, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := svc.AdminControlPreview(canceled, full, ControlPreviewQuery{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
