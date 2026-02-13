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
)

const (
	otpPurposeSignupPassword = "signup_password"
	otpPurposeSignupOTP      = "signup_otp"
	otpPurposeLoginOTP       = "login_otp"
)

var (
	errOTPSendRateLimited   = errors.New("too many verification code requests")
	errOTPVerifyRateLimited = errors.New("too many verification attempts")
	errOTPPurposeInvalid    = errors.New("invalid verification purpose")
	errOTPChallengeInvalid  = errors.New("verification request is invalid")
	errOTPCodeInvalid       = errors.New("incorrect verification code")
	errOTPCodeExpired       = errors.New("verification code expired")
	errOTPCodeRequired      = errors.New("verification code is required")
	errOTPChallengeRequired = errors.New("verification session is required")
)

type otpStore struct {
	client            *redis.Client
	keyPrefix         string
	challengeTTL      time.Duration
	challengePersist  time.Duration
	resendAfter       time.Duration
	maxVerifyAttempts int
}

type otpChallenge struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	Purpose    string    `json:"purpose"`
	CodeHash   string    `json:"codeHash"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Attempts   int       `json:"attempts"`
	MaxAttempt int       `json:"maxAttempt"`
}

func newOTPStore(addr, password string) (*otpStore, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, errors.New("otp redis addr is required")
	}
	challengeTTL := 5 * time.Minute
	return &otpStore{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
		}),
		keyPrefix:         "onebook:auth:otp",
		challengeTTL:      challengeTTL,
		challengePersist:  challengeTTL + time.Minute,
		resendAfter:       time.Minute,
		maxVerifyAttempts: 5,
	}, nil
}

func (s *otpStore) CreateChallenge(email, purpose string) (string, string, int, int, error) {
	if s == nil {
		return "", "", 0, 0, errors.New("otp store not configured")
	}
	email, err := normalizeEmail(email)
	if err != nil {
		return "", "", 0, 0, err
	}
	if !isValidOTPPurpose(purpose) {
		return "", "", 0, 0, errOTPPurposeInvalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resendKey := s.resendKey(email, purpose)
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
		Email:      email,
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

func (s *otpStore) VerifyChallenge(challengeID, email, purpose, code string) error {
	if s == nil {
		return errors.New("otp store not configured")
	}
	challengeID = strings.TrimSpace(challengeID)
	if challengeID == "" {
		return errOTPChallengeRequired
	}
	email, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	if !isValidOTPPurpose(purpose) {
		return errOTPPurposeInvalid
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
	if challenge.ID == "" || challenge.Email != email || challenge.Purpose != purpose {
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

func (s *otpStore) challengeKey(challengeID string) string {
	return fmt.Sprintf("%s:challenge:%s", s.keyPrefix, challengeID)
}

func (s *otpStore) resendKey(email, purpose string) string {
	return fmt.Sprintf("%s:resend:%s:%s", s.keyPrefix, purpose, email)
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

func isValidOTPPurpose(purpose string) bool {
	switch strings.TrimSpace(strings.ToLower(purpose)) {
	case otpPurposeSignupPassword, otpPurposeSignupOTP, otpPurposeLoginOTP:
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
