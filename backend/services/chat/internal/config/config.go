package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// FileConfig represents configuration loaded from YAML.
type FileConfig struct {
	Port            string `yaml:"port"`
	DatabaseURL     string `yaml:"databaseURL"`
	LogLevel        string `yaml:"logLevel"`
	GeminiAPIKey    string `yaml:"geminiAPIKey"`
	GenerationModel string `yaml:"generationModel"`
	EmbeddingModel  string `yaml:"embeddingModel"`
	TopK            int    `yaml:"topK"`
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
	if v := os.Getenv("GEMINI_API_KEY"); v != "" {
		cfg.GeminiAPIKey = v
	}
	if v := os.Getenv("GEMINI_GENERATION_MODEL"); v != "" {
		cfg.GenerationModel = v
	}
	if v := os.Getenv("GEMINI_EMBEDDING_MODEL"); v != "" {
		cfg.EmbeddingModel = v
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
	if cfg.GeminiAPIKey == "" {
		return errors.New("config: geminiAPIKey is required (set in config.yaml or GEMINI_API_KEY)")
	}
	if cfg.GenerationModel == "" {
		return errors.New("config: generationModel is required (set in config.yaml)")
	}
	if cfg.EmbeddingModel == "" {
		return errors.New("config: embeddingModel is required (set in config.yaml)")
	}
	return nil
}
