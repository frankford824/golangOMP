package retrieval

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEmbeddingClientUsesOpenAICompatibleContractAndRestoresInputOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("request=%s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"index":1,"embedding":[4,5,6]},{"index":0,"embedding":[1,2,3]}]}`))
	}))
	defer server.Close()
	client := NewOpenAICompatibleEmbeddingClient(EmbeddingConfig{Enabled: true, BaseURL: server.URL, APIKey: "secret", Model: "embed-v1", Dimensions: 3, Timeout: time.Second})
	vectors, err := client.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 2 || vectors[0][0] != 1 || vectors[1][0] != 4 {
		t.Fatalf("vectors=%v", vectors)
	}
}

func TestEmbeddingClientRejectsDimensionDrift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,2]}]}`))
	}))
	defer server.Close()
	client := NewOpenAICompatibleEmbeddingClient(EmbeddingConfig{Enabled: true, BaseURL: server.URL, APIKey: "secret", Model: "embed-v1", Dimensions: 3})
	if _, err := client.Embed(context.Background(), []string{"a"}); err == nil {
		t.Fatal("dimension drift should fail")
	}
}
