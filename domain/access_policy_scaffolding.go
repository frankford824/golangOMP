package domain

import "strings"

// PolicyMode describes how access rules are currently modeled.
// Step 49 keeps this as scaffolding only and does not introduce real IdP/org/RBAC engines.
type PolicyMode string

const (
	PolicyModeRouteRoleVisibilityScaffolding PolicyMode = "route_role_visibility_scaffolding"
)

// PolicyAPISurface classifies endpoint exposure for action summaries.
type PolicyAPISurface string

const (
	PolicyAPISurfaceFrontendReady   PolicyAPISurface = "frontend_ready"
	PolicyAPISurfaceInternal        PolicyAPISurface = "internal"
	PolicyAPISurfaceAdmin           PolicyAPISurface = "admin"
	PolicyAPISurfaceMockPlaceholder PolicyAPISurface = "mock_placeholder"
)

// ActionPolicySummary is a default action-level policy hint.
// It does not replace route-level checks or future fine-grained ABAC/RBAC policy engines.
type ActionPolicySummary struct {
	ActionKey    string           `json:"action_key"`
	AllowedRoles []Role           `json:"allowed_roles,omitempty"`
	APISurface   PolicyAPISurface `json:"api_surface"`
	Note         string           `json:"note,omitempty"`
}

// ResourceAccessPolicy is a resource-level default policy hint.
// It is intentionally additive and placeholder-only in the current phase.
type ResourceAccessPolicy struct {
	ResourceKey    string                `json:"resource_key"`
	VisibleToRoles []Role                `json:"visible_to_roles,omitempty"`
	ActionRoles    []ActionPolicySummary `json:"action_roles,omitempty"`
	PolicyMode     PolicyMode            `json:"policy_mode"`
}

// PolicyScopeSummary provides one cross-center scaffolding envelope for policy language reuse.
// It explains scope-level defaults without introducing a real auth/org system.
type PolicyScopeSummary struct {
	ScopeKey             string               `json:"scope_key"`
	ScopeName            string               `json:"scope_name"`
	AuthMode             AuthMode             `json:"auth_mode"`
	PolicyMode           PolicyMode           `json:"policy_mode"`
	ResourceAccessPolicy ResourceAccessPolicy `json:"resource_access_policy"`
	IsPlaceholder        bool                 `json:"is_placeholder"`
	Note                 string               `json:"note,omitempty"`
}

func actionPolicy(actionKey string, surface PolicyAPISurface, note string, roles ...Role) ActionPolicySummary {
	return ActionPolicySummary{
		ActionKey:    strings.TrimSpace(actionKey),
		AllowedRoles: cloneRoles(normalizeRolesFromValues(roles)),
		APISurface:   surface,
		Note:         strings.TrimSpace(note),
	}
}

func policyScaffolding(
	scopeKey string,
	scopeName string,
	resourceKey string,
	visible []Role,
	actions []ActionPolicySummary,
	note string,
) (PolicyMode, []Role, []ActionPolicySummary, *PolicyScopeSummary) {
	mode := PolicyModeRouteRoleVisibilityScaffolding
	resource := ResourceAccessPolicy{
		ResourceKey:    strings.TrimSpace(resourceKey),
		VisibleToRoles: cloneRoles(normalizeRolesFromValues(visible)),
		ActionRoles:    cloneActionPolicies(actions),
		PolicyMode:     mode,
	}
	summary := &PolicyScopeSummary{
		ScopeKey:             strings.TrimSpace(scopeKey),
		ScopeName:            strings.TrimSpace(scopeName),
		AuthMode:             AuthModeDebugHeaderRoleEnforced,
		PolicyMode:           mode,
		ResourceAccessPolicy: resource,
		IsPlaceholder:        true,
		Note:                 strings.TrimSpace(note),
	}
	return mode, cloneRoles(resource.VisibleToRoles), cloneActionPolicies(resource.ActionRoles), summary
}

func cloneRoles(values []Role) []Role {
	if len(values) == 0 {
		return nil
	}
	out := make([]Role, 0, len(values))
	out = append(out, values...)
	return out
}

func cloneActionPolicies(values []ActionPolicySummary) []ActionPolicySummary {
	if len(values) == 0 {
		return nil
	}
	out := make([]ActionPolicySummary, 0, len(values))
	for _, item := range values {
		copyItem := item
		copyItem.AllowedRoles = cloneRoles(item.AllowedRoles)
		out = append(out, copyItem)
	}
	return out
}

func normalizeRolesFromValues(values []Role) []Role {
	if len(values) == 0 {
		return nil
	}
	return NormalizeRoleValues(values)
}

func taskPolicyScaffolding(resourceKey string) (PolicyMode, []Role, []ActionPolicySummary, *PolicyScopeSummary) {
	switch strings.TrimSpace(resourceKey) {
	case "task_board_summary", "task_board_queues_response", "task_board_queue_summary", "task_board_queue":
		visible := []Role{RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleAdmin}
		actions := []ActionPolicySummary{
			actionPolicy("read_board_summary", PolicyAPISurfaceFrontendReady, "Task board summary is frontend-ready.", RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleAdmin),
			actionPolicy("read_board_queues", PolicyAPISurfaceFrontendReady, "Task board queues are frontend-ready.", RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleAdmin),
			actionPolicy("drill_down_task_list", PolicyAPISurfaceFrontendReady, "Queue drill-down still routes through task list contracts.", RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleAdmin),
		}
		return policyScaffolding(
			"task_center",
			"Task/Detail/Board",
			resourceKey,
			visible,
			actions,
			"Task board policy scaffolding expresses default queue visibility and drill-down operation roles only.",
		)
	case "cost_override_boundary":
		visible := []Role{RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleOutsource, RoleAdmin}
		actions := []ActionPolicySummary{
			actionPolicy("read_override_boundary", PolicyAPISurfaceFrontendReady, "Boundary summary is frontend-readable on task read/detail contracts.", RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleOutsource, RoleAdmin),
			actionPolicy("read_override_timeline", PolicyAPISurfaceFrontendReady, "Dedicated cost-override timeline read route remains frontend-ready.", RoleOps, RoleWarehouse, RoleAdmin),
			actionPolicy("review_override_placeholder", PolicyAPISurfaceInternal, "Placeholder review action stays internal.", RoleOps, RoleWarehouse, RoleAdmin),
			actionPolicy("finance_mark_placeholder", PolicyAPISurfaceAdmin, "Finance-mark placeholder action is high-sensitivity admin/internal.", RoleERP, RoleAdmin),
		}
		return policyScaffolding(
			"cost_governance_center",
			"Cost Governance / Override Boundary",
			resourceKey,
			visible,
			actions,
			"This boundary is policy scaffolding only and is not a real approval/finance permission workflow.",
		)
	default:
		visible := []Role{RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleOutsource, RoleAdmin}
		actions := []ActionPolicySummary{
			actionPolicy("read_task", PolicyAPISurfaceFrontendReady, "Task list/read/detail contracts are frontend-ready.", RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleOutsource, RoleAdmin),
			actionPolicy("update_business_info", PolicyAPISurfaceFrontendReady, "Business-info maintenance remains frontend-ready.", RoleOps, RoleWarehouse, RoleAdmin),
			actionPolicy("update_procurement", PolicyAPISurfaceFrontendReady, "Procurement maintenance remains frontend-ready.", RoleOps, RoleWarehouse, RoleAdmin),
			actionPolicy("close_task", PolicyAPISurfaceFrontendReady, "Task close remains frontend-ready.", RoleOps, RoleWarehouse, RoleAdmin),
			actionPolicy("review_override_placeholder", PolicyAPISurfaceInternal, "Placeholder review action stays internal.", RoleOps, RoleWarehouse, RoleAdmin),
			actionPolicy("finance_mark_placeholder", PolicyAPISurfaceAdmin, "Finance-mark placeholder action is high-sensitivity admin/internal.", RoleERP, RoleAdmin),
		}
		return policyScaffolding(
			"task_center",
			"Task/Detail/Board",
			resourceKey,
			visible,
			actions,
			"Task center policy scaffolding reuses route-role defaults and does not introduce real org visibility trimming yet.",
		)
	}
}

func exportPolicyScaffolding(resourceKey string) (PolicyMode, []Role, []ActionPolicySummary, *PolicyScopeSummary) {
	visible := []Role{RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleAdmin}
	actions := []ActionPolicySummary{
		actionPolicy("create_export_job", PolicyAPISurfaceFrontendReady, "Export job create/list/detail/events/download routes are frontend-ready.", RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleAdmin),
		actionPolicy("read_export_job", PolicyAPISurfaceFrontendReady, "Export job create/list/detail/events/download routes are frontend-ready.", RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleAdmin),
		actionPolicy("claim_or_read_download", PolicyAPISurfaceFrontendReady, "Download handoff routes are frontend-ready placeholder contracts.", RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleAdmin),
		actionPolicy("manage_dispatch_or_attempt", PolicyAPISurfaceAdmin, "Dispatch/attempt inspection and start/advance actions remain admin placeholder only.", RoleAdmin),
	}
	return policyScaffolding(
		"export_center",
		"Export Center",
		resourceKey,
		visible,
		actions,
		"Export-center policy scaffolding keeps frontend vs admin placeholder boundaries explicit without real scheduler/identity integration.",
	)
}

func integrationPolicyScaffolding(resourceKey string) (PolicyMode, []Role, []ActionPolicySummary, *PolicyScopeSummary) {
	visible := []Role{RoleAdmin, RoleERP}
	actions := []ActionPolicySummary{
		actionPolicy("list_connectors", PolicyAPISurfaceInternal, "Connector catalog is internal placeholder only.", RoleAdmin, RoleERP),
		actionPolicy("manage_call_logs", PolicyAPISurfaceInternal, "Call-log create/list/detail/advance stays internal placeholder only.", RoleAdmin, RoleERP),
		actionPolicy("manage_executions", PolicyAPISurfaceInternal, "Execution list/create/advance stays internal placeholder only.", RoleAdmin, RoleERP),
		actionPolicy("retry_or_replay", PolicyAPISurfaceInternal, "Retry/replay are internal placeholder actions.", RoleAdmin, RoleERP),
	}
	return policyScaffolding(
		"integration_center",
		"Integration Center",
		resourceKey,
		visible,
		actions,
		"Integration-center policy scaffolding remains internal placeholder only and does not introduce real external-system auth.",
	)
}

func uploadPolicyScaffolding(resourceKey string) (PolicyMode, []Role, []ActionPolicySummary, *PolicyScopeSummary) {
	visible := []Role{RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleOutsource, RoleAdmin}
	actions := []ActionPolicySummary{
		actionPolicy("create_or_read_upload_request", PolicyAPISurfaceInternal, "Upload-request APIs stay internal placeholder only.", RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleOutsource, RoleAdmin),
		actionPolicy("advance_upload_request", PolicyAPISurfaceInternal, "Upload-request advance is internal placeholder only.", RoleOps, RoleDesigner, RoleAuditA, RoleAuditB, RoleWarehouse, RoleOutsource, RoleAdmin),
		actionPolicy("bind_via_submit_design", PolicyAPISurfaceFrontendReady, "Binding an upload request through submit-design is frontend-ready task flow.", RoleDesigner, RoleOps),
		actionPolicy("bind_via_mock_upload", PolicyAPISurfaceMockPlaceholder, "Mock-upload binding remains mock placeholder only.", RoleDesigner, RoleOps),
	}
	return policyScaffolding(
		"upload_storage_center",
		"Upload/Storage Boundary",
		resourceKey,
		visible,
		actions,
		"Upload/storage policy scaffolding only expresses placeholder lifecycle boundaries and route-surface defaults.",
	)
}

func HydrateTaskListItemPolicy(item *TaskListItem) {
	if item == nil {
		return
	}
	item.PolicyMode, item.VisibleToRoles, item.ActionRoles, item.PolicyScopeSummary = taskPolicyScaffolding("task_list_item")
	HydrateTaskListItemPlatformEntry(item)
	if item.ProcurementSummary != nil {
		HydrateProcurementSummaryPolicy(item.ProcurementSummary)
	}
}

func HydrateTaskReadModelPolicy(model *TaskReadModel) {
	if model == nil {
		return
	}
	model.PolicyMode, model.VisibleToRoles, model.ActionRoles, model.PolicyScopeSummary = taskPolicyScaffolding("task_read_model")
	HydrateTaskReadModelPlatformEntry(model)
	if model.ProcurementSummary != nil {
		HydrateProcurementSummaryPolicy(model.ProcurementSummary)
	}
	if model.OverrideBoundary != nil {
		HydrateTaskCostOverrideBoundaryPolicy(model.OverrideBoundary)
	}
}

func HydrateTaskDetailAggregatePolicy(aggregate *TaskDetailAggregate) {
	if aggregate == nil {
		return
	}
	aggregate.PolicyMode, aggregate.VisibleToRoles, aggregate.ActionRoles, aggregate.PolicyScopeSummary = taskPolicyScaffolding("task_detail_aggregate")
	HydrateTaskDetailAggregatePlatformEntry(aggregate)
	if aggregate.ProcurementSummary != nil {
		HydrateProcurementSummaryPolicy(aggregate.ProcurementSummary)
	}
	if aggregate.OverrideBoundary != nil {
		HydrateTaskCostOverrideBoundaryPolicy(aggregate.OverrideBoundary)
	}
}

func HydrateTaskBoardSummaryPolicy(summary *TaskBoardSummary) {
	if summary == nil {
		return
	}
	summary.PolicyMode, summary.VisibleToRoles, summary.ActionRoles, summary.PolicyScopeSummary = taskPolicyScaffolding("task_board_summary")
}

func HydrateTaskBoardQueueSummaryPolicy(queue *TaskBoardQueueSummary) {
	if queue == nil {
		return
	}
	queue.PolicyMode, queue.VisibleToRoles, queue.ActionRoles, queue.PolicyScopeSummary = taskPolicyScaffolding("task_board_queue_summary")
}

func HydrateTaskBoardQueuePolicy(queue *TaskBoardQueue) {
	if queue == nil {
		return
	}
	queue.PolicyMode, queue.VisibleToRoles, queue.ActionRoles, queue.PolicyScopeSummary = taskPolicyScaffolding("task_board_queue")
}

func HydrateTaskBoardQueuesResponsePolicy(resp *TaskBoardQueuesResponse) {
	if resp == nil {
		return
	}
	resp.PolicyMode, resp.VisibleToRoles, resp.ActionRoles, resp.PolicyScopeSummary = taskPolicyScaffolding("task_board_queues_response")
}

func HydrateProcurementSummaryPolicy(summary *ProcurementSummary) {
	if summary == nil {
		return
	}
	summary.PolicyMode, summary.VisibleToRoles, summary.ActionRoles, summary.PolicyScopeSummary = taskPolicyScaffolding("task_procurement_summary")
	HydrateProcurementSummaryPlatformEntry(summary)
}

func HydrateTaskCostOverrideBoundaryPolicy(boundary *TaskCostOverrideGovernanceBoundary) {
	if boundary == nil {
		return
	}
	boundary.PolicyMode, boundary.VisibleToRoles, boundary.ActionRoles, boundary.PolicyScopeSummary = taskPolicyScaffolding("cost_override_boundary")
	HydrateTaskCostOverrideBoundaryPlatformEntry(boundary)
}

func HydrateExportJobPolicy(job *ExportJob) {
	if job == nil {
		return
	}
	job.PolicyMode, job.VisibleToRoles, job.ActionRoles, job.PolicyScopeSummary = exportPolicyScaffolding("export_job")
	HydrateExportJobPlatformEntry(job)
}

func HydrateIntegrationCallLogPolicy(log *IntegrationCallLog) {
	if log == nil {
		return
	}
	log.PolicyMode, log.VisibleToRoles, log.ActionRoles, log.PolicyScopeSummary = integrationPolicyScaffolding("integration_call_log")
}

func HydrateIntegrationExecutionPolicy(execution *IntegrationExecution) {
	if execution == nil {
		return
	}
	execution.PolicyMode, execution.VisibleToRoles, execution.ActionRoles, execution.PolicyScopeSummary = integrationPolicyScaffolding("integration_execution")
}

func HydrateUploadRequestPolicy(request *UploadRequest) {
	if request == nil {
		return
	}
	request.PolicyMode, request.VisibleToRoles, request.ActionRoles, request.PolicyScopeSummary = uploadPolicyScaffolding("upload_request")
}
