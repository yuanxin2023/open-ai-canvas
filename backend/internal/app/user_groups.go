// 用户分组仅控制系统渠道与模型的可见范围，不改变积分价格或结算规则。
//
// 目录、报价、任务准入、系统渠道中转和账单预留都从 userGroupAccess 读取同一份权限快照；
// 目录只是读路径，真正的拦截在任务准入和账单预留，客户端提交隐藏渠道 ID 也会失败关闭。

package app

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

const userGroupCacheTTL = 30 * time.Second

var userGroupCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

// userGroupState 是 Service 内的分组缓存；多实例部署下由短 TTL 收敛。
type userGroupState struct {
	mu       sync.RWMutex
	snapshot *userGroupSnapshot
	at       time.Time
}

type userGroupSnapshot struct {
	byCode map[string]*resolvedUserGroup
}

type resolvedUserGroup struct {
	model.UserGroup
	channels map[string]bool
}

// userGroupAccess 是一次请求的分组上下文。bypassVisibility 只放开渠道可见性（管理员和内部能力探测），
// 此上下文不参与价格计算。
type userGroupAccess struct {
	userID           string
	group            *resolvedUserGroup
	bypassVisibility bool
}

// UserGroupView 是管理端分组读模型。
type UserGroupView struct {
	model.UserGroup
	IsDefault  bool     `json:"isDefault"`
	ChannelIDs []string `json:"channelIds"`
	UserCount  int64    `json:"userCount"`
}

// UserGroupRequest 是创建/更新分组的入参。编码创建后不可修改。
type UserGroupRequest struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	BadgeLabel  string   `json:"badgeLabel"`
	BadgeStyle  string   `json:"badgeStyle"`
	SortOrder   int      `json:"sortOrder"`
	AllChannels bool     `json:"allChannels"`
	ChannelIDs  []string `json:"channelIds"`
}

// PublicUserGroup 是用户端会话里的分组展示信息，不含渠道绑定。
type PublicUserGroup struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	BadgeLabel string `json:"badgeLabel"`
	BadgeStyle string `json:"badgeStyle"`
}

func normalizeUserGroupCode(code string) string {
	value := strings.ToLower(strings.TrimSpace(code))
	if value == "" {
		return model.DefaultUserGroupCode
	}
	return value
}

// storedUserGroupCode 是写入 users.user_group 的值；默认分组保存为空串，与历史用户一致。
func storedUserGroupCode(code string) string {
	value := normalizeUserGroupCode(code)
	if value == model.DefaultUserGroupCode {
		return ""
	}
	return value
}

// EnsureDefaultUserGroup 在启动时补齐 default 分组；它默认开放全部系统渠道，旧部署升级后目录不变。
func (s *Service) EnsureDefaultUserGroup() error {
	_, err := s.repo.UserGroupByCode(model.DefaultUserGroupCode)
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	group := &model.UserGroup{
		ID: newID(), Code: model.DefaultUserGroupCode, Name: "默认分组", BadgeStyle: model.UserGroupBadgeNone,
		AllChannels: true,
	}
	if err := s.repo.SaveUserGroupWithChannels(group, nil, true); err != nil {
		return err
	}
	s.invalidateUserGroups()
	return nil
}

func (s *Service) invalidateUserGroups() {
	s.userGroups.mu.Lock()
	s.userGroups.snapshot = nil
	s.userGroups.mu.Unlock()
}

func (s *Service) userGroupSnapshot() (*userGroupSnapshot, error) {
	s.userGroups.mu.RLock()
	if s.userGroups.snapshot != nil && time.Since(s.userGroups.at) < userGroupCacheTTL {
		snapshot := s.userGroups.snapshot
		s.userGroups.mu.RUnlock()
		return snapshot, nil
	}
	s.userGroups.mu.RUnlock()
	groups, err := s.repo.UserGroups()
	if err != nil {
		return nil, err
	}
	bindings, err := s.repo.UserGroupChannels()
	if err != nil {
		return nil, err
	}
	snapshot := &userGroupSnapshot{byCode: make(map[string]*resolvedUserGroup, len(groups))}
	byID := make(map[string]*resolvedUserGroup, len(groups))
	for _, group := range groups {
		resolved := &resolvedUserGroup{UserGroup: group, channels: map[string]bool{}}
		snapshot.byCode[group.Code] = resolved
		byID[group.ID] = resolved
	}
	for _, binding := range bindings {
		if group := byID[binding.GroupID]; group != nil {
			group.channels[binding.ChannelID] = true
		}
	}
	s.userGroups.mu.Lock()
	s.userGroups.snapshot = snapshot
	s.userGroups.at = time.Now()
	s.userGroups.mu.Unlock()
	return snapshot, nil
}

func (s *Service) resolvedUserGroup(code string) (*resolvedUserGroup, error) {
	snapshot, err := s.userGroupSnapshot()
	if err != nil {
		return nil, err
	}
	normalized := normalizeUserGroupCode(code)
	group := snapshot.byCode[normalized]
	if group == nil && normalized == model.DefaultUserGroupCode {
		// 默认分组是旧账号的权限基线：启动时已补齐，这里兜住新库或被并发清空的窗口。
		if ensureErr := s.EnsureDefaultUserGroup(); ensureErr != nil {
			if _, lookupErr := s.repo.UserGroupByCode(model.DefaultUserGroupCode); lookupErr != nil {
				return nil, ensureErr
			}
		}
		s.invalidateUserGroups()
		if snapshot, err = s.userGroupSnapshot(); err != nil {
			return nil, err
		}
		group = snapshot.byCode[normalized]
	}
	if group == nil {
		// 分组被删除前必须清空用户，所以这里只会是配置损坏；失败关闭，不回退到默认分组。
		return nil, Forbidden("当前账号分组配置无效，请联系管理员")
	}
	return group, nil
}

// userGroupAccessForUser 构造登录用户的分组上下文。管理员可见全部渠道。
func (s *Service) userGroupAccessForUser(user *model.User) (*userGroupAccess, error) {
	if user == nil {
		return nil, Unauthorized("请先登录")
	}
	group, err := s.resolvedUserGroup(user.UserGroup)
	if err != nil {
		return nil, err
	}
	return &userGroupAccess{userID: user.ID, group: group, bypassVisibility: user.Role == model.UserRoleAdmin}, nil
}

// userGroupAccess 按用户 ID 读取分组上下文；账单和任务准入使用它，用户不存在时失败关闭。
func (s *Service) userGroupAccess(userID string) (*userGroupAccess, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, Unauthorized("请先登录")
	}
	user, err := s.repo.User(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, Unauthorized("账号不存在或已删除")
	}
	if err != nil {
		return nil, err
	}
	return s.userGroupAccessForUser(user)
}

// systemGroupAccess 只用于不面向具体用户的内部能力探测（模型名称、能力画像）；
// 它放开渠道可见性，不能用于用户任务准入。
func (s *Service) systemGroupAccess() (*userGroupAccess, error) {
	group, err := s.resolvedUserGroup(model.DefaultUserGroupCode)
	if err != nil {
		return nil, err
	}
	return &userGroupAccess{group: group, bypassVisibility: true}, nil
}

func (a *userGroupAccess) allowsChannel(channelID string) bool {
	if a == nil || a.group == nil {
		return false
	}
	if a.bypassVisibility || a.group.AllChannels {
		return true
	}
	return a.group.channels[strings.TrimSpace(channelID)]
}

func (a *userGroupAccess) requireChannel(channelID string) error {
	if a.allowsChannel(channelID) {
		return nil
	}
	return ModelCatalogMismatch("当前账号分组无权使用该模型，请刷新后重新选择")
}

func (a *userGroupAccess) groupCode() string {
	if a == nil || a.group == nil {
		return ""
	}
	return a.group.Code
}

// PublicUserGroupFor 返回会话展示用的分组；分组缺失时返回 nil，不阻断登录态读取。
func (s *Service) PublicUserGroupFor(user *model.User) *PublicUserGroup {
	if user == nil {
		return nil
	}
	group, err := s.resolvedUserGroup(user.UserGroup)
	if err != nil {
		return nil
	}
	return &PublicUserGroup{Code: group.Code, Name: group.Name, BadgeLabel: group.BadgeLabel, BadgeStyle: group.BadgeStyle}
}

// requireExistingUserGroup 校验管理员给用户分配的分组存在，返回写入 users 表的编码。
func (s *Service) requireExistingUserGroup(code string) (string, error) {
	normalized := normalizeUserGroupCode(code)
	if normalized == model.DefaultUserGroupCode {
		if _, err := s.resolvedUserGroup(normalized); err != nil {
			return "", err
		}
		return "", nil
	}
	if _, err := s.repo.UserGroupByCode(normalized); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", BadAuthRequest("用户分组不存在")
		}
		return "", err
	}
	return storedUserGroupCode(normalized), nil
}

func (s *Service) AdminUserGroups(actor *model.User) ([]UserGroupView, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUserGroups); err != nil {
		return nil, err
	}
	groups, err := s.repo.UserGroups()
	if err != nil {
		return nil, err
	}
	bindings, err := s.repo.UserGroupChannels()
	if err != nil {
		return nil, err
	}
	counts, err := s.repo.UserGroupUserCounts()
	if err != nil {
		return nil, err
	}
	channelsByGroup := make(map[string][]string, len(groups))
	for _, binding := range bindings {
		channelsByGroup[binding.GroupID] = append(channelsByGroup[binding.GroupID], binding.ChannelID)
	}
	result := make([]UserGroupView, 0, len(groups))
	for _, group := range groups {
		count := counts[group.Code]
		if group.Code == model.DefaultUserGroupCode {
			count += counts[""]
		}
		channelIDs := channelsByGroup[group.ID]
		if channelIDs == nil {
			channelIDs = []string{}
		}
		result = append(result, UserGroupView{UserGroup: group, IsDefault: group.Code == model.DefaultUserGroupCode, ChannelIDs: channelIDs, UserCount: count})
	}
	return result, nil
}

func (s *Service) CreateUserGroup(actor *model.User, req UserGroupRequest) (*UserGroupView, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUserGroups); err != nil {
		return nil, err
	}
	code := strings.ToLower(strings.TrimSpace(req.Code))
	if !userGroupCodePattern.MatchString(code) {
		return nil, BadAuthRequest("分组编码只能使用小写字母、数字、下划线或连字符，最多 40 位")
	}
	if _, err := s.repo.UserGroupByCode(code); err == nil {
		return nil, BadAuthRequest("分组编码已存在")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	group := &model.UserGroup{ID: newID(), Code: code}
	channelIDs, err := s.applyUserGroupRequest(group, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveUserGroupWithChannels(group, channelIDs, true); err != nil {
		return nil, err
	}
	s.invalidateUserGroups()
	if err := s.appendAdminAudit(actor, "user_group.create", "user_group", group.ID, "创建用户分组", userGroupAuditMetadata(group, channelIDs)); err != nil {
		return nil, err
	}
	return s.adminUserGroupView(actor, group.ID)
}

func (s *Service) UpdateUserGroup(actor *model.User, id string, req UserGroupRequest) (*UserGroupView, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUserGroups); err != nil {
		return nil, err
	}
	group, err := s.repo.UserGroup(strings.TrimSpace(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("用户分组不存在")
	}
	if err != nil {
		return nil, err
	}
	if code := strings.ToLower(strings.TrimSpace(req.Code)); code != "" && code != group.Code {
		return nil, BadAuthRequest("分组编码创建后不能修改")
	}
	channelIDs, err := s.applyUserGroupRequest(group, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveUserGroupWithChannels(group, channelIDs, false); err != nil {
		return nil, err
	}
	s.invalidateUserGroups()
	if err := s.appendAdminAudit(actor, "user_group.update", "user_group", group.ID, "更新用户分组", userGroupAuditMetadata(group, channelIDs)); err != nil {
		return nil, err
	}
	return s.adminUserGroupView(actor, group.ID)
}

func (s *Service) DeleteUserGroup(actor *model.User, id string) error {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionUserGroups); err != nil {
		return err
	}
	group, err := s.repo.UserGroup(strings.TrimSpace(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotFound("用户分组不存在")
	}
	if err != nil {
		return err
	}
	if group.Code == model.DefaultUserGroupCode {
		return BadAuthRequest("默认分组不能删除")
	}
	if err := s.repo.DeleteUserGroup(group.ID, group.Code); err != nil {
		if errors.Is(err, repository.ErrUserGroupInUse) {
			return BadAuthRequest("分组下仍有用户，请先把用户调整到其他分组")
		}
		return err
	}
	s.invalidateUserGroups()
	return s.appendAdminAudit(actor, "user_group.delete", "user_group", group.ID, "删除用户分组", map[string]any{"code": group.Code})
}

func (s *Service) adminUserGroupView(actor *model.User, id string) (*UserGroupView, error) {
	groups, err := s.AdminUserGroups(actor)
	if err != nil {
		return nil, err
	}
	for index := range groups {
		if groups[index].ID == id {
			return &groups[index], nil
		}
	}
	return nil, NotFound("用户分组不存在")
}

// applyUserGroupRequest 校验并写入可编辑字段，返回去重后的渠道绑定。
func (s *Service) applyUserGroupRequest(group *model.UserGroup, req UserGroupRequest) ([]string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > 80 {
		return nil, BadAuthRequest("分组名称不能为空且不能超过 80 字")
	}
	description := strings.TrimSpace(req.Description)
	if len([]rune(description)) > 500 {
		return nil, BadAuthRequest("分组说明不能超过 500 字")
	}
	badgeLabel := strings.TrimSpace(req.BadgeLabel)
	if len([]rune(badgeLabel)) > 16 {
		return nil, BadAuthRequest("徽章文字不能超过 16 字")
	}
	badgeStyle := strings.ToLower(strings.TrimSpace(req.BadgeStyle))
	switch badgeStyle {
	case "", model.UserGroupBadgeNone:
		badgeStyle = model.UserGroupBadgeNone
	case model.UserGroupBadgeVIP, model.UserGroupBadgeSVIP:
	default:
		return nil, BadAuthRequest("徽章样式无效")
	}
	channelIDs := []string{}
	if !req.AllChannels {
		channelIDs = uniqueNonEmpty(req.ChannelIDs)
		if len(channelIDs) > 0 {
			channels, channelErr := s.repo.SystemChannelsByIDs(channelIDs, true)
			if channelErr != nil {
				return nil, channelErr
			}
			if len(channels) != len(channelIDs) {
				return nil, BadAuthRequest("分组绑定的系统渠道不存在或已删除")
			}
		}
		sort.Strings(channelIDs)
	}
	group.Name = name
	group.Description = description
	group.BadgeLabel = badgeLabel
	group.BadgeStyle = badgeStyle
	group.SortOrder = req.SortOrder
	group.AllChannels = req.AllChannels
	return channelIDs, nil
}

func userGroupAuditMetadata(group *model.UserGroup, channelIDs []string) map[string]any {
	return map[string]any{
		"code":        group.Code,
		"allChannels": group.AllChannels, "channelIds": channelIDs, "badgeStyle": group.BadgeStyle,
	}
}

// ModelCatalogForUser 是 HTTP 目录入口：按登录用户的分组过滤渠道，积分价格保持原值。
func (s *Service) GuestModelCatalog(intent *ModelRequestIntent) (*ModelCatalogResponse, error) {
	group, err := s.resolvedUserGroup(model.DefaultUserGroupCode)
	if err != nil {
		return nil, err
	}
	return s.ModelCatalog(&userGroupAccess{group: group}, intent)
}

func (s *Service) ModelCatalogForUser(user *model.User, intent *ModelRequestIntent) (*ModelCatalogResponse, error) {
	access, err := s.userGroupAccessForUser(user)
	if err != nil {
		return nil, err
	}
	return s.ModelCatalog(access, intent)
}

// PublicLogicalModelsForUser 是前台模型目录的 HTTP 入口。
func (s *Service) PublicLogicalModelsForUser(user *model.User, intent *ModelRequestIntent) ([]PublicLogicalModel, error) {
	access, err := s.userGroupAccessForUser(user)
	if err != nil {
		return nil, err
	}
	return s.PublicLogicalModels(access, intent)
}

// RequireUserChannelAccess 供系统渠道中转校验分组权限。
func (s *Service) RequireUserChannelAccess(user *model.User, channelID string) error {
	access, err := s.userGroupAccessForUser(user)
	if err != nil {
		return err
	}
	return access.requireChannel(channelID)
}
