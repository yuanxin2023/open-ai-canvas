package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

type AdminCleanupHistoryReport struct {
	Apply   bool                       `json:"apply"`
	Scanned int                        `json:"scanned"`
	Impact  AdminResourceCleanupImpact `json:"impact"`
	Skipped []AdminCleanupHistorySkip  `json:"skipped"`
}

type AdminCleanupHistorySkip struct {
	ResourceID string `json:"resourceId"`
	Reason     string `json:"reason"`
}

// The maintenance entry point deliberately avoids Service.New: even a preview
// must not bootstrap plugins or create plugin files in the selected data directory.
func CleanupAdminResourceHistory(repo *repository.Repository, apply bool) (AdminCleanupHistoryReport, error) {
	return (&Service{repo: repo}).CleanupAdminResourceHistory(apply)
}

// Maintenance only: successful immutable audits prove both ownership and cause.
// Missing files without this evidence are never interpreted as an admin deletion.
func (s *Service) CleanupAdminResourceHistory(apply bool) (AdminCleanupHistoryReport, error) {
	report := AdminCleanupHistoryReport{Apply: apply, Skipped: []AdminCleanupHistorySkip{}}
	after := ""
	byUser := map[string]map[string]model.Resource{}
	actorByUser := map[string]string{}
	for {
		audits, err := s.repo.AdminResourceDeletionAudits(after)
		if err != nil {
			return report, err
		}
		if len(audits) == 0 {
			break
		}
		for _, audit := range audits {
			after = audit.ID
			report.Scanned++
			var metadata struct {
				UserID     string `json:"userId"`
				DeleteMode string `json:"deleteMode"`
			}
			if json.Unmarshal([]byte(audit.MetadataJSON), &metadata) != nil || strings.TrimSpace(metadata.UserID) == "" || metadata.DeleteMode != "admin_force" {
				report.Skipped = append(report.Skipped, AdminCleanupHistorySkip{audit.TargetID, "审计缺少管理员强制删除及资源归属证据"})
				continue
			}
			if byUser[metadata.UserID] == nil {
				byUser[metadata.UserID] = map[string]model.Resource{}
			}
			byUser[metadata.UserID][audit.TargetID] = model.Resource{ID: audit.TargetID, UserID: metadata.UserID}
			actorByUser[metadata.UserID] = audit.ActorUserID
		}
	}
	users := make([]string, 0, len(byUser))
	for userID := range byUser {
		users = append(users, userID)
	}
	sort.Strings(users)
	for _, userID := range users {
		resources := []model.Resource{}
		ids := []string{}
		for id, resource := range byUser[userID] {
			resources = append(resources, resource)
			ids = append(ids, id)
		}
		sort.Strings(ids)
		var impact AdminResourceCleanupImpact
		err := s.repo.Transaction(func(repo *repository.Repository) error {
			if apply {
				if err := repo.LockUserDataCleanup(userID); err != nil {
					return err
				}
			}
			owners, err := repo.UsersByIDs([]string{userID})
			if err != nil {
				return err
			}
			if _, exists := owners[userID]; !exists {
				return fmt.Errorf("资源所属用户已不存在，保留审计记录")
			}
			current, err := repo.AdminResourcesByIDs(ids)
			if err != nil {
				return err
			}
			existing := map[string]bool{}
			for _, resource := range current {
				existing[resource.ID] = true
				report.Skipped = append(report.Skipped, AdminCleanupHistorySkip{resource.ID, "资源仍存在，保留"})
			}
			blocked, err := s.adminResourceDeletionBlocks(repo, resources, ids)
			if err != nil {
				return err
			}
			candidates := []model.Resource{}
			for _, resource := range resources {
				if existing[resource.ID] {
					continue
				}
				if reason, protected := blocked[resource.ID]; protected {
					report.Skipped = append(report.Skipped, AdminCleanupHistorySkip{resource.ID, reason.Reason})
					continue
				}
				candidates = append(candidates, resource)
			}
			impact, err = s.cleanupAdminResourceDocuments(repo, candidates, apply)
			if err != nil {
				return err
			}
			if !apply || impact == (AdminResourceCleanupImpact{}) {
				return nil
			}
			raw, err := json.Marshal(map[string]any{"resourceIds": ids, "impact": impact})
			if err != nil {
				return err
			}
			return repo.RecordAdminCleanupAudit(model.AdminAuditEvent{ID: newID(), ActorUserID: actorByUser[userID], Action: "resource.cleanup_history", TargetType: "user", TargetID: userID, Summary: "管理员资源历史残留专项清理", MetadataJSON: string(raw), CreatedAt: time.Now()})
		})
		if err != nil {
			report.Skipped = append(report.Skipped, AdminCleanupHistorySkip{strings.Join(ids, ","), fmt.Sprintf("用户批次已回滚：%v", err)})
			continue
		}
		report.Impact.add(impact)
	}
	return report, nil
}
