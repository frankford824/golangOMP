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

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Scene       string
	System      string
	Messages    []ChatMessage
	MaxTokens   int
	Temperature float64
}

type ChatStreamResult struct {
	Provider     string
	Model        string
	InputTokens  int64
	OutputTokens int64
	FinishReason string
}

type ChatProvider interface {
	Ready() bool
	CompleteText(ctx context.Context, request ChatRequest) (string, ChatStreamResult, error)
	Stream(ctx context.Context, request ChatRequest, onDelta func(string) error) (ChatStreamResult, error)
}

type ProviderStreamError struct {
	Type    string
	Message string
}

func (e *ProviderStreamError) Error() string {
	return "ai provider stream error: " + strings.TrimSpace(e.Type+" "+e.Message)
}

func (c *AnthropicCompatibleClient) CompleteText(ctx context.Context, request ChatRequest) (string, ChatStreamResult, error) {
	if !c.Ready() {
		return "", ChatStreamResult{}, errors.New("ai chat provider is not configured")
	}
	body := c.chatRequestBody(request, false)
	raw, err := json.Marshal(body)
	if err != nil {
		return "", ChatStreamResult{}, fmt.Errorf("marshal ai chat request: %w", err)
	}
	responseBody, err := c.doMessagesRequest(ctx, normalizeScene(request.Scene), raw, len(request.System), body.MaxTokens)
	if err != nil {
		return "", ChatStreamResult{}, err
	}
	text, err := extractAnthropicText(responseBody)
	if err != nil {
		return "", ChatStreamResult{}, err
	}
	var envelope struct {
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(responseBody, &envelope)
	return text, ChatStreamResult{
		Provider: c.cfg.Provider, Model: firstNonEmpty(envelope.Model, c.cfg.Model),
		InputTokens: envelope.Usage.InputTokens, OutputTokens: envelope.Usage.OutputTokens,
		FinishReason: envelope.StopReason,
	}, nil
}

func (c *AnthropicCompatibleClient) Stream(ctx context.Context, request ChatRequest, onDelta func(string) error) (ChatStreamResult, error) {
	if !c.Ready() {
		return ChatStreamResult{}, errors.New("ai chat provider is not configured")
	}
	body := c.chatRequestBody(request, true)
	raw, err := json.Marshal(body)
	if err != nil {
		return ChatStreamResult{}, fmt.Errorf("marshal streaming ai request: %w", err)
	}
	fields := c.aiLogFields(normalizeScene(request.Scene), raw, len(request.System), body.MaxTokens)
	if err := c.reserveAICall(ctx, normalizeScene(request.Scene), fields); err != nil {
		return ChatStreamResult{}, err
	}
	startedAt := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, messagesURL(c.cfg.BaseURL), bytes.NewReader(raw))
	if err != nil {
		return ChatStreamResult{}, fmt.Errorf("build streaming ai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", c.cfg.APIKey)
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return ChatStreamResult{}, fmt.Errorf("call streaming ai provider: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return ChatStreamResult{}, fmt.Errorf("ai provider returned HTTP %d: %s", resp.StatusCode, providerErrorSummary(responseBody))
	}
	result := ChatStreamResult{Provider: c.cfg.Provider, Model: c.cfg.Model}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	eventName := ""
	dataLines := make([]string, 0, 2)
	stopped := false
	process := func() error {
		if len(dataLines) == 0 {
			eventName = ""
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("decode ai stream event %q: %w", eventName, err)
		}
		if event.Type == "" {
			event.Type = eventName
		}
		eventName = ""
		switch event.Type {
		case "message_start":
			result.Model = firstNonEmpty(event.Message.Model, result.Model)
			result.InputTokens = event.Message.Usage.InputTokens
		case "content_block_delta":
			if event.Delta.Type == "text_delta" && event.Delta.Text != "" && onDelta != nil {
				if err := onDelta(event.Delta.Text); err != nil {
					return err
				}
			}
		case "message_delta":
			result.OutputTokens = event.Usage.OutputTokens
			result.FinishReason = event.Delta.StopReason
		case "message_stop":
			stopped = true
		case "error":
			return &ProviderStreamError{Type: event.Error.Type, Message: event.Error.Message}
		case "ping", "content_block_start", "content_block_stop":
			// Valid protocol events without user-visible text.
		default:
			// Anthropic may add event types; forward compatibility requires ignore.
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := process(); err != nil {
				return result, err
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if len(dataLines) > 0 {
		if err := process(); err != nil {
			return result, err
		}
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("read ai stream: %w", err)
	}
	if !stopped {
		return result, io.ErrUnexpectedEOF
	}
	c.aiLogger().Info("ai provider stream finished", append(fields,
		zap.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		zap.Int64("input_tokens", result.InputTokens), zap.Int64("output_tokens", result.OutputTokens),
		zap.String("finish_reason", result.FinishReason))...)
	return result, nil
}

func (c *AnthropicCompatibleClient) chatRequestBody(request ChatRequest, stream bool) anthropicMessageRequest {
	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = c.cfg.MaxTokens
	}
	if maxTokens <= 0 {
		maxTokens = 1600
	}
	messages := make([]anthropicMessage, 0, len(request.Messages))
	for _, message := range request.Messages {
		role := strings.TrimSpace(message.Role)
		if role != "assistant" {
			role = "user"
		}
		messages = append(messages, anthropicMessage{Role: role, Content: message.Content})
	}
	return anthropicMessageRequest{
		Model: c.cfg.Model, MaxTokens: maxTokens, Temperature: request.Temperature,
		System: request.System, Thinking: disabledThinkingConfig(), Messages: messages, Stream: stream,
	}
}

type anthropicStreamEvent struct {
	Type    string `json:"type"`
	Message struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens int64 `json:"input_tokens"`
		} `json:"usage"`
	} `json:"message"`
	Delta struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func normalizeScene(scene string) string {
	if scene = strings.TrimSpace(scene); scene != "" {
		return scene
	}
	return "data_center_chat"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
