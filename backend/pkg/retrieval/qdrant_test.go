package retrieval

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQdrantPointIDDeterministic(t *testing.T) {
	payload := map[string]any{
		"book_id":  "book-1",
		"chunk_id": "ae56145986c5af247ecd931d",
	}

	first := qdrantPointID("ae56145986c5af247ecd931d", payload)
	second := qdrantPointID("ae56145986c5af247ecd931d", payload)
	if first == "" {
		t.Fatal("qdrantPointID() returned empty string")
	}
	if first != second {
		t.Fatalf("qdrantPointID() not deterministic: %q != %q", first, second)
	}
}

func TestQdrantPointIDDifferentBookProducesDifferentUUID(t *testing.T) {
	id1 := qdrantPointID("same-chunk", map[string]any{
		"book_id":  "book-1",
		"chunk_id": "same-chunk",
	})
	id2 := qdrantPointID("same-chunk", map[string]any{
		"book_id":  "book-2",
		"chunk_id": "same-chunk",
	})
	if id1 == id2 {
		t.Fatalf("qdrantPointID() should differ across books: %q", id1)
	}
}

func TestParseQdrantPointsUsesBusinessChunkID(t *testing.T) {
	points := parseQdrantPoints([]any{
		map[string]any{
			"id":    "8f9fe5f5-d4d5-5c6f-a1d0-5d18877fd4f0",
			"score": 0.9,
			"payload": map[string]any{
				"chunk_id": "ae56145986c5af247ecd931d",
				"content":  "content",
			},
		},
	})
	if len(points) != 1 {
		t.Fatalf("parseQdrantPoints() len = %d, want 1", len(points))
	}
	if points[0].ID != "ae56145986c5af247ecd931d" {
		t.Fatalf("parseQdrantPoints() ID = %q, want business chunk id", points[0].ID)
	}
}

func TestEnsureCollectionIgnoresAlreadyExistsConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/collections/onebook_chunks" {
			t.Fatalf("path = %s, want /collections/onebook_chunks", r.URL.Path)
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"status":{"error":"Wrong input: Collection ` + "`onebook_chunks`" + ` already exists!"},"time":0.0}`))
	}))
	defer server.Close()

	client, err := NewQdrantClient(server.URL, "", "onebook_chunks", 3072)
	if err != nil {
		t.Fatalf("NewQdrantClient() error = %v", err)
	}
	if err := client.EnsureCollection(context.Background()); err != nil {
		t.Fatalf("EnsureCollection() error = %v, want nil", err)
	}
}

func TestQueryDenseReturnsEmptyWhenCollectionMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status":{"error":"Collection not found"},"time":0.0}`))
	}))
	defer server.Close()

	client, err := NewQdrantClient(server.URL, "", "onebook_chunks", 3)
	if err != nil {
		t.Fatalf("NewQdrantClient() error = %v", err)
	}
	points, err := client.QueryDense(context.Background(), "book-1", []float32{0.1, 0.2, 0.3}, 5)
	if err != nil {
		t.Fatalf("QueryDense() error = %v, want nil", err)
	}
	if len(points) != 0 {
		t.Fatalf("QueryDense() len = %d, want 0", len(points))
	}
}
