package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"path/filepath"
	"strings"
	"time"

	"onebookai/internal/util"
	"onebookai/pkg/auth"
	"onebookai/pkg/domain"
	"onebookai/pkg/storage"
	"onebookai/pkg/store"
)

// Config holds runtime configuration for the core application.
type Config struct {
	DatabaseURL         string
	RedisAddr           string
	RedisPassword       string
	SessionTTL          time.Duration
	RefreshTTL          time.Duration
	JWTPrivateKeyPath   string
	JWTPublicKeyPath    string
	JWTKeyID            string
	JWTVerifyPublicKeys map[string]string
	JWTIssuer           string
	JWTAudience         string
	JWTLeeway           time.Duration
	Store               store.Store
	Sessions            store.SessionStore
	RefreshTokens       store.RefreshTokenStore
	AvatarObjects       storage.ObjectStore
	MinioEndpoint       string
	MinioAccessKey      string
	MinioSecretKey      string
	MinioBucket         string
	MinioUseSSL         bool
	EvalStorageDir      string
	EvalWorkerPoll      time.Duration
}

// App is the core application service wiring together storage and auth logic.
type App struct {
	store         store.Store
	sessions      store.SessionStore
	refreshTokens store.RefreshTokenStore
	avatarObjects storage.ObjectStore
	refreshTTL    time.Duration
	evals         *evalCenter
}

type LoginActivity struct {
	IP        string
	UserAgent string
}

const maxAvatarUploadBytes int64 = 5 * 1024 * 1024

type OAuthLoginInput struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	AvatarURL     string
	LoginActivity LoginActivity
}

// New constructs the application with database storage and session management.
func New(cfg Config) (*App, error) {
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL == 0 {
		cfg.RefreshTTL = 7 * 24 * time.Hour
	}

	dataStore := cfg.Store
	if dataStore == nil {
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("database URL required")
		}
		var err error
		dataStore, err = store.NewGormStore(cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("init postgres store: %w", err)
		}
	}

	sessionStore := cfg.Sessions
	if sessionStore == nil {
		if strings.TrimSpace(cfg.JWTPrivateKeyPath) == "" {
			return nil, fmt.Errorf("jwtPrivateKeyPath is required")
		}
		if strings.TrimSpace(cfg.RedisAddr) == "" {
			return nil, fmt.Errorf("redisAddr is required for jwt+redis session strategy")
		}
		jwtOpts := store.JWTOptions{
			Issuer:   cfg.JWTIssuer,
			Audience: cfg.JWTAudience,
			Leeway:   cfg.JWTLeeway,
		}
		revoker := store.NewRedisTokenRevoker(cfg.RedisAddr, cfg.RedisPassword)
		rsStore, err := store.NewJWTRS256SessionStoreFromPEMWithOptions(
			cfg.JWTPrivateKeyPath,
			cfg.JWTPublicKeyPath,
			cfg.JWTKeyID,
			cfg.JWTVerifyPublicKeys,
			cfg.SessionTTL,
			revoker,
			jwtOpts,
		)
		if err != nil {
			return nil, fmt.Errorf("init rs256 jwt session store: %w", err)
		}
		sessionStore = rsStore
	}

	refreshStore := cfg.RefreshTokens
	if refreshStore == nil {
		if strings.TrimSpace(cfg.RedisAddr) == "" {
			return nil, fmt.Errorf("redisAddr is required for jwt+redis refresh token strategy")
		}
		refreshStore = store.NewRedisRefreshTokenStore(cfg.RedisAddr, cfg.RedisPassword)
	}

	avatarObjects := cfg.AvatarObjects
	if avatarObjects == nil && strings.TrimSpace(cfg.MinioEndpoint) != "" && strings.TrimSpace(cfg.MinioBucket) != "" {
		objStore, err := storage.NewMinioStore(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioBucket, cfg.MinioUseSSL)
		if err != nil {
			return nil, fmt.Errorf("init avatar object store: %w", err)
		}
		avatarObjects = objStore
	}

	evals, err := newEvalCenter(dataStore, strings.TrimSpace(cfg.EvalStorageDir), cfg.EvalWorkerPoll)
	if err != nil {
		return nil, fmt.Errorf("init eval center: %w", err)
	}

	return &App{
		store:         dataStore,
		sessions:      sessionStore,
		refreshTokens: refreshStore,
		avatarObjects: avatarObjects,
		refreshTTL:    cfg.RefreshTTL,
		evals:         evals,
	}, nil
}

// SignUp registers a new user with default role user.
func (a *App) SignUp(email, password string) (domain.User, string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return domain.User{}, "", "", ErrEmailAndPasswordRequired
	}
	return a.SignUpWithVerifiedIdentity(domain.IdentityEmail, email, password)
}

func (a *App) SignUpWithVerifiedIdentity(identityType domain.IdentityType, identifier, password string) (domain.User, string, string, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || password == "" {
		return domain.User{}, "", "", ErrEmailAndPasswordRequired
	}
	if !isSupportedIdentityType(identityType) {
		return domain.User{}, "", "", ErrUnsupportedIdentityType
	}
	if err := auth.ValidatePassword(password); err != nil {
		return domain.User{}, "", "", err
	}
	exists, err := a.store.HasUserIdentity(identityType, identifier)
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("check identity: %w", err)
	}
	if exists {
		return domain.User{}, "", "", ErrIdentifierAlreadyExists
	}
	role := domain.RoleUser
	count, err := a.store.UserCount()
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("count users: %w", err)
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("hash password: %w", err)
	}
	if count == 0 {
		role = domain.RoleAdmin
	}
	user, err := a.createUserWithIdentity(identityType, identifier, passwordHash, role)
	if err != nil {
		if strings.Contains(err.Error(), "identity already exists") {
			return domain.User{}, "", "", ErrIdentifierAlreadyExists
		}
		return domain.User{}, "", "", err
	}
	return a.issueUserTokens(user)
}

// SignUpPasswordless registers a new user without password and issues tokens.
func (a *App) SignUpPasswordless(email string) (domain.User, string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return domain.User{}, "", "", ErrEmailRequired
	}
	return a.SignUpPasswordlessWithVerifiedIdentity(domain.IdentityEmail, email)
}

func (a *App) SignUpPasswordlessWithVerifiedIdentity(identityType domain.IdentityType, identifier string) (domain.User, string, string, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return domain.User{}, "", "", ErrIdentifierRequired
	}
	if !isSupportedIdentityType(identityType) {
		return domain.User{}, "", "", ErrUnsupportedIdentityType
	}
	exists, err := a.store.HasUserIdentity(identityType, identifier)
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("check identity: %w", err)
	}
	if exists {
		return domain.User{}, "", "", ErrIdentifierAlreadyExists
	}
	role := domain.RoleUser
	count, err := a.store.UserCount()
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("count users: %w", err)
	}
	if count == 0 {
		role = domain.RoleAdmin
	}
	user, err := a.createUserWithIdentity(identityType, identifier, "", role)
	if err != nil {
		if strings.Contains(err.Error(), "identity already exists") {
			return domain.User{}, "", "", ErrIdentifierAlreadyExists
		}
		return domain.User{}, "", "", err
	}
	return a.issueUserTokens(user)
}

// Login validates credentials and issues a session token.
func (a *App) Login(email, password string) (domain.User, string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	return a.LoginWithPasswordByIdentity(domain.IdentityEmail, email, password, LoginActivity{})
}

func (a *App) LoginWithPasswordByIdentity(identityType domain.IdentityType, identifier, password string, activity LoginActivity) (domain.User, string, string, error) {
	identifier = strings.TrimSpace(identifier)
	if !isSupportedIdentityType(identityType) {
		return domain.User{}, "", "", ErrInvalidCredentials
	}
	user, ok, err := a.store.GetUserByIdentity(identityType, identifier)
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("fetch user: %w", err)
	}
	if !ok {
		return domain.User{}, "", "", ErrInvalidCredentials
	}
	if user.Status == domain.StatusDisabled {
		return domain.User{}, "", "", ErrUserDisabled
	}
	if !hasPassword(user.PasswordHash) {
		return domain.User{}, "", "", ErrPasswordNotSet
	}
	if !auth.CheckPassword(password, user.PasswordHash) {
		return domain.User{}, "", "", ErrInvalidCredentials
	}
	return a.issueLoginTokens(user, activity)
}

// LoginByEmail issues a session token for an existing account.
func (a *App) LoginByEmail(email string) (domain.User, string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	return a.LoginByIdentity(domain.IdentityEmail, email, LoginActivity{})
}

func (a *App) LoginByIdentity(identityType domain.IdentityType, identifier string, activity LoginActivity) (domain.User, string, string, error) {
	identifier = strings.TrimSpace(identifier)
	if !isSupportedIdentityType(identityType) {
		return domain.User{}, "", "", ErrInvalidCredentials
	}
	user, ok, err := a.store.GetUserByIdentity(identityType, identifier)
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("fetch user: %w", err)
	}
	if !ok {
		return domain.User{}, "", "", ErrInvalidCredentials
	}
	if user.Status == domain.StatusDisabled {
		return domain.User{}, "", "", ErrUserDisabled
	}
	return a.issueLoginTokens(user, activity)
}

func (a *App) CompleteOAuthLogin(input OAuthLoginInput) (domain.User, string, string, error) {
	provider := normalizeOAuthProvider(input.Provider)
	subject := strings.TrimSpace(input.Subject)
	if provider == "" || subject == "" {
		return domain.User{}, "", "", ErrIdentifierRequired
	}
	user, ok, err := a.store.GetUserByProviderIdentity(provider, subject)
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("fetch oauth identity: %w", err)
	}
	if ok {
		if user.Status == domain.StatusDisabled {
			return domain.User{}, "", "", ErrUserDisabled
		}
		if err := a.ensureUserProfile(user.ID, input.DisplayName, input.AvatarURL); err != nil {
			return domain.User{}, "", "", err
		}
		return a.issueLoginTokens(user, input.LoginActivity)
	}

	now := time.Now().UTC()
	email := normalizeOAuthEmail(input.Email)
	if input.EmailVerified && email != "" {
		user, ok, err = a.store.GetUserByIdentity(domain.IdentityEmail, email)
		if err != nil {
			return domain.User{}, "", "", fmt.Errorf("fetch email identity: %w", err)
		}
		if ok {
			if user.Status == domain.StatusDisabled {
				return domain.User{}, "", "", ErrUserDisabled
			}
			if err := a.store.SaveUserIdentity(domain.UserIdentity{
				ID:         util.NewID(),
				UserID:     user.ID,
				Type:       domain.IdentityOAuth,
				Provider:   provider,
				Identifier: subject,
				VerifiedAt: &now,
				IsPrimary:  false,
				CreatedAt:  now,
				UpdatedAt:  now,
			}); err != nil {
				if strings.Contains(err.Error(), "identity already exists") {
					existingUser, found, fetchErr := a.store.GetUserByProviderIdentity(provider, subject)
					if fetchErr != nil {
						return domain.User{}, "", "", fmt.Errorf("fetch oauth identity after conflict: %w", fetchErr)
					}
					if found {
						if existingUser.Status == domain.StatusDisabled {
							return domain.User{}, "", "", ErrUserDisabled
						}
						return a.issueLoginTokens(existingUser, input.LoginActivity)
					}
				}
				return domain.User{}, "", "", fmt.Errorf("save oauth identity: %w", err)
			}
			if err := a.ensureUserProfile(user.ID, input.DisplayName, input.AvatarURL); err != nil {
				return domain.User{}, "", "", err
			}
			return a.issueLoginTokens(user, input.LoginActivity)
		}
	}

	role := domain.RoleUser
	count, err := a.store.UserCount()
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("count users: %w", err)
	}
	if count == 0 {
		role = domain.RoleAdmin
	}
	user, err = a.createOAuthUser(provider, subject, email, input.EmailVerified, role, input.DisplayName, input.AvatarURL)
	if err != nil {
		if strings.Contains(err.Error(), "identity already exists") {
			existingUser, found, fetchErr := a.store.GetUserByProviderIdentity(provider, subject)
			if fetchErr != nil {
				return domain.User{}, "", "", fmt.Errorf("fetch oauth identity after create conflict: %w", fetchErr)
			}
			if found {
				if existingUser.Status == domain.StatusDisabled {
					return domain.User{}, "", "", ErrUserDisabled
				}
				return a.issueLoginTokens(existingUser, input.LoginActivity)
			}
			return domain.User{}, "", "", ErrIdentifierAlreadyExists
		}
		return domain.User{}, "", "", err
	}
	return a.issueLoginTokens(user, input.LoginActivity)
}

func (a *App) issueUserTokens(user domain.User) (domain.User, string, string, error) {
	user = a.userWithProfile(user, nil)
	accessToken, refreshToken, err := a.issueTokens(user.ID)
	if err != nil {
		return domain.User{}, "", "", err
	}
	return user, accessToken, refreshToken, nil
}

func (a *App) issueLoginTokens(user domain.User, activity LoginActivity) (domain.User, string, string, error) {
	now := time.Now().UTC()
	if err := a.store.UpdateUserLoginActivity(user.ID, activity.IP, activity.UserAgent, now); err != nil {
		return domain.User{}, "", "", fmt.Errorf("record login activity: %w", err)
	}
	profile, ok, err := a.store.GetUserProfile(user.ID)
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("fetch profile: %w", err)
	}
	if !ok {
		profile = domain.UserProfile{UserID: user.ID, LastLoginAt: &now, LoginCount: 1, CreatedAt: now, UpdatedAt: now}
	}
	return a.issueUserTokens(a.userWithProfile(user, &profile))
}

func (a *App) userWithProfile(user domain.User, profile *domain.UserProfile) domain.User {
	if profile == nil {
		fetched, ok, err := a.store.GetUserProfile(user.ID)
		if err == nil && ok {
			profile = &fetched
		}
	}
	if profile == nil {
		return user
	}
	user.DisplayName = profile.DisplayName
	user.AvatarURL = effectiveAvatarURL(user.ID, *profile)
	user.LastLoginAt = profile.LastLoginAt
	user.LoginCount = profile.LoginCount
	return user
}

func (a *App) ensureUserProfile(userID, displayName, avatarURL string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	now := time.Now().UTC()
	profile, ok, err := a.store.GetUserProfile(userID)
	if err != nil {
		return fmt.Errorf("fetch profile: %w", err)
	}
	if !ok {
		return a.store.SaveUserProfile(domain.UserProfile{
			UserID:      userID,
			DisplayName: sanitizeDisplayName(displayName),
			AvatarURL:   normalizeAvatarURL(avatarURL),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	changed := false
	if strings.TrimSpace(profile.DisplayName) == "" {
		if next := sanitizeDisplayName(displayName); next != "" {
			profile.DisplayName = next
			changed = true
		}
	}
	if strings.TrimSpace(profile.AvatarURL) == "" && strings.TrimSpace(profile.AvatarStorageKey) == "" {
		if next := normalizeAvatarURL(avatarURL); next != "" {
			profile.AvatarURL = next
			changed = true
		}
	}
	if !changed {
		return nil
	}
	profile.UpdatedAt = now
	return a.store.SaveUserProfile(profile)
}

func (a *App) UpdateMyProfile(user domain.User, email *string, displayName *string) (domain.User, error) {
	if email != nil {
		updated, err := a.UpdateMyEmail(user, *email)
		if err != nil {
			return domain.User{}, err
		}
		user = updated
	}
	if displayName != nil {
		profile, err := a.profileForUpdate(user.ID)
		if err != nil {
			return domain.User{}, err
		}
		profile.DisplayName = sanitizeDisplayName(*displayName)
		profile.UpdatedAt = time.Now().UTC()
		if err := a.store.SaveUserProfile(profile); err != nil {
			return domain.User{}, fmt.Errorf("update profile: %w", err)
		}
	}
	return a.AdminlessUser(user.ID)
}

func (a *App) AdminlessUser(userID string) (domain.User, error) {
	user, ok, err := a.store.GetUserByID(userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("fetch user: %w", err)
	}
	if !ok {
		return domain.User{}, fmt.Errorf("user not found")
	}
	return a.userWithProfile(user, nil), nil
}

func (a *App) UploadUserAvatar(user domain.User, r io.Reader, filename string) (domain.User, error) {
	if a.avatarObjects == nil {
		return domain.User{}, fmt.Errorf("avatar storage is not configured")
	}
	limited := &io.LimitedReader{R: r, N: maxAvatarUploadBytes + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return domain.User{}, fmt.Errorf("read avatar: %w", err)
	}
	if int64(len(data)) > maxAvatarUploadBytes {
		return domain.User{}, fmt.Errorf("avatar exceeds 5MB limit")
	}
	contentType, ext, err := avatarContentType(data, filename)
	if err != nil {
		return domain.User{}, err
	}
	profile, err := a.profileForUpdate(user.ID)
	if err != nil {
		return domain.User{}, err
	}
	oldKey := strings.TrimSpace(profile.AvatarStorageKey)
	key := fmt.Sprintf("avatars/%s/%s%s", user.ID, util.NewID(), ext)
	if err := a.avatarObjects.Put(context.Background(), key, bytes.NewReader(data), int64(len(data)), contentType); err != nil {
		return domain.User{}, err
	}
	now := time.Now().UTC()
	profile.AvatarStorageKey = key
	profile.AvatarContentType = contentType
	profile.UpdatedAt = now
	if err := a.store.SaveUserProfile(profile); err != nil {
		_ = a.avatarObjects.Delete(context.Background(), key)
		return domain.User{}, fmt.Errorf("save avatar profile: %w", err)
	}
	if oldKey != "" && oldKey != key {
		_ = a.avatarObjects.Delete(context.Background(), oldKey)
	}
	return a.AdminlessUser(user.ID)
}

func (a *App) UserAvatar(userID string) (io.ReadCloser, string, error) {
	if a.avatarObjects == nil {
		return nil, "", fmt.Errorf("avatar storage is not configured")
	}
	profile, ok, err := a.store.GetUserProfile(userID)
	if err != nil {
		return nil, "", fmt.Errorf("fetch profile: %w", err)
	}
	if !ok || strings.TrimSpace(profile.AvatarStorageKey) == "" {
		return nil, "", fmt.Errorf("avatar not found")
	}
	body, contentType, err := a.avatarObjects.Get(context.Background(), profile.AvatarStorageKey)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = profile.AvatarContentType
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	return body, contentType, nil
}

func (a *App) profileForUpdate(userID string) (domain.UserProfile, error) {
	now := time.Now().UTC()
	profile, ok, err := a.store.GetUserProfile(userID)
	if err != nil {
		return domain.UserProfile{}, fmt.Errorf("fetch profile: %w", err)
	}
	if ok {
		return profile, nil
	}
	return domain.UserProfile{UserID: userID, CreatedAt: now, UpdatedAt: now}, nil
}

// UserFromToken resolves a user from a session token.
func (a *App) UserFromToken(token string) (domain.User, bool) {
	uid, ok, err := a.sessions.GetUserIDByToken(token)
	if err != nil || !ok {
		return domain.User{}, false
	}
	user, found, err := a.store.GetUserByID(uid)
	if err != nil || !found {
		return domain.User{}, false
	}
	if user.Status == domain.StatusDisabled {
		return domain.User{}, false
	}
	return a.userWithProfile(user, nil), true
}

// Logout invalidates access token and optional refresh token.
func (a *App) Logout(accessToken, refreshToken string) error {
	if err := a.sessions.DeleteSession(accessToken); err != nil {
		return err
	}
	if err := a.RevokeRefreshToken(refreshToken); err != nil {
		return err
	}
	return nil
}

// Refresh rotates refresh token and issues a new token pair.
func (a *App) Refresh(refreshToken string) (domain.User, string, string, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return domain.User{}, "", "", ErrRefreshTokenRequired
	}
	userID, newRefreshToken, err := a.refreshTokens.RotateToken(refreshToken, a.refreshTTL)
	if err != nil {
		if errors.Is(err, store.ErrInvalidRefreshToken) || errors.Is(err, store.ErrRefreshTokenReplay) {
			return domain.User{}, "", "", ErrInvalidRefreshToken
		}
		return domain.User{}, "", "", fmt.Errorf("resolve refresh token: %w", err)
	}
	user, found, err := a.store.GetUserByID(userID)
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("fetch user: %w", err)
	}
	if !found || user.Status == domain.StatusDisabled {
		_ = a.refreshTokens.DeleteToken(newRefreshToken)
		return domain.User{}, "", "", ErrInvalidRefreshToken
	}
	accessToken, err := a.sessions.NewSession(user.ID)
	if err != nil {
		_ = a.refreshTokens.DeleteToken(newRefreshToken)
		return domain.User{}, "", "", fmt.Errorf("issue access token: %w", err)
	}
	return user, accessToken, newRefreshToken, nil
}

// RevokeRefreshToken invalidates a refresh token explicitly.
func (a *App) RevokeRefreshToken(refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}
	return a.refreshTokens.DeleteToken(refreshToken)
}

// HasUserEmail checks whether email already exists.
func (a *App) HasUserEmail(email string) (bool, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return false, ErrEmailRequired
	}
	return a.store.HasUserIdentity(domain.IdentityEmail, email)
}

func (a *App) HasUserIdentity(identityType domain.IdentityType, identifier string) (bool, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return false, ErrIdentifierRequired
	}
	if !isSupportedIdentityType(identityType) {
		return false, ErrUnsupportedIdentityType
	}
	return a.store.HasUserIdentity(identityType, identifier)
}

// CanLoginWithPassword reports whether an email should go to password login.
// Unknown/disabled/non-password accounts all return false to reduce account enumeration.
func (a *App) CanLoginWithPassword(email string) (bool, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return false, ErrEmailRequired
	}
	return a.CanLoginWithPasswordIdentity(domain.IdentityEmail, email)
}

func (a *App) CanLoginWithPasswordIdentity(identityType domain.IdentityType, identifier string) (bool, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return false, ErrIdentifierRequired
	}
	if !isSupportedIdentityType(identityType) {
		return false, ErrUnsupportedIdentityType
	}
	user, ok, err := a.store.GetUserByIdentity(identityType, identifier)
	if err != nil {
		return false, fmt.Errorf("fetch user: %w", err)
	}
	if !ok || user.Status == domain.StatusDisabled {
		return false, nil
	}
	return hasPassword(user.PasswordHash), nil
}

// ListUsers returns all users (admin use only).
func (a *App) ListUsers() ([]domain.User, error) {
	return a.store.ListUsers()
}

// ListUsersWithOptions returns users with filtering and pagination.
func (a *App) ListUsersWithOptions(opts store.UserListOptions) ([]domain.AdminUser, int, error) {
	users, total, err := a.store.ListUsersWithOptions(opts)
	if err != nil {
		return nil, 0, err
	}
	adminUsers, err := a.adminUsersFromUsers(users)
	if err != nil {
		return nil, 0, err
	}
	return adminUsers, total, nil
}

// AdminGetUser returns a single user by id.
func (a *App) AdminGetUser(userID string) (domain.AdminUser, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.AdminUser{}, fmt.Errorf("user id required")
	}
	user, ok, err := a.store.GetUserByID(userID)
	if err != nil {
		return domain.AdminUser{}, fmt.Errorf("fetch user: %w", err)
	}
	if !ok {
		return domain.AdminUser{}, fmt.Errorf("user not found")
	}
	adminUsers, err := a.adminUsersFromUsers([]domain.User{user})
	if err != nil {
		return domain.AdminUser{}, err
	}
	return adminUsers[0], nil
}

// UpdateMyEmail updates the current user's email.
func (a *App) UpdateMyEmail(user domain.User, newEmail string) (domain.User, error) {
	email := strings.TrimSpace(strings.ToLower(newEmail))
	if email == "" {
		return domain.User{}, ErrEmailRequired
	}
	if email == user.Email {
		return user, nil
	}
	existing, ok, err := a.store.GetUserByEmail(email)
	if err != nil {
		return domain.User{}, fmt.Errorf("check email: %w", err)
	}
	if ok && existing.ID != user.ID {
		return domain.User{}, ErrEmailAlreadyExists
	}
	now := time.Now().UTC()
	user.Email = email
	user.UpdatedAt = now
	if err := a.store.SaveUser(user); err != nil {
		return domain.User{}, fmt.Errorf("update user: %w", err)
	}
	if err := a.store.SaveUserIdentity(domain.UserIdentity{
		ID:         util.NewID(),
		UserID:     user.ID,
		Type:       domain.IdentityEmail,
		Identifier: email,
		VerifiedAt: &now,
		IsPrimary:  true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}); err != nil {
		return domain.User{}, fmt.Errorf("update email identity: %w", err)
	}
	return a.userWithProfile(user, nil), nil
}

// ChangePassword updates the user's password after verifying the current password.
func (a *App) ChangePassword(userID, currentPassword, newPassword string) error {
	if strings.TrimSpace(newPassword) == "" {
		return ErrNewPasswordRequired
	}
	if err := auth.ValidatePassword(newPassword); err != nil {
		return err
	}
	user, ok, err := a.store.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("fetch user: %w", err)
	}
	if !ok {
		return fmt.Errorf("user not found")
	}
	if user.Status == domain.StatusDisabled {
		return fmt.Errorf("user disabled")
	}
	if hasPassword(user.PasswordHash) {
		if strings.TrimSpace(currentPassword) == "" {
			return ErrCurrentPasswordRequired
		}
		if !auth.CheckPassword(currentPassword, user.PasswordHash) {
			return ErrInvalidCredentials
		}
		if currentPassword == newPassword {
			return ErrNewPasswordMustDiffer
		}
	}
	passwordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	revokeSince := time.Now().UTC()
	user.PasswordHash = passwordHash
	user.UpdatedAt = revokeSince
	if err := a.store.SaveUser(user); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if err := a.revokeAllUserTokens(userID, revokeSince); err != nil {
		return fmt.Errorf("revoke user tokens: %w", err)
	}
	return nil
}

// ResetPasswordByEmail updates password after reset verification.
func (a *App) ResetPasswordByEmail(email, newPassword string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return ErrEmailRequired
	}
	return a.ResetPasswordByIdentity(domain.IdentityEmail, email, newPassword)
}

func (a *App) ResetPasswordByIdentity(identityType domain.IdentityType, identifier, newPassword string) error {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return ErrIdentifierRequired
	}
	if !isSupportedIdentityType(identityType) {
		return ErrUnsupportedIdentityType
	}
	if strings.TrimSpace(newPassword) == "" {
		return ErrNewPasswordRequired
	}
	if err := auth.ValidatePassword(newPassword); err != nil {
		return err
	}
	user, ok, err := a.store.GetUserByIdentity(identityType, identifier)
	if err != nil {
		return fmt.Errorf("fetch user: %w", err)
	}
	if !ok {
		return ErrInvalidCredentials
	}
	if user.Status == domain.StatusDisabled {
		return ErrUserDisabled
	}
	if hasPassword(user.PasswordHash) && auth.CheckPassword(newPassword, user.PasswordHash) {
		return ErrNewPasswordMustDiffer
	}
	passwordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	revokeSince := time.Now().UTC()
	user.PasswordHash = passwordHash
	user.UpdatedAt = revokeSince
	if err := a.store.SaveUser(user); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if err := a.revokeAllUserTokens(user.ID, revokeSince); err != nil {
		return fmt.Errorf("revoke user tokens: %w", err)
	}
	return nil
}

func (a *App) issueTokens(userID string) (string, string, error) {
	accessToken, err := a.sessions.NewSession(userID)
	if err != nil {
		return "", "", fmt.Errorf("issue access token: %w", err)
	}
	refreshToken, err := a.refreshTokens.NewToken(userID, a.refreshTTL)
	if err != nil {
		return "", "", fmt.Errorf("issue refresh token: %w", err)
	}
	return accessToken, refreshToken, nil
}

func (a *App) adminUsersFromUsers(users []domain.User) ([]domain.AdminUser, error) {
	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	identitiesByUser, err := a.store.ListUserIdentities(userIDs)
	if err != nil {
		return nil, fmt.Errorf("fetch user identities: %w", err)
	}
	profilesByUser, err := a.store.ListUserProfiles(userIDs)
	if err != nil {
		return nil, fmt.Errorf("fetch user profiles: %w", err)
	}
	items := make([]domain.AdminUser, 0, len(users))
	for _, user := range users {
		email := ""
		phone := ""
		loginMethods := []string{}
		oauthProviders := []string{}
		emailVerified := false
		phoneVerified := false
		seenMethods := map[string]struct{}{}
		seenProviders := map[string]struct{}{}
		for _, identity := range identitiesByUser[user.ID] {
			switch identity.Type {
			case domain.IdentityEmail:
				if email == "" {
					email = identity.Identifier
				}
				emailVerified = emailVerified || identity.VerifiedAt != nil
				if _, ok := seenMethods["email"]; !ok {
					loginMethods = append(loginMethods, "email")
					seenMethods["email"] = struct{}{}
				}
			case domain.IdentityPhone:
				if phone == "" {
					phone = identity.Identifier
				}
				phoneVerified = phoneVerified || identity.VerifiedAt != nil
				if _, ok := seenMethods["phone"]; !ok {
					loginMethods = append(loginMethods, "phone")
					seenMethods["phone"] = struct{}{}
				}
			case domain.IdentityOAuth:
				provider := strings.TrimSpace(identity.Provider)
				if provider != "" {
					if _, ok := seenProviders[provider]; !ok {
						oauthProviders = append(oauthProviders, provider)
						seenProviders[provider] = struct{}{}
					}
					if _, ok := seenMethods[provider]; !ok {
						loginMethods = append(loginMethods, provider)
						seenMethods[provider] = struct{}{}
					}
				}
			}
		}
		if email == "" && strings.Contains(user.Email, "@") {
			email = user.Email
		}
		if phone == "" && user.Email != "" && !strings.Contains(user.Email, "@") {
			phone = user.Email
		}
		profile := profilesByUser[user.ID]
		items = append(items, domain.AdminUser{
			ID:             user.ID,
			Email:          email,
			Phone:          phone,
			DisplayName:    profile.DisplayName,
			AvatarURL:      effectiveAvatarURL(user.ID, profile),
			AdminNote:      profile.AdminNote,
			LoginMethods:   loginMethods,
			OAuthProviders: oauthProviders,
			EmailVerified:  emailVerified,
			PhoneVerified:  phoneVerified,
			PasswordSet:    hasPassword(user.PasswordHash),
			LastLoginAt:    profile.LastLoginAt,
			LoginCount:     profile.LoginCount,
			Role:           user.Role,
			Status:         user.Status,
			CreatedAt:      user.CreatedAt,
			UpdatedAt:      user.UpdatedAt,
		})
	}
	return items, nil
}

func currentUserContactIdentities(identities []domain.UserIdentity, user domain.User) (string, string) {
	email := ""
	phone := ""
	for _, identity := range identities {
		switch identity.Type {
		case domain.IdentityEmail:
			if email == "" {
				email = identity.Identifier
			}
		case domain.IdentityPhone:
			if phone == "" {
				phone = identity.Identifier
			}
		}
	}
	if email == "" && strings.Contains(user.Email, "@") {
		email = user.Email
	}
	if phone == "" && user.Email != "" && !strings.Contains(user.Email, "@") {
		phone = user.Email
	}
	return email, phone
}

func hasRemainingLoginIdentity(identities []domain.UserIdentity, replacingEmail bool, replacingPhone bool, finalEmail string, finalPhone string) bool {
	if finalEmail != "" || finalPhone != "" {
		return true
	}
	for _, identity := range identities {
		switch identity.Type {
		case domain.IdentityOAuth:
			return true
		case domain.IdentityEmail:
			if !replacingEmail {
				return true
			}
		case domain.IdentityPhone:
			if !replacingPhone {
				return true
			}
		}
	}
	return false
}

func adminVerifiedIdentity(userID string, identityType domain.IdentityType, provider string, identifier string, now time.Time) domain.UserIdentity {
	return domain.UserIdentity{
		ID:         util.NewID(),
		UserID:     userID,
		Type:       identityType,
		Provider:   provider,
		Identifier: identifier,
		VerifiedAt: &now,
		IsPrimary:  true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// AdminUpdateUser allows admins to change user profile, role, and status.
func (a *App) AdminUpdateUser(admin domain.User, userID string, role *domain.UserRole, status *domain.UserStatus, email *string, phone *string, displayName *string, avatarURL *string, adminNote *string) (domain.AdminUser, error) {
	target, ok, err := a.store.GetUserByID(userID)
	if err != nil {
		return domain.AdminUser{}, fmt.Errorf("fetch user: %w", err)
	}
	if !ok {
		return domain.AdminUser{}, fmt.Errorf("user not found")
	}
	if target.ID == admin.ID {
		if role != nil && *role != admin.Role {
			return domain.AdminUser{}, fmt.Errorf("cannot change own role")
		}
		if status != nil && *status == domain.StatusDisabled {
			return domain.AdminUser{}, fmt.Errorf("cannot disable self")
		}
	}
	if role != nil {
		target.Role = *role
	}
	if status != nil {
		target.Status = *status
	}
	profile, err := a.profileForUpdate(target.ID)
	if err != nil {
		return domain.AdminUser{}, err
	}
	if displayName != nil {
		profile.DisplayName = sanitizeDisplayName(*displayName)
	}
	if avatarURL != nil && strings.TrimSpace(profile.AvatarStorageKey) == "" {
		profile.AvatarURL = normalizeAvatarURL(*avatarURL)
	}
	if adminNote != nil {
		profile.AdminNote = strings.TrimSpace(*adminNote)
	}
	identitiesByUser, err := a.store.ListUserIdentities([]string{target.ID})
	if err != nil {
		return domain.AdminUser{}, fmt.Errorf("fetch user identities: %w", err)
	}
	finalEmail, finalPhone := currentUserContactIdentities(identitiesByUser[target.ID], target)
	var normalizedEmail string
	var normalizedPhone string
	if email != nil {
		normalizedEmail, err = normalizeAdminEmail(*email)
		if err != nil {
			return domain.AdminUser{}, err
		}
		if normalizedEmail != "" {
			existing, ok, err := a.store.GetUserByIdentity(domain.IdentityEmail, normalizedEmail)
			if err != nil {
				return domain.AdminUser{}, fmt.Errorf("check email identity: %w", err)
			}
			if ok && existing.ID != target.ID {
				return domain.AdminUser{}, ErrIdentifierAlreadyExists
			}
		}
		finalEmail = normalizedEmail
	}
	if phone != nil {
		normalizedPhone, err = normalizeAdminPhone(*phone)
		if err != nil {
			return domain.AdminUser{}, err
		}
		if normalizedPhone != "" {
			existing, ok, err := a.store.GetUserByIdentity(domain.IdentityPhone, normalizedPhone)
			if err != nil {
				return domain.AdminUser{}, fmt.Errorf("check phone identity: %w", err)
			}
			if ok && existing.ID != target.ID {
				return domain.AdminUser{}, ErrIdentifierAlreadyExists
			}
		}
		finalPhone = normalizedPhone
	}
	if (email != nil || phone != nil) && !hasRemainingLoginIdentity(identitiesByUser[target.ID], email != nil, phone != nil, finalEmail, finalPhone) {
		return domain.AdminUser{}, fmt.Errorf("user must have at least one login identity")
	}
	target.UpdatedAt = time.Now().UTC()
	target.Email = finalEmail
	if target.Email == "" {
		target.Email = finalPhone
	}
	if err := a.store.SaveUser(target); err != nil {
		return domain.AdminUser{}, fmt.Errorf("update user: %w", err)
	}
	if displayName != nil || avatarURL != nil || adminNote != nil {
		profile.UpdatedAt = target.UpdatedAt
		if err := a.store.SaveUserProfile(profile); err != nil {
			return domain.AdminUser{}, fmt.Errorf("update profile: %w", err)
		}
	}
	if email != nil {
		if normalizedEmail == "" {
			if err := a.store.DeleteUserIdentity(target.ID, domain.IdentityEmail); err != nil {
				return domain.AdminUser{}, fmt.Errorf("delete email identity: %w", err)
			}
		} else if err := a.store.SaveUserIdentity(adminVerifiedIdentity(target.ID, domain.IdentityEmail, "", normalizedEmail, target.UpdatedAt)); err != nil {
			return domain.AdminUser{}, fmt.Errorf("update email identity: %w", err)
		}
	}
	if phone != nil {
		if normalizedPhone == "" {
			if err := a.store.DeleteUserIdentity(target.ID, domain.IdentityPhone); err != nil {
				return domain.AdminUser{}, fmt.Errorf("delete phone identity: %w", err)
			}
		} else if err := a.store.SaveUserIdentity(adminVerifiedIdentity(target.ID, domain.IdentityPhone, "", normalizedPhone, target.UpdatedAt)); err != nil {
			return domain.AdminUser{}, fmt.Errorf("update phone identity: %w", err)
		}
	}
	if status != nil && *status == domain.StatusDisabled {
		if err := a.revokeAllUserTokens(target.ID, target.UpdatedAt); err != nil {
			return domain.AdminUser{}, fmt.Errorf("revoke disabled user tokens: %w", err)
		}
	}
	adminUsers, err := a.adminUsersFromUsers([]domain.User{target})
	if err != nil {
		return domain.AdminUser{}, err
	}
	return adminUsers[0], nil
}

func (a *App) AdminDeleteUser(admin domain.User, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user id required")
	}
	target, ok, err := a.store.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("fetch user: %w", err)
	}
	if !ok {
		return fmt.Errorf("user not found")
	}
	if target.ID == admin.ID {
		return fmt.Errorf("cannot delete self")
	}
	if err := a.revokeAllUserTokens(target.ID, time.Now().UTC()); err != nil {
		return fmt.Errorf("revoke deleted user tokens: %w", err)
	}
	if err := a.store.DeleteUser(target.ID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// SaveAdminAuditLog persists an admin audit event.
func (a *App) SaveAdminAuditLog(entry domain.AdminAuditLog) (domain.AdminAuditLog, error) {
	entry.ActorID = strings.TrimSpace(entry.ActorID)
	entry.Action = strings.TrimSpace(entry.Action)
	entry.TargetType = strings.TrimSpace(entry.TargetType)
	entry.TargetID = strings.TrimSpace(entry.TargetID)
	if entry.ActorID == "" || entry.Action == "" || entry.TargetType == "" || entry.TargetID == "" {
		return domain.AdminAuditLog{}, fmt.Errorf("actorId, action, targetType and targetId are required")
	}
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = util.NewID()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	} else {
		entry.CreatedAt = entry.CreatedAt.UTC()
	}
	if err := a.store.SaveAdminAuditLog(entry); err != nil {
		return domain.AdminAuditLog{}, fmt.Errorf("save admin audit log: %w", err)
	}
	return entry, nil
}

// ListAdminAuditLogs returns paginated admin audit logs.
func (a *App) ListAdminAuditLogs(opts store.AdminAuditLogListOptions) ([]domain.AdminAuditLog, int, error) {
	return a.store.ListAdminAuditLogs(opts)
}

// GetAdminOverview returns aggregate admin metrics.
func (a *App) GetAdminOverview(windowStart time.Time, windowHours int) (domain.AdminOverview, error) {
	return a.store.GetAdminOverview(windowStart, windowHours)
}

// JWKS returns public signing keys when session store supports it.
func (a *App) JWKS() []store.JWK {
	provider, ok := a.sessions.(store.JWKSProvider)
	if !ok {
		return nil
	}
	return provider.JWKS()
}

func (a *App) revokeAllUserTokens(userID string, since time.Time) error {
	if userID == "" {
		return nil
	}
	sessionRevoker, ok := a.sessions.(store.UserSessionRevoker)
	if !ok {
		return fmt.Errorf("session store does not support user token revocation")
	}
	if err := sessionRevoker.RevokeUserSessions(userID, since); err != nil {
		return err
	}
	refreshRevoker, ok := a.refreshTokens.(store.UserRefreshTokenRevoker)
	if !ok {
		return fmt.Errorf("refresh token store does not support user token revocation")
	}
	return refreshRevoker.RevokeUserRefreshTokens(userID)
}

func (a *App) createUser(email, passwordHash string, role domain.UserRole) (domain.User, error) {
	now := time.Now().UTC()
	user := domain.User{
		ID:           util.NewID(),
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		Status:       domain.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := a.store.SaveUser(user); err != nil {
		return domain.User{}, fmt.Errorf("save user: %w", err)
	}
	return user, nil
}

func (a *App) createUserWithIdentity(identityType domain.IdentityType, identifier, passwordHash string, role domain.UserRole) (domain.User, error) {
	now := time.Now().UTC()
	displayEmail := ""
	if identityType == domain.IdentityEmail || identityType == domain.IdentityPhone {
		displayEmail = identifier
	}
	user := domain.User{
		ID:           util.NewID(),
		Email:        displayEmail,
		PasswordHash: passwordHash,
		Role:         role,
		Status:       domain.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	identity := domain.UserIdentity{
		ID:         util.NewID(),
		UserID:     user.ID,
		Type:       identityType,
		Identifier: identifier,
		VerifiedAt: &now,
		IsPrimary:  true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := a.store.SaveUserWithIdentity(user, identity); err != nil {
		return domain.User{}, fmt.Errorf("save user identity: %w", err)
	}
	return user, nil
}

func (a *App) createOAuthUser(provider, subject, email string, emailVerified bool, role domain.UserRole, displayName string, avatarURL string) (domain.User, error) {
	now := time.Now().UTC()
	displayEmail := ""
	if emailVerified {
		displayEmail = email
	}
	user := domain.User{
		ID:           util.NewID(),
		Email:        displayEmail,
		PasswordHash: "",
		Role:         role,
		Status:       domain.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	identities := []domain.UserIdentity{{
		ID:         util.NewID(),
		UserID:     user.ID,
		Type:       domain.IdentityOAuth,
		Provider:   provider,
		Identifier: subject,
		VerifiedAt: &now,
		IsPrimary:  !emailVerified || email == "",
		CreatedAt:  now,
		UpdatedAt:  now,
	}}
	if emailVerified && email != "" {
		identities = append(identities, domain.UserIdentity{
			ID:         util.NewID(),
			UserID:     user.ID,
			Type:       domain.IdentityEmail,
			Identifier: email,
			VerifiedAt: &now,
			IsPrimary:  true,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}
	if err := a.store.SaveUserWithIdentities(user, identities); err != nil {
		return domain.User{}, fmt.Errorf("save oauth user identities: %w", err)
	}
	if err := a.ensureUserProfile(user.ID, displayName, avatarURL); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func isSupportedIdentityType(identityType domain.IdentityType) bool {
	return identityType == domain.IdentityEmail || identityType == domain.IdentityPhone
}

func hasPassword(passwordHash string) bool {
	return strings.TrimSpace(passwordHash) != ""
}

func normalizeOAuthProvider(provider string) string {
	normalized := strings.TrimSpace(strings.ToLower(provider))
	switch normalized {
	case "google", "microsoft":
		return normalized
	default:
		return ""
	}
}

func normalizeOAuthEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return ""
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address == "" || strings.Contains(parsed.Address, " ") {
		return ""
	}
	return strings.ToLower(parsed.Address)
}

func normalizeAdminEmail(email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", nil
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", fmt.Errorf("email format is invalid")
	}
	return email, nil
}

func normalizeAdminPhone(phone string) (string, error) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	if digits == "" {
		return "", nil
	}
	if strings.HasPrefix(digits, "0086") {
		digits = strings.TrimPrefix(digits, "0086")
	}
	if strings.HasPrefix(digits, "86") && len(digits) == 13 {
		digits = strings.TrimPrefix(digits, "86")
	}
	if len(digits) != 11 || !strings.HasPrefix(digits, "1") {
		return "", fmt.Errorf("phone format is invalid")
	}
	return "+86" + digits, nil
}

func sanitizeDisplayName(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 80 {
		value = value[:80]
	}
	return value
}

func normalizeAvatarURL(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 2048 {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "/") {
		return value
	}
	return ""
}

func effectiveAvatarURL(userID string, profile domain.UserProfile) string {
	if strings.TrimSpace(profile.AvatarStorageKey) != "" {
		return "/api/users/" + strings.TrimSpace(userID) + "/avatar"
	}
	return strings.TrimSpace(profile.AvatarURL)
}

func avatarContentType(data []byte, filename string) (string, string, error) {
	if len(data) == 0 {
		return "", "", fmt.Errorf("avatar file is empty")
	}
	detected := httpDetectContentType(data)
	switch detected {
	case "image/jpeg":
		return detected, ".jpg", nil
	case "image/png":
		return detected, ".png", nil
	case "image/gif":
		return detected, ".gif", nil
	case "image/webp":
		return detected, ".webp", nil
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".webp" && bytes.HasPrefix(data, []byte("RIFF")) && len(data) > 12 && string(data[8:12]) == "WEBP" {
		return "image/webp", ".webp", nil
	}
	return "", "", fmt.Errorf("avatar must be jpg, png, webp, or gif")
}

func httpDetectContentType(data []byte) string {
	sniff := data
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	return http.DetectContentType(sniff)
}
