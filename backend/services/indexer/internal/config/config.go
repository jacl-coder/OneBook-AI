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
	Port                        string `yaml:"port"`
	LogLevel                    string `yaml:"logLevel"`
	LogsDir                     string `yaml:"logsDir"`
	DatabaseURL                 string `yaml:"databaseURL"`
	BookServiceURL              string `yaml:"bookServiceURL"`
	InternalJWTPrivateKeyPath   string `yaml:"internalJwtPrivateKeyPath"`
	InternalJWTPublicKeyPath    string `yaml:"internalJwtPublicKeyPath"`
	InternalJWTVerifyPublicKeys string `yaml:"internalJwtVerifyPublicKeys"`
	InternalJWTKeyID            string `yaml:"internalJwtKeyId"`
	RabbitMQURL                 string `yaml:"rabbitmqURL"`
	QueueExchange               string `yaml:"queueExchange"`
	QueueName                   string `yaml:"queueName"`
	QueueConsumer               string `yaml:"queueConsumer"`
	QueueConcurrency            int    `yaml:"queueConcurrency"`
	QueueMaxRetries             int    `yaml:"queueMaxRetries"`
	QueueRetryDelaySeconds      int    `yaml:"queueRetryDelaySeconds"`
	EmbeddingProvider           string `yaml:"embeddingProvider"`
	EmbeddingBaseURL            string `yaml:"embeddingBaseURL"`
	EmbeddingModel              string `yaml:"embeddingModel"`
	EmbeddingDim                int    `yaml:"embeddingDim"`
	EmbeddingBatchSize          int    `yaml:"embeddingBatchSize"`
	EmbeddingConcurrency        int    `yaml:"embeddingConcurrency"`
	QdrantURL                   string `yaml:"qdrantURL"`
	QdrantAPIKey                string `yaml:"qdrantAPIKey"`
	QdrantCollection            string `yaml:"qdrantCollection"`
	OpenSearchURL               string `yaml:"openSearchURL"`
	OpenSearchIndex             string `yaml:"openSearchIndex"`
	OpenSearchUsername          string `yaml:"openSearchUsername"`
	OpenSearchPassword          string `yaml:"openSearchPassword"`
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
	if v := os.Getenv("RABBITMQ_URL"); v != "" {
		cfg.RabbitMQURL = v
	}
	if v := os.Getenv("INDEXER_QUEUE_EXCHANGE"); v != "" {
		cfg.QueueExchange = v
	}
	if v := os.Getenv("INDEXER_QUEUE_NAME"); v != "" {
		cfg.QueueName = v
	}
	if v := os.Getenv("INDEXER_QUEUE_CONSUMER"); v != "" {
		cfg.QueueConsumer = v
	}
	if v := os.Getenv("INDEXER_QUEUE_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.QueueConcurrency = n
		}
	}
	if v := os.Getenv("INDEXER_QUEUE_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.QueueMaxRetries = n
		}
	}
	if v := os.Getenv("INDEXER_QUEUE_RETRY_DELAY_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.QueueRetryDelaySeconds = n
		}
	}
	if v := os.Getenv("ONEBOOK_EMBEDDING_DIM"); v != "" {
		if dim, err := strconv.Atoi(v); err == nil {
			cfg.EmbeddingDim = dim
		}
	}
	if v := os.Getenv("EMBEDDING_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.EmbeddingBatchSize = n
		}
	}
	if v := os.Getenv("EMBEDDING_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.EmbeddingConcurrency = n
		}
	}
	if v := os.Getenv("QDRANT_URL"); v != "" {
		cfg.QdrantURL = v
	}
	if v := os.Getenv("QDRANT_API_KEY"); v != "" {
		cfg.QdrantAPIKey = v
	}
	if v := os.Getenv("QDRANT_COLLECTION"); v != "" {
		cfg.QdrantCollection = v
	}
	if v := os.Getenv("OPENSEARCH_URL"); v != "" {
		cfg.OpenSearchURL = v
	}
	if v := os.Getenv("OPENSEARCH_INDEX"); v != "" {
		cfg.OpenSearchIndex = v
	}
	if v := os.Getenv("OPENSEARCH_USERNAME"); v != "" {
		cfg.OpenSearchUsername = v
	}
	if v := os.Getenv("OPENSEARCH_PASSWORD"); v != "" {
		cfg.OpenSearchPassword = v
	}
	if v := os.Getenv("OLLAMA_HOST"); v != "" {
		cfg.EmbeddingBaseURL = v
		cfg.EmbeddingProvider = "ollama"
	}
	if v := os.Getenv("OLLAMA_EMBEDDING_MODEL"); v != "" {
		cfg.EmbeddingModel = v
		cfg.EmbeddingProvider = "ollama"
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
	if strings.TrimSpace(cfg.InternalJWTPrivateKeyPath) == "" || strings.TrimSpace(cfg.InternalJWTPublicKeyPath) == "" {
		return errors.New("config: internal service auth requires ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH + ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH")
	}
	if strings.TrimSpace(cfg.RabbitMQURL) == "" {
		return errors.New("config: rabbitmqURL is required (set in config.yaml or RABBITMQ_URL)")
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.EmbeddingProvider))
	if provider == "" {
		provider = "ollama"
	}
	if cfg.EmbeddingModel == "" {
		return errors.New("config: embeddingModel is required (set in config.yaml or OLLAMA_EMBEDDING_MODEL)")
	}
	if cfg.EmbeddingDim <= 0 {
		return errors.New("config: embeddingDim is required (set ONEBOOK_EMBEDDING_DIM)")
	}
	if strings.TrimSpace(cfg.QdrantURL) == "" {
		return errors.New("config: qdrantURL is required (set QDRANT_URL)")
	}
	if strings.TrimSpace(cfg.QdrantCollection) == "" {
		return errors.New("config: qdrantCollection is required (set QDRANT_COLLECTION)")
	}
	if strings.TrimSpace(cfg.OpenSearchURL) == "" {
		return errors.New("config: openSearchURL is required (set OPENSEARCH_URL)")
	}
	if strings.TrimSpace(cfg.OpenSearchIndex) == "" {
		return errors.New("config: openSearchIndex is required (set OPENSEARCH_INDEX)")
	}
	switch provider {
	case "ollama":
	default:
		return errors.New("config: embeddingProvider must be ollama")
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
