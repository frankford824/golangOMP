package aiagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SearchSemanticEvidence struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Text string `json:"text"`
}

type SearchSemanticTerms struct {
	Terms       []string  `json:"terms"`
	RawText     string    `json:"raw_text,omitempty"`
	GeneratedAt time.Time `json:"generated_at"`
	Model       string    `json:"model,omitempty"`
	Provider    string    `json:"provider,omitempty"`
}

func (c *AnthropicCompatibleClient) GenerateSearchSemanticTerms(ctx context.Context, evidence SearchSemanticEvidence) (*SearchSemanticTerms, error) {
	if !c.Ready() {
		return nil, errors.New("ai search semantic provider is not configured")
	}
	evidence.Text = truncateForError(evidence.Text, 1800)
	payload, err := json.Marshal(evidence)
	if err != nil {
		return nil, fmt.Errorf("marshal search semantic evidence: %w", err)
	}
	maxTokens := c.cfg.MaxTokens
	if maxTokens <= 0 || maxTokens > 300 {
		maxTokens = 300
	}
	body := anthropicMessageRequest{
		Model:       c.cfg.Model,
		MaxTokens:   maxTokens,
		Temperature: 0.1,
		System:      searchSemanticSystemPrompt,
		Thinking:    disabledThinkingConfig(),
		Messages: []anthropicMessage{{
			Role:    "user",
			Content: "请为下面这条运营系统搜索文档生成同义词、别名、品类归一化词条。证据 JSON：\n" + string(payload),
		}},
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal ai request: %w", err)
	}
	respBody, err := c.doMessagesRequest(ctx, "search_semantic_enrichment", rawBody, len(payload), maxTokens)
	if err != nil {
		return nil, err
	}
	text, err := extractAnthropicText(respBody)
	if err != nil {
		c.logAIResponseIssue("search_semantic_enrichment", "extract_text", len(respBody), err)
		return nil, err
	}
	terms, err := ParseSearchSemanticTermsText(text)
	if err != nil {
		c.logAIResponseIssue("search_semantic_enrichment", "parse_json", len(respBody), err)
		return nil, err
	}
	terms.RawText = strings.TrimSpace(text)
	terms.GeneratedAt = time.Now()
	terms.Model = c.cfg.Model
	terms.Provider = c.cfg.Provider
	return terms, nil
}

func ParseSearchSemanticTermsText(text string) (*SearchSemanticTerms, error) {
	candidate := extractJSONObject(text)
	if candidate == "" {
		return nil, errors.New("ai response does not contain a json object")
	}
	var terms SearchSemanticTerms
	if err := json.Unmarshal([]byte(candidate), &terms); err != nil {
		return nil, err
	}
	terms.Terms = normalizeSearchSemanticTerms(terms.Terms)
	if len(terms.Terms) == 0 {
		return nil, errors.New("ai response terms are empty")
	}
	return &terms, nil
}

func normalizeSearchSemanticTerms(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, min(len(values), 24))
	for _, value := range values {
		term := strings.TrimSpace(value)
		if term == "" {
			continue
		}
		term = truncateForError(term, 40)
		key := strings.ToLower(term)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, term)
		if len(out) >= 24 {
			break
		}
	}
	return out
}

const searchSemanticSystemPrompt = `你是运营系统搜索索引富化助手。请只基于输入文本生成可用于搜索召回的中文同义词、别名、简称、常见错写、品类归一化词。

要求：
1. 不要编造具体任务编号、SKU、人员姓名或系统中不存在的事实。
2. 不输出解释，不输出 Markdown。
3. terms 最多 24 个，每个词不超过 20 个中文字符。
4. 只输出一个 JSON 对象。

JSON 结构必须是：
{"terms":["词条1","词条2"]}`
