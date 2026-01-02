package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// FileConfig represents configuration loaded from YAML.
type FileConfig struct {
	Port           string `yaml:"port"`
	LogLevel       string `yaml:"logLevel"`
	DatabaseURL    string `yaml:"databaseURL"`
	BookServiceURL string `yaml:"bookServiceURL"`
	InternalToken  string `yaml:"internalToken"`
	GeminiAPIKey   string `yaml:"geminiAPIKey"`
	EmbeddingModel string `yaml:"embeddingModel"`
	EmbeddingDim   int    `yaml:"embeddingDim"`
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
	if cfg.InternalToken == "" {
		cfg.InternalToken = os.Getenv("ONEBOOK_INTERNAL_TOKEN")
	}
	if cfg.GeminiAPIKey == "" {
		cfg.GeminiAPIKey = os.Getenv("GEMINI_API_KEY")
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
	if cfg.InternalToken == "" {
		return errors.New("config: internalToken is required (set in config.yaml or ONEBOOK_INTERNAL_TOKEN)")
	}
	if cfg.GeminiAPIKey == "" {
		return errors.New("config: geminiAPIKey is required (set in config.yaml or GEMINI_API_KEY)")
	}
	if cfg.EmbeddingModel == "" {
		return errors.New("config: embeddingModel is required (set in config.yaml)")
	}
	return nil
}
