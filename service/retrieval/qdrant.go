package retrieval

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"workflow/domain"
)

type VectorStore interface {
	Ready() bool
	Search(ctx context.Context, vector []float32, limit int) ([]domain.AIRetrievalHit, error)
	Upsert(ctx context.Context, document domain.AIRetrievalDocument, vector []float32) error
	Delete(ctx context.Context, documentID string) error
}

type QdrantConfig struct {
	Enabled         bool
	BaseURL         string
	APIKey          string
	CollectionAlias string
	Timeout         time.Duration
	HTTP            *http.Client
}

type QdrantClient struct {
	cfg  QdrantConfig
	http *http.Client
}

func NewQdrantClient(cfg QdrantConfig) *QdrantClient {
	client := cfg.HTTP
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 2 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &QdrantClient{cfg: cfg, http: client}
}

func (c *QdrantClient) Ready() bool {
	return c != nil && c.cfg.Enabled && strings.TrimSpace(c.cfg.BaseURL) != "" && strings.TrimSpace(c.cfg.CollectionAlias) != ""
}

func (c *QdrantClient) Search(ctx context.Context, vector []float32, limit int) ([]domain.AIRetrievalHit, error) {
	if !c.Ready() {
		return nil, errors.New("qdrant is not configured")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	body := map[string]any{"query": vector, "limit": limit, "with_payload": true}
	var response struct {
		Result struct {
			Points []struct {
				ID      any            `json:"id"`
				Score   float64        `json:"score"`
				Payload map[string]any `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}
	if err := c.doJSON(ctx, http.MethodPost, c.collectionURL("/points/query"), body, &response); err != nil {
		return nil, err
	}
	items := make([]domain.AIRetrievalHit, 0, len(response.Result.Points))
	for _, point := range response.Result.Points {
		payload := point.Payload
		item := domain.AIRetrievalHit{
			DocumentID:    stringPayload(payload, "document_id"),
			EntityType:    stringPayload(payload, "entity_type"),
			EntityID:      stringPayload(payload, "entity_id"),
			Title:         stringPayload(payload, "title"),
			InternalRoute: stringPayload(payload, "internal_route"),
			Excerpt:       stringPayload(payload, "excerpt"),
			SourceVersion: stringPayload(payload, "source_version"),
			Score:         point.Score,
			Metadata:      payload,
			Source:        "qdrant",
		}
		if item.DocumentID == "" {
			item.DocumentID = fmt.Sprint(point.ID)
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *QdrantClient) Upsert(ctx context.Context, document domain.AIRetrievalDocument, vector []float32) error {
	if !c.Ready() {
		return errors.New("qdrant is not configured")
	}
	payload := map[string]any{
		"document_id": document.DocumentID, "entity_type": document.EntityType, "entity_id": document.EntityID,
		"title": document.Title, "internal_route": document.InternalRoute, "excerpt": truncate(document.SearchText, 1200),
		"source_version": document.EntityVersion, "visibility": document.Visibility, "content_hash": document.ContentHash,
		"embedding_version": document.EmbeddingVersion,
	}
	for key, value := range document.Metadata {
		if _, reserved := payload[key]; !reserved {
			payload[key] = value
		}
	}
	body := map[string]any{"points": []any{map[string]any{"id": qdrantPointID(document.DocumentID), "vector": vector, "payload": payload}}}
	return c.doJSON(ctx, http.MethodPut, c.collectionURL("/points?wait=true"), body, nil)
}

func (c *QdrantClient) Delete(ctx context.Context, documentID string) error {
	if !c.Ready() {
		return errors.New("qdrant is not configured")
	}
	body := map[string]any{"points": []string{qdrantPointID(documentID)}}
	return c.doJSON(ctx, http.MethodPost, c.collectionURL("/points/delete?wait=true"), body, nil)
}

func (c *QdrantClient) EnsureCollection(ctx context.Context, collection string, dimensions int) error {
	if c == nil || strings.TrimSpace(c.cfg.BaseURL) == "" || dimensions <= 0 {
		return errors.New("qdrant collection configuration is invalid")
	}
	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/collections/" + url.PathEscape(strings.TrimSpace(collection))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if key := strings.TrimSpace(c.cfg.APIKey); key != "" {
		request.Header.Set("api-key", key)
	}
	response, err := c.http.Do(request)
	if err == nil && response != nil {
		_ = response.Body.Close()
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return nil
		}
		if response.StatusCode != http.StatusNotFound {
			return fmt.Errorf("qdrant collection probe returned HTTP %d", response.StatusCode)
		}
	}
	body := map[string]any{"vectors": map[string]any{"size": dimensions, "distance": "Cosine"}, "on_disk_payload": true}
	return c.doJSON(ctx, http.MethodPut, endpoint, body, nil)
}

func (c *QdrantClient) SwitchAlias(ctx context.Context, alias, collection string) error {
	if c == nil || strings.TrimSpace(c.cfg.BaseURL) == "" || strings.TrimSpace(alias) == "" || strings.TrimSpace(collection) == "" {
		return errors.New("qdrant alias configuration is invalid")
	}
	exists, err := c.aliasExists(ctx, alias)
	if err != nil {
		return err
	}
	actions := []any{map[string]any{"create_alias": map[string]any{"alias_name": alias, "collection_name": collection}}}
	if exists {
		actions = append([]any{map[string]any{"delete_alias": map[string]any{"alias_name": alias}}}, actions...)
	}
	body := map[string]any{"actions": actions}
	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/collections/aliases"
	return c.doJSON(ctx, http.MethodPost, endpoint, body, nil)
}

func (c *QdrantClient) aliasExists(ctx context.Context, alias string) (bool, error) {
	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/aliases/" + url.PathEscape(strings.TrimSpace(alias))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	if key := strings.TrimSpace(c.cfg.APIKey); key != "" {
		request.Header.Set("api-key", key)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return false, fmt.Errorf("probe qdrant alias: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false, fmt.Errorf("qdrant alias probe returned HTTP %d", response.StatusCode)
	}
	return true, nil
}

func (c *QdrantClient) CreateSnapshot(ctx context.Context, collection string) error {
	if c == nil || strings.TrimSpace(c.cfg.BaseURL) == "" || strings.TrimSpace(collection) == "" {
		return errors.New("qdrant snapshot configuration is invalid")
	}
	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/collections/" + url.PathEscape(collection) + "/snapshots"
	return c.doJSON(ctx, http.MethodPost, endpoint, map[string]any{}, nil)
}

// DeleteCollection is intentionally explicit and is used by integration-test
// cleanup and operator-controlled retirement only. Runtime indexing never
// deletes a collection because older versions are the vector rollback path.
func (c *QdrantClient) DeleteCollection(ctx context.Context, collection string) error {
	if c == nil || strings.TrimSpace(c.cfg.BaseURL) == "" || strings.TrimSpace(collection) == "" {
		return errors.New("qdrant collection configuration is invalid")
	}
	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/collections/" + url.PathEscape(strings.TrimSpace(collection))
	return c.doJSON(ctx, http.MethodDelete, endpoint, map[string]any{}, nil)
}

func (c *QdrantClient) collectionURL(suffix string) string {
	return strings.TrimRight(c.cfg.BaseURL, "/") + "/collections/" + url.PathEscape(c.cfg.CollectionAlias) + suffix
}

func (c *QdrantClient) doJSON(ctx context.Context, method, endpoint string, body any, target any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal qdrant request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("build qdrant request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if key := strings.TrimSpace(c.cfg.APIKey); key != "" {
		req.Header.Set("api-key", key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call qdrant: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("read qdrant response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant returned HTTP %d", resp.StatusCode)
	}
	if target != nil && len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, target); err != nil {
			return fmt.Errorf("decode qdrant response: %w", err)
		}
	}
	return nil
}

func stringPayload(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len([]rune(value)) <= max {
		return value
	}
	return string([]rune(value)[:max])
}

// Qdrant point IDs accept unsigned integers or UUIDs. Retrieval document IDs
// deliberately include a type prefix (for example "task:123"), so derive a
// stable UUID while retaining the authoritative document ID in the payload.
func qdrantPointID(documentID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(documentID)))
	// RFC 4122 variant with a deterministic, name-derived version marker.
	sum[6] = (sum[6] & 0x0f) | 0x50
	sum[8] = (sum[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}
