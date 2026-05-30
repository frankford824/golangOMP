package aiagent

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

	"go.uber.org/zap"
)

type Config struct {
	Enabled   bool
	Provider  string
	BaseURL   string
	APIKey    string
	Model     string
	Timeout   time.Duration
	MaxTokens int
	HTTP      *http.Client
}

type AnthropicCompatibleClient struct {
	cfg    Config
	http   *http.Client
	logger *zap.Logger
}

func NewAnthropicCompatibleClient(cfg Config, logger *zap.Logger) *AnthropicCompatibleClient {
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
	if cfg.Provider == "" {
		cfg.Provider = "anthropic_compatible"
	}
	return &AnthropicCompatibleClient{cfg: cfg, http: client, logger: logger}
}

func (c *AnthropicCompatibleClient) Ready() bool {
	if c == nil {
		return false
	}
	return c.cfg.Enabled &&
		strings.TrimSpace(c.cfg.BaseURL) != "" &&
		strings.TrimSpace(c.cfg.APIKey) != "" &&
		strings.TrimSpace(c.cfg.Model) != ""
}

func (c *AnthropicCompatibleClient) GenerateTaskSummary(ctx context.Context, evidence any) (*TaskSummary, error) {
	if !c.Ready() {
		return nil, errors.New("ai summary provider is not configured")
	}
	payload, err := json.Marshal(evidence)
	if err != nil {
		return nil, fmt.Errorf("marshal task summary evidence: %w", err)
	}
	maxTokens := c.cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 900
	}
	body := anthropicMessageRequest{
		Model:       c.cfg.Model,
		MaxTokens:   maxTokens,
		Temperature: 0.2,
		System:      taskSummarySystemPrompt,
		Messages: []anthropicMessage{{
			Role:    "user",
			Content: "请基于下面这份任务证据，生成简短处置卡片。证据 JSON：\n" + string(payload),
		}},
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal ai request: %w", err)
	}

	reqCtx := ctx
	var cancel context.CancelFunc
	if c.cfg.Timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, messagesURL(c.cfg.BaseURL), bytes.NewReader(rawBody))
	if err != nil {
		return nil, fmt.Errorf("build ai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", c.cfg.APIKey)
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ai provider: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read ai response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.Warn("ai provider returned non-2xx", zap.Int("status", resp.StatusCode), zap.String("provider", c.cfg.Provider))
		return nil, fmt.Errorf("ai provider returned HTTP %d: %s", resp.StatusCode, truncateForError(string(respBody), 800))
	}

	text, err := extractAnthropicText(respBody)
	if err != nil {
		return nil, err
	}
	summary, err := ParseTaskSummaryText(text)
	if err != nil {
		summary = &TaskSummary{
			Decision:      "AI 已返回内容，但未能结构化",
			Impact:        truncateForError(strings.TrimSpace(text), 240),
			Headline:      "AI 已返回内容，但未能结构化",
			CurrentStatus: truncateForError(strings.TrimSpace(text), 240),
			RawText:       strings.TrimSpace(text),
			Confidence:    "low",
		}
	}
	normalizeTaskSummary(summary)
	summary.GeneratedAt = time.Now()
	summary.Model = c.cfg.Model
	summary.Provider = c.cfg.Provider
	return summary, nil
}

type anthropicMessageRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

const taskSummarySystemPrompt = `你是公司运营管理系统里的任务处置助手。请只基于用户给出的证据，不要编造。

输出要求：
1. 使用普通业务用户能看懂的中文，不输出接口名、JSON字段名、英文错误码；错误码只可放到 evidence。
2. 只回答：现在卡在哪、影响什么、谁下一步做什么。
3. 如果环节未开始，写“未进入该环节”，不要写“系统暂无记录”。
4. 只输出一个 JSON 对象，不要 Markdown，不要代码块。
5. 严格控长：decision 不超过 45 个中文字符；impact 不超过 60 个中文字符；actions 最多 3 条；evidence 最多 4 条。

JSON 结构必须是：
{
  "decision": "一句话处置判断",
  "impact": "对当前用户和流程的影响",
  "primary_blocker": {"title":"主卡点","owner":"责任方","reason":"业务原因"},
  "actions": [{"role":"责任角色","action":"下一步动作","timing":"现在/处理后/等待"}],
  "evidence": ["关键证据，不超过4条"],
  "confidence": "high|medium|low"
}`

func messagesURL(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(base, "/v1/messages") {
		return base
	}
	return base + "/v1/messages"
}

func extractAnthropicText(raw []byte) (string, error) {
	var envelope struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", fmt.Errorf("decode ai response: %w", err)
	}
	if len(envelope.Content) == 0 {
		return "", errors.New("ai response content is empty")
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(envelope.Content, &blocks); err == nil {
		parts := make([]string, 0, len(blocks))
		for _, block := range blocks {
			if block.Type == "" || block.Type == "text" {
				if text := strings.TrimSpace(block.Text); text != "" {
					parts = append(parts, text)
				}
			}
		}
		if joined := strings.TrimSpace(strings.Join(parts, "\n")); joined != "" {
			return joined, nil
		}
	}
	var text string
	if err := json.Unmarshal(envelope.Content, &text); err == nil && strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text), nil
	}
	return "", errors.New("ai response text is empty")
}

func ParseTaskSummaryText(text string) (*TaskSummary, error) {
	candidate := extractJSONObject(text)
	if candidate == "" {
		return nil, errors.New("ai response does not contain a json object")
	}
	var summary TaskSummary
	if err := json.Unmarshal([]byte(candidate), &summary); err != nil {
		return nil, err
	}
	if summary.People == nil {
		summary.People = []TaskSummaryPerson{}
	}
	if summary.Timeline == nil {
		summary.Timeline = []TaskSummaryTimelineItem{}
	}
	if summary.StuckPoints == nil {
		summary.StuckPoints = []TaskSummaryStuckPoint{}
	}
	if summary.SkuAssetERPCost == nil {
		summary.SkuAssetERPCost = []TaskSummarySkuAssetCost{}
	}
	if summary.NextActions == nil {
		summary.NextActions = []string{}
	}
	if summary.Actions == nil {
		summary.Actions = []TaskSummaryAction{}
	}
	if summary.Evidence == nil {
		summary.Evidence = []string{}
	}
	normalizeTaskSummary(&summary)
	return &summary, nil
}

func normalizeTaskSummary(summary *TaskSummary) {
	if summary == nil {
		return
	}
	if strings.TrimSpace(summary.Decision) == "" {
		summary.Decision = strings.TrimSpace(summary.Headline)
	}
	if strings.TrimSpace(summary.Impact) == "" {
		summary.Impact = strings.TrimSpace(summary.CurrentStatus)
	}
	if strings.TrimSpace(summary.Headline) == "" {
		summary.Headline = strings.TrimSpace(summary.Decision)
	}
	if strings.TrimSpace(summary.CurrentStatus) == "" {
		summary.CurrentStatus = strings.TrimSpace(summary.Impact)
	}
	if summary.PrimaryBlocker == nil && len(summary.StuckPoints) > 0 {
		point := summary.StuckPoints[0]
		summary.PrimaryBlocker = &TaskSummaryBlocker{
			Title:  point.Title,
			Owner:  point.Owner,
			Reason: point.Reason,
		}
	}
	if len(summary.Actions) == 0 && len(summary.NextActions) > 0 {
		limit := min(len(summary.NextActions), 3)
		for _, action := range summary.NextActions[:limit] {
			if strings.TrimSpace(action) == "" {
				continue
			}
			summary.Actions = append(summary.Actions, TaskSummaryAction{Role: "相关责任人", Action: strings.TrimSpace(action)})
		}
	}
	if len(summary.Evidence) == 0 && len(summary.SkuAssetERPCost) > 0 {
		limit := min(len(summary.SkuAssetERPCost), 4)
		for _, item := range summary.SkuAssetERPCost[:limit] {
			line := strings.TrimSpace(strings.Join([]string{item.SKU, item.ERPStatus, item.CostStatus, item.AssetStatus}, " "))
			if line != "" {
				summary.Evidence = append(summary.Evidence, line)
			}
		}
	}
	if len(summary.Actions) > 3 {
		summary.Actions = summary.Actions[:3]
	}
	if len(summary.Evidence) > 4 {
		summary.Evidence = summary.Evidence[:4]
	}
	summary.Decision = truncateForError(summary.Decision, 180)
	summary.Impact = truncateForError(summary.Impact, 240)
}

func extractJSONObject(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 3 {
			trimmed = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return ""
	}
	return strings.TrimSpace(trimmed[start : end+1])
}

func truncateForError(s string, limit int) string {
	s = strings.TrimSpace(s)
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
