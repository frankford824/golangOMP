package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type EmbeddingProvider interface {
	Ready() bool
	Version() string
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

type EmbeddingConfig struct {
	Enabled    bool
	BaseURL    string
	APIKey     string
	Model      string
	Dimensions int
	Timeout    time.Duration
	HTTP       *http.Client
}

type OpenAICompatibleEmbeddingClient struct {
	cfg  EmbeddingConfig
	http *http.Client
}

func NewOpenAICompatibleEmbeddingClient(cfg EmbeddingConfig) *OpenAICompatibleEmbeddingClient {
	client := cfg.HTTP
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 20 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &OpenAICompatibleEmbeddingClient{cfg: cfg, http: client}
}

func (c *OpenAICompatibleEmbeddingClient) Ready() bool {
	return c != nil && c.cfg.Enabled && strings.TrimSpace(c.cfg.BaseURL) != "" &&
		strings.TrimSpace(c.cfg.APIKey) != "" && strings.TrimSpace(c.cfg.Model) != "" && c.cfg.Dimensions > 0
}

func (c *OpenAICompatibleEmbeddingClient) Version() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.cfg.Model) + ":" + fmt.Sprint(c.cfg.Dimensions)
}

func (c *OpenAICompatibleEmbeddingClient) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if !c.Ready() {
		return nil, errors.New("embedding provider is not configured")
	}
	if len(inputs) == 0 {
		return [][]float32{}, nil
	}
	body := map[string]any{"model": c.cfg.Model, "input": inputs, "dimensions": c.cfg.Dimensions}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, embeddingsURL(c.cfg.BaseURL), bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("build embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call embedding provider: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding provider returned HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(envelope.Data) != len(inputs) {
		return nil, fmt.Errorf("embedding response count %d does not match input count %d", len(envelope.Data), len(inputs))
	}
	result := make([][]float32, len(inputs))
	for _, item := range envelope.Data {
		if item.Index < 0 || item.Index >= len(result) || len(item.Embedding) != c.cfg.Dimensions {
			return nil, fmt.Errorf("embedding response has invalid index or dimensions")
		}
		result[item.Index] = item.Embedding
	}
	for _, vector := range result {
		if len(vector) != c.cfg.Dimensions {
			return nil, fmt.Errorf("embedding response is missing an input vector")
		}
	}
	return result, nil
}

func embeddingsURL(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/v1") {
		return base + "/embeddings"
	}
	return base + "/v1/embeddings"
}
