package app

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
)

type AdminLedgerRedemptionReference struct {
	Code       string `json:"code,omitempty"`
	CodeSuffix string `json:"codeSuffix,omitempty"`
	Status     string `json:"status"`
}

type AdminUserLedgerEntry struct {
	model.CreditLedgerEntry
	Redemption *AdminLedgerRedemptionReference `json:"redemption,omitempty"`
}

type AdminUserLedgerPage struct {
	Account model.CreditAccount    `json:"account"`
	Entries []AdminUserLedgerEntry `json:"entries"`
	Total   int64                  `json:"total"`
	Page    int                    `json:"page"`
	Limit   int                    `json:"pageSize"`
}

type AdminUserLedgerRedemption struct {
	Code               AdminRedeemCodeDetail `json:"code"`
	Batch              model.RedeemBatch     `json:"batch"`
	AmountMicrocredits int64                 `json:"amountMicrocredits"`
}

// Only the admin view resolves secrets; wallet ledger responses remain unchanged.
func (s *Service) adminLedgerRedemptions(actor *model.User, userID string, entries []model.CreditLedgerEntry) (map[string]AdminUserLedgerRedemption, error) {
	results := make(map[string]AdminUserLedgerRedemption)
	ids := []string{}
	for _, entry := range entries {
		if entry.Type == model.CreditLedgerRedeem && entry.RedeemCodeID != "" {
			ids = append(ids, entry.RedeemCodeID)
		}
	}
	rows, err := s.repo.LedgerRedeemCodes(userID, uniqueNonEmpty(ids))
	if err != nil {
		return nil, err
	}
	batchIDs := []string{}
	for _, row := range rows {
		batchIDs = append(batchIDs, row.BatchID)
	}
	batches, err := s.repo.LedgerRedeemBatches(uniqueNonEmpty(batchIDs))
	if err != nil {
		return nil, err
	}
	batchByID := make(map[string]model.RedeemBatch)
	plainByHash := make(map[string]string)
	for _, batch := range batches {
		if !canViewLedgerRedeemBatch(actor, batch) {
			continue
		}
		codes, err := s.redeemBatchPlainCodes(batch.CodesCipher)
		if err != nil {
			return nil, err
		}
		for _, code := range codes {
			plainByHash[hashRedeemCode(code)] = code
		}
		batchByID[batch.ID] = batch
	}
	for _, entry := range entries {
		if entry.Type != model.CreditLedgerRedeem {
			continue
		}
		for _, row := range rows {
			if row.ID != entry.RedeemCodeID || row.Status != model.RedeemCodeRedeemed || (entry.RedeemBatchID != "" && entry.RedeemBatchID != row.BatchID) {
				continue
			}
			batch, exists := batchByID[row.BatchID]
			if !exists {
				continue
			}
			code := plainByHash[row.CodeHash]
			if batch.CodesCipher != "" && code == "" {
				return nil, errors.New("兑换码批次密文与关联兑换码不一致")
			}
			batch.CodesCipher = ""
			results[entry.ID] = AdminUserLedgerRedemption{Code: adminRedeemCodeDetail(row, code, time.Now()), Batch: batch, AmountMicrocredits: entry.AmountMicrocredits}
		}
	}
	return results, nil
}

func canViewLedgerRedeemBatch(actor *model.User, batch model.RedeemBatch) bool {
	return actor.AdminLevel != model.AdminLevelScoped || (batch.FundingSource == model.RedeemBatchFundingModuleAdmin && batch.CreatedBy == actor.ID)
}

func (s *Service) AdminUserLedgerRedemption(actor *model.User, userID, entryID string) (*AdminUserLedgerRedemption, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	if _, err := s.manageableAdminUser(actor, userID); err != nil {
		return nil, err
	}
	if err := s.RequireAdminPermission(actor, model.AdminPermissionRedeemCodes); err != nil {
		return nil, err
	}
	entry, err := s.repo.UserLedgerEntry(userID, strings.TrimSpace(entryID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("积分流水不存在")
	}
	if err != nil {
		return nil, err
	}
	if entry.Type != model.CreditLedgerRedeem || entry.RedeemCodeID == "" {
		return nil, NotFound("兑换码信息未记录")
	}
	details, err := s.adminLedgerRedemptions(actor, userID, []model.CreditLedgerEntry{*entry})
	if err != nil {
		return nil, err
	}
	detail, exists := details[entry.ID]
	if !exists {
		return nil, NotFound("关联记录不可用")
	}
	return &detail, nil
}
