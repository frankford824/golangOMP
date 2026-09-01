package domain

import (
	"encoding/json"
	"time"
)

type CostRuleBinding struct {
	ID              int64     `db:"id" json:"id"`
	IIDRaw          string    `db:"i_id_raw" json:"i_id_raw"`
	NormalizedIID   string    `db:"normalized_i_id" json:"normalized_i_id"`
	RuleGroup       string    `db:"rule_group" json:"rule_group"`
	DisplayName     string    `db:"display_name" json:"display_name"`
	Source          string    `db:"source" json:"source"`
	IsActive        bool      `db:"is_active" json:"is_active"`
	CreatedBy       *int64    `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy       *int64    `db:"updated_by" json:"updated_by,omitempty"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
	ActiveRuleCount int64     `db:"-" json:"active_rule_count,omitempty"`
}

type CostRuleBindingPatch struct {
	ID          int64
	IIDRaw      *string
	RuleGroup   *string
	DisplayName *string
	Source      *string
	IsActive    *bool
	UpdatedBy   *int64
}

type CostRuleBindingListResponse struct {
	Data       []*CostRuleBinding `json:"data"`
	Pagination PaginationMeta     `json:"pagination"`
}

type UnboundCostRuleCandidate struct {
	ERPProductIID        string   `json:"erp_i_id,omitempty"`
	ProductIID           string   `json:"product_i_id,omitempty"`
	NormalizedIID        string   `json:"normalized_i_id"`
	SuggestedRuleGroup   string   `json:"suggested_rule_group,omitempty"`
	SuggestedRuleGroups  []string `json:"suggested_rule_groups"`
	SuggestedGroupCount  int64    `json:"suggested_group_count"`
	MappingConfidence    string   `json:"mapping_confidence"`
	SuggestedDisplayName string   `json:"suggested_display_name,omitempty"`
	MatchCount           int64    `json:"match_count"`
	ExampleSKUCode       string   `json:"example_sku_code,omitempty"`
	ExampleTaskNo        string   `json:"example_task_no,omitempty"`
	AverageCostPrice     *float64 `json:"average_cost_price,omitempty"`
}

type ProductCostReconciliationStatus string

const (
	ProductCostReconciliationMatched       ProductCostReconciliationStatus = "matched"
	ProductCostReconciliationMismatch      ProductCostReconciliationStatus = "mismatched"
	ProductCostReconciliationSystemMissing ProductCostReconciliationStatus = "system_missing"
	ProductCostReconciliationERPMissing    ProductCostReconciliationStatus = "erp_missing"
	ProductCostReconciliationUnavailable   ProductCostReconciliationStatus = "unavailable"
)

// ProductCostReconciliation keeps the system rule result and the live ERP
// observation separate. A readback never silently overwrites either source.
type ProductCostReconciliation struct {
	ProductManagementRecordID int64                           `json:"product_management_record_id"`
	SKUCode                   string                          `json:"sku_code"`
	SystemCostPrice           *float64                        `json:"system_cost_price,omitempty"`
	ERPCostPrice              *float64                        `json:"erp_cost_price,omitempty"`
	CostDelta                 *float64                        `json:"cost_delta,omitempty"`
	Status                    ProductCostReconciliationStatus `json:"status"`
	CheckedAt                 time.Time                       `json:"checked_at"`
	Message                   string                          `json:"message"`
	SystemTrace               *ProductManagementCostTrace     `json:"system_trace,omitempty"`
	ERPProductIID             string                          `json:"erp_i_id,omitempty"`
	ERPProductName            string                          `json:"erp_product_name,omitempty"`
}

type ProductCostDashboardResponse struct {
	TotalRecords          int64                                `json:"total_records,omitempty"`
	IssueTotal            int64                                `json:"total_count"`
	LegacyFallbackCount   int64                                `json:"unbound_iid_count,omitempty"`
	LegacyFallbackRatio   float64                              `json:"legacy_fallback_ratio,omitempty"`
	LegacyFallbackEnabled bool                                 `json:"legacy_fallback_enabled"`
	LegacyFallbackMode    string                               `json:"legacy_fallback_mode,omitempty"`
	LegacyFallbackWarning string                               `json:"legacy_fallback_warning,omitempty"`
	LegacyFallbackTrend   []ProductCostLegacyFallbackTrendItem `json:"legacy_fallback_trend,omitempty"`
	Groups                []ProductCostIssueGroup              `json:"groups"`
	Tags                  []ProductCostIssueTag                `json:"tags"`
	GeneratedAt           time.Time                            `json:"generated_at"`
}

type ProductCostLegacyFallbackTrendItem struct {
	Date                string  `json:"date"`
	TotalRecords        int64   `json:"total_records"`
	LegacyFallbackCount int64   `json:"unbound_iid_count"`
	LegacyFallbackRatio float64 `json:"legacy_fallback_ratio"`
}

type ProductCostIssueGroup struct {
	Key   string `json:"code"`
	Label string `json:"label"`
	Count int64  `json:"count"`
}

type ProductCostIssueTag struct {
	Key     string `json:"code"`
	Label   string `json:"label"`
	Group   string `json:"group"`
	Count   int64  `json:"count"`
	Tooltip string `json:"tooltip,omitempty"`
}

type CostRuleMatchMode string

const (
	CostRuleMatchModeBindingERPIID     CostRuleMatchMode = "binding_erp_i_id"
	CostRuleMatchModeBindingProductIID CostRuleMatchMode = "binding_product_i_id"
	CostRuleMatchModeLegacyAlias       CostRuleMatchMode = "legacy_alias"
	CostRuleMatchModeNoMatch           CostRuleMatchMode = "no_match"
)

type CostRuleMatchTrace struct {
	MatchMode           CostRuleMatchMode `json:"match_mode,omitempty"`
	ERPIID              string            `json:"erp_i_id,omitempty"`
	ProductIID          string            `json:"product_i_id,omitempty"`
	NormalizedIID       string            `json:"normalized_i_id,omitempty"`
	RuleGroup           string            `json:"rule_group,omitempty"`
	LegacyAliasFallback bool              `json:"legacy_alias_fallback"`
}

func (t *CostRuleMatchTrace) Clone() *CostRuleMatchTrace {
	if t == nil {
		return nil
	}
	copied := *t
	return &copied
}

type CostRecalculationRunMode string

const (
	CostRecalculationRunModeSingle      CostRecalculationRunMode = "single"
	CostRecalculationRunModeExplicit    CostRecalculationRunMode = "explicit"
	CostRecalculationRunModeAllMatching CostRecalculationRunMode = "all_matching"
)

type CostRecalculationRunStatus string

const (
	CostRunStatusPreviewing         CostRecalculationRunStatus = "previewing"
	CostRunStatusPreviewed          CostRecalculationRunStatus = "previewed"
	CostRunStatusPreviewFailed      CostRecalculationRunStatus = "preview_failed"
	CostRunStatusApplying           CostRecalculationRunStatus = "applying"
	CostRunStatusApplied            CostRecalculationRunStatus = "applied"
	CostRunStatusPartiallyApplied   CostRecalculationRunStatus = "partially_applied"
	CostRunStatusApplyFailed        CostRecalculationRunStatus = "apply_failed"
	CostRunStatusERPSyncing         CostRecalculationRunStatus = "erp_syncing"
	CostRunStatusERPSynced          CostRecalculationRunStatus = "erp_synced"
	CostRunStatusPartiallyERPSynced CostRecalculationRunStatus = "partially_erp_synced"
	CostRunStatusCancelled          CostRecalculationRunStatus = "cancelled"
)

func (s CostRecalculationRunStatus) IsOpen() bool {
	switch s {
	case CostRunStatusPreviewing,
		CostRunStatusPreviewed,
		CostRunStatusApplying,
		CostRunStatusApplied,
		CostRunStatusPartiallyApplied,
		CostRunStatusERPSyncing:
		return true
	default:
		return false
	}
}

type CostRecalculationRunItemStatus string

const (
	CostRunItemStatusPreviewed CostRecalculationRunItemStatus = "previewed"
	CostRunItemStatusApplied   CostRecalculationRunItemStatus = "applied"
	CostRunItemStatusSkipped   CostRecalculationRunItemStatus = "skipped"
	CostRunItemStatusConflict  CostRecalculationRunItemStatus = "conflict"
	CostRunItemStatusFailed    CostRecalculationRunItemStatus = "failed"
	CostRunItemStatusERPQueued CostRecalculationRunItemStatus = "erp_queued"
	CostRunItemStatusERPSynced CostRecalculationRunItemStatus = "erp_synced"
	CostRunItemStatusERPFailed CostRecalculationRunItemStatus = "erp_failed"
)

type CreateCostRecalculationRunRequest struct {
	Mode                      string                      `json:"mode"`
	ProductManagementRecordID int64                       `json:"product_management_record_id,omitempty"`
	RecordIDs                 []int64                     `json:"record_ids,omitempty"`
	SKUCodes                  []string                    `json:"sku_codes,omitempty"`
	Filters                   ProductManagementCostFilter `json:"filters,omitempty"`
	IssueGroup                string                      `json:"issue_group,omitempty"`
	IssueTag                  string                      `json:"issue_tag,omitempty"`
	SyncERP                   bool                        `json:"sync_erp,omitempty"`
	ForceManual               bool                        `json:"force_manual,omitempty"`
	Reason                    string                      `json:"reason,omitempty"`
	Description               string                      `json:"description,omitempty"`
}

type ProductManagementCostFilter struct {
	Keyword   string `json:"keyword,omitempty"`
	GroupKey  string `json:"group_key,omitempty"`
	TagKey    string `json:"tag_key,omitempty"`
	RuleGroup string `json:"rule_group,omitempty"`
	PageSize  int    `json:"page_size,omitempty"`
}

type CostRecalculationRun struct {
	ID          int64                        `db:"id" json:"id"`
	RunNo       string                       `db:"run_no" json:"run_no"`
	Status      CostRecalculationRunStatus   `db:"status" json:"status"`
	Mode        string                       `db:"mode" json:"mode"`
	FiltersJSON json.RawMessage              `db:"filters_json" json:"filters_json,omitempty"`
	SummaryJSON json.RawMessage              `db:"summary_json" json:"summary_json,omitempty"`
	CreatedBy   *int64                       `db:"created_by" json:"created_by,omitempty"`
	AppliedBy   *int64                       `db:"applied_by" json:"applied_by,omitempty"`
	ERPSyncedBy *int64                       `db:"erp_synced_by" json:"erp_synced_by,omitempty"`
	CreatedAt   time.Time                    `db:"created_at" json:"created_at"`
	PreviewedAt *time.Time                   `db:"previewed_at" json:"previewed_at,omitempty"`
	AppliedAt   *time.Time                   `db:"applied_at" json:"applied_at,omitempty"`
	ERPSyncedAt *time.Time                   `db:"erp_synced_at" json:"erp_synced_at,omitempty"`
	CancelledAt *time.Time                   `db:"cancelled_at" json:"cancelled_at,omitempty"`
	UpdatedAt   time.Time                    `db:"updated_at" json:"updated_at"`
	Summary     *CostRecalculationRunSummary `db:"-" json:"summary,omitempty"`
	Items       []*CostRecalculationRunItem  `db:"-" json:"items,omitempty"`
}

type CostRecalculationRunSummary struct {
	TotalCount       int64  `json:"total_count"`
	PreviewedCount   int64  `json:"previewed_count"`
	AppliedCount     int64  `json:"applied_count"`
	SkippedCount     int64  `json:"skipped_count"`
	ConflictCount    int64  `json:"conflict_count"`
	FailedCount      int64  `json:"failed_count"`
	ERPQueuedCount   int64  `json:"erp_queued_count"`
	ERPSyncedCount   int64  `json:"erp_synced_count"`
	ERPFailedCount   int64  `json:"erp_failed_count"`
	ERPSyncableCount int64  `json:"erp_syncable_count"`
	TaskCount        int64  `json:"task_count"`
	ConfirmMessage   string `json:"confirm_message"`
	ConfirmationText string `json:"confirmation_text,omitempty"`
	ERPSyncMessage   string `json:"erp_sync_message,omitempty"`
}

type CostRecalculationRunItem struct {
	ID                        int64                          `db:"id" json:"id"`
	RunID                     int64                          `db:"run_id" json:"run_id"`
	ProductManagementRecordID int64                          `db:"product_management_record_id" json:"product_management_record_id"`
	TaskID                    *int64                         `db:"task_id" json:"task_id,omitempty"`
	TaskNo                    string                         `db:"task_no" json:"task_no"`
	TaskSKUItemID             *int64                         `db:"task_sku_item_id" json:"task_sku_item_id,omitempty"`
	SKUCode                   string                         `db:"sku_code" json:"sku_code"`
	ERPIID                    string                         `db:"erp_i_id" json:"erp_i_id,omitempty"`
	ProductIID                string                         `db:"product_i_id" json:"product_i_id,omitempty"`
	NormalizedIID             string                         `db:"normalized_i_id" json:"normalized_i_id,omitempty"`
	OldCostPrice              *float64                       `db:"old_cost_price" json:"old_cost_price,omitempty"`
	NewCostPrice              *float64                       `db:"new_cost_price" json:"new_cost_price,omitempty"`
	CostDelta                 *float64                       `db:"cost_delta" json:"cost_delta,omitempty"`
	OldRuleID                 *int64                         `db:"old_rule_id" json:"old_rule_id,omitempty"`
	NewRuleID                 *int64                         `db:"new_rule_id" json:"new_rule_id,omitempty"`
	NewRuleVersion            *int                           `db:"new_rule_version" json:"new_rule_version,omitempty"`
	MatchMode                 string                         `db:"match_mode" json:"match_mode,omitempty"`
	Status                    CostRecalculationRunItemStatus `db:"status" json:"status"`
	SkipReason                string                         `db:"skip_reason" json:"skip_reason,omitempty"`
	ConflictReason            string                         `db:"conflict_reason" json:"conflict_reason,omitempty"`
	PreviewSnapshotJSON       json.RawMessage                `db:"preview_snapshot_json" json:"preview_snapshot_json,omitempty"`
	ApplySnapshotJSON         json.RawMessage                `db:"apply_snapshot_json" json:"apply_snapshot_json,omitempty"`
	CreatedAt                 time.Time                      `db:"created_at" json:"created_at"`
	UpdatedAt                 time.Time                      `db:"updated_at" json:"updated_at"`
}

type ApplyCostRecalculationRunResponse struct {
	Run     *CostRecalculationRun       `json:"run"`
	Summary CostRecalculationRunSummary `json:"summary"`
}

type SyncCostRecalculationRunERPResponse struct {
	Run     *CostRecalculationRun       `json:"run"`
	Summary CostRecalculationRunSummary `json:"summary"`
}
