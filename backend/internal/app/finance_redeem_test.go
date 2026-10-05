package app

import (
	"strings"
	"testing"
	"time"

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
	page, err := svc.AdminRedeemCodePage(admin, created.Batch.ID, "", "", 1, 50)
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
	redeemed, err := svc.AdminRedeemCodePage(admin, created.Batch.ID, "", "redeemed", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(redeemed.Codes) != 1 || redeemed.Codes[0].RedeemedBy != user.ID || redeemed.Codes[0].RedeemedUsername != user.Username || redeemed.Codes[0].RedeemedDisplayName != user.Username || redeemed.Codes[0].RedeemedIP != "203.0.113.8" || redeemed.Codes[0].RedeemedAt == nil {
		t.Fatalf("redeemed code = %#v", redeemed.Codes)
	}
	lookup, err = svc.AdminLookupRedeemCode(admin, AdminRedeemCodeLookupRequest{Code: created.Codes[0]})
	if err != nil {
		t.Fatal(err)
	}
	if lookup.Code.Status != string(model.RedeemCodeRedeemed) || lookup.Code.RedeemedBy != user.ID || lookup.Code.RedeemedUsername != user.Username || lookup.Code.RedeemedDisplayName != user.Username || lookup.Code.RedeemedIP != "203.0.113.8" || lookup.Code.RedeemedAt == nil {
		t.Fatalf("redeemed lookup = %#v", lookup.Code)
	}
	if _, err := svc.AdminDisableRedeemCode(admin, created.Batch.ID, page.Codes[1].ID, ""); err != nil {
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

func TestScopedRedeemBatchesAreFundedAndIsolatedByCreator(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.User{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.RedeemBatch{}, &model.RedeemCode{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	full := &model.User{ID: "full", Username: "root", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull, Status: model.UserStatusActive}
	moduleA := &model.User{ID: "module-a", Username: "modulea", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionRedeemCodes}, Status: model.UserStatusActive}
	moduleB := &model.User{ID: "module-b", Username: "moduleb", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionRedeemCodes}, Status: model.UserStatusActive}
	redeemer := &model.User{ID: "redeemer", Username: "alice", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create([]*model.User{full, moduleA, moduleB, redeemer}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create([]model.CreditAccount{
		{UserID: moduleA.ID, AvailableMicrocredits: 10 * CreditScale, ReservedMicrocredits: 4 * CreditScale},
		{UserID: moduleB.ID, AvailableMicrocredits: 20 * CreditScale},
	}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db), dataDir: t.TempDir()}

	batchA, err := svc.AdminCreateRedeemBatch(moduleA, CreateRedeemBatchRequest{AmountMicrocredits: CreditScale, Count: 2, ExpiresAt: timePointer(time.Now().Add(time.Hour))})
	if err != nil {
		t.Fatal(err)
	}
	batchB, err := svc.AdminCreateRedeemBatch(moduleB, CreateRedeemBatchRequest{AmountMicrocredits: CreditScale, Count: 3})
	if err != nil {
		t.Fatal(err)
	}
	if batchA.Batch.FundingSource != model.RedeemBatchFundingModuleAdmin || batchA.Account == nil || batchA.Account.AvailableMicrocredits != 8*CreditScale || batchA.Account.ReservedMicrocredits != 6*CreditScale {
		t.Fatalf("module A creation = %#v", batchA)
	}
	if _, err := svc.AdminCreateRedeemBatch(moduleA, CreateRedeemBatchRequest{AmountMicrocredits: CreditScale, Count: 9}); err == nil {
		t.Fatal("module A created an underfunded batch")
	}
	var accountAfterRejectedCreate model.CreditAccount
	if err := db.First(&accountAfterRejectedCreate, "user_id = ?", moduleA.ID).Error; err != nil {
		t.Fatal(err)
	}
	var moduleABatchCount int64
	if err := db.Model(&model.RedeemBatch{}).Where("created_by = ?", moduleA.ID).Count(&moduleABatchCount).Error; err != nil {
		t.Fatal(err)
	}
	if accountAfterRejectedCreate.AvailableMicrocredits != 8*CreditScale || accountAfterRejectedCreate.ReservedMicrocredits != 6*CreditScale || moduleABatchCount != 1 {
		t.Fatalf("underfunded creation changed state: account=%#v batches=%d", accountAfterRejectedCreate, moduleABatchCount)
	}

	pageA, err := svc.AdminRedeemBatchPage(moduleA, AdminListQuery{FundingSource: string(model.RedeemBatchFundingModuleAdmin), Page: 1, Limit: 20})
	if err != nil || len(pageA.Batches) != 1 || pageA.Batches[0].ID != batchA.Batch.ID {
		t.Fatalf("module A page = %#v, %v", pageA, err)
	}
	if pageA.FundingSummary == nil || pageA.FundingSummary.AvailableMicrocredits != 8*CreditScale || pageA.FundingSummary.RedeemReservedMicrocredits != 2*CreditScale || pageA.FundingSummary.OtherReservedMicrocredits != 4*CreditScale || pageA.FundingSummary.TotalReservedMicrocredits != 6*CreditScale {
		t.Fatalf("module A funding summary = %#v", pageA.FundingSummary)
	}
	filteredPageA, err := svc.AdminRedeemBatchPage(moduleA, AdminListQuery{Keyword: "no-such-batch", Status: "expired", FundingSource: string(model.RedeemBatchFundingModuleAdmin), Page: 1, Limit: 1})
	if err != nil || len(filteredPageA.Batches) != 0 || filteredPageA.FundingSummary == nil || *filteredPageA.FundingSummary != *pageA.FundingSummary {
		t.Fatalf("filtered module A page = %#v, %v", filteredPageA, err)
	}
	pageB, err := svc.AdminRedeemBatchPage(moduleB, AdminListQuery{FundingSource: string(model.RedeemBatchFundingModuleAdmin), Page: 1, Limit: 20})
	if err != nil || len(pageB.Batches) != 1 || pageB.Batches[0].ID != batchB.Batch.ID {
		t.Fatalf("module B page = %#v, %v", pageB, err)
	}
	if pageB.FundingSummary == nil || pageB.FundingSummary.AvailableMicrocredits != 17*CreditScale || pageB.FundingSummary.RedeemReservedMicrocredits != 3*CreditScale || pageB.FundingSummary.OtherReservedMicrocredits != 0 {
		t.Fatalf("module B funding summary = %#v", pageB.FundingSummary)
	}
	allModules, err := svc.AdminRedeemBatchPage(full, AdminListQuery{FundingSource: string(model.RedeemBatchFundingModuleAdmin), Page: 1, Limit: 20})
	if err != nil || len(allModules.Batches) != 2 || allModules.FundingSummary != nil {
		t.Fatalf("full module page = %#v, %v", allModules, err)
	}
	filteredModules, err := svc.AdminRedeemBatchPage(full, AdminListQuery{FundingSource: string(model.RedeemBatchFundingModuleAdmin), CreatorID: moduleA.ID, Page: 1, Limit: 20})
	if err != nil || len(filteredModules.Batches) != 1 || filteredModules.Batches[0].ID != batchA.Batch.ID {
		t.Fatalf("full filtered module page = %#v, %v", filteredModules, err)
	}
	if _, err := svc.AdminRedeemCodePage(moduleA, batchB.Batch.ID, string(model.RedeemBatchFundingModuleAdmin), "", 1, 20); err == nil {
		t.Fatal("module A read module B batch")
	}
	if _, err := svc.AdminLookupRedeemCode(moduleB, AdminRedeemCodeLookupRequest{Code: batchA.Codes[0], FundingSource: string(model.RedeemBatchFundingModuleAdmin)}); err == nil {
		t.Fatal("module B looked up module A code")
	}
	if _, err := svc.AdminDisableRedeemBatch(moduleA, batchB.Batch.ID, string(model.RedeemBatchFundingModuleAdmin)); err == nil {
		t.Fatal("module A disabled module B batch")
	}

	if _, err := svc.RedeemCredits(redeemer, batchA.Codes[0], "203.0.113.10"); err != nil {
		t.Fatal(err)
	}
	var accountA model.CreditAccount
	if err := db.First(&accountA, "user_id = ?", moduleA.ID).Error; err != nil {
		t.Fatal(err)
	}
	if accountA.AvailableMicrocredits != 8*CreditScale || accountA.ReservedMicrocredits != 5*CreditScale {
		t.Fatalf("module A after redeem = %#v", accountA)
	}
	pageA, err = svc.AdminRedeemBatchPage(moduleA, AdminListQuery{FundingSource: string(model.RedeemBatchFundingModuleAdmin), Page: 1, Limit: 20})
	if err != nil || pageA.FundingSummary == nil || pageA.FundingSummary.RedeemReservedMicrocredits != CreditScale || pageA.FundingSummary.OtherReservedMicrocredits != 4*CreditScale || pageA.FundingSummary.TotalReservedMicrocredits != 5*CreditScale {
		t.Fatalf("module A funding summary after redeem = %#v, %v", pageA.FundingSummary, err)
	}
	disabled, err := svc.AdminDisableRedeemBatch(full, batchA.Batch.ID, string(model.RedeemBatchFundingModuleAdmin))
	if err != nil || disabled.DisabledCount != 1 || disabled.RefundedMicrocredits != CreditScale {
		t.Fatalf("disable module A batch = %#v, %v", disabled, err)
	}
	if err := db.First(&accountA, "user_id = ?", moduleA.ID).Error; err != nil {
		t.Fatal(err)
	}
	var accountB model.CreditAccount
	if err := db.First(&accountB, "user_id = ?", moduleB.ID).Error; err != nil {
		t.Fatal(err)
	}
	if accountA.AvailableMicrocredits != 9*CreditScale || accountA.ReservedMicrocredits != 4*CreditScale || accountB.AvailableMicrocredits != 17*CreditScale || accountB.ReservedMicrocredits != 3*CreditScale {
		t.Fatalf("isolated balances: A=%#v B=%#v", accountA, accountB)
	}
	pageA, err = svc.AdminRedeemBatchPage(moduleA, AdminListQuery{FundingSource: string(model.RedeemBatchFundingModuleAdmin), Page: 1, Limit: 20})
	if err != nil || pageA.FundingSummary == nil || pageA.FundingSummary.AvailableMicrocredits != 9*CreditScale || pageA.FundingSummary.RedeemReservedMicrocredits != 0 || pageA.FundingSummary.OtherReservedMicrocredits != 4*CreditScale {
		t.Fatalf("module A funding summary after disable = %#v, %v", pageA.FundingSummary, err)
	}

	platform, err := svc.AdminCreateRedeemBatch(full, CreateRedeemBatchRequest{AmountMicrocredits: CreditScale, Count: 1})
	if err != nil || platform.Batch.FundingSource != model.RedeemBatchFundingPlatform || platform.Account != nil {
		t.Fatalf("platform creation = %#v, %v", platform, err)
	}
	platformPage, err := svc.AdminRedeemBatchPage(full, AdminListQuery{FundingSource: string(model.RedeemBatchFundingPlatform), Page: 1, Limit: 20})
	if err != nil || platformPage.FundingSummary != nil {
		t.Fatalf("platform funding summary = %#v, %v", platformPage.FundingSummary, err)
	}
}

func TestScopedRedeemExpirationRefundsExactlyOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.User{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.RedeemBatch{}, &model.RedeemCode{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	module := &model.User{ID: "module", Username: "module", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionRedeemCodes}, Status: model.UserStatusActive}
	if err := db.Create(module).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: module.ID, AvailableMicrocredits: 5 * CreditScale}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db), dataDir: t.TempDir()}
	created, err := svc.AdminCreateRedeemBatch(module, CreateRedeemBatchRequest{AmountMicrocredits: CreditScale, Count: 2, ExpiresAt: timePointer(time.Now().Add(time.Hour))})
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Minute)
	if err := db.Model(&model.RedeemCode{}).Where("batch_id = ?", created.Batch.ID).Update("expires_at", past).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		page, err := svc.AdminRedeemBatchPage(module, AdminListQuery{FundingSource: string(model.RedeemBatchFundingModuleAdmin), Page: 1, Limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		if page.FundingSummary == nil || page.FundingSummary.AvailableMicrocredits != 5*CreditScale || page.FundingSummary.RedeemReservedMicrocredits != 0 || page.FundingSummary.TotalReservedMicrocredits != 0 {
			t.Fatalf("expired funding summary = %#v", page.FundingSummary)
		}
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", module.ID).Error; err != nil {
		t.Fatal(err)
	}
	if account.AvailableMicrocredits != 5*CreditScale || account.ReservedMicrocredits != 0 {
		t.Fatalf("account after expiration = %#v", account)
	}
	var refunds int64
	if err := db.Model(&model.CreditLedgerEntry{}).Where("redeem_batch_id = ? AND type = ?", created.Batch.ID, model.CreditLedgerRefund).Count(&refunds).Error; err != nil {
		t.Fatal(err)
	}
	if refunds != 1 {
		t.Fatalf("refund ledger count = %d", refunds)
	}
}

func TestScopedRedeemFundingSummaryRejectsAccountMismatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.CreditAccount{}, &model.RedeemBatch{}, &model.RedeemCode{}); err != nil {
		t.Fatal(err)
	}
	module := &model.User{ID: "module", Username: "module", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionRedeemCodes}, Status: model.UserStatusActive}
	batch := &model.RedeemBatch{ID: "batch", AmountMicrocredits: CreditScale, Count: 1, CreatedBy: module.ID, FundingSource: model.RedeemBatchFundingModuleAdmin}
	code := &model.RedeemCode{ID: "code", BatchID: batch.ID, AmountMicrocredits: CreditScale, Status: model.RedeemCodeUnused}
	if err := db.Create(module).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditAccount{UserID: module.ID, AvailableMicrocredits: CreditScale}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(batch).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(code).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db), dataDir: t.TempDir()}
	if _, err := svc.AdminRedeemBatchPage(module, AdminListQuery{FundingSource: string(model.RedeemBatchFundingModuleAdmin), Page: 1, Limit: 20}); err == nil || !strings.Contains(err.Error(), "冻结积分不一致") {
		t.Fatalf("funding mismatch error = %v", err)
	}
}

func timePointer(value time.Time) *time.Time { return &value }
