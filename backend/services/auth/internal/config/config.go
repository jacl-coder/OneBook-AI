package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// FileConfig represents configuration loaded from YAML.
type FileConfig struct {
	Port          string `yaml:"port"`
	DatabaseURL   string `yaml:"databaseURL"`
	RedisAddr     string `yaml:"redisAddr"`
	RedisPassword string `yaml:"redisPassword"`
	SessionTTL    string `yaml:"sessionTTL"`
	LogLevel      string `yaml:"logLevel"`
	JWTSecret     string `yaml:"jwtSecret"`
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
	// Sessions: prefer JWT; if JWTSecret not provided, require Redis.
	if cfg.JWTSecret == "" && cfg.RedisAddr == "" {
		return errors.New("config: jwtSecret or redisAddr is required (set in config.yaml)")
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
