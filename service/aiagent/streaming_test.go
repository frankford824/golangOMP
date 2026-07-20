package aiagent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAnthropicCompatibleClientStreamParsesTextAndIgnoresUnknownEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("Accept=%q", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-test\",\"usage\":{\"input_tokens\":7}}}\n\n"))
		_, _ = w.Write([]byte("event: ping\ndata: {\"type\":\"ping\"}\n\n"))
		_, _ = w.Write([]byte("event: future_event\ndata: {\"type\":\"future_event\",\"value\":1}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"你好\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"，世界\"}}\n\n"))
		_, _ = w.Write([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n\n"))
		_, _ = w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()
	client := NewAnthropicCompatibleClient(Config{Enabled: true, BaseURL: server.URL, APIKey: "test", Model: "fallback", HTTP: server.Client()}, nil)
	var text strings.Builder
	result, err := client.Stream(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "测试"}}}, func(delta string) error {
		text.WriteString(delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if text.String() != "你好，世界" || result.Model != "claude-test" || result.InputTokens != 7 || result.OutputTokens != 5 || result.FinishReason != "end_turn" {
		t.Fatalf("text=%q result=%+v", text.String(), result)
	}
}

func TestAnthropicCompatibleClientStreamReturnsInStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"busy\"}}\n\n"))
	}))
	defer server.Close()
	client := NewAnthropicCompatibleClient(Config{Enabled: true, BaseURL: server.URL, APIKey: "test", Model: "m", HTTP: server.Client()}, nil)
	_, err := client.Stream(context.Background(), ChatRequest{}, nil)
	var streamErr *ProviderStreamError
	if !errors.As(err, &streamErr) || streamErr.Type != "overloaded_error" {
		t.Fatalf("err=%v", err)
	}
}

func TestAnthropicCompatibleClientStreamHonorsCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	client := NewAnthropicCompatibleClient(Config{Enabled: true, BaseURL: server.URL, APIKey: "test", Model: "m", HTTP: server.Client()}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := client.Stream(ctx, ChatRequest{}, nil)
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		close(release)
		if err == nil || (!errors.Is(err, context.Canceled) && !strings.Contains(strings.ToLower(err.Error()), "canceled")) {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("stream did not cancel")
	}
}
