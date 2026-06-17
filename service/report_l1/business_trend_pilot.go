package report_l1

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/aiagent"
)

const CodeBusinessTrendNotConfigured = "business_trend_not_configured"

type BusinessTrendAnalysisGenerator interface {
	GenerateBusinessTrendAnalysis(ctx context.Context, evidence any) (*aiagent.BusinessTrendAnalysis, error)
}

type BusinessTrendAnalysisParams struct {
	From    time.Time
	To      time.Time
	Mode    string
	Sources []string
}

type businessTrendEvidence struct {
	Period         kpiAnalysisPeriod                   `json:"period"`
	Mode           string                              `json:"mode"`
	Internal       businessTrendInternalEvidence       `json:"internal"`
	ExternalItems  []TrendExternalItem                 `json:"external_items"`
	SourceStatuses []aiagent.BusinessTrendSourceStatus `json:"source_statuses"`
	GeneratedAt    time.Time                           `json:"generated_at"`
}

type businessTrendInternalEvidence struct {
	TotalTasks int                             `json:"total_tasks"`
	Keywords   []string                        `json:"keywords"`
	Hotspots   []aiagent.BusinessTrendHotspot  `json:"hotspots"`
	Samples    []aiagent.BusinessTrendEvidence `json:"samples"`
}

type trendTopicAccumulator struct {
	topic    string
	count    int
	keywords map[string]struct{}
	samples  []string
}

func (s *Service) BusinessTrendPilotAnalysis(ctx context.Context, actor domain.RequestActor, params BusinessTrendAnalysisParams) (*aiagent.BusinessTrendAnalysis, *domain.AppError) {
	if err := s.requireSuperAdmin(ctx, actor, "/v1/reports/business-trends/pilot-analysis"); err != nil {
		return nil, err
	}
	if appErr := s.validateBusinessTrendAnalysisParams(params); appErr != nil {
		return nil, appErr
	}
	if s.businessTrendRepo == nil {
		return nil, domain.NewAppError(CodeBusinessTrendNotConfigured, "业务热点分析服务尚未配置", nil)
	}

	evidence, err := s.collectBusinessTrendEvidence(ctx, params)
	return fallbackBusinessTrendAnalysis(evidence, err), nil
}

func (s *Service) validateBusinessTrendAnalysisParams(params BusinessTrendAnalysisParams) *domain.AppError {
	if params.From.IsZero() || params.To.IsZero() || params.From.After(params.To) {
		return domain.NewAppError(CodeInvalidDateRange, "from must be before or equal to to", nil)
	}
	return nil
}

func (s *Service) collectBusinessTrendEvidence(ctx context.Context, params BusinessTrendAnalysisParams) (businessTrendEvidence, error) {
	tasks, err := s.businessTrendRepo.ListRecentTaskTexts(ctx, repo.BusinessTrendFilter{
		From:           params.From,
		To:             params.To.AddDate(0, 0, 1),
		Limit:          240,
		BatchItemLimit: 600,
	})
	if err != nil {
		evidence := businessTrendEvidence{
			Period: kpiAnalysisPeriod{
				From: params.From.Format("2006-01-02"),
				To:   params.To.Format("2006-01-02"),
			},
			Mode: normalizeBusinessTrendMode(params.Mode),
			Internal: businessTrendInternalEvidence{
				TotalTasks: 0,
				Keywords:   []string{},
				Hotspots:   []aiagent.BusinessTrendHotspot{},
				Samples:    []aiagent.BusinessTrendEvidence{},
			},
			ExternalItems: []TrendExternalItem{},
			SourceStatuses: []aiagent.BusinessTrendSourceStatus{
				{Source: "内部任务", Status: "failed", Message: "内部任务暂时不可读，本次无法生成完整判断"},
			},
			GeneratedAt: time.Now().UTC(),
		}
		return evidence, err
	}

	internal := buildBusinessTrendInternalEvidence(tasks)
	externalItems, externalStatuses := s.fetchBusinessTrendExternal(ctx, params.Mode, params.Sources, internal.Keywords)
	statuses := append([]aiagent.BusinessTrendSourceStatus{{
		Source:  "内部任务",
		Status:  "used",
		Message: fmt.Sprintf("已读取 %d 条近期任务", len(tasks)),
		Items:   len(tasks),
	}}, externalStatuses...)
	evidence := businessTrendEvidence{
		Period: kpiAnalysisPeriod{
			From: params.From.Format("2006-01-02"),
			To:   params.To.Format("2006-01-02"),
		},
		Mode:           normalizeBusinessTrendMode(params.Mode),
		Internal:       internal,
		ExternalItems:  externalItems,
		SourceStatuses: statuses,
		GeneratedAt:    time.Now().UTC(),
	}
	return evidence, nil
}

func (s *Service) fetchBusinessTrendExternal(ctx context.Context, mode string, requestedSources []string, keywords []string) ([]TrendExternalItem, []aiagent.BusinessTrendSourceStatus) {
	expectedNames := s.businessTrendProviderNames
	if len(expectedNames) == 0 {
		expectedNames = []string{trendSourceChinaHot, trendSourceApify}
	}
	if normalizeBusinessTrendMode(mode) == "internal" {
		statuses := make([]aiagent.BusinessTrendSourceStatus, 0, len(expectedNames))
		for _, name := range expectedNames {
			statuses = append(statuses, aiagent.BusinessTrendSourceStatus{Source: name, Status: "skipped", Message: "本次选择仅分析内部任务"})
		}
		return []TrendExternalItem{}, statuses
	}

	allowed := func(name string) bool {
		if len(requestedSources) == 0 {
			return true
		}
		for _, source := range requestedSources {
			if strings.EqualFold(strings.TrimSpace(source), name) {
				return true
			}
		}
		return false
	}

	configured := make(map[string]TrendProvider, len(s.businessTrendProviders))
	for _, provider := range s.businessTrendProviders {
		if provider == nil || !allowed(provider.Name()) {
			continue
		}
		configured[provider.Name()] = provider
	}
	items := []TrendExternalItem{}
	statuses := []aiagent.BusinessTrendSourceStatus{}
	for _, name := range expectedNames {
		if !allowed(name) {
			statuses = append(statuses, aiagent.BusinessTrendSourceStatus{Source: name, Status: "skipped", Message: "本次未选择该热点来源"})
			continue
		}
		provider := configured[name]
		if provider == nil {
			statuses = append(statuses, aiagent.BusinessTrendSourceStatus{Source: name, Status: "skipped", Message: "暂未启用，本次基于内部任务判断"})
			continue
		}
		result, err := provider.Fetch(ctx, TrendProviderRequest{Keywords: keywords, Limit: 24})
		if err != nil {
			statuses = append(statuses, aiagent.BusinessTrendSourceStatus{Source: name, Status: "failed", Message: "热点来源暂时不可用，本次不影响内部分析"})
			continue
		}
		for _, item := range result.Items {
			if item.Source == "" {
				item.Source = name
			}
			items = append(items, item)
		}
		statuses = append(statuses, aiagent.BusinessTrendSourceStatus{Source: name, Status: "used", Message: "已纳入外部热点样本", Items: len(result.Items)})
	}
	if len(statuses) == 0 {
		statuses = append(statuses, aiagent.BusinessTrendSourceStatus{Source: "外部热点", Status: "skipped", Message: "暂未启用，本次基于内部任务判断"})
	}
	return items, statuses
}

func buildBusinessTrendInternalEvidence(tasks []domain.BusinessTrendTaskText) businessTrendInternalEvidence {
	hotspots := extractBusinessTrendHotspots(tasks)
	keywords := make([]string, 0, min(len(hotspots), 12))
	for _, hotspot := range hotspots {
		if strings.TrimSpace(hotspot.Topic) != "" {
			keywords = append(keywords, hotspot.Topic)
		}
	}
	return businessTrendInternalEvidence{
		TotalTasks: len(tasks),
		Keywords:   keywords,
		Hotspots:   hotspots,
		Samples:    buildBusinessTrendEvidenceSamples(tasks, 8),
	}
}

func extractBusinessTrendHotspots(tasks []domain.BusinessTrendTaskText) []aiagent.BusinessTrendHotspot {
	accs := map[string]*trendTopicAccumulator{}
	for _, task := range tasks {
		terms := businessTrendTaskTerms(task)
		seen := map[string]struct{}{}
		for _, term := range terms {
			topic := normalizeBusinessTrendTerm(term)
			if topic == "" {
				continue
			}
			if _, ok := seen[topic]; ok {
				continue
			}
			seen[topic] = struct{}{}
			acc := accs[topic]
			if acc == nil {
				acc = &trendTopicAccumulator{topic: topic, keywords: map[string]struct{}{}}
				accs[topic] = acc
			}
			acc.count++
			acc.keywords[topic] = struct{}{}
			if len(acc.samples) < 3 {
				acc.samples = append(acc.samples, businessTrendTaskSampleLine(task))
			}
		}
	}
	if len(accs) == 0 && len(tasks) > 0 {
		accs["其他任务需求"] = &trendTopicAccumulator{
			topic:    "其他任务需求",
			count:    len(tasks),
			keywords: map[string]struct{}{"其他任务需求": {}},
			samples:  firstBusinessTrendTaskSamples(tasks, 3),
		}
	}
	hotspots := make([]aiagent.BusinessTrendHotspot, 0, len(accs))
	for _, acc := range accs {
		keywords := make([]string, 0, len(acc.keywords))
		for keyword := range acc.keywords {
			keywords = append(keywords, keyword)
		}
		sort.Strings(keywords)
		hotspots = append(hotspots, aiagent.BusinessTrendHotspot{
			Topic:       acc.topic,
			Count:       acc.count,
			Signal:      fmt.Sprintf("近 %d 条任务提到该方向", acc.count),
			Keywords:    keywords,
			TaskSamples: acc.samples,
		})
	}
	sort.SliceStable(hotspots, func(i, j int) bool {
		if hotspots[i].Count != hotspots[j].Count {
			return hotspots[i].Count > hotspots[j].Count
		}
		return hotspots[i].Topic < hotspots[j].Topic
	})
	if len(hotspots) > 8 {
		hotspots = hotspots[:8]
	}
	return hotspots
}

func businessTrendTaskTerms(task domain.BusinessTrendTaskText) []string {
	var terms []string
	for _, value := range []string{task.ProductShortName, task.ProductName, task.CategoryName, task.Material, task.SizeText, task.CraftText} {
		terms = appendTermFragments(terms, value)
	}
	for _, item := range task.BatchItems {
		for _, value := range []string{item.ProductShortName, item.ProductName, item.CategoryCode, item.MaterialMode, item.DesignRequirement} {
			terms = appendTermFragments(terms, value)
		}
	}
	text := businessTrendTaskText(task)
	for _, keyword := range businessTrendLexicon {
		if strings.Contains(text, keyword) {
			terms = append(terms, keyword)
		}
	}
	return terms
}

func businessTrendTaskText(task domain.BusinessTrendTaskText) string {
	parts := []string{
		task.ProductName,
		task.ProductShortName,
		task.CategoryName,
		task.DemandText,
		task.CopyText,
		task.Remark,
		task.ChangeRequest,
		task.DesignRequirement,
		task.Material,
		task.SizeText,
		task.CraftText,
	}
	for _, item := range task.BatchItems {
		parts = append(parts, item.ProductName, item.ProductShortName, item.CategoryCode, item.DesignRequirement, item.MaterialMode)
	}
	return strings.Join(parts, " ")
}

func appendTermFragments(out []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return out
	}
	separators := func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', ',', '，', '、', '/', '\\', '|', ';', '；', ':', '：', '(', ')', '（', '）', '[', ']', '【', '】', '+', '＋':
			return true
		default:
			return false
		}
	}
	for _, part := range strings.FieldsFunc(value, separators) {
		if normalized := normalizeBusinessTrendTerm(part); normalized != "" {
			out = append(out, normalized)
		}
	}
	if normalized := normalizeBusinessTrendTerm(value); normalized != "" {
		out = append(out, normalized)
	}
	return out
}

func normalizeBusinessTrendTerm(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n-_—.。!！?？,，、:：;；/\\|")
	if value == "" {
		return ""
	}
	aliases := map[string]string{
		"生日": "生日场景", "宝宝宴": "宝宝宴", "满月": "满月场景", "百日宴": "百日宴",
		"婚礼": "婚庆场景", "婚庆": "婚庆场景", "毕业": "毕业季", "毕业季": "毕业季",
		"开业": "开业活动", "周年": "周年庆", "端午": "端午节", "父亲节": "父亲节",
		"母亲节": "母亲节", "七夕": "七夕", "中秋": "中秋节", "国庆": "国庆",
		"暑期": "暑期活动", "儿童节": "儿童节", "高考": "升学季", "中考": "升学季",
		"KT板": "KT板", "kt板": "KT板", "手举牌": "手举牌", "立牌": "立牌",
		"易拉宝": "易拉宝", "横幅": "横幅", "海报": "海报", "贴纸": "贴纸",
		"写真布": "写真布", "喷绘": "喷绘", "背景墙": "背景墙", "亚克力": "亚克力",
	}
	if alias := aliases[value]; alias != "" {
		value = alias
	}
	runes := []rune(value)
	if len(runes) < 2 || len(runes) > 24 {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "rw-") || strings.HasPrefix(lower, "sku") {
		return ""
	}
	for _, blocked := range []string{"无", "暂无", "其他", "默认", "常规", "新款", "批量", "产品", "图片", "设计", "任务", "需求"} {
		if value == blocked {
			return ""
		}
	}
	if len(runes) > 18 {
		value = string(runes[:18])
	}
	return value
}

var businessTrendLexicon = []string{
	"生日", "宝宝宴", "满月", "百日宴", "婚礼", "婚庆", "毕业", "开业", "周年",
	"端午", "父亲节", "母亲节", "七夕", "中秋", "国庆", "暑期", "儿童节", "高考", "中考",
	"露营", "旅游", "宠物", "IP", "联名", "国潮", "收纳",
	"KT板", "kt板", "手举牌", "立牌", "易拉宝", "横幅", "海报", "贴纸", "写真布", "喷绘", "门头", "背景墙", "亚克力", "帆布袋",
}

func buildBusinessTrendEvidenceSamples(tasks []domain.BusinessTrendTaskText, limit int) []aiagent.BusinessTrendEvidence {
	if limit <= 0 {
		limit = 8
	}
	samples := make([]aiagent.BusinessTrendEvidence, 0, min(len(tasks), limit))
	for _, task := range tasks {
		if len(samples) >= limit {
			break
		}
		note := firstNonEmpty(task.Remark, task.DesignRequirement, task.DemandText, task.CopyText, task.ProductShortName, task.ProductName)
		if note == "" {
			note = "近期任务样本"
		}
		samples = append(samples, aiagent.BusinessTrendEvidence{
			TaskNo:    task.TaskNo,
			TaskName:  firstNonEmpty(task.ProductShortName, task.ProductName),
			Source:    "内部任务",
			Note:      truncateBusinessText(note, 80),
			CreatedAt: task.CreatedAt.Format("2006-01-02"),
		})
	}
	return samples
}

func fallbackBusinessTrendAnalysis(evidence businessTrendEvidence, cause error) *aiagent.BusinessTrendAnalysis {
	headline := "近期业务热点已基于内部任务生成"
	if evidence.Internal.TotalTasks == 0 {
		headline = "本周期暂无足够任务数据"
	}
	if len(evidence.ExternalItems) > 0 {
		headline = "近期业务热点已结合外部样本生成"
	}
	overview := fmt.Sprintf("%s 至 %s：共读取 %d 条任务，当前重点关注 %s。",
		evidence.Period.From,
		evidence.Period.To,
		evidence.Internal.TotalTasks,
		businessTrendTopicSummary(evidence.Internal.Hotspots),
	)
	if evidence.Internal.TotalTasks == 0 {
		overview = "当前时间范围内暂无可分析任务，建议放宽时间范围后再生成。"
	}
	if len(evidence.ExternalItems) == 0 {
		overview += " 外部热点未启用或暂不可用，本次主要基于内部任务判断。"
	}
	directions := fallbackBusinessDirections(evidence.Internal.Hotspots)
	risks := fallbackBusinessRisks(evidence)
	if cause != nil {
		risks = append(risks, aiagent.BusinessTrendRisk{Level: "low", Title: "AI 深度解读暂时不可用", Reason: "已切换为系统规则摘要，不影响查看内部热点"})
	}
	return &aiagent.BusinessTrendAnalysis{
		Headline:           headline,
		Overview:           overview,
		InternalHotspots:   evidence.Internal.Hotspots,
		ExternalMatches:    fallbackExternalMatches(evidence.ExternalItems),
		BusinessDirections: directions,
		Risks:              risks,
		SourceStatuses:     evidence.SourceStatuses,
		EvidenceSamples:    evidence.Internal.Samples,
		Confidence:         fallbackBusinessConfidence(evidence),
		GeneratedAt:        time.Now().UTC(),
		Model:              "system",
		Provider:           "system_fallback",
	}
}

func mergeBusinessTrendFallbackContent(analysis *aiagent.BusinessTrendAnalysis, evidence businessTrendEvidence) {
	if len(analysis.InternalHotspots) == 0 {
		analysis.InternalHotspots = evidence.Internal.Hotspots
	}
	if len(analysis.SourceStatuses) == 0 {
		analysis.SourceStatuses = evidence.SourceStatuses
	}
	if len(analysis.EvidenceSamples) == 0 {
		analysis.EvidenceSamples = evidence.Internal.Samples
	}
	if len(analysis.ExternalMatches) == 0 {
		analysis.ExternalMatches = fallbackExternalMatches(evidence.ExternalItems)
	}
	if len(analysis.BusinessDirections) == 0 {
		analysis.BusinessDirections = fallbackBusinessDirections(evidence.Internal.Hotspots)
	}
}

func fallbackBusinessDirections(hotspots []aiagent.BusinessTrendHotspot) []aiagent.BusinessTrendDirection {
	if len(hotspots) == 0 {
		return []aiagent.BusinessTrendDirection{}
	}
	out := make([]aiagent.BusinessTrendDirection, 0, min(len(hotspots), 4))
	for _, hotspot := range hotspots {
		if len(out) >= 4 {
			break
		}
		out = append(out, aiagent.BusinessTrendDirection{
			Title:           hotspot.Topic,
			Reason:          hotspot.Signal,
			SuggestedAction: "优先复盘近期任务样本，整理可复用款式、素材和报价口径",
			Priority:        "medium",
		})
	}
	return out
}

func fallbackBusinessRisks(evidence businessTrendEvidence) []aiagent.BusinessTrendRisk {
	risks := []aiagent.BusinessTrendRisk{}
	if evidence.Internal.TotalTasks < 5 {
		risks = append(risks, aiagent.BusinessTrendRisk{Level: "medium", Title: "内部样本偏少", Reason: "本周期任务量较少，趋势判断需要继续观察"})
	}
	for _, status := range evidence.SourceStatuses {
		if status.Source == "内部任务" && status.Status == "failed" {
			risks = append(risks, aiagent.BusinessTrendRisk{Level: "high", Title: "内部任务暂时不可读", Reason: "本次无法读取近期任务，请稍后重试或联系管理员查看服务状态"})
			break
		}
	}
	if len(evidence.ExternalItems) == 0 {
		risks = append(risks, aiagent.BusinessTrendRisk{Level: "low", Title: "外部热点样本不足", Reason: "外部来源未启用或暂不可用，本次主要参考内部任务"})
	}
	return risks
}

func fallbackExternalMatches(items []TrendExternalItem) []aiagent.BusinessTrendMatch {
	out := make([]aiagent.BusinessTrendMatch, 0, min(len(items), 6))
	for _, item := range items {
		if len(out) >= 6 {
			break
		}
		out = append(out, aiagent.BusinessTrendMatch{
			Topic:           firstNonEmpty(item.Topic, item.Title),
			Source:          item.Source,
			Signal:          firstNonEmpty(item.Summary, item.Title),
			BusinessMeaning: "可作为内部任务选题的外部参考",
			Evidence:        []string{firstNonEmpty(item.Title, item.Topic)},
		})
	}
	return out
}

func businessTrendTopicSummary(hotspots []aiagent.BusinessTrendHotspot) string {
	if len(hotspots) == 0 {
		return "暂无明显集中方向"
	}
	names := make([]string, 0, min(len(hotspots), 3))
	for _, hotspot := range hotspots {
		if strings.TrimSpace(hotspot.Topic) != "" {
			names = append(names, hotspot.Topic)
		}
		if len(names) >= 3 {
			break
		}
	}
	if len(names) == 0 {
		return "暂无明显集中方向"
	}
	return strings.Join(names, "、")
}

func fallbackBusinessConfidence(evidence businessTrendEvidence) string {
	if evidence.Internal.TotalTasks >= 20 && len(evidence.ExternalItems) > 0 {
		return "high"
	}
	if evidence.Internal.TotalTasks >= 5 {
		return "medium"
	}
	return "low"
}

func businessTrendTaskSampleLine(task domain.BusinessTrendTaskText) string {
	name := firstNonEmpty(task.ProductShortName, task.ProductName, task.CategoryName, "未命名任务")
	return strings.TrimSpace(task.TaskNo + " " + truncateBusinessText(name, 34))
}

func firstBusinessTrendTaskSamples(tasks []domain.BusinessTrendTaskText, limit int) []string {
	out := make([]string, 0, min(len(tasks), limit))
	for _, task := range tasks {
		if len(out) >= limit {
			break
		}
		out = append(out, businessTrendTaskSampleLine(task))
	}
	return out
}

func truncateBusinessText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func normalizeBusinessTrendMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "external", "mixed", "internal_external", "internal+external":
		return "external"
	default:
		return "internal"
	}
}
