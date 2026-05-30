package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type workflowTraceEventRepo struct{ db *DB }

func NewWorkflowTraceEventRepo(db *DB) repo.WorkflowTraceEventRepo {
	return &workflowTraceEventRepo{db: db}
}

const workflowTraceEventSelectCols = `
	id, event_id, trace_id, event_source, event_type, action,
	actor_id, actor_username, actor_source, actor_auth_mode, actor_roles_json, actor_department, actor_team,
	route_method, route_path, route_full_path, http_status, latency_ms, client_ip, user_agent,
	page_url, page_name, component_id,
	task_id, task_module_id, module_key, sku_code, task_sku_item_id,
	asset_id, design_asset_id, task_asset_id, integration_call_log_id,
	resource_type, resource_id, outcome, payload_json, occurred_at, created_at`

func (r *workflowTraceEventRepo) Create(ctx context.Context, tx repo.Tx, event *domain.WorkflowTraceEvent) (int64, error) {
	if event == nil {
		return 0, fmt.Errorf("workflow trace event is nil")
	}
	rolesJSON, err := json.Marshal(domain.NormalizeRoleValues(event.ActorRoles))
	if err != nil {
		return 0, fmt.Errorf("marshal workflow trace roles: %w", err)
	}
	exec := interface {
		ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	}(r.db.db)
	if tx != nil {
		exec = Unwrap(tx)
	}
	result, err := exec.ExecContext(ctx, `
		INSERT INTO workflow_trace_events (
			event_id, trace_id, event_source, event_type, action,
			actor_id, actor_username, actor_source, actor_auth_mode, actor_roles_json, actor_department, actor_team,
			route_method, route_path, route_full_path, http_status, latency_ms, client_ip, user_agent,
			page_url, page_name, component_id,
			task_id, task_module_id, module_key, sku_code, task_sku_item_id,
			asset_id, design_asset_id, task_asset_id, integration_call_log_id,
			resource_type, resource_id, outcome, payload_json, occurred_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(event.EventID),
		strings.TrimSpace(event.TraceID),
		strings.TrimSpace(event.EventSource),
		strings.TrimSpace(event.EventType),
		strings.TrimSpace(event.Action),
		toNullInt64(event.ActorID),
		strings.TrimSpace(event.ActorUsername),
		strings.TrimSpace(event.ActorSource),
		strings.TrimSpace(string(event.ActorAuthMode)),
		string(rolesJSON),
		strings.TrimSpace(event.ActorDepartment),
		strings.TrimSpace(event.ActorTeam),
		strings.TrimSpace(event.RouteMethod),
		strings.TrimSpace(event.RoutePath),
		strings.TrimSpace(event.RouteFullPath),
		toNullInt(event.HTTPStatus),
		toNullInt64(event.LatencyMS),
		strings.TrimSpace(event.ClientIP),
		strings.TrimSpace(event.UserAgent),
		strings.TrimSpace(event.PageURL),
		strings.TrimSpace(event.PageName),
		strings.TrimSpace(event.ComponentID),
		toNullInt64(event.TaskID),
		toNullInt64(event.TaskModuleID),
		strings.TrimSpace(event.ModuleKey),
		strings.TrimSpace(event.SKUCode),
		toNullInt64(event.TaskSKUItemID),
		toNullInt64(event.AssetID),
		toNullInt64(event.DesignAssetID),
		toNullInt64(event.TaskAssetID),
		toNullInt64(event.IntegrationCallLogID),
		strings.TrimSpace(event.ResourceType),
		strings.TrimSpace(event.ResourceID),
		strings.TrimSpace(event.Outcome),
		toNullJSONString(event.Payload),
		event.OccurredAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert workflow trace event: %w", err)
	}
	return result.LastInsertId()
}

func (r *workflowTraceEventRepo) List(ctx context.Context, filter repo.WorkflowTraceEventListFilter) ([]*domain.WorkflowTraceEvent, int64, error) {
	where := []string{"1=1"}
	args := make([]interface{}, 0, 12)
	if value := strings.TrimSpace(filter.TraceID); value != "" {
		where = append(where, "trace_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.EventSource); value != "" {
		where = append(where, "event_source = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.EventType); value != "" {
		where = append(where, "event_type = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.Action); value != "" {
		where = append(where, "action LIKE ?")
		args = append(args, "%"+value+"%")
	}
	if filter.ActorID != nil {
		where = append(where, "actor_id = ?")
		args = append(args, *filter.ActorID)
	}
	if value := strings.TrimSpace(filter.ActorUsername); value != "" {
		where = append(where, "actor_username LIKE ?")
		args = append(args, "%"+value+"%")
	}
	if value := strings.TrimSpace(filter.ActorSource); value != "" {
		where = append(where, "actor_source = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.ActorDepartment); value != "" {
		where = append(where, "actor_department = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.ActorTeam); value != "" {
		where = append(where, "actor_team = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.RoutePath); value != "" {
		where = append(where, "route_path = ?")
		args = append(args, value)
	}
	if filter.TaskID != nil {
		where = append(where, "task_id = ?")
		args = append(args, *filter.TaskID)
	}
	if value := strings.TrimSpace(filter.ModuleKey); value != "" {
		where = append(where, "module_key = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.SKUCode); value != "" {
		where = append(where, "sku_code = ?")
		args = append(args, value)
	}
	if filter.AssetID != nil {
		where = append(where, "asset_id = ?")
		args = append(args, *filter.AssetID)
	}
	if filter.DesignAssetID != nil {
		where = append(where, "design_asset_id = ?")
		args = append(args, *filter.DesignAssetID)
	}
	if filter.TaskAssetID != nil {
		where = append(where, "task_asset_id = ?")
		args = append(args, *filter.TaskAssetID)
	}
	if filter.IntegrationCallLogID != nil {
		where = append(where, "integration_call_log_id = ?")
		args = append(args, *filter.IntegrationCallLogID)
	}
	if value := strings.TrimSpace(filter.ResourceType); value != "" {
		where = append(where, "resource_type = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.ResourceID); value != "" {
		where = append(where, "resource_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.Outcome); value != "" {
		if value == domain.WorkflowTraceOutcomeFailed {
			where = append(where, "(outcome = ? AND (http_status IS NULL OR http_status >= 500))")
		} else {
			where = append(where, "outcome = ?")
		}
		args = append(args, value)
	}
	if filter.BusinessOnly {
		where = append(where, workflowTraceBusinessOnlyWhere())
	}
	if filter.From != nil {
		where = append(where, "occurred_at >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		where = append(where, "occurred_at <= ?")
		args = append(args, *filter.To)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_trace_events WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count workflow trace events: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize
	listArgs := append(append([]interface{}{}, args...), pageSize, offset)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT `+workflowTraceEventSelectCols+`
		FROM workflow_trace_events
		WHERE `+whereSQL+`
		ORDER BY occurred_at DESC, id DESC
		LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list workflow trace events: %w", err)
	}
	defer rows.Close()

	events := make([]*domain.WorkflowTraceEvent, 0)
	for rows.Next() {
		event, err := scanWorkflowTraceEvent(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan workflow trace event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate workflow trace events: %w", err)
	}
	return events, total, nil
}

func workflowTraceBusinessOnlyWhere() string {
	return `(
		event_source = 'frontend'
		OR task_id IS NOT NULL
		OR task_module_id IS NOT NULL
		OR task_sku_item_id IS NOT NULL
		OR asset_id IS NOT NULL
		OR design_asset_id IS NOT NULL
		OR task_asset_id IS NOT NULL
		OR integration_call_log_id IS NOT NULL
		OR NULLIF(TRIM(module_key), '') IS NOT NULL
		OR NULLIF(TRIM(sku_code), '') IS NOT NULL
		OR NULLIF(TRIM(resource_type), '') IS NOT NULL
		OR route_path LIKE '/v1/tasks%'
		OR route_path LIKE '/v1/task-create%'
		OR route_path LIKE '/v1/assets%'
		OR route_path LIKE '/v1/erp%'
		OR route_path LIKE '/v1/products%'
		OR route_path LIKE '/v1/sku%'
		OR route_path LIKE '/v1/warehouse%'
		OR route_path LIKE '/v1/audit%'
		OR route_path LIKE '/v1/reports%'
		OR route_path LIKE '/v1/finance%'
		OR route_path LIKE '/v1/export%'
	)`
}

func scanWorkflowTraceEvent(scanner interface {
	Scan(...interface{}) error
}) (*domain.WorkflowTraceEvent, error) {
	var event domain.WorkflowTraceEvent
	var actorRolesJSON string
	var actorID sql.NullInt64
	var httpStatus sql.NullInt64
	var latencyMS sql.NullInt64
	var taskID sql.NullInt64
	var taskModuleID sql.NullInt64
	var taskSKUItemID sql.NullInt64
	var assetID sql.NullInt64
	var designAssetID sql.NullInt64
	var taskAssetID sql.NullInt64
	var integrationCallLogID sql.NullInt64
	var payloadJSON sql.NullString
	if err := scanner.Scan(
		&event.ID,
		&event.EventID,
		&event.TraceID,
		&event.EventSource,
		&event.EventType,
		&event.Action,
		&actorID,
		&event.ActorUsername,
		&event.ActorSource,
		&event.ActorAuthMode,
		&actorRolesJSON,
		&event.ActorDepartment,
		&event.ActorTeam,
		&event.RouteMethod,
		&event.RoutePath,
		&event.RouteFullPath,
		&httpStatus,
		&latencyMS,
		&event.ClientIP,
		&event.UserAgent,
		&event.PageURL,
		&event.PageName,
		&event.ComponentID,
		&taskID,
		&taskModuleID,
		&event.ModuleKey,
		&event.SKUCode,
		&taskSKUItemID,
		&assetID,
		&designAssetID,
		&taskAssetID,
		&integrationCallLogID,
		&event.ResourceType,
		&event.ResourceID,
		&event.Outcome,
		&payloadJSON,
		&event.OccurredAt,
		&event.CreatedAt,
	); err != nil {
		return nil, err
	}
	event.ActorID = fromNullInt64(actorID)
	event.HTTPStatus = fromNullInt(httpStatus)
	event.LatencyMS = fromNullInt64(latencyMS)
	event.TaskID = fromNullInt64(taskID)
	event.TaskModuleID = fromNullInt64(taskModuleID)
	event.TaskSKUItemID = fromNullInt64(taskSKUItemID)
	event.AssetID = fromNullInt64(assetID)
	event.DesignAssetID = fromNullInt64(designAssetID)
	event.TaskAssetID = fromNullInt64(taskAssetID)
	event.IntegrationCallLogID = fromNullInt64(integrationCallLogID)
	roles, err := unmarshalOptionalRoles(actorRolesJSON)
	if err != nil {
		return nil, err
	}
	event.ActorRoles = domain.NormalizeRoleValues(roles)
	payload, err := unmarshalOptionalRawJSON(payloadJSON.String, "workflow trace payload")
	if err != nil {
		return nil, err
	}
	event.Payload = payload
	return &event, nil
}
