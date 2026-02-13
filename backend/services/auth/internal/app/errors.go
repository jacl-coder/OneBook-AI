package app

import "errors"

var (
	// ErrInvalidCredentials is returned when the supplied credentials do not match.
	// Keep the message stable to avoid breaking clients that display it.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrUserDisabled is returned when an account is disabled.
	// Handlers should generally NOT expose this to clients to avoid account enumeration.
	ErrUserDisabled = errors.New("user disabled")

	ErrEmailAndPasswordRequired = errors.New("email and password required")
	ErrEmailAlreadyExists       = errors.New("email already exists")

	ErrRefreshTokenRequired = errors.New("refresh token required")
	ErrInvalidRefreshToken  = errors.New("invalid refresh token")

	ErrEmailRequired = errors.New("email required")
)
