package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/aiagent"
	analyticssvc "workflow/service/analytics"
)

const maxAnalysisTools = 3

type AnalysisPlan struct {
	Tools []AnalysisToolCall `json:"tools"`
}

type AnalysisToolCall struct {
	Name       string                 `json:"name"`
	Query      string                 `json:"query,omitempty"`
	EntityID   string                 `json:"entity_id,omitempty"`
	From       string                 `json:"from,omitempty"`
	To         string                 `json:"to,omitempty"`
	MetricID   string                 `json:"metric_id,omitempty"`
	GroupBy    string                 `json:"group_by,omitempty"`
	EntityType string                 `json:"entity_type,omitempty"`
	Days       int                    `json:"days,omitempty"`
	Limit      int                    `json:"limit,omitempty"`
	Arguments  map[string]interface{} `json:"arguments,omitempty"`
}

type ToolOrchestrator struct {
	provider  aiagent.ChatProvider
	retriever EvidenceRetriever
	analytics repo.AIAnalysisRepo
	registry  *analyticssvc.Service
}

func (o *ToolOrchestrator) SetAnalyticsTools(registry *analyticssvc.Service) {
	if o != nil {
		o.registry = registry
	}
}

func NewToolOrchestrator(provider aiagent.ChatProvider, retriever EvidenceRetriever, analytics ...repo.AIAnalysisRepo) *ToolOrchestrator {
	o := &ToolOrchestrator{provider: provider, retriever: retriever}
	if len(analytics) > 0 {
		o.analytics = analytics[0]
	}
	return o
}

func (o *ToolOrchestrator) Gather(ctx context.Context, actor domain.RequestActor, question string, history []domain.AIMessage, limit int) ([]domain.AIRetrievalHit, domain.AIRetrievalMeta, error) {
	if o == nil || o.retriever == nil {
		return nil, domain.AIRetrievalMeta{}, fmt.Errorf("analysis retrieval is unavailable")
	}
	planningQuestion := buildAnalysisPlanningQuestion(question, history, 8, 4000)
	plan, err := o.plan(ctx, planningQuestion)
	if err != nil || len(plan.Tools) == 0 {
		hits, meta, searchErr := o.retriever.Search(ctx, actor, planningQuestion, limit)
		if searchErr == nil {
			meta.Reason = "planner_fallback"
		}
		return hits, meta, searchErr
	}
	type result struct {
		hits []domain.AIRetrievalHit
		meta domain.AIRetrievalMeta
		err  error
	}
	results := make([]result, len(plan.Tools))
	var wg sync.WaitGroup
	for index := range plan.Tools {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			call := plan.Tools[index]
			query := strings.TrimSpace(call.Query)
			if query == "" {
				query = planningQuestion
			}
			hits, meta, searchErr := o.execute(ctx, actor, call, query, planningQuestion, max(limit, 20))
			results[index] = result{hits: hits, meta: meta, err: searchErr}
		}()
	}
	wg.Wait()
	merged := make(map[string]domain.AIRetrievalHit)
	meta := domain.AIRetrievalMeta{Mode: "hybrid"}
	succeeded := 0
	for _, item := range results {
		if item.err != nil {
			meta.Degraded = true
			meta.Reason = "tool_partial_failure"
			continue
		}
		succeeded++
		meta.Candidates += item.meta.Candidates
		meta.Degraded = meta.Degraded || item.meta.Degraded
		for _, hit := range item.hits {
			if existing, exists := merged[hit.DocumentID]; !exists || hit.Score > existing.Score {
				merged[hit.DocumentID] = hit
			}
		}
	}
	if succeeded == 0 {
		hits, fallbackMeta, fallbackErr := o.retriever.Search(ctx, actor, planningQuestion, limit)
		if fallbackErr != nil {
			return nil, meta, fallbackErr
		}
		fallbackMeta.Degraded = true
		fallbackMeta.Reason = "tool_fallback"
		return hits, fallbackMeta, nil
	}
	hits := make([]domain.AIRetrievalHit, 0, len(merged))
	for _, hit := range merged {
		hits = append(hits, hit)
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, meta, nil
}

func (o *ToolOrchestrator) execute(ctx context.Context, actor domain.RequestActor, call AnalysisToolCall, query, originalQuestion string, limit int) ([]domain.AIRetrievalHit, domain.AIRetrievalMeta, error) {
	if isUnifiedAnalyticsTool(call.Name) {
		if o.registry == nil {
			return nil, domain.AIRetrievalMeta{}, fmt.Errorf("analytics tool registry is unavailable")
		}
		arguments := analyticsToolArguments(call, originalQuestion)
		output, appErr := o.registry.Call(ctx, actor, call.Name, arguments)
		if appErr != nil {
			return nil, domain.AIRetrievalMeta{}, fmt.Errorf("analytics tool %s: %s", call.Name, appErr.Message)
		}
		return output.Hits, domain.AIRetrievalMeta{Mode: "exact", Candidates: len(output.Hits)}, nil
	}
	if call.Name == "task_detail" || call.Name == "resource_group_detail" {
		if strings.TrimSpace(call.EntityID) == "" {
			hits, meta, err := o.retriever.Search(ctx, actor, query, limit)
			if err == nil {
				hits = filterToolHits(call.Name, "", hits)
			}
			return hits, meta, err
		}
		if o.analytics == nil {
			return nil, domain.AIRetrievalMeta{}, fmt.Errorf("analysis data source is unavailable")
		}
		entityID, err := strconv.ParseInt(strings.TrimSpace(call.EntityID), 10, 64)
		if err != nil || entityID <= 0 {
			return nil, domain.AIRetrievalMeta{}, fmt.Errorf("analysis tool %q entity_id is invalid", call.Name)
		}
		permission := domain.PermissionTaskView
		if call.Name == "resource_group_detail" {
			permission = domain.PermissionAssetView
		}
		if !domain.ActorHasPermission(actor, permission) {
			return []domain.AIRetrievalHit{}, domain.AIRetrievalMeta{Mode: "exact", Reason: "scope_denied"}, nil
		}
		access := domain.ResourceGroupAccessFilterForActor(actor, permission)
		var hits []domain.AIRetrievalHit
		if call.Name == "task_detail" {
			hits, err = o.analytics.GetTaskDetailEvidence(ctx, access, entityID)
		} else {
			hits, err = o.analytics.GetResourceGroupDetailEvidence(ctx, access, entityID)
		}
		return hits, domain.AIRetrievalMeta{Mode: "exact", Candidates: len(hits)}, err
	}
	if call.Name == "task_kpi" || call.Name == "business_trends" || call.Name == "experience_summary" {
		if o.analytics == nil {
			return nil, domain.AIRetrievalMeta{}, fmt.Errorf("analysis data source is unavailable")
		}
		access := domain.ResourceGroupAccessFilterForActor(actor, domain.PermissionTaskView)
		if !domain.ActorHasPermission(actor, domain.PermissionTaskView) {
			return []domain.AIRetrievalHit{}, domain.AIRetrievalMeta{Mode: "exact", Reason: "task_scope_denied"}, nil
		}
		from, to := analysisDateRange(call, originalQuestion, time.Now().UTC())
		var hits []domain.AIRetrievalHit
		var err error
		switch call.Name {
		case "task_kpi":
			if o.registry != nil {
				output, appErr := o.registry.Call(ctx, actor, "query_metric", map[string]interface{}{
					"metric_id": "design_productivity", "from": from.Format("2006-01-02"),
					"to": to.Add(-time.Nanosecond).Format("2006-01-02"), "limit": min(limit, 20),
				})
				if appErr != nil {
					return nil, domain.AIRetrievalMeta{}, fmt.Errorf("analytics compatibility tool: %s", appErr.Message)
				}
				hits = output.Hits
			} else {
				hits, err = o.analytics.ListKPIEvidence(ctx, access, from, to, limit)
			}
		case "business_trends":
			hits, err = o.analytics.ListBusinessTrendEvidence(ctx, access, from, to, limit)
		case "experience_summary":
			hits, err = o.analytics.ListExperienceEvidence(ctx, access, from, to, limit)
		}
		return hits, domain.AIRetrievalMeta{Mode: "exact", Candidates: len(hits)}, err
	}
	hits, meta, err := o.retriever.Search(ctx, actor, query, limit)
	if err == nil {
		hits = filterToolHits(call.Name, call.EntityID, hits)
	}
	return hits, meta, err
}

var relativeAnalysisDaysPattern = regexp.MustCompile(`(?i)(?:最近|过去|近|last)\s*(\d{1,3}|[一二两三四五六七八九十百]{1,5})\s*(?:天|days?)`)

func analysisDateRange(call AnalysisToolCall, question string, now time.Time) (time.Time, time.Time) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	// Relative ranges in the user's original question are authoritative. The
	// model planner may hallucinate stale absolute years; never let those fields
	// override an explicit "最近七天/最近7天" request.
	if match := relativeAnalysisDaysPattern.FindStringSubmatch(strings.TrimSpace(question)); len(match) == 2 {
		if days := parseRelativeAnalysisDays(match[1]); days >= 1 && days <= 366 {
			return today.AddDate(0, 0, -days+1), today.AddDate(0, 0, 1)
		}
	}
	if call.From != "" && call.To != "" {
		from, _ := time.ParseInLocation("2006-01-02", call.From, location)
		to, _ := time.ParseInLocation("2006-01-02", call.To, location)
		return from, to.AddDate(0, 0, 1)
	}
	return today.AddDate(0, 0, -29), today.AddDate(0, 0, 1)
}

func parseRelativeAnalysisDays(value string) int {
	value = strings.TrimSpace(value)
	if days, err := strconv.Atoi(value); err == nil {
		return days
	}
	digits := map[rune]int{'一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	total, current := 0, 0
	for _, char := range value {
		switch char {
		case '十':
			if current == 0 {
				current = 1
			}
			total += current * 10
			current = 0
		case '百':
			if current == 0 {
				current = 1
			}
			total += current * 100
			current = 0
		default:
			digit, ok := digits[char]
			if !ok {
				return 0
			}
			current = digit
		}
	}
	return total + current
}

func (o *ToolOrchestrator) plan(ctx context.Context, question string) (AnalysisPlan, error) {
	if o.provider == nil || !o.provider.Ready() {
		return AnalysisPlan{}, fmt.Errorf("analysis planner is unavailable")
	}
	systemPrompt := `你是只读数据分析规划器。仅返回 JSON，不要解释。格式：{"tools":[{"name":"query_metric","arguments":{"metric_id":"design_productivity","days":7}}]}。
最多 3 个工具。优先使用统一分析工具。还允许实体和文本检索：global_search、task_detail、resource_group_detail、business_trends、experience_summary。task_kpi 仅作旧客户端兼容，不应在新计划中使用。
所有日期和筛选参数必须放在 arguments，不得写进 query 文本。禁止 SQL、写入、上传、发布或改变状态。`
	if o.registry != nil {
		systemPrompt += "\n" + o.registry.PlannerInstructions()
	}
	text, _, err := o.provider.CompleteText(ctx, aiagent.ChatRequest{
		Scene:     "data_center_tool_plan",
		System:    systemPrompt,
		Messages:  []aiagent.ChatMessage{{Role: "user", Content: truncateRunes(question, 4000)}},
		MaxTokens: 500, Temperature: 0,
	})
	if err != nil {
		return AnalysisPlan{}, err
	}
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	var plan AnalysisPlan
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &plan); err != nil {
		return AnalysisPlan{}, fmt.Errorf("decode analysis plan: %w", err)
	}
	if err := validateAnalysisPlan(&plan); err != nil {
		return AnalysisPlan{}, err
	}
	return plan, nil
}

func validateAnalysisPlan(plan *AnalysisPlan) error {
	if plan == nil || len(plan.Tools) == 0 || len(plan.Tools) > maxAnalysisTools {
		return fmt.Errorf("analysis plan must contain 1-%d tools", maxAnalysisTools)
	}
	allowed := map[string]bool{
		"global_search": true, "task_detail": true, "resource_group_detail": true,
		"task_kpi": true, "business_trends": true, "experience_summary": true,
		"list_metrics": true, "describe_metric": true, "query_metric": true,
		"query_timeseries": true, "query_distribution": true, "trace_entity": true,
	}
	for index := range plan.Tools {
		call := &plan.Tools[index]
		call.Name = strings.TrimSpace(call.Name)
		call.Query = truncateRunes(strings.TrimSpace(call.Query), 1000)
		call.EntityID = truncateRunes(strings.TrimSpace(call.EntityID), 128)
		call.MetricID = truncateRunes(strings.TrimSpace(call.MetricID), 128)
		call.GroupBy = truncateRunes(strings.TrimSpace(call.GroupBy), 64)
		call.EntityType = truncateRunes(strings.TrimSpace(call.EntityType), 32)
		if !allowed[call.Name] {
			return fmt.Errorf("analysis tool %q is not allowed", call.Name)
		}
		if call.Name == "task_detail" || call.Name == "resource_group_detail" {
			if call.EntityID == "" && call.Query == "" {
				return fmt.Errorf("analysis tool %q requires entity_id or query", call.Name)
			}
		}
		if call.From != "" || call.To != "" {
			from, fromErr := time.Parse("2006-01-02", call.From)
			to, toErr := time.Parse("2006-01-02", call.To)
			if fromErr != nil || toErr != nil || to.Before(from) || to.Sub(from) > 366*24*time.Hour {
				return fmt.Errorf("analysis tool date range is invalid")
			}
		}
	}
	return nil
}

func isUnifiedAnalyticsTool(name string) bool {
	switch name {
	case "list_metrics", "describe_metric", "query_metric", "query_timeseries", "query_distribution", "trace_entity":
		return true
	default:
		return false
	}
}

func analyticsToolArguments(call AnalysisToolCall, question string) map[string]interface{} {
	arguments := make(map[string]interface{}, len(call.Arguments)+8)
	for key, value := range call.Arguments {
		arguments[key] = value
	}
	if _, ok := arguments["metric_id"]; !ok && call.MetricID != "" {
		arguments["metric_id"] = call.MetricID
	}
	if _, ok := arguments["group_by"]; !ok && call.GroupBy != "" {
		arguments["group_by"] = call.GroupBy
	}
	if _, ok := arguments["entity_type"]; !ok && call.EntityType != "" {
		arguments["entity_type"] = call.EntityType
	}
	if _, ok := arguments["entity_id"]; !ok && call.EntityID != "" {
		arguments["entity_id"] = call.EntityID
	}
	if _, ok := arguments["from"]; !ok && call.From != "" {
		arguments["from"] = call.From
	}
	if _, ok := arguments["to"]; !ok && call.To != "" {
		arguments["to"] = call.To
	}
	if _, ok := arguments["days"]; !ok && call.Days > 0 {
		arguments["days"] = call.Days
	}
	if _, ok := arguments["limit"]; !ok && call.Limit > 0 {
		arguments["limit"] = call.Limit
	}
	// The user's relative date expression is authoritative. This also carries
	// an earlier turn such as "最近七天" into a short follow-up like "具体人员".
	if match := relativeAnalysisDaysPattern.FindStringSubmatch(strings.TrimSpace(question)); len(match) == 2 {
		if days := parseRelativeAnalysisDays(match[1]); days >= 1 && days <= 366 {
			arguments["days"] = days
		}
	}
	arguments["_question"] = truncateRunes(strings.TrimSpace(question), 4000)
	return arguments
}

func buildAnalysisPlanningQuestion(current string, history []domain.AIMessage, maxUserTurns, maxChars int) string {
	current = strings.TrimSpace(current)
	if maxUserTurns <= 0 {
		maxUserTurns = 8
	}
	userTurns := make([]string, 0, maxUserTurns)
	for index := len(history) - 1; index >= 0 && len(userTurns) < maxUserTurns; index-- {
		message := history[index]
		content := strings.TrimSpace(message.Content)
		if message.Role != domain.AIMessageRoleUser || content == "" || content == current {
			continue
		}
		userTurns = append(userTurns, content)
	}
	for left, right := 0, len(userTurns)-1; left < right; left, right = left+1, right-1 {
		userTurns[left], userTurns[right] = userTurns[right], userTurns[left]
	}
	if len(userTurns) == 0 {
		return truncateRunes(current, maxChars)
	}
	var builder strings.Builder
	builder.WriteString("对话中的用户查询上下文（旧到新，仅用于补全当前只读查询）：\n")
	for _, turn := range userTurns {
		builder.WriteString("- ")
		builder.WriteString(turn)
		builder.WriteByte('\n')
	}
	builder.WriteString("当前问题：")
	builder.WriteString(current)
	return truncateRunes(builder.String(), maxChars)
}

func filterToolHits(tool, entityID string, hits []domain.AIRetrievalHit) []domain.AIRetrievalHit {
	wanted := ""
	switch tool {
	case "task_detail":
		wanted = "task"
	case "resource_group_detail":
		wanted = "task_resource_group"
	case "business_trends":
		wanted = "business_trend"
	case "experience_summary":
		wanted = "experience_summary"
	}
	out := make([]domain.AIRetrievalHit, 0, len(hits))
	for _, hit := range hits {
		if wanted != "" && hit.EntityType != wanted {
			continue
		}
		if strings.TrimSpace(entityID) != "" && hit.EntityID != strings.TrimSpace(entityID) {
			continue
		}
		out = append(out, hit)
	}
	return out
}
