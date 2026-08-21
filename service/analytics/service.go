package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const ProtocolVersion = "2025-11-25"

type ToolDefinition struct {
	Name        string                 `json:"name"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type ToolOutput struct {
	Text       string
	Structured interface{}
	Hits       []domain.AIRetrievalHit
}

type Service struct {
	repo      repo.AnalyticsRepo
	legacy    repo.AIAnalysisRepo
	metrics   map[string]domain.AnalyticsMetricDefinition
	toolIndex map[string]ToolDefinition
	tools     []ToolDefinition
}

func NewService(repository repo.AnalyticsRepo, legacy repo.AIAnalysisRepo) *Service {
	metrics := defaultMetricCatalog()
	tools := defaultTools()
	toolIndex := make(map[string]ToolDefinition, len(tools))
	for _, tool := range tools {
		toolIndex[tool.Name] = tool
	}
	return &Service{repo: repository, legacy: legacy, metrics: metrics, tools: tools, toolIndex: toolIndex}
}

func (s *Service) Tools(actor domain.RequestActor) ([]ToolDefinition, *domain.AppError) {
	if appErr := requireAnalyticsAccess(actor, false); appErr != nil {
		return nil, appErr
	}
	return append([]ToolDefinition(nil), s.tools...), nil
}

// PlannerInstructions is the versioned analytics skill supplied to the model.
// It is generated from the same catalog used by MCP tools/list so prompt
// guidance and executable metrics cannot drift independently.
func (s *Service) PlannerInstructions() string {
	if s == nil {
		return ""
	}
	lines := []string{
		"统一分析工具：list_metrics、describe_metric、query_metric、query_timeseries、query_distribution、trace_entity。",
		"query工具参数放在 arguments；日期使用 days 或 from/to；禁止SQL。可用指标：",
	}
	for _, metric := range s.metricList() {
		lines = append(lines, fmt.Sprintf("- %s：%s；允许分组=%s", metric.ID, metric.Description, strings.Join(metric.AllowedGroupBys, ",")))
	}
	return strings.Join(lines, "\n")
}

func (s *Service) Call(ctx context.Context, actor domain.RequestActor, name string, arguments map[string]interface{}) (*ToolOutput, *domain.AppError) {
	name = strings.TrimSpace(name)
	if _, ok := s.toolIndex[name]; !ok {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "analytics tool not found", map[string]interface{}{"name": name})
	}
	if appErr := requireAnalyticsAccess(actor, name != "list_metrics" && name != "describe_metric"); appErr != nil {
		return nil, appErr
	}
	switch name {
	case "list_metrics":
		metrics := s.metricList()
		output := structuredOutput(metrics)
		output.Hits = []domain.AIRetrievalHit{{DocumentID: "analytics:metric-catalog", EntityType: "analytics_catalog", EntityID: "metrics", Title: "数据中心指标目录", InternalRoute: "/data-center", Excerpt: output.Text, SourceVersion: fmt.Sprint(len(metrics)), Score: 1, Source: "analytics_registry"}}
		return output, nil
	case "describe_metric":
		metricID := argumentString(arguments, "metric_id")
		definition, ok := s.metrics[metricID]
		if !ok {
			return nil, invalidArgument("metric_id", "unknown analytics metric")
		}
		output := structuredOutput(definition)
		output.Hits = []domain.AIRetrievalHit{{DocumentID: "analytics:metric:" + definition.ID, EntityType: "analytics_catalog", EntityID: definition.ID, Title: definition.Name, InternalRoute: "/data-center", Excerpt: output.Text, SourceVersion: definition.ID, Score: 1, Source: "analytics_registry"}}
		return output, nil
	case "query_metric", "query_timeseries", "query_distribution":
		return s.queryMetric(ctx, actor, name, arguments)
	case "trace_entity":
		return s.traceEntity(ctx, actor, arguments)
	default:
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "analytics tool not found", nil)
	}
}

func (s *Service) queryMetric(ctx context.Context, actor domain.RequestActor, tool string, arguments map[string]interface{}) (*ToolOutput, *domain.AppError) {
	metricID := argumentString(arguments, "metric_id")
	definition, ok := s.metrics[metricID]
	if !ok {
		return nil, invalidArgument("metric_id", "unknown analytics metric")
	}
	from, to, appErr := analyticsRange(arguments, time.Now().UTC())
	if appErr != nil {
		return nil, appErr
	}
	groupBy := argumentString(arguments, "group_by")
	if tool == "query_timeseries" {
		groupBy = "day"
	} else if inferred := inferAnalyticsGroupBy(argumentString(arguments, "_question")); inferred != "" && analyticsGroupIsAllowed(definition.AllowedGroupBys, inferred) {
		// Explicit wording from the user wins over a planner-selected dimension.
		// This is semantic validation, not a per-question metric implementation.
		groupBy = inferred
	}
	if groupBy == "" {
		groupBy = "day"
	}
	limit := argumentInt(arguments, "limit", 100)
	if limit < 1 || limit > 200 {
		return nil, invalidArgument("limit", "limit must be between 1 and 200")
	}
	access := domain.ResourceGroupAccessFilterForActor(actor, domain.PermissionTaskView)
	if definition.Strategy == "design_productivity" {
		if s.legacy == nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "design productivity executor is unavailable", nil)
		}
		hits, err := s.legacy.ListKPIEvidence(ctx, access, from, to, min(limit, 20))
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "query design productivity", map[string]interface{}{"cause": err.Error()})
		}
		return hitsOutput(metricID, from, to, hits), nil
	}
	if s.repo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "analytics repository is unavailable", nil)
	}
	result, err := s.repo.QueryMetric(ctx, access, definition, domain.AnalyticsMetricQuery{
		MetricID: metricID, From: from, To: to, GroupBy: groupBy, Limit: limit,
	})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "query analytics metric", map[string]interface{}{"cause": err.Error()})
	}
	hits := metricResultHits(result)
	output := structuredOutput(result)
	output.Hits = hits
	return output, nil
}

func inferAnalyticsGroupBy(question string) string {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return ""
	}
	groups := []struct {
		name     string
		keywords []string
	}{
		{name: "person", keywords: []string{"设计师", "人员", "姓名", "员工", "谁", "个人"}},
		{name: "department", keywords: []string{"部门"}},
		{name: "team", keywords: []string{"团队", "小组", "组别"}},
		{name: "task_type", keywords: []string{"任务类型", "任务类别", "类型分布"}},
		{name: "day", keywords: []string{"每天", "每日", "按天", "逐日", "日趋势", "时间趋势"}},
		{name: "page", keywords: []string{"页面"}},
		{name: "route", keywords: []string{"接口", "路径", "api"}},
		{name: "outcome", keywords: []string{"结果分布", "成功失败", "成功率", "失败率"}},
	}
	for _, group := range groups {
		for _, keyword := range group.keywords {
			if strings.Contains(question, keyword) {
				return group.name
			}
		}
	}
	return ""
}

func analyticsGroupIsAllowed(allowed []string, candidate string) bool {
	for _, value := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func (s *Service) traceEntity(ctx context.Context, actor domain.RequestActor, arguments map[string]interface{}) (*ToolOutput, *domain.AppError) {
	if s.repo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "analytics repository is unavailable", nil)
	}
	entityType := argumentString(arguments, "entity_type")
	entityID := argumentString(arguments, "entity_id")
	if entityType == "" || entityID == "" {
		return nil, invalidArgument("entity", "entity_type and entity_id are required")
	}
	from, to, appErr := analyticsRange(arguments, time.Now().UTC())
	if appErr != nil {
		return nil, appErr
	}
	limit := argumentInt(arguments, "limit", 50)
	if limit < 1 || limit > 100 {
		return nil, invalidArgument("limit", "limit must be between 1 and 100")
	}
	access := domain.ResourceGroupAccessFilterForActor(actor, domain.PermissionTaskView)
	hits, err := s.repo.TraceEntity(ctx, access, domain.AnalyticsTraceQuery{EntityType: entityType, EntityID: entityID, From: from, To: to, Limit: limit})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "trace analytics entity", map[string]interface{}{"cause": err.Error()})
	}
	return hitsOutput("trace_entity", from, to, hits), nil
}

func (s *Service) metricList() []domain.AnalyticsMetricDefinition {
	items := make([]domain.AnalyticsMetricDefinition, 0, len(s.metrics))
	for _, metric := range s.metrics {
		items = append(items, metric)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func defaultMetricCatalog() map[string]domain.AnalyticsMetricDefinition {
	taskGroups := []string{"day", "person", "department", "team", "task_type", "event_type", "total"}
	traceGroups := []string{"day", "person", "department", "team", "route", "page", "outcome", "event_type", "source", "total"}
	taskMetric := func(id, name, description string, events ...string) domain.AnalyticsMetricDefinition {
		return domain.AnalyticsMetricDefinition{ID: id, Name: name, Description: description, Source: domain.AnalyticsMetricSourceTaskEvent,
			EventTypes: events, Measures: []string{"event_count", "task_count", "actor_count"}, AllowedGroupBys: taskGroups}
	}
	traceMetric := func(id, name, description string, events ...string) domain.AnalyticsMetricDefinition {
		return domain.AnalyticsMetricDefinition{ID: id, Name: name, Description: description, Source: domain.AnalyticsMetricSourceWorkflowTrace,
			EventTypes: events, Measures: []string{"event_count", "task_count", "actor_count", "average_latency_ms"}, AllowedGroupBys: traceGroups}
	}
	items := []domain.AnalyticsMetricDefinition{
		{ID: "design_productivity", Name: "设计产能", Description: "设计提交任务、设计单元、约图量、精修产出及人员分布。", Source: domain.AnalyticsMetricSourceDerived, Strategy: "design_productivity", Measures: []string{"task_count", "design_units", "estimated_images", "minimum_images"}, AllowedGroupBys: []string{"day", "person", "summary"}},
		taskMetric("task_events", "全部任务事件", "任务全生命周期业务事件。"),
		taskMetric("task_created", "任务创建", "新建任务事件。", "task.created"),
		taskMetric("task_assignment", "任务指派", "任务指派和重新指派。", "task.assigned", "task.reassigned", "task.batch_assigned"),
		taskMetric("task_design_submitted", "设计提交", "当前及兼容历史设计提交事件。", "task.design_submitted", "task.design.submitted"),
		taskMetric("task_audit_decision", "审核处理", "审核通过与退回设计。", "task.audit.approved", "task.audit.returned_to_design", "task.audit.rejected"),
		taskMetric("task_closed", "任务结单", "任务显式结单事件。", "task.closed"),
		taskMetric("asset_upload_completed", "资产上传完成", "上传会话完成。", "task.asset.upload_session.completed"),
		taskMetric("erp_image_sync", "ERP图片同步", "ERP图片同步成功、失败和等待上传。", "task.erp_image.auto_synced", "task.erp_image.auto_sync_failed", "task.erp_image.awaiting_upload"),
		taskMetric("task_filing", "ERP建档", "任务ERP建档触发。", "task.filing.triggered"),
		traceMetric("workflow_events", "全部系统埋点", "API、前端、系统和集成的统一埋点。"),
		traceMetric("api_requests", "API请求", "服务端API请求量、任务覆盖和平均延迟。", "api_request"),
		traceMetric("page_views", "页面访问", "前端页面访问埋点。", "page_view"),
		traceMetric("user_actions", "用户操作", "前端及系统用户操作埋点。", "user_action"),
	}
	out := make(map[string]domain.AnalyticsMetricDefinition, len(items))
	for _, item := range items {
		out[item.ID] = item
	}
	return out
}

func defaultTools() []ToolDefinition {
	dateProperties := map[string]interface{}{
		"from": map[string]interface{}{"type": "string", "format": "date"},
		"to":   map[string]interface{}{"type": "string", "format": "date"},
		"days": map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 366},
	}
	queryProperties := cloneSchemaProperties(dateProperties)
	queryProperties["metric_id"] = map[string]interface{}{"type": "string"}
	queryProperties["group_by"] = map[string]interface{}{"type": "string"}
	queryProperties["limit"] = map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 200}
	return []ToolDefinition{
		{Name: "list_metrics", Title: "列出指标", Description: "列出当前账号可使用的确定性统计指标、来源、度量和分组维度。", InputSchema: objectSchema(map[string]interface{}{}, nil)},
		{Name: "describe_metric", Title: "说明指标", Description: "返回一个指标的口径、数据源、度量和允许分组。", InputSchema: objectSchema(map[string]interface{}{"metric_id": map[string]interface{}{"type": "string"}}, []string{"metric_id"})},
		{Name: "query_metric", Title: "查询指标", Description: "按受控指标和维度查询；不接受SQL。", InputSchema: objectSchema(queryProperties, []string{"metric_id"})},
		{Name: "query_timeseries", Title: "查询时间序列", Description: "按北京时间日粒度查询指标趋势。", InputSchema: objectSchema(queryProperties, []string{"metric_id"})},
		{Name: "query_distribution", Title: "查询分布", Description: "按人员、部门、团队、任务类型、事件类型、页面、路径或结果查询分布。", InputSchema: objectSchema(queryProperties, []string{"metric_id", "group_by"})},
		{Name: "trace_entity", Title: "追踪实体", Description: "按任务、SKU、资产或人员追踪统一埋点链路。", InputSchema: objectSchema(map[string]interface{}{
			"entity_type": map[string]interface{}{"type": "string", "enum": []string{"task", "sku", "asset", "user"}},
			"entity_id":   map[string]interface{}{"type": "string"}, "from": dateProperties["from"], "to": dateProperties["to"], "days": dateProperties["days"],
			"limit": map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 100},
		}, []string{"entity_type", "entity_id"})},
	}
}

func requireAnalyticsAccess(actor domain.RequestActor, requiresTaskView bool) *domain.AppError {
	if actor.ID <= 0 || actor.EffectiveAccess == nil || !domain.ActorHasPermission(actor, domain.PermissionReportView) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "report.view is required", nil)
	}
	if requiresTaskView && !domain.ActorHasPermission(actor, domain.PermissionTaskView) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "task.view is required for analytics queries", nil)
	}
	return nil
}

func analyticsRange(arguments map[string]interface{}, now time.Time) (time.Time, time.Time, *domain.AppError) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	if days := argumentInt(arguments, "days", 0); days > 0 {
		if days > 366 {
			return time.Time{}, time.Time{}, invalidArgument("days", "days must be between 1 and 366")
		}
		return today.AddDate(0, 0, -days+1), today.AddDate(0, 0, 1), nil
	}
	fromRaw, toRaw := argumentString(arguments, "from"), argumentString(arguments, "to")
	if fromRaw == "" && toRaw == "" {
		return today.AddDate(0, 0, -29), today.AddDate(0, 0, 1), nil
	}
	from, fromErr := time.ParseInLocation("2006-01-02", fromRaw, location)
	to, toErr := time.ParseInLocation("2006-01-02", toRaw, location)
	if fromErr != nil || toErr != nil || to.Before(from) || to.Sub(from) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, invalidArgument("date_range", "from/to must be valid Beijing dates within 366 days")
	}
	return from, to.AddDate(0, 0, 1), nil
}

func structuredOutput(value interface{}) *ToolOutput {
	raw, _ := json.Marshal(value)
	return &ToolOutput{Text: string(raw), Structured: value, Hits: []domain.AIRetrievalHit{}}
}

func hitsOutput(metric string, from, to time.Time, hits []domain.AIRetrievalHit) *ToolOutput {
	structured := map[string]interface{}{"metric_id": metric, "from": from, "to": to, "sources": hits}
	parts := make([]string, 0, len(hits))
	for _, hit := range hits {
		parts = append(parts, hit.Title+"："+hit.Excerpt)
	}
	return &ToolOutput{Text: strings.Join(parts, "\n"), Structured: structured, Hits: hits}
}

func metricResultHits(result *domain.AnalyticsMetricResult) []domain.AIRetrievalHit {
	if result == nil {
		return []domain.AIRetrievalHit{}
	}
	hits := make([]domain.AIRetrievalHit, 0, len(result.Rows))
	for _, row := range result.Rows {
		excerpt := fmt.Sprintf("指标：%s；分组：%s；事件%d；任务%d；人员%d；平均延迟%.1fms；区间%s至%s",
			result.MetricName, row.Label, row.EventCount, row.TaskCount, row.ActorCount, row.AverageLatencyMS,
			result.From.Format("2006-01-02"), result.To.Add(-time.Nanosecond).Format("2006-01-02"))
		hits = append(hits, domain.AIRetrievalHit{DocumentID: "analytics:" + result.MetricID + ":" + row.Key,
			EntityType: "analytics_metric", EntityID: row.Key, Title: result.MetricName + " · " + row.Label,
			InternalRoute: "/data-center", Excerpt: excerpt, SourceVersion: fmt.Sprintf("%d:%d", row.EventCount, row.TaskCount), Score: 1, Source: "analytics_registry"})
	}
	return hits
}

func invalidArgument(field, message string) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInvalidRequest, message, map[string]interface{}{"field": field})
}

func argumentString(arguments map[string]interface{}, key string) string {
	value, ok := arguments[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func argumentInt(arguments map[string]interface{}, key string, fallback int) int {
	raw, ok := arguments[key]
	if !ok || raw == nil || raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(raw)))
	if err != nil {
		return fallback
	}
	return value
}

func objectSchema(properties map[string]interface{}, required []string) map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}

func cloneSchemaProperties(source map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}
