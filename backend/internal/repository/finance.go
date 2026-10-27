package repository

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas/backend/internal/model"
)

// 先抢占唯一业务键再更新账户，确保注册和签到奖励在多实例并发下只入账一次。
func (r *Repository) GrantCreditsOnce(userID string, entryType model.CreditLedgerType, amount int64, referenceKey string, note string) (*model.CreditAccount, bool, error) {
	var account model.CreditAccount
	granted := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		account = model.CreditAccount{UserID: userID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&account).Error; err != nil {
			return err
		}
		entry := model.CreditLedgerEntry{ID: newRepositoryID(), UserID: userID, Type: entryType, AmountMicrocredits: amount, ReferenceKey: &referenceKey, Note: note}
		created := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "reference_key"}}, DoNothing: true}).Create(&entry)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			return tx.First(&account, "user_id = ?", userID).Error
		}
		granted = true
		if err := tx.Model(&model.CreditAccount{}).Where("user_id = ?", userID).Updates(map[string]any{
			"available_microcredits": gorm.Expr("available_microcredits + ?", amount),
			"version":                gorm.Expr("version + 1"),
			"updated_at":             time.Now(),
		}).Error; err != nil {
			return err
		}
		if err := tx.First(&account, "user_id = ?", userID).Error; err != nil {
			return err
		}
		return tx.Model(&entry).Updates(map[string]any{
			"available_delta_microcredits": amount,
			"available_after_microcredits": account.AvailableMicrocredits,
			"reserved_after_microcredits":  account.ReservedMicrocredits,
		}).Error
	})
	return &account, granted, err
}

type AdminRedeemCodeRow struct {
	model.RedeemCode
	RedeemedUsername    string `json:"redeemedUsername" gorm:"column:redeemed_username"`
	RedeemedDisplayName string `json:"redeemedDisplayName" gorm:"column:redeemed_display_name"`
}

type RedeemFundingSnapshot struct {
	AvailableMicrocredits      int64 `gorm:"column:available_microcredits"`
	TotalReservedMicrocredits  int64 `gorm:"column:total_reserved_microcredits"`
	RedeemReservedMicrocredits int64 `gorm:"column:redeem_reserved_microcredits"`
}

func (r *Repository) CreditAccount(userID string) (*model.CreditAccount, error) {
	var account model.CreditAccount
	err := r.db.First(&account, "user_id = ?", userID).Error
	if err == nil {
		return &account, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	account = model.CreditAccount{UserID: userID}
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *Repository) CreditAccounts(userIDs []string) ([]model.CreditAccount, error) {
	if len(userIDs) == 0 {
		return []model.CreditAccount{}, nil
	}
	var accounts []model.CreditAccount
	err := r.db.Where("user_id IN ?", userIDs).Find(&accounts).Error
	return accounts, err
}

func (r *Repository) RedeemFundingSnapshot(userID string, now time.Time) (*RedeemFundingSnapshot, error) {
	if _, err := r.CreditAccount(userID); err != nil {
		return nil, err
	}
	var snapshot RedeemFundingSnapshot
	err := r.db.Raw(`SELECT
		accounts.available_microcredits AS available_microcredits,
		accounts.reserved_microcredits AS total_reserved_microcredits,
		COALESCE((
			SELECT SUM(codes.amount_microcredits)
			FROM redeem_codes AS codes
			JOIN redeem_batches AS batches ON batches.id = codes.batch_id
			WHERE batches.funding_source = ?
				AND batches.created_by = ?
				AND codes.status = ?
				AND (codes.expires_at IS NULL OR codes.expires_at > ?)
		), 0) AS redeem_reserved_microcredits
	FROM credit_accounts AS accounts
	WHERE accounts.user_id = ?`, model.RedeemBatchFundingModuleAdmin, userID, model.RedeemCodeUnused, now, userID).Scan(&snapshot).Error
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *Repository) CreditLedger(userID string, entryType string, limit int, offset int) ([]model.CreditLedgerEntry, int64, error) {
	return r.creditLedger(userID, entryType, limit, offset, false)
}

// WalletCreditLedger keeps audit rows intact while presenting settled billing as one user-facing consumption row.
func (r *Repository) WalletCreditLedger(userID string, entryType string, limit int, offset int) ([]model.CreditLedgerEntry, int64, error) {
	return r.creditLedger(userID, entryType, limit, offset, true)
}

func (r *Repository) creditLedger(userID string, entryType string, limit int, offset int, walletView bool) ([]model.CreditLedgerEntry, int64, error) {
	var items []model.CreditLedgerEntry
	var total int64
	query := r.db.Model(&model.CreditLedgerEntry{}).Where("user_id = ? AND type <> ?", userID, model.CreditLedgerReserve)
	if walletView {
		query = query.Where(`NOT (type = ? AND billing_order_id <> '' AND EXISTS (
			SELECT 1 FROM billing_orders
			WHERE billing_orders.id = credit_ledger_entries.billing_order_id
			AND billing_orders.status = ?
		))`, model.CreditLedgerRefund, model.BillingStatusSettled)
	}
	switch entryType {
	case "income":
		query = query.Where("type IN ?", []model.CreditLedgerType{model.CreditLedgerRedeem, model.CreditLedgerPaymentTopup, model.CreditLedgerAdminGrant, model.CreditLedgerAdminAdjust, model.CreditLedgerSignupBonus, model.CreditLedgerCheckinBonus, model.CreditLedgerReferral})
	case "increase":
		query = query.Where("type IN ?", []model.CreditLedgerType{model.CreditLedgerRedeem, model.CreditLedgerPaymentTopup, model.CreditLedgerRefund, model.CreditLedgerSignupBonus, model.CreditLedgerCheckinBonus, model.CreditLedgerReferral})
	case "consume":
		query = query.Where("type = ?", model.CreditLedgerConsume)
	case "refund":
		query = query.Where("type = ?", model.CreditLedgerRefund)
	case "admin":
		query = query.Where("type IN ?", []model.CreditLedgerType{model.CreditLedgerAdminGrant, model.CreditLedgerAdminAdjust})
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	if err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	if walletView {
		if err := r.enrichWalletLedgerBilling(items); err != nil {
			return nil, 0, err
		}
	}
	return items, total, nil
}

func (r *Repository) enrichWalletLedgerBilling(items []model.CreditLedgerEntry) error {
	ids := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.BillingOrderID == "" {
			continue
		}
		if _, exists := seen[item.BillingOrderID]; exists {
			continue
		}
		seen[item.BillingOrderID] = struct{}{}
		ids = append(ids, item.BillingOrderID)
	}
	if len(ids) == 0 {
		return nil
	}
	var orders []model.BillingOrder
	if err := r.db.Where("id IN ?", ids).Find(&orders).Error; err != nil {
		return err
	}
	byID := make(map[string]model.BillingOrder, len(orders))
	for _, order := range orders {
		byID[order.ID] = order
	}
	for index := range items {
		order, exists := byID[items[index].BillingOrderID]
		if !exists {
			continue
		}
		items[index].BillingMode = order.BillingMode
		items[index].ReservedAmountMicrocredits = order.ReservedAmountMicrocredits
		items[index].ActualAmountMicrocredits = order.ActualAmountMicrocredits
		items[index].RefundedAmountMicrocredits = order.RefundedAmountMicrocredits
		items[index].InputTokens = order.InputTokens
		items[index].OutputTokens = order.OutputTokens
		items[index].CachedTokens = order.CachedTokens
		items[index].UsageAvailable = order.UsageAvailable
	}
	return nil
}

func (r *Repository) CreditLedgerReferenceExists(referenceKey string) (bool, error) {
	var count int64
	err := r.db.Model(&model.CreditLedgerEntry{}).Where("reference_key = ?", referenceKey).Count(&count).Error
	return count > 0, err
}

func (r *Repository) CreateTaskWithCreditReservation(task *model.Task, order *model.BillingOrder, activeTaskLimit int) error {
	return r.Transaction(func(repo *Repository) error {
		tx := repo.db
		if err := repo.requireCurrentCloudAgentParent(task); err != nil {
			return err
		}
		if err := New(tx).requireExclusiveCanvasAgent(task); err != nil {
			return err
		}
		if err := r.requireActiveLogicalModelForTask(tx, task); err != nil {
			return err
		}
		if err := enforceActiveTaskLimit(tx, task.UserID, activeTaskLimit); err != nil {
			return err
		}
		if err := reserveBillingOrder(tx, order); err != nil {
			return err
		}
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		if task.Operation == "cloud_agent" && task.ProjectID != "" {
			repo.canvasChanged(task.UserID, task.ProjectID)
		}
		return nil
	})
}

func (r *Repository) CreateTaskWithActiveLimit(task *model.Task, activeTaskLimit int) error {
	return r.Transaction(func(repo *Repository) error {
		tx := repo.db
		if err := repo.requireCurrentCloudAgentParent(task); err != nil {
			return err
		}
		if err := New(tx).requireExclusiveCanvasAgent(task); err != nil {
			return err
		}
		if err := r.requireActiveLogicalModelForTask(tx, task); err != nil {
			return err
		}
		if err := enforceActiveTaskLimit(tx, task.UserID, activeTaskLimit); err != nil {
			return err
		}
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		if task.Operation == "cloud_agent" && task.ProjectID != "" {
			repo.canvasChanged(task.UserID, task.ProjectID)
		}
		return nil
	})
}

func (r *Repository) RetryTaskWithBilling(userID string, prepared *model.Task, order *model.BillingOrder, activeTaskLimit int) (*model.Task, error) {
	var task model.Task
	taskID := prepared.ID
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := enforceActiveTaskLimit(tx, userID, activeTaskLimit); err != nil {
			return err
		}
		if order != nil {
			if err := reserveBillingOrder(tx, order); err != nil {
				return err
			}
		}
		updates := map[string]any{
			"status": model.TaskStatusQueued, "stage": "等待队列调度", "progress": 5, "error": "", "result_json": "",
			"execution_diagnostic_json": "", "cancellation_source": "", "cancellation_actor_id": "", "cancellation_requested_at": nil,
			"text_draft": "", "started_at": nil, "completed_at": nil,
			"provider_request_id": "", "poll_stage": "", "next_poll_at": nil,
			"provider_cancel_status": "", "provider_cancel_error": "", "provider_cancel_attempts": 0,
			"provider_cancel_requested_at": nil, "provider_cancelled_at": nil, "provider_cancel_next_check_at": nil,
			"route_run":                 gorm.Expr("route_run + ?", 1),
			"logical_model_revision_id": prepared.LogicalModelRevisionID, "route_id": prepared.RouteID,
			"channel_model_id": prepared.ChannelModelID, "input_json": prepared.InputJSON,
			"model": prepared.Model, "provider": prepared.Provider,
			"lease_owner": "", "lease_expires_at": nil, "updated_at": time.Now(),
		}
		if order != nil {
			updates["billing_order_id"] = order.ID
		} else {
			// 免费模式重试必须解除旧订单，避免任务执行阶段复用上一次的计费状态。
			updates["billing_order_id"] = ""
		}
		updated := tx.Model(&model.Task{}).
			Where("id = ? AND user_id = ? AND status IN ?", taskID, userID, []model.TaskStatus{model.TaskStatusFailed, model.TaskStatusCancelled}).
			Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrTaskNotRetryable
		}
		if err := tx.Delete(&model.TaskTextDelta{}, "user_id = ? AND task_id = ?", userID, taskID).Error; err != nil {
			return err
		}
		return tx.First(&task, "id = ? AND user_id = ?", taskID, userID).Error
	})
	return &task, err
}

func enforceActiveTaskLimit(tx *gorm.DB, userID string, activeTaskLimit int) error {
	var count int64
	if err := tx.Model(&model.Task{}).Where("user_id = ? AND status IN ?", userID, []model.TaskStatus{model.TaskStatusQueued, model.TaskStatusRunning}).Count(&count).Error; err != nil {
		return err
	}
	if count >= int64(activeTaskLimit) {
		return ErrActiveTaskLimit
	}
	return nil
}

func newRepositoryID() string {
	return randomRepositorySuffix()
}

func randomRepositorySuffix() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(value[:])
}
