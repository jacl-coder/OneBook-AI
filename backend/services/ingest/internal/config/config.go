package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// FileConfig represents configuration loaded from YAML.
type FileConfig struct {
	Port                   string `yaml:"port"`
	LogLevel               string `yaml:"logLevel"`
	DatabaseURL            string `yaml:"databaseURL"`
	BookServiceURL         string `yaml:"bookServiceURL"`
	IndexerURL             string `yaml:"indexerURL"`
	InternalToken          string `yaml:"internalToken"`
	RedisAddr              string `yaml:"redisAddr"`
	RedisPassword          string `yaml:"redisPassword"`
	QueueName              string `yaml:"queueName"`
	QueueGroup             string `yaml:"queueGroup"`
	QueueConcurrency       int    `yaml:"queueConcurrency"`
	QueueMaxRetries        int    `yaml:"queueMaxRetries"`
	QueueRetryDelaySeconds int    `yaml:"queueRetryDelaySeconds"`
	ChunkSize              int    `yaml:"chunkSize"`
	ChunkOverlap           int    `yaml:"chunkOverlap"`
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
	if v := os.Getenv("ONEBOOK_INTERNAL_TOKEN"); v != "" {
		cfg.InternalToken = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.RedisAddr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.RedisPassword = v
	}
	if v := os.Getenv("INGEST_QUEUE_NAME"); v != "" {
		cfg.QueueName = v
	}
	if v := os.Getenv("INGEST_QUEUE_GROUP"); v != "" {
		cfg.QueueGroup = v
	}
	if v := os.Getenv("INGEST_QUEUE_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.QueueConcurrency = n
		}
	}
	if v := os.Getenv("INGEST_QUEUE_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.QueueMaxRetries = n
		}
	}
	if v := os.Getenv("INGEST_QUEUE_RETRY_DELAY_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.QueueRetryDelaySeconds = n
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
	if cfg.BookServiceURL == "" {
		return errors.New("config: bookServiceURL is required (set in config.yaml)")
	}
	if cfg.IndexerURL == "" {
		return errors.New("config: indexerURL is required (set in config.yaml)")
	}
	if cfg.InternalToken == "" {
		return errors.New("config: internalToken is required (set in config.yaml or ONEBOOK_INTERNAL_TOKEN)")
	}
	if cfg.RedisAddr == "" {
		return errors.New("config: redisAddr is required (set in config.yaml or REDIS_ADDR)")
	}
	return nil
}
