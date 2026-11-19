package repository

import "infinite-canvas/backend/internal/model"

func (r *Repository) AdminResourceDeletionAudits(after string) ([]model.AdminAuditEvent, error) {
	var rows []model.AdminAuditEvent
	err := r.db.Where("action = 'resource.delete' AND target_type = 'resource' AND id > ?", after).Order("id").Limit(100).Find(&rows).Error
	return rows, err
}

func (r *Repository) RecordAdminCleanupAudit(event model.AdminAuditEvent) error {
	return r.db.Create(&event).Error
}
