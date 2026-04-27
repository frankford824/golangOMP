package domain

import "strings"

// PlatformEntryMode describes how KPI/finance/report platform entry is modeled in the current phase.
// Step 50 keeps this as boundary scaffolding only and does not introduce real BI/finance/report engines.
type PlatformEntryMode string

const (
	PlatformEntryModeScaffoldingOnly PlatformEntryMode = "scaffolding_only"
)

// PlatformEntryStatus describes current entry-readiness intent for one future platform lane.
type PlatformEntryStatus string

const (
	PlatformEntryStatusCandidate     PlatformEntryStatus = "candidate"
	PlatformEntryStatusConditional   PlatformEntryStatus = "conditional"
	PlatformEntryStatusNotApplicable PlatformEntryStatus = "not_applicable"
)

// KPIEntrySummary is a placeholder entry summary for future KPI platform connection.
type KPIEntrySummary struct {
	EntryStatus            PlatformEntryStatus `json:"entry_status"`
	EligibleNow            bool                `json:"eligible_now"`
	SourceReadModelFields  []string            `json:"source_read_model_fields,omitempty"`
	PlaceholderFields      []string            `json:"placeholder_fields,omitempty"`
	FutureMetricHints      []string            `json:"future_metric_hints,omitempty"`
	IsPlaceholder          bool                `json:"is_placeholder"`
	NotReadyBlockingReason string              `json:"not_ready_blocking_reason,omitempty"`
	Note                   string              `json:"note,omitempty"`
}

// FinanceEntrySummary is a placeholder entry summary for future finance platform connection.
type FinanceEntrySummary struct {
	EntryStatus            PlatformEntryStatus `json:"entry_status"`
	EligibleNow            bool                `json:"eligible_now"`
	SourceReadModelFields  []string            `json:"source_read_model_fields,omitempty"`
	PlaceholderFields      []string            `json:"placeholder_fields,omitempty"`
	FutureFinanceScopeHint []string            `json:"future_finance_scope_hints,omitempty"`
	IsPlaceholder          bool                `json:"is_placeholder"`
	NotReadyBlockingReason string              `json:"not_ready_blocking_reason,omitempty"`
	Note                   string              `json:"note,omitempty"`
}

// ReportEntrySummary is a placeholder entry summary for future report/export platform connection.
type ReportEntrySummary struct {
	EntryStatus            PlatformEntryStatus `json:"entry_status"`
	EligibleNow            bool                `json:"eligible_now"`
	SourceReadModelFields  []string            `json:"source_read_model_fields,omitempty"`
	PlaceholderFields      []string            `json:"placeholder_fields,omitempty"`
	FutureReportScopeHints []string            `json:"future_report_scope_hints,omitempty"`
	IsPlaceholder          bool                `json:"is_placeholder"`
	NotReadyBlockingReason string              `json:"not_ready_blocking_reason,omitempty"`
	Note                   string              `json:"note,omitempty"`
}

// PlatformEntryBoundary is the unified cross-center entry boundary language for KPI/finance/report.
// It is additive and read-model oriented, and does not implement real BI, accounting, or report engines.
type PlatformEntryBoundary struct {
	EntryMode           PlatformEntryMode    `json:"entry_mode"`
	ScopeKey            string               `json:"scope_key"`
	ScopeName           string               `json:"scope_name"`
	SourceObject        string               `json:"source_object"`
	SourceLayer         string               `json:"source_layer"`
	KPIEntrySummary     *KPIEntrySummary     `json:"kpi_entry_summary,omitempty"`
	FinanceEntrySummary *FinanceEntrySummary `json:"finance_entry_summary,omitempty"`
	ReportEntrySummary  *ReportEntrySummary  `json:"report_entry_summary,omitempty"`
	IsPlaceholder       bool                 `json:"is_placeholder"`
	Note                string               `json:"note,omitempty"`
}

func HydrateTaskListItemPlatformEntry(item *TaskListItem) {
	if item == nil {
		return
	}
	financeEligible := item.TaskType == TaskTypePurchaseTask ||
		item.ProcurementSummary != nil ||
		item.OverrideAt != nil ||
		item.ManualCostOverride ||
		item.CostPrice != nil ||
		item.EstimatedCost != nil
	item.PlatformEntryBoundary = buildTaskPlatformEntryBoundary("task_list_item", financeEligible)
}

func HydrateTaskReadModelPlatformEntry(model *TaskReadModel) {
	if model == nil {
		return
	}
	financeEligible := model.TaskType == TaskTypePurchaseTask ||
		model.ProcurementSummary != nil ||
		model.OverrideBoundary != nil
	model.PlatformEntryBoundary = buildTaskPlatformEntryBoundary("task_read_model", financeEligible)
}

func HydrateTaskDetailAggregatePlatformEntry(aggregate *TaskDetailAggregate) {
	if aggregate == nil {
		return
	}
	taskType := TaskType("")
	if aggregate.Task != nil {
		taskType = aggregate.Task.TaskType
	}
	financeEligible := taskType == TaskTypePurchaseTask ||
		aggregate.ProcurementSummary != nil ||
		aggregate.OverrideBoundary != nil
	aggregate.PlatformEntryBoundary = buildTaskPlatformEntryBoundary("task_detail_aggregate", financeEligible)
}

func HydrateProcurementSummaryPlatformEntry(summary *ProcurementSummary) {
	if summary == nil {
		return
	}
	summary.PlatformEntryBoundary = buildProcurementPlatformEntryBoundary()
}

func HydrateTaskCostOverrideBoundaryPlatformEntry(boundary *TaskCostOverrideGovernanceBoundary) {
	if boundary == nil {
		return
	}
	boundary.PlatformEntryBoundary = buildCostOverridePlatformEntryBoundary()
}

func HydrateExportJobPlatformEntry(job *ExportJob) {
	if job == nil {
		return
	}
	financeEligible := exportFinanceEntryEligible(job.SourceQueryType)
	job.PlatformEntryBoundary = buildExportJobPlatformEntryBoundary(job.SourceQueryType, financeEligible)
}

// ClonePlatformEntryBoundary deep-copies the boundary summary for reuse in service-layer read-model cloning.
func ClonePlatformEntryBoundary(boundary *PlatformEntryBoundary) *PlatformEntryBoundary {
	if boundary == nil {
		return nil
	}
	copyBoundary := *boundary
	copyBoundary.KPIEntrySummary = cloneKPIEntrySummary(boundary.KPIEntrySummary)
	copyBoundary.FinanceEntrySummary = cloneFinanceEntrySummary(boundary.FinanceEntrySummary)
	copyBoundary.ReportEntrySummary = cloneReportEntrySummary(boundary.ReportEntrySummary)
	return &copyBoundary
}

func buildTaskPlatformEntryBoundary(sourceObject string, financeEligible bool) *PlatformEntryBoundary {
	return buildPlatformEntryBoundary(
		"task_center",
		"Task Center",
		sourceObject,
		&KPIEntrySummary{
			EntryStatus: platformEntryStatusByEligibility(true),
			EligibleNow: true,
			SourceReadModelFields: []string{
				"task_type",
				"workflow.main_status",
				"workflow.sub_status",
				"deadline_at",
				"updated_at",
			},
			PlaceholderFields: []string{
				"kpi_period_key",
				"kpi_owner_dimension",
				"kpi_scorecard_snapshot_ref",
			},
			FutureMetricHints: []string{
				"task_volume",
				"on_time_rate",
				"rework_rate",
				"warehouse_handoff_efficiency",
			},
			IsPlaceholder: true,
			Note:          "Task center currently exposes KPI-ready source fields only; no real KPI computation is executed in this phase.",
		},
		&FinanceEntrySummary{
			EntryStatus: platformEntryStatusByEligibility(financeEligible),
			EligibleNow: financeEligible,
			SourceReadModelFields: []string{
				"procurement_summary.procurement_price",
				"procurement_summary.cost_price",
				"procurement_summary.estimated_cost",
				"override_governance_boundary.finance_status",
			},
			PlaceholderFields: []string{
				"ledger_entry_ref",
				"reconciliation_status",
				"settlement_batch_ref",
				"invoice_status",
			},
			FutureFinanceScopeHint: []string{
				"task_cost_stat",
				"category_cost_stat",
				"gross_margin_stat",
			},
			IsPlaceholder: true,
			NotReadyBlockingReason: platformEntryBlockingReason(financeEligible,
				"Current task read model has no active procurement/cost-governance finance signal yet."),
			Note: "Task center finance entry remains boundary-only and does not create accounting entries or settlement records.",
		},
		&ReportEntrySummary{
			EntryStatus: platformEntryStatusByEligibility(true),
			EligibleNow: true,
			SourceReadModelFields: []string{
				"task_no",
				"task_type",
				"workflow",
				"procurement_summary",
				"product_selection",
			},
			PlaceholderFields: []string{
				"report_dataset_id",
				"report_refresh_job_ref",
				"report_slice_signature",
			},
			FutureReportScopeHints: []string{
				"ops_report",
				"procurement_report",
				"management_dashboard_export",
			},
			IsPlaceholder: true,
			Note:          "Task center report entry is read-model entry scaffolding only and does not run a real report engine.",
		},
		"Unified task-center entry boundary for future KPI/finance/report platform docking.",
	)
}

func buildProcurementPlatformEntryBoundary() *PlatformEntryBoundary {
	return buildPlatformEntryBoundary(
		"procurement_center",
		"Procurement Summary",
		"procurement_summary",
		&KPIEntrySummary{
			EntryStatus: platformEntryStatusByEligibility(true),
			EligibleNow: true,
			SourceReadModelFields: []string{
				"status",
				"coordination_status",
				"warehouse_prepare_ready",
				"warehouse_receive_ready",
				"expected_delivery_at",
			},
			PlaceholderFields: []string{
				"kpi_procurement_sla_bucket",
				"kpi_supplier_dimension",
			},
			FutureMetricHints: []string{
				"procurement_cycle_time",
				"handoff_timeliness",
				"arrival_delay_rate",
			},
			IsPlaceholder: true,
			Note:          "Procurement KPI entry uses existing coordination read models only; no metric aggregation is executed here.",
		},
		&FinanceEntrySummary{
			EntryStatus: platformEntryStatusByEligibility(true),
			EligibleNow: true,
			SourceReadModelFields: []string{
				"procurement_price",
				"cost_price",
				"estimated_cost",
				"manual_cost_override",
				"override_governance_boundary.finance_status",
			},
			PlaceholderFields: []string{
				"voucher_no",
				"ledger_post_status",
				"settlement_document_ref",
			},
			FutureFinanceScopeHint: []string{
				"procurement_cost_tracking",
				"cost_variance_tracking",
			},
			IsPlaceholder: true,
			Note:          "Procurement finance entry is placeholder boundary only; no ledger posting/reconciliation/settlement is implemented.",
		},
		&ReportEntrySummary{
			EntryStatus: platformEntryStatusByEligibility(true),
			EligibleNow: true,
			SourceReadModelFields: []string{
				"status",
				"coordination_status",
				"supplier_name",
				"quantity",
				"expected_delivery_at",
			},
			PlaceholderFields: []string{
				"report_group_key",
				"report_partition_ref",
			},
			FutureReportScopeHints: []string{
				"procurement_lane_report",
				"warehouse_handoff_report",
			},
			IsPlaceholder: true,
			Note:          "Procurement report entry remains a source-boundary contract and does not generate final reports in this phase.",
		},
		"Unified procurement summary entry boundary for future KPI/finance/report platform docking.",
	)
}

func buildCostOverridePlatformEntryBoundary() *PlatformEntryBoundary {
	return buildPlatformEntryBoundary(
		"cost_governance_center",
		"Cost Governance Boundary",
		"cost_override_boundary",
		&KPIEntrySummary{
			EntryStatus: platformEntryStatusByEligibility(true),
			EligibleNow: true,
			SourceReadModelFields: []string{
				"review_status",
				"finance_status",
				"latest_boundary_actor",
				"latest_boundary_at",
			},
			PlaceholderFields: []string{
				"kpi_override_quality_bucket",
				"kpi_review_cycle_bucket",
			},
			FutureMetricHints: []string{
				"override_frequency",
				"review_turnaround_time",
			},
			IsPlaceholder: true,
			Note:          "Cost-governance KPI entry is a timeline-summary boundary only and does not execute KPI scoring.",
		},
		&FinanceEntrySummary{
			EntryStatus: platformEntryStatusByEligibility(true),
			EligibleNow: true,
			SourceReadModelFields: []string{
				"finance_required",
				"finance_status",
				"finance_view_ready",
				"finance_placeholder_summary",
			},
			PlaceholderFields: []string{
				"accounting_entry_ref",
				"reconciliation_batch_ref",
				"invoice_ref",
			},
			FutureFinanceScopeHint: []string{
				"override_finance_handoff_queue",
				"finance_followup_trace",
			},
			IsPlaceholder: true,
			Note:          "Cost-governance finance entry remains a placeholder boundary and is not a real accounting/reconciliation system.",
		},
		&ReportEntrySummary{
			EntryStatus: platformEntryStatusByEligibility(true),
			EligibleNow: true,
			SourceReadModelFields: []string{
				"governance_boundary_summary",
				"latest_review_action",
				"latest_finance_action",
			},
			PlaceholderFields: []string{
				"governance_report_dataset_ref",
				"governance_report_slice_ref",
			},
			FutureReportScopeHints: []string{
				"override_governance_report",
				"finance_followup_report",
			},
			IsPlaceholder: true,
			Note:          "Cost-governance report entry is boundary scaffolding only and does not include a real report engine.",
		},
		"Unified cost-governance entry boundary for future KPI/finance/report platform docking.",
	)
}

func buildExportJobPlatformEntryBoundary(sourceQueryType ExportSourceQueryType, financeEligible bool) *PlatformEntryBoundary {
	return buildPlatformEntryBoundary(
		"export_center",
		"Export Center",
		"export_job",
		&KPIEntrySummary{
			EntryStatus: platformEntryStatusByEligibility(true),
			EligibleNow: true,
			SourceReadModelFields: []string{
				"source_query_type",
				"status",
				"latest_status_at",
				"attempt_count",
				"dispatch_count",
			},
			PlaceholderFields: []string{
				"kpi_export_sla_bucket",
				"kpi_delivery_channel_dimension",
			},
			FutureMetricHints: []string{
				"export_throughput",
				"export_success_rate",
				"export_turnaround_time",
			},
			IsPlaceholder: true,
			Note:          "Export KPI entry only exposes lifecycle/admission fields and does not implement KPI analytics.",
		},
		&FinanceEntrySummary{
			EntryStatus: platformEntryStatusByEligibility(financeEligible),
			EligibleNow: financeEligible,
			SourceReadModelFields: []string{
				"source_query_type",
				"source_filters",
				"query_template",
				"normalized_filters",
			},
			PlaceholderFields: []string{
				"finance_report_package_ref",
				"finance_distribution_ref",
			},
			FutureFinanceScopeHint: []string{
				"finance_report_export_handoff",
				"cost_margin_report_export_handoff",
			},
			IsPlaceholder: true,
			NotReadyBlockingReason: platformEntryBlockingReason(financeEligible,
				"Current export source is not finance-oriented (`procurement_summary` or `task_query`) in this read model."),
			Note: "Export finance entry remains a source-handoff boundary only and does not produce accounting artifacts.",
		},
		&ReportEntrySummary{
			EntryStatus: platformEntryStatusByEligibility(true),
			EligibleNow: true,
			SourceReadModelFields: []string{
				"source_query_type",
				"source_filters",
				"query_template",
				"normalized_filters",
				"result_ref",
			},
			PlaceholderFields: []string{
				"report_platform_job_ref",
				"report_delivery_ticket_ref",
			},
			FutureReportScopeHints: []string{
				"report_platform_export_handoff",
				"management_report_export_handoff",
			},
			IsPlaceholder: true,
			Note:          "Export report entry is an entry boundary over existing export jobs and not a real report-generation engine.",
		},
		"Unified export-job entry boundary for future KPI/finance/report platform docking.",
	)
}

func buildPlatformEntryBoundary(
	scopeKey string,
	scopeName string,
	sourceObject string,
	kpi *KPIEntrySummary,
	finance *FinanceEntrySummary,
	report *ReportEntrySummary,
	note string,
) *PlatformEntryBoundary {
	return &PlatformEntryBoundary{
		EntryMode:           PlatformEntryModeScaffoldingOnly,
		ScopeKey:            strings.TrimSpace(scopeKey),
		ScopeName:           strings.TrimSpace(scopeName),
		SourceObject:        strings.TrimSpace(sourceObject),
		SourceLayer:         "read_model_projection",
		KPIEntrySummary:     cloneKPIEntrySummary(kpi),
		FinanceEntrySummary: cloneFinanceEntrySummary(finance),
		ReportEntrySummary:  cloneReportEntrySummary(report),
		IsPlaceholder:       true,
		Note:                strings.TrimSpace(note),
	}
}

func platformEntryStatusByEligibility(eligible bool) PlatformEntryStatus {
	if eligible {
		return PlatformEntryStatusCandidate
	}
	return PlatformEntryStatusConditional
}

func platformEntryBlockingReason(eligible bool, reason string) string {
	if eligible {
		return ""
	}
	return strings.TrimSpace(reason)
}

func exportFinanceEntryEligible(sourceQueryType ExportSourceQueryType) bool {
	switch sourceQueryType {
	case ExportSourceQueryTypeProcurementSummary, ExportSourceQueryTypeTaskQuery:
		return true
	default:
		return false
	}
}

func cloneKPIEntrySummary(value *KPIEntrySummary) *KPIEntrySummary {
	if value == nil {
		return nil
	}
	copyValue := *value
	copyValue.SourceReadModelFields = cloneStringSlice(value.SourceReadModelFields)
	copyValue.PlaceholderFields = cloneStringSlice(value.PlaceholderFields)
	copyValue.FutureMetricHints = cloneStringSlice(value.FutureMetricHints)
	return &copyValue
}

func cloneFinanceEntrySummary(value *FinanceEntrySummary) *FinanceEntrySummary {
	if value == nil {
		return nil
	}
	copyValue := *value
	copyValue.SourceReadModelFields = cloneStringSlice(value.SourceReadModelFields)
	copyValue.PlaceholderFields = cloneStringSlice(value.PlaceholderFields)
	copyValue.FutureFinanceScopeHint = cloneStringSlice(value.FutureFinanceScopeHint)
	return &copyValue
}

func cloneReportEntrySummary(value *ReportEntrySummary) *ReportEntrySummary {
	if value == nil {
		return nil
	}
	copyValue := *value
	copyValue.SourceReadModelFields = cloneStringSlice(value.SourceReadModelFields)
	copyValue.PlaceholderFields = cloneStringSlice(value.PlaceholderFields)
	copyValue.FutureReportScopeHints = cloneStringSlice(value.FutureReportScopeHints)
	return &copyValue
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	out = append(out, values...)
	return out
}
