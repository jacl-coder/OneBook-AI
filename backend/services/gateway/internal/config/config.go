package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FileConfig represents configuration loaded from YAML.
type FileConfig struct {
	Port                       string   `yaml:"port"`
	LogLevel                   string   `yaml:"logLevel"`
	AuthServiceURL             string   `yaml:"authServiceURL"`
	AuthJWKSURL                string   `yaml:"authJwksURL"`
	RefreshCookieName          string   `yaml:"refreshCookieName"`
	RefreshCookieDomain        string   `yaml:"refreshCookieDomain"`
	RefreshCookiePath          string   `yaml:"refreshCookiePath"`
	RefreshCookieSecure        bool     `yaml:"refreshCookieSecure"`
	RefreshCookieSameSite      string   `yaml:"refreshCookieSameSite"`
	RefreshCookieMaxAgeSeconds int      `yaml:"refreshCookieMaxAgeSeconds"`
	JWTIssuer                  string   `yaml:"jwtIssuer"`
	JWTAudience                string   `yaml:"jwtAudience"`
	JWTLeeway                  string   `yaml:"jwtLeeway"`
	RedisAddr                  string   `yaml:"redisAddr"`
	RedisPassword              string   `yaml:"redisPassword"`
	TrustedProxyCIDRs          []string `yaml:"trustedProxyCidrs"`
	SignupRateLimitPerMinute   int      `yaml:"signupRateLimitPerMinute"`
	LoginRateLimitPerMinute    int      `yaml:"loginRateLimitPerMinute"`
	RefreshRateLimitPerMinute  int      `yaml:"refreshRateLimitPerMinute"`
	PasswordRateLimitPerMinute int      `yaml:"passwordRateLimitPerMinute"`
	BookServiceURL             string   `yaml:"bookServiceURL"`
	ChatServiceURL             string   `yaml:"chatServiceURL"`
	MaxUploadBytes             int64    `yaml:"maxUploadBytes"`
	AllowedExtensions          []string `yaml:"allowedExtensions"`
}

// Load reads config from path (defaults to config.yaml).
func Load(path string) (FileConfig, error) {
	cfg := FileConfig{}
	if path == "" {
		path = ConfigPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if v := os.Getenv("GATEWAY_MAX_UPLOAD_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxUploadBytes = n
		}
	}
	if v := os.Getenv("GATEWAY_AUTH_JWKS_URL"); v != "" {
		cfg.AuthJWKSURL = v
	}
	if v := os.Getenv("GATEWAY_REFRESH_COOKIE_NAME"); v != "" {
		cfg.RefreshCookieName = strings.TrimSpace(v)
	}
	if v := os.Getenv("GATEWAY_REFRESH_COOKIE_DOMAIN"); v != "" {
		cfg.RefreshCookieDomain = strings.TrimSpace(v)
	}
	if v := os.Getenv("GATEWAY_REFRESH_COOKIE_PATH"); v != "" {
		cfg.RefreshCookiePath = strings.TrimSpace(v)
	}
	if v := os.Getenv("GATEWAY_REFRESH_COOKIE_SECURE"); v != "" {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			cfg.RefreshCookieSecure = b
		}
	}
	if v := os.Getenv("GATEWAY_REFRESH_COOKIE_SAME_SITE"); v != "" {
		cfg.RefreshCookieSameSite = strings.TrimSpace(v)
	}
	if v := os.Getenv("GATEWAY_REFRESH_COOKIE_MAX_AGE_SECONDS"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.RefreshCookieMaxAgeSeconds = n
		}
	}
	if v := os.Getenv("JWT_ISSUER"); v != "" {
		cfg.JWTIssuer = v
	}
	if v := os.Getenv("JWT_AUDIENCE"); v != "" {
		cfg.JWTAudience = v
	}
	if v := os.Getenv("JWT_LEEWAY"); v != "" {
		cfg.JWTLeeway = v
	}
	if v := os.Getenv("GATEWAY_ALLOWED_EXTENSIONS"); v != "" {
		cfg.AllowedExtensions = splitCSV(v)
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.RedisAddr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.RedisPassword = v
	}
	if v := os.Getenv("GATEWAY_TRUSTED_PROXY_CIDRS"); v != "" {
		cfg.TrustedProxyCIDRs = splitCSV(v)
	}
	if v := os.Getenv("GATEWAY_SIGNUP_RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.SignupRateLimitPerMinute = n
		}
	}
	if v := os.Getenv("GATEWAY_LOGIN_RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.LoginRateLimitPerMinute = n
		}
	}
	if v := os.Getenv("GATEWAY_REFRESH_RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RefreshRateLimitPerMinute = n
		}
	}
	if v := os.Getenv("GATEWAY_PASSWORD_RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.PasswordRateLimitPerMinute = n
		}
	}
	if err := validateConfig(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validateConfig(cfg FileConfig) error {
	if cfg.Port == "" {
		return errors.New("config: port is required (set in config.yaml)")
	}
	if cfg.AuthServiceURL == "" {
		return errors.New("config: authServiceURL is required (set in config.yaml)")
	}
	if strings.TrimSpace(cfg.AuthJWKSURL) == "" {
		return errors.New("config: authJwksURL is required (set in config.yaml or GATEWAY_AUTH_JWKS_URL)")
	}
	if cfg.BookServiceURL == "" {
		return errors.New("config: bookServiceURL is required (set in config.yaml)")
	}
	if cfg.ChatServiceURL == "" {
		return errors.New("config: chatServiceURL is required (set in config.yaml)")
	}
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return errors.New("config: redisAddr is required for distributed rate limiting")
	}
	if cfg.SignupRateLimitPerMinute < 0 || cfg.LoginRateLimitPerMinute < 0 || cfg.RefreshRateLimitPerMinute < 0 || cfg.PasswordRateLimitPerMinute < 0 {
		return errors.New("config: rate limits must be >= 0")
	}
	return nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

// ParseJWTLeeway parses optional JWT leeway duration string.
func ParseJWTLeeway(leewayStr string) (time.Duration, error) {
	if leewayStr == "" {
		return 0, nil
	}
	dur, err := time.ParseDuration(leewayStr)
	if err != nil {
		return 0, fmt.Errorf("invalid jwtLeeway duration: %w", err)
	}
	return dur, nil
}
