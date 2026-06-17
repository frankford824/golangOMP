package report_l1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	trendSourceChinaHot = "中文热点"
	trendSourceApify    = "Apify 免费热点"
)

type TrendProvider interface {
	Name() string
	Fetch(ctx context.Context, req TrendProviderRequest) (TrendProviderResult, error)
}

type TrendProviderRequest struct {
	Keywords []string `json:"keywords"`
	Limit    int      `json:"limit"`
}

type TrendProviderResult struct {
	Source string              `json:"source"`
	Items  []TrendExternalItem `json:"items"`
}

type TrendExternalItem struct {
	Source   string `json:"source"`
	Topic    string `json:"topic"`
	Title    string `json:"title"`
	Summary  string `json:"summary,omitempty"`
	URL      string `json:"url,omitempty"`
	HotValue string `json:"hot_value,omitempty"`
}

type TrendProviderConfig struct {
	ChinaHotURL         string
	ApifyToken          string
	ApifyBaseURL        string
	ApifyDouyinHotActor string
	ApifyDouyinActor    string
	ApifyRedNoteActor   string
	Apify1688Actor      string
	ApifyTaobaoActor    string
	Timeout             time.Duration
	MaxExternalKeywords int
	MaxExternalItems    int
}

func NewDefaultTrendProviders(cfg TrendProviderConfig, logger *zap.Logger) ([]TrendProvider, []string) {
	expected := []string{trendSourceChinaHot, trendSourceApify}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout}
	var providers []TrendProvider
	if strings.TrimSpace(cfg.ChinaHotURL) != "" {
		providers = append(providers, &httpTrendProvider{
			name:   trendSourceChinaHot,
			url:    strings.TrimSpace(cfg.ChinaHotURL),
			client: httpClient,
			logger: logger,
		})
	}
	if strings.TrimSpace(cfg.ApifyToken) != "" {
		providers = append(providers, &apifyTrendProvider{
			token:        strings.TrimSpace(cfg.ApifyToken),
			baseURL:      firstNonEmpty(cfg.ApifyBaseURL, "https://api.apify.com"),
			douyinHot:    firstNonEmpty(cfg.ApifyDouyinHotActor, "zen-studio/douyin-hot-search-scraper"),
			douyinSearch: firstNonEmpty(cfg.ApifyDouyinActor, "zen-studio/douyin-search-scraper"),
			redNote:      firstNonEmpty(cfg.ApifyRedNoteActor, "zen-studio/rednote-search-scraper"),
			ali1688:      firstNonEmpty(cfg.Apify1688Actor, "automation-lab/1688-scraper"),
			taobao:       firstNonEmpty(cfg.ApifyTaobaoActor, "zen-studio/taobao-detail-scraper"),
			client:       httpClient,
			logger:       logger,
		})
	}
	return providers, expected
}

type httpTrendProvider struct {
	name   string
	url    string
	client *http.Client
	logger *zap.Logger
}

func (p *httpTrendProvider) Name() string { return p.name }

func (p *httpTrendProvider) Fetch(ctx context.Context, req TrendProviderRequest) (TrendProviderResult, error) {
	if p.client == nil {
		p.client = &http.Client{Timeout: 20 * time.Second}
	}
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(body))
	if err != nil {
		return TrendProviderResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return TrendProviderResult{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return TrendProviderResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TrendProviderResult{}, fmt.Errorf("热点服务暂时不可用")
	}
	items := parseTrendItems(p.name, raw, req.Limit)
	return TrendProviderResult{Source: p.name, Items: items}, nil
}

type apifyTrendProvider struct {
	token        string
	baseURL      string
	douyinHot    string
	douyinSearch string
	redNote      string
	ali1688      string
	taobao       string
	client       *http.Client
	logger       *zap.Logger
}

func (p *apifyTrendProvider) Name() string { return trendSourceApify }

func (p *apifyTrendProvider) Fetch(ctx context.Context, req TrendProviderRequest) (TrendProviderResult, error) {
	if p.client == nil {
		p.client = &http.Client{Timeout: 20 * time.Second}
	}
	limit := req.Limit
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	actors := []struct {
		source string
		actor  string
		input  map[string]any
	}{
		{source: "抖音热榜", actor: p.douyinHot, input: map[string]any{"maxItems": min(limit, 50), "limit": min(limit, 50)}},
		{source: "抖音搜索", actor: p.douyinSearch, input: map[string]any{"keywords": req.Keywords, "maxItems": min(limit, 24), "limit": min(limit, 24)}},
		{source: "小红书搜索", actor: p.redNote, input: map[string]any{"keywords": req.Keywords, "maxItems": min(limit, 24), "limit": min(limit, 24)}},
		{source: "1688", actor: p.ali1688, input: map[string]any{"keywords": req.Keywords, "maxItems": min(limit, 12), "limit": min(limit, 12)}},
		{source: "淘宝", actor: p.taobao, input: map[string]any{"keywords": req.Keywords, "maxItems": min(limit, 12), "limit": min(limit, 12)}},
	}
	items := make([]TrendExternalItem, 0, limit)
	var failures []string
	for _, actor := range actors {
		if strings.TrimSpace(actor.actor) == "" {
			continue
		}
		got, err := p.runActor(ctx, actor.source, actor.actor, actor.input, max(1, limit-len(items)))
		if err != nil {
			failures = append(failures, actor.source)
			if p.logger != nil {
				p.logger.Warn("business trend apify source failed", zap.String("source", actor.source), zap.String("error", err.Error()))
			}
			continue
		}
		items = append(items, got...)
		if len(items) >= limit {
			break
		}
	}
	if len(items) == 0 && len(failures) > 0 {
		return TrendProviderResult{}, fmt.Errorf("免费热点源暂时不可用")
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return TrendProviderResult{Source: trendSourceApify, Items: items}, nil
}

func (p *apifyTrendProvider) runActor(ctx context.Context, source, actor string, input map[string]any, limit int) ([]TrendExternalItem, error) {
	endpoint := strings.TrimRight(p.baseURL, "/") + "/v2/acts/" + url.PathEscape(strings.ReplaceAll(actor, "/", "~")) + "/run-sync-get-dataset-items?token=" + url.QueryEscape(p.token)
	body, _ := json.Marshal(input)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s 返回异常", source)
	}
	items := parseTrendItems(source, raw, limit)
	if len(items) == 0 {
		return nil, fmt.Errorf("%s 暂无可用热点", source)
	}
	return items, nil
}

func parseTrendItems(source string, raw []byte, limit int) []TrendExternalItem {
	if limit <= 0 {
		limit = 20
	}
	var anyValue any
	if err := json.Unmarshal(raw, &anyValue); err != nil {
		return []TrendExternalItem{}
	}
	rows := flattenTrendRows(anyValue)
	out := make([]TrendExternalItem, 0, min(len(rows), limit))
	for _, row := range rows {
		if len(out) >= limit {
			break
		}
		title := firstMapString(row, "title", "name", "keyword", "word", "query", "content", "desc")
		if strings.TrimSpace(title) == "" {
			continue
		}
		summary := firstMapString(row, "summary", "description", "text", "note", "caption")
		out = append(out, TrendExternalItem{
			Source:   source,
			Topic:    title,
			Title:    title,
			Summary:  summary,
			URL:      firstMapString(row, "url", "link", "shareUrl", "share_url"),
			HotValue: firstMapString(row, "hot", "heat", "rank", "score", "value"),
		})
	}
	return out
}

func flattenTrendRows(value any) []map[string]any {
	switch typed := value.(type) {
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if row, ok := item.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	case map[string]any:
		for _, key := range []string{"items", "data", "result", "results", "records"} {
			if rows := flattenTrendRows(typed[key]); len(rows) > 0 {
				return rows
			}
		}
		return []map[string]any{typed}
	default:
		return []map[string]any{}
	}
}

func firstMapString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := row[key]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}
