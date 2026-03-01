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
	Port               string `yaml:"port"`
	DatabaseURL        string `yaml:"databaseURL"`
	LogLevel           string `yaml:"logLevel"`
	AuthServiceURL     string `yaml:"authServiceURL"`
	AuthJWKSURL        string `yaml:"authJwksURL"`
	JWTIssuer          string `yaml:"jwtIssuer"`
	JWTAudience        string `yaml:"jwtAudience"`
	JWTLeeway          string `yaml:"jwtLeeway"`
	BookServiceURL     string `yaml:"bookServiceURL"`
	GenerationProvider string `yaml:"generationProvider"`
	GenerationBaseURL  string `yaml:"generationBaseURL"`
	GenerationAPIKey   string `yaml:"generationAPIKey"`
	GenerationModel    string `yaml:"generationModel"`
	EmbeddingProvider  string `yaml:"embeddingProvider"`
	EmbeddingBaseURL   string `yaml:"embeddingBaseURL"`
	EmbeddingModel     string `yaml:"embeddingModel"`
	EmbeddingDim       int    `yaml:"embeddingDim"`
	TopK               int    `yaml:"topK"`
	HistoryLimit       int    `yaml:"historyLimit"`
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
	if v := os.Getenv("CHAT_AUTH_SERVICE_URL"); v != "" {
		cfg.AuthServiceURL = v
	}
	if v := os.Getenv("CHAT_AUTH_JWKS_URL"); v != "" {
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
	if v := os.Getenv("CHAT_BOOK_SERVICE_URL"); v != "" {
		cfg.BookServiceURL = v
	}
	if v := os.Getenv("GENERATION_PROVIDER"); v != "" {
		cfg.GenerationProvider = v
	}
	if v := os.Getenv("GENERATION_BASE_URL"); v != "" {
		cfg.GenerationBaseURL = v
	}
	if v := os.Getenv("GENERATION_API_KEY"); v != "" {
		cfg.GenerationAPIKey = v
	}
	if v := os.Getenv("GENERATION_MODEL"); v != "" {
		cfg.GenerationModel = v
	}
	if v := os.Getenv("ONEBOOK_EMBEDDING_DIM"); v != "" {
		if dim, err := strconv.Atoi(v); err == nil {
			cfg.EmbeddingDim = dim
		}
	}
	if v := os.Getenv("OLLAMA_HOST"); v != "" {
		cfg.EmbeddingBaseURL = v
		cfg.EmbeddingProvider = "ollama"
	}
	if v := os.Getenv("OLLAMA_EMBEDDING_MODEL"); v != "" {
		cfg.EmbeddingModel = v
		cfg.EmbeddingProvider = "ollama"
	}
	if v := os.Getenv("CHAT_HISTORY_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.HistoryLimit = n
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
	if cfg.AuthServiceURL == "" {
		return errors.New("config: authServiceURL is required (set in config.yaml or CHAT_AUTH_SERVICE_URL)")
	}
	if strings.TrimSpace(cfg.AuthJWKSURL) == "" {
		return errors.New("config: authJwksURL is required (set in config.yaml or CHAT_AUTH_JWKS_URL)")
	}
	if cfg.BookServiceURL == "" {
		return errors.New("config: bookServiceURL is required (set in config.yaml or CHAT_BOOK_SERVICE_URL)")
	}
	genProvider := strings.ToLower(strings.TrimSpace(cfg.GenerationProvider))
	if genProvider == "" {
		genProvider = "gemini"
	}
	switch genProvider {
	case "gemini":
		if cfg.GenerationAPIKey == "" {
			return errors.New("config: generationAPIKey is required for gemini (set GENERATION_API_KEY)")
		}
	case "ollama":
		// ollama generation does not require an API key
	case "openai-compat":
		if cfg.GenerationBaseURL == "" {
			return errors.New("config: generationBaseURL is required for openai-compat (set GENERATION_BASE_URL)")
		}
	default:
		return fmt.Errorf("config: generationProvider must be gemini, ollama, or openai-compat (got %q)", genProvider)
	}
	if cfg.GenerationModel == "" {
		return errors.New("config: generationModel is required (set GENERATION_MODEL)")
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
	switch provider {
	case "ollama":
	default:
		return errors.New("config: embeddingProvider must be ollama")
	}
	return nil
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
