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
	Port                        string   `yaml:"port"`
	DatabaseURL                 string   `yaml:"databaseURL"`
	LogLevel                    string   `yaml:"logLevel"`
	LogsDir                     string   `yaml:"logsDir"`
	AuthServiceURL              string   `yaml:"authServiceURL"`
	AuthJWKSURL                 string   `yaml:"authJwksURL"`
	JWTIssuer                   string   `yaml:"jwtIssuer"`
	JWTAudience                 string   `yaml:"jwtAudience"`
	JWTLeeway                   string   `yaml:"jwtLeeway"`
	MinioEndpoint               string   `yaml:"minioEndpoint"`
	MinioAccessKey              string   `yaml:"minioAccessKey"`
	MinioSecretKey              string   `yaml:"minioSecretKey"`
	MinioBucket                 string   `yaml:"minioBucket"`
	MinioUseSSL                 bool     `yaml:"minioUseSSL"`
	IngestURL                   string   `yaml:"ingestURL"`
	InternalJWTPrivateKeyPath   string   `yaml:"internalJwtPrivateKeyPath"`
	InternalJWTPublicKeyPath    string   `yaml:"internalJwtPublicKeyPath"`
	InternalJWTVerifyPublicKeys string   `yaml:"internalJwtVerifyPublicKeys"`
	InternalJWTKeyID            string   `yaml:"internalJwtKeyId"`
	MaxUploadBytes              int64    `yaml:"maxUploadBytes"`
	AllowedExtensions           []string `yaml:"allowedExtensions"`
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
	if v := os.Getenv("LOGS_DIR"); v != "" {
		cfg.LogsDir = v
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("BOOK_AUTH_SERVICE_URL"); v != "" {
		cfg.AuthServiceURL = v
	}
	if v := os.Getenv("BOOK_AUTH_JWKS_URL"); v != "" {
		cfg.AuthJWKSURL = v
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
	if v := os.Getenv("MINIO_ENDPOINT"); v != "" {
		cfg.MinioEndpoint = v
	}
	if v := os.Getenv("MINIO_ACCESS_KEY"); v != "" {
		cfg.MinioAccessKey = v
	}
	if v := os.Getenv("MINIO_SECRET_KEY"); v != "" {
		cfg.MinioSecretKey = v
	}
	if v := os.Getenv("MINIO_BUCKET"); v != "" {
		cfg.MinioBucket = v
	}
	if v := os.Getenv("MINIO_USE_SSL"); v == "true" {
		cfg.MinioUseSSL = true
	}
	if v := os.Getenv("ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH"); v != "" {
		cfg.InternalJWTPrivateKeyPath = v
	}
	if v := os.Getenv("ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH"); v != "" {
		cfg.InternalJWTPublicKeyPath = v
	}
	if v := os.Getenv("ONEBOOK_INTERNAL_JWT_VERIFY_PUBLIC_KEYS"); v != "" {
		cfg.InternalJWTVerifyPublicKeys = v
	}
	if v := os.Getenv("ONEBOOK_INTERNAL_JWT_KEY_ID"); v != "" {
		cfg.InternalJWTKeyID = v
	}
	if v := os.Getenv("BOOK_MAX_UPLOAD_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxUploadBytes = n
		}
	}
	if v := os.Getenv("BOOK_ALLOWED_EXTENSIONS"); v != "" {
		cfg.AllowedExtensions = splitCSV(v)
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
	if cfg.AuthServiceURL == "" {
		return errors.New("config: authServiceURL is required (set in config.yaml or BOOK_AUTH_SERVICE_URL)")
	}
	if strings.TrimSpace(cfg.AuthJWKSURL) == "" {
		return errors.New("config: authJwksURL is required (set in config.yaml or BOOK_AUTH_JWKS_URL)")
	}
	if cfg.MinioEndpoint == "" {
		return errors.New("config: minioEndpoint is required (set in config.yaml)")
	}
	if cfg.MinioAccessKey == "" {
		return errors.New("config: minioAccessKey is required (set in config.yaml)")
	}
	if cfg.MinioSecretKey == "" {
		return errors.New("config: minioSecretKey is required (set in config.yaml)")
	}
	if cfg.MinioBucket == "" {
		return errors.New("config: minioBucket is required (set in config.yaml)")
	}
	if cfg.IngestURL == "" {
		return errors.New("config: ingestURL is required (set in config.yaml)")
	}
	if strings.TrimSpace(cfg.InternalJWTPrivateKeyPath) == "" || strings.TrimSpace(cfg.InternalJWTPublicKeyPath) == "" {
		return errors.New("config: internal service auth requires ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH + ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH")
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
