package domain

import (
	"strings"
	"time"
)

type ExportType string

const (
	ExportTypeTaskList           ExportType = "task_list"
	ExportTypeTaskBoardQueue     ExportType = "task_board_queue"
	ExportTypeProcurementSummary ExportType = "procurement_summary"
	ExportTypeWarehouseReceipts  ExportType = "warehouse_receipts"
)

func (t ExportType) Valid() bool {
	switch t {
	case ExportTypeTaskList, ExportTypeTaskBoardQueue, ExportTypeProcurementSummary, ExportTypeWarehouseReceipts:
		return true
	default:
		return false
	}
}

type ExportSourceQueryType string

const (
	ExportSourceQueryTypeTaskQuery          ExportSourceQueryType = "task_query"
	ExportSourceQueryTypeTaskBoardQueue     ExportSourceQueryType = "task_board_queue"
	ExportSourceQueryTypeProcurementSummary ExportSourceQueryType = "procurement_summary"
	ExportSourceQueryTypeWarehouseReceipts  ExportSourceQueryType = "warehouse_receipts"
)

func (t ExportSourceQueryType) Valid() bool {
	switch t {
	case ExportSourceQueryTypeTaskQuery, ExportSourceQueryTypeTaskBoardQueue, ExportSourceQueryTypeProcurementSummary, ExportSourceQueryTypeWarehouseReceipts:
		return true
	default:
		return false
	}
}

type ExportJobStatus string

const (
	ExportJobStatusQueued    ExportJobStatus = "queued"
	ExportJobStatusRunning   ExportJobStatus = "running"
	ExportJobStatusReady     ExportJobStatus = "ready"
	ExportJobStatusFailed    ExportJobStatus = "failed"
	ExportJobStatusCancelled ExportJobStatus = "cancelled"
)

func (s ExportJobStatus) Valid() bool {
	switch s {
	case ExportJobStatusQueued, ExportJobStatusRunning, ExportJobStatusReady, ExportJobStatusFailed, ExportJobStatusCancelled:
		return true
	default:
		return false
	}
}

type ExportJobProgressHint string

const (
	ExportJobProgressHintCreated       ExportJobProgressHint = "created"
	ExportJobProgressHintProcessing    ExportJobProgressHint = "processing"
	ExportJobProgressHintDownloadReady ExportJobProgressHint = "download_ready"
	ExportJobProgressHintFailed        ExportJobProgressHint = "failed"
	ExportJobProgressHintCancelled     ExportJobProgressHint = "cancelled"
)

type ExportJobAdmissionDecisionType string

const (
	ExportJobAdmissionDecisionTypeDispatch   ExportJobAdmissionDecisionType = "dispatch"
	ExportJobAdmissionDecisionTypeRedispatch ExportJobAdmissionDecisionType = "redispatch"
	ExportJobAdmissionDecisionTypeStart      ExportJobAdmissionDecisionType = "start"
	ExportJobAdmissionDecisionTypeAttempt    ExportJobAdmissionDecisionType = "attempt"
)

const (
	ExportJobAdmissionReasonQueuedWithoutDispatch                           = "queued_without_dispatch"
	ExportJobAdmissionReasonNoHistoricalDispatch                            = "no_historical_dispatch"
	ExportJobAdmissionReasonRunningDispatchBlocked                          = "job_running_dispatch_blocked"
	ExportJobAdmissionReasonReadyDispatchBlocked                            = "job_ready_dispatch_blocked"
	ExportJobAdmissionReasonFailedDispatchBlocked                           = "job_failed_dispatch_blocked_requeue_required"
	ExportJobAdmissionReasonCancelledDispatchBlocked                        = "job_cancelled_dispatch_blocked_requeue_required"
	ExportJobAdmissionReasonUnknownDispatchBlocked                          = "job_not_queued_dispatch_blocked"
	ExportJobAdmissionReasonLatestDispatchSubmittedPendingResolution        = "latest_dispatch_submitted_pending_resolution"
	ExportJobAdmissionReasonLatestDispatchReceivedPendingStartOrResolution  = "latest_dispatch_received_pending_start_or_resolution"
	ExportJobAdmissionReasonLatestDispatchRejectedRedispatchAllowed         = "latest_dispatch_rejected_redispatch_allowed"
	ExportJobAdmissionReasonLatestDispatchExpiredRedispatchAllowed          = "latest_dispatch_expired_redispatch_allowed"
	ExportJobAdmissionReasonLatestDispatchNotExecutedRedispatchAllowed      = "latest_dispatch_not_executed_redispatch_allowed"
	ExportJobAdmissionReasonLatestDispatchUnknownStatus                     = "latest_dispatch_unknown_status"
	ExportJobAdmissionReasonRunningStartBlocked                             = "job_running_start_blocked"
	ExportJobAdmissionReasonReadyStartBlocked                               = "job_ready_start_blocked"
	ExportJobAdmissionReasonFailedStartBlocked                              = "job_failed_start_blocked_requeue_required"
	ExportJobAdmissionReasonCancelledStartBlocked                           = "job_cancelled_start_blocked_requeue_required"
	ExportJobAdmissionReasonUnknownStartBlocked                             = "job_not_queued_start_blocked"
	ExportJobAdmissionReasonLatestDispatchReceivedStartAllowed              = "latest_dispatch_received_start_allowed"
	ExportJobAdmissionReasonNoDispatchAutoPlaceholderAllowed                = "no_dispatch_auto_placeholder_allowed"
	ExportJobAdmissionReasonLatestDispatchRejectedAutoPlaceholderAllowed    = "latest_dispatch_rejected_auto_placeholder_allowed"
	ExportJobAdmissionReasonLatestDispatchExpiredAutoPlaceholderAllowed     = "latest_dispatch_expired_auto_placeholder_allowed"
	ExportJobAdmissionReasonLatestDispatchNotExecutedAutoPlaceholderAllowed = "latest_dispatch_not_executed_auto_placeholder_allowed"
	ExportJobAdmissionReasonRunningAttemptBlocked                           = "job_running_attempt_blocked"
	ExportJobAdmissionReasonReadyAttemptBlocked                             = "job_ready_attempt_blocked"
	ExportJobAdmissionReasonFailedAttemptBlocked                            = "job_failed_attempt_blocked_requeue_required"
	ExportJobAdmissionReasonCancelledAttemptBlocked                         = "job_cancelled_attempt_blocked_requeue_required"
	ExportJobAdmissionReasonUnknownAttemptBlocked                           = "job_not_queued_attempt_blocked"
)

const (
	ExportJobDispatchStartAdmissionReasonReceivedStartAdmitted         = "dispatch_received_start_admitted"
	ExportJobDispatchStartAdmissionReasonSubmittedPending              = "dispatch_submitted_pending_receive"
	ExportJobDispatchStartAdmissionReasonRejectedRequiresRedispatch    = "dispatch_rejected_requires_redispatch"
	ExportJobDispatchStartAdmissionReasonExpiredRequiresRedispatch     = "dispatch_expired_requires_redispatch"
	ExportJobDispatchStartAdmissionReasonNotExecutedRequiresRedispatch = "dispatch_not_executed_requires_redispatch"
	ExportJobDispatchStartAdmissionReasonUnknown                       = "dispatch_status_unknown"
)

type ExportJobStartMode string

const (
	ExportJobStartModeExplicitInternal ExportJobStartMode = "explicit_internal_start"
)

type ExportJobExecutionMode string

const (
	ExportJobExecutionModeManualPlaceholderRunner ExportJobExecutionMode = "manual_placeholder_runner"
)

type ExportJobAdapterMode string

const (
	ExportJobAdapterModeDispatchThenAttempt ExportJobAdapterMode = "dispatch_then_attempt"
)

type ExportJobStorageMode string

const (
	ExportJobStorageModeLifecycleManagedResultRef ExportJobStorageMode = "lifecycle_managed_result_ref"
)

type ExportJobDeliveryMode string

const (
	ExportJobDeliveryModeClaimReadRefreshHandoff ExportJobDeliveryMode = "claim_read_refresh_handoff"
)

type ExportJobExecutionBoundary struct {
	BoundaryKey              string `json:"boundary_key"`
	StartLayer               string `json:"start_layer"`
	DispatchLayer            string `json:"dispatch_layer"`
	AttemptLayer             string `json:"attempt_layer"`
	ResultGenerationLayer    string `json:"result_generation_layer"`
	FutureRunnerReplaceLayer string `json:"future_runner_replace_layer"`
	Placeholder              bool   `json:"placeholder"`
	Note                     string `json:"note,omitempty"`
}

type ExportJobStorageBoundary struct {
	BoundaryKey               string `json:"boundary_key"`
	ResultSourceLayer         string `json:"result_source_layer"`
	StorageLayer              string `json:"storage_layer"`
	StorageRefField           string `json:"storage_ref_field"`
	FutureStorageReplaceLayer string `json:"future_storage_replace_layer"`
	Placeholder               bool   `json:"placeholder"`
	Note                      string `json:"note,omitempty"`
}

type ExportJobDeliveryBoundary struct {
	BoundaryKey                string `json:"boundary_key"`
	DeliveryLayer              string `json:"delivery_layer"`
	DeliveryRefField           string `json:"delivery_ref_field"`
	ClaimAction                string `json:"claim_action"`
	ReadAction                 string `json:"read_action"`
	RefreshAction              string `json:"refresh_action"`
	FutureDeliveryReplaceLayer string `json:"future_delivery_replace_layer"`
	Placeholder                bool   `json:"placeholder"`
	Note                       string `json:"note,omitempty"`
}

type ExportJobAdvanceAction string

const (
	ExportJobAdvanceActionStart     ExportJobAdvanceAction = "start"
	ExportJobAdvanceActionMarkReady ExportJobAdvanceAction = "mark_ready"
	ExportJobAdvanceActionFail      ExportJobAdvanceAction = "fail"
	ExportJobAdvanceActionCancel    ExportJobAdvanceAction = "cancel"
	ExportJobAdvanceActionRequeue   ExportJobAdvanceAction = "requeue"
)

func (a ExportJobAdvanceAction) Valid() bool {
	switch a {
	case ExportJobAdvanceActionStart, ExportJobAdvanceActionMarkReady, ExportJobAdvanceActionFail, ExportJobAdvanceActionCancel, ExportJobAdvanceActionRequeue:
		return true
	default:
		return false
	}
}

// ExportSourceFilters keeps only the source-context fields that are not already
// represented by task-query `query_template` / `normalized_filters`.
type ExportSourceFilters struct {
	QueueKey   string        `json:"queue_key,omitempty"`
	BoardView  TaskBoardView `json:"board_view,omitempty"`
	TaskID     *int64        `json:"task_id,omitempty"`
	ReceiverID *int64        `json:"receiver_id,omitempty"`
	Status     string        `json:"status,omitempty"`
}

type ExportResultRef struct {
	RefType       string     `json:"ref_type"`
	RefKey        string     `json:"ref_key"`
	FileName      string     `json:"file_name,omitempty"`
	MimeType      string     `json:"mime_type,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	IsPlaceholder bool       `json:"is_placeholder"`
	Note          string     `json:"note,omitempty"`
}

type ExportJobDownloadHandoff struct {
	ExportJobID         int64            `json:"export_job_id"`
	Status              ExportJobStatus  `json:"status"`
	DownloadReady       bool             `json:"download_ready"`
	ClaimAvailable      bool             `json:"claim_available"`
	ReadAvailable       bool             `json:"read_available"`
	IsExpired           bool             `json:"is_expired"`
	CanRefresh          bool             `json:"can_refresh"`
	ResultRef           *ExportResultRef `json:"result_ref,omitempty"`
	FileName            string           `json:"file_name,omitempty"`
	MimeType            string           `json:"mime_type,omitempty"`
	IsPlaceholder       bool             `json:"is_placeholder"`
	ExpiresAt           *time.Time       `json:"expires_at,omitempty"`
	Note                string           `json:"note,omitempty"`
	ClaimedAt           *time.Time       `json:"claimed_at,omitempty"`
	ClaimedByActorID    *int64           `json:"claimed_by_actor_id,omitempty"`
	ClaimedByActorType  string           `json:"claimed_by_actor_type,omitempty"`
	LastReadAt          *time.Time       `json:"last_read_at,omitempty"`
	LastReadByActorID   *int64           `json:"last_read_by_actor_id,omitempty"`
	LastReadByActorType string           `json:"last_read_by_actor_type,omitempty"`
}

type ExportTemplate struct {
	Key                       string                  `json:"key"`
	Name                      string                  `json:"name"`
	Description               string                  `json:"description,omitempty"`
	ExportType                ExportType              `json:"export_type"`
	SupportedSourceQueryTypes []ExportSourceQueryType `json:"supported_source_query_types"`
	ResultFormat              string                  `json:"result_format"`
	PlaceholderOnly           bool                    `json:"placeholder_only"`
}

type ExportJobAdmissionDecision struct {
	DecisionType         ExportJobAdmissionDecisionType `json:"decision_type"`
	Allowed              bool                           `json:"allowed"`
	Reason               string                         `json:"reason"`
	JobStatus            ExportJobStatus                `json:"job_status"`
	LatestDispatchStatus *ExportJobDispatchStatus       `json:"latest_dispatch_status,omitempty"`
	LatestAttemptStatus  *ExportJobAttemptStatus        `json:"latest_attempt_status,omitempty"`
}

type ExportJob struct {
	ExportJobID             int64                       `json:"export_job_id"`
	TemplateKey             string                      `json:"template_key"`
	ExportType              ExportType                  `json:"export_type"`
	SourceQueryType         ExportSourceQueryType       `json:"source_query_type"`
	SourceFilters           ExportSourceFilters         `json:"source_filters"`
	NormalizedFilters       *TaskQueryFilterDefinition  `json:"normalized_filters,omitempty"`
	QueryTemplate           *TaskQueryTemplate          `json:"query_template,omitempty"`
	RequestedBy             RequestActor                `json:"requested_by"`
	Status                  ExportJobStatus             `json:"status"`
	ProgressHint            ExportJobProgressHint       `json:"progress_hint"`
	LatestStatusAt          time.Time                   `json:"latest_status_at"`
	DownloadReady           bool                        `json:"download_ready"`
	CanStart                bool                        `json:"can_start"`
	CanStartReason          string                      `json:"can_start_reason,omitempty"`
	CanAttempt              bool                        `json:"can_attempt"`
	CanAttemptReason        string                      `json:"can_attempt_reason,omitempty"`
	CanRetry                bool                        `json:"can_retry"`
	CanDispatch             bool                        `json:"can_dispatch"`
	CanDispatchReason       string                      `json:"can_dispatch_reason,omitempty"`
	CanRedispatch           bool                        `json:"can_redispatch"`
	CanRedispatchReason     string                      `json:"can_redispatch_reason,omitempty"`
	DispatchabilityReason   string                      `json:"dispatchability_reason,omitempty"`
	AttemptabilityReason    string                      `json:"attemptability_reason,omitempty"`
	LatestAdmissionDecision *ExportJobAdmissionDecision `json:"latest_admission_decision,omitempty"`
	StartMode               ExportJobStartMode          `json:"start_mode"`
	ExecutionMode           ExportJobExecutionMode      `json:"execution_mode"`
	AdapterMode             ExportJobAdapterMode        `json:"adapter_mode"`
	DispatchMode            BoundaryDispatchMode        `json:"dispatch_mode"`
	StorageMode             ExportJobStorageMode        `json:"storage_mode"`
	DeliveryMode            ExportJobDeliveryMode       `json:"delivery_mode"`
	AdapterRefSummary       *AdapterRefSummary          `json:"adapter_ref_summary,omitempty"`
	ResourceRefSummary      *ResourceRefSummary         `json:"resource_ref_summary,omitempty"`
	HandoffRefSummary       *HandoffRefSummary          `json:"handoff_ref_summary,omitempty"`
	ExecutionBoundary       ExportJobExecutionBoundary  `json:"execution_boundary"`
	StorageBoundary         ExportJobStorageBoundary    `json:"storage_boundary"`
	DeliveryBoundary        ExportJobDeliveryBoundary   `json:"delivery_boundary"`
	IsExpired               bool                        `json:"is_expired"`
	CanRefresh              bool                        `json:"can_refresh"`
	ResultRef               *ExportResultRef            `json:"result_ref,omitempty"`
	DispatchCount           int64                       `json:"dispatch_count"`
	LatestDispatch          *ExportJobDispatch          `json:"latest_dispatch,omitempty"`
	AttemptCount            int64                       `json:"attempt_count"`
	LatestAttempt           *ExportJobAttempt           `json:"latest_attempt,omitempty"`
	EventCount              int64                       `json:"event_count"`
	LatestEvent             *ExportJobEventSummary      `json:"latest_event,omitempty"`
	LatestDispatchEvent     *ExportJobEventSummary      `json:"latest_dispatch_event,omitempty"`
	LatestRunnerEvent       *ExportJobEventSummary      `json:"latest_runner_event,omitempty"`
	PolicyMode              PolicyMode                  `json:"policy_mode,omitempty"`
	VisibleToRoles          []Role                      `json:"visible_to_roles,omitempty"`
	ActionRoles             []ActionPolicySummary       `json:"action_roles,omitempty"`
	PolicyScopeSummary      *PolicyScopeSummary         `json:"policy_scope_summary,omitempty"`
	PlatformEntryBoundary   *PlatformEntryBoundary      `json:"platform_entry_boundary,omitempty"`
	Remark                  string                      `json:"remark"`
	CreatedAt               time.Time                   `json:"created_at"`
	FinishedAt              *time.Time                  `json:"finished_at,omitempty"`
	UpdatedAt               time.Time                   `json:"updated_at"`
}

func ExportJobProgressForStatus(status ExportJobStatus) ExportJobProgressHint {
	switch status {
	case ExportJobStatusRunning:
		return ExportJobProgressHintProcessing
	case ExportJobStatusReady:
		return ExportJobProgressHintDownloadReady
	case ExportJobStatusFailed:
		return ExportJobProgressHintFailed
	case ExportJobStatusCancelled:
		return ExportJobProgressHintCancelled
	default:
		return ExportJobProgressHintCreated
	}
}

func ExportJobDownloadReady(status ExportJobStatus, resultRef *ExportResultRef) bool {
	return status == ExportJobStatusReady && resultRef != nil && strings.TrimSpace(resultRef.RefKey) != ""
}

func ExportJobStartAdmission(status ExportJobStatus, latestDispatch *ExportJobDispatch) (bool, string) {
	if status != ExportJobStatusQueued {
		switch status {
		case ExportJobStatusRunning:
			return false, ExportJobAdmissionReasonRunningStartBlocked
		case ExportJobStatusReady:
			return false, ExportJobAdmissionReasonReadyStartBlocked
		case ExportJobStatusFailed:
			return false, ExportJobAdmissionReasonFailedStartBlocked
		case ExportJobStatusCancelled:
			return false, ExportJobAdmissionReasonCancelledStartBlocked
		default:
			return false, ExportJobAdmissionReasonUnknownStartBlocked
		}
	}
	if latestDispatch == nil {
		return true, ExportJobAdmissionReasonNoDispatchAutoPlaceholderAllowed
	}
	switch latestDispatch.Status {
	case ExportJobDispatchStatusSubmitted:
		return false, ExportJobAdmissionReasonLatestDispatchSubmittedPendingResolution
	case ExportJobDispatchStatusReceived:
		return true, ExportJobAdmissionReasonLatestDispatchReceivedStartAllowed
	case ExportJobDispatchStatusRejected:
		return true, ExportJobAdmissionReasonLatestDispatchRejectedAutoPlaceholderAllowed
	case ExportJobDispatchStatusExpired:
		return true, ExportJobAdmissionReasonLatestDispatchExpiredAutoPlaceholderAllowed
	case ExportJobDispatchStatusNotExecuted:
		return true, ExportJobAdmissionReasonLatestDispatchNotExecutedAutoPlaceholderAllowed
	default:
		return false, ExportJobAdmissionReasonLatestDispatchUnknownStatus
	}
}

func ExportJobCanStart(status ExportJobStatus, latestDispatch *ExportJobDispatch) bool {
	allowed, _ := ExportJobStartAdmission(status, latestDispatch)
	return allowed
}

func ExportJobAttemptAdmission(status ExportJobStatus, latestDispatch *ExportJobDispatch) (bool, string) {
	if status != ExportJobStatusQueued {
		switch status {
		case ExportJobStatusRunning:
			return false, ExportJobAdmissionReasonRunningAttemptBlocked
		case ExportJobStatusReady:
			return false, ExportJobAdmissionReasonReadyAttemptBlocked
		case ExportJobStatusFailed:
			return false, ExportJobAdmissionReasonFailedAttemptBlocked
		case ExportJobStatusCancelled:
			return false, ExportJobAdmissionReasonCancelledAttemptBlocked
		default:
			return false, ExportJobAdmissionReasonUnknownAttemptBlocked
		}
	}
	return ExportJobStartAdmission(status, latestDispatch)
}

func ExportJobCanAttempt(status ExportJobStatus, latestDispatch *ExportJobDispatch) bool {
	allowed, _ := ExportJobAttemptAdmission(status, latestDispatch)
	return allowed
}

func ExportJobCanRetry(status ExportJobStatus, attemptCount int64) bool {
	return status == ExportJobStatusQueued && attemptCount > 0
}

func ExportJobDownloadExpired(status ExportJobStatus, resultRef *ExportResultRef, now time.Time) bool {
	if !ExportJobDownloadReady(status, resultRef) || resultRef == nil || resultRef.ExpiresAt == nil {
		return false
	}
	return !resultRef.ExpiresAt.UTC().After(now.UTC())
}

func ExportJobCanAccessDownload(status ExportJobStatus, resultRef *ExportResultRef, now time.Time) bool {
	return ExportJobDownloadReady(status, resultRef) && !ExportJobDownloadExpired(status, resultRef, now)
}

func ExportJobCanRefreshDownload(status ExportJobStatus, resultRef *ExportResultRef, now time.Time) bool {
	return ExportJobDownloadExpired(status, resultRef, now)
}

func HydrateExportJobDerived(job *ExportJob) {
	if job == nil {
		return
	}
	if job.LatestStatusAt.IsZero() {
		if !job.UpdatedAt.IsZero() {
			job.LatestStatusAt = job.UpdatedAt
		} else {
			job.LatestStatusAt = job.CreatedAt
		}
	}
	job.ProgressHint = ExportJobProgressForStatus(job.Status)
	job.DownloadReady = ExportJobDownloadReady(job.Status, job.ResultRef)
	if job.LatestDispatch != nil {
		HydrateExportJobDispatchDerived(job.LatestDispatch)
	}
	if job.LatestAttempt != nil {
		HydrateExportJobAttemptDerived(job.LatestAttempt)
	}
	job.CanStart, job.CanStartReason = ExportJobStartAdmission(job.Status, job.LatestDispatch)
	job.CanAttempt, job.CanAttemptReason = ExportJobAttemptAdmission(job.Status, job.LatestDispatch)
	job.CanRetry = ExportJobCanRetry(job.Status, job.AttemptCount)
	job.CanDispatch, job.CanDispatchReason = ExportJobDispatchAdmission(job.Status, job.LatestDispatch)
	job.CanRedispatch, job.CanRedispatchReason = ExportJobRedispatchAdmission(job.Status, job.DispatchCount, job.LatestDispatch)
	job.DispatchabilityReason = job.CanDispatchReason
	job.AttemptabilityReason = job.CanAttemptReason
	job.LatestAdmissionDecision = BuildExportJobLatestAdmissionDecision(job)
	job.StartMode = ExportJobStartModeExplicitInternal
	job.ExecutionMode = ExportJobExecutionModeManualPlaceholderRunner
	job.AdapterMode = ExportJobAdapterModeDispatchThenAttempt
	job.DispatchMode = BoundaryDispatchModeDispatchRecord
	job.StorageMode = ExportJobStorageModeLifecycleManagedResultRef
	job.DeliveryMode = ExportJobDeliveryModeClaimReadRefreshHandoff
	job.AdapterRefSummary = buildExportAdapterRefSummary(job)
	job.ResourceRefSummary = buildExportResourceRefSummary(job.ResultRef)
	job.HandoffRefSummary = buildExportHandoffRefSummary(job.LatestDispatch)
	job.ExecutionBoundary = DefaultExportJobExecutionBoundary()
	job.StorageBoundary = DefaultExportJobStorageBoundary()
	job.DeliveryBoundary = DefaultExportJobDeliveryBoundary()
	HydrateExportJobPolicy(job)
}

func BuildExportJobLatestAdmissionDecision(job *ExportJob) *ExportJobAdmissionDecision {
	if job == nil {
		return nil
	}
	decision := &ExportJobAdmissionDecision{
		JobStatus:            job.Status,
		LatestDispatchStatus: exportJobDispatchStatusPtr(job.LatestDispatch),
		LatestAttemptStatus:  exportJobAttemptStatusPtr(job.LatestAttempt),
	}
	switch {
	case !job.CanAttempt:
		decision.DecisionType = ExportJobAdmissionDecisionTypeAttempt
		decision.Allowed = false
		decision.Reason = job.CanAttemptReason
	case !job.CanDispatch:
		decision.DecisionType = ExportJobAdmissionDecisionTypeDispatch
		decision.Allowed = false
		decision.Reason = job.CanDispatchReason
	case job.DispatchCount > 0 && !job.CanRedispatch:
		decision.DecisionType = ExportJobAdmissionDecisionTypeRedispatch
		decision.Allowed = false
		decision.Reason = job.CanRedispatchReason
	default:
		decision.DecisionType = ExportJobAdmissionDecisionTypeAttempt
		decision.Allowed = true
		decision.Reason = job.CanAttemptReason
	}
	return decision
}

func exportJobDispatchStatusPtr(dispatch *ExportJobDispatch) *ExportJobDispatchStatus {
	if dispatch == nil {
		return nil
	}
	status := dispatch.Status
	return &status
}

func exportJobAttemptStatusPtr(attempt *ExportJobAttempt) *ExportJobAttemptStatus {
	if attempt == nil {
		return nil
	}
	status := attempt.Status
	return &status
}

func HydrateExportJobDownloadState(job *ExportJob, now time.Time) {
	if job == nil {
		return
	}
	job.IsExpired = ExportJobDownloadExpired(job.Status, job.ResultRef, now)
	job.CanRefresh = ExportJobCanRefreshDownload(job.Status, job.ResultRef, now)
}

func BuildExportJobDownloadHandoff(job *ExportJob, now time.Time) *ExportJobDownloadHandoff {
	if job == nil {
		return nil
	}
	claimAvailable := ExportJobCanAccessDownload(job.Status, job.ResultRef, now)
	handoff := &ExportJobDownloadHandoff{
		ExportJobID:    job.ExportJobID,
		Status:         job.Status,
		DownloadReady:  job.DownloadReady,
		ClaimAvailable: claimAvailable,
		ReadAvailable:  claimAvailable,
		IsExpired:      ExportJobDownloadExpired(job.Status, job.ResultRef, now),
		CanRefresh:     ExportJobCanRefreshDownload(job.Status, job.ResultRef, now),
		ResultRef:      cloneExportResultRefForHandoff(job.ResultRef),
	}
	if job.ResultRef != nil {
		handoff.FileName = job.ResultRef.FileName
		handoff.MimeType = job.ResultRef.MimeType
		handoff.IsPlaceholder = job.ResultRef.IsPlaceholder
		handoff.Note = job.ResultRef.Note
		if job.ResultRef.ExpiresAt != nil {
			expiresAtCopy := job.ResultRef.ExpiresAt.UTC()
			handoff.ExpiresAt = &expiresAtCopy
		}
	}
	return handoff
}

func cloneExportResultRefForHandoff(value *ExportResultRef) *ExportResultRef {
	if value == nil {
		return nil
	}
	copyValue := *value
	if value.ExpiresAt != nil {
		expiresAtCopy := value.ExpiresAt.UTC()
		copyValue.ExpiresAt = &expiresAtCopy
	}
	return &copyValue
}

func buildExportAdapterRefSummary(job *ExportJob) *AdapterRefSummary {
	if job == nil {
		return nil
	}
	adapterKey := string(ExportJobRunnerAdapterKeyManualPlaceholder)
	if job.LatestAttempt != nil && strings.TrimSpace(string(job.LatestAttempt.AdapterKey)) != "" {
		adapterKey = strings.TrimSpace(string(job.LatestAttempt.AdapterKey))
	}
	return BuildAdapterRefSummary("runner_adapter", adapterKey, true, "Placeholder export runner adapter boundary.")
}

func buildExportResourceRefSummary(resultRef *ExportResultRef) *ResourceRefSummary {
	if resultRef == nil {
		return nil
	}
	return BuildResourceRefSummary(resultRef.RefType, resultRef.RefKey, resultRef.FileName, resultRef.MimeType, nil, "", resultRef.ExpiresAt, resultRef.IsPlaceholder, resultRef.Note)
}

func buildExportHandoffRefSummary(dispatch *ExportJobDispatch) *HandoffRefSummary {
	if dispatch == nil {
		return nil
	}
	return BuildHandoffRefSummary(
		"dispatch",
		dispatch.DispatchID,
		string(dispatch.Status),
		&dispatch.SubmittedAt,
		dispatch.ReceivedAt,
		dispatch.FinishedAt,
		dispatch.ExpiresAt,
		true,
		dispatch.AdapterNote,
	)
}

func DefaultExportJobExecutionBoundary() ExportJobExecutionBoundary {
	return ExportJobExecutionBoundary{
		BoundaryKey:              "start_dispatch_attempt_layered",
		StartLayer:               "export_job_start_boundary",
		DispatchLayer:            "export_job_dispatches",
		AttemptLayer:             "export_job_attempts",
		ResultGenerationLayer:    "export_job_lifecycle_advance",
		FutureRunnerReplaceLayer: "runner_adapter_between_dispatch_and_attempt_result",
		Placeholder:              true,
		Note:                     "Start owns queued-to-running initiation, dispatch owns adapter handoff, attempts own one concrete execution try, and ready/fail/cancel lifecycle actions still mint placeholder result state until a real runner replaces this seam.",
	}
}

func DefaultExportJobStorageBoundary() ExportJobStorageBoundary {
	return ExportJobStorageBoundary{
		BoundaryKey:               "result_ref_placeholder_storage",
		ResultSourceLayer:         "export_job_lifecycle_result_ref",
		StorageLayer:              "result_ref_metadata_only",
		StorageRefField:           "result_ref",
		FutureStorageReplaceLayer: "storage_adapter_backing_result_ref",
		Placeholder:               true,
		Note:                      "Current ready-state output is structured result_ref metadata only; no file bytes, NAS path, or object-storage object is created in this phase.",
	}
}

func DefaultExportJobDeliveryBoundary() ExportJobDeliveryBoundary {
	return ExportJobDeliveryBoundary{
		BoundaryKey:                "claim_read_refresh_download_handoff",
		DeliveryLayer:              "export_job_download_handoff",
		DeliveryRefField:           "result_ref",
		ClaimAction:                "claim_download",
		ReadAction:                 "download",
		RefreshAction:              "refresh_download",
		FutureDeliveryReplaceLayer: "delivery_adapter_behind_download_handoff",
		Placeholder:                true,
		Note:                       "Claim/read/refresh routes deliver placeholder handoff metadata only; a future download service should sit behind this boundary rather than changing export lifecycle or result_ref semantics.",
	}
}
