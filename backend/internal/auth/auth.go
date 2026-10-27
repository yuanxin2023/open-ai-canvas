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
	LoginContext  LoginContext `json:"-"`
	Username      string       `json:"username,omitempty"`
	Email         string       `json:"email"`
	EmailCode     string       `json:"emailCode"`
	Phone         string       `json:"phone,omitempty"`
	SMSCode       string       `json:"smsCode,omitempty"`
	Ticket        string       `json:"ticket,omitempty"`
	Password      string       `json:"password"`
	ReferralCode  string       `json:"referralCode,omitempty"`
	AcceptedTerms bool         `json:"acceptedTerms"`
}

type LoginRequest struct {
	LoginContext LoginContext `json:"-"`
	Username     string       `json:"username"`
	Password     string       `json:"password"`
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
	AvatarPreset     *string `json:"avatarPreset"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type PublicAuthSettings struct {
	VerificationPolicy
	SMSBindingAvailable    bool   `json:"smsBindingAvailable"`
	EmailBindingAvailable  bool   `json:"emailBindingAvailable"`
	FirstUser              bool   `json:"firstUser"`
	RegistrationEnabled    bool   `json:"registrationEnabled"`
	LinuxDOEnabled         bool   `json:"linuxdoEnabled"`
	EmailEnabled           bool   `json:"emailEnabled"`
	EmailCodeRequired      bool   `json:"emailCodeRequired"`
	AgreementTitle         string `json:"agreementTitle,omitempty"`
	AgreementContent       string `json:"agreementContent,omitempty"`
	EmailFirstRegistration bool   `json:"emailFirstRegistration"`
	ReferralEnabled        bool   `json:"referralEnabled"`
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
	AdminAccess          *AdminAccess         `json:"adminAccess,omitempty"`
}

type AdminAccess struct {
	Level       model.AdminLevel        `json:"level"`
	Permissions []model.AdminPermission `json:"permissions"`
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
		// 首个管理员同样要看到服务协议，不能因为跳过注册校验就丢失协议字段。
		agreementTitle, agreementContent := s.RegistrationAgreement()
		return &PublicAuthSettings{FirstUser: true, RegistrationEnabled: true, LinuxDOEnabled: false, AgreementTitle: agreementTitle, AgreementContent: agreementContent, EmailFirstRegistration: true}, nil
	}
	referralEnabled := false
	if checker, ok := s.host.(interface{ ReferralEnabled() (bool, error) }); ok {
		referralEnabled, err = checker.ReferralEnabled()
		if err != nil {
			return nil, err
		}
	}
	registrationEnabled, err := s.RegistrationEnabled()
	if err != nil {
		return nil, err
	}
	emailEnabled, err := s.EmailEnabled()
	if err != nil {
		return nil, err
	}
	p, err := s.verificationPolicy()
	if err != nil {
		return nil, err
	}
	smsLogin, err := s.smsAvailable("login")
	if err != nil {
		return nil, err
	}
	smsRegister, err := s.smsAvailable("register")
	if err != nil {
		return nil, err
	}
	smsBind, err := s.smsAvailable("bind")
	if err != nil {
		return nil, err
	}
	p.SMSLogin = p.SMSLogin && smsLogin
	p.EmailLogin = p.EmailLogin && emailEnabled
	p.SMSRegistration = p.SMSRegistration && smsRegister
	p.EmailRegistration = p.EmailRegistration && emailEnabled
	p.SMSAndEmailRegistration = p.SMSAndEmailRegistration && smsRegister && emailEnabled
	agreementTitle, agreementContent := s.RegistrationAgreement()
	return &PublicAuthSettings{
		VerificationPolicy:     p,
		EmailFirstRegistration: true,
		ReferralEnabled:        referralEnabled,
		SMSBindingAvailable:    smsBind,
		EmailBindingAvailable:  emailEnabled,
		FirstUser:              false,
		RegistrationEnabled:    registrationEnabled,
		LinuxDOEnabled:         s.LinuxDOEnabled(),
		EmailEnabled:           emailEnabled,
		EmailCodeRequired:      p.EmailRegistration || p.SMSAndEmailRegistration,
		AgreementTitle:         agreementTitle,
		AgreementContent:       agreementContent,
	}, nil
}

func (s *Service) Register(req RegisterRequest) (*AuthSessionResult, error) {
	return s.RegisterWithEnvironment(req, LoginEnvironment{
		IPAddress: req.LoginContext.IP, UserAgent: req.LoginContext.UserAgent,
		DeviceType: req.LoginContext.DeviceType, Browser: req.LoginContext.Browser,
		BrowserVersion: req.LoginContext.BrowserVersion, OS: req.LoginContext.OS, OSVersion: req.LoginContext.OSVersion,
	})
}

func (s *Service) RegisterWithEnvironment(req RegisterRequest, environment LoginEnvironment) (*AuthSessionResult, error) {
	if !req.AcceptedTerms {
		return nil, kernel.BadAuthRequest("请先同意" + s.AgreementTitleForMessage())
	}
	environment = normalizeLoginEnvironment(environment)
	email := NormalizeEmail(req.Email)
	if email != "" {
		if err := ValidateEmail(email); err != nil {
			return nil, err
		}
	}
	if err := ValidatePassword(req.Password); err != nil {
		return nil, err
	}
	requestedUsername := NormalizeUsername(req.Username)
	if requestedUsername != "" {
		if err := ValidateUsername(requestedUsername); err != nil {
			return nil, err
		}
	}
	s.registrationMu.Lock()
	defer s.registrationMu.Unlock()
	count, err := s.repo.UserCount()
	if err != nil {
		return nil, err
	}
	var verifiedCode *model.EmailVerificationCode
	var verification *model.AuthVerification
	phone := ""
	if count == 0 && email == "" {
		return nil, kernel.BadAuthRequest("请输入邮箱")
	}
	if count > 0 {
		enabled, err := s.RegistrationEnabled()
		if err != nil {
			return nil, err
		}
		if !enabled {
			return nil, kernel.Forbidden("管理员未开放新用户注册")
		}
		if req.Ticket != "" {
			verification, err = s.verifyTicket("register", VerificationConfirm{Ticket: req.Ticket, EmailCode: req.EmailCode, SMSCode: req.SMSCode})
			if err != nil {
				return nil, err
			}
			if verification.Email != email || (verification.Phone != "" && strings.TrimSpace(req.Phone) != verification.Phone && "+86"+strings.TrimSpace(req.Phone) != verification.Phone) || (verification.Phone == "" && req.Phone != "") {
				return nil, invalidVerification()
			}
			phone = verification.Phone
			if email != "" {
				if err := s.validateRegistrationEmailDomain(email); err != nil {
					return nil, err
				}
			}
			if err := s.contactAvailable(email, phone, ""); err != nil {
				return nil, err
			}
		} else {
			policy, err := s.verificationPolicy()
			if err != nil {
				return nil, err
			}
			if !policy.allows("register", "email") || req.Phone != "" {
				return nil, kernel.BadAuthRequest("请先获取本次注册验证码")
			}
			if email == "" {
				return nil, kernel.BadAuthRequest("请输入邮箱")
			}
			if err := s.validateRegistrationEmailDomain(email); err != nil {
				return nil, err
			}
			verifiedCode, err = s.VerifyRegistrationEmailCode(email, req.EmailCode)
			if err != nil {
				return nil, err
			}
		}
	}
	if strings.TrimSpace(req.ReferralCode) != "" {
		checker, ok := s.host.(interface{ ReferralEnabled() (bool, error) })
		if !ok || count == 0 {
			return nil, kernel.BadAuthRequest("当前无法使用推广码")
		}
		enabled, err := checker.ReferralEnabled()
		if err != nil {
			return nil, err
		}
		if !enabled {
			return nil, kernel.BadAuthRequest("邀请返利尚未开启")
		}
	}
	if email != "" {
		if err := s.repo.CheckEmailAvailable(email, ""); err != nil {
			return nil, err
		}
	}
	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	user := model.User{
		ID: kernel.NewID(), Email: email, Phone: phone,
		Role: model.UserRoleUser, Status: model.UserStatusActive,
		PasswordHash: passwordHash, RegistrationIP: environment.IPAddress,
		CreatedAt: now, UpdatedAt: now,
	}
	if count == 0 {
		user.Role, user.AdminLevel = model.UserRoleAdmin, model.AdminLevelFull
	}
	if verification != nil {
		if email != "" {
			user.EmailVerifiedAt = &now
		}
		if phone != "" {
			user.PhoneVerifiedAt = &now
		}
	} else if verifiedCode != nil {
		user.EmailVerifiedAt = &now
	}
	created := false
	for attempt := 0; attempt < 32; attempt++ {
		if requestedUsername != "" {
			user.Username = requestedUsername
			user.UsernameCustomizedAt = &now
		} else {
			user.Username = kernel.DefaultLoginUsernameCandidate(email, user.ID, attempt)
		}
		user.DisplayName, user.ProfileName = user.Username, user.Username
		if verification != nil {
			err = s.repo.CreateUserWithVerification(&user, verification.ID, req.ReferralCode)
		} else if verifiedCode != nil {
			err = s.repo.CreateUserWithEmailVerification(&user, verifiedCode.ID, now, req.ReferralCode)
		} else {
			err = s.repo.CreateRegisteredUser(&user)
		}
		if err == nil {
			created = true
			break
		}
		if errors.Is(err, repository.ErrReferralCodeInvalid) {
			return nil, kernel.BadAuthRequest("推广码无效或已失效")
		}
		if !isUsernameUniqueViolation(err) {
			return nil, err
		}
		if requestedUsername != "" {
			return nil, kernel.BadAuthRequest("用户名已存在")
		}
	}
	if !created {
		return nil, kernel.WrapAppError(500, "无法生成唯一用户名", err)
	}
	if err := s.host.EnsureSignupBonus(user.ID); err != nil {
		return nil, err
	}
	event := NewLoginLog("password", "registration", loginContextFromEnvironment(environment))
	result, err := s.createAuthSession(&user, event)
	if err != nil {
		return nil, s.recordLoginFailure(event, err)
	}
	return result, nil
}

func (s *Service) LoginRateLimitSubject(account string) (string, error) {
	account = strings.TrimSpace(account)
	user, err := s.repo.UserByAccount(account)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "input:" + strings.ToLower(account), nil
	}
	if err != nil {
		return "", err
	}
	return "user:" + user.ID, nil
}

func (s *Service) Login(req LoginRequest) (result *AuthSessionResult, err error) {
	event := NewLoginLog("password", "login", req.LoginContext)
	event.Account = MaskLoginAccount(req.Username)
	defer func() {
		if err != nil {
			err = s.recordLoginFailure(event, err)
		}
	}()
	account := NormalizeUsername(req.Username)
	user, err := s.repo.UserByAccount(account)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, kernel.Unauthorized("用户名、邮箱或密码不正确")
		}
		return nil, err
	}
	loginLogUser(event, user)
	if user.Status != model.UserStatusActive {
		return nil, kernel.Forbidden("该账号已被禁用")
	}
	if !verifyPassword(req.Password, user.PasswordHash) {
		return nil, kernel.Unauthorized("用户名、邮箱或密码不正确")
	}
	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	if err := s.repo.UpdateUserLoginTime(user.ID, now); err != nil {
		return nil, err
	}
	if err := s.host.EnsureSignupBonus(user.ID); err != nil {
		return nil, err
	}
	return s.createAuthSession(user, event)
}

func (s *Service) LoginWithEnvironment(req LoginRequest, environment LoginEnvironment) (*AuthSessionResult, error) {
	environment = normalizeLoginEnvironment(environment)
	req.LoginContext = loginContextFromEnvironment(environment)
	return s.Login(req)
}

func (s *Service) Logout(cookieValue string, contexts ...LoginContext) (err error) {
	sessionID, token := parseSessionCookie(cookieValue)
	if sessionID == "" || token == "" {
		return nil
	}
	var context LoginContext
	if len(contexts) > 0 {
		context = contexts[0]
	}
	event := NewLoginLog("unknown", "logout", context)
	event.Event = "logout"
	event.SessionRef = HashToken(sessionID)
	defer func() {
		if err != nil {
			err = s.recordLoginFailure(event, err)
		}
	}()
	return s.repo.RevokeAuthSessionWithLog(sessionID, HashToken(token), event)
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
	if err := s.repo.HydrateAdminAccess(user); err != nil {
		return nil, err
	}
	return user, nil
}

// 认证响应只补充当前用户自己的第三方公开身份，不把身份表或密钥字段暴露给其他列表接口。
func (s *Service) PublicAuthUser(user *model.User) (AuthUser, error) {
	if err := s.repo.HydrateAdminAccess(user); err != nil {
		return AuthUser{}, err
	}
	result := AuthUser{User: *user, AvatarResourceID: user.AvatarResourceID}
	if user.Role == model.UserRoleAdmin {
		result.AdminAccess = &AdminAccess{Level: user.AdminLevel, Permissions: append([]model.AdminPermission(nil), user.AdminPermissions...)}
	}
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
	if user.AvatarMode == "custom" {
		result.AvatarURL = profileAvatarURL(user)
	} else if user.AvatarMode != "default" && user.AvatarResourceID != "" {
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
	if req.AvatarPreset != nil {
		if req.AvatarResourceID != nil || !validProfileAvatarPreset(*req.AvatarPreset) {
			return AuthUser{}, kernel.BadAuthRequest("请选择一种有效的头像方式")
		}
	}
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
		UserID: stored.ID, Username: username, AvatarResourceID: avatarResourceID, AvatarPreset: req.AvatarPreset,
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
	return s.ChangeProfilePassword(user, cookieValue, ProfilePasswordRequest{CurrentPassword: req.CurrentPassword, NewPassword: req.NewPassword})
}

func allowedProfileAvatarMIME(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func (s *Service) createAuthSession(user *model.User, event *model.LoginLog) (*AuthSessionResult, error) {
	loginLogUser(event, user)
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
	event.SessionRef = HashToken(session.ID)
	event.CreatedAt = now.UTC()
	if err := s.repo.CreateAuthSessionForUser(user, &session, event); err != nil {
		return nil, profileWriteError(err)
	}
	s.host.RecordActivity(user.ID, "login", 1)
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

func (s *Service) createAuthSessionWithEnvironment(user *model.User, method string, environment LoginEnvironment) (*AuthSessionResult, error) {
	environment = normalizeLoginEnvironment(environment)
	source := "login"
	if method == "email_register" {
		method, source = "password", "registration"
	}
	return s.createAuthSession(user, NewLoginLog(method, source, loginContextFromEnvironment(environment)))
}
