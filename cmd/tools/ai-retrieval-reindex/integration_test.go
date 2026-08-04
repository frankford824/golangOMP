//go:build integration

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"workflow/service/retrieval"
	"workflow/testsupport/r35"
)

func TestAIReindexBuildsMySQLProjectionAndRealQdrantAlias(t *testing.T) {
	qdrantURL := strings.TrimSpace(os.Getenv("QDRANT_TEST_URL"))
	qdrantKey := strings.TrimSpace(os.Getenv("QDRANT_TEST_API_KEY"))
	if qdrantURL == "" || qdrantKey == "" {
		t.Skip("QDRANT_TEST_URL and QDRANT_TEST_API_KEY are required")
	}
	db := r35.MustOpenTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	requireAIReindexSchema(t, db)
	seedAIReindexSources(t, db)

	embedding := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" || r.Header.Get("Authorization") != "Bearer integration-key" {
			http.Error(w, "unexpected embedding request", http.StatusUnauthorized)
			return
		}
		var request struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		data := make([]map[string]any, len(request.Input))
		for index := range request.Input {
			data[index] = map[string]any{"index": index, "embedding": []float32{1, float32(index), 0.5}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer embedding.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	alias := "yongbo_reindex_e2e_current_" + suffix
	target := "yongbo_reindex_e2e_" + suffix
	t.Setenv("AI_CHAT_ENABLED", "false")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")
	t.Setenv("VECTOR_SEARCH_ENABLED", "true")
	t.Setenv("AI_EMBEDDING_ENABLED", "true")
	t.Setenv("AI_EMBEDDING_BASE_URL", embedding.URL)
	t.Setenv("AI_EMBEDDING_API_KEY", "integration-key")
	t.Setenv("AI_EMBEDDING_MODEL", "integration-embedding")
	t.Setenv("AI_EMBEDDING_VERSION", "integration-embedding:v1")
	t.Setenv("AI_EMBEDDING_DIMENSIONS", "3")
	t.Setenv("QDRANT_BASE_URL", qdrantURL)
	t.Setenv("QDRANT_API_KEY", qdrantKey)
	t.Setenv("QDRANT_COLLECTION_ALIAS", alias)
	t.Setenv("QDRANT_TIMEOUT", "10s")
	t.Setenv("AI_RETRIEVAL_WORKER_ENABLED", "false")

	result, err := run(context.Background(), os.Getenv("MYSQL_DSN"), target, true, true, true, 2, time.Minute)
	if err != nil {
		t.Fatalf("first reindex: %v", err)
	}
	cleanupClient := retrieval.NewQdrantClient(retrieval.QdrantConfig{
		Enabled: true, BaseURL: qdrantURL, APIKey: qdrantKey, CollectionAlias: target, Timeout: 10 * time.Second,
	})
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := cleanupClient.DeleteCollection(cleanupCtx, target); err != nil {
			t.Errorf("delete reindex integration collection: %v", err)
		}
	})
	if result.Projection.Tasks != 1 || result.Projection.ResourceGroups != 0 || result.Projection.ExternalAssets != 1 ||
		result.Projection.Documents != 2 || result.Indexed != 2 || !result.AliasSwitched || !result.SnapshotCreated {
		t.Fatalf("unexpected first reindex report: %+v", result)
	}
	assertAIReindexState(t, db, alias, qdrantURL, qdrantKey)

	result, err = run(context.Background(), os.Getenv("MYSQL_DSN"), target, true, true, false, 2, time.Minute)
	if err != nil {
		t.Fatalf("idempotent reindex: %v", err)
	}
	if result.Indexed != 2 || !result.AliasSwitched {
		t.Fatalf("unexpected rerun report: %+v", result)
	}
	assertAIReindexState(t, db, alias, qdrantURL, qdrantKey)
}

func requireAIReindexSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"tasks", "task_search_documents", "task_asset_group_search_documents", "external_asset_records", "ai_retrieval_documents", "ai_retrieval_outbox"} {
		var found string
		if err := db.QueryRow(`SELECT table_name FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&found); err != nil {
			t.Fatalf("required table %s is missing; apply the verified baseline and migration 129 first: %v", table, err)
		}
	}
}

func seedAIReindexSources(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`DELETE FROM ai_retrieval_outbox`,
		`DELETE FROM ai_retrieval_documents`,
		`DELETE FROM task_asset_group_search_documents`,
		`DELETE FROM task_search_documents`,
		`DELETE FROM external_asset_records`,
		`DELETE FROM tasks WHERE id=99012901`,
		`INSERT INTO tasks
		  (id, task_no, source_mode, sku_code, product_name_snapshot, task_type, creator_id, task_status,
		   priority, owner_team, business_lane, is_batch_task, batch_item_count, batch_mode,
		   primary_sku_code, sku_generation_status, owner_department, owner_org_team, workflow_revision)
		 VALUES
		  (99012901, 'AI-E2E-99012901', 'existing_product', 'AI-SKU-99012901', '蓝色套装', 'regular', 1,
		   'Completed', 'normal', 'AI测试组', 'normal', 0, 1, 'single', 'AI-SKU-99012901', 'not_applicable',
		   'AI测试部', 'AI测试组', 1)`,
		`INSERT INTO task_search_documents
		  (task_id, task_no, product_name_snapshot, sku_code, primary_sku_code, task_type, task_status,
		   priority, owner_department, owner_team, owner_org_team, creator_id, creator_name,
		   requester_name, designer_name, current_handler_name, created_at, updated_at, asset_text, search_text)
		 VALUES
		  (99012901, 'AI-E2E-99012901', '蓝色套装', 'AI-SKU-99012901', 'AI-SKU-99012901', 'regular',
		   'Completed', 'normal', 'AI测试部', 'AI测试组', 'AI测试组', 1, '集成测试', '', '', '', UTC_TIMESTAMP(),
		   UTC_TIMESTAMP(), '蓝色 套装 成品', 'AI-E2E-99012901 AI-SKU-99012901 蓝色套装 最终成品图')`,
		`INSERT INTO external_asset_records
		  (id, provider, kind, driver, mount_path, origin_path_hash, origin_path, parent_path, file_name,
		   file_ext, mime_type, file_size, is_dir, status, searchable_text)
		 VALUES
		  (99012902, 'integration', 'nas_local', 'fixture', '/integration',
		   'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', '/integration/blue-set.png',
		   '/integration', 'blue-set.png', 'png', 'image/png', 128, 0, 'indexed', '蓝色套装 外部参考图')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed AI reindex fixture: %v\n%s", err, statement)
		}
	}
	t.Cleanup(func() {
		for _, statement := range []string{
			`DELETE FROM ai_retrieval_outbox`,
			`DELETE FROM ai_retrieval_documents`,
			`DELETE FROM task_search_documents WHERE task_id=99012901`,
			`DELETE FROM external_asset_records WHERE id=99012902`,
			`DELETE FROM tasks WHERE id=99012901`,
		} {
			if _, err := db.Exec(statement); err != nil {
				t.Errorf("cleanup AI reindex fixture: %v", err)
			}
		}
	})
}

func assertAIReindexState(t *testing.T, db *sql.DB, alias, qdrantURL, qdrantKey string) {
	t.Helper()
	var documents, indexed, outbox int
	if err := db.QueryRow(`SELECT COUNT(*), SUM(vector_indexed_at IS NOT NULL) FROM ai_retrieval_documents WHERE deleted_at IS NULL`).Scan(&documents, &indexed); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM ai_retrieval_outbox`).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if documents != 2 || indexed != 2 || outbox != 2 {
		t.Fatalf("unexpected mysql projection state documents=%d indexed=%d outbox=%d", documents, indexed, outbox)
	}
	client := retrieval.NewQdrantClient(retrieval.QdrantConfig{
		Enabled: true, BaseURL: qdrantURL, APIKey: qdrantKey, CollectionAlias: alias, Timeout: 10 * time.Second,
	})
	hits, err := client.Search(context.Background(), []float32{1, 0, 0.5}, 10)
	if err != nil {
		t.Fatalf("search qdrant alias: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 qdrant hits, got %+v", hits)
	}
}
