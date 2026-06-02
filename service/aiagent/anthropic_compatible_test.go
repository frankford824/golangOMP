package aiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestParseTaskSummaryTextFromFencedJSON(t *testing.T) {
	raw := "```json\n{\"headline\":\"任务正常推进\",\"current_status\":\"已进入审核\",\"people\":[],\"timeline\":[],\"stuck_points\":[],\"sku_asset_erp_cost\":[],\"next_actions\":[\"等待审核\"],\"confidence\":\"high\"}\n```"
	got, err := ParseTaskSummaryText(raw)
	if err != nil {
		t.Fatalf("ParseTaskSummaryText() error = %v", err)
	}
	if got.Headline != "任务正常推进" {
		t.Fatalf("headline = %q", got.Headline)
	}
	if len(got.NextActions) != 1 || got.NextActions[0] != "等待审核" {
		t.Fatalf("next actions = %#v", got.NextActions)
	}
}

func TestMessagesURL(t *testing.T) {
	got := messagesURL("https://api.minimaxi.com/anthropic/")
	want := "https://api.minimaxi.com/anthropic/v1/messages"
	if got != want {
		t.Fatalf("messagesURL() = %q, want %q", got, want)
	}
}

func TestAnthropicClientLogsSanitizedSuccessMetadata(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-api-key" {
			t.Fatalf("Authorization=%q", got)
		}
		writeAnthropicText(t, w, `{"decision":"任务正常","impact":"继续推进","actions":[],"evidence":[],"confidence":"high"}`)
	}))
	defer server.Close()

	client := NewAnthropicCompatibleClient(Config{
		Enabled:   true,
		Provider:  "test_provider",
		BaseURL:   server.URL,
		APIKey:    "secret-api-key",
		Model:     "MiniMax-M3",
		Timeout:   time.Second,
		MaxTokens: 64,
	}, zap.New(core))
	summary, err := client.GenerateTaskSummary(context.Background(), map[string]string{
		"task": "sensitive-task-content",
	})
	if err != nil {
		t.Fatalf("GenerateTaskSummary() error=%v", err)
	}
	if summary.Decision != "任务正常" {
		t.Fatalf("summary=%+v", summary)
	}

	dump := fmt.Sprintf("%+v", logs.All())
	for _, forbidden := range []string{"secret-api-key", "sensitive-task-content", "请基于下面这份任务证据"} {
		if strings.Contains(dump, forbidden) {
			t.Fatalf("log leaked sensitive content %q in %s", forbidden, dump)
		}
	}
	if !strings.Contains(dump, "ai provider request started") || !strings.Contains(dump, "ai provider request finished") {
		t.Fatalf("missing request lifecycle logs: %s", dump)
	}
	if !strings.Contains(dump, "task_summary") || !strings.Contains(dump, "request_bytes") || !strings.Contains(dump, "duration_ms") {
		t.Fatalf("missing sanitized metadata: %s", dump)
	}
}

func TestAnthropicClientLogsSanitizedProviderFailure(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"type":    "rate_limit_error",
				"message": "too many requests",
			},
		})
	}))
	defer server.Close()

	client := NewAnthropicCompatibleClient(Config{
		Enabled: true,
		BaseURL: server.URL,
		APIKey:  "secret-api-key",
		Model:   "MiniMax-M3",
		Timeout: time.Second,
	}, zap.New(core))
	_, err := client.GenerateKPIAnalysis(context.Background(), map[string]string{
		"kpi": "sensitive-kpi-content",
	})
	if err == nil {
		t.Fatal("GenerateKPIAnalysis() expected error")
	}

	dump := fmt.Sprintf("%+v", logs.All())
	for _, forbidden := range []string{"secret-api-key", "sensitive-kpi-content", "请基于下面这份绩效证据"} {
		if strings.Contains(dump, forbidden) {
			t.Fatalf("log leaked sensitive content %q in %s", forbidden, dump)
		}
	}
	if !strings.Contains(dump, "ai provider request failed") ||
		!strings.Contains(dump, "kpi_analysis") ||
		!strings.Contains(dump, "non_2xx") ||
		!strings.Contains(dump, "rate_limit_error") {
		t.Fatalf("missing sanitized failure metadata: %s", dump)
	}
}

func writeAnthropicText(t *testing.T, w http.ResponseWriter, text string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"content": []map[string]string{{
			"type": "text",
			"text": text,
		}},
	}); err != nil {
		t.Fatalf("write response: %v", err)
	}
}
