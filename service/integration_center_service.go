package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type CreateIntegrationCallLogParams struct {
	ConnectorKey   domain.IntegrationConnectorKey
	OperationKey   string
	Direction      domain.IntegrationCallDirection
	ResourceType   string
	ResourceID     *int64
	RequestPayload json.RawMessage
	Remark         string
}

type AdvanceIntegrationCallLogParams struct {
	Status          domain.IntegrationCallStatus
	ResponsePayload json.RawMessage
	ErrorMessage    string
	Remark          string
}

type CreateIntegrationExecutionParams struct {
	ExecutionMode domain.IntegrationExecutionMode
	TriggerSource string
	AdapterNote   string
}

type RetryIntegrationCallLogParams struct {
	ExecutionMode domain.IntegrationExecutionMode
	AdapterNote   string
}

type ReplayIntegrationCallLogParams struct {
	ExecutionMode domain.IntegrationExecutionMode
	AdapterNote   string
}

type AdvanceIntegrationExecutionParams struct {
	Status          domain.IntegrationExecutionStatus
	ResponsePayload json.RawMessage
	ErrorMessage    string
	AdapterNote     string
	Retryable       *bool
}

type IntegrationCallLogFilter struct {
	ConnectorKey *domain.IntegrationConnectorKey
	Status       *domain.IntegrationCallStatus
	ResourceType string
	ResourceID   *int64
	Page         int
	PageSize     int
}

type IntegrationCenterService interface {
	ListConnectors(ctx context.Context) ([]domain.IntegrationConnector, *domain.AppError)
	CreateCallLog(ctx context.Context, params CreateIntegrationCallLogParams) (*domain.IntegrationCallLog, *domain.AppError)
	AdvanceCallLog(ctx context.Context, id int64, params AdvanceIntegrationCallLogParams) (*domain.IntegrationCallLog, *domain.AppError)
	ListCallLogs(ctx context.Context, filter IntegrationCallLogFilter) ([]*domain.IntegrationCallLog, domain.PaginationMeta, *domain.AppError)
	GetCallLog(ctx context.Context, id int64) (*domain.IntegrationCallLog, *domain.AppError)
	CreateExecution(ctx context.Context, id int64, params CreateIntegrationExecutionParams) (*domain.IntegrationExecution, *domain.AppError)
	RetryCallLog(ctx context.Context, id int64, params RetryIntegrationCallLogParams) (*domain.IntegrationCallLog, *domain.AppError)
	ReplayCallLog(ctx context.Context, id int64, params ReplayIntegrationCallLogParams) (*domain.IntegrationCallLog, *domain.AppError)
	AdvanceExecution(ctx context.Context, id int64, executionID string, params AdvanceIntegrationExecutionParams) (*domain.IntegrationExecution, *domain.AppError)
	ListExecutions(ctx context.Context, id int64) ([]*domain.IntegrationExecution, *domain.AppError)
}

type integrationCenterService struct {
	callLogRepo   repo.IntegrationCallLogRepo
	executionRepo repo.IntegrationExecutionRepo
	txRunner      repo.TxRunner
	nowFn         func() time.Time
}

const (
	integrationExecutionTriggerManualStart  = "manual_execution_start"
	integrationExecutionTriggerCompatRoute  = "call_log_advance_compat"
	integrationExecutionTriggerManualRetry  = "manual_retry"
	integrationExecutionTriggerManualReplay = "manual_replay"
)

func NewIntegrationCenterService(callLogRepo repo.IntegrationCallLogRepo, executionRepo repo.IntegrationExecutionRepo, txRunner repo.TxRunner) IntegrationCenterService {
	return &integrationCenterService{
		callLogRepo:   callLogRepo,
		executionRepo: executionRepo,
		txRunner:      txRunner,
		nowFn:         time.Now,
	}
}

func (s *integrationCenterService) ListConnectors(_ context.Context) ([]domain.IntegrationConnector, *domain.AppError) {
	connectors := []domain.IntegrationConnector{
		{
			Key:             domain.IntegrationConnectorKeyERPProductStub,
			Name:            "ERP Product Stub",
			Description:     "Placeholder ERP product sync/integration connector over the current stub source.",
			Direction:       domain.IntegrationCallDirectionOutbound,
			PlaceholderOnly: true,
		},
		{
			Key:             domain.IntegrationConnectorKeyERPBridgeProductUpsert,
			Name:            "ERP Bridge Product Upsert",
			Description:     "Narrow ERP Bridge filing connector used by task business-info filing for existing-product tasks.",
			Direction:       domain.IntegrationCallDirectionOutbound,
			PlaceholderOnly: false,
		},
		{
			Key:             domain.IntegrationConnectorKeyERPBridgeItemStyleUpdate,
			Name:            "ERP Bridge Item Style Update",
			Description:     "Bridge-owned ERP item style update boundary for i_id-centered style changes.",
			Direction:       domain.IntegrationCallDirectionOutbound,
			PlaceholderOnly: false,
		},
		{
			Key:             domain.IntegrationConnectorKeyERPBridgeProductShelve,
			Name:            "ERP Bridge Product Shelve Batch",
			Description:     "Bridge-owned ERP batch shelve integration boundary.",
			Direction:       domain.IntegrationCallDirectionOutbound,
			PlaceholderOnly: false,
		},
		{
			Key:             domain.IntegrationConnectorKeyERPBridgeProductUnshelve,
			Name:            "ERP Bridge Product Unshelve Batch",
			Description:     "Bridge-owned ERP batch unshelve integration boundary.",
			Direction:       domain.IntegrationCallDirectionOutbound,
			PlaceholderOnly: false,
		},
		{
			Key:             domain.IntegrationConnectorKeyERPBridgeVirtualInventory,
			Name:            "ERP Bridge Virtual Inventory Update",
			Description:     "Bridge-owned ERP virtual inventory update integration boundary.",
			Direction:       domain.IntegrationCallDirectionOutbound,
			PlaceholderOnly: false,
		},
		{
			Key:             domain.IntegrationConnectorKeyExportAdapterBridge,
			Name:            "Export Adapter Bridge",
			Description:     "Placeholder connector for export adapter handoff and future runner/integration bridge.",
			Direction:       domain.IntegrationCallDirectionOutbound,
			PlaceholderOnly: true,
		},
	}
	return connectors, nil
}

func (s *integrationCenterService) CreateCallLog(ctx context.Context, params CreateIntegrationCallLogParams) (*domain.IntegrationCallLog, *domain.AppError) {
	if !params.ConnectorKey.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "connector_key is not supported", nil)
	}
	if !params.Direction.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "direction must be outbound/inbound", nil)
	}
	operationKey := strings.TrimSpace(params.OperationKey)
	if operationKey == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "operation_key is required", nil)
	}
	requestPayload, appErr := normalizeOptionalRawJSON(params.RequestPayload, "request_payload")
	if appErr != nil {
		return nil, appErr
	}

	actor, _ := resolveWorkbenchActorScope(ctx)
	now := s.nowFn().UTC()
	log := &domain.IntegrationCallLog{
		ConnectorKey:   params.ConnectorKey,
		OperationKey:   operationKey,
		Direction:      params.Direction,
		ResourceType:   strings.TrimSpace(params.ResourceType),
		ResourceID:     params.ResourceID,
		Status:         domain.IntegrationCallStatusQueued,
		RequestedBy:    actor,
		RequestPayload: requestPayload,
		Remark:         strings.TrimSpace(params.Remark),
		CreatedAt:      now,
		LatestStatusAt: now,
		UpdatedAt:      now,
	}
	domain.HydrateIntegrationCallLogDerived(log)

	var callLogID int64
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		id, err := s.callLogRepo.Create(ctx, tx, log)
		if err != nil {
			return err
		}
		callLogID = id
		return nil
	}); err != nil {
		return nil, infraError("create integration call log", err)
	}
	return s.GetCallLog(ctx, callLogID)
}

func (s *integrationCenterService) AdvanceCallLog(ctx context.Context, id int64, params AdvanceIntegrationCallLogParams) (*domain.IntegrationCallLog, *domain.AppError) {
	if !params.Status.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "status must be queued/sent/succeeded/failed/cancelled", nil)
	}
	responsePayload, appErr := normalizeOptionalRawJSON(params.ResponsePayload, "response_payload")
	if appErr != nil {
		return nil, appErr
	}
	log, appErr := s.GetCallLog(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	now := s.nowFn().UTC()

	if params.Status == domain.IntegrationCallStatusQueued {
		update, appErr := buildIntegrationCallLogRequeueUpdate(log, strings.TrimSpace(params.Remark), now)
		if appErr != nil {
			return nil, appErr
		}
		if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			return s.callLogRepo.Update(ctx, tx, update)
		}); err != nil {
			return nil, infraError("requeue integration call log", err)
		}
		return s.GetCallLog(ctx, id)
	}

	execution, appErr := s.ensureExecutionForCompatibilityAdvance(ctx, log, now)
	if appErr != nil {
		return nil, appErr
	}
	_, appErr = s.AdvanceExecution(ctx, id, execution.ExecutionID, AdvanceIntegrationExecutionParams{
		Status:          mapCallLogStatusToExecutionStatus(params.Status),
		ResponsePayload: responsePayload,
		ErrorMessage:    strings.TrimSpace(params.ErrorMessage),
		AdapterNote:     compatibilityAdvanceAdapterNote(params.Status, strings.TrimSpace(params.Remark)),
		Retryable:       compatibilityAdvanceRetryable(params.Status),
	})
	if appErr != nil {
		return nil, appErr
	}
	return s.GetCallLog(ctx, id)
}

func (s *integrationCenterService) ListCallLogs(ctx context.Context, filter IntegrationCallLogFilter) ([]*domain.IntegrationCallLog, domain.PaginationMeta, *domain.AppError) {
	if filter.ConnectorKey != nil && !filter.ConnectorKey.Valid() {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "connector_key is not supported", nil)
	}
	if filter.Status != nil && !filter.Status.Valid() {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "status must be queued/sent/succeeded/failed/cancelled", nil)
	}
	logs, total, err := s.callLogRepo.List(ctx, repo.IntegrationCallLogListFilter{
		ConnectorKey: filter.ConnectorKey,
		Status:       filter.Status,
		ResourceType: strings.TrimSpace(filter.ResourceType),
		ResourceID:   filter.ResourceID,
		Page:         filter.Page,
		PageSize:     filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list integration call logs", err)
	}
	if logs == nil {
		logs = []*domain.IntegrationCallLog{}
	}
	if err := s.hydrateExecutionSummaries(ctx, logs); err != nil {
		return nil, domain.PaginationMeta{}, infraError("hydrate integration execution summaries", err)
	}
	return logs, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *integrationCenterService) GetCallLog(ctx context.Context, id int64) (*domain.IntegrationCallLog, *domain.AppError) {
	log, err := s.callLogRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get integration call log", err)
	}
	if log == nil {
		return nil, domain.ErrNotFound
	}
	if err := s.hydrateExecutionSummaries(ctx, []*domain.IntegrationCallLog{log}); err != nil {
		return nil, infraError("hydrate integration execution summary", err)
	}
	return log, nil
}

func (s *integrationCenterService) CreateExecution(ctx context.Context, id int64, params CreateIntegrationExecutionParams) (*domain.IntegrationExecution, *domain.AppError) {
	log, appErr := s.GetCallLog(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := validateIntegrationExecutionCreate(log); appErr != nil {
		return nil, appErr
	}
	executionMode := normalizeIntegrationExecutionMode(params.ExecutionMode)
	if !executionMode.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "execution_mode must be manual_placeholder_adapter", nil)
	}

	now := s.nowFn().UTC()
	execution := &domain.IntegrationExecution{
		CallLogID:      log.CallLogID,
		ConnectorKey:   log.ConnectorKey,
		ExecutionMode:  executionMode,
		TriggerSource:  normalizeIntegrationExecutionTriggerSource(params.TriggerSource),
		Status:         domain.IntegrationExecutionStatusPrepared,
		LatestStatusAt: now,
		StartedAt:      now,
		AdapterNote:    strings.TrimSpace(params.AdapterNote),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		createdExecution, err := s.executionRepo.Create(ctx, tx, execution)
		if err != nil {
			return err
		}
		execution = createdExecution
		if log.Status != domain.IntegrationCallStatusFailed && log.Status != domain.IntegrationCallStatusCancelled {
			return nil
		}
		update, appErr := buildIntegrationCallLogRequeueUpdate(log, log.Remark, now)
		if appErr != nil {
			return appErr
		}
		return s.callLogRepo.Update(ctx, tx, update)
	}); err != nil {
		return nil, infraError("create integration execution", err)
	}
	domain.HydrateIntegrationExecutionDerived(execution)
	return execution, nil
}

func (s *integrationCenterService) RetryCallLog(ctx context.Context, id int64, params RetryIntegrationCallLogParams) (*domain.IntegrationCallLog, *domain.AppError) {
	_, appErr := s.createExecutionForAction(ctx, id, createIntegrationActionParams{
		ExecutionMode: params.ExecutionMode,
		TriggerSource: integrationExecutionTriggerManualRetry,
		AdapterNote:   strings.TrimSpace(params.AdapterNote),
	})
	if appErr != nil {
		return nil, appErr
	}
	return s.GetCallLog(ctx, id)
}

func (s *integrationCenterService) ReplayCallLog(ctx context.Context, id int64, params ReplayIntegrationCallLogParams) (*domain.IntegrationCallLog, *domain.AppError) {
	_, appErr := s.createExecutionForAction(ctx, id, createIntegrationActionParams{
		ExecutionMode: params.ExecutionMode,
		TriggerSource: integrationExecutionTriggerManualReplay,
		AdapterNote:   strings.TrimSpace(params.AdapterNote),
	})
	if appErr != nil {
		return nil, appErr
	}
	return s.GetCallLog(ctx, id)
}

func (s *integrationCenterService) AdvanceExecution(ctx context.Context, id int64, executionID string, params AdvanceIntegrationExecutionParams) (*domain.IntegrationExecution, *domain.AppError) {
	if !params.Status.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "status must be prepared/dispatched/received/completed/failed/cancelled", nil)
	}
	responsePayload, appErr := normalizeOptionalRawJSON(params.ResponsePayload, "response_payload")
	if appErr != nil {
		return nil, appErr
	}

	log, appErr := s.GetCallLog(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	execution, err := s.executionRepo.GetByExecutionID(ctx, executionID)
	if err != nil {
		return nil, infraError("get integration execution", err)
	}
	if execution == nil || execution.CallLogID != id {
		return nil, domain.ErrNotFound
	}

	now := s.nowFn().UTC()
	executionUpdate, callLogUpdate, appErr := nextIntegrationExecutionAdvance(log, execution, responsePayload, params, now)
	if appErr != nil {
		return nil, appErr
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.executionRepo.Update(ctx, tx, executionUpdate); err != nil {
			return err
		}
		return s.callLogRepo.Update(ctx, tx, callLogUpdate)
	}); err != nil {
		return nil, infraError("advance integration execution", err)
	}
	updatedExecution, err := s.executionRepo.GetByExecutionID(ctx, executionID)
	if err != nil {
		return nil, infraError("get integration execution after advance", err)
	}
	if updatedExecution == nil {
		return nil, domain.ErrNotFound
	}
	domain.HydrateIntegrationExecutionDerived(updatedExecution)
	return updatedExecution, nil
}

func (s *integrationCenterService) ListExecutions(ctx context.Context, id int64) ([]*domain.IntegrationExecution, *domain.AppError) {
	log, err := s.callLogRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get integration call log for execution list", err)
	}
	if log == nil {
		return nil, domain.ErrNotFound
	}
	executions, err := s.executionRepo.ListByCallLogID(ctx, id)
	if err != nil {
		return nil, infraError("list integration executions", err)
	}
	if executions == nil {
		executions = []*domain.IntegrationExecution{}
	}
	for _, execution := range executions {
		domain.HydrateIntegrationExecutionDerived(execution)
	}
	return executions, nil
}

func (s *integrationCenterService) hydrateExecutionSummaries(ctx context.Context, logs []*domain.IntegrationCallLog) error {
	if len(logs) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(logs))
	for _, log := range logs {
		if log == nil {
			continue
		}
		ids = append(ids, log.CallLogID)
	}
	summaries, err := s.executionRepo.SummariesByCallLogIDs(ctx, ids)
	if err != nil {
		return err
	}
	for _, log := range logs {
		if log == nil {
			continue
		}
		if summary, ok := summaries[log.CallLogID]; ok {
			log.ExecutionCount = summary.ExecutionCount
			log.LatestExecution = summary.LatestExecution
			log.RetryCount = summary.RetryCount
			log.ReplayCount = summary.ReplayCount
			log.LatestRetryAction = domain.BuildIntegrationExecutionActionSummary(summary.LatestRetryExecution)
			log.LatestReplayAction = domain.BuildIntegrationExecutionActionSummary(summary.LatestReplayExecution)
		}
		domain.HydrateIntegrationCallLogDerived(log)
	}
	return nil
}

type createIntegrationActionParams struct {
	ExecutionMode domain.IntegrationExecutionMode
	TriggerSource string
	AdapterNote   string
}

func (s *integrationCenterService) createExecutionForAction(ctx context.Context, id int64, params createIntegrationActionParams) (*domain.IntegrationExecution, *domain.AppError) {
	log, appErr := s.GetCallLog(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := validateIntegrationExecutionAction(log, params.TriggerSource); appErr != nil {
		return nil, appErr
	}
	executionMode := normalizeIntegrationExecutionMode(params.ExecutionMode)
	if !executionMode.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "execution_mode must be manual_placeholder_adapter", nil)
	}

	now := s.nowFn().UTC()
	execution := &domain.IntegrationExecution{
		CallLogID:      log.CallLogID,
		ConnectorKey:   log.ConnectorKey,
		ExecutionMode:  executionMode,
		TriggerSource:  normalizeIntegrationExecutionTriggerSource(params.TriggerSource),
		Status:         domain.IntegrationExecutionStatusPrepared,
		LatestStatusAt: now,
		StartedAt:      now,
		AdapterNote:    strings.TrimSpace(params.AdapterNote),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		createdExecution, err := s.executionRepo.Create(ctx, tx, execution)
		if err != nil {
			return err
		}
		execution = createdExecution
		if !integrationActionRequiresRequeue(log.Status) {
			return nil
		}
		update, appErr := buildIntegrationCallLogRequeueUpdate(log, log.Remark, now)
		if appErr != nil {
			return appErr
		}
		return s.callLogRepo.Update(ctx, tx, update)
	}); err != nil {
		return nil, infraError("create integration execution", err)
	}
	domain.HydrateIntegrationExecutionDerived(execution)
	return execution, nil
}

func normalizeOptionalRawJSON(raw json.RawMessage, label string) (json.RawMessage, *domain.AppError) {
	if len(raw) == 0 {
		return nil, nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	if !json.Valid(raw) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("%s must be valid json", label), nil)
	}
	return append(json.RawMessage(nil), raw...), nil
}

func normalizeIntegrationExecutionMode(mode domain.IntegrationExecutionMode) domain.IntegrationExecutionMode {
	if strings.TrimSpace(string(mode)) == "" {
		return domain.IntegrationExecutionModeManualPlaceholderAdapter
	}
	return domain.IntegrationExecutionMode(strings.TrimSpace(string(mode)))
}

func normalizeIntegrationExecutionTriggerSource(triggerSource string) string {
	trimmed := strings.TrimSpace(triggerSource)
	if trimmed == "" {
		return integrationExecutionTriggerManualStart
	}
	return trimmed
}

func validateIntegrationExecutionCreate(log *domain.IntegrationCallLog) *domain.AppError {
	if log == nil {
		return domain.ErrNotFound
	}
	if log.Status == domain.IntegrationCallStatusSucceeded {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "cannot create a new integration execution when the call log is already succeeded", integrationCallLogStateDetails(log))
	}
	if log.LatestExecution != nil && !domain.IntegrationExecutionTerminal(log.LatestExecution.Status) {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "cannot create a new integration execution while the latest execution is still unresolved", integrationCallLogStateDetails(log))
	}
	if log.Status != domain.IntegrationCallStatusQueued && !log.CanRetry {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, fmt.Sprintf("cannot create a new integration execution when call log status is %s", log.Status), integrationCallLogStateDetails(log))
	}
	return nil
}

func validateIntegrationExecutionAction(log *domain.IntegrationCallLog, triggerSource string) *domain.AppError {
	switch strings.TrimSpace(triggerSource) {
	case integrationExecutionTriggerManualRetry:
		if !log.CanRetry {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "integration call log is not retryable", integrationCallLogStateDetails(log))
		}
	case integrationExecutionTriggerManualReplay:
		if !log.CanReplay {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "integration call log is not replayable", integrationCallLogStateDetails(log))
		}
	default:
		return validateIntegrationExecutionCreate(log)
	}
	if log.LatestExecution != nil && !domain.IntegrationExecutionTerminal(log.LatestExecution.Status) {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "cannot create a new integration execution while the latest execution is still unresolved", integrationCallLogStateDetails(log))
	}
	return nil
}

func integrationActionRequiresRequeue(status domain.IntegrationCallStatus) bool {
	return status == domain.IntegrationCallStatusFailed || status == domain.IntegrationCallStatusCancelled || status == domain.IntegrationCallStatusSucceeded
}

func buildIntegrationCallLogRequeueUpdate(current *domain.IntegrationCallLog, remark string, now time.Time) (repo.IntegrationCallLogUpdate, *domain.AppError) {
	if current == nil {
		return repo.IntegrationCallLogUpdate{}, domain.ErrNotFound
	}
	if current.Status != domain.IntegrationCallStatusFailed && current.Status != domain.IntegrationCallStatusCancelled && current.Status != domain.IntegrationCallStatusSucceeded {
		return repo.IntegrationCallLogUpdate{}, domain.NewAppError(domain.ErrCodeInvalidStateTransition, fmt.Sprintf("cannot move integration call log from %s to queued", current.Status), integrationCallLogStateDetails(current))
	}
	return repo.IntegrationCallLogUpdate{
		CallLogID:       current.CallLogID,
		Status:          domain.IntegrationCallStatusQueued,
		LatestStatusAt:  now,
		StartedAt:       nil,
		FinishedAt:      nil,
		ResponsePayload: nil,
		ErrorMessage:    "",
		Remark:          strings.TrimSpace(remark),
	}, nil
}

func nextIntegrationExecutionAdvance(currentLog *domain.IntegrationCallLog, currentExecution *domain.IntegrationExecution, responsePayload json.RawMessage, params AdvanceIntegrationExecutionParams, now time.Time) (repo.IntegrationExecutionUpdate, repo.IntegrationCallLogUpdate, *domain.AppError) {
	if currentLog == nil || currentExecution == nil {
		return repo.IntegrationExecutionUpdate{}, repo.IntegrationCallLogUpdate{}, domain.ErrNotFound
	}
	if domain.IntegrationExecutionTerminal(currentExecution.Status) {
		return repo.IntegrationExecutionUpdate{}, repo.IntegrationCallLogUpdate{}, domain.NewAppError(domain.ErrCodeInvalidStateTransition, fmt.Sprintf("cannot advance integration execution from terminal status %s", currentExecution.Status), integrationExecutionStateDetails(currentLog, currentExecution))
	}
	if !allowedIntegrationExecutionTransition(currentExecution.Status, params.Status) {
		return repo.IntegrationExecutionUpdate{}, repo.IntegrationCallLogUpdate{}, domain.NewAppError(domain.ErrCodeInvalidStateTransition, fmt.Sprintf("cannot move integration execution from %s to %s", currentExecution.Status, params.Status), integrationExecutionStateDetails(currentLog, currentExecution))
	}

	errorMessage := strings.TrimSpace(params.ErrorMessage)
	adapterNote := strings.TrimSpace(params.AdapterNote)
	retryable := false
	if params.Retryable != nil {
		retryable = *params.Retryable
	}
	finishedAt := (*time.Time)(nil)
	if domain.IntegrationExecutionTerminal(params.Status) {
		finishedAt = &now
	}
	if params.Status == domain.IntegrationExecutionStatusFailed && params.Retryable == nil {
		retryable = true
	}

	executionUpdate := repo.IntegrationExecutionUpdate{
		ExecutionID:    currentExecution.ExecutionID,
		Status:         params.Status,
		LatestStatusAt: now,
		FinishedAt:     finishedAt,
		ErrorMessage:   errorMessage,
		AdapterNote:    adapterNote,
		Retryable:      retryable,
	}
	callLogUpdate := repo.IntegrationCallLogUpdate{
		CallLogID:       currentLog.CallLogID,
		Status:          mapExecutionStatusToCallLogStatus(params.Status),
		LatestStatusAt:  now,
		StartedAt:       &currentExecution.StartedAt,
		FinishedAt:      finishedAt,
		ResponsePayload: responsePayload,
		ErrorMessage:    "",
		Remark:          currentLog.Remark,
	}
	if params.Status == domain.IntegrationExecutionStatusFailed || params.Status == domain.IntegrationExecutionStatusCancelled {
		callLogUpdate.ErrorMessage = errorMessage
	}
	if params.Status == domain.IntegrationExecutionStatusDispatched || params.Status == domain.IntegrationExecutionStatusReceived {
		callLogUpdate.FinishedAt = nil
		callLogUpdate.ResponsePayload = nil
	}
	if adapterNote != "" {
		callLogUpdate.Remark = adapterNote
	}
	return executionUpdate, callLogUpdate, nil
}

func allowedIntegrationExecutionTransition(current, next domain.IntegrationExecutionStatus) bool {
	switch current {
	case domain.IntegrationExecutionStatusPrepared:
		return next == domain.IntegrationExecutionStatusDispatched || next == domain.IntegrationExecutionStatusCompleted || next == domain.IntegrationExecutionStatusFailed || next == domain.IntegrationExecutionStatusCancelled
	case domain.IntegrationExecutionStatusDispatched:
		return next == domain.IntegrationExecutionStatusReceived || next == domain.IntegrationExecutionStatusCompleted || next == domain.IntegrationExecutionStatusFailed || next == domain.IntegrationExecutionStatusCancelled
	case domain.IntegrationExecutionStatusReceived:
		return next == domain.IntegrationExecutionStatusCompleted || next == domain.IntegrationExecutionStatusFailed || next == domain.IntegrationExecutionStatusCancelled
	default:
		return false
	}
}

func mapExecutionStatusToCallLogStatus(status domain.IntegrationExecutionStatus) domain.IntegrationCallStatus {
	switch status {
	case domain.IntegrationExecutionStatusCompleted:
		return domain.IntegrationCallStatusSucceeded
	case domain.IntegrationExecutionStatusFailed:
		return domain.IntegrationCallStatusFailed
	case domain.IntegrationExecutionStatusCancelled:
		return domain.IntegrationCallStatusCancelled
	default:
		return domain.IntegrationCallStatusSent
	}
}

func mapCallLogStatusToExecutionStatus(status domain.IntegrationCallStatus) domain.IntegrationExecutionStatus {
	switch status {
	case domain.IntegrationCallStatusSucceeded:
		return domain.IntegrationExecutionStatusCompleted
	case domain.IntegrationCallStatusFailed:
		return domain.IntegrationExecutionStatusFailed
	case domain.IntegrationCallStatusCancelled:
		return domain.IntegrationExecutionStatusCancelled
	default:
		return domain.IntegrationExecutionStatusDispatched
	}
}

func compatibilityAdvanceRetryable(status domain.IntegrationCallStatus) *bool {
	switch status {
	case domain.IntegrationCallStatusFailed:
		value := true
		return &value
	case domain.IntegrationCallStatusCancelled:
		value := false
		return &value
	default:
		return nil
	}
}

func compatibilityAdvanceAdapterNote(status domain.IntegrationCallStatus, remark string) string {
	base := "Backward-compatible call-log lifecycle advance reused execution boundary."
	if strings.TrimSpace(remark) == "" {
		return base
	}
	return base + " " + strings.TrimSpace(remark)
}

func (s *integrationCenterService) ensureExecutionForCompatibilityAdvance(ctx context.Context, log *domain.IntegrationCallLog, now time.Time) (*domain.IntegrationExecution, *domain.AppError) {
	if log == nil {
		return nil, domain.ErrNotFound
	}
	if log.LatestExecution != nil && !domain.IntegrationExecutionTerminal(log.LatestExecution.Status) {
		return log.LatestExecution, nil
	}
	return s.CreateExecution(ctx, log.CallLogID, CreateIntegrationExecutionParams{
		ExecutionMode: domain.IntegrationExecutionModeManualPlaceholderAdapter,
		TriggerSource: integrationExecutionTriggerCompatRoute,
		AdapterNote:   "Created by backward-compatible call-log advance route.",
	})
}

func integrationCallLogStateDetails(log *domain.IntegrationCallLog) map[string]interface{} {
	if log == nil {
		return nil
	}
	details := map[string]interface{}{
		"call_log_id":          log.CallLogID,
		"status":               log.Status,
		"can_retry":            log.CanRetry,
		"can_replay":           log.CanReplay,
		"retryability_reason":  log.RetryabilityReason,
		"replayability_reason": log.ReplayabilityReason,
		"execution_count":      log.ExecutionCount,
		"retry_count":          log.RetryCount,
		"replay_count":         log.ReplayCount,
		"placeholder_only":     true,
	}
	if log.LatestExecution != nil {
		details["latest_execution"] = log.LatestExecution
	}
	if log.LatestRetryAction != nil {
		details["latest_retry_action"] = log.LatestRetryAction
	}
	if log.LatestReplayAction != nil {
		details["latest_replay_action"] = log.LatestReplayAction
	}
	return details
}

func integrationExecutionStateDetails(log *domain.IntegrationCallLog, execution *domain.IntegrationExecution) map[string]interface{} {
	details := integrationCallLogStateDetails(log)
	if details == nil {
		details = map[string]interface{}{
			"placeholder_only": true,
		}
	}
	if execution != nil {
		details["execution_id"] = execution.ExecutionID
		details["execution_status"] = execution.Status
		details["execution_mode"] = execution.ExecutionMode
		details["trigger_source"] = execution.TriggerSource
		details["retryable"] = execution.Retryable
	}
	return details
}
