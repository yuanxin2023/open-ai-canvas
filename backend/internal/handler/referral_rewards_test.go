package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"infinite-canvas/backend/internal/auth"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestReferralRewardsHTTPIdentityContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rewards.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { conn.Close() })
	if err := db.AutoMigrate(&model.User{}, &model.AuthSession{}, &model.AdminPermissionGrant{}, &model.SystemSetting{}, &model.ReferralProfile{}, &model.ReferralPolicyVersion{}, &model.PaymentOrder{}, &model.ReferralReward{}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"full", "allowed", "denied", "user"} {
		role, level := model.UserRoleAdmin, model.AdminLevelScoped
		if id == "full" {
			level = model.AdminLevelFull
		}
		if id == "user" {
			role = model.UserRoleUser
		}
		if err := db.Create(&model.User{ID: id, Username: id, Email: id + "@example.com", PasswordHash: "private-hash", Role: role, AdminLevel: level, Status: model.UserStatusActive}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.AuthSession{ID: id, UserID: id, TokenHash: auth.HashToken("test"), ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.AdminPermissionGrant{UserID: "allowed", Permission: model.AdminPermissionReferrals}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ReferralProfile{UserID: "user", Code: "ABC234"}).Error; err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := db.Create(&model.PaymentOrder{ID: "internal-id", UserID: "full", IdempotencyKey: "paid", MerchantOrderNo: "MERCHANT-123", Status: model.PaymentOrderCredited, AmountFen: 1234, Currency: "CNY", CreditedAt: &stamp}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]model.ReferralReward{
		{ID: "a", InviterID: "user", InviteeID: "full", PaymentOrderID: "internal-id", Status: model.ReferralRewardPending, CreatedAt: stamp},
		{ID: "b", InviterID: "missing", InviteeID: "missing", PaymentOrderID: "missing", Status: model.ReferralRewardPending, CreatedAt: stamp.Add(-time.Hour)},
	}).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	registerReferralRoutes(router.Group("/api"), service.New(repository.New(db), t.TempDir()))
	call := func(actor, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api"+path, nil)
		if actor != "" {
			req.AddCookie(&http.Cookie{Name: service.SessionCookieName, Value: actor + ".test"})
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	for _, actor := range []string{"", "user", "denied", "full", "allowed"} {
		want := http.StatusOK
		if actor == "" {
			want = http.StatusUnauthorized
		} else if actor == "user" || actor == "denied" {
			want = http.StatusForbidden
		}
		response := call(actor, "/admin/referrals/rewards?status=pending&pageSize=20")
		if response.Code != want {
			t.Fatalf("actor %q: %d %s", actor, response.Code, response.Body.String())
		}
		var body struct {
			Code int `json:"code"`
			Data struct {
				Rewards []map[string]json.RawMessage `json:"rewards"`
				Total   int64                        `json:"total"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if want != http.StatusOK {
			if body.Code == 0 {
				t.Fatal("error returned success code")
			}
			continue
		}
		if body.Code != 0 || body.Data.Total != 2 || len(body.Data.Rewards) != 2 {
			t.Fatalf("invalid page: %s", response.Body.String())
		}
		for _, key := range []string{"inviter", "invitee"} {
			var summary map[string]string
			if err := json.Unmarshal(body.Data.Rewards[0][key], &summary); err != nil {
				t.Fatal(err)
			}
			id := "user"
			if key == "invitee" {
				id = "full"
			}
			if len(summary) != 3 || summary["id"] != id || summary["username"] != id || summary["email"] != id+"@example.com" {
				t.Fatalf("wrong %s: %+v", key, summary)
			}
		}
		var payment map[string]any
		if err := json.Unmarshal(body.Data.Rewards[0]["paymentOrder"], &payment); err != nil {
			t.Fatal(err)
		}
		if len(payment) != 7 || payment["id"] != "internal-id" || payment["userId"] != "full" || payment["merchantOrderNo"] != "MERCHANT-123" || payment["amountFen"] != float64(1234) || payment["status"] != "credited" || payment["creditedAt"] != stamp.Format(time.RFC3339) || payment["currency"] != "CNY" {
			t.Fatalf("wrong payment summary: %+v", payment)
		}
		for _, key := range []string{"inviter", "invitee", "paymentOrder"} {
			if string(body.Data.Rewards[1][key]) != "null" {
				t.Fatalf("missing %s must be explicit null", key)
			}
		}
	}
	response := call("user", "/referrals/me")
	var dashboard struct {
		Code int `json:"code"`
		Data struct {
			Rewards []map[string]json.RawMessage `json:"rewards"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &dashboard); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || dashboard.Code != 0 || len(dashboard.Data.Rewards) != 1 {
		t.Fatalf("user dashboard: %d %s", response.Code, response.Body.String())
	}
	for _, key := range []string{"inviter", "invitee", "paymentOrder"} {
		if _, exists := dashboard.Data.Rewards[0][key]; exists {
			t.Fatalf("admin %s leaked into user response", key)
		}
	}
}
