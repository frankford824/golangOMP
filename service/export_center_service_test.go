package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestExportCenterServiceCreateTaskBoardQueueJob(t *testing.T) {
	repoStub := newExportJobRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 3, 4, 5, 0, time.UTC)
	}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       88,
		Roles:    []domain.Role{domain.RoleOps, domain.RoleAdmin},
		Source:   "header_placeholder",
		AuthMode: domain.AuthModePlaceholderNoEnforcement,
	})
	queryTemplate := domain.TaskQueryTemplate{
		TaskType:              "purchase_task",
		CoordinationStatus:    "awaiting_arrival",
		WarehousePrepareReady: boolPtr(true),
	}

	job, appErr := svc.CreateJob(ctx, CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskBoardQueue,
		SourceQueryType: domain.ExportSourceQueryTypeTaskBoardQueue,
		SourceFilters: domain.ExportSourceFilters{
			QueueKey:  "warehouse_pending_prepare",
			BoardView: domain.TaskBoardViewProcurement,
		},
		QueryTemplate: &queryTemplate,
		Remark:        "front-end handoff",
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}
	if job.ExportJobID == 0 {
		t.Fatalf("CreateJob() export_job_id = %d", job.ExportJobID)
	}
	if job.TemplateKey != "task_board_queue_basic" {
		t.Fatalf("template_key = %q, want task_board_queue_basic", job.TemplateKey)
	}
	if job.Status != domain.ExportJobStatusQueued {
		t.Fatalf("status = %s, want queued", job.Status)
	}
	if job.ProgressHint != domain.ExportJobProgressHintCreated {
		t.Fatalf("progress_hint = %s, want created", job.ProgressHint)
	}
	if job.DownloadReady {
		t.Fatal("download_ready = true, want false")
	}
	if !job.CanStart || job.StartMode != domain.ExportJobStartModeExplicitInternal || job.ExecutionMode != domain.ExportJobExecutionModeManualPlaceholderRunner {
		t.Fatalf("start semantics = %+v", job)
	}
	if !job.CanAttempt || job.CanAttemptReason != domain.ExportJobAdmissionReasonNoDispatchAutoPlaceholderAllowed {
		t.Fatalf("attempt semantics = can_attempt:%v reason:%s", job.CanAttempt, job.CanAttemptReason)
	}
	if !job.CanDispatch || job.CanDispatchReason != domain.ExportJobAdmissionReasonQueuedWithoutDispatch {
		t.Fatalf("dispatch semantics = can_dispatch:%v reason:%s", job.CanDispatch, job.CanDispatchReason)
	}
	if job.DispatchabilityReason != job.CanDispatchReason || job.AttemptabilityReason != job.CanAttemptReason {
		t.Fatalf("admission aliases = dispatch:%s attempt:%s", job.DispatchabilityReason, job.AttemptabilityReason)
	}
	if job.LatestAdmissionDecision == nil || !job.LatestAdmissionDecision.Allowed || job.LatestAdmissionDecision.DecisionType != domain.ExportJobAdmissionDecisionTypeAttempt {
		t.Fatalf("latest_admission_decision = %+v", job.LatestAdmissionDecision)
	}
	if job.AdapterMode != domain.ExportJobAdapterModeDispatchThenAttempt || job.StorageMode != domain.ExportJobStorageModeLifecycleManagedResultRef || job.DeliveryMode != domain.ExportJobDeliveryModeClaimReadRefreshHandoff {
		t.Fatalf("boundary modes = %+v", job)
	}
	if job.DispatchMode != domain.BoundaryDispatchModeDispatchRecord {
		t.Fatalf("dispatch_mode = %s", job.DispatchMode)
	}
	if job.ExecutionBoundary.BoundaryKey != "start_dispatch_attempt_layered" || job.ExecutionBoundary.StartLayer != "export_job_start_boundary" || job.ExecutionBoundary.ResultGenerationLayer != "export_job_lifecycle_advance" {
		t.Fatalf("execution_boundary = %+v", job.ExecutionBoundary)
	}
	if job.StorageBoundary.BoundaryKey != "result_ref_placeholder_storage" || job.StorageBoundary.StorageRefField != "result_ref" {
		t.Fatalf("storage_boundary = %+v", job.StorageBoundary)
	}
	if job.DeliveryBoundary.BoundaryKey != "claim_read_refresh_download_handoff" || job.DeliveryBoundary.ClaimAction != "claim_download" || job.DeliveryBoundary.ReadAction != "download" || job.DeliveryBoundary.RefreshAction != "refresh_download" {
		t.Fatalf("delivery_boundary = %+v", job.DeliveryBoundary)
	}
	if job.PolicyMode != domain.PolicyModeRouteRoleVisibilityScaffolding || job.PolicyScopeSummary == nil {
		t.Fatalf("policy scaffolding = mode:%s summary:%+v", job.PolicyMode, job.PolicyScopeSummary)
	}
	if job.PlatformEntryBoundary == nil || job.PlatformEntryBoundary.EntryMode != domain.PlatformEntryModeScaffoldingOnly {
		t.Fatalf("platform_entry_boundary = %+v", job.PlatformEntryBoundary)
	}
	if job.PlatformEntryBoundary.ReportEntrySummary == nil || !job.PlatformEntryBoundary.ReportEntrySummary.EligibleNow {
		t.Fatalf("report_entry_summary = %+v", job.PlatformEntryBoundary)
	}
	if job.PlatformEntryBoundary.FinanceEntrySummary == nil {
		t.Fatalf("finance_entry_summary = %+v", job.PlatformEntryBoundary)
	}
	if job.PlatformEntryBoundary.FinanceEntrySummary.EntryStatus != domain.PlatformEntryStatusConditional || job.PlatformEntryBoundary.FinanceEntrySummary.EligibleNow {
		t.Fatalf("finance_entry_summary eligibility = %+v", job.PlatformEntryBoundary.FinanceEntrySummary)
	}
	if len(job.VisibleToRoles) == 0 || len(job.ActionRoles) == 0 {
		t.Fatalf("policy roles/actions = visible:%+v actions:%+v", job.VisibleToRoles, job.ActionRoles)
	}
	if job.PolicyScopeSummary.ResourceAccessPolicy.ResourceKey != "export_job" {
		t.Fatalf("policy resource key = %s, want export_job", job.PolicyScopeSummary.ResourceAccessPolicy.ResourceKey)
	}
	if job.RequestedBy.ID != 88 {
		t.Fatalf("requested_by.id = %d, want 88", job.RequestedBy.ID)
	}
	if job.ResultRef == nil || job.ResultRef.RefType != "download_handoff_placeholder" || !job.ResultRef.IsPlaceholder {
		t.Fatalf("result_ref = %+v", job.ResultRef)
	}
	if job.AdapterRefSummary == nil || job.AdapterRefSummary.RefKey != string(domain.ExportJobRunnerAdapterKeyManualPlaceholder) {
		t.Fatalf("adapter_ref_summary = %+v", job.AdapterRefSummary)
	}
	if job.ResourceRefSummary == nil || job.ResourceRefSummary.RefKey != job.ResultRef.RefKey {
		t.Fatalf("resource_ref_summary = %+v", job.ResourceRefSummary)
	}
	if job.QueryTemplate == nil || job.QueryTemplate.CoordinationStatus != "awaiting_arrival" {
		t.Fatalf("query_template = %+v", job.QueryTemplate)
	}
	if job.NormalizedFilters == nil || len(job.NormalizedFilters.TaskTypes) != 1 || job.NormalizedFilters.TaskTypes[0] != domain.TaskTypePurchaseTask {
		t.Fatalf("normalized_filters = %+v", job.NormalizedFilters)
	}
	if job.EventCount != 1 {
		t.Fatalf("event_count = %d, want 1", job.EventCount)
	}
	if job.LatestEvent == nil || job.LatestEvent.EventType != domain.ExportJobEventCreated {
		t.Fatalf("latest_event = %+v", job.LatestEvent)
	}
	if job.LatestRunnerEvent != nil {
		t.Fatalf("latest_runner_event = %+v, want nil", job.LatestRunnerEvent)
	}
	if job.AttemptCount != 0 || job.LatestAttempt != nil || job.CanRetry {
		t.Fatalf("attempt summary = %+v", job)
	}
	if len(eventRepoStub.eventsByJob[job.ExportJobID]) != 1 {
		t.Fatalf("stored events = %d, want 1", len(eventRepoStub.eventsByJob[job.ExportJobID]))
	}
}

func TestExportCenterServiceRejectsMissingTaskQueryTemplate(t *testing.T) {
	svc := NewExportCenterService(newExportJobRepoStub(), newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), newExportJobEventRepoStub(), noopTxRunner{}).(*exportCenterService)

	_, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("expected invalid request, got %+v", appErr)
	}
}

func TestExportCenterServiceRejectsWarehouseQueryTemplate(t *testing.T) {
	svc := NewExportCenterService(newExportJobRepoStub(), newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), newExportJobEventRepoStub(), noopTxRunner{}).(*exportCenterService)
	queryTemplate := domain.TaskQueryTemplate{TaskType: "purchase_task"}

	_, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeWarehouseReceipts,
		SourceQueryType: domain.ExportSourceQueryTypeWarehouseReceipts,
		QueryTemplate:   &queryTemplate,
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("expected invalid request, got %+v", appErr)
	}
}

func TestExportCenterServiceListTemplates(t *testing.T) {
	svc := NewExportCenterService(newExportJobRepoStub(), newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), newExportJobEventRepoStub(), noopTxRunner{}).(*exportCenterService)

	templates, appErr := svc.ListTemplates(context.Background())
	if appErr != nil {
		t.Fatalf("ListTemplates() unexpected error: %+v", appErr)
	}
	if len(templates) != 4 {
		t.Fatalf("template count = %d, want 4", len(templates))
	}
	if !templates[0].PlaceholderOnly {
		t.Fatal("expected placeholder-only template catalog")
	}
}

func TestExportCenterServiceAdvanceLifecycleToReady(t *testing.T) {
	repoStub := newExportJobRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 4, 0, 0, 0, time.UTC)
	}

	job, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
		QueryTemplate:   &domain.TaskQueryTemplate{TaskType: "new_product_development"},
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 4, 5, 0, 0, time.UTC)
	}
	job, appErr = svc.AdvanceJob(context.Background(), job.ExportJobID, AdvanceExportJobParams{
		Action: domain.ExportJobAdvanceActionStart,
	})
	if appErr != nil {
		t.Fatalf("AdvanceJob(start) unexpected error: %+v", appErr)
	}
	if job.Status != domain.ExportJobStatusRunning || job.ProgressHint != domain.ExportJobProgressHintProcessing {
		t.Fatalf("AdvanceJob(start) job = %+v", job)
	}
	if job.CanStart || job.LatestRunnerEvent == nil || job.LatestRunnerEvent.EventType != domain.ExportJobEventStarted {
		t.Fatalf("AdvanceJob(start) runner state = %+v", job)
	}
	if job.AttemptCount != 1 || job.LatestAttempt == nil || job.LatestAttempt.Status != domain.ExportJobAttemptStatusRunning || job.LatestAttempt.AttemptNo != 1 {
		t.Fatalf("AdvanceJob(start) latest_attempt = %+v", job.LatestAttempt)
	}
	if job.HandoffRefSummary == nil || job.HandoffRefSummary.RefType != "dispatch" {
		t.Fatalf("AdvanceJob(start) handoff_ref_summary = %+v", job.HandoffRefSummary)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 4, 10, 0, 0, time.UTC)
	}
	job, appErr = svc.AdvanceJob(context.Background(), job.ExportJobID, AdvanceExportJobParams{
		Action:         domain.ExportJobAdvanceActionMarkReady,
		ResultFileName: "task_export_demo.csv",
	})
	if appErr != nil {
		t.Fatalf("AdvanceJob(mark_ready) unexpected error: %+v", appErr)
	}
	if job.Status != domain.ExportJobStatusReady || !job.DownloadReady {
		t.Fatalf("AdvanceJob(mark_ready) job = %+v", job)
	}
	if job.StorageMode != domain.ExportJobStorageModeLifecycleManagedResultRef || job.DeliveryMode != domain.ExportJobDeliveryModeClaimReadRefreshHandoff {
		t.Fatalf("ready boundary modes = %+v", job)
	}
	if job.StorageBoundary.ResultSourceLayer != "export_job_lifecycle_result_ref" || job.DeliveryBoundary.DeliveryLayer != "export_job_download_handoff" {
		t.Fatalf("ready boundaries = storage:%+v delivery:%+v", job.StorageBoundary, job.DeliveryBoundary)
	}
	if job.IsExpired || job.CanRefresh {
		t.Fatalf("AdvanceJob(mark_ready) expiry flags = %+v", job)
	}
	if job.ResultRef == nil || job.ResultRef.RefType != "download_handoff" || job.ResultRef.ExpiresAt == nil {
		t.Fatalf("AdvanceJob(mark_ready) result_ref = %+v", job.ResultRef)
	}
	if job.FinishedAt == nil {
		t.Fatal("AdvanceJob(mark_ready) finished_at = nil")
	}
	if job.EventCount != 9 {
		t.Fatalf("event_count = %d, want 9", job.EventCount)
	}
	if job.LatestEvent == nil || job.LatestEvent.EventType != domain.ExportJobEventAdvancedToReady {
		t.Fatalf("latest_event = %+v", job.LatestEvent)
	}
	if job.LatestRunnerEvent == nil || job.LatestRunnerEvent.EventType != domain.ExportJobEventAttemptSucceeded {
		t.Fatalf("latest_runner_event = %+v", job.LatestRunnerEvent)
	}
	if job.AttemptCount != 1 || job.LatestAttempt == nil || job.LatestAttempt.Status != domain.ExportJobAttemptStatusSucceeded {
		t.Fatalf("latest_attempt = %+v", job.LatestAttempt)
	}
	events, appErr := svc.ListJobEvents(context.Background(), job.ExportJobID)
	if appErr != nil {
		t.Fatalf("ListJobEvents() unexpected error: %+v", appErr)
	}
	if len(events) != 9 {
		t.Fatalf("event length = %d, want 9", len(events))
	}
	if events[0].EventType != domain.ExportJobEventCreated ||
		events[1].EventType != domain.ExportJobEventDispatchSubmitted ||
		events[2].EventType != domain.ExportJobEventDispatchReceived ||
		events[3].EventType != domain.ExportJobEventRunnerInitiated ||
		events[4].EventType != domain.ExportJobEventStarted ||
		events[5].EventType != domain.ExportJobEventAdvancedToRunning ||
		events[6].EventType != domain.ExportJobEventResultRefUpdated ||
		events[7].EventType != domain.ExportJobEventAttemptSucceeded ||
		events[8].EventType != domain.ExportJobEventAdvancedToReady {
		t.Fatalf("event timeline = %+v", eventTypes(events))
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(events[6].Payload, &payload); err != nil {
		t.Fatalf("unmarshal result_ref_updated payload: %v", err)
	}
	if _, ok := payload["current_result_ref"]; !ok {
		t.Fatalf("result_ref_updated payload = %+v", payload)
	}
}

func TestExportCenterServiceStartJobAddsExplicitRunnerBoundary(t *testing.T) {
	repoStub := newExportJobRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 4, 20, 0, 0, time.UTC)
	}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       501,
		Roles:    []domain.Role{domain.RoleAdmin},
		Source:   "header_placeholder",
		AuthMode: domain.AuthModePlaceholderNoEnforcement,
	})
	job, appErr := svc.CreateJob(ctx, CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
		QueryTemplate:   &domain.TaskQueryTemplate{TaskType: "new_product_development"},
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 4, 21, 0, 0, time.UTC)
	}
	job, appErr = svc.StartJob(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("StartJob() unexpected error: %+v", appErr)
	}
	if job.Status != domain.ExportJobStatusRunning || job.CanStart {
		t.Fatalf("StartJob() job = %+v", job)
	}
	if job.StartMode != domain.ExportJobStartModeExplicitInternal || job.ExecutionMode != domain.ExportJobExecutionModeManualPlaceholderRunner {
		t.Fatalf("StartJob() start metadata = %+v", job)
	}
	if job.AdapterMode != domain.ExportJobAdapterModeDispatchThenAttempt || job.ExecutionBoundary.FutureRunnerReplaceLayer != "runner_adapter_between_dispatch_and_attempt_result" {
		t.Fatalf("StartJob() runner boundary = %+v %+v", job.AdapterMode, job.ExecutionBoundary)
	}
	if job.LatestRunnerEvent == nil || job.LatestRunnerEvent.EventType != domain.ExportJobEventStarted {
		t.Fatalf("StartJob() latest_runner_event = %+v", job.LatestRunnerEvent)
	}
	if job.AttemptCount != 1 || job.LatestAttempt == nil || job.LatestAttempt.Status != domain.ExportJobAttemptStatusRunning || job.LatestAttempt.AttemptNo != 1 {
		t.Fatalf("StartJob() latest_attempt = %+v", job.LatestAttempt)
	}

	events, appErr := svc.ListJobEvents(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("ListJobEvents() unexpected error: %+v", appErr)
	}
	if len(events) != 6 {
		t.Fatalf("event length = %d, want 6", len(events))
	}
	if eventTypes(events)[1] != domain.ExportJobEventDispatchSubmitted || eventTypes(events)[2] != domain.ExportJobEventDispatchReceived || eventTypes(events)[3] != domain.ExportJobEventRunnerInitiated || eventTypes(events)[4] != domain.ExportJobEventStarted || eventTypes(events)[5] != domain.ExportJobEventAdvancedToRunning {
		t.Fatalf("event timeline = %+v", eventTypes(events))
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(events[3].Payload, &payload); err != nil {
		t.Fatalf("unmarshal runner_initiated payload: %v", err)
	}
	if payload["initiation_source"] != exportJobInitiationSourceStartEndpoint {
		t.Fatalf("runner_initiated payload = %+v", payload)
	}
}

func TestExportCenterServiceRejectsDuplicateStart(t *testing.T) {
	repoStub := newExportJobRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 4, 30, 0, 0, time.UTC)
	}

	job, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
		QueryTemplate:   &domain.TaskQueryTemplate{TaskType: "new_product_development"},
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 4, 31, 0, 0, time.UTC)
	}
	if _, appErr = svc.StartJob(context.Background(), job.ExportJobID); appErr != nil {
		t.Fatalf("StartJob() unexpected error: %+v", appErr)
	}

	_, appErr = svc.StartJob(context.Background(), job.ExportJobID)
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("second StartJob() error = %+v, want invalid state transition", appErr)
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("StartJob() details = %#v", appErr.Details)
	}
	if details["status"] != domain.ExportJobStatusRunning || details["can_start"] != false {
		t.Fatalf("StartJob() details = %#v", details)
	}
	if details["can_start_reason"] != domain.ExportJobAdmissionReasonRunningStartBlocked || details["can_attempt_reason"] != domain.ExportJobAdmissionReasonRunningAttemptBlocked {
		t.Fatalf("StartJob() admission reasons = %#v", details)
	}
}

func TestExportCenterServiceAdvanceLifecycleToFailedAndRequeue(t *testing.T) {
	repoStub := newExportJobRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 5, 0, 0, 0, time.UTC)
	}

	job, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeWarehouseReceipts,
		SourceQueryType: domain.ExportSourceQueryTypeWarehouseReceipts,
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 5, 2, 0, 0, time.UTC)
	}
	job, appErr = svc.AdvanceJob(context.Background(), job.ExportJobID, AdvanceExportJobParams{
		Action:        domain.ExportJobAdvanceActionFail,
		FailureReason: "placeholder runner rejected request",
	})
	if appErr != nil {
		t.Fatalf("AdvanceJob(fail) unexpected error: %+v", appErr)
	}
	if job.Status != domain.ExportJobStatusFailed || job.ProgressHint != domain.ExportJobProgressHintFailed {
		t.Fatalf("AdvanceJob(fail) job = %+v", job)
	}
	if job.ResultRef == nil || !strings.Contains(job.ResultRef.Note, "placeholder runner rejected request") {
		t.Fatalf("AdvanceJob(fail) result_ref = %+v", job.ResultRef)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 5, 4, 0, 0, time.UTC)
	}
	job, appErr = svc.AdvanceJob(context.Background(), job.ExportJobID, AdvanceExportJobParams{
		Action: domain.ExportJobAdvanceActionRequeue,
	})
	if appErr != nil {
		t.Fatalf("AdvanceJob(requeue) unexpected error: %+v", appErr)
	}
	if job.Status != domain.ExportJobStatusQueued || job.FinishedAt != nil {
		t.Fatalf("AdvanceJob(requeue) job = %+v", job)
	}
	if job.EventCount != 3 {
		t.Fatalf("event_count = %d, want 3", job.EventCount)
	}
	if job.LatestEvent == nil || job.LatestEvent.EventType != domain.ExportJobEventAdvancedToQueued {
		t.Fatalf("latest_event = %+v", job.LatestEvent)
	}
	if job.CanStart != true {
		t.Fatalf("AdvanceJob(requeue) can_start = %v, want true", job.CanStart)
	}
}

func TestExportCenterServiceListJobsHydratesEventSummaries(t *testing.T) {
	repoStub := newExportJobRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 6, 0, 0, 0, time.UTC)
	}

	first, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
		QueryTemplate:   &domain.TaskQueryTemplate{TaskType: "new_product_development"},
	})
	if appErr != nil {
		t.Fatalf("CreateJob(first) unexpected error: %+v", appErr)
	}
	second, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeWarehouseReceipts,
		SourceQueryType: domain.ExportSourceQueryTypeWarehouseReceipts,
	})
	if appErr != nil {
		t.Fatalf("CreateJob(second) unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 6, 5, 0, 0, time.UTC)
	}
	if _, appErr = svc.AdvanceJob(context.Background(), first.ExportJobID, AdvanceExportJobParams{
		Action: domain.ExportJobAdvanceActionStart,
	}); appErr != nil {
		t.Fatalf("AdvanceJob(first/start) unexpected error: %+v", appErr)
	}

	jobs, _, appErr := svc.ListJobs(context.Background(), ExportJobFilter{})
	if appErr != nil {
		t.Fatalf("ListJobs() unexpected error: %+v", appErr)
	}
	if len(jobs) != 2 {
		t.Fatalf("job length = %d, want 2", len(jobs))
	}
	for _, job := range jobs {
		if job.ExportJobID == first.ExportJobID {
			if job.EventCount != 6 || job.LatestEvent == nil || job.LatestEvent.EventType != domain.ExportJobEventAdvancedToRunning {
				t.Fatalf("first job summary = %+v", job)
			}
			if job.LatestRunnerEvent == nil || job.LatestRunnerEvent.EventType != domain.ExportJobEventStarted || job.CanStart {
				t.Fatalf("first job runner summary = %+v", job)
			}
			if job.AttemptCount != 1 || job.LatestAttempt == nil || job.LatestAttempt.Status != domain.ExportJobAttemptStatusRunning {
				t.Fatalf("first job attempt summary = %+v", job)
			}
		}
		if job.ExportJobID == second.ExportJobID {
			if job.EventCount != 1 || job.LatestEvent == nil || job.LatestEvent.EventType != domain.ExportJobEventCreated {
				t.Fatalf("second job summary = %+v", job)
			}
			if job.LatestRunnerEvent != nil || !job.CanStart {
				t.Fatalf("second job runner summary = %+v", job)
			}
			if job.AttemptCount != 0 || job.LatestAttempt != nil || job.CanRetry {
				t.Fatalf("second job attempt summary = %+v", job)
			}
		}
	}
}

func TestExportCenterServiceClaimAndReadDownloadHandoff(t *testing.T) {
	repoStub := newExportJobRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 7, 0, 0, 0, time.UTC)
	}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       201,
		Roles:    []domain.Role{domain.RoleOps},
		Source:   "header_placeholder",
		AuthMode: domain.AuthModePlaceholderNoEnforcement,
	})
	job, appErr := svc.CreateJob(ctx, CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
		QueryTemplate:   &domain.TaskQueryTemplate{TaskType: "new_product_development"},
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 7, 1, 0, 0, time.UTC)
	}
	if _, appErr = svc.AdvanceJob(ctx, job.ExportJobID, AdvanceExportJobParams{
		Action: domain.ExportJobAdvanceActionStart,
	}); appErr != nil {
		t.Fatalf("AdvanceJob(start) unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 7, 2, 0, 0, time.UTC)
	}
	if _, appErr = svc.AdvanceJob(ctx, job.ExportJobID, AdvanceExportJobParams{
		Action:         domain.ExportJobAdvanceActionMarkReady,
		ResultFileName: "task_export_ready.csv",
	}); appErr != nil {
		t.Fatalf("AdvanceJob(mark_ready) unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 7, 3, 0, 0, time.UTC)
	}
	claim, appErr := svc.ClaimDownload(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("ClaimDownload() unexpected error: %+v", appErr)
	}
	if claim.ExportJobID != job.ExportJobID || !claim.DownloadReady || !claim.ClaimAvailable || !claim.ReadAvailable {
		t.Fatalf("ClaimDownload() handoff = %+v", claim)
	}
	if claim.IsExpired || claim.CanRefresh {
		t.Fatalf("ClaimDownload() expiry flags = %+v", claim)
	}
	if claim.FileName != "task_export_ready.csv" || claim.MimeType != "text/csv" || !claim.IsPlaceholder {
		t.Fatalf("ClaimDownload() handoff = %+v", claim)
	}
	if claim.ClaimedAt == nil || claim.ClaimedByActorID == nil || *claim.ClaimedByActorID != 201 || claim.ClaimedByActorType != "header_placeholder" {
		t.Fatalf("ClaimDownload() audit = %+v", claim)
	}
	if claim.LastReadAt != nil {
		t.Fatalf("ClaimDownload() last_read_at = %+v, want nil", claim.LastReadAt)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 7, 4, 0, 0, time.UTC)
	}
	read, appErr := svc.ReadDownload(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("ReadDownload() unexpected error: %+v", appErr)
	}
	if read.ClaimedAt == nil || read.LastReadAt == nil {
		t.Fatalf("ReadDownload() audit = %+v", read)
	}
	if read.LastReadByActorID == nil || *read.LastReadByActorID != 201 || read.LastReadByActorType != "header_placeholder" {
		t.Fatalf("ReadDownload() audit = %+v", read)
	}

	current, appErr := svc.GetJob(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("GetJob() unexpected error: %+v", appErr)
	}
	if current.EventCount != 11 {
		t.Fatalf("event_count = %d, want 11", current.EventCount)
	}
	if current.LatestEvent == nil || current.LatestEvent.EventType != domain.ExportJobEventDownloadRead {
		t.Fatalf("latest_event = %+v", current.LatestEvent)
	}
	if current.LatestRunnerEvent == nil || current.LatestRunnerEvent.EventType != domain.ExportJobEventAttemptSucceeded {
		t.Fatalf("latest_runner_event = %+v", current.LatestRunnerEvent)
	}
	if current.AttemptCount != 1 || current.LatestAttempt == nil || current.LatestAttempt.Status != domain.ExportJobAttemptStatusSucceeded {
		t.Fatalf("latest_attempt = %+v", current.LatestAttempt)
	}

	events, appErr := svc.ListJobEvents(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("ListJobEvents() unexpected error: %+v", appErr)
	}
	if len(events) != 11 {
		t.Fatalf("event length = %d, want 11", len(events))
	}
	if events[9].EventType != domain.ExportJobEventDownloadClaimed || events[10].EventType != domain.ExportJobEventDownloadRead {
		t.Fatalf("event timeline = %+v", eventTypes(events))
	}
}

func TestExportCenterServiceRejectsDownloadClaimWhenNotReady(t *testing.T) {
	svc := NewExportCenterService(newExportJobRepoStub(), newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), newExportJobEventRepoStub(), noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	}

	job, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
		QueryTemplate:   &domain.TaskQueryTemplate{TaskType: "new_product_development"},
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}

	_, appErr = svc.ClaimDownload(context.Background(), job.ExportJobID)
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("ClaimDownload() error = %+v, want invalid state transition", appErr)
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("ClaimDownload() details = %#v", appErr.Details)
	}
	if details["status"] != domain.ExportJobStatusQueued {
		t.Fatalf("ClaimDownload() details.status = %#v, want queued", details["status"])
	}
}

func TestExportCenterServiceExpiredDownloadRequiresRefresh(t *testing.T) {
	repoStub := newExportJobRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       301,
		Roles:    []domain.Role{domain.RoleOps},
		Source:   "header_placeholder",
		AuthMode: domain.AuthModePlaceholderNoEnforcement,
	})
	job, appErr := svc.CreateJob(ctx, CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
		QueryTemplate:   &domain.TaskQueryTemplate{TaskType: "new_product_development"},
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 9, 1, 0, 0, time.UTC)
	}
	if _, appErr = svc.AdvanceJob(ctx, job.ExportJobID, AdvanceExportJobParams{
		Action: domain.ExportJobAdvanceActionStart,
	}); appErr != nil {
		t.Fatalf("AdvanceJob(start) unexpected error: %+v", appErr)
	}

	expiresAt := time.Date(2026, 3, 10, 9, 3, 0, 0, time.UTC)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 9, 2, 0, 0, time.UTC)
	}
	job, appErr = svc.AdvanceJob(ctx, job.ExportJobID, AdvanceExportJobParams{
		Action:         domain.ExportJobAdvanceActionMarkReady,
		ResultFileName: "task_export_expiring.csv",
		ExpiresAt:      &expiresAt,
	})
	if appErr != nil {
		t.Fatalf("AdvanceJob(mark_ready) unexpected error: %+v", appErr)
	}
	originalRefKey := job.ResultRef.RefKey

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 9, 4, 0, 0, time.UTC)
	}
	job, appErr = svc.GetJob(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("GetJob() unexpected error: %+v", appErr)
	}
	if !job.IsExpired || !job.CanRefresh {
		t.Fatalf("GetJob() expiry flags = %+v", job)
	}

	jobs, _, appErr := svc.ListJobs(ctx, ExportJobFilter{})
	if appErr != nil {
		t.Fatalf("ListJobs() unexpected error: %+v", appErr)
	}
	if len(jobs) != 1 || !jobs[0].IsExpired || !jobs[0].CanRefresh {
		t.Fatalf("ListJobs() jobs = %+v", jobs)
	}

	_, appErr = svc.ClaimDownload(ctx, job.ExportJobID)
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("ClaimDownload() error = %+v, want invalid state transition", appErr)
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("ClaimDownload() details = %#v", appErr.Details)
	}
	if details["is_expired"] != true || details["can_refresh"] != true {
		t.Fatalf("ClaimDownload() details = %#v", details)
	}

	current, appErr := svc.GetJob(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("GetJob() after expired claim unexpected error: %+v", appErr)
	}
	if current.EventCount != 10 {
		t.Fatalf("event_count after expired claim = %d, want 10", current.EventCount)
	}
	if current.LatestEvent == nil || current.LatestEvent.EventType != domain.ExportJobEventDownloadExpired {
		t.Fatalf("latest_event after expired claim = %+v", current.LatestEvent)
	}

	_, appErr = svc.ReadDownload(ctx, job.ExportJobID)
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("ReadDownload() error = %+v, want invalid state transition", appErr)
	}
	current, appErr = svc.GetJob(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("GetJob() after expired read unexpected error: %+v", appErr)
	}
	if current.EventCount != 10 {
		t.Fatalf("event_count after expired read = %d, want 10", current.EventCount)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 9, 6, 0, 0, time.UTC)
	}
	refreshed, appErr := svc.RefreshDownload(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("RefreshDownload() unexpected error: %+v", appErr)
	}
	if refreshed.ResultRef == nil || refreshed.ResultRef.RefKey == originalRefKey {
		t.Fatalf("RefreshDownload() result_ref = %+v", refreshed.ResultRef)
	}
	if refreshed.ExpiresAt == nil || !refreshed.ExpiresAt.After(time.Date(2026, 3, 10, 9, 6, 0, 0, time.UTC)) {
		t.Fatalf("RefreshDownload() expires_at = %+v", refreshed.ExpiresAt)
	}
	if refreshed.IsExpired || refreshed.CanRefresh || !refreshed.ClaimAvailable || !refreshed.ReadAvailable {
		t.Fatalf("RefreshDownload() handoff = %+v", refreshed)
	}
	if refreshed.ClaimedAt != nil || refreshed.LastReadAt != nil {
		t.Fatalf("RefreshDownload() audit = %+v", refreshed)
	}

	current, appErr = svc.GetJob(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("GetJob() after refresh unexpected error: %+v", appErr)
	}
	if current.EventCount != 12 {
		t.Fatalf("event_count after refresh = %d, want 12", current.EventCount)
	}
	if current.LatestEvent == nil || current.LatestEvent.EventType != domain.ExportJobEventDownloadRefreshed {
		t.Fatalf("latest_event after refresh = %+v", current.LatestEvent)
	}
	if current.ResultRef == nil || current.ResultRef.RefKey == originalRefKey {
		t.Fatalf("current.ResultRef = %+v", current.ResultRef)
	}
	if current.IsExpired || current.CanRefresh {
		t.Fatalf("current expiry flags = %+v", current)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 9, 7, 0, 0, time.UTC)
	}
	claim, appErr := svc.ClaimDownload(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("ClaimDownload() after refresh unexpected error: %+v", appErr)
	}
	if claim.ClaimedAt == nil || claim.ClaimedByActorID == nil || *claim.ClaimedByActorID != 301 {
		t.Fatalf("ClaimDownload() after refresh audit = %+v", claim)
	}

	events, appErr := svc.ListJobEvents(ctx, job.ExportJobID)
	if appErr != nil {
		t.Fatalf("ListJobEvents() unexpected error: %+v", appErr)
	}
	if len(events) != 13 {
		t.Fatalf("event length = %d, want 13", len(events))
	}
	if eventTypes(events)[9] != domain.ExportJobEventDownloadExpired || eventTypes(events)[10] != domain.ExportJobEventResultRefUpdated || eventTypes(events)[11] != domain.ExportJobEventDownloadRefreshed || eventTypes(events)[12] != domain.ExportJobEventDownloadClaimed {
		t.Fatalf("event timeline = %+v", eventTypes(events))
	}
}

func TestExportCenterServiceRejectsRefreshBeforeExpiry(t *testing.T) {
	repoStub := newExportJobRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, newExportJobDispatchRepoStub(), newExportJobAttemptRepoStub(), eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	}

	job, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
		QueryTemplate:   &domain.TaskQueryTemplate{TaskType: "new_product_development"},
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 10, 1, 0, 0, time.UTC)
	}
	if _, appErr = svc.AdvanceJob(context.Background(), job.ExportJobID, AdvanceExportJobParams{
		Action: domain.ExportJobAdvanceActionStart,
	}); appErr != nil {
		t.Fatalf("AdvanceJob(start) unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 10, 2, 0, 0, time.UTC)
	}
	if _, appErr = svc.AdvanceJob(context.Background(), job.ExportJobID, AdvanceExportJobParams{
		Action: domain.ExportJobAdvanceActionMarkReady,
	}); appErr != nil {
		t.Fatalf("AdvanceJob(mark_ready) unexpected error: %+v", appErr)
	}

	_, appErr = svc.RefreshDownload(context.Background(), job.ExportJobID)
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("RefreshDownload() error = %+v, want invalid state transition", appErr)
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("RefreshDownload() details = %#v", appErr.Details)
	}
	if details["is_expired"] != false || details["can_refresh"] != false {
		t.Fatalf("RefreshDownload() details = %#v", details)
	}
}

func TestExportCenterServiceListJobAttemptsAcrossRetry(t *testing.T) {
	repoStub := newExportJobRepoStub()
	attemptRepoStub := newExportJobAttemptRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, newExportJobDispatchRepoStub(), attemptRepoStub, eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 11, 0, 0, 0, time.UTC)
	}

	job, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
		QueryTemplate:   &domain.TaskQueryTemplate{TaskType: "new_product_development"},
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 11, 1, 0, 0, time.UTC)
	}
	if _, appErr = svc.StartJob(context.Background(), job.ExportJobID); appErr != nil {
		t.Fatalf("StartJob() unexpected error: %+v", appErr)
	}
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 11, 2, 0, 0, time.UTC)
	}
	job, appErr = svc.AdvanceJob(context.Background(), job.ExportJobID, AdvanceExportJobParams{
		Action:        domain.ExportJobAdvanceActionFail,
		FailureReason: "first placeholder attempt failed",
	})
	if appErr != nil {
		t.Fatalf("AdvanceJob(fail) unexpected error: %+v", appErr)
	}
	if job.AttemptCount != 1 || job.LatestAttempt == nil || job.LatestAttempt.Status != domain.ExportJobAttemptStatusFailed {
		t.Fatalf("failed attempt summary = %+v", job.LatestAttempt)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 11, 3, 0, 0, time.UTC)
	}
	job, appErr = svc.AdvanceJob(context.Background(), job.ExportJobID, AdvanceExportJobParams{
		Action: domain.ExportJobAdvanceActionRequeue,
	})
	if appErr != nil {
		t.Fatalf("AdvanceJob(requeue) unexpected error: %+v", appErr)
	}
	if !job.CanRetry || !job.CanStart {
		t.Fatalf("requeue retry flags = %+v", job)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 11, 4, 0, 0, time.UTC)
	}
	job, appErr = svc.AdvanceJob(context.Background(), job.ExportJobID, AdvanceExportJobParams{
		Action: domain.ExportJobAdvanceActionStart,
	})
	if appErr != nil {
		t.Fatalf("AdvanceJob(start retry) unexpected error: %+v", appErr)
	}
	if job.AttemptCount != 2 || job.LatestAttempt == nil || job.LatestAttempt.AttemptNo != 2 || job.LatestAttempt.Status != domain.ExportJobAttemptStatusRunning {
		t.Fatalf("retry attempt summary = %+v", job.LatestAttempt)
	}

	attempts, appErr := svc.ListJobAttempts(context.Background(), job.ExportJobID)
	if appErr != nil {
		t.Fatalf("ListJobAttempts() unexpected error: %+v", appErr)
	}
	if len(attempts) != 2 {
		t.Fatalf("attempt length = %d, want 2", len(attempts))
	}
	if attempts[0].AttemptNo != 2 || attempts[0].Status != domain.ExportJobAttemptStatusRunning {
		t.Fatalf("latest attempt = %+v", attempts[0])
	}
	if !attempts[0].BlocksNewAttempt || attempts[0].NextAttemptAdmissionReason != "attempt_running_blocks_new_attempt" {
		t.Fatalf("latest attempt admission = %+v", attempts[0])
	}
	if attempts[1].AttemptNo != 1 || attempts[1].Status != domain.ExportJobAttemptStatusFailed || attempts[1].ErrorMessage != "first placeholder attempt failed" {
		t.Fatalf("first attempt = %+v", attempts[1])
	}
	if attempts[1].BlocksNewAttempt || attempts[1].NextAttemptAdmissionReason != "attempt_failed_requires_job_requeue" {
		t.Fatalf("first attempt admission = %+v", attempts[1])
	}
}

func TestExportCenterServiceManualDispatchLifecycle(t *testing.T) {
	repoStub := newExportJobRepoStub()
	dispatchRepoStub := newExportJobDispatchRepoStub()
	attemptRepoStub := newExportJobAttemptRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, dispatchRepoStub, attemptRepoStub, eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	}

	job, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
		QueryTemplate:   &domain.TaskQueryTemplate{TaskType: "new_product_development"},
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 12, 1, 0, 0, time.UTC)
	}
	dispatch, appErr := svc.CreateJobDispatch(context.Background(), job.ExportJobID, CreateExportDispatchParams{
		TriggerSource: "manual_test_dispatch",
		Remark:        "submitted for adapter handoff",
	})
	if appErr != nil {
		t.Fatalf("CreateJobDispatch() unexpected error: %+v", appErr)
	}
	if dispatch.Status != domain.ExportJobDispatchStatusSubmitted || dispatch.DispatchNo != 1 {
		t.Fatalf("dispatch = %+v", dispatch)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 12, 2, 0, 0, time.UTC)
	}
	dispatch, appErr = svc.AdvanceJobDispatch(context.Background(), job.ExportJobID, dispatch.DispatchID, AdvanceExportDispatchParams{
		Action: "receive",
		Remark: "adapter accepted dispatch",
	})
	if appErr != nil {
		t.Fatalf("AdvanceJobDispatch(receive) unexpected error: %+v", appErr)
	}
	if dispatch.Status != domain.ExportJobDispatchStatusReceived || dispatch.ReceivedAt == nil {
		t.Fatalf("received dispatch = %+v", dispatch)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 12, 3, 0, 0, time.UTC)
	}
	job, appErr = svc.StartJob(context.Background(), job.ExportJobID)
	if appErr != nil {
		t.Fatalf("StartJob() unexpected error: %+v", appErr)
	}
	if job.LatestAttempt == nil || job.LatestAttempt.DispatchID != dispatch.DispatchID {
		t.Fatalf("latest_attempt = %+v", job.LatestAttempt)
	}

	dispatches, appErr := svc.ListJobDispatches(context.Background(), job.ExportJobID)
	if appErr != nil {
		t.Fatalf("ListJobDispatches() unexpected error: %+v", appErr)
	}
	if len(dispatches) != 1 || dispatches[0].Status != domain.ExportJobDispatchStatusReceived {
		t.Fatalf("dispatches = %+v", dispatches)
	}
	if !dispatches[0].StartAdmissible || dispatches[0].StartAdmissionReason != domain.ExportJobDispatchStartAdmissionReasonReceivedStartAdmitted {
		t.Fatalf("dispatch start admission = %+v", dispatches[0])
	}
}

func TestExportCenterServiceDispatchTerminalPaths(t *testing.T) {
	repoStub := newExportJobRepoStub()
	dispatchRepoStub := newExportJobDispatchRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, dispatchRepoStub, newExportJobAttemptRepoStub(), eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 12, 10, 0, 0, time.UTC)
	}

	job, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
		QueryTemplate:   &domain.TaskQueryTemplate{TaskType: "new_product_development"},
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}

	rejectedDispatch, appErr := svc.CreateJobDispatch(context.Background(), job.ExportJobID, CreateExportDispatchParams{})
	if appErr != nil {
		t.Fatalf("CreateJobDispatch(rejected) unexpected error: %+v", appErr)
	}
	rejectedDispatch, appErr = svc.AdvanceJobDispatch(context.Background(), job.ExportJobID, rejectedDispatch.DispatchID, AdvanceExportDispatchParams{
		Action: "reject",
		Reason: "adapter rejected request",
	})
	if appErr != nil {
		t.Fatalf("AdvanceJobDispatch(reject) unexpected error: %+v", appErr)
	}
	if rejectedDispatch.Status != domain.ExportJobDispatchStatusRejected {
		t.Fatalf("rejected dispatch = %+v", rejectedDispatch)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 12, 11, 0, 0, time.UTC)
	}
	receivedDispatch, appErr := svc.CreateJobDispatch(context.Background(), job.ExportJobID, CreateExportDispatchParams{})
	if appErr != nil {
		t.Fatalf("CreateJobDispatch(received) unexpected error: %+v", appErr)
	}
	receivedDispatch, appErr = svc.AdvanceJobDispatch(context.Background(), job.ExportJobID, receivedDispatch.DispatchID, AdvanceExportDispatchParams{
		Action: "receive",
	})
	if appErr != nil {
		t.Fatalf("AdvanceJobDispatch(receive) unexpected error: %+v", appErr)
	}
	notExecutedDispatch, appErr := svc.AdvanceJobDispatch(context.Background(), job.ExportJobID, receivedDispatch.DispatchID, AdvanceExportDispatchParams{
		Action: "mark_not_executed",
		Reason: "adapter accepted but did not execute",
	})
	if appErr != nil {
		t.Fatalf("AdvanceJobDispatch(mark_not_executed) unexpected error: %+v", appErr)
	}
	if notExecutedDispatch.Status != domain.ExportJobDispatchStatusNotExecuted {
		t.Fatalf("not executed dispatch = %+v", notExecutedDispatch)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 12, 12, 0, 0, time.UTC)
	}
	expiredDispatch, appErr := svc.CreateJobDispatch(context.Background(), job.ExportJobID, CreateExportDispatchParams{})
	if appErr != nil {
		t.Fatalf("CreateJobDispatch(expired) unexpected error: %+v", appErr)
	}
	expiredDispatch, appErr = svc.AdvanceJobDispatch(context.Background(), job.ExportJobID, expiredDispatch.DispatchID, AdvanceExportDispatchParams{
		Action: "expire",
	})
	if appErr != nil {
		t.Fatalf("AdvanceJobDispatch(expire) unexpected error: %+v", appErr)
	}
	if expiredDispatch.Status != domain.ExportJobDispatchStatusExpired || expiredDispatch.FinishedAt == nil {
		t.Fatalf("expired dispatch = %+v", expiredDispatch)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 12, 13, 0, 0, time.UTC)
	}
	blockingDispatch, appErr := svc.CreateJobDispatch(context.Background(), job.ExportJobID, CreateExportDispatchParams{})
	if appErr != nil {
		t.Fatalf("CreateJobDispatch(blocking) unexpected error: %+v", appErr)
	}
	current, appErr := svc.GetJob(context.Background(), job.ExportJobID)
	if appErr != nil {
		t.Fatalf("GetJob() unexpected error: %+v", appErr)
	}
	if current.DispatchCount != 4 || current.LatestDispatch == nil || current.LatestDispatch.DispatchID != blockingDispatch.DispatchID {
		t.Fatalf("current dispatch summary = %+v", current)
	}
	if current.CanStart || current.CanDispatch || current.CanRedispatch {
		t.Fatalf("current dispatch flags = %+v", current)
	}
	if current.CanDispatchReason != domain.ExportJobAdmissionReasonLatestDispatchSubmittedPendingResolution || current.CanAttemptReason != domain.ExportJobAdmissionReasonLatestDispatchSubmittedPendingResolution {
		t.Fatalf("current admission reasons = can_dispatch:%s can_attempt:%s", current.CanDispatchReason, current.CanAttemptReason)
	}
	if current.LatestAdmissionDecision == nil || current.LatestAdmissionDecision.Allowed || current.LatestAdmissionDecision.Reason != domain.ExportJobAdmissionReasonLatestDispatchSubmittedPendingResolution {
		t.Fatalf("current latest_admission_decision = %+v", current.LatestAdmissionDecision)
	}
	if current.LatestDispatchEvent == nil || current.LatestDispatchEvent.EventType != domain.ExportJobEventDispatchSubmitted {
		t.Fatalf("current latest_dispatch_event = %+v", current.LatestDispatchEvent)
	}
	_, appErr = svc.StartJob(context.Background(), job.ExportJobID)
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("StartJob() error = %+v, want invalid state transition", appErr)
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok || details["latest_dispatch"] == nil {
		t.Fatalf("StartJob() details = %#v", appErr.Details)
	}
	if details["can_start_reason"] != domain.ExportJobAdmissionReasonLatestDispatchSubmittedPendingResolution || details["can_attempt_reason"] != domain.ExportJobAdmissionReasonLatestDispatchSubmittedPendingResolution {
		t.Fatalf("StartJob() admission details = %#v", details)
	}
	if blockingDispatch.Status != domain.ExportJobDispatchStatusSubmitted {
		t.Fatalf("blocking dispatch = %+v", blockingDispatch)
	}
}

func TestExportCenterServiceAdmissionReasonsAcrossDispatchStates(t *testing.T) {
	repoStub := newExportJobRepoStub()
	dispatchRepoStub := newExportJobDispatchRepoStub()
	eventRepoStub := newExportJobEventRepoStub()
	svc := NewExportCenterService(repoStub, dispatchRepoStub, newExportJobAttemptRepoStub(), eventRepoStub, noopTxRunner{}).(*exportCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 11, 8, 0, 0, 0, time.UTC)
	}

	job, appErr := svc.CreateJob(context.Background(), CreateExportJobParams{
		ExportType:      domain.ExportTypeTaskList,
		SourceQueryType: domain.ExportSourceQueryTypeTaskQuery,
		QueryTemplate:   &domain.TaskQueryTemplate{TaskType: "new_product_development"},
	})
	if appErr != nil {
		t.Fatalf("CreateJob() unexpected error: %+v", appErr)
	}
	if !job.CanDispatch || job.CanDispatchReason != domain.ExportJobAdmissionReasonQueuedWithoutDispatch {
		t.Fatalf("initial dispatch admission = %+v", job)
	}
	if !job.CanAttempt || job.CanAttemptReason != domain.ExportJobAdmissionReasonNoDispatchAutoPlaceholderAllowed {
		t.Fatalf("initial attempt admission = %+v", job)
	}

	dispatch, appErr := svc.CreateJobDispatch(context.Background(), job.ExportJobID, CreateExportDispatchParams{})
	if appErr != nil {
		t.Fatalf("CreateJobDispatch() unexpected error: %+v", appErr)
	}
	if dispatch.StartAdmissible || dispatch.StartAdmissionReason != domain.ExportJobDispatchStartAdmissionReasonSubmittedPending {
		t.Fatalf("submitted dispatch admission = %+v", dispatch)
	}

	job, appErr = svc.GetJob(context.Background(), job.ExportJobID)
	if appErr != nil {
		t.Fatalf("GetJob(submitted) unexpected error: %+v", appErr)
	}
	if job.CanDispatch || job.CanAttempt {
		t.Fatalf("submitted job flags = %+v", job)
	}
	if job.CanDispatchReason != domain.ExportJobAdmissionReasonLatestDispatchSubmittedPendingResolution || job.CanAttemptReason != domain.ExportJobAdmissionReasonLatestDispatchSubmittedPendingResolution {
		t.Fatalf("submitted admission reasons = dispatch:%s attempt:%s", job.CanDispatchReason, job.CanAttemptReason)
	}

	dispatch, appErr = svc.AdvanceJobDispatch(context.Background(), job.ExportJobID, dispatch.DispatchID, AdvanceExportDispatchParams{Action: "receive"})
	if appErr != nil {
		t.Fatalf("AdvanceJobDispatch(receive) unexpected error: %+v", appErr)
	}
	if !dispatch.StartAdmissible || dispatch.StartAdmissionReason != domain.ExportJobDispatchStartAdmissionReasonReceivedStartAdmitted {
		t.Fatalf("received dispatch admission = %+v", dispatch)
	}

	job, appErr = svc.GetJob(context.Background(), job.ExportJobID)
	if appErr != nil {
		t.Fatalf("GetJob(received) unexpected error: %+v", appErr)
	}
	if job.CanDispatch {
		t.Fatalf("received can_dispatch = true, want false")
	}
	if !job.CanAttempt {
		t.Fatalf("received can_attempt = false, want true")
	}
	if job.CanDispatchReason != domain.ExportJobAdmissionReasonLatestDispatchReceivedPendingStartOrResolution || job.CanAttemptReason != domain.ExportJobAdmissionReasonLatestDispatchReceivedStartAllowed {
		t.Fatalf("received admission reasons = dispatch:%s attempt:%s", job.CanDispatchReason, job.CanAttemptReason)
	}
}

type exportJobRepoStub struct {
	nextID int64
	jobs   map[int64]*domain.ExportJob
}

func newExportJobRepoStub() *exportJobRepoStub {
	return &exportJobRepoStub{
		nextID: 1,
		jobs:   map[int64]*domain.ExportJob{},
	}
}

func (r *exportJobRepoStub) Create(_ context.Context, _ repo.Tx, job *domain.ExportJob) (int64, error) {
	if job == nil {
		return 0, fmt.Errorf("job is nil")
	}
	copyJob := *job
	copyJob.ExportJobID = r.nextID
	r.jobs[r.nextID] = &copyJob
	r.nextID++
	return copyJob.ExportJobID, nil
}

func (r *exportJobRepoStub) GetByID(_ context.Context, id int64) (*domain.ExportJob, error) {
	job, ok := r.jobs[id]
	if !ok {
		return nil, nil
	}
	copyJob := *job
	return &copyJob, nil
}

func (r *exportJobRepoStub) List(_ context.Context, filter repo.ExportJobListFilter) ([]*domain.ExportJob, int64, error) {
	out := make([]*domain.ExportJob, 0, len(r.jobs))
	for _, job := range r.jobs {
		if filter.Status != nil && job.Status != *filter.Status {
			continue
		}
		if filter.SourceQueryType != nil && job.SourceQueryType != *filter.SourceQueryType {
			continue
		}
		if filter.RequestedByID != nil && job.RequestedBy.ID != *filter.RequestedByID {
			continue
		}
		copyJob := *job
		out = append(out, &copyJob)
	}
	return out, int64(len(out)), nil
}

func (r *exportJobRepoStub) UpdateLifecycle(_ context.Context, _ repo.Tx, update repo.ExportJobLifecycleUpdate) error {
	job, ok := r.jobs[update.ExportJobID]
	if !ok {
		return fmt.Errorf("job not found")
	}
	job.Status = update.Status
	job.LatestStatusAt = update.LatestStatusAt
	job.FinishedAt = update.FinishedAt
	job.ResultRef = update.ResultRef
	job.Remark = update.Remark
	job.UpdatedAt = update.LatestStatusAt
	domain.HydrateExportJobDerived(job)
	return nil
}

type exportJobDispatchRepoStub struct {
	dispatchesByJob map[int64][]*domain.ExportJobDispatch
}

func newExportJobDispatchRepoStub() *exportJobDispatchRepoStub {
	return &exportJobDispatchRepoStub{
		dispatchesByJob: map[int64][]*domain.ExportJobDispatch{},
	}
}

func (r *exportJobDispatchRepoStub) Create(_ context.Context, _ repo.Tx, dispatch *domain.ExportJobDispatch) (*domain.ExportJobDispatch, error) {
	if dispatch == nil {
		return nil, fmt.Errorf("dispatch is nil")
	}
	copyDispatch := *dispatch
	copyDispatch.DispatchNo = len(r.dispatchesByJob[dispatch.ExportJobID]) + 1
	if strings.TrimSpace(copyDispatch.DispatchID) == "" {
		copyDispatch.DispatchID = fmt.Sprintf("dsp-%d-%d", dispatch.ExportJobID, copyDispatch.DispatchNo)
	}
	domain.HydrateExportJobDispatchDerived(&copyDispatch)
	r.dispatchesByJob[dispatch.ExportJobID] = append(r.dispatchesByJob[dispatch.ExportJobID], &copyDispatch)
	return &copyDispatch, nil
}

func (r *exportJobDispatchRepoStub) GetByDispatchID(_ context.Context, dispatchID string) (*domain.ExportJobDispatch, error) {
	for _, dispatches := range r.dispatchesByJob {
		for _, dispatch := range dispatches {
			if dispatch.DispatchID != dispatchID {
				continue
			}
			copyDispatch := *dispatch
			domain.HydrateExportJobDispatchDerived(&copyDispatch)
			return &copyDispatch, nil
		}
	}
	return nil, nil
}

func (r *exportJobDispatchRepoStub) GetLatestByExportJobID(_ context.Context, exportJobID int64) (*domain.ExportJobDispatch, error) {
	dispatches := r.dispatchesByJob[exportJobID]
	if len(dispatches) == 0 {
		return nil, nil
	}
	copyDispatch := *dispatches[len(dispatches)-1]
	domain.HydrateExportJobDispatchDerived(&copyDispatch)
	return &copyDispatch, nil
}

func (r *exportJobDispatchRepoStub) ListByExportJobID(_ context.Context, exportJobID int64) ([]*domain.ExportJobDispatch, error) {
	source := r.dispatchesByJob[exportJobID]
	out := make([]*domain.ExportJobDispatch, 0, len(source))
	for i := len(source) - 1; i >= 0; i-- {
		copyDispatch := *source[i]
		domain.HydrateExportJobDispatchDerived(&copyDispatch)
		out = append(out, &copyDispatch)
	}
	return out, nil
}

func (r *exportJobDispatchRepoStub) Update(_ context.Context, _ repo.Tx, update repo.ExportJobDispatchUpdate) error {
	for _, dispatches := range r.dispatchesByJob {
		for _, dispatch := range dispatches {
			if dispatch.DispatchID != update.DispatchID {
				continue
			}
			dispatch.Status = update.Status
			dispatch.ReceivedAt = update.ReceivedAt
			dispatch.FinishedAt = update.FinishedAt
			dispatch.ExpiresAt = update.ExpiresAt
			dispatch.StatusReason = update.StatusReason
			dispatch.AdapterNote = update.AdapterNote
			if update.FinishedAt != nil {
				dispatch.UpdatedAt = update.FinishedAt.UTC()
			} else if update.ReceivedAt != nil {
				dispatch.UpdatedAt = update.ReceivedAt.UTC()
			}
			return nil
		}
	}
	return fmt.Errorf("dispatch not found")
}

func (r *exportJobDispatchRepoStub) SummariesByExportJobIDs(_ context.Context, exportJobIDs []int64) (map[int64]repo.ExportJobDispatchAggregate, error) {
	out := make(map[int64]repo.ExportJobDispatchAggregate, len(exportJobIDs))
	for _, exportJobID := range exportJobIDs {
		dispatches := r.dispatchesByJob[exportJobID]
		aggregate := repo.ExportJobDispatchAggregate{
			DispatchCount: int64(len(dispatches)),
		}
		if len(dispatches) > 0 {
			copyDispatch := *dispatches[len(dispatches)-1]
			domain.HydrateExportJobDispatchDerived(&copyDispatch)
			aggregate.LatestDispatch = &copyDispatch
		}
		out[exportJobID] = aggregate
	}
	return out, nil
}

type exportJobEventRepoStub struct {
	eventsByJob map[int64][]*domain.ExportJobEvent
}

func newExportJobEventRepoStub() *exportJobEventRepoStub {
	return &exportJobEventRepoStub{
		eventsByJob: map[int64][]*domain.ExportJobEvent{},
	}
}

func (r *exportJobEventRepoStub) Append(_ context.Context, _ repo.Tx, event *domain.ExportJobEvent) (*domain.ExportJobEvent, error) {
	if event == nil {
		return nil, fmt.Errorf("event is nil")
	}
	copyEvent := *event
	copyEvent.EventID = fmt.Sprintf("evt-%d-%d", event.ExportJobID, len(r.eventsByJob[event.ExportJobID])+1)
	copyEvent.Sequence = int64(len(r.eventsByJob[event.ExportJobID]) + 1)
	if len(event.Payload) > 0 {
		copyEvent.Payload = append(json.RawMessage(nil), event.Payload...)
	}
	r.eventsByJob[event.ExportJobID] = append(r.eventsByJob[event.ExportJobID], &copyEvent)
	return &copyEvent, nil
}

func (r *exportJobEventRepoStub) ListByExportJobID(_ context.Context, exportJobID int64) ([]*domain.ExportJobEvent, error) {
	source := r.eventsByJob[exportJobID]
	out := make([]*domain.ExportJobEvent, 0, len(source))
	for _, event := range source {
		copyEvent := *event
		if len(event.Payload) > 0 {
			copyEvent.Payload = append(json.RawMessage(nil), event.Payload...)
		}
		out = append(out, &copyEvent)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Sequence < out[j].Sequence
	})
	return out, nil
}

func (r *exportJobEventRepoStub) ListRecent(_ context.Context, filter repo.ExportJobEventListFilter) ([]*domain.ExportJobEvent, int64, error) {
	out := make([]*domain.ExportJobEvent, 0)
	for exportJobID, events := range r.eventsByJob {
		if filter.ExportJobID != nil && exportJobID != *filter.ExportJobID {
			continue
		}
		for _, event := range events {
			if filter.EventType != "" && event.EventType != filter.EventType {
				continue
			}
			copyEvent := *event
			if len(event.Payload) > 0 {
				copyEvent.Payload = append(json.RawMessage(nil), event.Payload...)
			}
			out = append(out, &copyEvent)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, int64(len(out)), nil
}

func (r *exportJobEventRepoStub) SummariesByExportJobIDs(_ context.Context, exportJobIDs []int64) (map[int64]repo.ExportJobEventAggregate, error) {
	out := make(map[int64]repo.ExportJobEventAggregate, len(exportJobIDs))
	for _, exportJobID := range exportJobIDs {
		events := r.eventsByJob[exportJobID]
		aggregate := repo.ExportJobEventAggregate{
			EventCount: int64(len(events)),
		}
		if len(events) > 0 {
			aggregate.LatestEvent = domain.SummarizeExportJobEvent(events[len(events)-1])
		}
		out[exportJobID] = aggregate
	}
	return out, nil
}

func (r *exportJobEventRepoStub) LatestSummariesByExportJobIDsAndTypes(_ context.Context, exportJobIDs []int64, eventTypes []string) (map[int64]*domain.ExportJobEventSummary, error) {
	out := make(map[int64]*domain.ExportJobEventSummary, len(exportJobIDs))
	if len(eventTypes) == 0 {
		return out, nil
	}
	allowed := make(map[string]struct{}, len(eventTypes))
	for _, eventType := range eventTypes {
		allowed[eventType] = struct{}{}
	}
	for _, exportJobID := range exportJobIDs {
		events := r.eventsByJob[exportJobID]
		for i := len(events) - 1; i >= 0; i-- {
			event := events[i]
			if _, ok := allowed[event.EventType]; !ok {
				continue
			}
			out[exportJobID] = domain.SummarizeExportJobEvent(event)
			break
		}
	}
	return out, nil
}

type exportJobAttemptRepoStub struct {
	attemptsByJob map[int64][]*domain.ExportJobAttempt
}

func newExportJobAttemptRepoStub() *exportJobAttemptRepoStub {
	return &exportJobAttemptRepoStub{
		attemptsByJob: map[int64][]*domain.ExportJobAttempt{},
	}
}

func (r *exportJobAttemptRepoStub) Create(_ context.Context, _ repo.Tx, attempt *domain.ExportJobAttempt) (*domain.ExportJobAttempt, error) {
	if attempt == nil {
		return nil, fmt.Errorf("attempt is nil")
	}
	copyAttempt := *attempt
	copyAttempt.AttemptNo = len(r.attemptsByJob[attempt.ExportJobID]) + 1
	if strings.TrimSpace(copyAttempt.AttemptID) == "" {
		copyAttempt.AttemptID = fmt.Sprintf("att-%d-%d", attempt.ExportJobID, copyAttempt.AttemptNo)
	}
	domain.HydrateExportJobAttemptDerived(&copyAttempt)
	r.attemptsByJob[attempt.ExportJobID] = append(r.attemptsByJob[attempt.ExportJobID], &copyAttempt)
	return &copyAttempt, nil
}

func (r *exportJobAttemptRepoStub) GetLatestByExportJobID(_ context.Context, exportJobID int64) (*domain.ExportJobAttempt, error) {
	attempts := r.attemptsByJob[exportJobID]
	if len(attempts) == 0 {
		return nil, nil
	}
	copyAttempt := *attempts[len(attempts)-1]
	domain.HydrateExportJobAttemptDerived(&copyAttempt)
	return &copyAttempt, nil
}

func (r *exportJobAttemptRepoStub) ListByExportJobID(_ context.Context, exportJobID int64) ([]*domain.ExportJobAttempt, error) {
	source := r.attemptsByJob[exportJobID]
	out := make([]*domain.ExportJobAttempt, 0, len(source))
	for i := len(source) - 1; i >= 0; i-- {
		copyAttempt := *source[i]
		domain.HydrateExportJobAttemptDerived(&copyAttempt)
		out = append(out, &copyAttempt)
	}
	return out, nil
}

func (r *exportJobAttemptRepoStub) Update(_ context.Context, _ repo.Tx, update repo.ExportJobAttemptUpdate) error {
	for _, attempts := range r.attemptsByJob {
		for _, attempt := range attempts {
			if attempt.AttemptID != update.AttemptID {
				continue
			}
			attempt.Status = update.Status
			attempt.FinishedAt = update.FinishedAt
			attempt.ErrorMessage = update.ErrorMessage
			attempt.AdapterNote = update.AdapterNote
			if update.FinishedAt != nil {
				attempt.UpdatedAt = update.FinishedAt.UTC()
			}
			return nil
		}
	}
	return fmt.Errorf("attempt not found")
}

func (r *exportJobAttemptRepoStub) SummariesByExportJobIDs(_ context.Context, exportJobIDs []int64) (map[int64]repo.ExportJobAttemptAggregate, error) {
	out := make(map[int64]repo.ExportJobAttemptAggregate, len(exportJobIDs))
	for _, exportJobID := range exportJobIDs {
		attempts := r.attemptsByJob[exportJobID]
		aggregate := repo.ExportJobAttemptAggregate{
			AttemptCount: int64(len(attempts)),
		}
		if len(attempts) > 0 {
			copyAttempt := *attempts[len(attempts)-1]
			domain.HydrateExportJobAttemptDerived(&copyAttempt)
			aggregate.LatestAttempt = &copyAttempt
		}
		out[exportJobID] = aggregate
	}
	return out, nil
}

func boolPtr(v bool) *bool {
	return &v
}

func eventTypes(events []*domain.ExportJobEvent) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, event.EventType)
	}
	return out
}
