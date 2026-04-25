package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsFeatureFlagsToEnabled(t *testing.T) {
	path := writeTempConfig(t, `
port: "8084"
databaseURL: "postgres://onebook:onebook@localhost:5432/onebook?sslmode=disable"
authServiceURL: "http://localhost:8082"
authJwksURL: "http://localhost:8082/auth/jwks"
bookServiceURL: "http://localhost:8083"
generationProvider: "ollama"
generationBaseURL: "http://localhost:11434"
generationModel: "qwen3"
embeddingProvider: "ollama"
embeddingBaseURL: "http://localhost:11434"
embeddingModel: "qwen3-embedding"
embeddingDim: 3072
qdrantURL: "http://localhost:6333"
qdrantCollection: "onebook_chunks"
openSearchURL: "http://localhost:9200"
openSearchIndex: "onebook_lexical_chunks"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.QueryRewriteEnabled {
		t.Fatal("QueryRewriteEnabled = false, want true")
	}
	if !cfg.MultiQueryEnabled {
		t.Fatal("MultiQueryEnabled = false, want true")
	}
	if !cfg.AbstainEnabled {
		t.Fatal("AbstainEnabled = false, want true")
	}
}

func TestLoadReadsFeatureFlagsFromEnv(t *testing.T) {
	t.Setenv("CHAT_QUERY_REWRITE_ENABLED", "false")
	t.Setenv("CHAT_MULTI_QUERY_ENABLED", "false")
	t.Setenv("CHAT_ABSTAIN_ENABLED", "false")

	path := writeTempConfig(t, `
port: "8084"
databaseURL: "postgres://onebook:onebook@localhost:5432/onebook?sslmode=disable"
authServiceURL: "http://localhost:8082"
authJwksURL: "http://localhost:8082/auth/jwks"
bookServiceURL: "http://localhost:8083"
generationProvider: "ollama"
generationBaseURL: "http://localhost:11434"
generationModel: "qwen3"
embeddingProvider: "ollama"
embeddingBaseURL: "http://localhost:11434"
embeddingModel: "qwen3-embedding"
embeddingDim: 3072
qdrantURL: "http://localhost:6333"
qdrantCollection: "onebook_chunks"
openSearchURL: "http://localhost:9200"
openSearchIndex: "onebook_lexical_chunks"
queryRewriteEnabled: true
multiQueryEnabled: true
abstainEnabled: true
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.QueryRewriteEnabled {
		t.Fatal("QueryRewriteEnabled = true, want false")
	}
	if cfg.MultiQueryEnabled {
		t.Fatal("MultiQueryEnabled = true, want false")
	}
	if cfg.AbstainEnabled {
		t.Fatal("AbstainEnabled = true, want false")
	}
}

func writeTempConfig(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}
