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
