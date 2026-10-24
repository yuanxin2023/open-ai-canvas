package repository

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestControlPreviewAggregates(t *testing.T) {
	t.Run("sqlite", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "preview.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal(err)
		}
		conn, _ := db.DB()
		t.Cleanup(func() { _ = conn.Close() })
		testControlPreviewAggregates(t, db)
	})
	t.Run("postgres", func(t *testing.T) {
		dsn := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("set CANVAS_TEST_POSTGRES_DSN to an isolated test database")
		}
		admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal(err)
		}
		schema := fmt.Sprintf("control_preview_%d", time.Now().UnixNano())
		if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
			conn, _ := admin.DB()
			_ = conn.Close()
		})
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		q := parsed.Query()
		q.Set("search_path", schema)
		parsed.RawQuery = q.Encode()
		db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal(err)
		}
		conn, _ := db.DB()
		t.Cleanup(func() { _ = conn.Close() })
		testControlPreviewAggregates(t, db)
	})
}

func testControlPreviewAggregates(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&model.User{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.BillingOrder{}, &model.PaymentOrder{}, &model.Task{}, &model.ApiCallLog{}); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 10, 23, 16, 0, 0, 0, time.UTC)
	beijing := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := from.Add(15 * time.Hour)
	create := func(value any) {
		t.Helper()
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	create(&[]model.User{{ID: "u1", Username: "alpha", Role: model.UserRoleUser}, {ID: "u2", Username: "beta", Role: model.UserRoleUser, Status: "disabled"}, {ID: "admin", Username: "admin", Role: model.UserRoleAdmin}})
	create(&[]model.CreditAccount{{UserID: "u1", AvailableMicrocredits: 1000, ReservedMicrocredits: 100}, {UserID: "u2", AvailableMicrocredits: 2000}, {UserID: "admin", AvailableMicrocredits: 999999}})
	create(&[]model.BillingOrder{
		{ID: "b1", UserID: "u1", IdempotencyKey: "b1", Model: "model-a", Capability: "text", Status: model.BillingStatusSettled},
		{ID: "b2", UserID: "u2", IdempotencyKey: "b2", Model: "model-a", Capability: "text", Status: model.BillingStatusSettled},
		{ID: "old", UserID: "u1", IdempotencyKey: "old", Model: "model-a", Capability: "text", Status: model.BillingStatusSettled},
		{ID: "ba", UserID: "admin", IdempotencyKey: "ba", Model: "model-a", Capability: "text", Status: model.BillingStatusSettled},
	})
	entries := []model.CreditLedgerEntry{
		{ID: "c1", UserID: "u1", Type: model.CreditLedgerConsume, AmountMicrocredits: -100, BillingOrderID: "b1"},
		{ID: "c2", UserID: "u2", Type: model.CreditLedgerConsume, AmountMicrocredits: -100, BillingOrderID: "b2"},
		{ID: "c-admin", UserID: "admin", Type: model.CreditLedgerConsume, AmountMicrocredits: -9000, BillingOrderID: "ba"},
		{ID: "code-funding", UserID: "u1", Type: model.CreditLedgerConsume, AmountMicrocredits: -600, Scene: "redeem_code"},
		{ID: "redeem", UserID: "u1", Type: model.CreditLedgerRedeem, AmountMicrocredits: 600},
		{ID: "payment", UserID: "u1", Type: model.CreditLedgerPaymentTopup, AmountMicrocredits: 700},
		{ID: "grant", UserID: "u1", Type: model.CreditLedgerAdminGrant, AmountMicrocredits: 300},
		{ID: "deduct", UserID: "u1", Type: model.CreditLedgerAdminAdjust, AmountMicrocredits: -200},
		{ID: "signup", UserID: "u2", Type: model.CreditLedgerSignupBonus, AmountMicrocredits: 10},
		{ID: "checkin", UserID: "u2", Type: model.CreditLedgerCheckinBonus, AmountMicrocredits: 20},
		{ID: "referral", UserID: "u2", Type: model.CreditLedgerReferral, AmountMicrocredits: 30},
		{ID: "refund", UserID: "u1", Type: model.CreditLedgerRefund, AmountMicrocredits: 500},
		{ID: "reserve", UserID: "u1", Type: model.CreditLedgerReserve, AmountMicrocredits: -500},
		{ID: "bad-owner", UserID: "u2", Type: model.CreditLedgerConsume, AmountMicrocredits: -9999, BillingOrderID: "b1"},
	}
	for index := range entries {
		entries[index].CreatedAt = from.Add(time.Hour)
		if index%2 == 0 {
			entries[index].CreatedAt = entries[index].CreatedAt.In(beijing)
		}
	}
	create(&entries)
	create(&model.CreditLedgerEntry{ID: "yesterday", UserID: "u1", Type: model.CreditLedgerConsume, AmountMicrocredits: -50, BillingOrderID: "old", CreatedAt: from.Add(-time.Minute).In(beijing)})
	create(&model.CreditLedgerEntry{ID: "prior-day-bonus", UserID: "u1", Type: model.CreditLedgerCheckinBonus, AmountMicrocredits: 9000, CreatedAt: from.Add(-time.Minute).In(beijing)})
	create(&model.CreditLedgerEntry{ID: "future-bonus", UserID: "u1", Type: model.CreditLedgerCheckinBonus, AmountMicrocredits: 9000, CreatedAt: now.Add(time.Minute).In(beijing)})
	paidAt := from.Add(time.Hour).In(beijing)
	create(&[]model.PaymentOrder{
		{ID: "p1", UserID: "u1", IdempotencyKey: "p1", MerchantOrderNo: "p1", Status: model.PaymentOrderCredited, Currency: "CNY", AmountFen: 1200, CreditedAt: &paidAt, CreatedAt: from.Add(-24 * time.Hour)},
		{ID: "p2", UserID: "u1", IdempotencyKey: "p2", MerchantOrderNo: "p2", Status: model.PaymentOrderPending, Currency: "CNY", AmountFen: 9999, ProviderPaidAt: &paidAt},
	})
	create(&[]model.Task{{ID: "running", UserID: "u1", Status: model.TaskStatusRunning}, {ID: "failed", UserID: "u2", Status: model.TaskStatusFailed}, {ID: "done", UserID: "u1", Status: model.TaskStatusSucceeded}})
	logs := []model.ApiCallLog{
		{ID: "retry-failed", UserID: "u1", BillingOrderID: "b1", Model: "fallback-old", Status: model.ApiCallStatusFailed},
		{ID: "retry-success", UserID: "u1", BillingOrderID: "b1", Model: "model-a", Status: model.ApiCallStatusSucceeded},
		{ID: "other-channel", UserID: "u2", BillingOrderID: "b2", Model: "model-a", ChannelID: "second", Status: model.ApiCallStatusSucceeded},
		{ID: "pending", UserID: "u1", TaskID: "running", Model: "video-model", Capability: "video", Status: model.ApiCallStatusSucceeded},
		{ID: "final-failed", UserID: "u2", TaskID: "failed", Model: "video-model", Capability: "video", Status: model.ApiCallStatusSucceeded},
		{ID: "free", UserID: "u1", TaskID: "done", Model: "free-model", Status: model.ApiCallStatusSucceeded},
		{ID: "legacy", UserID: "u1", Model: "legacy-model", Status: model.ApiCallStatusSucceeded},
		{ID: "poll", UserID: "u1", RequestKind: "poll", Model: "ignore"},
		{ID: "download", UserID: "u1", RequestKind: "download", Model: "ignore"},
		{ID: "preflight", UserID: "u1", Method: "INTERNAL", Model: "ignore"},
		{ID: "admin-log", UserID: "admin", Model: "ignore"},
		{ID: "prior-day-retry", UserID: "u1", BillingOrderID: "old", Model: "old-model", Status: model.ApiCallStatusSucceeded},
	}
	for index := range logs {
		if logs[index].Capability == "" {
			logs[index].Capability = "text"
		}
		if logs[index].RequestKind == "" {
			logs[index].RequestKind = "create"
		}
		if logs[index].Method == "" {
			logs[index].Method = "POST"
		}
		logs[index].CreatedAt = from.Add(time.Duration(index+1) * time.Minute)
		if index%2 == 0 {
			logs[index].CreatedAt = logs[index].CreatedAt.In(beijing)
		}
	}
	create(&logs)
	create(&model.ApiCallLog{ID: "prior-day-root", UserID: "u1", BillingOrderID: "old", Model: "old-model", Capability: "text", RequestKind: "create", Method: "POST", Status: model.ApiCallStatusFailed, CreatedAt: from.Add(-time.Minute).In(beijing)})
	repo := New(db)
	filter := ControlPreviewFilter{From: from, To: now, Page: 1, PageSize: 1}
	snapshot, err := repo.ControlPreviewSnapshot(filter, true)
	if err != nil {
		t.Fatal(err)
	}
	kpi := snapshot.Totals
	if kpi.AvailableMicrocredits != 3000 || kpi.ReservedMicrocredits != 100 || kpi.ConsumedMicrocredits != 200 || kpi.LifetimeConsumedMicrocredits != 250 || kpi.InflowMicrocredits != 1660 || kpi.ConsumingUsers != 2 || kpi.PaidAmountFen != 1200 || kpi.PaidOrders != 1 || kpi.PayingUsers != 1 {
		t.Fatalf("incorrect ledger/payment totals: %+v", kpi)
	}
	if kpi.Generations != 6 || kpi.Succeeded != 4 || kpi.Failed != 1 || kpi.Pending != 1 || kpi.UnlinkedGenerations != 1 {
		t.Fatalf("incorrect generation totals: %+v", kpi)
	}
	if len(snapshot.Ranking) != 2 || snapshot.Ranking[0].UserID != "u1" || snapshot.Ranking[1].Share != 0.5 {
		t.Fatalf("ranking: %+v", snapshot.Ranking)
	}
	var modelA *ControlPreviewModel
	for index := range snapshot.Models {
		if snapshot.Models[index].Model == "model-a" {
			modelA = &snapshot.Models[index]
		}
		if snapshot.Models[index].Model == "fallback-old" || snapshot.Models[index].Model == "old-model" {
			t.Fatal("retry changed model attribution or period")
		}
	}
	if modelA == nil || modelA.Generations != 2 || modelA.ConsumedMicrocredits != 200 {
		t.Fatalf("model aggregation: %+v", modelA)
	}
	if len(snapshot.Buckets) != 1 || snapshot.Buckets[0].Bucket != "2026-10-24 01:00" || snapshot.Buckets[0].InflowMicrocredits != 1660 {
		t.Fatalf("timezone buckets: %+v", snapshot.Buckets)
	}
	filter.Keyword = "beta"
	users, total, err := repo.ControlPreviewUsers(filter, false)
	if err != nil || total != 1 || len(users) != 1 || users[0].Rank != 2 || users[0].Share != 0.5 {
		t.Fatalf("filtered ranking must retain global rank/share: %+v %d %v", users, total, err)
	}
	filter.Keyword, filter.Model, filter.Capability = "", "free-model", "text"
	users, total, err = repo.ControlPreviewUsers(filter, true)
	if err != nil || total != 1 || users[0].ConsumedMicrocredits != 0 || users[0].Generations != 1 {
		t.Fatalf("free model users: %+v %d %v", users, total, err)
	}
	filter.UserID, filter.Model, filter.Capability = "u1", "", ""
	user, err := repo.ControlPreviewUser(filter)
	if err != nil || user.Username != "alpha" || user.ConsumedMicrocredits != 100 {
		t.Fatalf("user: %+v %v", user, err)
	}
	ledger, total, err := repo.ControlPreviewLedger(filter)
	if err != nil || total != 1 || len(ledger) != 1 || ledger[0].ID != "c1" {
		t.Fatalf("ledger: %+v %d %v", ledger, total, err)
	}
	filter.UserID = "admin"
	if _, err := repo.ControlPreviewUser(filter); err == nil {
		t.Fatal("administrator exposed as an ordinary user")
	}
	if err := db.Delete(&model.User{}, "id = ?", "u2").Error; err != nil {
		t.Fatal(err)
	}
	filter.UserID = ""
	updated, err := repo.ControlPreviewSnapshot(filter, false)
	if err != nil || updated.Totals.ConsumedMicrocredits != 100 {
		t.Fatalf("purged users must disappear: %+v %v", updated, err)
	}
}
