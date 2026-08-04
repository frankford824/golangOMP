//go:build integration

package retrieval

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func TestQdrantRealCollectionAliasSnapshotAndDocumentLifecycle(t *testing.T) {
	baseURL := strings.TrimSpace(os.Getenv("QDRANT_TEST_URL"))
	apiKey := strings.TrimSpace(os.Getenv("QDRANT_TEST_API_KEY"))
	if baseURL == "" || apiKey == "" {
		t.Skip("QDRANT_TEST_URL and QDRANT_TEST_API_KEY are required")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	collection := "yongbo_retrieval_integration_" + suffix
	alias := "yongbo_retrieval_integration_current_" + suffix
	client := NewQdrantClient(QdrantConfig{
		Enabled: true, BaseURL: baseURL, APIKey: apiKey,
		CollectionAlias: collection, Timeout: 10 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.EnsureCollection(ctx, collection, 3); err != nil {
		t.Fatalf("ensure collection: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := client.DeleteCollection(cleanupCtx, collection); err != nil {
			t.Errorf("delete integration collection: %v", err)
		}
	})
	document := domain.AIRetrievalDocument{
		DocumentID: "task:integration:1", EntityType: "task", EntityID: "1",
		Title: "集成测试任务", SearchText: "蓝色套装 花纹", InternalRoute: "/tasks/1",
		EntityVersion: "v1", Visibility: "internal", ContentHash: "hash-v1", EmbeddingVersion: "test-v1",
	}
	if err := client.Upsert(ctx, document, []float32{1, 0, 0}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := client.SwitchAlias(ctx, alias, collection); err != nil {
		t.Fatalf("switch alias: %v", err)
	}
	aliasClient := NewQdrantClient(QdrantConfig{
		Enabled: true, BaseURL: baseURL, APIKey: apiKey,
		CollectionAlias: alias, Timeout: 10 * time.Second,
	})
	hits, err := aliasClient.Search(ctx, []float32{1, 0, 0}, 10)
	if err != nil {
		t.Fatalf("search through alias: %v", err)
	}
	if len(hits) != 1 || hits[0].DocumentID != document.DocumentID || hits[0].InternalRoute != document.InternalRoute {
		t.Fatalf("unexpected hits: %+v", hits)
	}
	if err := client.CreateSnapshot(ctx, collection); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if err := aliasClient.Delete(ctx, document.DocumentID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	hits, err = aliasClient.Search(ctx, []float32{1, 0, 0}, 10)
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("deleted document remains visible: %+v", hits)
	}
}
