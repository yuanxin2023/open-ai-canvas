package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"net"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const SessionCookieName = "open_ai_canvas_session"

const sessionMaxAge = 30 * 24 * time.Hour

const usernameChangeLimit = 3
const usernameChangeWindow = 30 * 24 * time.Hour

// AuthError 保留为兼容别名；跨认证域的新代码应直接使用 AppError。
type AuthError = kernel.AppError

type RegisterRequest struct {
	Email     string `json:"email"`
	EmailCode string `json:"emailCode"`
	Password  string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginEnvironment struct {
	IPAddress      string
	UserAgent      string
	DeviceType     string
	Browser        string
	BrowserVersion string
	OS             string
	OSVersion      string
}

type UpdateProfileRequest struct {
	Username         string  `json:"username"`
	AvatarResourceID *string `json:"avatarResourceId"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type PublicAuthSettings struct {
	FirstUser              bool `json:"firstUser"`
	RegistrationEnabled    bool `json:"registrationEnabled"`
	LinuxDOEnabled         bool `json:"linuxdoEnabled"`
	EmailEnabled           bool `json:"emailEnabled"`
	EmailCodeRequired      bool `json:"emailCodeRequired"`
	EmailFirstRegistration bool `json:"emailFirstRegistration"`
}

type AuthSessionResult struct {
	User       AuthUser `json:"user"`
	Session    string   `json:"session"`
	MaxAgeSecs int      `json:"maxAgeSecs"`
}

type AuthUser struct {
	model.User
	AvatarResourceID     string               `json:"avatarResourceId,omitempty"`
	AvatarURL            string               `json:"avatarUrl,omitempty"`
	IdentityProvider     string               `json:"identityProvider,omitempty"`
	IdentityID           string               `json:"identityId,omitempty"`
	IdentityUsername     string               `json:"identityUsername,omitempty"`
	UsernameChangePolicy UsernameChangePolicy `json:"usernameChangePolicy"`
}

type UsernameChangePolicy struct {
	Customized      bool       `json:"customized"`
	Limit           *int       `json:"limit"`
	Used            int        `json:"used"`
	Remaining       *int       `json:"remaining"`
	WindowDays      int        `json:"windowDays"`
	NextAvailableAt *time.Time `json:"nextAvailableAt,omitempty"`
}

func (s *Service) PublicAuthSettings() (*PublicAuthSettings, error) {
	count, err := s.repo.UserCount()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return &PublicAuthSettings{FirstUser: true, RegistrationEnabled: true, LinuxDOEnabled: false, EmailFirstRegistration: true}, nil
	}
	registrationEnabled, err := s.RegistrationEnabled()
	if err != nil {
		return nil, err
	}
	emailEnabled, err := s.EmailEnabled()
	if err != nil {
		return nil, err
	}
	return &PublicAuthSettings{FirstUser: false, RegistrationEnabled: registrationEnabled, LinuxDOEnabled: s.LinuxDOEnabled(), EmailEnabled: emailEnabled, EmailCodeRequired: true, EmailFirstRegistration: true}, nil
}

func (s *Service) Register(req RegisterRequest) (*AuthSessionResult, error) {
	return s.RegisterWithEnvironment(req, LoginEnvironment{})
}

func (s *Service) RegisterWithEnvironment(req RegisterRequest, environment LoginEnvironment) (*AuthSessionResult, error) {
	environment = normalizeLoginEnvironment(environment)
	email := NormalizeEmail(req.Email)
	if email == "" {
		return nil, kernel.BadAuthRequest("请输入邮箱")
	}
	if err := ValidateEmail(email); err != nil {
		return nil, err
	}
	if err := ValidatePassword(req.Password); err != nil {
		return nil, err
	}
	s.registrationMu.Lock()
	defer s.registrationMu.Unlock()
	count, err := s.repo.UserCount()
	if err != nil {
		return nil, err
	}
	var verifiedCode *model.EmailVerificationCode
	if count > 0 {
		registrationEnabled, err := s.RegistrationEnabled()
		if err != nil {
			return nil, err
		}
		if !registrationEnabled {
			return nil, kernel.Forbidden("管理员未开放新用户注册")
		}
		if err := s.validateRegistrationEmailDomain(email); err != nil {
			return nil, err
		}
		verifiedCode, err = s.VerifyRegistrationEmailCode(email, req.EmailCode)
		if err != nil {
			return nil, err
		}
	}
	if _, err := s.repo.UserByEmail(email); err == nil {
		return nil, kernel.BadAuthRequest("邮箱已被注册")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	userID := kernel.NewID()
	user := model.User{
		ID:             userID,
		Email:          email,
		Role:           model.UserRoleUser,
		Status:         model.UserStatusActive,
		PasswordHash:   passwordHash,
		RegistrationIP: environment.IPAddress,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if count == 0 {
		user.Role = model.UserRoleAdmin
	}
	created := false
	for attempt := 0; attempt < 32; attempt++ {
		user.Username = kernel.DefaultLoginUsernameCandidate(email, userID, attempt)
		user.DisplayName = user.Username
		user.ProfileName = user.Username
		if verifiedCode != nil {
			err = s.repo.CreateUserWithEmailVerification(&user, verifiedCode.ID, time.Now())
		} else {
			err = s.repo.Create(&user)
		}
		if err == nil {
			created = true
			break
		}
		if !isUsernameUniqueViolation(err) {
			return nil, err
		}
	}
	if !created {
		return nil, kernel.WrapAppError(500, "无法生成唯一用户名", err)
	}
	if err := s.host.EnsureSignupBonus(user.ID); err != nil {
		return nil, err
	}
	return s.createAuthSession(&user, "email_register", environment)
}

func (s *Service) Login(req LoginRequest) (*AuthSessionResult, error) {
	return s.LoginWithEnvironment(req, LoginEnvironment{})
}

func (s *Service) LoginWithEnvironment(req LoginRequest, environment LoginEnvironment) (*AuthSessionResult, error) {
	environment = normalizeLoginEnvironment(environment)
	account := NormalizeUsername(req.Username)
	user, err := s.repo.UserByAccount(account)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, kernel.Unauthorized("用户名、邮箱或密码不正确")
		}
		return nil, err
	}
	if user.Status != model.UserStatusActive {
		return nil, kernel.Forbidden("该账号已被禁用")
	}
	if !verifyPassword(req.Password, user.PasswordHash) {
		return nil, kernel.Unauthorized("用户名、邮箱或密码不正确")
	}
	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	if err := s.repo.Save(user); err != nil {
		return nil, err
	}
	if err := s.host.EnsureSignupBonus(user.ID); err != nil {
		return nil, err
	}
	s.host.RecordActivity(user.ID, "login", 1)
	return s.createAuthSession(user, "password", environment)
}

func (s *Service) Logout(cookieValue string) error {
	sessionID, _ := parseSessionCookie(cookieValue)
	if sessionID == "" {
		return nil
	}
	return s.repo.DeleteAuthSession(sessionID)
}

func (s *Service) CurrentUser(cookieValue string) (*model.User, error) {
	sessionID, token := parseSessionCookie(cookieValue)
	if sessionID == "" || token == "" {
		return nil, kernel.Unauthorized("请先登录")
	}
	session, err := s.repo.AuthSession(sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, kernel.Unauthorized("登录状态已失效")
		}
		return nil, err
	}
	if time.Now().After(session.ExpiresAt) || session.TokenHash != HashToken(token) {
		if cleanupErr := s.repo.DeleteAuthSession(sessionID); cleanupErr != nil {
			log.Printf("expired auth session cleanup failed: session_id=%s error=%v", sessionID, cleanupErr)
		}
		return nil, kernel.Unauthorized("登录状态已失效")
	}
	user, err := s.repo.User(session.UserID)
	if err != nil {
		return nil, err
	}
	if user.Status != model.UserStatusActive {
		return nil, kernel.Forbidden("该账号已被禁用")
	}
	return user, nil
}

// 认证响应只补充当前用户自己的第三方公开身份，不把身份表或密钥字段暴露给其他列表接口。
func (s *Service) PublicAuthUser(user *model.User) (AuthUser, error) {
	result := AuthUser{User: *user, AvatarResourceID: user.AvatarResourceID}
	result.UsernameChangePolicy = UsernameChangePolicy{Customized: user.UsernameCustomizedAt != nil, WindowDays: 30}
	if user.Role != model.UserRoleAdmin {
		limit := usernameChangeLimit
		usage, err := s.repo.UsernameChangeUsage(user.ID, time.Now().Add(-usernameChangeWindow), int64(limit))
		if err != nil {
			return AuthUser{}, err
		}
		remaining := max(0, limit-int(usage.Used))
		result.UsernameChangePolicy.Limit = &limit
		result.UsernameChangePolicy.Used = int(usage.Used)
		result.UsernameChangePolicy.Remaining = &remaining
		result.UsernameChangePolicy.NextAvailableAt = usage.NextAvailableAt
	}
	if user.AvatarResourceID != "" {
		resource, err := s.repo.ResourceForUser(user.ID, user.AvatarResourceID)
		if err == nil && resource.Status == model.ResourceStatusReady {
			result.AvatarURL = "/api/resources/" + url.PathEscape(resource.ID) + "/file?direct=1"
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return AuthUser{}, err
		}
	}
	identity, err := s.repo.UserIdentityForUser(user.ID, "linuxdo")
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, nil
	}
	if err != nil {
		return AuthUser{}, err
	}
	result.IdentityProvider = identity.Provider
	result.IdentityID = identity.Subject
	result.IdentityUsername = identity.ProviderUsername
	return result, nil
}

func (s *Service) UpdateProfile(user *model.User, req UpdateProfileRequest) (AuthUser, error) {
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return AuthUser{}, kernel.Unauthorized("请先登录")
	}
	stored, err := s.repo.User(user.ID)
	if err != nil {
		return AuthUser{}, err
	}
	if stored.Status != model.UserStatusActive {
		return AuthUser{}, kernel.Forbidden("该账号已被禁用")
	}
	username := NormalizeUsername(req.Username)
	usernameChanged := username != NormalizeUsername(stored.Username)
	if usernameChanged {
		if err := ValidateUsername(username); err != nil {
			return AuthUser{}, err
		}
		if existing, lookupErr := s.repo.UserByUsername(username); lookupErr == nil {
			if existing.ID != stored.ID {
				return AuthUser{}, kernel.BadAuthRequest("用户名已存在")
			}
		} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return AuthUser{}, lookupErr
		}
	} else {
		username = stored.Username
	}
	var avatarResourceID *string
	if req.AvatarResourceID != nil {
		value := strings.TrimSpace(*req.AvatarResourceID)
		avatarResourceID = &value
		if value != "" {
			resource, err := s.repo.ResourceForUser(stored.ID, value)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return AuthUser{}, kernel.BadAuthRequest("头像资源不存在或不属于当前用户")
				}
				return AuthUser{}, err
			}
			if resource.Status != model.ResourceStatusReady || resource.Kind != "image" || !allowedProfileAvatarMIME(resource.MimeType) {
				return AuthUser{}, kernel.BadAuthRequest("头像必须是已上传的 JPG、PNG 或 WebP 图片")
			}
			if resource.Size <= 0 || resource.Size > 2<<20 {
				return AuthUser{}, kernel.BadAuthRequest("头像大小不能超过 2 MB")
			}
		}
	}
	now := time.Now()
	updated, usage, err := s.repo.UpdateUserProfile(repository.UpdateUserProfileInput{
		UserID: stored.ID, Username: username, AvatarResourceID: avatarResourceID,
		ChangeID: kernel.NewID(), Now: now, Window: usernameChangeWindow, Limit: usernameChangeLimit,
	})
	if errors.Is(err, repository.ErrUsernameChangeLimit) {
		retryAfter := 1
		if usage.NextAvailableAt != nil {
			retryAfter = max(1, int(time.Until(*usage.NextAvailableAt).Seconds()+0.999))
		}
		return AuthUser{}, &kernel.AppError{Status: 429, Code: kernel.CodeUsernameChangeLimit, Reason: kernel.ReasonUsernameChangeLimit, Message: "过去 30 天已修改 3 次用户名，请稍后再试", Retryable: true, RetryAfterSeconds: retryAfter}
	}
	if err != nil {
		if isUsernameUniqueViolation(err) {
			return AuthUser{}, kernel.BadAuthRequest("用户名已存在")
		}
		return AuthUser{}, err
	}
	return s.PublicAuthUser(updated)
}

func isUsernameUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "idx_users_username") || strings.Contains(message, "idx_users_username_ci") ||
		(strings.Contains(message, "users.username") && strings.Contains(message, "unique")) ||
		(strings.Contains(message, "username") && strings.Contains(message, "duplicate key"))
}

func (s *Service) ChangePassword(user *model.User, cookieValue string, req ChangePasswordRequest) error {
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return kernel.Unauthorized("请先登录")
	}
	sessionID, token := parseSessionCookie(cookieValue)
	if sessionID == "" || token == "" {
		return kernel.Unauthorized("登录状态已失效")
	}
	session, err := s.repo.AuthSession(sessionID)
	if err != nil || session.UserID != user.ID || time.Now().After(session.ExpiresAt) || session.TokenHash != HashToken(token) {
		return kernel.Unauthorized("登录状态已失效")
	}
	stored, err := s.repo.User(user.ID)
	if err != nil {
		return err
	}
	if stored.Status != model.UserStatusActive {
		return kernel.Forbidden("该账号已被禁用")
	}
	if strings.TrimSpace(stored.PasswordHash) == "" || !verifyPassword(req.CurrentPassword, stored.PasswordHash) {
		return kernel.BadAuthRequest("当前密码不正确")
	}
	if err := ValidatePassword(req.NewPassword); err != nil {
		return err
	}
	if verifyPassword(req.NewPassword, stored.PasswordHash) {
		return kernel.BadAuthRequest("新密码不能与当前密码相同")
	}
	passwordHash, err := HashPassword(req.NewPassword)
	if err != nil {
		return err
	}
	return s.repo.ChangeUserPassword(stored.ID, sessionID, passwordHash, time.Now())
}

func allowedProfileAvatarMIME(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func (s *Service) createAuthSession(user *model.User, loginMethod string, environment LoginEnvironment) (*AuthSessionResult, error) {
	publicUser, err := s.PublicAuthUser(user)
	if err != nil {
		return nil, err
	}
	token := RandomToken()
	now := time.Now()
	session := model.AuthSession{
		ID:        kernel.NewID(),
		UserID:    user.ID,
		TokenHash: HashToken(token),
		ExpiresAt: now.Add(sessionMaxAge),
		CreatedAt: now,
		UpdatedAt: now,
	}
	event := model.UserLoginEvent{
		ID:             kernel.NewID(),
		UserID:         user.ID,
		SessionID:      session.ID,
		LoginMethod:    truncateLoginEnvironment(loginMethod, 32),
		IPAddress:      environment.IPAddress,
		UserAgent:      environment.UserAgent,
		DeviceType:     environment.DeviceType,
		Browser:        environment.Browser,
		BrowserVersion: environment.BrowserVersion,
		OS:             environment.OS,
		OSVersion:      environment.OSVersion,
		CreatedAt:      now,
	}
	if err := s.repo.CreateAuthSessionWithLoginEvent(&session, &event); err != nil {
		return nil, err
	}
	return &AuthSessionResult{User: publicUser, Session: session.ID + "." + token, MaxAgeSecs: int(sessionMaxAge.Seconds())}, nil
}

func normalizeLoginEnvironment(value LoginEnvironment) LoginEnvironment {
	if parsed := net.ParseIP(strings.TrimSpace(value.IPAddress)); parsed != nil {
		value.IPAddress = parsed.String()
	} else {
		value.IPAddress = ""
	}
	value.UserAgent = truncateLoginEnvironment(value.UserAgent, 1024)
	value.DeviceType = truncateLoginEnvironment(value.DeviceType, 32)
	value.Browser = truncateLoginEnvironment(value.Browser, 80)
	value.BrowserVersion = truncateLoginEnvironment(value.BrowserVersion, 40)
	value.OS = truncateLoginEnvironment(value.OS, 80)
	value.OSVersion = truncateLoginEnvironment(value.OSVersion, 40)
	return value
}

func truncateLoginEnvironment(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func verifyPassword(password string, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func RandomToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return kernel.NewID() + kernel.NewID()
	}
	return hex.EncodeToString(b[:])
}

func parseSessionCookie(value string) (string, string) {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func NormalizeUsername(value string) string {
	return kernel.NormalizeLoginUsername(value)
}

func NormalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func NormalizeDisplayName(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	runes := []rune(value)
	if len(runes) > 40 {
		value = string(runes[:40])
	}
	return value
}

func ValidateUsername(value string) error {
	if err := kernel.ValidateLoginUsername(value); err != nil {
		return kernel.BadAuthRequest(err.Error())
	}
	return nil
}

func ValidatePassword(value string) error {
	if len([]rune(value)) < 8 {
		return kernel.BadAuthRequest("密码至少 8 位")
	}
	return nil
}

func ValidateEmail(value string) error {
	if _, err := mail.ParseAddress(value); err != nil {
		return kernel.BadAuthRequest("邮箱格式不正确")
	}
	return nil
}
