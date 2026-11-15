package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func ledgerRedemptionFixture(t *testing.T) (*Service, *gorm.DB, *model.User, *model.User, *CreateRedeemBatchResult) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.User{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.RedeemBatch{}, &model.RedeemCode{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	admin := &model.User{ID: "admin", Username: "admin", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull, Status: model.UserStatusActive}
	user := &model.User{ID: "user", Username: "alice", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create([]*model.User{admin, user}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db), dataDir: t.TempDir()}
	batch, err := svc.AdminCreateRedeemBatch(admin, CreateRedeemBatchRequest{AmountMicrocredits: CreditScale, Count: 2, Note: "活动批次"})
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range batch.Codes {
		if _, err := svc.RedeemCredits(user, code, "203.0.113.8"); err != nil {
			t.Fatal(err)
		}
	}
	return svc, db, admin, user, batch
}

func TestAdminUserLedgerRedemption(t *testing.T) {
	svc, db, admin, user, batch := ledgerRedemptionFixture(t)
	page, err := svc.AdminUserLedger(admin, user.ID, "increase", 1, 20)
	if err != nil || len(page.Entries) != 2 || page.Total != 2 {
		t.Fatalf("ledger: %+v, %v", page, err)
	}
	entry := page.Entries[0]
	if entry.Redemption == nil || entry.Redemption.Status != "available" || len(entry.Redemption.Code) != 32 {
		t.Fatalf("missing complete code: %+v", entry)
	}
	detail, err := svc.AdminUserLedgerRedemption(admin, user.ID, entry.ID)
	if err != nil || detail.Code.Code != entry.Redemption.Code || detail.Code.RedeemedBy != user.ID || detail.Code.RedeemedUsername != "alice" || detail.Code.RedeemedIP != "203.0.113.8" || detail.Code.RedeemedAt == nil || detail.Batch.Note != "活动批次" || detail.AmountMicrocredits != CreditScale {
		t.Fatalf("detail: %+v, %v", detail, err)
	}
	// The shared ledger model must not carry admin-only plaintext.
	raw, err := json.Marshal(WalletSummary{Entries: []model.CreditLedgerEntry{entry.CreditLedgerEntry}})
	if err != nil || strings.Contains(string(raw), entry.Redemption.Code) || strings.Contains(string(raw), "redemption") {
		t.Fatalf("wallet leaked admin fields: %s, %v", raw, err)
	}
	all, err := svc.AdminUserLedger(admin, user.ID, "all", 1, 20)
	if err != nil || all.Entries[0].Redemption.Code != entry.Redemption.Code {
		t.Fatalf("all view: %+v, %v", all, err)
	}
	if err := db.Model(&model.RedeemBatch{}).Where("id = ?", batch.Batch.ID).Updates(map[string]any{"codes_cipher": "", "code_secrets_cleared_at": time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	historical, err := svc.AdminUserLedgerRedemption(admin, user.ID, entry.ID)
	if err != nil || historical.Code.Code != "" || historical.Code.CodeSuffix == "" || historical.Code.RedeemedBy != user.ID {
		t.Fatalf("cleared detail: %+v, %v", historical, err)
	}
	page, err = svc.AdminUserLedger(admin, user.ID, "increase", 1, 20)
	if err != nil || page.Entries[0].Redemption.Status != "available" || page.Entries[0].Redemption.Code != "" || page.Entries[0].Redemption.CodeSuffix == "" {
		t.Fatalf("cleared list: %+v, %v", page, err)
	}
}

func TestAdminUserLedgerRedemptionPermissions(t *testing.T) {
	svc, db, admin, user, batch := ledgerRedemptionFixture(t)
	page, err := svc.AdminUserLedger(admin, user.ID, "increase", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	entryID := page.Entries[0].ID
	scoped := &model.User{ID: "scoped", Role: model.UserRoleAdmin, Status: model.UserStatusActive, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionUsers}}
	page, err = svc.AdminUserLedger(scoped, user.ID, "increase", 1, 20)
	if err != nil || page.Entries[0].Redemption != nil {
		t.Fatalf("users-only list: %+v, %v", page, err)
	}
	if _, err := svc.AdminUserLedgerRedemption(scoped, user.ID, entryID); err == nil {
		t.Fatal("users-only admin read code")
	}
	scoped.AdminPermissions = append(scoped.AdminPermissions, model.AdminPermissionRedeemCodes)
	page, err = svc.AdminUserLedger(scoped, user.ID, "increase", 1, 20)
	if err != nil || page.Entries[0].Redemption != nil {
		t.Fatalf("foreign batch exposed: %+v, %v", page, err)
	}
	if _, err := svc.AdminUserLedgerRedemption(scoped, user.ID, entryID); err == nil {
		t.Fatal("scoped admin read platform batch")
	}
	if err := db.Model(&model.RedeemBatch{}).Where("id = ?", batch.Batch.ID).Updates(map[string]any{"created_by": scoped.ID, "funding_source": model.RedeemBatchFundingModuleAdmin}).Error; err != nil {
		t.Fatal(err)
	}
	if detail, err := svc.AdminUserLedgerRedemption(scoped, user.ID, entryID); err != nil || detail.Code.Code == "" {
		t.Fatalf("own batch: %+v, %v", detail, err)
	}
	scoped.ID = "another-admin"
	if _, err := svc.AdminUserLedgerRedemption(scoped, user.ID, entryID); err == nil {
		t.Fatal("scoped admin read another creator's code")
	}
	scoped.AdminPermissions = []model.AdminPermission{model.AdminPermissionRedeemCodes}
	if _, err := svc.AdminUserLedgerRedemption(scoped, user.ID, entryID); err == nil {
		t.Fatal("redeem-only admin bypassed user permission")
	}
	if _, err := svc.AdminUserLedgerRedemption(user, user.ID, entryID); err == nil {
		t.Fatal("ordinary user accessed admin detail")
	}
	if _, err := svc.AdminUserLedgerRedemption(admin, admin.ID, entryID); err == nil {
		t.Fatal("read another user's ledger entry")
	}
	if _, err := svc.AdminUserLedgerRedemption(scoped, admin.ID, entryID); err == nil {
		t.Fatal("scoped admin inspected administrator")
	}
}

func TestAdminUserLedgerRedemptionMissingAndErrors(t *testing.T) {
	for _, scenario := range []string{"missing code id", "missing code", "foreign redeemer", "wrong batch", "refund", "cleared association", "corrupt cipher", "query error"} {
		t.Run(scenario, func(t *testing.T) {
			svc, db, admin, user, batch := ledgerRedemptionFixture(t)
			page, err := svc.AdminUserLedger(admin, user.ID, "increase", 1, 20)
			if err != nil {
				t.Fatal(err)
			}
			entry := page.Entries[0]
			switch scenario {
			case "missing code id":
				err = db.Model(&model.CreditLedgerEntry{}).Where("id = ?", entry.ID).Update("redeem_code_id", "").Error
			case "missing code":
				err = db.Delete(&model.RedeemCode{}, "id = ?", entry.RedeemCodeID).Error
			case "foreign redeemer":
				err = db.Model(&model.RedeemCode{}).Where("id = ?", entry.RedeemCodeID).Update("redeemed_by", "other").Error
			case "wrong batch":
				err = db.Model(&model.CreditLedgerEntry{}).Where("id = ?", entry.ID).Update("redeem_batch_id", "other").Error
			case "refund":
				err = db.Model(&model.CreditLedgerEntry{}).Where("id = ?", entry.ID).Updates(map[string]any{"type": model.CreditLedgerRefund, "scene": "redeem_code"}).Error
			case "cleared association":
				err = db.Delete(&model.RedeemBatch{}, "id = ?", batch.Batch.ID).Error
			case "corrupt cipher":
				err = db.Model(&model.RedeemBatch{}).Where("id = ?", batch.Batch.ID).Update("codes_cipher", "invalid").Error
			case "query error":
				err = db.Migrator().DropTable(&model.RedeemCode{})
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.AdminUserLedgerRedemption(admin, user.ID, entry.ID); err == nil {
				t.Fatal("invalid association or failure accepted")
			}
			page, err = svc.AdminUserLedger(admin, user.ID, "increase", 1, 20)
			if scenario == "corrupt cipher" || scenario == "query error" {
				if err == nil {
					t.Fatal("list hid storage error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range page.Entries {
				if row.ID != entry.ID {
					continue
				}
				if scenario == "refund" {
					if row.Redemption != nil {
						t.Fatal("refund treated as redemption")
					}
				} else if row.Redemption == nil || row.Redemption.Status == "available" {
					t.Fatalf("missing data appears actionable: %+v", row)
				}
			}
		})
	}
}
