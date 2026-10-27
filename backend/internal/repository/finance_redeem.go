// 积分调整与兑换码批次。

package repository

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas/backend/internal/model"
)

func (r *Repository) AdjustCredits(userID string, actorUserID string, amount int64, note string) (*model.CreditAccount, error) {
	var account model.CreditAccount
	err := r.db.Transaction(func(tx *gorm.DB) error {
		account = model.CreditAccount{UserID: userID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&account).Error; err != nil {
			return err
		}
		accountQuery := tx.Model(&model.CreditAccount{}).Where("user_id = ?", userID)
		if amount < 0 {
			accountQuery = accountQuery.Where("available_microcredits + ? >= 0", amount)
		}
		updated := accountQuery.
			Updates(map[string]any{
				"available_microcredits": gorm.Expr("available_microcredits + ?", amount),
				"version":                gorm.Expr("version + 1"),
				"updated_at":             time.Now(),
			})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrInsufficientCredits
		}
		if err := tx.First(&account, "user_id = ?", userID).Error; err != nil {
			return err
		}
		entryType := model.CreditLedgerAdminAdjust
		if amount > 0 {
			entryType = model.CreditLedgerAdminGrant
		}
		return tx.Create(&model.CreditLedgerEntry{
			ID:                         newRepositoryID(),
			UserID:                     userID,
			Type:                       entryType,
			AmountMicrocredits:         amount,
			AvailableDeltaMicrocredits: amount,
			AvailableAfterMicrocredits: account.AvailableMicrocredits,
			ReservedAfterMicrocredits:  account.ReservedMicrocredits,
			ActorUserID:                actorUserID,
			Note:                       note,
		}).Error
	})
	return &account, err
}

func (r *Repository) CreateRedeemBatch(batch *model.RedeemBatch, codes []model.RedeemCode, totalMicrocredits int64, audit *model.AdminAuditEvent) (*model.CreditAccount, error) {
	var account *model.CreditAccount
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if batch.FundingSource == model.RedeemBatchFundingModuleAdmin {
			current := model.CreditAccount{UserID: batch.CreatedBy}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&current).Error; err != nil {
				return err
			}
			updated := tx.Model(&model.CreditAccount{}).
				Where("user_id = ? AND available_microcredits >= ?", batch.CreatedBy, totalMicrocredits).
				Updates(map[string]any{
					"available_microcredits": gorm.Expr("available_microcredits - ?", totalMicrocredits),
					"reserved_microcredits":  gorm.Expr("reserved_microcredits + ?", totalMicrocredits),
					"version":                gorm.Expr("version + 1"),
					"updated_at":             time.Now(),
				})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ErrInsufficientCredits
			}
			if err := tx.First(&current, "user_id = ?", batch.CreatedBy).Error; err != nil {
				return err
			}
			account = &current
		}
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
		if err := tx.CreateInBatches(&codes, 200).Error; err != nil {
			return err
		}
		if audit != nil {
			if err := tx.Create(audit).Error; err != nil {
				return err
			}
		}
		if account == nil {
			return nil
		}
		return tx.Create(&model.CreditLedgerEntry{
			ID:                         newRepositoryID(),
			UserID:                     batch.CreatedBy,
			Type:                       model.CreditLedgerReserve,
			AvailableDeltaMicrocredits: -totalMicrocredits,
			ReservedDeltaMicrocredits:  totalMicrocredits,
			AvailableAfterMicrocredits: account.AvailableMicrocredits,
			ReservedAfterMicrocredits:  account.ReservedMicrocredits,
			RedeemBatchID:              batch.ID,
			Scene:                      "redeem_code",
			Note:                       "生成兑换码批次冻结积分",
		}).Error
	})
	return account, err
}

func (r *Repository) AdminRedeemBatches(keyword string, validity string, lifecycle string, fundingSource model.RedeemBatchFundingSource, creatorID string, limit int, offset int) ([]model.RedeemBatch, int64, error) {
	var items []model.RedeemBatch
	var total int64
	query := r.db.Model(&model.RedeemBatch{}).Where("redeem_batches.funding_source = ?", fundingSource)
	if creatorID != "" {
		query = query.Where("redeem_batches.created_by = ?", creatorID)
	}
	if value := strings.TrimSpace(keyword); value != "" {
		pattern := "%" + strings.ToLower(value) + "%"
		query = query.Where("lower(note) LIKE ? OR CAST(amount_microcredits AS TEXT) LIKE ? OR CAST(count AS TEXT) LIKE ?", pattern, pattern, pattern)
	}
	if validity == "active" {
		query = query.Where("expires_at IS NULL OR expires_at > ?", time.Now())
	} else if validity == "expired" {
		query = query.Where("expires_at IS NOT NULL AND expires_at <= ?", time.Now())
	}
	if lifecycle == "ongoing" {
		query = query.Where("redeem_batches.terminal_at IS NULL")
	} else if lifecycle == "completed" {
		query = query.Where("redeem_batches.terminal_at IS NOT NULL")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	now := time.Now()
	listQuery := query.Select(`redeem_batches.id, redeem_batches.amount_microcredits, redeem_batches.count,
		redeem_batches.note, redeem_batches.created_by, redeem_batches.funding_source, redeem_batches.expires_at,
		redeem_batches.terminal_at, redeem_batches.code_secrets_cleared_at, redeem_batches.created_at,
		users.username AS creator_username,
		(redeem_batches.amount_microcredits * redeem_batches.count) AS total_microcredits,
		(SELECT COUNT(*) FROM redeem_codes rc WHERE rc.batch_id = redeem_batches.id AND rc.status = 'unused' AND (rc.expires_at IS NULL OR rc.expires_at > ?)) AS available_count,
		(SELECT COUNT(*) FROM redeem_codes rc WHERE rc.batch_id = redeem_batches.id AND rc.status = 'redeemed') AS redeemed_count,
		(SELECT COUNT(*) FROM redeem_codes rc WHERE rc.batch_id = redeem_batches.id AND rc.status = 'disabled') AS disabled_count,
		(SELECT COUNT(*) FROM redeem_codes rc WHERE rc.batch_id = redeem_batches.id AND (rc.status = 'expired' OR (rc.status = 'unused' AND rc.expires_at IS NOT NULL AND rc.expires_at <= ?))) AS expired_count`, now, now).
		Joins("LEFT JOIN users ON users.id = redeem_batches.created_by")
	if err := listQuery.Order("redeem_batches.created_at desc").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) RedeemBatch(id string, fundingSource model.RedeemBatchFundingSource, creatorID string) (*model.RedeemBatch, error) {
	var batch model.RedeemBatch
	now := time.Now()
	query := r.db.Model(&model.RedeemBatch{}).Select(`redeem_batches.*,
		users.username AS creator_username,
		(redeem_batches.amount_microcredits * redeem_batches.count) AS total_microcredits,
		(SELECT COUNT(*) FROM redeem_codes rc WHERE rc.batch_id = redeem_batches.id AND rc.status = 'unused' AND (rc.expires_at IS NULL OR rc.expires_at > ?)) AS available_count,
		(SELECT COUNT(*) FROM redeem_codes rc WHERE rc.batch_id = redeem_batches.id AND rc.status = 'redeemed') AS redeemed_count,
		(SELECT COUNT(*) FROM redeem_codes rc WHERE rc.batch_id = redeem_batches.id AND rc.status = 'disabled') AS disabled_count,
		(SELECT COUNT(*) FROM redeem_codes rc WHERE rc.batch_id = redeem_batches.id AND (rc.status = 'expired' OR (rc.status = 'unused' AND rc.expires_at IS NOT NULL AND rc.expires_at <= ?))) AS expired_count`, now, now).
		Joins("LEFT JOIN users ON users.id = redeem_batches.created_by").
		Where("redeem_batches.id = ? AND redeem_batches.funding_source = ?", id, fundingSource)
	if creatorID != "" {
		query = query.Where("redeem_batches.created_by = ?", creatorID)
	}
	if err := query.First(&batch).Error; err != nil {
		return nil, err
	}
	return &batch, nil
}

func (r *Repository) AdminRedeemCodes(batchID string, status string, limit int, offset int) ([]AdminRedeemCodeRow, int64, error) {
	var items []AdminRedeemCodeRow
	var total int64
	query := r.db.Model(&model.RedeemCode{}).Where("redeem_codes.batch_id = ?", batchID)
	now := time.Now()
	switch status {
	case "available":
		query = query.Where("redeem_codes.status = ? AND (redeem_codes.expires_at IS NULL OR redeem_codes.expires_at > ?)", model.RedeemCodeUnused, now)
	case "redeemed":
		query = query.Where("redeem_codes.status = ?", model.RedeemCodeRedeemed)
	case "disabled":
		query = query.Where("redeem_codes.status = ?", model.RedeemCodeDisabled)
	case "expired":
		query = query.Where("redeem_codes.status = ? OR (redeem_codes.status = ? AND redeem_codes.expires_at IS NOT NULL AND redeem_codes.expires_at <= ?)", model.RedeemCodeExpired, model.RedeemCodeUnused, now)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Select("redeem_codes.*, users.username AS redeemed_username, users.username AS redeemed_display_name").
		Joins("LEFT JOIN users ON users.id = redeem_codes.redeemed_by").
		Order("redeem_codes.created_at asc, redeem_codes.id asc").Limit(limit).Offset(offset).Scan(&items).Error
	return items, total, err
}

func (r *Repository) AdminRedeemCodeByHash(codeHash string, fundingSource model.RedeemBatchFundingSource, creatorID string) (*AdminRedeemCodeRow, error) {
	var item AdminRedeemCodeRow
	result := r.db.Model(&model.RedeemCode{}).
		Select("redeem_codes.*, users.username AS redeemed_username, users.username AS redeemed_display_name").
		Joins("LEFT JOIN users ON users.id = redeem_codes.redeemed_by").
		Joins("JOIN redeem_batches ON redeem_batches.id = redeem_codes.batch_id").
		Where("redeem_codes.code_hash = ? AND redeem_batches.funding_source = ?", codeHash, fundingSource)
	if creatorID != "" {
		result = result.Where("redeem_batches.created_by = ?", creatorID)
	}
	result = result.
		Limit(1).
		Scan(&item)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func (r *Repository) AdminRedeemCodesByHashes(codeHashes []string, fundingSource model.RedeemBatchFundingSource, creatorID string) ([]AdminRedeemCodeRow, error) {
	if len(codeHashes) == 0 {
		return []AdminRedeemCodeRow{}, nil
	}
	var items []AdminRedeemCodeRow
	query := r.db.Model(&model.RedeemCode{}).
		Select("redeem_codes.*, users.username AS redeemed_username, users.username AS redeemed_display_name").
		Joins("LEFT JOIN users ON users.id = redeem_codes.redeemed_by").
		Joins("JOIN redeem_batches ON redeem_batches.id = redeem_codes.batch_id").
		Where("redeem_codes.code_hash IN ? AND redeem_batches.funding_source = ?", codeHashes, fundingSource)
	if creatorID != "" {
		query = query.Where("redeem_batches.created_by = ?", creatorID)
	}
	err := query.Scan(&items).Error
	return items, err
}

func (r *Repository) AdminRedeemBatchesWithCodeSecrets(fundingSource model.RedeemBatchFundingSource, creatorID string) ([]model.RedeemBatch, error) {
	var items []model.RedeemBatch
	now := time.Now()
	query := r.db.Model(&model.RedeemBatch{}).Select(`redeem_batches.*,
		users.username AS creator_username,
		(redeem_batches.amount_microcredits * redeem_batches.count) AS total_microcredits,
		(SELECT COUNT(*) FROM redeem_codes rc WHERE rc.batch_id = redeem_batches.id AND rc.status = 'unused' AND (rc.expires_at IS NULL OR rc.expires_at > ?)) AS available_count,
		(SELECT COUNT(*) FROM redeem_codes rc WHERE rc.batch_id = redeem_batches.id AND rc.status = 'redeemed') AS redeemed_count,
		(SELECT COUNT(*) FROM redeem_codes rc WHERE rc.batch_id = redeem_batches.id AND rc.status = 'disabled') AS disabled_count,
		(SELECT COUNT(*) FROM redeem_codes rc WHERE rc.batch_id = redeem_batches.id AND (rc.status = 'expired' OR (rc.status = 'unused' AND rc.expires_at IS NOT NULL AND rc.expires_at <= ?))) AS expired_count`, now, now).
		Joins("LEFT JOIN users ON users.id = redeem_batches.created_by").
		Where("redeem_batches.codes_cipher <> '' AND redeem_batches.funding_source = ?", fundingSource)
	if creatorID != "" {
		query = query.Where("redeem_batches.created_by = ?", creatorID)
	}
	err := query.
		Order("redeem_batches.created_at desc").
		Find(&items).Error
	return items, err
}

func (r *Repository) RedeemCode(userID string, codeHash string, redeemedIP string) (*model.CreditAccount, error) {
	var account model.CreditAccount
	invalid := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var code model.RedeemCode
		codeQuery := tx.Where("code_hash = ?", codeHash)
		if tx.Dialector.Name() == "postgres" {
			codeQuery = codeQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := codeQuery.First(&code).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				invalid = true
				return nil
			}
			return err
		}
		now := time.Now()
		var batch model.RedeemBatch
		if err := redeemBatchForUpdate(tx, &batch, code.BatchID); err != nil {
			return err
		}
		if code.Status != model.RedeemCodeUnused {
			invalid = true
			return nil
		}
		if code.ExpiresAt != nil && !code.ExpiresAt.After(now) {
			if _, _, err := settleRedeemCodes(tx, &batch, []string{code.ID}, model.RedeemCodeExpired, now); err != nil {
				return err
			}
			invalid = true
			return nil
		}
		query := tx.Model(&model.RedeemCode{}).Where("id = ? AND status = ?", code.ID, model.RedeemCodeUnused)
		if code.ExpiresAt != nil {
			query = query.Where("expires_at > ?", now)
		}
		updated := query.Updates(map[string]any{"status": model.RedeemCodeRedeemed, "redeemed_by": userID, "redeemed_at": &now, "redeemed_ip": redeemedIP, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			invalid = true
			return nil
		}
		if batch.FundingSource == model.RedeemBatchFundingModuleAdmin {
			settled := tx.Model(&model.CreditAccount{}).
				Where("user_id = ? AND reserved_microcredits >= ?", batch.CreatedBy, code.AmountMicrocredits).
				Updates(map[string]any{
					"reserved_microcredits": gorm.Expr("reserved_microcredits - ?", code.AmountMicrocredits),
					"version":               gorm.Expr("version + 1"),
					"updated_at":            now,
				})
			if settled.Error != nil {
				return settled.Error
			}
			if settled.RowsAffected != 1 {
				return errors.New("兑换码创建者冻结积分不一致")
			}
			var creatorAccount model.CreditAccount
			if err := tx.First(&creatorAccount, "user_id = ?", batch.CreatedBy).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.CreditLedgerEntry{
				ID: newRepositoryID(), UserID: batch.CreatedBy, Type: model.CreditLedgerConsume,
				AmountMicrocredits: -code.AmountMicrocredits, ReservedDeltaMicrocredits: -code.AmountMicrocredits,
				AvailableAfterMicrocredits: creatorAccount.AvailableMicrocredits, ReservedAfterMicrocredits: creatorAccount.ReservedMicrocredits,
				RedeemBatchID: batch.ID, RedeemCodeID: code.ID, Scene: "redeem_code", Note: "兑换码已核销",
			}).Error; err != nil {
				return err
			}
		}
		account = model.CreditAccount{UserID: userID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&account).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CreditAccount{}).Where("user_id = ?", userID).Updates(map[string]any{
			"available_microcredits": gorm.Expr("available_microcredits + ?", code.AmountMicrocredits),
			"version":                gorm.Expr("version + 1"),
			"updated_at":             now,
		}).Error; err != nil {
			return err
		}
		if err := tx.First(&account, "user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.CreditLedgerEntry{
			ID:                         newRepositoryID(),
			UserID:                     userID,
			Type:                       model.CreditLedgerRedeem,
			AmountMicrocredits:         code.AmountMicrocredits,
			AvailableDeltaMicrocredits: code.AmountMicrocredits,
			AvailableAfterMicrocredits: account.AvailableMicrocredits,
			ReservedAfterMicrocredits:  account.ReservedMicrocredits,
			RedeemCodeID:               code.ID,
			RedeemBatchID:              batch.ID,
			Note:                       "兑换码充值",
		}).Error; err != nil {
			return err
		}
		return markRedeemBatchTerminalIfSettled(tx, &batch, now)
	})
	if err == nil && invalid {
		return nil, ErrRedeemCodeInvalid
	}
	return &account, err
}

func settleRedeemCodes(tx *gorm.DB, batch *model.RedeemBatch, codeIDs []string, status model.RedeemCodeStatus, now time.Time) (int64, int64, error) {
	if len(codeIDs) == 0 {
		return 0, 0, nil
	}
	updated := tx.Model(&model.RedeemCode{}).
		Where("id IN ? AND batch_id = ? AND status = ?", codeIDs, batch.ID, model.RedeemCodeUnused).
		Updates(map[string]any{"status": status, "updated_at": now})
	if updated.Error != nil || updated.RowsAffected == 0 {
		return updated.RowsAffected, 0, updated.Error
	}
	if err := markRedeemBatchTerminalIfSettled(tx, batch, now); err != nil {
		return 0, 0, err
	}
	if batch.FundingSource != model.RedeemBatchFundingModuleAdmin {
		return updated.RowsAffected, 0, nil
	}
	if batch.AmountMicrocredits <= 0 || updated.RowsAffected > (1<<63-1)/batch.AmountMicrocredits {
		return 0, 0, errors.New("兑换码退款金额无效")
	}
	refunded := updated.RowsAffected * batch.AmountMicrocredits
	accountUpdate := tx.Model(&model.CreditAccount{}).
		Where("user_id = ? AND reserved_microcredits >= ?", batch.CreatedBy, refunded).
		Updates(map[string]any{
			"available_microcredits": gorm.Expr("available_microcredits + ?", refunded),
			"reserved_microcredits":  gorm.Expr("reserved_microcredits - ?", refunded),
			"version":                gorm.Expr("version + 1"),
			"updated_at":             now,
		})
	if accountUpdate.Error != nil {
		return 0, 0, accountUpdate.Error
	}
	if accountUpdate.RowsAffected != 1 {
		return 0, 0, errors.New("兑换码创建者冻结积分不一致")
	}
	var account model.CreditAccount
	if err := tx.First(&account, "user_id = ?", batch.CreatedBy).Error; err != nil {
		return 0, 0, err
	}
	note := "兑换码禁用退回"
	if status == model.RedeemCodeExpired {
		note = "兑换码过期退回"
	}
	entry := model.CreditLedgerEntry{
		ID: newRepositoryID(), UserID: batch.CreatedBy, Type: model.CreditLedgerRefund,
		AmountMicrocredits: refunded, AvailableDeltaMicrocredits: refunded, ReservedDeltaMicrocredits: -refunded,
		AvailableAfterMicrocredits: account.AvailableMicrocredits, ReservedAfterMicrocredits: account.ReservedMicrocredits,
		RedeemBatchID: batch.ID, Scene: "redeem_code", Note: note,
	}
	if updated.RowsAffected == 1 {
		entry.RedeemCodeID = codeIDs[0]
	}
	if err := tx.Create(&entry).Error; err != nil {
		return 0, 0, err
	}
	return updated.RowsAffected, refunded, nil
}

func markRedeemBatchTerminalIfSettled(tx *gorm.DB, batch *model.RedeemBatch, now time.Time) error {
	var remaining int64
	if err := tx.Model(&model.RedeemCode{}).
		Where("batch_id = ? AND status = ?", batch.ID, model.RedeemCodeUnused).
		Count(&remaining).Error; err != nil {
		return err
	}
	if remaining > 0 {
		return nil
	}
	terminalAt := now
	if batch.ExpiresAt != nil && batch.ExpiresAt.Before(terminalAt) {
		terminalAt = *batch.ExpiresAt
	}
	return tx.Model(&model.RedeemBatch{}).
		Where("id = ? AND terminal_at IS NULL", batch.ID).
		Update("terminal_at", terminalAt).Error
}

func redeemBatchForUpdate(tx *gorm.DB, batch *model.RedeemBatch, batchID string) error {
	query := tx.Where("id = ?", batchID)
	if tx.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return query.First(batch).Error
}

func (r *Repository) ClearTerminalRedeemBatchSecrets(cutoff time.Time, clearedAt time.Time, limit int) (int64, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var ids []string
	if err := r.db.Model(&model.RedeemBatch{}).
		Where("terminal_at IS NOT NULL AND terminal_at <= ? AND codes_cipher <> ''", cutoff).
		Order("terminal_at asc, id asc").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Model(&model.RedeemBatch{}).
		Where("id IN ? AND terminal_at IS NOT NULL AND terminal_at <= ? AND codes_cipher <> ''", ids, cutoff).
		Updates(map[string]any{"codes_cipher": "", "code_secrets_cleared_at": clearedAt})
	return result.RowsAffected, result.Error
}

func (r *Repository) SettleExpiredRedeemCodes(now time.Time, creatorID string, batchLimit int) (int64, int64, error) {
	if batchLimit <= 0 || batchLimit > 500 {
		batchLimit = 100
	}
	query := r.db.Model(&model.RedeemBatch{}).
		Distinct("redeem_batches.id").
		Joins("JOIN redeem_codes ON redeem_codes.batch_id = redeem_batches.id").
		Where("redeem_codes.status = ? AND redeem_codes.expires_at IS NOT NULL AND redeem_codes.expires_at <= ?", model.RedeemCodeUnused, now)
	if creatorID != "" {
		query = query.Where("redeem_batches.funding_source = ? AND redeem_batches.created_by = ?", model.RedeemBatchFundingModuleAdmin, creatorID)
	}
	var batchIDs []string
	if err := query.Order("redeem_batches.id").Limit(batchLimit).Pluck("redeem_batches.id", &batchIDs).Error; err != nil {
		return 0, 0, err
	}
	var expiredTotal int64
	var refundedTotal int64
	for _, batchID := range batchIDs {
		expired, refunded, err := r.settleExpiredRedeemBatch(batchID, now)
		if err != nil {
			return expiredTotal, refundedTotal, err
		}
		expiredTotal += expired
		refundedTotal += refunded
	}
	return expiredTotal, refundedTotal, nil
}

func (r *Repository) settleExpiredRedeemBatch(batchID string, now time.Time) (int64, int64, error) {
	var expired int64
	var refunded int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var batch model.RedeemBatch
		batchQuery := tx.Where("id = ?", batchID)
		if tx.Dialector.Name() == "postgres" {
			batchQuery = batchQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := batchQuery.First(&batch).Error; err != nil {
			return err
		}
		var codeIDs []string
		if err := tx.Model(&model.RedeemCode{}).
			Where("batch_id = ? AND status = ? AND expires_at IS NOT NULL AND expires_at <= ?", batch.ID, model.RedeemCodeUnused, now).
			Pluck("id", &codeIDs).Error; err != nil {
			return err
		}
		var err error
		expired, refunded, err = settleRedeemCodes(tx, &batch, codeIDs, model.RedeemCodeExpired, now)
		return err
	})
	return expired, refunded, err
}

func (r *Repository) HasActiveFundedRedeemCodes(creatorID string, now time.Time) (bool, error) {
	var count int64
	err := r.db.Model(&model.RedeemCode{}).
		Joins("JOIN redeem_batches ON redeem_batches.id = redeem_codes.batch_id").
		Where("redeem_batches.funding_source = ? AND redeem_batches.created_by = ?", model.RedeemBatchFundingModuleAdmin, creatorID).
		Where("redeem_codes.status = ? AND (redeem_codes.expires_at IS NULL OR redeem_codes.expires_at > ?)", model.RedeemCodeUnused, now).
		Limit(1).Count(&count).Error
	return count > 0, err
}
