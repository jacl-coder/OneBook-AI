package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadChunkEnvOverrides(t *testing.T) {
	t.Setenv("INGEST_CHUNK_SIZE", "1024")
	t.Setenv("INGEST_CHUNK_OVERLAP", "256")

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `
port: "8084"
logLevel: "info"
databaseURL: "postgres://onebook:onebook@localhost:5432/onebook?sslmode=disable"
bookServiceURL: "http://localhost:8082"
indexerURL: "http://localhost:8085"
internalJwtPrivateKeyPath: "secrets/internal-jwt/private.pem"
internalJwtPublicKeyPath: "secrets/internal-jwt/public.pem"
internalJwtKeyId: "internal-active"
redisAddr: "localhost:6379"
chunkSize: 800
chunkOverlap: 120
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ChunkSize != 1024 {
		t.Fatalf("chunkSize = %d, want 1024", cfg.ChunkSize)
	}
	if cfg.ChunkOverlap != 256 {
		t.Fatalf("chunkOverlap = %d, want 256", cfg.ChunkOverlap)
	}
}

func TestValidateConfigRejectsInvalidChunkSettings(t *testing.T) {
	cfg := FileConfig{
		Port:                      "8084",
		DatabaseURL:               "postgres://onebook:onebook@localhost:5432/onebook?sslmode=disable",
		BookServiceURL:            "http://localhost:8082",
		IndexerURL:                "http://localhost:8085",
		InternalJWTPrivateKeyPath: "secrets/internal-jwt/private.pem",
		InternalJWTPublicKeyPath:  "secrets/internal-jwt/public.pem",
		RedisAddr:                 "localhost:6379",
		ChunkSize:                 100,
		ChunkOverlap:              100,
	}
	if err := validateConfig(cfg); err == nil {
		t.Fatalf("validateConfig() expected error for chunkOverlap >= chunkSize")
	}
}
