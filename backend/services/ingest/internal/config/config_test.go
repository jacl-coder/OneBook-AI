package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadChunkEnvOverrides(t *testing.T) {
	t.Setenv("INGEST_CHUNK_SIZE", "1024")
	t.Setenv("INGEST_CHUNK_OVERLAP", "256")
	t.Setenv("INGEST_OCR_ENABLED", "true")
	t.Setenv("INGEST_OCR_COMMAND", "paddleocr")
	t.Setenv("INGEST_OCR_DEVICE", "cpu")
	t.Setenv("INGEST_OCR_TIMEOUT_SECONDS", "180")
	t.Setenv("INGEST_PDF_MIN_PAGE_RUNES", "96")
	t.Setenv("INGEST_PDF_MIN_PAGE_SCORE", "0.55")
	t.Setenv("INGEST_PDF_OCR_MIN_SCORE_DELTA", "0.12")

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
	if !cfg.OCREnabled {
		t.Fatalf("ocrEnabled = false, want true")
	}
	if cfg.OCRCommand != "paddleocr" {
		t.Fatalf("ocrCommand = %q, want %q", cfg.OCRCommand, "paddleocr")
	}
	if cfg.OCRDevice != "cpu" {
		t.Fatalf("ocrDevice = %q, want %q", cfg.OCRDevice, "cpu")
	}
	if cfg.OCRTimeoutSeconds != 180 {
		t.Fatalf("ocrTimeoutSeconds = %d, want 180", cfg.OCRTimeoutSeconds)
	}
	if cfg.PDFMinPageRunes != 96 {
		t.Fatalf("pdfMinPageRunes = %d, want 96", cfg.PDFMinPageRunes)
	}
	if cfg.PDFMinPageScore != 0.55 {
		t.Fatalf("pdfMinPageScore = %f, want 0.55", cfg.PDFMinPageScore)
	}
	if cfg.PDFOCRMinScoreDelta != 0.12 {
		t.Fatalf("pdfOcrMinScoreDelta = %f, want 0.12", cfg.PDFOCRMinScoreDelta)
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

func TestValidateConfigRejectsMissingOCRCommand(t *testing.T) {
	cfg := FileConfig{
		Port:                      "8084",
		DatabaseURL:               "postgres://onebook:onebook@localhost:5432/onebook?sslmode=disable",
		BookServiceURL:            "http://localhost:8082",
		IndexerURL:                "http://localhost:8085",
		InternalJWTPrivateKeyPath: "secrets/internal-jwt/private.pem",
		InternalJWTPublicKeyPath:  "secrets/internal-jwt/public.pem",
		RedisAddr:                 "localhost:6379",
		ChunkSize:                 800,
		ChunkOverlap:              120,
		OCREnabled:                true,
		OCRCommand:                " ",
	}
	if err := validateConfig(cfg); err == nil {
		t.Fatalf("validateConfig() expected error for missing OCR command")
	}
}

func TestValidateConfigRejectsInvalidPDFThresholds(t *testing.T) {
	cfg := FileConfig{
		Port:                      "8084",
		DatabaseURL:               "postgres://onebook:onebook@localhost:5432/onebook?sslmode=disable",
		BookServiceURL:            "http://localhost:8082",
		IndexerURL:                "http://localhost:8085",
		InternalJWTPrivateKeyPath: "secrets/internal-jwt/private.pem",
		InternalJWTPublicKeyPath:  "secrets/internal-jwt/public.pem",
		RedisAddr:                 "localhost:6379",
		ChunkSize:                 800,
		ChunkOverlap:              120,
		PDFMinPageScore:           1.5,
	}
	if err := validateConfig(cfg); err == nil {
		t.Fatalf("validateConfig() expected error for invalid pdfMinPageScore")
	}
}
