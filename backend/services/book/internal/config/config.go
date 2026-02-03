package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileConfig represents configuration loaded from YAML.
type FileConfig struct {
	Port              string   `yaml:"port"`
	DatabaseURL       string   `yaml:"databaseURL"`
	LogLevel          string   `yaml:"logLevel"`
	MinioEndpoint     string   `yaml:"minioEndpoint"`
	MinioAccessKey    string   `yaml:"minioAccessKey"`
	MinioSecretKey    string   `yaml:"minioSecretKey"`
	MinioBucket       string   `yaml:"minioBucket"`
	MinioUseSSL       bool     `yaml:"minioUseSSL"`
	IngestURL         string   `yaml:"ingestURL"`
	InternalToken     string   `yaml:"internalToken"`
	MaxUploadBytes    int64    `yaml:"maxUploadBytes"`
	AllowedExtensions []string `yaml:"allowedExtensions"`
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
	if v := os.Getenv("ONEBOOK_INTERNAL_TOKEN"); v != "" {
		cfg.InternalToken = v
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
	if cfg.InternalToken == "" {
		return errors.New("config: internalToken is required (set in config.yaml or ONEBOOK_INTERNAL_TOKEN)")
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
