package domain

import (
	"encoding/json"
	"time"
)

const (
	WorkflowTraceSourceAPI         = "api"
	WorkflowTraceSourceFrontend    = "frontend"
	WorkflowTraceSourceSystem      = "system"
	WorkflowTraceSourceIntegration = "integration"

	WorkflowTraceEventAPIRequest = "api_request"
	WorkflowTraceEventPageView   = "page_view"
	WorkflowTraceEventUserAction = "user_action"

	WorkflowTraceOutcomeSucceeded = "succeeded"
	WorkflowTraceOutcomeFailed    = "failed"
)

// WorkflowTraceEvent is the lightweight cross-system trace ledger entry.
// It intentionally keeps optional business identifiers denormalized so later
// AI/reporting queries can join task, SKU, asset, ERP, and UI events cheaply.
type WorkflowTraceEvent struct {
	ID                   int64           `db:"id" json:"id"`
	EventID              string          `db:"event_id" json:"event_id"`
	TraceID              string          `db:"trace_id" json:"trace_id,omitempty"`
	EventSource          string          `db:"event_source" json:"event_source"`
	EventType            string          `db:"event_type" json:"event_type"`
	Action               string          `db:"action" json:"action,omitempty"`
	ActorID              *int64          `db:"actor_id" json:"actor_id,omitempty"`
	ActorUsername        string          `db:"actor_username" json:"actor_username,omitempty"`
	ActorSource          string          `db:"actor_source" json:"actor_source,omitempty"`
	ActorAuthMode        AuthMode        `db:"actor_auth_mode" json:"actor_auth_mode,omitempty"`
	ActorRoles           []Role          `db:"-" json:"actor_roles,omitempty"`
	ActorDepartment      string          `db:"actor_department" json:"actor_department,omitempty"`
	ActorTeam            string          `db:"actor_team" json:"actor_team,omitempty"`
	RouteMethod          string          `db:"route_method" json:"route_method,omitempty"`
	RoutePath            string          `db:"route_path" json:"route_path,omitempty"`
	RouteFullPath        string          `db:"route_full_path" json:"route_full_path,omitempty"`
	HTTPStatus           *int            `db:"http_status" json:"http_status,omitempty"`
	LatencyMS            *int64          `db:"latency_ms" json:"latency_ms,omitempty"`
	ClientIP             string          `db:"client_ip" json:"client_ip,omitempty"`
	UserAgent            string          `db:"user_agent" json:"user_agent,omitempty"`
	PageURL              string          `db:"page_url" json:"page_url,omitempty"`
	PageName             string          `db:"page_name" json:"page_name,omitempty"`
	ComponentID          string          `db:"component_id" json:"component_id,omitempty"`
	TaskID               *int64          `db:"task_id" json:"task_id,omitempty"`
	TaskModuleID         *int64          `db:"task_module_id" json:"task_module_id,omitempty"`
	ModuleKey            string          `db:"module_key" json:"module_key,omitempty"`
	SKUCode              string          `db:"sku_code" json:"sku_code,omitempty"`
	TaskSKUItemID        *int64          `db:"task_sku_item_id" json:"task_sku_item_id,omitempty"`
	AssetID              *int64          `db:"asset_id" json:"asset_id,omitempty"`
	DesignAssetID        *int64          `db:"design_asset_id" json:"design_asset_id,omitempty"`
	TaskAssetID          *int64          `db:"task_asset_id" json:"task_asset_id,omitempty"`
	IntegrationCallLogID *int64          `db:"integration_call_log_id" json:"integration_call_log_id,omitempty"`
	ResourceType         string          `db:"resource_type" json:"resource_type,omitempty"`
	ResourceID           string          `db:"resource_id" json:"resource_id,omitempty"`
	Outcome              string          `db:"outcome" json:"outcome,omitempty"`
	Payload              json.RawMessage `db:"payload_json" json:"payload,omitempty"`
	OccurredAt           time.Time       `db:"occurred_at" json:"occurred_at"`
	CreatedAt            time.Time       `db:"created_at" json:"created_at"`
}
