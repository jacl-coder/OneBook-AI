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
	DatabaseURL   string
	RedisAddr     string
	RedisPassword string
	SessionTTL    time.Duration
	JWTSecret     string
	Store         store.Store
	Sessions      store.SessionStore
}

// App is the core application service wiring together storage and auth logic.
type App struct {
	store    store.Store
	sessions store.SessionStore
}

// New constructs the application with database storage and session management.
func New(cfg Config) (*App, error) {
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 24 * time.Hour
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
		switch {
		case cfg.JWTSecret != "":
			var revoker store.TokenRevoker
			if cfg.RedisAddr != "" {
				revoker = store.NewRedisTokenRevoker(cfg.RedisAddr, cfg.RedisPassword)
			} else {
				revoker = store.NewMemoryTokenRevoker()
			}
			sessionStore = store.NewJWTSessionStore(cfg.JWTSecret, cfg.SessionTTL, revoker)
		case cfg.RedisAddr != "":
			sessionStore = store.NewRedisSessionStore(cfg.RedisAddr, cfg.RedisPassword, cfg.SessionTTL)
		default:
			return nil, fmt.Errorf("session store required (jwtSecret or redisAddr)")
		}
	}

	return &App{
		store:    dataStore,
		sessions: sessionStore,
	}, nil
}

// SignUp registers a new user with default role user.
func (a *App) SignUp(email, password string) (domain.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return domain.User{}, "", errors.New("email and password required")
	}
	exists, err := a.store.HasUserEmail(email)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("check email: %w", err)
	}
	if exists {
		return domain.User{}, "", fmt.Errorf("email already exists")
	}
	role := domain.RoleUser
	count, err := a.store.UserCount()
	if err != nil {
		return domain.User{}, "", fmt.Errorf("count users: %w", err)
	}
	if count == 0 {
		role = domain.RoleAdmin
	}
	now := time.Now().UTC()
	user := domain.User{
		ID:           util.NewID(),
		Email:        email,
		PasswordHash: auth.HashPassword(password),
		Role:         role,
		Status:       domain.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := a.store.SaveUser(user); err != nil {
		return domain.User{}, "", fmt.Errorf("save user: %w", err)
	}
	token, err := a.sessions.NewSession(user.ID)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("issue session: %w", err)
	}
	return user, token, nil
}

// Login validates credentials and issues a session token.
func (a *App) Login(email, password string) (domain.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, ok, err := a.store.GetUserByEmail(email)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("fetch user: %w", err)
	}
	if !ok {
		return domain.User{}, "", fmt.Errorf("invalid credentials")
	}
	if user.Status == domain.StatusDisabled {
		return domain.User{}, "", fmt.Errorf("user disabled")
	}
	if !auth.CheckPassword(password, user.PasswordHash) {
		return domain.User{}, "", fmt.Errorf("invalid credentials")
	}
	token, err := a.sessions.NewSession(user.ID)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("issue session: %w", err)
	}
	return user, token, nil
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

// Logout removes a session token.
func (a *App) Logout(token string) error {
	return a.sessions.DeleteSession(token)
}

// ListUsers returns all users (admin use only).
func (a *App) ListUsers() ([]domain.User, error) {
	return a.store.ListUsers()
}

// UpdateMyEmail updates the current user's email.
func (a *App) UpdateMyEmail(user domain.User, newEmail string) (domain.User, error) {
	email := strings.TrimSpace(strings.ToLower(newEmail))
	if email == "" {
		return domain.User{}, fmt.Errorf("email required")
	}
	if email == user.Email {
		return user, nil
	}
	existing, ok, err := a.store.GetUserByEmail(email)
	if err != nil {
		return domain.User{}, fmt.Errorf("check email: %w", err)
	}
	if ok && existing.ID != user.ID {
		return domain.User{}, fmt.Errorf("email already exists")
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
		return fmt.Errorf("new password required")
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
	if !auth.CheckPassword(currentPassword, user.PasswordHash) {
		return fmt.Errorf("invalid credentials")
	}
	user.PasswordHash = auth.HashPassword(newPassword)
	user.UpdatedAt = time.Now().UTC()
	if err := a.store.SaveUser(user); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
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
	return target, nil
}
