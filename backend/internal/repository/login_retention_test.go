package repository

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func withLoginAuditDatabases(t *testing.T, run func(*testing.T, *gorm.DB)) {
	t.Helper()
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
			var db *gorm.DB
			var err error
			if dialect == "sqlite" {
				db, err = gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "audit.db")+"?_busy_timeout=10000&_journal_mode=WAL"), config)
			} else {
				dsn := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("set CANVAS_TEST_POSTGRES_DSN to an isolated test database")
				}
				admin, openErr := gorm.Open(postgres.Open(dsn), config)
				if openErr != nil {
					t.Fatal(openErr)
				}
				schema := fmt.Sprintf("login_audit_%d", time.Now().UnixNano())
				if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
					connection, _ := admin.DB()
					_ = connection.Close()
				})
				parsed, parseErr := url.Parse(dsn)
				if parseErr != nil {
					t.Fatal(parseErr)
				}
				query := parsed.Query()
				query.Set("search_path", schema)
				parsed.RawQuery = query.Encode()
				db, err = gorm.Open(postgres.Open(parsed.String()), config)
			}
			if err != nil {
				t.Fatal(err)
			}
			connection, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			connection.SetMaxOpenConns(8)
			t.Cleanup(func() { _ = connection.Close() })
			if err := db.AutoMigrate(&model.User{}, &model.AuthSession{}, &model.UserLoginEvent{}, &model.LoginLog{}); err != nil {
				t.Fatal(err)
			}
			run(t, db)
		})
	}
}

func loginAuditCount(t *testing.T, db *gorm.DB, row any, where string, args ...any) int64 {
	t.Helper()
	var count int64
	if err := db.Model(row).Where(where, args...).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func TestLoginAuditRetentionCombinesStoresAndPreservesOtherEvents(t *testing.T) {
	withLoginAuditDatabases(t, func(t *testing.T, db *gorm.DB) {
		now := time.Now().UTC().Truncate(time.Millisecond)
		user := model.User{ID: "owner", Username: "owner", PasswordHash: "fixture-hash", Status: model.UserStatusActive}
		other := model.User{ID: "other", Username: "other", Status: model.UserStatusActive}
		if err := db.Create(&[]model.User{user, other}).Error; err != nil {
			t.Fatal(err)
		}
		var legacy []model.UserLoginEvent
		var logs []model.LoginLog
		for i := 0; i < 110; i++ {
			legacy = append(legacy, model.UserLoginEvent{ID: fmt.Sprintf("legacy-%03d", i), UserID: user.ID, CreatedAt: now.Add(-time.Duration(i*2+1) * time.Minute)})
			logs = append(logs, model.LoginLog{ID: fmt.Sprintf("current-%03d", i), UserID: &user.ID, Event: "login", Result: "success", CreatedAt: now.Add(-time.Duration(i*2+2) * time.Minute)})
		}
		legacy = append(legacy,
			model.UserLoginEvent{ID: "expired-legacy", UserID: user.ID, CreatedAt: now.Add(-91 * 24 * time.Hour)},
			model.UserLoginEvent{ID: "other-legacy", UserID: other.ID, SessionID: "other-session", CreatedAt: now},
		)
		for i := 0; i < 210; i++ {
			logs = append(logs, model.LoginLog{ID: fmt.Sprintf("failed-%03d", i), UserID: &user.ID, Event: "login", Result: "failed", CreatedAt: now.Add(-time.Hour)})
		}
		for _, row := range []model.LoginLog{
			{ID: "recent-logout", UserID: &user.ID, Event: "logout", Result: "success", CreatedAt: now},
			{ID: "recent-unknown", Event: "login", Result: "failed", CreatedAt: now},
			{ID: "expired-current", UserID: &user.ID, Event: "login", Result: "success", CreatedAt: now.Add(-91 * 24 * time.Hour)},
			{ID: "expired-failure", UserID: &user.ID, Event: "login", Result: "failed", CreatedAt: now.Add(-91 * 24 * time.Hour)},
			{ID: "expired-logout", UserID: &user.ID, Event: "logout", Result: "success", CreatedAt: now.Add(-91 * 24 * time.Hour)},
			{ID: "expired-unknown", Event: "login", Result: "failed", CreatedAt: now.Add(-91 * 24 * time.Hour)},
		} {
			logs = append(logs, row)
		}
		if err := db.Create(&legacy).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.CreateInBatches(&logs, 100).Error; err != nil {
			t.Fatal(err)
		}
		session := model.AuthSession{ID: "new-session", UserID: user.ID, TokenHash: "fixture-token", ExpiresAt: now.Add(time.Hour)}
		event := model.LoginLog{ID: "new-success", UserID: &user.ID, Event: "login", Result: "success", CreatedAt: now}
		repo := New(db)
		if err := repo.CreateAuthSessionForUser(&user, &session, &event); err != nil {
			t.Fatal(err)
		}
		oldCount := loginAuditCount(t, db, &model.UserLoginEvent{}, "user_id = ?", user.ID)
		newCount := loginAuditCount(t, db, &model.LoginLog{}, "user_id = ? AND event = ? AND result = ?", user.ID, "login", "success")
		if oldCount+newCount != 200 || oldCount != 100 || newCount != 100 {
			t.Fatalf("retained legacy=%d current=%d", oldCount, newCount)
		}
		for attempt := 0; attempt < 2; attempt++ {
			if err := repo.PruneLoginAudit(now); err != nil {
				t.Fatal(err)
			}
		}
		if got := loginAuditCount(t, db, &model.LoginLog{}, "result = ?", "failed"); got != 211 {
			t.Fatalf("failure cap incorrectly applied: %d", got)
		}
		if got := loginAuditCount(t, db, &model.LoginLog{}, "event = ?", "logout"); got != 1 {
			t.Fatalf("logout retention=%d", got)
		}
		if got := loginAuditCount(t, db, &model.LoginLog{}, "id LIKE ?", "expired-%"); got != 0 {
			t.Fatalf("expired audit retained: %d", got)
		}
		if got := loginAuditCount(t, db, &model.UserLoginEvent{}, "user_id = ?", other.ID); got != 1 {
			t.Fatal("another user's history was trimmed")
		}
		if got := loginAuditCount(t, db, &model.AuthSession{}, "id = ?", session.ID); got != 1 {
			t.Fatal("audit pruning revoked a valid session")
		}
		if got := loginAuditCount(t, db, &model.UserLoginEvent{}, "id = ?", "legacy-099"); got != 1 {
			t.Fatal("latest legacy boundary lost")
		}
		if got := loginAuditCount(t, db, &model.LoginLog{}, "id = ?", "current-099"); got != 0 {
			t.Fatal("older current event displaced a newer legacy event")
		}
	})
}

func TestLoginAuditRetentionUsesInstantsAndStableCrossStoreTies(t *testing.T) {
	withLoginAuditDatabases(t, func(t *testing.T, db *gorm.DB) {
		now := time.Now().UTC().Truncate(time.Second)
		user := model.User{ID: "ties", Username: "ties", Status: model.UserStatusActive}
		boundaryUser := model.User{ID: "boundary", Username: "boundary", Status: model.UserStatusActive}
		if err := db.Create(&[]model.User{user, boundaryUser}).Error; err != nil {
			t.Fatal(err)
		}
		var expected []string
		for i := 0; i < 105; i++ {
			id := fmt.Sprintf("same-%03d", i)
			legacy := model.UserLoginEvent{ID: id, UserID: user.ID, CreatedAt: now.In(time.FixedZone("past", -12*3600))}
			current := model.LoginLog{ID: id, UserID: &user.ID, Event: "login", Result: "success", CreatedAt: now.In(time.FixedZone("future", 14*3600))}
			if err := db.Create(&legacy).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&current).Error; err != nil {
				t.Fatal(err)
			}
			expected = append(expected, "legacy:"+id, id)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(expected)))
		expected = expected[:200]
		cutoff := now.Add(-userLoginEventRetention)
		for _, offset := range []int{-12, 14} {
			for _, expired := range []bool{false, true} {
				at := cutoff
				if expired {
					at = at.Add(-time.Second)
				}
				at = at.In(time.FixedZone("stored", offset*3600))
				id := fmt.Sprintf("boundary-%d-%t", offset, expired)
				if err := db.Create(&model.UserLoginEvent{ID: id, UserID: boundaryUser.ID, CreatedAt: at}).Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Create(&model.LoginLog{ID: id, Event: "login", Result: "failed", CreatedAt: at}).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
		repo := New(db)
		if err := repo.PruneLoginAudit(now); err != nil {
			t.Fatal(err)
		}
		got, total, err := repo.LoginLogs(LoginLogFilter{Keyword: user.ID}, 300, 0)
		if err != nil || total != 200 {
			t.Fatalf("total=%d error=%v", total, err)
		}
		ids := make([]string, 0, len(got))
		for _, row := range got {
			ids = append(ids, row.ID)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(ids)))
		if !reflect.DeepEqual(ids, expected) {
			t.Fatalf("cross-store ties changed retained IDs: %v", ids)
		}
		if got := loginAuditCount(t, db, &model.UserLoginEvent{}, "user_id = ?", boundaryUser.ID); got != 2 {
			t.Fatalf("historical 90-day boundary=%d", got)
		}
		if got := loginAuditCount(t, db, &model.LoginLog{}, "result = ?", "failed"); got != 2 {
			t.Fatalf("unknown-account 90-day boundary=%d", got)
		}
	})
}

func TestLoginAuditRetentionFailureRollsBackSessionAndAudit(t *testing.T) {
	withLoginAuditDatabases(t, func(t *testing.T, db *gorm.DB) {
		user := model.User{ID: "rollback", Username: "rollback", PasswordHash: "fixture-hash", Status: model.UserStatusActive}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		if err := db.Create(&model.UserLoginEvent{ID: "old", UserID: user.ID, CreatedAt: now.Add(-91 * 24 * time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Callback().Delete().Before("gorm:delete").Register("test:fail-login-cleanup", func(tx *gorm.DB) {
			if tx.Statement.Table == "user_login_events" {
				tx.AddError(errors.New("injected cleanup failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Callback().Delete().Remove("test:fail-login-cleanup") })
		session := model.AuthSession{ID: "rollback-session", UserID: user.ID, ExpiresAt: now.Add(time.Hour)}
		event := model.LoginLog{ID: "rollback-log", UserID: &user.ID, Event: "login", Result: "success", CreatedAt: now}
		if err := New(db).CreateAuthSessionForUser(&user, &session, &event); err == nil {
			t.Fatal("cleanup failure ignored")
		}
		if loginAuditCount(t, db, &model.AuthSession{}, "1 = 1") != 0 || loginAuditCount(t, db, &model.LoginLog{}, "1 = 1") != 0 {
			t.Fatal("failed login partially committed")
		}
		if loginAuditCount(t, db, &model.UserLoginEvent{}, "id = ?", "old") != 1 {
			t.Fatal("failed login removed historical audit")
		}
	})
}

func TestLoginAuditRetentionSerializesConcurrentLoginsAndSweep(t *testing.T) {
	withLoginAuditDatabases(t, func(t *testing.T, db *gorm.DB) {
		user := model.User{ID: "concurrent", Username: "concurrent", PasswordHash: "fixture-hash", Status: model.UserStatusActive}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		var rows []model.UserLoginEvent
		for i := 0; i < 199; i++ {
			rows = append(rows, model.UserLoginEvent{ID: fmt.Sprintf("before-%03d", i), UserID: user.ID, CreatedAt: now.Add(-time.Hour)})
		}
		if err := db.Create(&rows).Error; err != nil {
			t.Fatal(err)
		}
		repo := New(db)
		start := make(chan struct{})
		results := make(chan error, 13)
		var workers sync.WaitGroup
		for i := 0; i < 12; i++ {
			workers.Add(1)
			go func(i int) {
				defer workers.Done()
				<-start
				session := model.AuthSession{ID: fmt.Sprintf("session-%d", i), UserID: user.ID, ExpiresAt: now.Add(time.Hour)}
				event := model.LoginLog{ID: fmt.Sprintf("success-%d", i), UserID: &user.ID, Event: "login", Result: "success", CreatedAt: now.Add(time.Duration(i) * time.Second)}
				results <- repo.CreateAuthSessionForUser(&user, &session, &event)
			}(i)
		}
		workers.Add(1)
		go func() { defer workers.Done(); <-start; results <- repo.PruneLoginAudit(now) }()
		close(start)
		workers.Wait()
		close(results)
		for err := range results {
			if err != nil {
				t.Fatal(err)
			}
		}
		oldCount := loginAuditCount(t, db, &model.UserLoginEvent{}, "user_id = ?", user.ID)
		newCount := loginAuditCount(t, db, &model.LoginLog{}, "user_id = ?", user.ID)
		if oldCount+newCount != 200 || newCount != 12 {
			t.Fatalf("concurrent retention legacy=%d current=%d", oldCount, newCount)
		}
		if loginAuditCount(t, db, &model.AuthSession{}, "user_id = ?", user.ID) != 12 {
			t.Fatal("concurrent logins lost sessions")
		}
	})
}
