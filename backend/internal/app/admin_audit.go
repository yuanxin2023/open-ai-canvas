package app

import (
	"encoding/json"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

type AdminUserDetail struct {
	User             AdminManagedUser            `json:"user"`
	RegistrationIP   string                      `json:"registrationIp"`
	Account          model.CreditAccount         `json:"account"`
	Counts           repository.AdminUserCounts  `json:"counts"`
	StorageUsage     repository.UserStorageUsage `json:"storageUsage"`
	StoredFileBytes  int64                       `json:"storedFileBytes"`
	DailyUploadBytes int64                       `json:"dailyUploadBytes"`
	Quota            RuntimeResourcePolicy       `json:"quota"`
}

type AdminLoginEventPage struct {
	Events []model.UserLoginEvent `json:"events"`
	Total  int64                  `json:"total"`
	Page   int                    `json:"page"`
	Limit  int                    `json:"pageSize"`
}

type AdminUserLoginEventQuery struct {
	Page        int
	Limit       int
	StartAt     string
	EndAt       string
	LoginMethod string
	IP          string
}

type AdminTaskPage struct {
	Tasks []model.Task `json:"tasks"`
	Total int64        `json:"total"`
	Page  int          `json:"page"`
	Limit int          `json:"pageSize"`
}

type AdminAuditPage struct {
	Events []model.AdminAuditEvent `json:"events"`
	Total  int64                   `json:"total"`
	Page   int                     `json:"page"`
	Limit  int                     `json:"pageSize"`
}

func (s *Service) appendAdminAudit(actor *model.User, action string, targetType string, targetID string, summary string, metadata any) error {
	event, err := newAdminAuditEvent(actor, action, targetType, targetID, summary, metadata)
	if err != nil {
		return err
	}
	return s.repo.AppendAdminAudit(event)
}

func newAdminAuditEvent(actor *model.User, action string, targetType string, targetID string, summary string, metadata any) (*model.AdminAuditEvent, error) {
	if actor == nil {
		return nil, Unauthorized("请先登录")
	}
	encoded := ""
	if metadata != nil {
		data, err := json.Marshal(metadata)
		if err != nil {
			return nil, err
		}
		encoded = string(data)
	}
	return &model.AdminAuditEvent{
		ID: newID(), ActorUserID: actor.ID, Action: strings.TrimSpace(action), TargetType: strings.TrimSpace(targetType),
		TargetID: strings.TrimSpace(targetID), Summary: truncateRunes(strings.TrimSpace(summary), 500), MetadataJSON: encoded, CreatedAt: time.Now(),
	}, nil
}

func (s *Service) manageableAdminUser(actor *model.User, userID string) (*model.User, error) {
	user, err := s.repo.User(strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	if user.Role == model.UserRoleAdmin {
		if err := s.RequireFullAdmin(actor); err != nil {
			return nil, adminPermissionDenied("只有全权限管理员可以查看或管理其他管理员")
		}
		if err := s.repo.HydrateAdminAccess(user); err != nil {
			return nil, err
		}
	}
	return user, nil
}

func (s *Service) AdminUserDetail(actor *model.User, userID string) (*AdminUserDetail, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	user, err := s.manageableAdminUser(actor, userID)
	if err != nil {
		return nil, err
	}
	account, err := s.repo.CreditAccount(user.ID)
	if err != nil {
		return nil, err
	}
	counts, err := s.repo.AdminUserCounts(user.ID)
	if err != nil {
		return nil, err
	}
	usage, err := s.repo.UserStorageUsage(user.ID)
	if err != nil {
		return nil, err
	}
	storedFileBytes, err := s.repo.UserStoredFileBytes(user.ID)
	if err != nil {
		return nil, err
	}
	dailyUploadBytes, err := s.repo.DailyUploadBytes(user.ID, time.Now().UTC().Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	policy, err := s.RuntimePolicy()
	if err != nil {
		return nil, err
	}
	return &AdminUserDetail{
		User: AdminManagedUser{User: *user, Remark: user.AdminRemark, AdminAccess: adminAccessView(user)}, RegistrationIP: user.RegistrationIP, Account: *account, Counts: counts, StorageUsage: usage,
		StoredFileBytes: storedFileBytes, DailyUploadBytes: dailyUploadBytes, Quota: policy.Resource,
	}, nil
}

func (s *Service) AdminUserLoginEvents(actor *model.User, userID string, query AdminUserLoginEventQuery) (*AdminLoginEventPage, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	if _, err := s.manageableAdminUser(actor, userID); err != nil {
		return nil, err
	}
	filter, err := normalizeAdminUserLoginEventFilter(query)
	if err != nil {
		return nil, err
	}
	page, limit := normalizeAdminPage(query.Page, query.Limit)
	events, total, err := s.repo.AdminUserLoginEvents(userID, filter, limit, (page-1)*limit)
	return &AdminLoginEventPage{Events: events, Total: total, Page: page, Limit: limit}, err
}

func normalizeAdminUserLoginEventFilter(query AdminUserLoginEventQuery) (repository.AdminUserLoginEventFilter, error) {
	filter := repository.AdminUserLoginEventFilter{
		LoginMethod: strings.TrimSpace(query.LoginMethod),
		IP:          strings.TrimSpace(query.IP),
	}
	if value := strings.TrimSpace(query.StartAt); value != "" {
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return repository.AdminUserLoginEventFilter{}, BadAuthRequest("startAt 必须是 RFC3339 时间")
		}
		parsed = parsed.UTC()
		filter.StartAt = &parsed
	}
	if value := strings.TrimSpace(query.EndAt); value != "" {
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return repository.AdminUserLoginEventFilter{}, BadAuthRequest("endAt 必须是 RFC3339 时间")
		}
		parsed = parsed.UTC()
		filter.EndAt = &parsed
	}
	if filter.StartAt != nil && filter.EndAt != nil && filter.StartAt.After(*filter.EndAt) {
		return repository.AdminUserLoginEventFilter{}, BadAuthRequest("startAt 不能晚于 endAt")
	}
	return filter, nil
}

func (s *Service) AdminUserLedger(actor *model.User, userID string, entryType string, page int, limit int) (*WalletSummary, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	if _, err := s.manageableAdminUser(actor, userID); err != nil {
		return nil, err
	}
	page, limit = normalizeAdminPage(page, limit)
	account, err := s.repo.CreditAccount(userID)
	if err != nil {
		return nil, err
	}
	entries, total, err := s.repo.CreditLedger(userID, entryType, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	return &WalletSummary{Account: *account, Entries: entries, Total: total, Page: page, Limit: limit}, nil
}

func (s *Service) AdminUserTasks(actor *model.User, userID string, page int, limit int) (*AdminTaskPage, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	if _, err := s.manageableAdminUser(actor, userID); err != nil {
		return nil, err
	}
	page, limit = normalizeAdminPage(page, limit)
	tasks, total, err := s.repo.AdminUserTasks(userID, limit, (page-1)*limit)
	return &AdminTaskPage{Tasks: tasks, Total: total, Page: page, Limit: limit}, err
}

func (s *Service) AdminUserAuditEvents(actor *model.User, userID string, page int, limit int) (*AdminAuditPage, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	if _, err := s.manageableAdminUser(actor, userID); err != nil {
		return nil, err
	}
	page, limit = normalizeAdminPage(page, limit)
	events, total, err := s.repo.AdminAuditEvents("user", userID, limit, (page-1)*limit)
	return &AdminAuditPage{Events: events, Total: total, Page: page, Limit: limit}, err
}
