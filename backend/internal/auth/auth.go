package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"infinite-canvas/backend/internal/kernel"
	"log"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const SessionCookieName = "open_ai_canvas_session"

const sessionMaxAge = 30 * 24 * time.Hour

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

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
	AvatarResourceID string `json:"avatarResourceId,omitempty"`
	AvatarURL        string `json:"avatarUrl,omitempty"`
	IdentityProvider string `json:"identityProvider,omitempty"`
	IdentityID       string `json:"identityId,omitempty"`
	IdentityUsername string `json:"identityUsername,omitempty"`
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
		ID:           userID,
		Username:     userID,
		Email:        email,
		DisplayName:  registrationDisplayName(email, userID),
		Role:         model.UserRoleUser,
		Status:       model.UserStatusActive,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if count == 0 {
		user.Role = model.UserRoleAdmin
	}
	if verifiedCode != nil {
		if err := s.repo.CreateUserWithEmailVerification(&user, verifiedCode.ID, time.Now()); err != nil {
			return nil, err
		}
	} else if err := s.repo.Create(&user); err != nil {
		return nil, err
	}
	if err := s.host.EnsureSignupBonus(user.ID); err != nil {
		return nil, err
	}
	return s.createAuthSession(&user)
}

func registrationDisplayName(email string, fallback string) string {
	local, _, found := strings.Cut(NormalizeEmail(email), "@")
	if !found {
		local = ""
	}
	return NormalizeDisplayName(local, fallback)
}

func (s *Service) Login(req LoginRequest) (*AuthSessionResult, error) {
	account := strings.TrimSpace(req.Username)
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
	return s.createAuthSession(user)
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
	if result.ProfileName == "" && result.Username != result.ID {
		result.ProfileName = result.Username
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
	username := NormalizeUsername(req.Username)
	if err := ValidateUsername(username); err != nil {
		return AuthUser{}, err
	}
	stored, err := s.repo.User(user.ID)
	if err != nil {
		return AuthUser{}, err
	}
	if stored.Status != model.UserStatusActive {
		return AuthUser{}, kernel.Forbidden("该账号已被禁用")
	}
	if existing, lookupErr := s.repo.UserByUsername(username); lookupErr == nil {
		if existing.ID != stored.ID {
			return AuthUser{}, kernel.BadAuthRequest("用户名已存在")
		}
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return AuthUser{}, lookupErr
	}
	if req.AvatarResourceID != nil {
		avatarResourceID := strings.TrimSpace(*req.AvatarResourceID)
		if avatarResourceID != "" {
			resource, err := s.repo.ResourceForUser(stored.ID, avatarResourceID)
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
		stored.AvatarResourceID = avatarResourceID
	}
	stored.Username = username
	stored.ProfileName = username
	stored.DisplayName = username
	stored.UpdatedAt = time.Now()
	if err := s.repo.Save(stored); err != nil {
		if isUsernameUniqueViolation(err) {
			return AuthUser{}, kernel.BadAuthRequest("用户名已存在")
		}
		return AuthUser{}, err
	}
	return s.PublicAuthUser(stored)
}

func isUsernameUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "idx_users_username") ||
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

func (s *Service) createAuthSession(user *model.User) (*AuthSessionResult, error) {
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
	if err := s.repo.Create(&session); err != nil {
		return nil, err
	}
	return &AuthSessionResult{User: publicUser, Session: session.ID + "." + token, MaxAgeSecs: int(sessionMaxAge.Seconds())}, nil
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
	return strings.TrimSpace(value)
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
	if !usernamePattern.MatchString(value) {
		return kernel.BadAuthRequest("用户名需为 3-32 位字母、数字、下划线或连字符")
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
