package aiagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// OpenAICompatibleClient implements the small ChatProvider surface used by the
// Data Center. It deliberately does not expose provider-native tool calls: the
// application planner produces a bounded allow-listed tool plan, and all facts
// still come from deterministic server-side read tools.
type OpenAICompatibleClient struct {
	cfg    Config
	http   *http.Client
	logger *zap.Logger
}

func NewOpenAICompatibleClient(cfg Config, logger *zap.Logger) *OpenAICompatibleClient {
	client := cfg.HTTP
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	if strings.TrimSpace(cfg.Provider) == "" {
		cfg.Provider = "openai_compatible"
	}
	return &OpenAICompatibleClient{cfg: cfg, http: client, logger: logger}
}

func (c *OpenAICompatibleClient) Ready() bool {
	return c != nil && c.cfg.Enabled && strings.TrimSpace(c.cfg.BaseURL) != "" &&
		strings.TrimSpace(c.cfg.APIKey) != "" && strings.TrimSpace(c.cfg.Model) != ""
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatRequest struct {
	Model          string              `json:"model"`
	Messages       []openAIChatMessage `json:"messages"`
	MaxTokens      int                 `json:"max_tokens,omitempty"`
	Temperature    float64             `json:"temperature,omitempty"`
	Stream         bool                `json:"stream"`
	EnableThinking *bool               `json:"enable_thinking,omitempty"`
}

type openAIChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

func (c *OpenAICompatibleClient) CompleteText(ctx context.Context, request ChatRequest) (string, ChatStreamResult, error) {
	if !c.Ready() {
		return "", ChatStreamResult{}, errors.New("ai chat provider is not configured")
	}
	body := c.requestBody(request, false)
	if err := c.reserve(ctx, normalizeScene(request.Scene)); err != nil {
		return "", ChatStreamResult{}, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", ChatStreamResult{}, fmt.Errorf("marshal openai-compatible chat request: %w", err)
	}
	startedAt := time.Now()
	responseBody, err := c.do(ctx, raw, "application/json")
	if err != nil {
		return "", ChatStreamResult{}, err
	}
	var response openAIChatResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return "", ChatStreamResult{}, fmt.Errorf("decode openai-compatible chat response: %w", err)
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return "", ChatStreamResult{}, errors.New("openai-compatible chat response contains no text")
	}
	c.logger.Info("openai-compatible provider request finished",
		zap.String("scene", normalizeScene(request.Scene)), zap.String("provider", c.cfg.Provider),
		zap.String("model", firstNonEmpty(response.Model, c.cfg.Model)), zap.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	return response.Choices[0].Message.Content, ChatStreamResult{
		Provider: c.cfg.Provider, Model: firstNonEmpty(response.Model, c.cfg.Model),
		InputTokens: response.Usage.PromptTokens, OutputTokens: response.Usage.CompletionTokens,
		FinishReason: response.Choices[0].FinishReason,
	}, nil
}

func (c *OpenAICompatibleClient) Stream(ctx context.Context, request ChatRequest, onDelta func(string) error) (ChatStreamResult, error) {
	if !c.Ready() {
		return ChatStreamResult{}, errors.New("ai chat provider is not configured")
	}
	body := c.requestBody(request, true)
	if err := c.reserve(ctx, normalizeScene(request.Scene)); err != nil {
		return ChatStreamResult{}, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return ChatStreamResult{}, fmt.Errorf("marshal streaming openai-compatible request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIChatCompletionsURL(c.cfg.BaseURL), bytes.NewReader(raw))
	if err != nil {
		return ChatStreamResult{}, fmt.Errorf("build streaming openai-compatible request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return ChatStreamResult{}, fmt.Errorf("call streaming openai-compatible provider: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return ChatStreamResult{}, fmt.Errorf("ai provider returned HTTP %d: %s", resp.StatusCode, providerErrorSummary(responseBody))
	}

	result := ChatStreamResult{Provider: c.cfg.Provider, Model: c.cfg.Model}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	seenDone := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			seenDone = true
			break
		}
		var event struct {
			Model   string `json:"model"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int64 `json:"prompt_tokens"`
				CompletionTokens int64 `json:"completion_tokens"`
			} `json:"usage,omitempty"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return result, fmt.Errorf("decode openai-compatible stream event: %w", err)
		}
		result.Model = firstNonEmpty(event.Model, result.Model)
		if event.Usage != nil {
			result.InputTokens = event.Usage.PromptTokens
			result.OutputTokens = event.Usage.CompletionTokens
		}
		for _, choice := range event.Choices {
			if choice.Delta.Content != "" && onDelta != nil {
				if err := onDelta(choice.Delta.Content); err != nil {
					return result, err
				}
			}
			if choice.FinishReason != "" {
				result.FinishReason = choice.FinishReason
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("read openai-compatible stream: %w", err)
	}
	if !seenDone && result.FinishReason == "" {
		return result, io.ErrUnexpectedEOF
	}
	return result, nil
}

func (c *OpenAICompatibleClient) requestBody(request ChatRequest, stream bool) openAIChatRequest {
	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = c.cfg.MaxTokens
	}
	if maxTokens <= 0 {
		maxTokens = 1600
	}
	messages := make([]openAIChatMessage, 0, len(request.Messages)+1)
	if strings.TrimSpace(request.System) != "" {
		messages = append(messages, openAIChatMessage{Role: "system", Content: request.System})
	}
	for _, message := range request.Messages {
		role := strings.TrimSpace(message.Role)
		if role != "assistant" {
			role = "user"
		}
		messages = append(messages, openAIChatMessage{Role: role, Content: message.Content})
	}
	body := openAIChatRequest{Model: c.cfg.Model, Messages: messages, MaxTokens: maxTokens, Temperature: request.Temperature, Stream: stream}
	if c.cfg.DisableThinking {
		disabled := false
		body.EnableThinking = &disabled
	}
	return body
}

func (c *OpenAICompatibleClient) do(ctx context.Context, raw []byte, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIChatCompletionsURL(c.cfg.BaseURL), bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("build openai-compatible request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call openai-compatible provider: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read openai-compatible response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai provider returned HTTP %d: %s", resp.StatusCode, providerErrorSummary(responseBody))
	}
	return responseBody, nil
}

func (c *OpenAICompatibleClient) reserve(ctx context.Context, scene string) error {
	if c.cfg.RateLimiter == nil {
		return nil
	}
	window := c.cfg.RateLimitWindow
	if window <= 0 {
		window = 5 * time.Hour
	}
	maxCalls := c.cfg.RateLimitMax
	if maxCalls <= 0 {
		maxCalls = 800
	}
	reservation, err := c.cfg.RateLimiter.Reserve(ctx, "ai-provider:"+c.cfg.Provider+":"+c.cfg.Model, window, maxCalls)
	if err != nil {
		return fmt.Errorf("ai provider rate limit check failed: %w", err)
	}
	if !reservation.Allowed {
		return fmt.Errorf("ai provider rate limit exceeded for %s", scene)
	}
	return nil
}

func openAIChatCompletionsURL(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

var _ ChatProvider = (*OpenAICompatibleClient)(nil)
