package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestIntegrationCenterServiceListConnectorsIncludesERPBridgeProductUpsert(t *testing.T) {
	svc := NewIntegrationCenterService(newIntegrationCallLogRepoStub(), newIntegrationExecutionRepoStub(), noopTxRunner{})

	connectors, appErr := svc.ListConnectors(context.Background())
	if appErr != nil {
		t.Fatalf("ListConnectors() unexpected error: %+v", appErr)
	}
	found := false
	for _, item := range connectors {
		if item.Key == domain.IntegrationConnectorKeyERPBridgeProductUpsert {
			found = true
			if item.PlaceholderOnly {
				t.Fatalf("connector = %+v, want non-placeholder", item)
			}
		}
	}
	if !found {
		t.Fatalf("connectors = %+v", connectors)
	}
}

func TestIntegrationCenterServiceCreateAndAdvanceCallLogCompatibility(t *testing.T) {
	callLogRepoStub := newIntegrationCallLogRepoStub()
	executionRepoStub := newIntegrationExecutionRepoStub()
	svc := NewIntegrationCenterService(callLogRepoStub, executionRepoStub, noopTxRunner{}).(*integrationCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 13, 0, 0, 0, time.UTC)
	}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       901,
		Roles:    []domain.Role{domain.RoleAdmin, domain.RoleERP},
		Source:   "header_placeholder",
		AuthMode: domain.AuthModePlaceholderNoEnforcement,
	})
	log, appErr := svc.CreateCallLog(ctx, CreateIntegrationCallLogParams{
		ConnectorKey:   domain.IntegrationConnectorKeyERPProductStub,
		OperationKey:   "products.sync.run",
		Direction:      domain.IntegrationCallDirectionOutbound,
		ResourceType:   "erp_sync_run",
		RequestPayload: json.RawMessage(`{"mode":"stub"}`),
		Remark:         "manual stub sync",
	})
	if appErr != nil {
		t.Fatalf("CreateCallLog() unexpected error: %+v", appErr)
	}
	if log.Status != domain.IntegrationCallStatusQueued || log.ProgressHint != domain.IntegrationCallProgressHintQueued || log.CanReplay {
		t.Fatalf("queued log = %+v", log)
	}
	if log.AdapterMode != domain.BoundaryAdapterModeCallLogThenExecution || log.DispatchMode != domain.BoundaryDispatchModeExecutionProgress {
		t.Fatalf("queued modes = %+v", log)
	}
	if log.PolicyMode != domain.PolicyModeRouteRoleVisibilityScaffolding || log.PolicyScopeSummary == nil {
		t.Fatalf("queued policy scaffolding = mode:%s summary:%+v", log.PolicyMode, log.PolicyScopeSummary)
	}
	if len(log.VisibleToRoles) == 0 || len(log.ActionRoles) == 0 {
		t.Fatalf("queued policy roles/actions = visible:%+v actions:%+v", log.VisibleToRoles, log.ActionRoles)
	}
	if log.AdapterRefSummary == nil || log.AdapterRefSummary.RefKey != string(domain.IntegrationConnectorKeyERPProductStub) {
		t.Fatalf("queued adapter_ref_summary = %+v", log.AdapterRefSummary)
	}
	if log.ExecutionCount != 0 || log.LatestExecution != nil {
		t.Fatalf("queued execution summary = %+v", log)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 13, 1, 0, 0, time.UTC)
	}
	log, appErr = svc.AdvanceCallLog(ctx, log.CallLogID, AdvanceIntegrationCallLogParams{
		Status: domain.IntegrationCallStatusSent,
		Remark: "sent to stub connector",
	})
	if appErr != nil {
		t.Fatalf("AdvanceCallLog(sent) unexpected error: %+v", appErr)
	}
	if log.Status != domain.IntegrationCallStatusSent || log.StartedAt == nil {
		t.Fatalf("sent log = %+v", log)
	}
	if log.ExecutionCount != 1 || log.LatestExecution == nil || log.LatestExecution.Status != domain.IntegrationExecutionStatusDispatched {
		t.Fatalf("sent execution summary = %+v", log)
	}
	if log.LatestExecution.PolicyMode != domain.PolicyModeRouteRoleVisibilityScaffolding || log.LatestExecution.PolicyScopeSummary == nil {
		t.Fatalf("sent execution policy = %+v", log.LatestExecution)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 13, 2, 0, 0, time.UTC)
	}
	log, appErr = svc.AdvanceCallLog(ctx, log.CallLogID, AdvanceIntegrationCallLogParams{
		Status:          domain.IntegrationCallStatusSucceeded,
		ResponsePayload: json.RawMessage(`{"synced":42}`),
	})
	if appErr != nil {
		t.Fatalf("AdvanceCallLog(succeeded) unexpected error: %+v", appErr)
	}
	if log.Status != domain.IntegrationCallStatusSucceeded || log.FinishedAt == nil || !log.CanReplay || log.CanRetry {
		t.Fatalf("succeeded log = %+v", log)
	}
	if log.ReplayabilityReason != "latest_succeeded_execution_replay_allowed" {
		t.Fatalf("replayability_reason = %s", log.ReplayabilityReason)
	}
	if string(log.ResponsePayload) != `{"synced":42}` {
		t.Fatalf("response_payload = %s", string(log.ResponsePayload))
	}
	if log.LatestExecution == nil || log.LatestExecution.Status != domain.IntegrationExecutionStatusCompleted || log.LatestExecution.FinishedAt == nil {
		t.Fatalf("latest execution after success = %+v", log.LatestExecution)
	}
}

func TestIntegrationCenterServiceExecutionLifecycleAndRetrySummary(t *testing.T) {
	callLogRepoStub := newIntegrationCallLogRepoStub()
	executionRepoStub := newIntegrationExecutionRepoStub()
	svc := NewIntegrationCenterService(callLogRepoStub, executionRepoStub, noopTxRunner{}).(*integrationCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 13, 10, 0, 0, time.UTC)
	}

	log, appErr := svc.CreateCallLog(context.Background(), CreateIntegrationCallLogParams{
		ConnectorKey: domain.IntegrationConnectorKeyExportAdapterBridge,
		OperationKey: "export.dispatch.push",
		Direction:    domain.IntegrationCallDirectionOutbound,
		ResourceType: "export_job",
	})
	if appErr != nil {
		t.Fatalf("CreateCallLog() unexpected error: %+v", appErr)
	}

	execution, appErr := svc.CreateExecution(context.Background(), log.CallLogID, CreateIntegrationExecutionParams{
		TriggerSource: "manual_test_start",
		AdapterNote:   "prepared by admin",
	})
	if appErr != nil {
		t.Fatalf("CreateExecution() unexpected error: %+v", appErr)
	}
	if execution.Status != domain.IntegrationExecutionStatusPrepared || execution.ExecutionNo != 1 {
		t.Fatalf("created execution = %+v", execution)
	}
	if execution.AdapterRefSummary == nil || execution.HandoffRefSummary == nil {
		t.Fatalf("created execution summaries = %+v", execution)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 13, 11, 0, 0, time.UTC)
	}
	execution, appErr = svc.AdvanceExecution(context.Background(), log.CallLogID, execution.ExecutionID, AdvanceIntegrationExecutionParams{
		Status:      domain.IntegrationExecutionStatusDispatched,
		AdapterNote: "adapter accepted dispatch",
	})
	if appErr != nil {
		t.Fatalf("AdvanceExecution(dispatched) unexpected error: %+v", appErr)
	}
	if execution.Status != domain.IntegrationExecutionStatusDispatched {
		t.Fatalf("dispatched execution = %+v", execution)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 13, 12, 0, 0, time.UTC)
	}
	execution, appErr = svc.AdvanceExecution(context.Background(), log.CallLogID, execution.ExecutionID, AdvanceIntegrationExecutionParams{
		Status:      domain.IntegrationExecutionStatusReceived,
		AdapterNote: "adapter received request",
	})
	if appErr != nil {
		t.Fatalf("AdvanceExecution(received) unexpected error: %+v", appErr)
	}
	if execution.Status != domain.IntegrationExecutionStatusReceived {
		t.Fatalf("received execution = %+v", execution)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 13, 13, 0, 0, time.UTC)
	}
	execution, appErr = svc.AdvanceExecution(context.Background(), log.CallLogID, execution.ExecutionID, AdvanceIntegrationExecutionParams{
		Status:       domain.IntegrationExecutionStatusFailed,
		ErrorMessage: "stub timeout",
		Retryable:    integrationBoolPtr(true),
	})
	if appErr != nil {
		t.Fatalf("AdvanceExecution(failed) unexpected error: %+v", appErr)
	}
	if execution.Status != domain.IntegrationExecutionStatusFailed || !execution.Retryable {
		t.Fatalf("failed execution = %+v", execution)
	}

	log, appErr = svc.GetCallLog(context.Background(), log.CallLogID)
	if appErr != nil {
		t.Fatalf("GetCallLog() unexpected error: %+v", appErr)
	}
	if log.Status != domain.IntegrationCallStatusFailed || !log.CanRetry || !log.CanReplay {
		t.Fatalf("failed log summary = %+v", log)
	}
	if log.RetryabilityReason != "latest_failed_execution_retryable" || log.ReplayabilityReason != "latest_failed_execution_replay_allowed" {
		t.Fatalf("failed reasons = retry:%s replay:%s", log.RetryabilityReason, log.ReplayabilityReason)
	}
	if log.ExecutionCount != 1 || log.LatestExecution == nil || log.LatestExecution.Status != domain.IntegrationExecutionStatusFailed {
		t.Fatalf("failed execution summary = %+v", log)
	}

	executions, appErr := svc.ListExecutions(context.Background(), log.CallLogID)
	if appErr != nil {
		t.Fatalf("ListExecutions() unexpected error: %+v", appErr)
	}
	if len(executions) != 1 || executions[0].ExecutionID != execution.ExecutionID {
		t.Fatalf("executions = %+v", executions)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 13, 14, 0, 0, time.UTC)
	}
	retried, appErr := svc.CreateExecution(context.Background(), log.CallLogID, CreateIntegrationExecutionParams{
		TriggerSource: "manual_retry",
	})
	if appErr != nil {
		t.Fatalf("CreateExecution(retry) unexpected error: %+v", appErr)
	}
	if retried.ExecutionNo != 2 || retried.Status != domain.IntegrationExecutionStatusPrepared {
		t.Fatalf("retry execution = %+v", retried)
	}

	log, appErr = svc.GetCallLog(context.Background(), log.CallLogID)
	if appErr != nil {
		t.Fatalf("GetCallLog(after retry) unexpected error: %+v", appErr)
	}
	if log.Status != domain.IntegrationCallStatusQueued || log.ExecutionCount != 2 || log.CanRetry {
		t.Fatalf("requeued log summary = %+v", log)
	}
	if log.LatestExecution == nil || log.LatestExecution.ExecutionNo != 2 || log.LatestExecution.Status != domain.IntegrationExecutionStatusPrepared {
		t.Fatalf("requeued latest execution = %+v", log.LatestExecution)
	}
	if log.RetryCount != 1 || log.LatestRetryAction == nil || log.LatestRetryAction.ExecutionNo != 2 {
		t.Fatalf("retry action summary = %+v", log)
	}
	if log.ReplayCount != 0 || log.LatestReplayAction != nil {
		t.Fatalf("unexpected replay action summary = %+v", log)
	}
}

func TestIntegrationCenterServiceReplayDistinguishesFromRetry(t *testing.T) {
	callLogRepoStub := newIntegrationCallLogRepoStub()
	executionRepoStub := newIntegrationExecutionRepoStub()
	svc := NewIntegrationCenterService(callLogRepoStub, executionRepoStub, noopTxRunner{}).(*integrationCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC)
	}

	log, appErr := svc.CreateCallLog(context.Background(), CreateIntegrationCallLogParams{
		ConnectorKey: domain.IntegrationConnectorKeyERPProductStub,
		OperationKey: "products.sync.replay",
		Direction:    domain.IntegrationCallDirectionOutbound,
	})
	if appErr != nil {
		t.Fatalf("CreateCallLog() unexpected error: %+v", appErr)
	}
	execution, appErr := svc.CreateExecution(context.Background(), log.CallLogID, CreateIntegrationExecutionParams{})
	if appErr != nil {
		t.Fatalf("CreateExecution() unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 14, 1, 0, 0, time.UTC)
	}
	_, appErr = svc.AdvanceExecution(context.Background(), log.CallLogID, execution.ExecutionID, AdvanceIntegrationExecutionParams{
		Status: domain.IntegrationExecutionStatusCompleted,
	})
	if appErr != nil {
		t.Fatalf("AdvanceExecution(completed) unexpected error: %+v", appErr)
	}

	log, appErr = svc.GetCallLog(context.Background(), log.CallLogID)
	if appErr != nil {
		t.Fatalf("GetCallLog() unexpected error: %+v", appErr)
	}
	if log.CanRetry || !log.CanReplay {
		t.Fatalf("success replayability = %+v", log)
	}
	if _, appErr = svc.RetryCallLog(context.Background(), log.CallLogID, RetryIntegrationCallLogParams{}); appErr == nil {
		t.Fatal("RetryCallLog() expected invalid state error for succeeded log")
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 14, 2, 0, 0, time.UTC)
	}
	log, appErr = svc.ReplayCallLog(context.Background(), log.CallLogID, ReplayIntegrationCallLogParams{
		AdapterNote: "manual replay after success",
	})
	if appErr != nil {
		t.Fatalf("ReplayCallLog() unexpected error: %+v", appErr)
	}
	if log.Status != domain.IntegrationCallStatusQueued || log.ExecutionCount != 2 {
		t.Fatalf("replayed log = %+v", log)
	}
	if log.LatestExecution == nil || log.LatestExecution.ActionType != domain.IntegrationExecutionActionTypeReplay {
		t.Fatalf("latest replay execution = %+v", log.LatestExecution)
	}
	if log.RetryCount != 0 || log.ReplayCount != 1 {
		t.Fatalf("action counts = retry:%d replay:%d", log.RetryCount, log.ReplayCount)
	}
	if log.LatestReplayAction == nil || log.LatestReplayAction.ActionType != domain.IntegrationExecutionActionTypeReplay || log.LatestReplayAction.ExecutionNo != 2 {
		t.Fatalf("latest replay action = %+v", log.LatestReplayAction)
	}
	if log.LatestRetryAction != nil {
		t.Fatalf("unexpected retry action = %+v", log.LatestRetryAction)
	}
}

func TestIntegrationCenterServiceCancelledCallPrefersReplay(t *testing.T) {
	callLogRepoStub := newIntegrationCallLogRepoStub()
	executionRepoStub := newIntegrationExecutionRepoStub()
	svc := NewIntegrationCenterService(callLogRepoStub, executionRepoStub, noopTxRunner{}).(*integrationCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 14, 10, 0, 0, time.UTC)
	}

	log, appErr := svc.CreateCallLog(context.Background(), CreateIntegrationCallLogParams{
		ConnectorKey: domain.IntegrationConnectorKeyExportAdapterBridge,
		OperationKey: "export.dispatch.cancelled",
		Direction:    domain.IntegrationCallDirectionOutbound,
	})
	if appErr != nil {
		t.Fatalf("CreateCallLog() unexpected error: %+v", appErr)
	}
	execution, appErr := svc.CreateExecution(context.Background(), log.CallLogID, CreateIntegrationExecutionParams{})
	if appErr != nil {
		t.Fatalf("CreateExecution() unexpected error: %+v", appErr)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 14, 11, 0, 0, time.UTC)
	}
	_, appErr = svc.AdvanceExecution(context.Background(), log.CallLogID, execution.ExecutionID, AdvanceIntegrationExecutionParams{
		Status:    domain.IntegrationExecutionStatusCancelled,
		Retryable: integrationBoolPtr(false),
	})
	if appErr != nil {
		t.Fatalf("AdvanceExecution(cancelled) unexpected error: %+v", appErr)
	}

	log, appErr = svc.GetCallLog(context.Background(), log.CallLogID)
	if appErr != nil {
		t.Fatalf("GetCallLog() unexpected error: %+v", appErr)
	}
	if log.CanRetry || !log.CanReplay {
		t.Fatalf("cancelled replayability = %+v", log)
	}
	if log.RetryabilityReason != "cancelled_call_prefers_replay" || log.ReplayabilityReason != "latest_cancelled_execution_replay_allowed" {
		t.Fatalf("cancelled reasons = retry:%s replay:%s", log.RetryabilityReason, log.ReplayabilityReason)
	}
}

type integrationCallLogRepoStub struct {
	nextID int64
	logs   map[int64]*domain.IntegrationCallLog
}

func newIntegrationCallLogRepoStub() *integrationCallLogRepoStub {
	return &integrationCallLogRepoStub{
		nextID: 1,
		logs:   map[int64]*domain.IntegrationCallLog{},
	}
}

func (r *integrationCallLogRepoStub) Create(_ context.Context, _ repo.Tx, log *domain.IntegrationCallLog) (int64, error) {
	if log == nil {
		return 0, fmt.Errorf("log is nil")
	}
	copyLog := cloneIntegrationCallLog(*log)
	copyLog.CallLogID = r.nextID
	r.logs[r.nextID] = &copyLog
	r.nextID++
	return copyLog.CallLogID, nil
}

func (r *integrationCallLogRepoStub) GetByID(_ context.Context, id int64) (*domain.IntegrationCallLog, error) {
	log, ok := r.logs[id]
	if !ok {
		return nil, nil
	}
	copyLog := cloneIntegrationCallLog(*log)
	return &copyLog, nil
}

func (r *integrationCallLogRepoStub) List(_ context.Context, filter repo.IntegrationCallLogListFilter) ([]*domain.IntegrationCallLog, int64, error) {
	out := make([]*domain.IntegrationCallLog, 0, len(r.logs))
	for _, log := range r.logs {
		if filter.ConnectorKey != nil && log.ConnectorKey != *filter.ConnectorKey {
			continue
		}
		if filter.Status != nil && log.Status != *filter.Status {
			continue
		}
		if filter.ResourceType != "" && log.ResourceType != filter.ResourceType {
			continue
		}
		if filter.ResourceID != nil && (log.ResourceID == nil || *log.ResourceID != *filter.ResourceID) {
			continue
		}
		copyLog := cloneIntegrationCallLog(*log)
		out = append(out, &copyLog)
	}
	return out, int64(len(out)), nil
}

func (r *integrationCallLogRepoStub) Update(_ context.Context, _ repo.Tx, update repo.IntegrationCallLogUpdate) error {
	log, ok := r.logs[update.CallLogID]
	if !ok {
		return fmt.Errorf("log not found")
	}
	log.Status = update.Status
	log.LatestStatusAt = update.LatestStatusAt
	log.StartedAt = update.StartedAt
	log.FinishedAt = update.FinishedAt
	log.ResponsePayload = append(json.RawMessage(nil), update.ResponsePayload...)
	log.ErrorMessage = update.ErrorMessage
	log.Remark = update.Remark
	log.UpdatedAt = update.LatestStatusAt
	domain.HydrateIntegrationCallLogDerived(log)
	return nil
}

type integrationExecutionRepoStub struct {
	executionsByCallLog map[int64][]*domain.IntegrationExecution
}

func newIntegrationExecutionRepoStub() *integrationExecutionRepoStub {
	return &integrationExecutionRepoStub{
		executionsByCallLog: map[int64][]*domain.IntegrationExecution{},
	}
}

func (r *integrationExecutionRepoStub) Create(_ context.Context, _ repo.Tx, execution *domain.IntegrationExecution) (*domain.IntegrationExecution, error) {
	if execution == nil {
		return nil, fmt.Errorf("execution is nil")
	}
	copyExecution := cloneIntegrationExecution(*execution)
	copyExecution.ExecutionNo = len(r.executionsByCallLog[execution.CallLogID]) + 1
	if copyExecution.ExecutionID == "" {
		copyExecution.ExecutionID = fmt.Sprintf("iex-%d-%d", execution.CallLogID, copyExecution.ExecutionNo)
	}
	r.executionsByCallLog[execution.CallLogID] = append(r.executionsByCallLog[execution.CallLogID], &copyExecution)
	return &copyExecution, nil
}

func (r *integrationExecutionRepoStub) GetByExecutionID(_ context.Context, executionID string) (*domain.IntegrationExecution, error) {
	for _, executions := range r.executionsByCallLog {
		for _, execution := range executions {
			if execution.ExecutionID != executionID {
				continue
			}
			copyExecution := cloneIntegrationExecution(*execution)
			return &copyExecution, nil
		}
	}
	return nil, nil
}

func (r *integrationExecutionRepoStub) GetLatestByCallLogID(_ context.Context, callLogID int64) (*domain.IntegrationExecution, error) {
	executions := r.executionsByCallLog[callLogID]
	if len(executions) == 0 {
		return nil, nil
	}
	copyExecution := cloneIntegrationExecution(*executions[len(executions)-1])
	return &copyExecution, nil
}

func (r *integrationExecutionRepoStub) ListByCallLogID(_ context.Context, callLogID int64) ([]*domain.IntegrationExecution, error) {
	source := r.executionsByCallLog[callLogID]
	out := make([]*domain.IntegrationExecution, 0, len(source))
	for idx := len(source) - 1; idx >= 0; idx-- {
		copyExecution := cloneIntegrationExecution(*source[idx])
		out = append(out, &copyExecution)
	}
	return out, nil
}

func (r *integrationExecutionRepoStub) Update(_ context.Context, _ repo.Tx, update repo.IntegrationExecutionUpdate) error {
	for _, executions := range r.executionsByCallLog {
		for _, execution := range executions {
			if execution.ExecutionID != update.ExecutionID {
				continue
			}
			execution.Status = update.Status
			execution.LatestStatusAt = update.LatestStatusAt
			execution.FinishedAt = update.FinishedAt
			execution.ErrorMessage = update.ErrorMessage
			execution.AdapterNote = update.AdapterNote
			execution.Retryable = update.Retryable
			execution.UpdatedAt = update.LatestStatusAt
			return nil
		}
	}
	return fmt.Errorf("execution not found")
}

func (r *integrationExecutionRepoStub) SummariesByCallLogIDs(_ context.Context, callLogIDs []int64) (map[int64]repo.IntegrationExecutionAggregate, error) {
	out := make(map[int64]repo.IntegrationExecutionAggregate, len(callLogIDs))
	for _, callLogID := range callLogIDs {
		executions := r.executionsByCallLog[callLogID]
		aggregate := repo.IntegrationExecutionAggregate{
			ExecutionCount: int64(len(executions)),
		}
		for _, execution := range executions {
			if execution.TriggerSource == integrationExecutionTriggerManualRetry {
				aggregate.RetryCount++
				copyExecution := cloneIntegrationExecution(*execution)
				aggregate.LatestRetryExecution = &copyExecution
			}
			if execution.TriggerSource == integrationExecutionTriggerManualReplay {
				aggregate.ReplayCount++
				copyExecution := cloneIntegrationExecution(*execution)
				aggregate.LatestReplayExecution = &copyExecution
			}
		}
		if len(executions) > 0 {
			copyExecution := cloneIntegrationExecution(*executions[len(executions)-1])
			aggregate.LatestExecution = &copyExecution
		}
		out[callLogID] = aggregate
	}
	return out, nil
}

func cloneIntegrationCallLog(log domain.IntegrationCallLog) domain.IntegrationCallLog {
	copyLog := log
	if log.ResourceID != nil {
		resourceID := *log.ResourceID
		copyLog.ResourceID = &resourceID
	}
	if log.StartedAt != nil {
		startedAt := log.StartedAt.UTC()
		copyLog.StartedAt = &startedAt
	}
	if log.FinishedAt != nil {
		finishedAt := log.FinishedAt.UTC()
		copyLog.FinishedAt = &finishedAt
	}
	if log.RequestPayload != nil {
		copyLog.RequestPayload = append(json.RawMessage(nil), log.RequestPayload...)
	}
	if log.ResponsePayload != nil {
		copyLog.ResponsePayload = append(json.RawMessage(nil), log.ResponsePayload...)
	}
	if log.LatestExecution != nil {
		latestExecution := cloneIntegrationExecution(*log.LatestExecution)
		copyLog.LatestExecution = &latestExecution
	}
	return copyLog
}

func cloneIntegrationExecution(execution domain.IntegrationExecution) domain.IntegrationExecution {
	copyExecution := execution
	if execution.FinishedAt != nil {
		finishedAt := execution.FinishedAt.UTC()
		copyExecution.FinishedAt = &finishedAt
	}
	return copyExecution
}

func integrationBoolPtr(value bool) *bool {
	return &value
}
