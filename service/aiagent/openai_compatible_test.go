package aiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatibleClientCompleteText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("request path=%s authorization=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		var body openAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "test-model" || len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Stream {
			t.Fatalf("body = %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"content":"{\"tools\":[]}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":5}}`))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(Config{Enabled: true, Provider: "openai_compatible", BaseURL: server.URL + "/v1", APIKey: "test-key", Model: "test-model", HTTP: server.Client()}, nil)
	text, result, err := client.CompleteText(context.Background(), ChatRequest{Scene: "plan", System: "system", Messages: []ChatMessage{{Role: "user", Content: "question"}}, MaxTokens: 500})
	if err != nil || text != `{"tools":[]}` || result.Model != "served-model" || result.InputTokens != 12 || result.OutputTokens != 5 {
		t.Fatalf("text=%q result=%+v err=%v", text, result, err)
	}
}

func TestOpenAICompatibleClientStreamsChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, line := range []string{
			`data: {"model":"served-model","choices":[{"delta":{"content":"你好"},"finish_reason":""}]}`,
			`data: {"model":"served-model","choices":[{"delta":{"content":"，世界"},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		} {
			_, _ = fmt.Fprintln(w, line)
			_, _ = fmt.Fprintln(w)
			flusher.Flush()
		}
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(Config{Enabled: true, BaseURL: server.URL, APIKey: "test", Model: "model", HTTP: server.Client()}, nil)
	var output strings.Builder
	result, err := client.Stream(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hello"}}}, func(delta string) error {
		output.WriteString(delta)
		return nil
	})
	if err != nil || output.String() != "你好，世界" || result.Model != "served-model" || result.FinishReason != "stop" {
		t.Fatalf("output=%q result=%+v err=%v", output.String(), result, err)
	}
}

func TestOpenAIChatCompletionsURL(t *testing.T) {
	for input, want := range map[string]string{
		"https://api.example.com/v1":                  "https://api.example.com/v1/chat/completions",
		"https://api.example.com":                     "https://api.example.com/chat/completions",
		"https://api.example.com/v1/chat/completions": "https://api.example.com/v1/chat/completions",
	} {
		if got := openAIChatCompletionsURL(input); got != want {
			t.Fatalf("openAIChatCompletionsURL(%q)=%q want=%q", input, got, want)
		}
	}
}
