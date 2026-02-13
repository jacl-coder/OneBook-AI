package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"onebookai/internal/util"
	"onebookai/pkg/auth"
	"onebookai/pkg/domain"
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
}

// App is the core application service wiring together storage and auth logic.
type App struct {
	store         store.Store
	sessions      store.SessionStore
	refreshTokens store.RefreshTokenStore
	refreshTTL    time.Duration
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

	return &App{
		store:         dataStore,
		sessions:      sessionStore,
		refreshTokens: refreshStore,
		refreshTTL:    cfg.RefreshTTL,
	}, nil
}

// SignUp registers a new user with default role user.
func (a *App) SignUp(email, password string) (domain.User, string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return domain.User{}, "", "", ErrEmailAndPasswordRequired
	}
	if err := auth.ValidatePassword(password); err != nil {
		return domain.User{}, "", "", err
	}
	exists, err := a.store.HasUserEmail(email)
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("check email: %w", err)
	}
	if exists {
		return domain.User{}, "", "", ErrEmailAlreadyExists
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
	user, err := a.createUser(email, passwordHash, role)
	if err != nil {
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
	exists, err := a.store.HasUserEmail(email)
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("check email: %w", err)
	}
	if exists {
		return domain.User{}, "", "", ErrEmailAlreadyExists
	}
	role := domain.RoleUser
	count, err := a.store.UserCount()
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("count users: %w", err)
	}
	if count == 0 {
		role = domain.RoleAdmin
	}
	user, err := a.createUser(email, "", role)
	if err != nil {
		return domain.User{}, "", "", err
	}
	return a.issueUserTokens(user)
}

// Login validates credentials and issues a session token.
func (a *App) Login(email, password string) (domain.User, string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, ok, err := a.store.GetUserByEmail(email)
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
	return a.issueUserTokens(user)
}

// LoginByEmail issues a session token for an existing account.
func (a *App) LoginByEmail(email string) (domain.User, string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, ok, err := a.store.GetUserByEmail(email)
	if err != nil {
		return domain.User{}, "", "", fmt.Errorf("fetch user: %w", err)
	}
	if !ok {
		return domain.User{}, "", "", ErrInvalidCredentials
	}
	if user.Status == domain.StatusDisabled {
		return domain.User{}, "", "", ErrUserDisabled
	}
	return a.issueUserTokens(user)
}

func (a *App) issueUserTokens(user domain.User) (domain.User, string, string, error) {
	accessToken, refreshToken, err := a.issueTokens(user.ID)
	if err != nil {
		return domain.User{}, "", "", err
	}
	return user, accessToken, refreshToken, nil
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
	return user, true
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
	return a.store.HasUserEmail(email)
}

// ListUsers returns all users (admin use only).
func (a *App) ListUsers() ([]domain.User, error) {
	return a.store.ListUsers()
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
	user.Email = email
	user.UpdatedAt = time.Now().UTC()
	if err := a.store.SaveUser(user); err != nil {
		return domain.User{}, fmt.Errorf("update user: %w", err)
	}
	return user, nil
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
			return fmt.Errorf("new password must differ from current password")
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

// AdminUpdateUser allows admins to change role/status.
func (a *App) AdminUpdateUser(admin domain.User, userID string, role *domain.UserRole, status *domain.UserStatus) (domain.User, error) {
	target, ok, err := a.store.GetUserByID(userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("fetch user: %w", err)
	}
	if !ok {
		return domain.User{}, fmt.Errorf("user not found")
	}
	if target.ID == admin.ID {
		if role != nil && *role != admin.Role {
			return domain.User{}, fmt.Errorf("cannot change own role")
		}
		if status != nil && *status == domain.StatusDisabled {
			return domain.User{}, fmt.Errorf("cannot disable self")
		}
	}
	if role != nil {
		target.Role = *role
	}
	if status != nil {
		target.Status = *status
	}
	target.UpdatedAt = time.Now().UTC()
	if err := a.store.SaveUser(target); err != nil {
		return domain.User{}, fmt.Errorf("update user: %w", err)
	}
	if status != nil && *status == domain.StatusDisabled {
		if err := a.revokeAllUserTokens(target.ID, target.UpdatedAt); err != nil {
			return domain.User{}, fmt.Errorf("revoke disabled user tokens: %w", err)
		}
	}
	return target, nil
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

func hasPassword(passwordHash string) bool {
	return strings.TrimSpace(passwordHash) != ""
}
