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
	Port                        string  `yaml:"port"`
	LogLevel                    string  `yaml:"logLevel"`
	LogsDir                     string  `yaml:"logsDir"`
	DatabaseURL                 string  `yaml:"databaseURL"`
	BookServiceURL              string  `yaml:"bookServiceURL"`
	IndexerURL                  string  `yaml:"indexerURL"`
	InternalJWTPrivateKeyPath   string  `yaml:"internalJwtPrivateKeyPath"`
	InternalJWTPublicKeyPath    string  `yaml:"internalJwtPublicKeyPath"`
	InternalJWTVerifyPublicKeys string  `yaml:"internalJwtVerifyPublicKeys"`
	InternalJWTKeyID            string  `yaml:"internalJwtKeyId"`
	RedisAddr                   string  `yaml:"redisAddr"`
	RedisPassword               string  `yaml:"redisPassword"`
	QueueName                   string  `yaml:"queueName"`
	QueueGroup                  string  `yaml:"queueGroup"`
	QueueConcurrency            int     `yaml:"queueConcurrency"`
	QueueMaxRetries             int     `yaml:"queueMaxRetries"`
	QueueRetryDelaySeconds      int     `yaml:"queueRetryDelaySeconds"`
	ChunkSize                   int     `yaml:"chunkSize"`
	ChunkOverlap                int     `yaml:"chunkOverlap"`
	OCREnabled                  bool    `yaml:"ocrEnabled"`
	OCRCommand                  string  `yaml:"ocrCommand"`
	OCRDevice                   string  `yaml:"ocrDevice"`
	OCRTimeoutSeconds           int     `yaml:"ocrTimeoutSeconds"`
	PDFMinPageRunes             int     `yaml:"pdfMinPageRunes"`
	PDFMinPageScore             float64 `yaml:"pdfMinPageScore"`
	PDFOCRMinScoreDelta         float64 `yaml:"pdfOcrMinScoreDelta"`
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
	if v := os.Getenv("INGEST_CHUNK_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ChunkSize = n
		}
	}
	if v := os.Getenv("INGEST_CHUNK_OVERLAP"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ChunkOverlap = n
		}
	}
	if v := os.Getenv("INGEST_OCR_ENABLED"); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			cfg.OCREnabled = enabled
		}
	}
	if v := os.Getenv("INGEST_OCR_COMMAND"); v != "" {
		cfg.OCRCommand = v
	}
	if v := os.Getenv("INGEST_OCR_DEVICE"); v != "" {
		cfg.OCRDevice = v
	}
	if v := os.Getenv("INGEST_OCR_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.OCRTimeoutSeconds = n
		}
	}
	if v := os.Getenv("INGEST_PDF_MIN_PAGE_RUNES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.PDFMinPageRunes = n
		}
	}
	if v := os.Getenv("INGEST_PDF_MIN_PAGE_SCORE"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.PDFMinPageScore = n
		}
	}
	if v := os.Getenv("INGEST_PDF_OCR_MIN_SCORE_DELTA"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.PDFOCRMinScoreDelta = n
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
	if strings.TrimSpace(cfg.InternalJWTPrivateKeyPath) == "" || strings.TrimSpace(cfg.InternalJWTPublicKeyPath) == "" {
		return errors.New("config: internal service auth requires ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH + ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH")
	}
	if cfg.RedisAddr == "" {
		return errors.New("config: redisAddr is required (set in config.yaml or REDIS_ADDR)")
	}
	if cfg.ChunkSize <= 0 {
		return errors.New("config: chunkSize must be > 0 (set in config.yaml or INGEST_CHUNK_SIZE)")
	}
	if cfg.ChunkOverlap < 0 {
		return errors.New("config: chunkOverlap must be >= 0 (set in config.yaml or INGEST_CHUNK_OVERLAP)")
	}
	if cfg.ChunkOverlap >= cfg.ChunkSize {
		return errors.New("config: chunkOverlap must be smaller than chunkSize")
	}
	if cfg.OCREnabled && strings.TrimSpace(cfg.OCRCommand) == "" {
		return errors.New("config: ocrCommand is required when ocrEnabled=true")
	}
	if cfg.OCRTimeoutSeconds < 0 {
		return errors.New("config: ocrTimeoutSeconds must be >= 0")
	}
	if cfg.PDFMinPageRunes < 0 {
		return errors.New("config: pdfMinPageRunes must be >= 0")
	}
	if cfg.PDFMinPageScore < 0 || cfg.PDFMinPageScore > 1 {
		return errors.New("config: pdfMinPageScore must be between 0 and 1")
	}
	if cfg.PDFOCRMinScoreDelta < 0 || cfg.PDFOCRMinScoreDelta > 1 {
		return errors.New("config: pdfOcrMinScoreDelta must be between 0 and 1")
	}
	return nil
}
