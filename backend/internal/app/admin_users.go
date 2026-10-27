// 管理端用户管理：列表、创建、更新、删除与批量停用。

package app

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func (s *Service) RequireAdmin(user *model.User) error {
	if user == nil {
		return Unauthorized("请先登录")
	}
	if user.Role != model.UserRoleAdmin {
		return Forbidden("需要管理员权限")
	}
	return nil
}

func (s *Service) AdminUsers(actor *model.User, query AdminListQuery) (*AdminUserPage, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	query.Type = string(model.UserRoleUser)
	return s.adminUserPage(query, false)
}

func (s *Service) AdminReferences(actor *model.User) (*AdminReferenceData, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	result := &AdminReferenceData{
		Users:    []AdminUserReference{},
		Channels: []AdminChannelReference{},
	}
	if actor.AdminLevel == model.AdminLevelFull || actor.AdminLevel == "" ||
		actor.HasAdminPermission(model.AdminPermissionAnalyticsOverview) || actor.HasAdminPermission(model.AdminPermissionAPILogs) ||
		actor.HasAdminPermission(model.AdminPermissionUsers) || actor.HasAdminPermission(model.AdminPermissionAgentLessons) ||
		actor.HasAdminPermission(model.AdminPermissionCredits) || actor.HasAdminPermission(model.AdminPermissionReferrals) || actor.HasAdminPermission(model.AdminPermissionStorageResources) {
		users, err := s.repo.AdminUserReferences("", actor.AdminLevel == model.AdminLevelFull || actor.AdminLevel == "", 100)
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			result.Users = append(result.Users, AdminUserReference{ID: user.ID, Username: user.Username, DisplayName: user.Username})
		}
	}
	if actor.AdminLevel == model.AdminLevelFull || actor.AdminLevel == "" ||
		actor.HasAdminPermission(model.AdminPermissionAnalyticsOverview) || actor.HasAdminPermission(model.AdminPermissionAPILogs) ||
		actor.HasAdminPermission(model.AdminPermissionChannels) || actor.HasAdminPermission(model.AdminPermissionLogicalModels) {
		channels, err := s.repo.AdminSystemChannelReferences()
		if err != nil {
			return nil, err
		}
		channelIDs := make([]string, 0, len(channels))
		for _, channel := range channels {
			channelIDs = append(channelIDs, channel.ID)
		}
		channelModels, err := s.repo.ChannelModelReferences(channelIDs)
		if err != nil {
			return nil, err
		}
		modelsByChannel := make(map[string][]model.ChannelModel, len(channels))
		for _, item := range channelModels {
			modelsByChannel[item.ChannelID] = append(modelsByChannel[item.ChannelID], item)
		}
		for _, channel := range channels {
			items := modelsByChannel[channel.ID]
			models := make([]string, 0, len(items))
			displayNames := make([]string, 0, len(items))
			for _, item := range items {
				if item.Enabled {
					models = append(models, item.ModelKey)
				}
				displayNames = append(displayNames, strings.TrimSpace(item.DisplayName))
			}
			result.Channels = append(result.Channels, AdminChannelReference{ID: channel.ID, Name: channel.Name, Enabled: channel.Enabled, Models: uniqueNonEmpty(models), ModelDisplayNames: uniqueNonEmpty(displayNames)})
		}
	}
	return result, nil
}

func (s *Service) CreateAdminUser(actor *model.User, req CreateAdminUserRequest) (*AdminUser, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	username := normalizeUsername(req.Username)
	email := normalizeEmail(req.Email)
	displayName := username
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if err := validatePassword(req.Password); err != nil {
		return nil, err
	}
	if email != "" {
		if err := validateEmail(email); err != nil {
			return nil, err
		}
	}
	if req.Role != model.UserRoleAdmin && req.Role != model.UserRoleUser {
		return nil, BadAuthRequest("\u7528\u6237\u89d2\u8272\u65e0\u6548")
	}
	var adminPermissions []model.AdminPermission
	adminLevel := model.AdminLevel("")
	if req.Role == model.UserRoleAdmin {
		if err := s.RequireFullAdmin(actor); err != nil {
			return nil, err
		}
		var accessErr error
		adminLevel, adminPermissions, accessErr = normalizeAdminAccess(req.AdminAccess)
		if accessErr != nil {
			return nil, accessErr
		}
	} else if req.AdminAccess != nil {
		return nil, BadAuthRequest("普通用户不能配置管理员权限")
	}
	if req.Status != model.UserStatusActive && req.Status != model.UserStatusDisabled {
		return nil, BadAuthRequest("\u7528\u6237\u72b6\u6001\u65e0\u6548")
	}
	if _, err := s.repo.UserByUsername(username); err == nil {
		return nil, BadAuthRequest("\u7528\u6237\u540d\u5df2\u5b58\u5728")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if email != "" {
		if _, err := s.repo.UserByEmail(email); err == nil {
			return nil, BadAuthRequest("\u90ae\u7bb1\u5df2\u88ab\u6ce8\u518c")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	remark, err := normalizeAdminRemark(req.Remark)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		ID:                   newID(),
		Username:             username,
		Email:                email,
		DisplayName:          displayName,
		ProfileName:          displayName,
		Role:                 req.Role,
		AdminLevel:           adminLevel,
		Status:               req.Status,
		PasswordHash:         passwordHash,
		AdminRemark:          remark,
		UsernameCustomizedAt: &now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.repo.CreateAdminManagedUser(user, adminPermissions, actor.ID); err != nil {
		return nil, err
	}
	if err := s.repo.HydrateAdminAccess(user); err != nil {
		return nil, err
	}
	if err := s.ensureSignupBonus(user.ID); err != nil {
		return nil, err
	}
	if err := s.appendAdminAudit(actor, "user.create", "user", user.ID, "\u521b\u5efa\u7528\u6237\u8d26\u53f7", map[string]any{
		"role": user.Role, "status": user.Status, "adminLevel": user.AdminLevel, "permissions": user.AdminPermissions, "remarkSet": remark != "",
	}); err != nil {
		return nil, err
	}
	account, err := s.repo.CreditAccount(user.ID)
	if err != nil {
		return nil, err
	}
	return &AdminUser{
		User:                  *user,
		Remark:                user.AdminRemark,
		AvailableMicrocredits: account.AvailableMicrocredits,
		ReservedMicrocredits:  account.ReservedMicrocredits,
		AdminAccess:           adminAccessView(user),
	}, nil
}

func (s *Service) UpdateUser(actor *model.User, userID string, req UpdateUserRequest) (*AdminManagedUser, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	user, err := s.repo.User(userID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.HydrateAdminAccess(user); err != nil {
		return nil, err
	}
	previousRole := user.Role
	previousLevel := user.AdminLevel
	previousPermissions := append([]model.AdminPermission(nil), user.AdminPermissions...)
	actorIsFull := actor.AdminLevel == model.AdminLevelFull || actor.AdminLevel == ""
	if user.Role == model.UserRoleAdmin && !actorIsFull {
		return nil, adminPermissionDenied("只有全权限管理员可以管理其他管理员")
	}
	nextRole := user.Role
	if req.Role == model.UserRoleAdmin || req.Role == model.UserRoleUser {
		nextRole = req.Role
	}
	nextStatus := user.Status
	if req.Status == model.UserStatusActive || req.Status == model.UserStatusDisabled {
		nextStatus = req.Status
	}
	if (nextRole != user.Role || req.AdminAccess != nil) && !actorIsFull {
		return nil, adminPermissionDenied("只有全权限管理员可以任命管理员和分配权限")
	}
	if actor.ID == user.ID && (nextRole != user.Role || nextStatus != user.Status || req.AdminAccess != nil) {
		return nil, BadAuthRequest("不能修改当前全权限管理员自己的角色、状态或权限")
	}
	nextLevel := user.AdminLevel
	nextPermissions := append([]model.AdminPermission(nil), user.AdminPermissions...)
	if nextRole == model.UserRoleAdmin {
		if user.Role != model.UserRoleAdmin && req.AdminAccess == nil {
			return nil, BadAuthRequest("任命管理员时必须配置管理员权限")
		}
		if req.AdminAccess != nil {
			nextLevel, nextPermissions, err = normalizeAdminAccess(req.AdminAccess)
			if err != nil {
				return nil, err
			}
		}
	} else {
		if req.AdminAccess != nil {
			return nil, BadAuthRequest("普通用户不能配置管理员权限")
		}
		nextLevel = ""
		nextPermissions = nil
	}
	protectLastFull := user.Role == model.UserRoleAdmin && (user.AdminLevel == model.AdminLevelFull || user.AdminLevel == "") &&
		(nextRole != model.UserRoleAdmin || nextLevel != model.AdminLevelFull || nextStatus != model.UserStatusActive)
	if protectLastFull {
		count, countErr := s.repo.ActiveFullAdminCountExcluding(user.ID)
		if countErr != nil {
			return nil, countErr
		}
		if count == 0 {
			return nil, BadAuthRequest("至少需要保留一个可用的全权限管理员")
		}
	}
	accessChanged := user.Role != nextRole || user.AdminLevel != nextLevel || !adminPermissionsEqual(user.AdminPermissions, nextPermissions)
	statusChanged := user.Status != nextStatus
	user.DisplayName = user.Username
	user.ProfileName = user.Username
	if req.Email != "" {
		email := normalizeEmail(req.Email)
		if err := validateEmail(email); err != nil {
			return nil, err
		}
		existing, err := s.repo.UserByEmail(email)
		if err == nil && existing.ID != user.ID {
			return nil, BadAuthRequest("邮箱已被注册")
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		user.Email = email
	}
	passwordReset := false
	remarkChanged := false
	if req.Remark != nil {
		remark, err := normalizeAdminRemark(*req.Remark)
		if err != nil {
			return nil, err
		}
		remarkChanged = user.AdminRemark != remark
		user.AdminRemark = remark
	}
	if req.Password != "" {
		if err := validatePassword(req.Password); err != nil {
			return nil, err
		}
		hash, err := hashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		user.PasswordHash = hash
		passwordReset = true
	}
	user.Role = nextRole
	user.AdminLevel = nextLevel
	user.Status = nextStatus
	user.UpdatedAt = time.Now()
	if err := s.repo.SaveAdminManagedUser(user, nextPermissions, actor.ID, passwordReset || accessChanged || statusChanged, protectLastFull); errors.Is(err, repository.ErrLastActiveFullAdmin) {
		return nil, BadAuthRequest("至少需要保留一个可用的全权限管理员")
	} else if err != nil {
		return nil, err
	}
	if err := s.repo.HydrateAdminAccess(user); err != nil {
		return nil, err
	}
	auditSummary := "更新用户账号状态或资料"
	if passwordReset {
		auditSummary = "更新用户资料并重置密码"
	}
	if err := s.appendAdminAudit(actor, "user.update", "user", user.ID, auditSummary, map[string]any{
		"role": user.Role, "status": user.Status,
		"adminAccessBefore": map[string]any{"role": previousRole, "level": previousLevel, "permissions": previousPermissions},
		"adminAccessAfter":  map[string]any{"role": user.Role, "level": user.AdminLevel, "permissions": user.AdminPermissions},
		"accessChanged":     accessChanged, "passwordReset": passwordReset, "remarkChanged": remarkChanged,
	}); err != nil {
		return nil, err
	}
	return &AdminManagedUser{User: *user, Remark: user.AdminRemark, AdminAccess: adminAccessView(user)}, nil
}

func (s *Service) DeleteUser(actor *model.User, userID string) error {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return err
	}
	if actor.ID == userID {
		return BadAuthRequest("不能删除当前登录的管理员账号")
	}
	user, err := s.repo.User(userID)
	if err != nil {
		return err
	}
	if err := s.repo.HydrateAdminAccess(user); err != nil {
		return err
	}
	protectLastFull := user.Role == model.UserRoleAdmin && user.AdminLevel == model.AdminLevelFull && user.Status == model.UserStatusActive
	if user.Role == model.UserRoleAdmin {
		if err := s.RequireFullAdmin(actor); err != nil {
			return err
		}
		count, err := s.repo.ActiveFullAdminCountExcluding(user.ID)
		if err != nil {
			return err
		}
		if count == 0 {
			return BadAuthRequest("至少需要保留一个可用的全权限管理员")
		}
	}
	// 有资金流水后必须保留用户主体，删除入口改为停用并清除全部登录态。
	user.Status = model.UserStatusDisabled
	user.UpdatedAt = time.Now()
	permissions := user.AdminPermissions
	if user.AdminLevel == model.AdminLevelFull {
		permissions = nil
	}
	if err := s.repo.SaveAdminManagedUser(user, permissions, actor.ID, true, protectLastFull); errors.Is(err, repository.ErrLastActiveFullAdmin) {
		return BadAuthRequest("至少需要保留一个可用的全权限管理员")
	} else if err != nil {
		return err
	}
	if err := s.repo.DeleteUserTaskTextDeltas(user.ID); err != nil {
		return err
	}
	return s.appendAdminAudit(actor, "user.disable", "user", user.ID, "停用用户并清除登录态", nil)
}

func (s *Service) BulkDisableUsers(actor *model.User, req BulkDisableUsersRequest) (*BulkDisableUsersResult, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(req.UserIDs))
	userIDs := make([]string, 0, len(req.UserIDs))
	for _, rawID := range req.UserIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return nil, BadAuthRequest("用户 ID 无效")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		userIDs = append(userIDs, id)
	}
	if len(userIDs) == 0 {
		return nil, BadAuthRequest("请选择要停用的用户")
	}
	if len(userIDs) > 100 {
		return nil, BadAuthRequest("单次最多停用 100 个用户")
	}
	for _, userID := range userIDs {
		user, userErr := s.repo.User(userID)
		if userErr != nil {
			return nil, userErr
		}
		if user.Role == model.UserRoleAdmin {
			return nil, BadAuthRequest("批量停用只支持普通用户")
		}
	}
	metadata, err := json.Marshal(map[string]any{"userIds": userIDs, "count": len(userIDs)})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	events := make([]model.AdminAuditEvent, 0, len(userIDs))
	for _, userID := range userIDs {
		events = append(events, model.AdminAuditEvent{ID: newID(), ActorUserID: actor.ID, Action: "user.bulk_disable", TargetType: "user", TargetID: userID, Summary: "批量停用用户并清除登录态", MetadataJSON: string(metadata), CreatedAt: now})
	}
	users, err := s.repo.BulkDisableUsers(actor.ID, userIDs, events, now)
	if errors.Is(err, repository.ErrBulkUserNotFound) {
		return nil, BadAuthRequest("部分用户不存在，请刷新列表后重试")
	}
	if errors.Is(err, repository.ErrBulkCurrentAdmin) {
		return nil, BadAuthRequest("不能停用当前登录的管理员账号")
	}
	if errors.Is(err, repository.ErrBulkLastActiveAdmin) {
		return nil, BadAuthRequest("批量操作后至少需要保留一个可用管理员")
	}
	if err != nil {
		return nil, err
	}
	return &BulkDisableUsersResult{Users: users, DisabledCount: len(users)}, nil
}

func (s *Service) Administrators(actor *model.User, query AdminListQuery) (*AdminUserPage, error) {
	if err := s.RequireFullAdmin(actor); err != nil {
		return nil, err
	}
	query.Type = string(model.UserRoleAdmin)
	return s.adminUserPage(query, true)
}

func (s *Service) CreateAdministrator(actor *model.User, req CreateAdministratorRequest) (*AdminUser, error) {
	if err := s.RequireFullAdmin(actor); err != nil {
		return nil, err
	}
	return s.CreateAdminUser(actor, CreateAdminUserRequest{
		Username: req.Username, Email: req.Email, Remark: req.Remark, Password: req.Password,
		Role: model.UserRoleAdmin, Status: req.Status, AdminAccess: req.AdminAccess,
	})
}

func (s *Service) CreateOrdinaryUser(actor *model.User, req CreateOrdinaryUserRequest) (*AdminUser, error) {
	return s.CreateAdminUser(actor, CreateAdminUserRequest{
		Username: req.Username, Email: req.Email, Remark: req.Remark, Password: req.Password,
		Role: model.UserRoleUser, Status: req.Status,
	})
}

func (s *Service) DeleteOrdinaryUser(actor *model.User, userID string) error {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return err
	}
	if _, err := s.requireTargetRole(userID, model.UserRoleUser); err != nil {
		return err
	}
	return s.DeleteUser(actor, userID)
}

func (s *Service) DemoteAdministrator(actor *model.User, userID string) (*AdminManagedUser, error) {
	if err := s.RequireFullAdmin(actor); err != nil {
		return nil, err
	}
	user, err := s.requireTargetRole(userID, model.UserRoleAdmin)
	if err != nil {
		return nil, err
	}
	return s.UpdateUser(actor, user.ID, UpdateUserRequest{Role: model.UserRoleUser})
}

func (s *Service) PromoteAdministrator(actor *model.User, req PromoteAdministratorRequest) (*AdminManagedUser, error) {
	if err := s.RequireFullAdmin(actor); err != nil {
		return nil, err
	}
	user, err := s.requireTargetRole(req.UserID, model.UserRoleUser)
	if err != nil {
		return nil, err
	}
	return s.UpdateUser(actor, user.ID, UpdateUserRequest{Role: model.UserRoleAdmin, AdminAccess: req.AdminAccess})
}

func (s *Service) PurgeAdministrator(actor *model.User, userID string) error {
	if err := s.RequireFullAdmin(actor); err != nil {
		return err
	}
	if _, err := s.requireTargetRole(userID, model.UserRoleAdmin); err != nil {
		return err
	}
	return s.PurgeUser(actor, userID)
}

func (s *Service) PurgeOrdinaryUser(actor *model.User, userID string) error {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return err
	}
	if _, err := s.requireTargetRole(userID, model.UserRoleUser); err != nil {
		return err
	}
	return s.PurgeUser(actor, userID)
}

func (s *Service) PurgeUser(actor *model.User, userID string) error {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return err
	}
	if actor.ID == userID {
		return BadAuthRequest("不能注销当前登录的管理员账号")
	}
	user, err := s.repo.User(userID)
	if err != nil {
		return err
	}
	if pending, err := s.repo.HasPendingReferralReward(userID); err != nil {
		return err
	} else if pending {
		return BadAuthRequest("该用户仍有待审核邀请返利，请先处理后再注销")
	}
	protectLastFull := user.Role == model.UserRoleAdmin && (user.AdminLevel == model.AdminLevelFull || user.AdminLevel == "") && user.Status == model.UserStatusActive
	if user.Role == model.UserRoleAdmin {
		if err := s.RequireFullAdmin(actor); err != nil {
			return err
		}
		count, err := s.repo.ActiveFullAdminCountExcluding(user.ID)
		if err != nil {
			return err
		}
		if count == 0 {
			return BadAuthRequest("至少需要保留一个可用的全权限管理员")
		}
	}
	if err := s.settleExpiredRedeemCodesForScope(user.ID); err != nil {
		return err
	}
	if active, err := s.repo.HasActiveFundedRedeemCodes(user.ID, time.Now()); err != nil {
		return err
	} else if active {
		return BadAuthRequest("该管理员仍有未使用的个人出资兑换码，请先禁用或等待过期结算后再注销")
	}

	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	snapshot, err := s.repo.UserPurgeSnapshot(user.ID)
	if err != nil {
		return err
	}
	for _, taskID := range snapshot.TaskIDs {
		s.cancelActiveTask(taskID)
	}
	resourceIDs := make([]string, 0, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		resourceIDs = append(resourceIDs, resource.ID)
	}
	if len(s.appearanceResourceReferences(resourceIDs)) > 0 || len(s.customerServiceResourceReferences(resourceIDs)) > 0 {
		return BadAuthRequest("该用户的资源仍被平台外观或客服配置使用，请先替换相关资源后再注销")
	}
	physicalObjects := make(map[string]*model.Resource, len(snapshot.Resources))
	for index := range snapshot.Resources {
		resource := &snapshot.Resources[index]
		if strings.TrimSpace(resource.ObjectKey) == "" {
			continue
		}
		sharedCount, countErr := s.repo.ResourceStorageReferenceCount(resource, resourceIDs)
		if countErr != nil {
			return countErr
		}
		if sharedCount == 0 {
			physicalObjects[resourceStorageIdentity(resource)] = resource
		}
	}
	metadata, err := json.Marshal(map[string]any{"resourceCount": len(snapshot.Resources), "taskCount": len(snapshot.TaskIDs)})
	if err != nil {
		return err
	}
	deletionJobs := resourceDeletionJobs(user.ID, physicalObjects)
	audit := model.AdminAuditEvent{
		ID: newID(), ActorUserID: actor.ID, Action: "user.purge", TargetType: "user", TargetID: user.ID,
		Summary: "注销用户并清理全部用户数据", MetadataJSON: string(metadata), CreatedAt: time.Now(),
	}
	if err := s.repo.PurgeUserData(user.ID, resourceIDs, deletionJobs, audit, protectLastFull); err != nil {
		if errors.Is(err, repository.ErrUserPurgePendingReferral) {
			return BadAuthRequest("该用户仍有待审核邀请返利，请先处理后再注销")
		}
		if errors.Is(err, repository.ErrUserPurgeChanged) {
			return BadAuthRequest("用户数据正在变化，请稍后重试注销")
		}
		if errors.Is(err, repository.ErrLastActiveFullAdmin) {
			return BadAuthRequest("至少需要保留一个可用的全权限管理员")
		}
		return err
	}
	if len(deletionJobs) > 0 {
		s.runWorkerTask(func() { s.drainResourceDeletionJobs(len(deletionJobs)) })
	}
	return nil
}

func (s *Service) SearchAdminUserReferences(actor *model.User, keyword string, limit int) ([]AdminUserReference, error) {
	if err := s.RequireAnyAdminPermission(actor, model.AdminPermissionUsers, model.AdminPermissionAnalyticsOverview, model.AdminPermissionAPILogs,
		model.AdminPermissionAgentLessons, model.AdminPermissionCredits, model.AdminPermissionReferrals, model.AdminPermissionStorageResources); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	limit = min(limit, 100)
	users, err := s.repo.AdminUserReferences(keyword, actor.AdminLevel == model.AdminLevelFull || actor.AdminLevel == "", limit)
	if err != nil {
		return nil, err
	}
	includeBalances := actor.AdminLevel == model.AdminLevelFull || actor.AdminLevel == "" || actor.HasAdminPermission(model.AdminPermissionCredits)
	accountByUserID := map[string]model.CreditAccount{}
	if includeBalances {
		ids := make([]string, 0, len(users))
		for _, user := range users {
			ids = append(ids, user.ID)
		}
		accounts, accountErr := s.repo.CreditAccounts(ids)
		if accountErr != nil {
			return nil, accountErr
		}
		for _, account := range accounts {
			accountByUserID[account.UserID] = account
		}
	}
	result := make([]AdminUserReference, 0, len(users))
	for _, user := range users {
		item := AdminUserReference{ID: user.ID, Username: user.Username, DisplayName: user.Username}
		if account, exists := accountByUserID[user.ID]; exists {
			available, reserved := account.AvailableMicrocredits, account.ReservedMicrocredits
			item.AvailableMicrocredits = &available
			item.ReservedMicrocredits = &reserved
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Service) UpdateAdministrator(actor *model.User, userID string, req UpdateAdministratorRequest) (*AdminManagedUser, error) {
	if err := s.RequireFullAdmin(actor); err != nil {
		return nil, err
	}
	user, err := s.requireTargetRole(userID, model.UserRoleAdmin)
	if err != nil {
		return nil, err
	}
	return s.UpdateUser(actor, user.ID, UpdateUserRequest{
		Email: req.Email, Remark: req.Remark, Password: req.Password, Role: model.UserRoleAdmin,
		Status: req.Status, AdminAccess: req.AdminAccess,
	})
}

func (s *Service) UpdateOrdinaryUser(actor *model.User, userID string, req UpdateOrdinaryUserRequest) (*AdminManagedUser, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUsers); err != nil {
		return nil, err
	}
	user, err := s.requireTargetRole(userID, model.UserRoleUser)
	if err != nil {
		return nil, err
	}
	return s.UpdateUser(actor, user.ID, UpdateUserRequest{
		Email: req.Email, Remark: req.Remark, Password: req.Password, Role: model.UserRoleUser, Status: req.Status,
	})
}

func (s *Service) adminUserPage(query AdminListQuery, hydrateAdminAccess bool) (*AdminUserPage, error) {
	page, limit := normalizeAdminPage(query.Page, query.Limit)
	users, total, err := s.repo.AdminUsers(query.Keyword, model.UserRole(query.Type), model.UserStatus(query.Status), limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	accounts, err := s.repo.CreditAccounts(userIDs)
	if err != nil {
		return nil, err
	}
	accountByUserID := make(map[string]model.CreditAccount, len(accounts))
	for _, account := range accounts {
		accountByUserID[account.UserID] = account
	}
	result := make([]AdminUser, 0, len(users))
	for _, user := range users {
		if hydrateAdminAccess {
			if err := s.repo.HydrateAdminAccess(&user); err != nil {
				return nil, err
			}
		}
		account := accountByUserID[user.ID]
		result = append(result, AdminUser{User: user, Remark: user.AdminRemark, AvailableMicrocredits: account.AvailableMicrocredits, ReservedMicrocredits: account.ReservedMicrocredits, AdminAccess: adminAccessView(&user)})
	}
	return &AdminUserPage{Users: result, Total: total, Page: page, Limit: limit}, nil
}
