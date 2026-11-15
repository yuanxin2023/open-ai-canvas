package repository

import "infinite-canvas/backend/internal/model"

func (r *Repository) UserLedgerEntry(userID, entryID string) (*model.CreditLedgerEntry, error) {
	var entry model.CreditLedgerEntry
	err := r.db.Where("user_id = ? AND id = ?", userID, entryID).First(&entry).Error
	return &entry, err
}

func (r *Repository) LedgerRedeemCodes(userID string, ids []string) ([]AdminRedeemCodeRow, error) {
	items := []AdminRedeemCodeRow{}
	if len(ids) == 0 {
		return items, nil
	}
	err := r.db.Model(&model.RedeemCode{}).
		Select("redeem_codes.*, users.username AS redeemed_username, users.username AS redeemed_display_name").
		Joins("LEFT JOIN users ON users.id = redeem_codes.redeemed_by").
		Where("redeem_codes.id IN ? AND redeem_codes.redeemed_by = ?", ids, userID).Scan(&items).Error
	return items, err
}

func (r *Repository) LedgerRedeemBatches(ids []string) ([]model.RedeemBatch, error) {
	items := []model.RedeemBatch{}
	if len(ids) == 0 {
		return items, nil
	}
	err := r.db.Where("id IN ?", ids).Find(&items).Error
	return items, err
}
