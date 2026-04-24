package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"onebookai/internal/util"
	"onebookai/pkg/domain"
)

const (
	verificationPurposeSignup        = "signup"
	verificationPurposeLogin         = "login"
	verificationPurposePasswordReset = "password_reset"
)

var (
	errOTPSendRateLimited             = errors.New("too many verification code requests")
	errOTPVerifyRateLimited           = errors.New("too many verification attempts")
	errPasswordResetVerifyRateLimited = errors.New("too many password reset verification attempts")
	errOTPPurposeInvalid              = errors.New("invalid verification purpose")
	errOTPChallengeInvalid            = errors.New("verification request is invalid")
	errOTPCodeInvalid                 = errors.New("incorrect verification code")
	errOTPCodeExpired                 = errors.New("verification code expired")
	errOTPCodeRequired                = errors.New("verification code is required")
	errOTPChallengeRequired           = errors.New("verification session is required")
	errVerificationTokenInvalid       = errors.New("verification session is invalid")
	errVerificationTokenRequired      = errors.New("verification session is required")
)

type otpStore struct {
	client            *redis.Client
	keyPrefix         string
	challengeTTL      time.Duration
	challengePersist  time.Duration
	resendAfter       time.Duration
	verificationTTL   time.Duration
	maxVerifyAttempts int
}

type otpChallenge struct {
	ID         string    `json:"id"`
	Channel    string    `json:"channel"`
	Identifier string    `json:"identifier"`
	Purpose    string    `json:"purpose"`
	CodeHash   string    `json:"codeHash"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Attempts   int       `json:"attempts"`
	MaxAttempt int       `json:"maxAttempt"`
}

type verificationToken struct {
	Channel    string    `json:"channel"`
	Identifier string    `json:"identifier"`
	Purpose    string    `json:"purpose"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

func newOTPStore(addr, password string) (*otpStore, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, errors.New("otp redis addr is required")
	}
	challengeTTL := 10 * time.Minute
	return &otpStore{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
		}),
		keyPrefix:         "onebook:auth:otp",
		challengeTTL:      challengeTTL,
		challengePersist:  challengeTTL + time.Minute,
		resendAfter:       time.Minute,
		verificationTTL:   10 * time.Minute,
		maxVerifyAttempts: 5,
	}, nil
}

func (s *otpStore) CreateChallenge(channel domain.IdentityType, identifier, purpose string) (string, string, int, int, error) {
	if s == nil {
		return "", "", 0, 0, errors.New("otp store not configured")
	}
	purpose = normalizeVerificationPurpose(purpose)
	if !isValidVerificationPurpose(purpose) {
		return "", "", 0, 0, errOTPPurposeInvalid
	}
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", "", 0, 0, errors.New("identifier is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resendKey := s.resendKey(channel, identifier, purpose)
	allowed, err := s.client.SetNX(ctx, resendKey, "1", s.resendAfter).Result()
	if err != nil {
		return "", "", 0, 0, err
	}
	if !allowed {
		return "", "", 0, 0, errOTPSendRateLimited
	}

	code, err := generateNumericCode(6)
	if err != nil {
		_ = s.client.Del(ctx, resendKey).Err()
		return "", "", 0, 0, fmt.Errorf("generate otp code: %w", err)
	}
	codeHash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		_ = s.client.Del(ctx, resendKey).Err()
		return "", "", 0, 0, fmt.Errorf("hash otp code: %w", err)
	}
	challengeID := util.NewID()
	challenge := otpChallenge{
		ID:         challengeID,
		Channel:    string(channel),
		Identifier: identifier,
		Purpose:    purpose,
		CodeHash:   string(codeHash),
		ExpiresAt:  time.Now().UTC().Add(s.challengeTTL),
		Attempts:   0,
		MaxAttempt: s.maxVerifyAttempts,
	}
	raw, err := json.Marshal(challenge)
	if err != nil {
		_ = s.client.Del(ctx, resendKey).Err()
		return "", "", 0, 0, fmt.Errorf("marshal otp challenge: %w", err)
	}
	if err := s.client.Set(ctx, s.challengeKey(challengeID), raw, s.challengePersist).Err(); err != nil {
		_ = s.client.Del(ctx, resendKey).Err()
		return "", "", 0, 0, err
	}
	return challengeID, code, int(s.challengeTTL.Seconds()), int(s.resendAfter.Seconds()), nil
}

func (s *otpStore) VerifyChallenge(challengeID string, channel domain.IdentityType, identifier, purpose, code string) error {
	if s == nil {
		return errors.New("otp store not configured")
	}
	challengeID = strings.TrimSpace(challengeID)
	if challengeID == "" {
		return errOTPChallengeRequired
	}
	purpose = normalizeVerificationPurpose(purpose)
	if !isValidVerificationPurpose(purpose) {
		return errOTPPurposeInvalid
	}
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return errors.New("identifier is required")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return errOTPCodeRequired
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	key := s.challengeKey(challengeID)
	raw, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return errOTPChallengeInvalid
	}
	if err != nil {
		return err
	}
	var challenge otpChallenge
	if err := json.Unmarshal(raw, &challenge); err != nil {
		return fmt.Errorf("unmarshal otp challenge: %w", err)
	}
	if challenge.ID == "" || challenge.Channel != string(channel) || challenge.Identifier != identifier || challenge.Purpose != purpose {
		return errOTPChallengeInvalid
	}
	if time.Now().UTC().After(challenge.ExpiresAt) {
		_ = s.client.Del(ctx, key).Err()
		return errOTPCodeExpired
	}
	if challenge.Attempts >= challenge.MaxAttempt {
		_ = s.client.Del(ctx, key).Err()
		return errOTPChallengeInvalid
	}
	if bcrypt.CompareHashAndPassword([]byte(challenge.CodeHash), []byte(code)) != nil {
		challenge.Attempts++
		if challenge.Attempts >= challenge.MaxAttempt {
			_ = s.client.Del(ctx, key).Err()
		} else {
			raw, marshalErr := json.Marshal(challenge)
			if marshalErr == nil {
				ttl, ttlErr := s.client.TTL(ctx, key).Result()
				if ttlErr == nil && ttl > 0 {
					_ = s.client.Set(ctx, key, raw, ttl).Err()
				}
			}
		}
		return errOTPCodeInvalid
	}
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return err
	}
	return nil
}

func (s *otpStore) DeleteChallenge(challengeID string, channel domain.IdentityType, identifier, purpose string) error {
	if s == nil {
		return errors.New("otp store not configured")
	}
	challengeID = strings.TrimSpace(challengeID)
	identifier = strings.TrimSpace(identifier)
	purpose = normalizeVerificationPurpose(purpose)
	if challengeID == "" || identifier == "" || !isValidVerificationPurpose(purpose) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Del(ctx, s.challengeKey(challengeID), s.resendKey(channel, identifier, purpose)).Err()
}

func (s *otpStore) challengeKey(challengeID string) string {
	return fmt.Sprintf("%s:challenge:%s", s.keyPrefix, challengeID)
}

func (s *otpStore) resendKey(channel domain.IdentityType, identifier, purpose string) string {
	return fmt.Sprintf("%s:resend:%s:%s:%s", s.keyPrefix, purpose, channel, identifier)
}

func (s *otpStore) verificationTokenKey(token string) string {
	return fmt.Sprintf("%s:verification-token:%s", s.keyPrefix, token)
}

func (s *otpStore) CreateVerificationToken(channel domain.IdentityType, identifier, purpose string) (string, int, error) {
	if s == nil {
		return "", 0, errors.New("otp store not configured")
	}
	purpose = normalizeVerificationPurpose(purpose)
	token := util.NewID()
	payload := verificationToken{
		Channel:    string(channel),
		Identifier: strings.TrimSpace(identifier),
		Purpose:    purpose,
		ExpiresAt:  time.Now().UTC().Add(s.verificationTTL),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.client.Set(ctx, s.verificationTokenKey(token), raw, s.verificationTTL).Err(); err != nil {
		return "", 0, err
	}
	return token, int(s.verificationTTL.Seconds()), nil
}

func (s *otpStore) ValidateVerificationToken(token string, channel domain.IdentityType, identifier, purpose string) error {
	if s == nil {
		return errors.New("otp store not configured")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errVerificationTokenRequired
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	val, err := s.client.Get(ctx, s.verificationTokenKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return errVerificationTokenInvalid
	}
	if err != nil {
		return err
	}
	var payload verificationToken
	if err := json.Unmarshal(val, &payload); err != nil {
		return fmt.Errorf("unmarshal verification token: %w", err)
	}
	if payload.Channel != string(channel) ||
		payload.Identifier != strings.TrimSpace(identifier) ||
		payload.Purpose != normalizeVerificationPurpose(purpose) ||
		time.Now().UTC().After(payload.ExpiresAt) {
		return errVerificationTokenInvalid
	}
	return nil
}

func (s *otpStore) ConsumeVerificationToken(token string) error {
	if s == nil {
		return errors.New("otp store not configured")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errVerificationTokenRequired
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	deleted, err := s.client.Del(ctx, s.verificationTokenKey(token)).Result()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return errVerificationTokenInvalid
	}
	return nil
}

func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", errors.New("email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "", errors.New("email format is invalid")
	}
	return email, nil
}

func normalizeVerificationIdentifier(channel, identifier string) (domain.IdentityType, string, error) {
	normalizedChannel := strings.TrimSpace(strings.ToLower(channel))
	if normalizedChannel == "" {
		if strings.Contains(identifier, "@") {
			normalizedChannel = string(domain.IdentityEmail)
		} else {
			normalizedChannel = string(domain.IdentityPhone)
		}
	}
	switch normalizedChannel {
	case string(domain.IdentityEmail):
		email, err := normalizeEmail(identifier)
		return domain.IdentityEmail, email, err
	case string(domain.IdentityPhone), "sms":
		phone, err := normalizeCNPhone(identifier)
		return domain.IdentityPhone, phone, err
	default:
		return "", "", errors.New("verification channel is invalid")
	}
}

func normalizeCNPhone(phone string) (string, error) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	if strings.HasPrefix(digits, "0086") {
		digits = strings.TrimPrefix(digits, "0086")
	}
	if strings.HasPrefix(digits, "86") && len(digits) == 13 {
		digits = strings.TrimPrefix(digits, "86")
	}
	if len(digits) != 11 || !strings.HasPrefix(digits, "1") {
		return "", errors.New("phone format is invalid")
	}
	return "+86" + digits, nil
}

func normalizeVerificationPurpose(purpose string) string {
	switch strings.TrimSpace(strings.ToLower(purpose)) {
	case "signup", "signup_password", "signup_otp":
		return verificationPurposeSignup
	case "login", "login_otp":
		return verificationPurposeLogin
	case "password_reset", "reset_password":
		return verificationPurposePasswordReset
	default:
		return strings.TrimSpace(strings.ToLower(purpose))
	}
}

func isValidVerificationPurpose(purpose string) bool {
	switch normalizeVerificationPurpose(purpose) {
	case verificationPurposeSignup, verificationPurposeLogin, verificationPurposePasswordReset:
		return true
	default:
		return false
	}
}

func generateNumericCode(length int) (string, error) {
	if length <= 0 {
		length = 6
	}
	var b strings.Builder
	b.Grow(length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + n.Int64()))
	}
	return b.String(), nil
}

func maskEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}
	local := parts[0]
	domain := parts[1]
	switch len(local) {
	case 0:
		return "***@" + domain
	case 1:
		return local + "***@" + domain
	case 2:
		return local[:1] + "***@" + domain
	default:
		return local[:1] + "***" + local[len(local)-1:] + "@" + domain
	}
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) <= 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

func maskIdentifier(channel domain.IdentityType, identifier string) string {
	if channel == domain.IdentityPhone {
		return maskPhone(identifier)
	}
	return maskEmail(identifier)
}
