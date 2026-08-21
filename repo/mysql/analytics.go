package mysqlrepo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type analyticsRepo struct{ db *DB }

func NewAnalyticsRepo(db *DB) repo.AnalyticsRepo { return &analyticsRepo{db: db} }

func (r *analyticsRepo) QueryMetric(
	ctx context.Context,
	access domain.ResourceGroupAccessFilter,
	definition domain.AnalyticsMetricDefinition,
	query domain.AnalyticsMetricQuery,
) (*domain.AnalyticsMetricResult, error) {
	if !query.To.After(query.From) {
		return nil, fmt.Errorf("analytics range is invalid")
	}
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 100
	}
	groupBy := strings.TrimSpace(query.GroupBy)
	if groupBy == "" {
		groupBy = "day"
	}
	if !analyticsGroupAllowed(definition.AllowedGroupBys, groupBy) {
		return nil, fmt.Errorf("analytics group_by %q is not allowed for %s", groupBy, definition.ID)
	}
	switch definition.Source {
	case domain.AnalyticsMetricSourceTaskEvent:
		return r.queryTaskEventMetric(ctx, access, definition, query, groupBy)
	case domain.AnalyticsMetricSourceWorkflowTrace:
		return r.queryWorkflowTraceMetric(ctx, access, definition, query, groupBy)
	default:
		return nil, fmt.Errorf("analytics metric source %q requires a derived executor", definition.Source)
	}
}

func (r *analyticsRepo) queryTaskEventMetric(
	ctx context.Context,
	access domain.ResourceGroupAccessFilter,
	definition domain.AnalyticsMetricDefinition,
	query domain.AnalyticsMetricQuery,
	groupBy string,
) (*domain.AnalyticsMetricResult, error) {
	keyExpr, labelExpr := taskEventAnalyticsGroup(groupBy)
	where := []string{"tel.created_at >= ?", "tel.created_at < ?"}
	args := []interface{}{query.From, query.To}
	if len(definition.EventTypes) > 0 {
		where = append(where, "tel.event_type IN ("+resourceGroupPlaceholders(len(definition.EventTypes))+")")
		for _, eventType := range definition.EventTypes {
			args = append(args, eventType)
		}
	}
	where, args = appendResourceGroupAccessScope(where, args, access)
	args = append(args, query.Limit)
	querySQL := `
		SELECT ` + keyExpr + ` AS row_key, ` + labelExpr + ` AS row_label,
		       COUNT(*) AS event_count, COUNT(DISTINCT tel.task_id) AS task_count,
		       COUNT(DISTINCT tel.operator_id) AS actor_count, 0 AS average_latency_ms
		  FROM task_event_logs tel
		  JOIN tasks t ON t.id = tel.task_id
	  LEFT JOIN users actor ON actor.id = COALESCE(tel.operator_id, t.designer_id)
		 WHERE ` + strings.Join(where, " AND ") + `
	 GROUP BY row_key, row_label
	 ORDER BY ` + analyticsOrderBy(groupBy) + ` LIMIT ?`
	return r.scanAnalyticsMetricRows(ctx, definition, query, groupBy, querySQL, args...)
}

func (r *analyticsRepo) queryWorkflowTraceMetric(
	ctx context.Context,
	access domain.ResourceGroupAccessFilter,
	definition domain.AnalyticsMetricDefinition,
	query domain.AnalyticsMetricQuery,
	groupBy string,
) (*domain.AnalyticsMetricResult, error) {
	keyExpr, labelExpr := workflowTraceAnalyticsGroup(groupBy)
	where := []string{"trace.occurred_at >= ?", "trace.occurred_at < ?"}
	args := []interface{}{query.From, query.To}
	if len(definition.EventTypes) > 0 {
		where = append(where, "trace.event_type IN ("+resourceGroupPlaceholders(len(definition.EventTypes))+")")
		for _, eventType := range definition.EventTypes {
			args = append(args, eventType)
		}
	}
	if !access.Global {
		where = append(where, "t.id IS NOT NULL")
		where, args = appendResourceGroupAccessScope(where, args, access)
	}
	args = append(args, query.Limit)
	querySQL := `
		SELECT ` + keyExpr + ` AS row_key, ` + labelExpr + ` AS row_label,
		       COUNT(*) AS event_count, COUNT(DISTINCT trace.task_id) AS task_count,
		       COUNT(DISTINCT trace.actor_id) AS actor_count, COALESCE(AVG(trace.latency_ms), 0) AS average_latency_ms
		  FROM workflow_trace_events trace
	  LEFT JOIN tasks t ON t.id = trace.task_id
		 WHERE ` + strings.Join(where, " AND ") + `
	 GROUP BY row_key, row_label
	 ORDER BY ` + analyticsOrderBy(groupBy) + ` LIMIT ?`
	return r.scanAnalyticsMetricRows(ctx, definition, query, groupBy, querySQL, args...)
}

func (r *analyticsRepo) scanAnalyticsMetricRows(
	ctx context.Context,
	definition domain.AnalyticsMetricDefinition,
	query domain.AnalyticsMetricQuery,
	groupBy, querySQL string,
	args ...interface{},
) (*domain.AnalyticsMetricResult, error) {
	queryCtx, cancel := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, querySQL, args...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("query analytics metric %s: %w", definition.ID, err)
	}
	defer cancel()
	defer rows.Close()
	result := &domain.AnalyticsMetricResult{
		MetricID: definition.ID, MetricName: definition.Name, From: query.From, To: query.To,
		TimeZone: "Asia/Shanghai", GroupBy: groupBy, Rows: make([]domain.AnalyticsMetricRow, 0, query.Limit),
	}
	for rows.Next() {
		var row domain.AnalyticsMetricRow
		if err := rows.Scan(&row.Key, &row.Label, &row.EventCount, &row.TaskCount, &row.ActorCount, &row.AverageLatencyMS); err != nil {
			return nil, fmt.Errorf("scan analytics metric %s: %w", definition.ID, err)
		}
		result.Rows = append(result.Rows, row)
	}
	return result, rows.Err()
}

func (r *analyticsRepo) TraceEntity(ctx context.Context, access domain.ResourceGroupAccessFilter, query domain.AnalyticsTraceQuery) ([]domain.AIRetrievalHit, error) {
	where := []string{"trace.occurred_at >= ?", "trace.occurred_at < ?"}
	args := []interface{}{query.From, query.To}
	switch query.EntityType {
	case "task":
		id, err := strconv.ParseInt(query.EntityID, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("task entity_id is invalid")
		}
		where = append(where, "trace.task_id = ?")
		args = append(args, id)
	case "sku":
		where = append(where, "trace.sku_code = ?")
		args = append(args, strings.TrimSpace(query.EntityID))
	case "asset":
		id, err := strconv.ParseInt(query.EntityID, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("asset entity_id is invalid")
		}
		where = append(where, "(trace.asset_id = ? OR trace.design_asset_id = ? OR trace.task_asset_id = ?)")
		args = append(args, id, id, id)
	case "user":
		id, err := strconv.ParseInt(query.EntityID, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("user entity_id is invalid")
		}
		where = append(where, "trace.actor_id = ?")
		args = append(args, id)
	default:
		return nil, fmt.Errorf("entity_type must be task, sku, asset, or user")
	}
	if !access.Global {
		where = append(where, "t.id IS NOT NULL")
		where, args = appendResourceGroupAccessScope(where, args, access)
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 50
	}
	args = append(args, query.Limit)
	queryCtx, cancel := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, `
		SELECT trace.id, trace.event_type, trace.action, trace.outcome, trace.occurred_at,
		       COALESCE(trace.task_id, 0), COALESCE(t.task_no, ''), COALESCE(trace.sku_code, ''),
		       COALESCE(trace.actor_username, ''), COALESCE(trace.route_path, '')
		  FROM workflow_trace_events trace LEFT JOIN tasks t ON t.id = trace.task_id
		 WHERE `+strings.Join(where, " AND ")+`
	 ORDER BY trace.occurred_at DESC, trace.id DESC LIMIT ?`, args...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("trace analytics entity: %w", err)
	}
	defer cancel()
	defer rows.Close()
	hits := make([]domain.AIRetrievalHit, 0, query.Limit)
	for rows.Next() {
		var id, taskID int64
		var eventType, action, outcome, taskNo, sku, actor, route string
		var occurredAt time.Time
		if err := rows.Scan(&id, &eventType, &action, &outcome, &occurredAt, &taskID, &taskNo, &sku, &actor, &route); err != nil {
			return nil, fmt.Errorf("scan analytics trace entity: %w", err)
		}
		title := compactAnalysisText([]string{eventType, action, taskNo, sku}, 255)
		excerpt := fmt.Sprintf("事件：%s；动作：%s；结果：%s；人员：%s；时间：%s；路径：%s",
			eventType, action, outcome, actor, occurredAt.Format("2006-01-02 15:04:05"), route)
		hits = append(hits, analysisHit(fmt.Sprintf("analytics:trace:%d", id), "analytics_trace", query.EntityID, title, route, excerpt, fmt.Sprint(id)))
	}
	return hits, rows.Err()
}

func analyticsGroupAllowed(allowed []string, value string) bool {
	for _, candidate := range allowed {
		if candidate == value {
			return true
		}
	}
	return false
}

func taskEventAnalyticsGroup(groupBy string) (string, string) {
	switch groupBy {
	case "day":
		return "DATE_FORMAT(tel.created_at, '%Y-%m-%d')", "DATE_FORMAT(tel.created_at, '%Y-%m-%d')"
	case "person":
		return "COALESCE(CAST(actor.id AS CHAR), '0')", "COALESCE(NULLIF(actor.display_name, ''), NULLIF(actor.username, ''), '未识别人员')"
	case "department":
		return "COALESCE(t.owner_department, '')", "COALESCE(NULLIF(t.owner_department, ''), '未归属部门')"
	case "team":
		return "COALESCE(t.owner_org_team, '')", "COALESCE(NULLIF(t.owner_org_team, ''), '未归属团队')"
	case "task_type":
		return "COALESCE(t.task_type, '')", "COALESCE(NULLIF(t.task_type, ''), '未分类任务')"
	case "event_type":
		return "tel.event_type", "tel.event_type"
	default:
		return "'total'", "'全部'"
	}
}

func workflowTraceAnalyticsGroup(groupBy string) (string, string) {
	switch groupBy {
	case "day":
		return "DATE_FORMAT(trace.occurred_at, '%Y-%m-%d')", "DATE_FORMAT(trace.occurred_at, '%Y-%m-%d')"
	case "person":
		return "COALESCE(CAST(trace.actor_id AS CHAR), '0')", "COALESCE(NULLIF(trace.actor_username, ''), '未识别人员')"
	case "department":
		return "COALESCE(trace.actor_department, '')", "COALESCE(NULLIF(trace.actor_department, ''), '未归属部门')"
	case "team":
		return "COALESCE(trace.actor_team, '')", "COALESCE(NULLIF(trace.actor_team, ''), '未归属团队')"
	case "route":
		return "COALESCE(trace.route_path, '')", "COALESCE(NULLIF(trace.route_path, ''), '无路径')"
	case "page":
		return "COALESCE(trace.page_name, '')", "COALESCE(NULLIF(trace.page_name, ''), '无页面')"
	case "outcome":
		return "COALESCE(trace.outcome, '')", "COALESCE(NULLIF(trace.outcome, ''), '未标记结果')"
	case "event_type":
		return "trace.event_type", "trace.event_type"
	case "source":
		return "trace.event_source", "trace.event_source"
	default:
		return "'total'", "'全部'"
	}
}

func analyticsOrderBy(groupBy string) string {
	if groupBy == "day" {
		return "row_key ASC"
	}
	return "event_count DESC, row_key ASC"
}

var _ repo.AnalyticsRepo = (*analyticsRepo)(nil)
