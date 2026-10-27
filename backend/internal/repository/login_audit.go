package repository

import (
	"crypto/sha256"
	"encoding/hex"

	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
)

type loginAuditRow struct {
	model.LoginLog
	LegacySessionID string `gorm:"column:legacy_session_id" json:"-"`
}

// Historical events stay intact. New events are written only to login_logs;
// namespaced IDs keep archive rows addressable without colliding with new IDs.
func (r *Repository) loginAuditQuery() *gorm.DB {
	current := r.db.Model(&model.LoginLog{}).Select(`id, user_id, username, display_name, account,
        event, method, source, result, ip, user_agent, device_type, browser, browser_version,
        os, os_version, session_ref, reason, message, created_at, '' AS legacy_session_id`)
	historical := r.db.Table("user_login_events AS legacy_events").
		Select(`'legacy:' || legacy_events.id AS id,
            legacy_events.user_id AS user_id,
            COALESCE(audit_users.username, '') AS username,
            COALESCE(audit_users.display_name, '') AS display_name,
            '' AS account, 'login' AS event,
            legacy_events.login_method AS method, 'legacy' AS source, 'success' AS result,
            legacy_events.ip_address AS ip, legacy_events.user_agent AS user_agent,
            legacy_events.device_type AS device_type, legacy_events.browser AS browser,
            legacy_events.browser_version AS browser_version, legacy_events.os AS os,
            legacy_events.os_version AS os_version,
            '' AS session_ref, '' AS reason, '' AS message, legacy_events.created_at AS created_at,
            legacy_events.session_id AS legacy_session_id`).
		Joins("LEFT JOIN users AS audit_users ON audit_users.id = legacy_events.user_id")
	union := r.db.Raw("? UNION ALL ?", current, historical)
	return r.db.Table("(?) AS login_audit", union)
}

func loginAuditModel(row loginAuditRow) model.LoginLog {
	if row.LegacySessionID != "" {
		digest := sha256.Sum256([]byte(row.LegacySessionID))
		row.SessionRef = hex.EncodeToString(digest[:])
	}
	return row.LoginLog
}

func loginAuditPage(query *gorm.DB, limit, offset int) ([]model.LoginLog, int64, error) {
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []loginAuditRow
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	logs := make([]model.LoginLog, 0, len(rows))
	for _, row := range rows {
		logs = append(logs, loginAuditModel(row))
	}
	return logs, total, nil
}
