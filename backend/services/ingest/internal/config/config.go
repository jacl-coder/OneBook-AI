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
	IndexerURL     string `yaml:"indexerURL"`
	InternalToken  string `yaml:"internalToken"`
	ChunkSize      int    `yaml:"chunkSize"`
	ChunkOverlap   int    `yaml:"chunkOverlap"`
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
	return nil
}
