package aiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestParseBusinessTrendAnalysisTextFromFencedJSON(t *testing.T) {
	raw := "```json\n{\"headline\":\"毕业季物料升温\",\"overview\":\"内部任务集中在毕业季手举牌。\",\"internal_hotspots\":[{\"topic\":\"毕业季\",\"count\":3,\"signal\":\"任务集中\",\"keywords\":[\"毕业季\"],\"task_samples\":[\"RW-1 毕业手举牌\"]}],\"external_matches\":[],\"business_directions\":[],\"risks\":[],\"source_statuses\":[{\"source\":\"内部任务\",\"status\":\"used\",\"message\":\"已读取任务\",\"items\":3}],\"evidence_samples\":[],\"confidence\":\"medium\"}\n```"
	got, err := ParseBusinessTrendAnalysisText(raw)
	if err != nil {
		t.Fatalf("ParseBusinessTrendAnalysisText() error = %v", err)
	}
	if got.Headline != "毕业季物料升温" || len(got.InternalHotspots) != 1 {
		t.Fatalf("analysis=%+v", got)
	}
	if len(got.SourceStatuses) != 1 || got.SourceStatuses[0].Status != "used" {
		t.Fatalf("source statuses=%+v", got.SourceStatuses)
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
		assertThinkingDisabled(t, r)
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
		assertThinkingDisabled(t, r)
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

func TestAnthropicBusinessTrendAnalysisUsesTokenFloor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MaxTokens int                      `json:"max_tokens"`
			Thinking  *anthropicThinkingConfig `json:"thinking"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Thinking == nil || body.Thinking.Type != "disabled" {
			t.Fatalf("thinking config = %+v, want disabled", body.Thinking)
		}
		if body.MaxTokens < 1800 {
			t.Fatalf("max_tokens=%d, want at least 1800", body.MaxTokens)
		}
		writeAnthropicText(t, w, `{"headline":"毕业季物料升温","overview":"内部任务集中在毕业季手举牌。","internal_hotspots":[],"external_matches":[],"business_directions":[],"risks":[],"source_statuses":[],"evidence_samples":[],"confidence":"medium"}`)
	}))
	defer server.Close()

	client := NewAnthropicCompatibleClient(Config{
		Enabled:   true,
		BaseURL:   server.URL,
		APIKey:    "secret-api-key",
		Model:     "MiniMax-M3",
		Timeout:   time.Second,
		MaxTokens: 900,
	}, zap.NewNop())

	analysis, err := client.GenerateBusinessTrendAnalysis(context.Background(), map[string]string{"topic": "毕业季"})
	if err != nil {
		t.Fatalf("GenerateBusinessTrendAnalysis() error=%v", err)
	}
	if analysis.Headline != "毕业季物料升温" {
		t.Fatalf("analysis=%+v", analysis)
	}
}

func TestAnthropicClientRateLimitPreventsExtraProviderCalls(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		assertThinkingDisabled(t, r)
		writeAnthropicText(t, w, `{"decision":"任务正常","impact":"继续推进","actions":[],"evidence":[],"confidence":"high"}`)
	}))
	defer server.Close()

	client := NewAnthropicCompatibleClient(Config{
		Enabled:         true,
		BaseURL:         server.URL,
		APIKey:          "secret-api-key",
		Model:           "MiniMax-M3",
		Timeout:         time.Second,
		MaxTokens:       64,
		RateLimitWindow: 5 * time.Hour,
		RateLimitMax:    1,
	}, zap.NewNop())

	if _, err := client.GenerateTaskSummary(context.Background(), map[string]string{"task": "first"}); err != nil {
		t.Fatalf("first GenerateTaskSummary() error=%v", err)
	}
	if _, err := client.GenerateTaskSummary(context.Background(), map[string]string{"task": "second"}); err == nil {
		t.Fatal("second GenerateTaskSummary() expected rate limit error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
}

func TestAnthropicClientDistributedRateLimitPreventsProviderCall(t *testing.T) {
	var providerCalls int32
	limiter := &staticAIRateLimiter{
		reservation: AIRateLimitReservation{
			Allowed: false,
			Count:   800,
			ResetAt: time.Now().Add(time.Hour),
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&providerCalls, 1)
		writeAnthropicText(t, w, `{"decision":"任务正常","impact":"继续推进","actions":[],"evidence":[],"confidence":"high"}`)
	}))
	defer server.Close()

	client := NewAnthropicCompatibleClient(Config{
		Enabled:     true,
		BaseURL:     server.URL,
		APIKey:      "secret-api-key",
		Model:       "MiniMax-M3",
		Timeout:     time.Second,
		MaxTokens:   64,
		RateLimiter: limiter,
	}, zap.NewNop())

	if _, err := client.GenerateTaskSummary(context.Background(), map[string]string{"task": "blocked"}); err == nil {
		t.Fatal("GenerateTaskSummary() expected distributed rate limit error")
	}
	if got := atomic.LoadInt32(&limiter.calls); got != 1 {
		t.Fatalf("limiter calls = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&providerCalls); got != 0 {
		t.Fatalf("provider calls = %d, want 0", got)
	}
}

type staticAIRateLimiter struct {
	reservation AIRateLimitReservation
	err         error
	calls       int32
}

func (l *staticAIRateLimiter) Reserve(context.Context, string, time.Duration, int) (AIRateLimitReservation, error) {
	atomic.AddInt32(&l.calls, 1)
	return l.reservation, l.err
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

func assertThinkingDisabled(t *testing.T, r *http.Request) {
	t.Helper()
	var body struct {
		Thinking *anthropicThinkingConfig `json:"thinking"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if body.Thinking == nil || body.Thinking.Type != "disabled" {
		t.Fatalf("thinking config = %+v, want disabled", body.Thinking)
	}
}
