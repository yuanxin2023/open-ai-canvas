package app

import (
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRedeemBatchCanBeReviewedAndRecordsAuditIP(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.User{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.RedeemBatch{}, &model.RedeemCode{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	admin := &model.User{ID: "admin-1", Username: "admin", DisplayName: "管理员", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	user := &model.User{ID: "user-1", Username: "alice", DisplayName: "Alice", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db), dataDir: t.TempDir()}
	created, err := svc.AdminCreateRedeemBatch(admin, CreateRedeemBatchRequest{AmountMicrocredits: CreditScale, Count: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Codes) != 2 {
		t.Fatalf("created codes = %d", len(created.Codes))
	}
	page, err := svc.AdminRedeemCodePage(admin, created.Batch.ID, "", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !page.PlaintextAvailable || len(page.Codes) != 2 || page.Codes[0].Code == "" {
		t.Fatalf("initial code page = %#v", page)
	}
	lookup, err := svc.AdminLookupRedeemCode(admin, AdminRedeemCodeLookupRequest{Code: "  " + strings.ToUpper(created.Codes[0]) + "  "})
	if err != nil {
		t.Fatal(err)
	}
	if lookup.Code.Code != created.Codes[0] || lookup.Code.Status != string(model.RedeemCodeUnused) || lookup.Batch.ID != created.Batch.ID {
		t.Fatalf("unused lookup = %#v", lookup)
	}
	nonHexCode := strings.Repeat("z", 32)
	if err := db.Create(&model.RedeemCode{
		ID:                 "code-non-hex",
		BatchID:            created.Batch.ID,
		CodeHash:           hashRedeemCode(nonHexCode),
		CodeSuffix:         nonHexCode[len(nonHexCode)-4:],
		AmountMicrocredits: CreditScale,
		Status:             model.RedeemCodeUnused,
	}).Error; err != nil {
		t.Fatal(err)
	}
	nonHexLookup, err := svc.AdminLookupRedeemCode(admin, AdminRedeemCodeLookupRequest{Code: nonHexCode})
	if err != nil {
		t.Fatal(err)
	}
	if nonHexLookup.Code.ID != "code-non-hex" || nonHexLookup.Code.Code != nonHexCode {
		t.Fatalf("non-hex lookup = %#v", nonHexLookup.Code)
	}
	partial, err := svc.AdminSearchRedeemCodes(admin, AdminRedeemCodeSearchRequest{Query: created.Codes[0][:31]})
	if err != nil {
		t.Fatal(err)
	}
	if partial.Total != 1 || len(partial.Matches) != 1 || partial.Matches[0].Code.Code != created.Codes[0] {
		t.Fatalf("partial lookup = %#v", partial)
	}
	if _, err := svc.RedeemCredits(user, created.Codes[0], "203.0.113.8"); err != nil {
		t.Fatal(err)
	}
	redeemed, err := svc.AdminRedeemCodePage(admin, created.Batch.ID, "redeemed", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(redeemed.Codes) != 1 || redeemed.Codes[0].RedeemedBy != user.ID || redeemed.Codes[0].RedeemedUsername != user.Username || redeemed.Codes[0].RedeemedIP != "203.0.113.8" || redeemed.Codes[0].RedeemedAt == nil {
		t.Fatalf("redeemed code = %#v", redeemed.Codes)
	}
	lookup, err = svc.AdminLookupRedeemCode(admin, AdminRedeemCodeLookupRequest{Code: created.Codes[0]})
	if err != nil {
		t.Fatal(err)
	}
	if lookup.Code.Status != string(model.RedeemCodeRedeemed) || lookup.Code.RedeemedBy != user.ID || lookup.Code.RedeemedUsername != user.Username || lookup.Code.RedeemedIP != "203.0.113.8" || lookup.Code.RedeemedAt == nil {
		t.Fatalf("redeemed lookup = %#v", lookup.Code)
	}
	if err := svc.AdminDisableRedeemCode(admin, created.Batch.ID, page.Codes[1].ID); err != nil {
		t.Fatal(err)
	}
	disabled, err := svc.AdminLookupRedeemCode(admin, AdminRedeemCodeLookupRequest{Code: created.Codes[1]})
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Code.Status != string(model.RedeemCodeDisabled) {
		t.Fatalf("disabled lookup = %#v", disabled.Code)
	}
}
