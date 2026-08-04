package retrieval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"workflow/domain"
)

func TestQdrantClientSearchUpsertDeleteAndAPIKey(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") != "q-secret" {
			t.Fatalf("missing api key")
		}
		paths = append(paths, r.Method+" "+r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/collections/stable/points/query" {
			_, _ = w.Write([]byte(`{"result":{"points":[{"id":"doc","score":0.8,"payload":{"document_id":"doc","entity_type":"task","entity_id":"4","title":"任务 4"}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":true}`))
	}))
	defer server.Close()
	client := NewQdrantClient(QdrantConfig{Enabled: true, BaseURL: server.URL, APIKey: "q-secret", CollectionAlias: "stable"})
	hits, err := client.Search(context.Background(), []float32{1, 2}, 5)
	if err != nil || len(hits) != 1 || hits[0].DocumentID != "doc" {
		t.Fatalf("hits=%+v err=%v", hits, err)
	}
	if err := client.Upsert(context.Background(), domain.AIRetrievalDocument{DocumentID: "doc", EntityType: "task", EntityID: "4", Metadata: map[string]any{"task_no": "T4"}}, []float32{1, 2}); err != nil {
		t.Fatal(err)
	}
	if err := client.Delete(context.Background(), "doc"); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("paths=%v", paths)
	}
}

func TestQdrantCollectionAndAliasLifecycle(t *testing.T) {
	var mu sync.Mutex
	var requests []struct {
		method, path string
		body         map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		requests = append(requests, struct {
			method, path string
			body         map[string]any
		}{r.Method, r.URL.Path, body})
		mu.Unlock()
		if r.Method == http.MethodGet && r.URL.Path == "/collections/v2" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":true}`))
	}))
	defer server.Close()
	client := NewQdrantClient(QdrantConfig{Enabled: true, BaseURL: server.URL, APIKey: "q", CollectionAlias: "stable"})
	if err := client.EnsureCollection(context.Background(), "v2", 768); err != nil {
		t.Fatal(err)
	}
	if err := client.CreateSnapshot(context.Background(), "v2"); err != nil {
		t.Fatal(err)
	}
	if err := client.SwitchAlias(context.Background(), "stable", "v2"); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteCollection(context.Background(), "v2"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 6 {
		t.Fatalf("requests=%+v", requests)
	}
	put := requests[1]
	vectors := put.body["vectors"].(map[string]any)
	if vectors["size"].(float64) != 768 || vectors["distance"] != "Cosine" {
		t.Fatalf("collection body=%v", put.body)
	}
	aliases := requests[4].body["actions"].([]any)
	if len(aliases) != 2 {
		t.Fatalf("alias actions=%v", aliases)
	}
	if requests[5].method != http.MethodDelete || requests[5].path != "/collections/v2" {
		t.Fatalf("delete request=%+v", requests[5])
	}
}

func TestQdrantPointIDIsStableUUID(t *testing.T) {
	first := qdrantPointID("task:123")
	if first != qdrantPointID("task:123") {
		t.Fatal("point ID must be stable")
	}
	if first == qdrantPointID("task:124") {
		t.Fatal("different document IDs must not share a point ID")
	}
	if len(first) != 36 || first[14] != '5' {
		t.Fatalf("point ID is not a deterministic UUID: %q", first)
	}
}
