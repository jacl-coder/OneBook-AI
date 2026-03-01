package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"onebookai/internal/ratelimit"
	"onebookai/internal/util"
	"onebookai/pkg/domain"
	"onebookai/pkg/store"
	"onebookai/services/auth/internal/app"
	"onebookai/services/auth/internal/security"
)

// Config wires required dependencies for the HTTP server.
type Config struct {
	App                        *app.App
	RedisAddr                  string
	RedisPassword              string
	TrustedProxyCIDRs          []string
	SignupRateLimitPerMinute   int
	LoginRateLimitPerMinute    int
	RefreshRateLimitPerMinute  int
	PasswordRateLimitPerMinute int
}

// Server exposes HTTP endpoints for the auth service.
type Server struct {
	app             *app.App
	otp             *otpStore
	mux             *http.ServeMux
	signupLimiter   *ratelimit.FixedWindowLimiter
	loginLimiter    *ratelimit.FixedWindowLimiter
	refreshLimiter  *ratelimit.FixedWindowLimiter
	passwordLimiter *ratelimit.FixedWindowLimiter
	trustedProxies  *util.TrustedProxies
	alerter         *security.AuditAlerter
}

// New constructs the server with routes configured.
func New(cfg Config) (*Server, error) {
	signupLimit := cfg.SignupRateLimitPerMinute
	if signupLimit <= 0 {
		signupLimit = 5
	}
	loginLimit := cfg.LoginRateLimitPerMinute
	if loginLimit <= 0 {
		loginLimit = 10
	}
	refreshLimit := cfg.RefreshRateLimitPerMinute
	if refreshLimit <= 0 {
		refreshLimit = 20
	}
	passwordLimit := cfg.PasswordRateLimitPerMinute
	if passwordLimit <= 0 {
		passwordLimit = 10
	}
	rateWindow := time.Minute
	newLimiter := func(name string, limit int) (*ratelimit.FixedWindowLimiter, error) {
		prefix := "onebook:auth:ratelimit:" + name
		limiter, err := ratelimit.NewRedisFixedWindowLimiter(cfg.RedisAddr, cfg.RedisPassword, prefix, limit, rateWindow)
		if err != nil {
			return nil, fmt.Errorf("init %s limiter: %w", name, err)
		}
		return limiter, nil
	}
	signupLimiter, err := newLimiter("signup", signupLimit)
	if err != nil {
		return nil, err
	}
	loginLimiter, err := newLimiter("login", loginLimit)
	if err != nil {
		return nil, err
	}
	refreshLimiter, err := newLimiter("refresh", refreshLimit)
	if err != nil {
		return nil, err
	}
	passwordLimiter, err := newLimiter("password", passwordLimit)
	if err != nil {
		return nil, err
	}
	trustedProxies, err := util.NewTrustedProxies(cfg.TrustedProxyCIDRs)
	if err != nil {
		return nil, fmt.Errorf("parse trustedProxyCIDRs: %w", err)
	}
	otpStore, err := newOTPStore(cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		return nil, fmt.Errorf("init otp store: %w", err)
	}
	s := &Server{
		app:             cfg.App,
		otp:             otpStore,
		mux:             http.NewServeMux(),
		signupLimiter:   signupLimiter,
		loginLimiter:    loginLimiter,
		refreshLimiter:  refreshLimiter,
		passwordLimiter: passwordLimiter,
		trustedProxies:  trustedProxies,
		alerter:         security.NewAuditAlerter(cfg.RedisAddr, cfg.RedisPassword, "onebook:auth:alerts"),
	}
	s.routes()
	return s, nil
}

// Router returns the configured handler.
func (s *Server) Router() http.Handler {
	return util.WithRequestID(util.WithRequestLog("auth", util.WithSecurityHeaders(s.mux)))
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)

	// auth
	s.mux.HandleFunc("/auth/signup", s.handleSignup)
	s.mux.HandleFunc("/auth/login", s.handleLogin)
	s.mux.HandleFunc("/auth/login/methods", s.handleLoginMethods)
	s.mux.HandleFunc("/auth/otp/send", s.handleOTPSend)
	s.mux.HandleFunc("/auth/otp/verify", s.handleOTPVerify)
	s.mux.HandleFunc("/auth/password/reset/verify", s.handlePasswordResetVerify)
	s.mux.HandleFunc("/auth/password/reset/complete", s.handlePasswordResetComplete)
	s.mux.HandleFunc("/auth/refresh", s.handleRefresh)
	s.mux.HandleFunc("/auth/logout", s.handleLogout)
	s.mux.HandleFunc("/auth/jwks", s.handleJWKS)
	s.mux.HandleFunc("/.well-known/jwks.json", s.handleJWKS)
	s.mux.Handle("/auth/me", s.authenticated(s.handleMe))
	s.mux.Handle("/auth/me/password", s.authenticated(s.handleChangePassword))

	// admin
	s.mux.Handle("/auth/admin/users", s.adminOnly(s.handleAdminUsers))
	s.mux.Handle("/auth/admin/users/", s.adminOnly(s.handleAdminUserByID))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func isPasswordPolicyError(err error) bool {
	if err == nil {
		return false
	}
	// backend/pkg/auth.ValidatePassword returns errors starting with this prefix.
	return strings.HasPrefix(err.Error(), "password must")
}

// auth wrappers
type authHandler func(http.ResponseWriter, *http.Request, domain.User)

func (s *Server) authenticated(next authHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.authorize(r)
		if !ok {
			s.audit(r, "auth.authorize", "fail")
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		s.audit(r, "auth.authorize", "success", "user_id", user.ID)
		next(w, r, user)
	})
}

func (s *Server) adminOnly(next authHandler) http.Handler {
	return s.authenticated(func(w http.ResponseWriter, r *http.Request, user domain.User) {
		if user.Role != domain.RoleAdmin {
			s.audit(r, "auth.admin.authorize", "fail", "user_id", user.ID)
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		s.audit(r, "auth.admin.authorize", "success", "user_id", user.ID)
		next(w, r, user)
	})
}

func (s *Server) authorize(r *http.Request) (domain.User, bool) {
	token, ok := bearerToken(r)
	if !ok {
		return domain.User{}, false
	}
	return s.app.UserFromToken(token)
}

// auth handlers
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.signupLimiter, "too many signup attempts") {
		s.audit(r, "auth.signup", "rate_limited")
		return
	}
	var req authRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.signup", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, accessToken, refreshToken, err := s.app.SignUp(req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrEmailAndPasswordRequired):
			s.audit(r, "auth.signup", "fail", "reason", "missing_fields")
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, app.ErrEmailAlreadyExists):
			s.audit(r, "auth.signup", "fail", "reason", "email_exists")
			writeError(w, http.StatusBadRequest, err.Error())
		case isPasswordPolicyError(err):
			s.audit(r, "auth.signup", "fail", "reason", "invalid_password")
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			util.LoggerFromContext(r.Context()).Error("auth.signup_error", "err", err)
			s.audit(r, "auth.signup", "fail", "reason", "internal_error")
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	s.audit(r, "auth.signup", "success", "user_id", user.ID)
	writeJSON(w, http.StatusCreated, authResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.loginLimiter, "too many login attempts") {
		s.audit(r, "auth.login", "rate_limited")
		return
	}
	var req authRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.login", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, accessToken, refreshToken, err := s.app.Login(req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrPasswordNotSet):
			s.audit(r, "auth.login", "fail", "reason", "password_not_set")
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, app.ErrUserDisabled):
			s.audit(r, "auth.login", "fail", "reason", "user_disabled")
			writeError(w, http.StatusUnauthorized, app.ErrInvalidCredentials.Error())
		case errors.Is(err, app.ErrInvalidCredentials):
			s.audit(r, "auth.login", "fail", "reason", "invalid_credentials")
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			util.LoggerFromContext(r.Context()).Error("auth.login_error", "err", err)
			s.audit(r, "auth.login", "fail", "reason", "internal_error")
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	s.audit(r, "auth.login", "success", "user_id", user.ID)
	writeJSON(w, http.StatusOK, authResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

func (s *Server) handleLoginMethods(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req loginMethodsRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.login.methods", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		s.audit(r, "auth.login.methods", "fail", "reason", "invalid_email")
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := strings.Join([]string{r.URL.Path, util.ClientIP(r, s.trustedProxies), email}, "|")
	if !s.allowRateKey(w, s.loginLimiter, key, "too many login method checks") {
		s.audit(r, "auth.login.methods", "rate_limited")
		return
	}
	passwordLogin, err := s.app.CanLoginWithPassword(email)
	if err != nil {
		util.LoggerFromContext(r.Context()).Error("auth.login.methods_error", "err", err)
		s.audit(r, "auth.login.methods", "fail", "reason", "internal_error")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.audit(r, "auth.login.methods", "success")
	writeJSON(w, http.StatusOK, loginMethodsResponse{PasswordLogin: passwordLogin})
}

func (s *Server) handleOTPSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req otpSendRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.otp.send", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		s.audit(r, "auth.otp.send", "fail", "reason", "invalid_email")
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	purpose := strings.TrimSpace(strings.ToLower(req.Purpose))
	if !isValidOTPPurpose(purpose) {
		s.audit(r, "auth.otp.send", "fail", "reason", "invalid_purpose")
		writeError(w, http.StatusBadRequest, errOTPPurposeInvalid.Error())
		return
	}
	key := strings.Join([]string{r.URL.Path, util.ClientIP(r, s.trustedProxies), email}, "|")
	if !s.allowRateKey(w, s.signupLimiter, key, errOTPSendRateLimited.Error()) {
		s.audit(r, "auth.otp.send", "rate_limited")
		return
	}
	if purpose == otpPurposeSignupPassword || purpose == otpPurposeSignupOTP {
		exists, err := s.app.HasUserEmail(email)
		if err != nil {
			util.LoggerFromContext(r.Context()).Error("auth.otp.send_error", "err", err)
			s.audit(r, "auth.otp.send", "fail", "reason", "internal_error")
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if exists {
			s.audit(r, "auth.otp.send", "fail", "reason", "email_exists")
			writeError(w, http.StatusBadRequest, app.ErrEmailAlreadyExists.Error())
			return
		}
	}
	challengeID, code, expiresIn, resendAfter, err := s.otp.CreateChallenge(email, purpose)
	if err != nil {
		switch {
		case errors.Is(err, errOTPSendRateLimited):
			s.audit(r, "auth.otp.send", "rate_limited")
			writeError(w, http.StatusTooManyRequests, err.Error())
		case errors.Is(err, errOTPPurposeInvalid):
			s.audit(r, "auth.otp.send", "fail", "reason", "invalid_purpose")
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			util.LoggerFromContext(r.Context()).Error("auth.otp.send_error", "err", err)
			s.audit(r, "auth.otp.send", "fail", "reason", "internal_error")
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	util.LoggerFromContext(r.Context()).Info(
		"auth_otp_sent",
		"email", maskEmail(email),
		"purpose", purpose,
		"challenge_id", challengeID,
		"otp_code", code,
		"request_id", util.RequestIDFromRequest(r),
	)
	s.audit(r, "auth.otp.send", "success")
	writeJSON(w, http.StatusAccepted, otpSendResponse{
		ChallengeID:        challengeID,
		ExpiresInSeconds:   expiresIn,
		ResendAfterSeconds: resendAfter,
		MaskedEmail:        maskEmail(email),
	})
}

func (s *Server) handleOTPVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req otpVerifyRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.otp.verify", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		s.audit(r, "auth.otp.verify", "fail", "reason", "invalid_email")
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	purpose := strings.TrimSpace(strings.ToLower(req.Purpose))
	if !isValidOTPPurpose(purpose) {
		s.audit(r, "auth.otp.verify", "fail", "reason", "invalid_purpose")
		writeError(w, http.StatusBadRequest, errOTPPurposeInvalid.Error())
		return
	}
	if purpose == otpPurposeResetPassword {
		s.audit(r, "auth.otp.verify", "fail", "reason", "unsupported_purpose")
		writeError(w, http.StatusBadRequest, "use password reset verification endpoint")
		return
	}
	if purpose == otpPurposeSignupPassword && strings.TrimSpace(req.Password) == "" {
		s.audit(r, "auth.otp.verify", "fail", "reason", "missing_password")
		writeError(w, http.StatusBadRequest, "password is required for password sign-up")
		return
	}
	key := strings.Join([]string{r.URL.Path, util.ClientIP(r, s.trustedProxies), email}, "|")
	if !s.allowRateKey(w, s.loginLimiter, key, errOTPVerifyRateLimited.Error()) {
		s.audit(r, "auth.otp.verify", "rate_limited")
		return
	}
	if err := s.otp.VerifyChallenge(req.ChallengeID, email, purpose, req.Code); err != nil {
		switch {
		case errors.Is(err, errOTPChallengeInvalid):
			s.audit(r, "auth.otp.verify", "fail", "reason", "challenge_invalid")
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, errOTPCodeInvalid):
			s.audit(r, "auth.otp.verify", "fail", "reason", "code_invalid")
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, errOTPCodeExpired):
			s.audit(r, "auth.otp.verify", "fail", "reason", "code_expired")
			writeError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, errOTPChallengeRequired), errors.Is(err, errOTPCodeRequired), errors.Is(err, errOTPPurposeInvalid):
			s.audit(r, "auth.otp.verify", "fail", "reason", "invalid_request")
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			util.LoggerFromContext(r.Context()).Error("auth.otp.verify_error", "err", err)
			s.audit(r, "auth.otp.verify", "fail", "reason", "internal_error")
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	user, accessToken, refreshToken, err := s.completeOTPFlow(purpose, email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrEmailAlreadyExists), errors.Is(err, app.ErrEmailAndPasswordRequired), errors.Is(err, app.ErrEmailRequired), isPasswordPolicyError(err):
			s.audit(r, "auth.otp.verify", "fail", "reason", "invalid_request")
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, app.ErrUserDisabled), errors.Is(err, app.ErrInvalidCredentials):
			s.audit(r, "auth.otp.verify", "fail", "reason", "invalid_credentials")
			writeError(w, http.StatusUnauthorized, app.ErrInvalidCredentials.Error())
		default:
			util.LoggerFromContext(r.Context()).Error("auth.otp.verify_flow_error", "err", err)
			s.audit(r, "auth.otp.verify", "fail", "reason", "internal_error")
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	s.audit(r, "auth.otp.verify", "success", "user_id", user.ID, "purpose", purpose)
	writeJSON(w, http.StatusOK, authResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

func (s *Server) handlePasswordResetVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req passwordResetVerifyRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.password.reset.verify", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		s.audit(r, "auth.password.reset.verify", "fail", "reason", "invalid_email")
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := strings.Join([]string{r.URL.Path, util.ClientIP(r, s.trustedProxies), email}, "|")
	if !s.allowRateKey(w, s.loginLimiter, key, errPasswordResetVerifyRateLimited.Error()) {
		s.audit(r, "auth.password.reset.verify", "rate_limited")
		return
	}
	if err := s.otp.VerifyChallenge(req.ChallengeID, email, otpPurposeResetPassword, req.Code); err != nil {
		switch {
		case errors.Is(err, errOTPChallengeInvalid):
			s.audit(r, "auth.password.reset.verify", "fail", "reason", "challenge_invalid")
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, errOTPCodeInvalid):
			s.audit(r, "auth.password.reset.verify", "fail", "reason", "code_invalid")
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, errOTPCodeExpired):
			s.audit(r, "auth.password.reset.verify", "fail", "reason", "code_expired")
			writeError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, errOTPChallengeRequired), errors.Is(err, errOTPCodeRequired):
			s.audit(r, "auth.password.reset.verify", "fail", "reason", "invalid_request")
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			util.LoggerFromContext(r.Context()).Error("auth.password.reset.verify_error", "err", err)
			s.audit(r, "auth.password.reset.verify", "fail", "reason", "internal_error")
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	resetToken, expiresInSeconds, err := s.otp.CreateResetToken(email)
	if err != nil {
		util.LoggerFromContext(r.Context()).Error("auth.password.reset.token_error", "err", err)
		s.audit(r, "auth.password.reset.verify", "fail", "reason", "internal_error")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.audit(r, "auth.password.reset.verify", "success")
	writeJSON(w, http.StatusOK, passwordResetVerifyResponse{
		ResetToken:       resetToken,
		ExpiresInSeconds: expiresInSeconds,
	})
}

func (s *Server) handlePasswordResetComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.passwordLimiter, "too many password reset attempts") {
		s.audit(r, "auth.password.reset.complete", "rate_limited")
		return
	}
	var req passwordResetCompleteRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.password.reset.complete", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		s.audit(r, "auth.password.reset.complete", "fail", "reason", "invalid_email")
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.otp.ValidateResetToken(req.ResetToken, email); err != nil {
		switch {
		case errors.Is(err, errResetTokenRequired):
			s.audit(r, "auth.password.reset.complete", "fail", "reason", "token_required")
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, errResetTokenInvalid):
			s.audit(r, "auth.password.reset.complete", "fail", "reason", "token_invalid")
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			util.LoggerFromContext(r.Context()).Error("auth.password.reset.validate_error", "err", err)
			s.audit(r, "auth.password.reset.complete", "fail", "reason", "internal_error")
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	if err := s.app.ResetPasswordByEmail(email, req.NewPassword); err != nil {
		s.audit(r, "auth.password.reset.complete", "fail", "reason", err.Error())
		switch {
		case errors.Is(err, app.ErrEmailRequired), errors.Is(err, app.ErrNewPasswordRequired), isPasswordPolicyError(err):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, app.ErrInvalidCredentials), errors.Is(err, app.ErrUserDisabled):
			writeError(w, http.StatusUnauthorized, app.ErrInvalidCredentials.Error())
		default:
			util.LoggerFromContext(r.Context()).Error("auth.password.reset.complete_error", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	if err := s.otp.ConsumeResetToken(req.ResetToken); err != nil {
		util.LoggerFromContext(r.Context()).Warn("auth.password.reset.consume_token_error", "err", err)
	}
	s.audit(r, "auth.password.reset.complete", "success")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.refreshLimiter, "too many refresh attempts") {
		s.audit(r, "auth.refresh", "rate_limited")
		return
	}
	var req refreshRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.refresh", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, accessToken, refreshToken, err := s.app.Refresh(req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrRefreshTokenRequired):
			s.audit(r, "auth.refresh", "fail", "reason", "missing_refresh_token")
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, app.ErrInvalidRefreshToken):
			s.audit(r, "auth.refresh", "fail", "reason", "invalid_refresh_token")
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			util.LoggerFromContext(r.Context()).Error("auth.refresh_error", "err", err)
			s.audit(r, "auth.refresh", "fail", "reason", "internal_error")
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	s.audit(r, "auth.refresh", "success", "user_id", user.ID)
	writeJSON(w, http.StatusOK, authResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.refreshLimiter, "too many logout attempts") {
		s.audit(r, "auth.logout", "rate_limited")
		return
	}
	var req logoutRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		s.audit(r, "auth.logout", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		s.audit(r, "auth.logout", "fail", "reason", "missing_token")
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.app.Logout(token, req.RefreshToken); err != nil {
		s.audit(r, "auth.logout", "fail", "reason", err.Error())
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.audit(r, "auth.logout", "success")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	keys := s.app.JWKS()
	if len(keys) == 0 {
		writeError(w, http.StatusNotFound, "jwks not configured")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, jwksResponse{Keys: keys})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, user domain.User) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, user)
	case http.MethodPatch:
		var req updateMeRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.Email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}
		updated, err := s.app.UpdateMyEmail(user, req.Email)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.passwordLimiter, "too many password change attempts") {
		s.audit(r, "auth.password.change", "rate_limited", "user_id", user.ID)
		return
	}
	var req changePasswordRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.password.change", "fail", "user_id", user.ID, "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		s.audit(r, "auth.password.change", "fail", "user_id", user.ID, "reason", "missing_fields")
		writeError(w, http.StatusBadRequest, app.ErrNewPasswordRequired.Error())
		return
	}
	if err := s.app.ChangePassword(user.ID, req.CurrentPassword, req.NewPassword); err != nil {
		s.audit(r, "auth.password.change", "fail", "user_id", user.ID, "reason", err.Error())
		switch {
		case errors.Is(err, app.ErrCurrentPasswordRequired), errors.Is(err, app.ErrNewPasswordRequired), errors.Is(err, app.ErrInvalidCredentials), isPasswordPolicyError(err):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			util.LoggerFromContext(r.Context()).Error("auth.password.change_error", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	s.audit(r, "auth.password.change", "success", "user_id", user.ID)
	w.WriteHeader(http.StatusNoContent)
}

// admin handlers
func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	users, err := s.app.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": users,
		"count": len(users),
	})
}

func (s *Server) handleAdminUserByID(w http.ResponseWriter, r *http.Request, user domain.User) {
	id := strings.TrimPrefix(r.URL.Path, "/auth/admin/users/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPatch {
		methodNotAllowed(w)
		return
	}
	var req adminUserUpdateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	var role *domain.UserRole
	if req.Role != "" {
		parsed, ok := parseUserRole(req.Role)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid role")
			return
		}
		role = &parsed
	}
	var status *domain.UserStatus
	if req.Status != "" {
		parsed, ok := parseUserStatus(req.Status)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid status")
			return
		}
		status = &parsed
	}
	if role == nil && status == nil {
		writeError(w, http.StatusBadRequest, "role or status is required")
		return
	}
	updated, err := s.app.AdminUpdateUser(user, id, role, status)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginMethodsRequest struct {
	Email string `json:"email"`
}

type loginMethodsResponse struct {
	PasswordLogin bool `json:"passwordLogin"`
}

type otpSendRequest struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"`
}

type otpSendResponse struct {
	ChallengeID        string `json:"challengeId"`
	ExpiresInSeconds   int    `json:"expiresInSeconds"`
	ResendAfterSeconds int    `json:"resendAfterSeconds"`
	MaskedEmail        string `json:"maskedEmail,omitempty"`
}

type otpVerifyRequest struct {
	ChallengeID string `json:"challengeId"`
	Email       string `json:"email"`
	Purpose     string `json:"purpose"`
	Code        string `json:"code"`
	Password    string `json:"password,omitempty"`
}

type passwordResetVerifyRequest struct {
	ChallengeID string `json:"challengeId"`
	Email       string `json:"email"`
	Code        string `json:"code"`
}

type passwordResetVerifyResponse struct {
	ResetToken       string `json:"resetToken"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
}

type passwordResetCompleteRequest struct {
	Email       string `json:"email"`
	ResetToken  string `json:"resetToken"`
	NewPassword string `json:"newPassword"`
}

type authResponse struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refreshToken,omitempty"`
	User         domain.User `json:"user"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken,omitempty"`
}

type updateMeRequest struct {
	Email string `json:"email"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type adminUserUpdateRequest struct {
	Role   string `json:"role"`
	Status string `json:"status"`
}

type jwksResponse struct {
	Keys []store.JWK `json:"keys"`
}

func bearerToken(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		util.LoggerFromContext(r.Context()).Warn("missing bearer prefix", "path", r.URL.Path)
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		util.LoggerFromContext(r.Context()).Warn("empty bearer token", "path", r.URL.Path)
		return "", false
	}
	return token, true
}

func parseUserRole(role string) (domain.UserRole, bool) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case string(domain.RoleUser):
		return domain.RoleUser, true
	case string(domain.RoleAdmin):
		return domain.RoleAdmin, true
	default:
		return "", false
	}
}

func parseUserStatus(status string) (domain.UserStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case string(domain.StatusActive):
		return domain.StatusActive, true
	case string(domain.StatusDisabled):
		return domain.StatusDisabled, true
	default:
		return "", false
	}
}

func (s *Server) allowRate(w http.ResponseWriter, r *http.Request, limiter *ratelimit.FixedWindowLimiter, msg string) bool {
	key := strings.Join([]string{r.URL.Path, util.ClientIP(r, s.trustedProxies)}, "|")
	return s.allowRateKey(w, limiter, key, msg)
}

func (s *Server) allowRateKey(w http.ResponseWriter, limiter *ratelimit.FixedWindowLimiter, key, msg string) bool {
	if limiter.Allow(key) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeError(w, http.StatusTooManyRequests, msg)
	return false
}

func (s *Server) completeOTPFlow(purpose, email, password string) (domain.User, string, string, error) {
	switch purpose {
	case otpPurposeSignupPassword:
		return s.app.SignUp(email, password)
	case otpPurposeSignupOTP:
		return s.app.SignUpPasswordless(email)
	case otpPurposeLoginOTP:
		return s.app.LoginByEmail(email)
	default:
		return domain.User{}, "", "", errOTPPurposeInvalid
	}
}

func (s *Server) audit(r *http.Request, event, outcome string, attrs ...any) {
	ip := util.ClientIP(r, s.trustedProxies)
	logAttrs := []any{
		"event", event,
		"outcome", outcome,
		"path", r.URL.Path,
		"method", r.Method,
		"ip", ip,
		"request_id", util.RequestIDFromRequest(r),
	}
	logAttrs = append(logAttrs, attrs...)
	if s.alerter != nil && outcome != "success" {
		alert, err := s.alerter.Observe(event, outcome, ip)
		if err != nil {
			util.LoggerFromContext(r.Context()).Error("security_alert_error", "event", event, "outcome", outcome, "ip", ip, "err", err)
		} else if alert.Triggered {
			util.LoggerFromContext(r.Context()).Error(
				"security_alert",
				"event", event,
				"outcome", outcome,
				"ip", ip,
				"count", alert.Count,
				"threshold", alert.Threshold,
				"window", alert.Window.String(),
			)
		}
	}
	if outcome == "success" {
		util.LoggerFromContext(r.Context()).Info("security_event", logAttrs...)
		return
	}
	util.LoggerFromContext(r.Context()).Warn("security_event", logAttrs...)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type errorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"requestId,omitempty"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{
		Error:     msg,
		Code:      errorCodeForAuth(status, msg),
		RequestID: strings.TrimSpace(w.Header().Get("X-Request-Id")),
	})
}

func errorCodeForAuth(status int, msg string) string {
	message := strings.ToLower(strings.TrimSpace(msg))
	switch {
	case strings.Contains(message, "incorrect email address or password"), strings.Contains(message, "invalid credentials"):
		return "AUTH_INVALID_CREDENTIALS"
	case strings.Contains(message, "invalid refresh token"), strings.Contains(message, "refresh token required"), strings.Contains(message, "refresh token revoked"), strings.Contains(message, "refresh token expired"):
		if strings.Contains(message, "required") {
			return "AUTH_REFRESH_TOKEN_REQUIRED"
		}
		return "AUTH_INVALID_REFRESH_TOKEN"
	case strings.Contains(message, "email and password required"):
		return "AUTH_INVALID_REQUEST"
	case strings.Contains(message, "email already exists"):
		return "AUTH_EMAIL_ALREADY_EXISTS"
	case message == app.ErrPasswordNotSet.Error():
		return "AUTH_PASSWORD_NOT_SET"
	case strings.HasPrefix(message, "password must"):
		return "AUTH_PASSWORD_POLICY_VIOLATION"
	case message == errOTPChallengeInvalid.Error():
		return "AUTH_OTP_CHALLENGE_INVALID"
	case message == errOTPCodeInvalid.Error():
		return "AUTH_OTP_CODE_INVALID"
	case message == errOTPCodeExpired.Error():
		return "AUTH_OTP_CODE_EXPIRED"
	case message == errOTPSendRateLimited.Error():
		return "AUTH_OTP_SEND_RATE_LIMITED"
	case message == errOTPVerifyRateLimited.Error():
		return "AUTH_OTP_VERIFY_RATE_LIMITED"
	case message == errPasswordResetVerifyRateLimited.Error():
		return "AUTH_PASSWORD_RESET_VERIFY_RATE_LIMITED"
	case message == errOTPPurposeInvalid.Error():
		return "AUTH_INVALID_REQUEST"
	case message == errOTPCodeRequired.Error(), message == errOTPChallengeRequired.Error():
		return "AUTH_INVALID_REQUEST"
	case message == errResetTokenRequired.Error():
		return "AUTH_PASSWORD_RESET_TOKEN_REQUIRED"
	case message == errResetTokenInvalid.Error():
		return "AUTH_PASSWORD_RESET_TOKEN_INVALID"
	case message == "email required", message == "email is required":
		return "AUTH_EMAIL_REQUIRED"
	case message == app.ErrCurrentPasswordRequired.Error():
		return "AUTH_CURRENT_PASSWORD_REQUIRED"
	case message == app.ErrNewPasswordRequired.Error():
		return "AUTH_NEW_PASSWORD_REQUIRED"
	case message == "invalid role":
		return "ADMIN_INVALID_ROLE"
	case message == "invalid status":
		return "ADMIN_INVALID_STATUS"
	case message == "role or status is required":
		return "ADMIN_UPDATE_FIELDS_REQUIRED"
	case message == "too many signup attempts":
		return "AUTH_SIGNUP_RATE_LIMITED"
	case message == "too many login attempts":
		return "AUTH_LOGIN_RATE_LIMITED"
	case message == "too many login method checks":
		return "AUTH_LOGIN_METHOD_RATE_LIMITED"
	case message == "too many refresh attempts", message == "too many logout attempts":
		return "AUTH_REFRESH_RATE_LIMITED"
	case message == "too many password change attempts":
		return "AUTH_PASSWORD_CHANGE_RATE_LIMITED"
	case message == "too many password reset attempts":
		return "AUTH_PASSWORD_RESET_RATE_LIMITED"
	case message == "password is required for password sign-up":
		return "AUTH_PASSWORD_REQUIRED"
	case message == "jwks not configured":
		return "AUTH_JWKS_NOT_CONFIGURED"
	case message == "unauthorized":
		return "AUTH_INVALID_TOKEN"
	case message == "forbidden":
		return "ADMIN_FORBIDDEN"
	case message == "method not allowed":
		return "SYSTEM_METHOD_NOT_ALLOWED"
	case message == "not found":
		return "SYSTEM_NOT_FOUND"
	}

	switch status {
	case http.StatusBadRequest:
		return "AUTH_INVALID_REQUEST"
	case http.StatusUnauthorized:
		return "AUTH_INVALID_TOKEN"
	case http.StatusForbidden:
		return "ADMIN_FORBIDDEN"
	case http.StatusNotFound:
		return "SYSTEM_NOT_FOUND"
	case http.StatusMethodNotAllowed:
		return "SYSTEM_METHOD_NOT_ALLOWED"
	case http.StatusTooManyRequests:
		return "SYSTEM_RATE_LIMITED"
	default:
		if status >= http.StatusInternalServerError {
			return "SYSTEM_INTERNAL_ERROR"
		}
		return "REQUEST_ERROR"
	}
}
