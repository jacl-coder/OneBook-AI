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
	Port                       string `yaml:"port"`
	DatabaseURL                string `yaml:"databaseURL"`
	RedisAddr                  string `yaml:"redisAddr"`
	RedisPassword              string `yaml:"redisPassword"`
	SessionTTL                 string `yaml:"sessionTTL"`
	RefreshTTL                 string `yaml:"refreshTTL"`
	LogLevel                   string `yaml:"logLevel"`
	JWTPrivateKeyPath          string `yaml:"jwtPrivateKeyPath"`
	JWTPublicKeyPath           string `yaml:"jwtPublicKeyPath"`
	JWTKeyID                   string `yaml:"jwtKeyId"`
	JWTVerifyPublicKeys        string `yaml:"jwtVerifyPublicKeys"`
	JWTIssuer                  string `yaml:"jwtIssuer"`
	JWTAudience                string `yaml:"jwtAudience"`
	JWTLeeway                  string `yaml:"jwtLeeway"`
	SignupRateLimitPerMinute   int    `yaml:"signupRateLimitPerMinute"`
	LoginRateLimitPerMinute    int    `yaml:"loginRateLimitPerMinute"`
	RefreshRateLimitPerMinute  int    `yaml:"refreshRateLimitPerMinute"`
	PasswordRateLimitPerMinute int    `yaml:"passwordRateLimitPerMinute"`
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
	// Override with environment variables
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.RedisAddr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.RedisPassword = v
	}
	if v := os.Getenv("JWT_PRIVATE_KEY_PATH"); v != "" {
		cfg.JWTPrivateKeyPath = v
	}
	if v := os.Getenv("JWT_PUBLIC_KEY_PATH"); v != "" {
		cfg.JWTPublicKeyPath = v
	}
	if v := os.Getenv("JWT_KEY_ID"); v != "" {
		cfg.JWTKeyID = v
	}
	if v := os.Getenv("JWT_VERIFY_PUBLIC_KEYS"); v != "" {
		cfg.JWTVerifyPublicKeys = v
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
	if v := os.Getenv("AUTH_REFRESH_TTL"); v != "" {
		cfg.RefreshTTL = v
	}
	if v := os.Getenv("AUTH_SIGNUP_RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.SignupRateLimitPerMinute = n
		}
	}
	if v := os.Getenv("AUTH_LOGIN_RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.LoginRateLimitPerMinute = n
		}
	}
	if v := os.Getenv("AUTH_REFRESH_RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RefreshRateLimitPerMinute = n
		}
	}
	if v := os.Getenv("AUTH_PASSWORD_RATE_LIMIT_PER_MINUTE"); v != "" {
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
	if cfg.DatabaseURL == "" {
		return errors.New("config: databaseURL is required (set in config.yaml)")
	}
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return errors.New("config: redisAddr is required for jwt+redis session strategy")
	}
	// Sessions: production default requires RS256 signing key.
	if cfg.JWTPrivateKeyPath == "" {
		return errors.New("config: jwtPrivateKeyPath is required (set JWT_PRIVATE_KEY_PATH)")
	}
	if cfg.JWTPrivateKeyPath == "" && cfg.JWTPublicKeyPath != "" {
		return errors.New("config: jwtPublicKeyPath requires jwtPrivateKeyPath")
	}
	if cfg.SignupRateLimitPerMinute < 0 || cfg.LoginRateLimitPerMinute < 0 || cfg.RefreshRateLimitPerMinute < 0 || cfg.PasswordRateLimitPerMinute < 0 {
		return errors.New("config: rate limits must be >= 0")
	}
	return nil
}

// ParseSessionTTL parses optional session TTL duration string.
func ParseSessionTTL(ttlStr string) (time.Duration, error) {
	if ttlStr == "" {
		return 0, nil
	}
	dur, err := time.ParseDuration(ttlStr)
	if err != nil {
		return 0, fmt.Errorf("invalid sessionTTL duration: %w", err)
	}
	return dur, nil
}

// ParseRefreshTTL parses optional refresh TTL duration string.
func ParseRefreshTTL(ttlStr string) (time.Duration, error) {
	if ttlStr == "" {
		return 0, nil
	}
	dur, err := time.ParseDuration(ttlStr)
	if err != nil {
		return 0, fmt.Errorf("invalid refreshTTL duration: %w", err)
	}
	return dur, nil
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

// ParseVerifyPublicKeys parses "kid=path,kid2=path2" into a map.
func ParseVerifyPublicKeys(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	pairs := strings.Split(raw, ",")
	out := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid jwtVerifyPublicKeys entry %q", pair)
		}
		kid := strings.TrimSpace(parts[0])
		path := strings.TrimSpace(parts[1])
		if kid == "" || path == "" {
			return nil, fmt.Errorf("invalid jwtVerifyPublicKeys entry %q", pair)
		}
		out[kid] = path
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
