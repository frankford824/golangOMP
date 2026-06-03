package aiagent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Config struct {
	Enabled         bool
	Provider        string
	BaseURL         string
	APIKey          string
	Model           string
	Timeout         time.Duration
	MaxTokens       int
	RateLimitWindow time.Duration
	RateLimitMax    int
	RateLimiter     AIRateLimiter
	HTTP            *http.Client
}

type AIRateLimitReservation struct {
	Allowed bool
	Count   int
	ResetAt time.Time
}

type AIRateLimiter interface {
	Reserve(ctx context.Context, key string, window time.Duration, maxCalls int) (AIRateLimitReservation, error)
}

type AnthropicCompatibleClient struct {
	cfg             Config
	http            *http.Client
	logger          *zap.Logger
	rateMu          sync.Mutex
	rateWindowStart time.Time
	rateWindowCalls int
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
	if cfg.RateLimitWindow <= 0 {
		cfg.RateLimitWindow = 5 * time.Hour
	}
	if cfg.RateLimitMax <= 0 {
		cfg.RateLimitMax = 800
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
		Thinking:    disabledThinkingConfig(),
		Messages: []anthropicMessage{{
			Role:    "user",
			Content: "请基于下面这份任务证据，生成简短处置卡片。证据 JSON：\n" + string(payload),
		}},
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal ai request: %w", err)
	}

	respBody, err := c.doMessagesRequest(ctx, "task_summary", rawBody, len(payload), maxTokens)
	if err != nil {
		return nil, err
	}
	text, err := extractAnthropicText(respBody)
	if err != nil {
		c.logAIResponseIssue("task_summary", "extract_text", len(respBody), err)
		return nil, err
	}
	summary, err := ParseTaskSummaryText(text)
	if err != nil {
		c.logAIResponseIssue("task_summary", "parse_json", len(respBody), err)
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

func (c *AnthropicCompatibleClient) GenerateKPIAnalysis(ctx context.Context, evidence any) (*KPIAnalysis, error) {
	if !c.Ready() {
		return nil, errors.New("ai analysis provider is not configured")
	}
	payload, err := json.Marshal(evidence)
	if err != nil {
		return nil, fmt.Errorf("marshal kpi analysis evidence: %w", err)
	}
	maxTokens := c.cfg.MaxTokens
	if maxTokens < 1200 {
		maxTokens = 1200
	}
	body := anthropicMessageRequest{
		Model:       c.cfg.Model,
		MaxTokens:   maxTokens,
		Temperature: 0.2,
		System:      kpiAnalysisSystemPrompt,
		Thinking:    disabledThinkingConfig(),
		Messages: []anthropicMessage{{
			Role:    "user",
			Content: "请基于下面这份绩效证据，生成管理层可直接阅读的 KPI 分析。证据 JSON：\n" + string(payload),
		}},
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal ai request: %w", err)
	}

	respBody, err := c.doMessagesRequest(ctx, "kpi_analysis", rawBody, len(payload), maxTokens)
	if err != nil {
		return nil, err
	}
	text, err := extractAnthropicText(respBody)
	if err != nil {
		c.logAIResponseIssue("kpi_analysis", "extract_text", len(respBody), err)
		return nil, err
	}
	analysis, err := ParseKPIAnalysisText(text)
	if err != nil {
		c.logAIResponseIssue("kpi_analysis", "parse_json", len(respBody), err)
		analysis = &KPIAnalysis{
			Headline:   "AI 已返回内容，但未能结构化",
			Overview:   truncateForError(strings.TrimSpace(text), 320),
			RawText:    strings.TrimSpace(text),
			Confidence: "low",
		}
	}
	normalizeKPIAnalysis(analysis)
	analysis.GeneratedAt = time.Now()
	analysis.Model = c.cfg.Model
	analysis.Provider = c.cfg.Provider
	return analysis, nil
}

func (c *AnthropicCompatibleClient) doMessagesRequest(ctx context.Context, scene string, rawBody []byte, evidenceBytes, maxTokens int) ([]byte, error) {
	startedAt := time.Now()
	fields := c.aiLogFields(scene, rawBody, evidenceBytes, maxTokens)
	if err := c.reserveAICall(ctx, scene, fields); err != nil {
		return nil, err
	}
	c.aiLogger().Info("ai provider request started", fields...)

	reqCtx := ctx
	var cancel context.CancelFunc
	if c.cfg.Timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, messagesURL(c.cfg.BaseURL), bytes.NewReader(rawBody))
	if err != nil {
		c.logAIRequestFailure(scene, rawBody, evidenceBytes, maxTokens, startedAt, 0, 0, "build_request", err, "")
		return nil, fmt.Errorf("build ai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", c.cfg.APIKey)
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		c.logAIRequestFailure(scene, rawBody, evidenceBytes, maxTokens, startedAt, 0, 0, aiErrorKind(err), err, "")
		return nil, fmt.Errorf("call ai provider: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		c.logAIRequestFailure(scene, rawBody, evidenceBytes, maxTokens, startedAt, resp.StatusCode, 0, "read_response", err, "")
		return nil, fmt.Errorf("read ai response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logAIRequestFailure(scene, rawBody, evidenceBytes, maxTokens, startedAt, resp.StatusCode, len(respBody), "non_2xx", nil, providerErrorSummary(respBody))
		return nil, fmt.Errorf("ai provider returned HTTP %d: %s", resp.StatusCode, truncateForError(string(respBody), 800))
	}

	fields = append(fields,
		zap.Int("status", resp.StatusCode),
		zap.Int("response_bytes", len(respBody)),
		zap.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)
	c.aiLogger().Info("ai provider request finished", fields...)
	return respBody, nil
}

func (c *AnthropicCompatibleClient) reserveAICall(ctx context.Context, scene string, fields []zap.Field) error {
	if c == nil {
		return errors.New("ai provider client is nil")
	}
	window := c.cfg.RateLimitWindow
	if window <= 0 {
		window = 5 * time.Hour
	}
	maxCalls := c.cfg.RateLimitMax
	if maxCalls <= 0 {
		maxCalls = 800
	}
	if c.cfg.RateLimiter != nil {
		if ctx == nil {
			ctx = context.Background()
		}
		limitKey := c.rateLimitKey()
		reservation, err := c.cfg.RateLimiter.Reserve(ctx, limitKey, window, maxCalls)
		if err != nil {
			logFields := append([]zap.Field{}, fields...)
			logFields = append(logFields,
				zap.String("rate_limit_key", limitKey),
				zap.Int("rate_limit_max_calls", maxCalls),
				zap.Int64("rate_limit_window_ms", window.Milliseconds()),
				zap.String("error", truncateForError(err.Error(), 240)),
			)
			c.aiLogger().Warn("ai provider rate limit check failed", logFields...)
			return fmt.Errorf("ai provider rate limit check failed: %w", err)
		}
		if !reservation.Allowed {
			logFields := append([]zap.Field{}, fields...)
			logFields = append(logFields,
				zap.String("rate_limit_key", limitKey),
				zap.Int("rate_limit_max_calls", maxCalls),
				zap.Int("rate_limit_count", reservation.Count),
				zap.Int64("rate_limit_window_ms", window.Milliseconds()),
				zap.Time("rate_limit_reset_at", reservation.ResetAt),
			)
			c.aiLogger().Warn("ai provider rate limit exceeded", logFields...)
			return fmt.Errorf("ai provider rate limit exceeded: max %d calls per %s", maxCalls, window)
		}
		return nil
	}
	now := time.Now()
	c.rateMu.Lock()
	defer c.rateMu.Unlock()
	if c.rateWindowStart.IsZero() || now.Sub(c.rateWindowStart) >= window {
		c.rateWindowStart = now
		c.rateWindowCalls = 0
	}
	if c.rateWindowCalls >= maxCalls {
		resetAt := c.rateWindowStart.Add(window)
		logFields := append([]zap.Field{}, fields...)
		logFields = append(logFields,
			zap.Int("rate_limit_max_calls", maxCalls),
			zap.Int64("rate_limit_window_ms", window.Milliseconds()),
			zap.Time("rate_limit_reset_at", resetAt),
		)
		c.aiLogger().Warn("ai provider rate limit exceeded", logFields...)
		return fmt.Errorf("ai provider rate limit exceeded: max %d calls per %s", maxCalls, window)
	}
	c.rateWindowCalls++
	return nil
}

func (c *AnthropicCompatibleClient) rateLimitKey() string {
	if c == nil {
		return "unknown:unknown"
	}
	return normalizeRateLimitPart(c.cfg.Provider) + ":" + normalizeRateLimitPart(c.cfg.Model)
}

func normalizeRateLimitPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	lastWasDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
			lastWasDash = false
			continue
		}
		if !lastWasDash {
			b.WriteByte('-')
			lastWasDash = true
		}
	}
	normalized := strings.Trim(b.String(), "-")
	if normalized == "" {
		return "unknown"
	}
	return normalized
}

func (c *AnthropicCompatibleClient) aiLogFields(scene string, rawBody []byte, evidenceBytes, maxTokens int) []zap.Field {
	timeoutMS := int64(0)
	if c.cfg.Timeout > 0 {
		timeoutMS = c.cfg.Timeout.Milliseconds()
	}
	sum := sha256.Sum256(rawBody)
	return []zap.Field{
		zap.String("scene", scene),
		zap.String("provider", c.cfg.Provider),
		zap.String("model", c.cfg.Model),
		zap.String("endpoint", sanitizedEndpoint(messagesURL(c.cfg.BaseURL))),
		zap.Int("evidence_bytes", evidenceBytes),
		zap.Int("request_bytes", len(rawBody)),
		zap.String("request_sha256", fmt.Sprintf("%x", sum[:6])),
		zap.Int("max_tokens", maxTokens),
		zap.Int64("timeout_ms", timeoutMS),
	}
}

func (c *AnthropicCompatibleClient) logAIRequestFailure(scene string, rawBody []byte, evidenceBytes, maxTokens int, startedAt time.Time, status, responseBytes int, errorKind string, err error, providerError string) {
	fields := c.aiLogFields(scene, rawBody, evidenceBytes, maxTokens)
	fields = append(fields,
		zap.Int("status", status),
		zap.Int("response_bytes", responseBytes),
		zap.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		zap.String("error_kind", errorKind),
	)
	if err != nil {
		fields = append(fields, zap.String("error", truncateForError(err.Error(), 240)))
	}
	if providerError != "" {
		fields = append(fields, zap.String("provider_error", truncateForError(providerError, 240)))
	}
	c.aiLogger().Warn("ai provider request failed", fields...)
}

func (c *AnthropicCompatibleClient) logAIResponseIssue(scene, issue string, responseBytes int, err error) {
	fields := []zap.Field{
		zap.String("scene", scene),
		zap.String("provider", c.cfg.Provider),
		zap.String("model", c.cfg.Model),
		zap.Int("response_bytes", responseBytes),
		zap.String("error_kind", issue),
	}
	if err != nil {
		fields = append(fields, zap.String("error", truncateForError(err.Error(), 240)))
	}
	c.aiLogger().Warn("ai provider response not structured", fields...)
}

func (c *AnthropicCompatibleClient) aiLogger() *zap.Logger {
	if c != nil && c.logger != nil {
		return c.logger
	}
	return zap.NewNop()
}

func aiErrorKind(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "network_timeout"
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "timeout") || strings.Contains(text, "deadline exceeded") {
		return "timeout"
	}
	return "transport_error"
}

func providerErrorSummary(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "non-json error body omitted"
	}
	values := make([]string, 0, 4)
	if errObj, ok := envelope["error"].(map[string]any); ok {
		for _, key := range []string{"type", "code", "message"} {
			if value := strings.TrimSpace(fmt.Sprint(errObj[key])); value != "" && value != "<nil>" {
				values = append(values, key+"="+value)
			}
		}
	}
	for _, key := range []string{"type", "code", "message", "msg"} {
		if value := strings.TrimSpace(fmt.Sprint(envelope[key])); value != "" && value != "<nil>" {
			values = append(values, key+"="+value)
		}
	}
	if len(values) == 0 {
		return "json error body without standard message"
	}
	return strings.Join(values, "; ")
}

func sanitizedEndpoint(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

type anthropicMessageRequest struct {
	Model       string                   `json:"model"`
	MaxTokens   int                      `json:"max_tokens"`
	Temperature float64                  `json:"temperature,omitempty"`
	System      string                   `json:"system,omitempty"`
	Thinking    *anthropicThinkingConfig `json:"thinking,omitempty"`
	Messages    []anthropicMessage       `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicThinkingConfig struct {
	Type string `json:"type"`
}

func disabledThinkingConfig() *anthropicThinkingConfig {
	return &anthropicThinkingConfig{Type: "disabled"}
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

const kpiAnalysisSystemPrompt = `你是公司运营管理系统里的绩效分析助手。请只基于用户给出的证据，不要编造。

输出要求：
1. 使用普通管理者能看懂的中文，不输出接口名、JSON 字段名、英文错误码；技术细节只可放到 evidence。
2. 基础 KPI 数字必须尊重证据，不得自行改写或估算。
3. 重点回答：本周期业务量、人员效率、异常风险、下一步管理动作。
4. 人员姓名要使用证据中的真实姓名或显示姓名，不要用账号名替代真实姓名，除非证据没有姓名。
5. task_samples 必须优先体现“谁在几号创建了什么任务、设计何时提交了什么类型文件、审核何时处理”等链路。
6. 只输出一个 JSON 对象，不要 Markdown，不要代码块。
7. 严格控长：highlights 最多 5 条；people_insights 最多 8 条；task_samples 最多 6 条；risks 最多 5 条；actions 最多 5 条；evidence 最多 8 条。

JSON 结构必须是：
{
  "headline": "一句话总判断",
  "overview": "本周期绩效概览，2到4句话",
  "highlights": [{"title":"指标名称","value":"指标值","note":"业务解释"}],
  "people_insights": [{"role":"运营/设计/审核","name":"人员姓名","metric":"关键指标","signal":"表现或异常","action":"建议动作"}],
  "task_samples": [{"task_no":"任务编号","task_name":"任务名称","task_type":"任务类型","timeline":["时间 人员 动作"],"observation":"链路观察"}],
  "risks": [{"level":"high|medium|low","title":"风险点","reason":"原因"}],
  "actions": [{"owner":"责任角色或人员","action":"建议动作","timing":"现在/本周/持续观察"}],
  "evidence": ["关键证据，不超过8条"],
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

func ParseKPIAnalysisText(text string) (*KPIAnalysis, error) {
	candidate := extractJSONObject(text)
	if candidate == "" {
		return nil, errors.New("ai response does not contain a json object")
	}
	var analysis KPIAnalysis
	if err := json.Unmarshal([]byte(candidate), &analysis); err != nil {
		return nil, err
	}
	normalizeKPIAnalysis(&analysis)
	return &analysis, nil
}

func normalizeKPIAnalysis(analysis *KPIAnalysis) {
	if analysis == nil {
		return
	}
	if analysis.Highlights == nil {
		analysis.Highlights = []KPIAnalysisHighlight{}
	}
	if analysis.PeopleInsights == nil {
		analysis.PeopleInsights = []KPIAnalysisPersonInsight{}
	}
	if analysis.TaskSamples == nil {
		analysis.TaskSamples = []KPIAnalysisTaskSample{}
	}
	if analysis.Risks == nil {
		analysis.Risks = []KPIAnalysisRisk{}
	}
	if analysis.Actions == nil {
		analysis.Actions = []KPIAnalysisAction{}
	}
	if analysis.Evidence == nil {
		analysis.Evidence = []string{}
	}
	if strings.TrimSpace(analysis.Headline) == "" {
		analysis.Headline = "本周期绩效分析已生成"
	}
	if strings.TrimSpace(analysis.Overview) == "" {
		analysis.Overview = analysis.Headline
	}
	if len(analysis.Highlights) > 5 {
		analysis.Highlights = analysis.Highlights[:5]
	}
	if len(analysis.PeopleInsights) > 8 {
		analysis.PeopleInsights = analysis.PeopleInsights[:8]
	}
	if len(analysis.TaskSamples) > 6 {
		analysis.TaskSamples = analysis.TaskSamples[:6]
	}
	if len(analysis.Risks) > 5 {
		analysis.Risks = analysis.Risks[:5]
	}
	if len(analysis.Actions) > 5 {
		analysis.Actions = analysis.Actions[:5]
	}
	if len(analysis.Evidence) > 8 {
		analysis.Evidence = analysis.Evidence[:8]
	}
	analysis.Headline = truncateForError(analysis.Headline, 180)
	analysis.Overview = truncateForError(analysis.Overview, 520)
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
