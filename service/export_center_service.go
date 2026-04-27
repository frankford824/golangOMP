package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

type CreateExportJobParams struct {
	ExportType        domain.ExportType
	TemplateKey       string
	SourceQueryType   domain.ExportSourceQueryType
	SourceFilters     domain.ExportSourceFilters
	QueryTemplate     *domain.TaskQueryTemplate
	NormalizedFilters *domain.TaskQueryFilterDefinition
	Remark            string
}

type AdvanceExportJobParams struct {
	Action         domain.ExportJobAdvanceAction
	ResultFileName string
	ResultMimeType string
	ExpiresAt      *time.Time
	FailureReason  string
	Remark         string
}

type CreateExportDispatchParams struct {
	TriggerSource string
	ExpiresAt     *time.Time
	Remark        string
}

type AdvanceExportDispatchParams struct {
	Action string
	Reason string
	Remark string
}

type ExportJobFilter struct {
	Status          *domain.ExportJobStatus
	SourceQueryType *domain.ExportSourceQueryType
	RequestedByID   *int64
	Page            int
	PageSize        int
}

type ExportCenterService interface {
	ListTemplates(ctx context.Context) ([]domain.ExportTemplate, *domain.AppError)
	CreateJob(ctx context.Context, params CreateExportJobParams) (*domain.ExportJob, *domain.AppError)
	CreateJobDispatch(ctx context.Context, id int64, params CreateExportDispatchParams) (*domain.ExportJobDispatch, *domain.AppError)
	AdvanceJobDispatch(ctx context.Context, id int64, dispatchID string, params AdvanceExportDispatchParams) (*domain.ExportJobDispatch, *domain.AppError)
	StartJob(ctx context.Context, id int64) (*domain.ExportJob, *domain.AppError)
	AdvanceJob(ctx context.Context, id int64, params AdvanceExportJobParams) (*domain.ExportJob, *domain.AppError)
	ListJobs(ctx context.Context, filter ExportJobFilter) ([]*domain.ExportJob, domain.PaginationMeta, *domain.AppError)
	GetJob(ctx context.Context, id int64) (*domain.ExportJob, *domain.AppError)
	ListJobAttempts(ctx context.Context, id int64) ([]*domain.ExportJobAttempt, *domain.AppError)
	ListJobDispatches(ctx context.Context, id int64) ([]*domain.ExportJobDispatch, *domain.AppError)
	ListJobEvents(ctx context.Context, id int64) ([]*domain.ExportJobEvent, *domain.AppError)
	ClaimDownload(ctx context.Context, id int64) (*domain.ExportJobDownloadHandoff, *domain.AppError)
	ReadDownload(ctx context.Context, id int64) (*domain.ExportJobDownloadHandoff, *domain.AppError)
	RefreshDownload(ctx context.Context, id int64) (*domain.ExportJobDownloadHandoff, *domain.AppError)
}

type exportCenterService struct {
	exportJobRepo         repo.ExportJobRepo
	exportJobDispatchRepo repo.ExportJobDispatchRepo
	exportJobAttemptRepo  repo.ExportJobAttemptRepo
	exportJobEventRepo    repo.ExportJobEventRepo
	txRunner              repo.TxRunner
	nowFn                 func() time.Time
}

const exportDownloadHandoffTTL = 24 * time.Hour

const (
	exportJobInitiationSourceStartEndpoint = "start_endpoint"
	exportJobInitiationSourceAdvanceAction = "advance_action_compat"
	exportJobDispatchSourceManualSubmit    = "manual_dispatch_submit"
)

func NewExportCenterService(exportJobRepo repo.ExportJobRepo, exportJobDispatchRepo repo.ExportJobDispatchRepo, exportJobAttemptRepo repo.ExportJobAttemptRepo, exportJobEventRepo repo.ExportJobEventRepo, txRunner repo.TxRunner) ExportCenterService {
	return &exportCenterService{
		exportJobRepo:         exportJobRepo,
		exportJobDispatchRepo: exportJobDispatchRepo,
		exportJobAttemptRepo:  exportJobAttemptRepo,
		exportJobEventRepo:    exportJobEventRepo,
		txRunner:              txRunner,
		nowFn:                 time.Now,
	}
}

func (s *exportCenterService) ListTemplates(_ context.Context) ([]domain.ExportTemplate, *domain.AppError) {
	templates := exportTemplateCatalog()
	out := make([]domain.ExportTemplate, 0, len(templates))
	out = append(out, templates...)
	return out, nil
}

func (s *exportCenterService) CreateJob(ctx context.Context, params CreateExportJobParams) (*domain.ExportJob, *domain.AppError) {
	template, appErr := resolveExportTemplate(params.ExportType, params.TemplateKey, params.SourceQueryType)
	if appErr != nil {
		return nil, appErr
	}
	sourceFilters, queryTemplate, normalizedFilters, appErr := validateExportSourceContract(
		params.SourceQueryType,
		params.SourceFilters,
		params.QueryTemplate,
		params.NormalizedFilters,
	)
	if appErr != nil {
		return nil, appErr
	}

	actor, _ := resolveWorkbenchActorScope(ctx)
	now := s.nowFn().UTC()
	job := &domain.ExportJob{
		TemplateKey:       template.Key,
		ExportType:        params.ExportType,
		SourceQueryType:   params.SourceQueryType,
		SourceFilters:     sourceFilters,
		QueryTemplate:     queryTemplate,
		NormalizedFilters: normalizedFilters,
		RequestedBy:       actor,
		Status:            domain.ExportJobStatusQueued,
		ResultRef:         buildPlaceholderResultRef(template, now),
		Remark:            strings.TrimSpace(params.Remark),
		CreatedAt:         now,
		LatestStatusAt:    now,
		UpdatedAt:         now,
	}
	domain.HydrateExportJobDerived(job)

	var jobID int64
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		id, err := s.exportJobRepo.Create(ctx, tx, job)
		if err != nil {
			return err
		}
		jobID = id
		job.ExportJobID = id
		event, err := buildCreatedExportJobEvent(*job, actor, now)
		if err != nil {
			return err
		}
		if _, err := s.exportJobEventRepo.Append(ctx, tx, event); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, infraError("create export job", err)
	}

	return s.GetJob(ctx, jobID)
}

func (s *exportCenterService) CreateJobDispatch(ctx context.Context, id int64, params CreateExportDispatchParams) (*domain.ExportJobDispatch, *domain.AppError) {
	job, appErr := s.GetJob(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	now := s.nowFn().UTC()
	latestDispatch, err := s.exportJobDispatchRepo.GetLatestByExportJobID(ctx, id)
	if err != nil {
		return nil, infraError("get latest export job dispatch", err)
	}
	if appErr := validateExportJobDispatchCreate(job, latestDispatch); appErr != nil {
		return nil, appErr
	}

	actor, _ := resolveWorkbenchActorScope(ctx)
	dispatch := buildSubmittedExportJobDispatch(job.ExportJobID, strings.TrimSpace(params.TriggerSource), strings.TrimSpace(params.Remark), params.ExpiresAt, now)
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		createdDispatch, err := s.exportJobDispatchRepo.Create(ctx, tx, dispatch)
		if err != nil {
			return err
		}
		dispatch = createdDispatch
		event, err := buildDispatchSubmittedExportJobEvent(*job, *dispatch, actor, now)
		if err != nil {
			return err
		}
		_, err = s.exportJobEventRepo.Append(ctx, tx, event)
		return err
	}); err != nil {
		return nil, infraError("create export job dispatch", err)
	}
	domain.HydrateExportJobDispatchDerived(dispatch)
	return dispatch, nil
}

func (s *exportCenterService) AdvanceJobDispatch(ctx context.Context, id int64, dispatchID string, params AdvanceExportDispatchParams) (*domain.ExportJobDispatch, *domain.AppError) {
	job, appErr := s.GetJob(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	dispatch, err := s.exportJobDispatchRepo.GetByDispatchID(ctx, dispatchID)
	if err != nil {
		return nil, infraError("get export job dispatch", err)
	}
	if dispatch == nil || dispatch.ExportJobID != id {
		return nil, domain.ErrNotFound
	}
	action := normalizeExportJobDispatchAction(params.Action)
	if action == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "dispatch action must be receive/reject/expire/mark_not_executed", nil)
	}
	now := s.nowFn().UTC()
	update, eventType, eventNote, appErr := nextExportJobDispatchUpdate(*job, *dispatch, action, strings.TrimSpace(params.Reason), strings.TrimSpace(params.Remark), now)
	if appErr != nil {
		return nil, appErr
	}
	actor, _ := resolveWorkbenchActorScope(ctx)
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.exportJobDispatchRepo.Update(ctx, tx, update); err != nil {
			return err
		}
		updatedDispatch := applyExportJobDispatchUpdate(*dispatch, update, now)
		event, err := buildDispatchAdvancedExportJobEvent(*job, updatedDispatch, actor, eventType, eventNote, now)
		if err != nil {
			return err
		}
		if _, err := s.exportJobEventRepo.Append(ctx, tx, event); err != nil {
			return err
		}
		dispatch = &updatedDispatch
		return nil
	}); err != nil {
		return nil, infraError("advance export job dispatch", err)
	}
	domain.HydrateExportJobDispatchDerived(dispatch)
	return dispatch, nil
}

func (s *exportCenterService) StartJob(ctx context.Context, id int64) (*domain.ExportJob, *domain.AppError) {
	return s.startJob(ctx, id, exportJobInitiationSourceStartEndpoint)
}

func (s *exportCenterService) AdvanceJob(ctx context.Context, id int64, params AdvanceExportJobParams) (*domain.ExportJob, *domain.AppError) {
	if !params.Action.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "action must be start/mark_ready/fail/cancel/requeue", nil)
	}
	if params.Action == domain.ExportJobAdvanceActionStart {
		return s.startJob(ctx, id, exportJobInitiationSourceAdvanceAction)
	}
	job, appErr := s.GetJob(ctx, id)
	if appErr != nil {
		return nil, appErr
	}

	actor, _ := resolveWorkbenchActorScope(ctx)
	now := s.nowFn().UTC()
	nextStatus, finishedAt, appErr := nextExportJobLifecycleState(job.Status, params.Action, now)
	if appErr != nil {
		return nil, appErr
	}
	nextResultRef := buildAdvancedResultRef(*job, params, nextStatus, now)
	nextRemark := strings.TrimSpace(params.Remark)
	if nextRemark == "" {
		nextRemark = strings.TrimSpace(job.Remark)
	}
	resultRefChanged := exportResultRefAuditChanged(job.ResultRef, nextResultRef)
	latestAttempt, appErr := s.latestAttemptForAdvance(ctx, job, params.Action)
	if appErr != nil {
		return nil, appErr
	}
	dispatchForAttempt, appErr := s.dispatchForAttempt(ctx, latestAttempt)
	if appErr != nil {
		return nil, appErr
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.exportJobRepo.UpdateLifecycle(ctx, tx, repo.ExportJobLifecycleUpdate{
			ExportJobID:    id,
			Status:         nextStatus,
			LatestStatusAt: now,
			FinishedAt:     finishedAt,
			ResultRef:      nextResultRef,
			Remark:         nextRemark,
		}); err != nil {
			return err
		}
		if resultRefChanged {
			event, err := buildResultRefUpdatedExportJobEvent(*job, nextResultRef, actor, nextStatus, params, now)
			if err != nil {
				return err
			}
			if _, err := s.exportJobEventRepo.Append(ctx, tx, event); err != nil {
				return err
			}
		}
		if latestAttempt != nil {
			attemptUpdate := buildExportJobAttemptUpdate(*latestAttempt, nextStatus, params, now)
			if err := s.exportJobAttemptRepo.Update(ctx, tx, attemptUpdate); err != nil {
				return err
			}
			finishedAttempt := applyExportJobAttemptUpdate(*latestAttempt, attemptUpdate, now)
			attemptEvent, err := buildAttemptFinishedExportJobEvent(*job, dispatchForAttempt, finishedAttempt, actor, nextStatus, params, nextResultRef, now)
			if err != nil {
				return err
			}
			if attemptEvent != nil {
				if _, err := s.exportJobEventRepo.Append(ctx, tx, attemptEvent); err != nil {
					return err
				}
			}
			latestAttempt = &finishedAttempt
		}
		event, err := buildLifecycleAdvancedExportJobEvent(*job, dispatchForAttempt, nextStatus, actor, params, latestAttempt, now)
		if err != nil {
			return err
		}
		if _, err := s.exportJobEventRepo.Append(ctx, tx, event); err != nil {
			return err
		}
		return nil
	})
	if txErr != nil {
		return nil, infraError("advance export job", txErr)
	}

	return s.GetJob(ctx, id)
}

func (s *exportCenterService) startJob(ctx context.Context, id int64, initiationSource string) (*domain.ExportJob, *domain.AppError) {
	job, appErr := s.GetJob(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	latestDispatch, err := s.exportJobDispatchRepo.GetLatestByExportJobID(ctx, id)
	if err != nil {
		return nil, infraError("get latest export job dispatch for start", err)
	}
	if appErr := validateExportJobStart(job, latestDispatch); appErr != nil {
		return nil, appErr
	}

	actor, _ := resolveWorkbenchActorScope(ctx)
	now := s.nowFn().UTC()
	nextStatus := domain.ExportJobStatusRunning
	dispatch := latestDispatch
	attempt := &domain.ExportJobAttempt{
		ExportJobID:   job.ExportJobID,
		TriggerSource: strings.TrimSpace(initiationSource),
		ExecutionMode: domain.ExportJobExecutionModeManualPlaceholderRunner,
		AdapterKey:    domain.ExportJobRunnerAdapterKeyManualPlaceholder,
		Status:        domain.ExportJobAttemptStatusRunning,
		StartedAt:     now,
		AdapterNote:   exportJobAttemptStartNote(initiationSource),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	nextResultRef := buildAdvancedResultRef(*job, AdvanceExportJobParams{
		Action: domain.ExportJobAdvanceActionStart,
	}, nextStatus, now)
	resultRefChanged := exportResultRefAuditChanged(job.ResultRef, nextResultRef)

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if dispatch == nil || dispatch.Status != domain.ExportJobDispatchStatusReceived {
			submittedDispatch := buildSubmittedExportJobDispatch(job.ExportJobID, strings.TrimSpace(initiationSource), exportJobDispatchAutoStartSubmitNote(), nil, now)
			createdDispatch, err := s.exportJobDispatchRepo.Create(ctx, tx, submittedDispatch)
			if err != nil {
				return err
			}
			dispatch = createdDispatch
			submittedEvent, err := buildDispatchSubmittedExportJobEvent(*job, *dispatch, actor, now)
			if err != nil {
				return err
			}
			if _, err := s.exportJobEventRepo.Append(ctx, tx, submittedEvent); err != nil {
				return err
			}
			dispatchUpdate := repo.ExportJobDispatchUpdate{
				DispatchID:  dispatch.DispatchID,
				Status:      domain.ExportJobDispatchStatusReceived,
				ReceivedAt:  &now,
				ExpiresAt:   dispatch.ExpiresAt,
				AdapterNote: exportJobDispatchAutoStartReceiveNote(),
			}
			if err := s.exportJobDispatchRepo.Update(ctx, tx, dispatchUpdate); err != nil {
				return err
			}
			updatedDispatch := applyExportJobDispatchUpdate(*dispatch, dispatchUpdate, now)
			receivedEvent, err := buildDispatchAdvancedExportJobEvent(*job, updatedDispatch, actor, domain.ExportJobEventDispatchReceived, "Placeholder adapter-dispatch handoff received at start boundary.", now)
			if err != nil {
				return err
			}
			if _, err := s.exportJobEventRepo.Append(ctx, tx, receivedEvent); err != nil {
				return err
			}
			dispatch = &updatedDispatch
		}
		attempt.DispatchID = dispatch.DispatchID
		createdAttempt, err := s.exportJobAttemptRepo.Create(ctx, tx, attempt)
		if err != nil {
			return err
		}
		attempt = createdAttempt

		runnerInitiatedEvent, err := buildRunnerInitiatedExportJobEvent(*job, dispatch, *attempt, actor, initiationSource, now)
		if err != nil {
			return err
		}
		if _, err := s.exportJobEventRepo.Append(ctx, tx, runnerInitiatedEvent); err != nil {
			return err
		}

		if err := s.exportJobRepo.UpdateLifecycle(ctx, tx, repo.ExportJobLifecycleUpdate{
			ExportJobID:    id,
			Status:         nextStatus,
			LatestStatusAt: now,
			FinishedAt:     nil,
			ResultRef:      nextResultRef,
			Remark:         strings.TrimSpace(job.Remark),
		}); err != nil {
			return err
		}

		if resultRefChanged {
			resultRefUpdatedEvent, err := buildResultRefUpdatedExportJobEvent(*job, nextResultRef, actor, nextStatus, AdvanceExportJobParams{
				Action: domain.ExportJobAdvanceActionStart,
			}, now)
			if err != nil {
				return err
			}
			if _, err := s.exportJobEventRepo.Append(ctx, tx, resultRefUpdatedEvent); err != nil {
				return err
			}
		}

		startedEvent, err := buildStartedExportJobEvent(*job, dispatch, *attempt, actor, initiationSource, nextResultRef, now)
		if err != nil {
			return err
		}
		if _, err := s.exportJobEventRepo.Append(ctx, tx, startedEvent); err != nil {
			return err
		}

		lifecycleEvent, err := buildLifecycleAdvancedExportJobEvent(*job, dispatch, nextStatus, actor, AdvanceExportJobParams{
			Action: domain.ExportJobAdvanceActionStart,
		}, attempt, now)
		if err != nil {
			return err
		}
		if _, err := s.exportJobEventRepo.Append(ctx, tx, lifecycleEvent); err != nil {
			return err
		}

		return nil
	})
	if txErr != nil {
		return nil, infraError("start export job", txErr)
	}

	return s.GetJob(ctx, id)
}

func (s *exportCenterService) ListJobs(ctx context.Context, filter ExportJobFilter) ([]*domain.ExportJob, domain.PaginationMeta, *domain.AppError) {
	if filter.Status != nil && !filter.Status.Valid() {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "status must be queued/running/ready/failed/cancelled", nil)
	}
	if filter.SourceQueryType != nil && !filter.SourceQueryType.Valid() {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "source_query_type must be task_query/task_board_queue/procurement_summary/warehouse_receipts", nil)
	}
	jobs, total, err := s.exportJobRepo.List(ctx, repo.ExportJobListFilter{
		Status:          filter.Status,
		SourceQueryType: filter.SourceQueryType,
		RequestedByID:   filter.RequestedByID,
		Page:            filter.Page,
		PageSize:        filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list export jobs", err)
	}
	if jobs == nil {
		jobs = []*domain.ExportJob{}
	}
	if err := s.hydrateExportJobDispatchSummaries(ctx, jobs); err != nil {
		return nil, domain.PaginationMeta{}, infraError("hydrate export job dispatch summaries", err)
	}
	if err := s.hydrateExportJobAttemptSummaries(ctx, jobs); err != nil {
		return nil, domain.PaginationMeta{}, infraError("hydrate export job attempt summaries", err)
	}
	if err := s.hydrateExportJobEventSummaries(ctx, jobs); err != nil {
		return nil, domain.PaginationMeta{}, infraError("hydrate export job event summaries", err)
	}
	s.hydrateExportJobDownloadStates(jobs, s.nowFn().UTC())
	return jobs, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *exportCenterService) GetJob(ctx context.Context, id int64) (*domain.ExportJob, *domain.AppError) {
	job, err := s.exportJobRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get export job", err)
	}
	if job == nil {
		return nil, domain.ErrNotFound
	}
	if err := s.hydrateExportJobDispatchSummaries(ctx, []*domain.ExportJob{job}); err != nil {
		return nil, infraError("hydrate export job dispatch summary", err)
	}
	if err := s.hydrateExportJobAttemptSummaries(ctx, []*domain.ExportJob{job}); err != nil {
		return nil, infraError("hydrate export job attempt summary", err)
	}
	if err := s.hydrateExportJobEventSummaries(ctx, []*domain.ExportJob{job}); err != nil {
		return nil, infraError("hydrate export job event summary", err)
	}
	s.hydrateExportJobDownloadStates([]*domain.ExportJob{job}, s.nowFn().UTC())
	return job, nil
}

func (s *exportCenterService) ListJobAttempts(ctx context.Context, id int64) ([]*domain.ExportJobAttempt, *domain.AppError) {
	job, err := s.exportJobRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get export job for attempt list", err)
	}
	if job == nil {
		return nil, domain.ErrNotFound
	}
	attempts, err := s.exportJobAttemptRepo.ListByExportJobID(ctx, id)
	if err != nil {
		return nil, infraError("list export job attempts", err)
	}
	if attempts == nil {
		attempts = []*domain.ExportJobAttempt{}
	}
	for _, attempt := range attempts {
		domain.HydrateExportJobAttemptDerived(attempt)
	}
	return attempts, nil
}

func (s *exportCenterService) ListJobDispatches(ctx context.Context, id int64) ([]*domain.ExportJobDispatch, *domain.AppError) {
	job, err := s.exportJobRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get export job for dispatch list", err)
	}
	if job == nil {
		return nil, domain.ErrNotFound
	}
	dispatches, err := s.exportJobDispatchRepo.ListByExportJobID(ctx, id)
	if err != nil {
		return nil, infraError("list export job dispatches", err)
	}
	if dispatches == nil {
		dispatches = []*domain.ExportJobDispatch{}
	}
	for _, dispatch := range dispatches {
		domain.HydrateExportJobDispatchDerived(dispatch)
	}
	return dispatches, nil
}

func (s *exportCenterService) ListJobEvents(ctx context.Context, id int64) ([]*domain.ExportJobEvent, *domain.AppError) {
	job, err := s.exportJobRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get export job for event list", err)
	}
	if job == nil {
		return nil, domain.ErrNotFound
	}
	events, err := s.exportJobEventRepo.ListByExportJobID(ctx, id)
	if err != nil {
		return nil, infraError("list export job events", err)
	}
	if events == nil {
		events = []*domain.ExportJobEvent{}
	}
	return events, nil
}

func (s *exportCenterService) ClaimDownload(ctx context.Context, id int64) (*domain.ExportJobDownloadHandoff, *domain.AppError) {
	return s.accessDownloadHandoff(ctx, id, domain.ExportJobEventDownloadClaimed)
}

func (s *exportCenterService) ReadDownload(ctx context.Context, id int64) (*domain.ExportJobDownloadHandoff, *domain.AppError) {
	return s.accessDownloadHandoff(ctx, id, domain.ExportJobEventDownloadRead)
}

func (s *exportCenterService) RefreshDownload(ctx context.Context, id int64) (*domain.ExportJobDownloadHandoff, *domain.AppError) {
	job, appErr := s.GetJob(ctx, id)
	if appErr != nil {
		return nil, appErr
	}

	now := s.nowFn().UTC()
	if appErr := validateExportDownloadRefresh(job, now); appErr != nil {
		if shouldAppendExportDownloadExpired(job, now) {
			actor, _ := resolveWorkbenchActorScope(ctx)
			events, err := s.exportJobEventRepo.ListByExportJobID(ctx, id)
			if err != nil {
				return nil, infraError("list export job events for refresh validation", err)
			}
			if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
				return s.appendDownloadExpiredEventIfNeeded(ctx, tx, job, actor, events, now)
			}); err != nil {
				return nil, infraError("append export job download_expired event", err)
			}
		}
		return nil, appErr
	}

	events, err := s.exportJobEventRepo.ListByExportJobID(ctx, id)
	if err != nil {
		return nil, infraError("list export job events for refresh", err)
	}

	actor, _ := resolveWorkbenchActorScope(ctx)
	nextResultRef := buildRefreshedResultRef(job.ResultRef, now)
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.appendDownloadExpiredEventIfNeeded(ctx, tx, job, actor, events, now); err != nil {
			return err
		}
		if err := s.exportJobRepo.UpdateLifecycle(ctx, tx, repo.ExportJobLifecycleUpdate{
			ExportJobID:    job.ExportJobID,
			Status:         job.Status,
			LatestStatusAt: job.LatestStatusAt,
			FinishedAt:     job.FinishedAt,
			ResultRef:      nextResultRef,
			Remark:         job.Remark,
		}); err != nil {
			return err
		}
		resultRefUpdatedEvent, err := buildResultRefUpdatedExportJobEvent(*job, nextResultRef, actor, job.Status, AdvanceExportJobParams{}, now)
		if err != nil {
			return err
		}
		if _, err := s.exportJobEventRepo.Append(ctx, tx, resultRefUpdatedEvent); err != nil {
			return err
		}
		refreshedEvent, err := buildDownloadRefreshedExportJobEvent(*job, nextResultRef, actor, now)
		if err != nil {
			return err
		}
		if _, err := s.exportJobEventRepo.Append(ctx, tx, refreshedEvent); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, infraError("refresh export job download handoff", err)
	}

	return s.buildDownloadHandoff(ctx, id)
}

func exportTemplateCatalog() []domain.ExportTemplate {
	return []domain.ExportTemplate{
		{
			Key:         "task_list_basic",
			Name:        "Task List Basic",
			Description: "Placeholder CSV-oriented skeleton for current `/v1/tasks` query results.",
			ExportType:  domain.ExportTypeTaskList,
			SupportedSourceQueryTypes: []domain.ExportSourceQueryType{
				domain.ExportSourceQueryTypeTaskQuery,
			},
			ResultFormat:    "csv",
			PlaceholderOnly: true,
		},
		{
			Key:         "task_board_queue_basic",
			Name:        "Task Board Queue Basic",
			Description: "Placeholder CSV-oriented skeleton for one board queue plus its current task-query handoff state.",
			ExportType:  domain.ExportTypeTaskBoardQueue,
			SupportedSourceQueryTypes: []domain.ExportSourceQueryType{
				domain.ExportSourceQueryTypeTaskBoardQueue,
			},
			ResultFormat:    "csv",
			PlaceholderOnly: true,
		},
		{
			Key:         "procurement_summary_basic",
			Name:        "Procurement Summary Basic",
			Description: "Placeholder CSV-oriented skeleton for procurement-facing task summary exports derived from stable task queries.",
			ExportType:  domain.ExportTypeProcurementSummary,
			SupportedSourceQueryTypes: []domain.ExportSourceQueryType{
				domain.ExportSourceQueryTypeProcurementSummary,
			},
			ResultFormat:    "csv",
			PlaceholderOnly: true,
		},
		{
			Key:         "warehouse_receipts_basic",
			Name:        "Warehouse Receipts Basic",
			Description: "Placeholder CSV-oriented skeleton for current `/v1/warehouse/receipts` query results.",
			ExportType:  domain.ExportTypeWarehouseReceipts,
			SupportedSourceQueryTypes: []domain.ExportSourceQueryType{
				domain.ExportSourceQueryTypeWarehouseReceipts,
			},
			ResultFormat:    "csv",
			PlaceholderOnly: true,
		},
	}
}

func resolveExportTemplate(exportType domain.ExportType, templateKey string, sourceType domain.ExportSourceQueryType) (domain.ExportTemplate, *domain.AppError) {
	if !exportType.Valid() {
		return domain.ExportTemplate{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "export_type must be task_list/task_board_queue/procurement_summary/warehouse_receipts", nil)
	}
	if !sourceType.Valid() {
		return domain.ExportTemplate{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "source_query_type must be task_query/task_board_queue/procurement_summary/warehouse_receipts", nil)
	}
	templateKey = strings.TrimSpace(templateKey)
	if templateKey == "" {
		templateKey = defaultTemplateKeyForExportType(exportType)
	}
	for _, template := range exportTemplateCatalog() {
		if template.Key != templateKey {
			continue
		}
		if template.ExportType != exportType {
			return domain.ExportTemplate{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "template_key does not match export_type", nil)
		}
		for _, candidate := range template.SupportedSourceQueryTypes {
			if candidate == sourceType {
				return template, nil
			}
		}
		return domain.ExportTemplate{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "template_key does not support the selected source_query_type", nil)
	}
	return domain.ExportTemplate{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "template_key is not supported", nil)
}

func defaultTemplateKeyForExportType(exportType domain.ExportType) string {
	switch exportType {
	case domain.ExportTypeTaskBoardQueue:
		return "task_board_queue_basic"
	case domain.ExportTypeProcurementSummary:
		return "procurement_summary_basic"
	case domain.ExportTypeWarehouseReceipts:
		return "warehouse_receipts_basic"
	default:
		return "task_list_basic"
	}
}

func validateExportSourceContract(
	sourceType domain.ExportSourceQueryType,
	sourceFilters domain.ExportSourceFilters,
	queryTemplate *domain.TaskQueryTemplate,
	normalizedFilters *domain.TaskQueryFilterDefinition,
) (domain.ExportSourceFilters, *domain.TaskQueryTemplate, *domain.TaskQueryFilterDefinition, *domain.AppError) {
	sourceFilters.QueueKey = strings.TrimSpace(sourceFilters.QueueKey)
	sourceFilters.Status = strings.TrimSpace(sourceFilters.Status)

	if sourceFilters.BoardView != "" && !sourceFilters.BoardView.Valid() {
		return domain.ExportSourceFilters{}, nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "source_filters.board_view must be all/ops/designer/audit/procurement/warehouse", nil)
	}
	if normalizedFilters != nil {
		if _, appErr := normalizeTaskFilter(TaskFilter{TaskQueryFilterDefinition: *normalizedFilters}); appErr != nil {
			return domain.ExportSourceFilters{}, nil, nil, appErr
		}
	}

	switch sourceType {
	case domain.ExportSourceQueryTypeTaskQuery, domain.ExportSourceQueryTypeProcurementSummary:
		if queryTemplate == nil {
			return domain.ExportSourceFilters{}, nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "query_template is required for task-query-derived export jobs", nil)
		}
		filter, appErr := taskQueryTemplateToTaskFilter(*queryTemplate)
		if appErr != nil {
			return domain.ExportSourceFilters{}, nil, nil, appErr
		}
		if normalizedFilters == nil {
			derived := filter.TaskQueryFilterDefinition
			normalizedFilters = &derived
		}
		return sourceFilters, queryTemplate, normalizedFilters, nil
	case domain.ExportSourceQueryTypeTaskBoardQueue:
		if sourceFilters.QueueKey == "" {
			return domain.ExportSourceFilters{}, nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "source_filters.queue_key is required for task_board_queue export jobs", nil)
		}
		if queryTemplate == nil {
			return domain.ExportSourceFilters{}, nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "query_template is required for task_board_queue export jobs", nil)
		}
		filter, appErr := taskQueryTemplateToTaskFilter(*queryTemplate)
		if appErr != nil {
			return domain.ExportSourceFilters{}, nil, nil, appErr
		}
		if normalizedFilters == nil {
			derived := filter.TaskQueryFilterDefinition
			normalizedFilters = &derived
		}
		return sourceFilters, queryTemplate, normalizedFilters, nil
	case domain.ExportSourceQueryTypeWarehouseReceipts:
		if queryTemplate != nil || normalizedFilters != nil {
			return domain.ExportSourceFilters{}, nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "warehouse_receipts export jobs do not accept task-query query_template or normalized_filters", nil)
		}
		if sourceFilters.QueueKey != "" || sourceFilters.BoardView != "" {
			return domain.ExportSourceFilters{}, nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "warehouse_receipts export jobs do not accept board queue fields", nil)
		}
		if sourceFilters.Status != "" {
			switch domain.WarehouseReceiptStatus(sourceFilters.Status) {
			case domain.WarehouseReceiptStatusReceived, domain.WarehouseReceiptStatusRejected, domain.WarehouseReceiptStatusCompleted:
			default:
				return domain.ExportSourceFilters{}, nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "source_filters.status must be received/rejected/completed", nil)
			}
		}
		return sourceFilters, nil, nil, nil
	default:
		return domain.ExportSourceFilters{}, nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "unsupported source_query_type", nil)
	}
}

func buildPlaceholderResultRef(template domain.ExportTemplate, now time.Time) *domain.ExportResultRef {
	timestamp := now.UTC().Format("20060102T150405Z")
	fileName := fmt.Sprintf("%s_%s.%s", template.Key, timestamp, template.ResultFormat)
	return &domain.ExportResultRef{
		RefType:       "download_handoff_placeholder",
		RefKey:        "export-result/" + uuid.NewString(),
		FileName:      fileName,
		MimeType:      mediaTypeForResultFormat(template.ResultFormat),
		IsPlaceholder: true,
		Note:          "Export job created. Download handoff is still placeholder metadata until the job is marked ready.",
	}
}

func mediaTypeForResultFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "csv":
		return "text/csv"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return "application/octet-stream"
	}
}

func nextExportJobLifecycleState(current domain.ExportJobStatus, action domain.ExportJobAdvanceAction, now time.Time) (domain.ExportJobStatus, *time.Time, *domain.AppError) {
	terminalAt := func() *time.Time {
		value := now
		return &value
	}

	switch action {
	case domain.ExportJobAdvanceActionStart:
		if current != domain.ExportJobStatusQueued {
			return "", nil, invalidExportLifecycleTransition(current, action)
		}
		return domain.ExportJobStatusRunning, nil, nil
	case domain.ExportJobAdvanceActionMarkReady:
		if current != domain.ExportJobStatusRunning {
			return "", nil, invalidExportLifecycleTransition(current, action)
		}
		return domain.ExportJobStatusReady, terminalAt(), nil
	case domain.ExportJobAdvanceActionFail:
		if current != domain.ExportJobStatusQueued && current != domain.ExportJobStatusRunning {
			return "", nil, invalidExportLifecycleTransition(current, action)
		}
		return domain.ExportJobStatusFailed, terminalAt(), nil
	case domain.ExportJobAdvanceActionCancel:
		if current != domain.ExportJobStatusQueued && current != domain.ExportJobStatusRunning {
			return "", nil, invalidExportLifecycleTransition(current, action)
		}
		return domain.ExportJobStatusCancelled, terminalAt(), nil
	case domain.ExportJobAdvanceActionRequeue:
		if current != domain.ExportJobStatusFailed && current != domain.ExportJobStatusCancelled {
			return "", nil, invalidExportLifecycleTransition(current, action)
		}
		return domain.ExportJobStatusQueued, nil, nil
	default:
		return "", nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "unsupported export job lifecycle action", nil)
	}
}

func normalizeExportJobDispatchAction(action string) string {
	switch strings.TrimSpace(action) {
	case "receive":
		return "receive"
	case "reject":
		return "reject"
	case "expire":
		return "expire"
	case "mark_not_executed":
		return "mark_not_executed"
	default:
		return ""
	}
}

func validateExportJobDispatchCreate(job *domain.ExportJob, latestDispatch *domain.ExportJobDispatch) *domain.AppError {
	if job == nil {
		return domain.ErrNotFound
	}
	allowed, reason := domain.ExportJobDispatchAdmission(job.Status, latestDispatch)
	if !allowed {
		return domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("cannot submit export job dispatch: %s", exportAdmissionReasonMessage(reason)),
			exportJobDispatchStateDetails(job, latestDispatch),
		)
	}
	return nil
}

func nextExportJobDispatchUpdate(job domain.ExportJob, dispatch domain.ExportJobDispatch, action, reason, remark string, now time.Time) (repo.ExportJobDispatchUpdate, string, string, *domain.AppError) {
	if job.Status != domain.ExportJobStatusQueued {
		return repo.ExportJobDispatchUpdate{}, "", "", domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("cannot advance dispatch when export job status is %s", job.Status),
			exportJobDispatchStateDetails(&job, &dispatch),
		)
	}
	update := repo.ExportJobDispatchUpdate{
		DispatchID:   dispatch.DispatchID,
		Status:       dispatch.Status,
		ReceivedAt:   dispatch.ReceivedAt,
		FinishedAt:   dispatch.FinishedAt,
		ExpiresAt:    dispatch.ExpiresAt,
		StatusReason: strings.TrimSpace(reason),
		AdapterNote:  strings.TrimSpace(remark),
	}
	switch action {
	case "receive":
		if dispatch.Status != domain.ExportJobDispatchStatusSubmitted {
			return repo.ExportJobDispatchUpdate{}, "", "", invalidExportJobDispatchTransition(job, dispatch, action)
		}
		update.Status = domain.ExportJobDispatchStatusReceived
		update.ReceivedAt = &now
		if update.AdapterNote == "" {
			update.AdapterNote = "Placeholder adapter-dispatch handoff received."
		}
		return update, domain.ExportJobEventDispatchReceived, "Placeholder adapter-dispatch handoff received.", nil
	case "reject":
		if dispatch.Status != domain.ExportJobDispatchStatusSubmitted {
			return repo.ExportJobDispatchUpdate{}, "", "", invalidExportJobDispatchTransition(job, dispatch, action)
		}
		update.Status = domain.ExportJobDispatchStatusRejected
		update.FinishedAt = &now
		if update.AdapterNote == "" {
			update.AdapterNote = "Placeholder adapter-dispatch handoff rejected."
		}
		return update, domain.ExportJobEventDispatchRejected, "Placeholder adapter-dispatch handoff rejected.", nil
	case "expire":
		if dispatch.Status != domain.ExportJobDispatchStatusSubmitted && dispatch.Status != domain.ExportJobDispatchStatusReceived {
			return repo.ExportJobDispatchUpdate{}, "", "", invalidExportJobDispatchTransition(job, dispatch, action)
		}
		update.Status = domain.ExportJobDispatchStatusExpired
		update.FinishedAt = &now
		expiresAt := now
		update.ExpiresAt = &expiresAt
		if update.AdapterNote == "" {
			update.AdapterNote = "Placeholder adapter-dispatch handoff expired."
		}
		return update, domain.ExportJobEventDispatchExpired, "Placeholder adapter-dispatch handoff expired.", nil
	case "mark_not_executed":
		if dispatch.Status != domain.ExportJobDispatchStatusReceived {
			return repo.ExportJobDispatchUpdate{}, "", "", invalidExportJobDispatchTransition(job, dispatch, action)
		}
		update.Status = domain.ExportJobDispatchStatusNotExecuted
		update.FinishedAt = &now
		if update.AdapterNote == "" {
			update.AdapterNote = "Placeholder adapter-dispatch handoff received but not executed."
		}
		return update, domain.ExportJobEventDispatchNotExecuted, "Placeholder adapter-dispatch handoff marked not executed.", nil
	default:
		return repo.ExportJobDispatchUpdate{}, "", "", domain.NewAppError(domain.ErrCodeInvalidRequest, "unsupported dispatch action", nil)
	}
}

func (s *exportCenterService) latestAttemptForAdvance(ctx context.Context, job *domain.ExportJob, action domain.ExportJobAdvanceAction) (*domain.ExportJobAttempt, *domain.AppError) {
	if job == nil || job.Status != domain.ExportJobStatusRunning {
		return nil, nil
	}
	switch action {
	case domain.ExportJobAdvanceActionMarkReady, domain.ExportJobAdvanceActionFail, domain.ExportJobAdvanceActionCancel:
	default:
		return nil, nil
	}
	latestAttempt, err := s.exportJobAttemptRepo.GetLatestByExportJobID(ctx, job.ExportJobID)
	if err != nil {
		return nil, infraError("get export job latest attempt", err)
	}
	if latestAttempt == nil {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			"cannot finalize export job running state without an active attempt record",
			exportJobRunnerStateDetails(job, nil, nil),
		)
	}
	if latestAttempt.Status != domain.ExportJobAttemptStatusRunning {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("cannot finalize export job attempt when latest attempt status is %s", latestAttempt.Status),
			exportJobRunnerStateDetails(job, nil, latestAttempt),
		)
	}
	return latestAttempt, nil
}

func (s *exportCenterService) dispatchForAttempt(ctx context.Context, attempt *domain.ExportJobAttempt) (*domain.ExportJobDispatch, *domain.AppError) {
	if attempt == nil || strings.TrimSpace(attempt.DispatchID) == "" {
		return nil, nil
	}
	dispatch, err := s.exportJobDispatchRepo.GetByDispatchID(ctx, attempt.DispatchID)
	if err != nil {
		return nil, infraError("get export job dispatch for attempt", err)
	}
	return dispatch, nil
}

func buildExportJobAttemptUpdate(attempt domain.ExportJobAttempt, nextStatus domain.ExportJobStatus, params AdvanceExportJobParams, now time.Time) repo.ExportJobAttemptUpdate {
	update := repo.ExportJobAttemptUpdate{
		AttemptID:   attempt.AttemptID,
		Status:      domain.ExportJobAttemptStatusRunning,
		AdapterNote: strings.TrimSpace(attempt.AdapterNote),
	}
	switch nextStatus {
	case domain.ExportJobStatusReady:
		update.Status = domain.ExportJobAttemptStatusSucceeded
		update.FinishedAt = &now
		update.AdapterNote = "Placeholder runner-adapter marked export attempt ready."
	case domain.ExportJobStatusFailed:
		update.Status = domain.ExportJobAttemptStatusFailed
		update.FinishedAt = &now
		update.ErrorMessage = strings.TrimSpace(params.FailureReason)
		if update.ErrorMessage != "" {
			update.AdapterNote = "Placeholder runner-adapter failed export attempt."
		} else {
			update.AdapterNote = "Placeholder runner-adapter failed export attempt without detailed reason."
		}
	case domain.ExportJobStatusCancelled:
		update.Status = domain.ExportJobAttemptStatusCancelled
		update.FinishedAt = &now
		update.AdapterNote = "Placeholder runner-adapter cancelled export attempt."
	}
	return update
}

func applyExportJobAttemptUpdate(attempt domain.ExportJobAttempt, update repo.ExportJobAttemptUpdate, now time.Time) domain.ExportJobAttempt {
	attempt.Status = update.Status
	attempt.FinishedAt = update.FinishedAt
	attempt.ErrorMessage = update.ErrorMessage
	attempt.AdapterNote = update.AdapterNote
	attempt.UpdatedAt = now
	return attempt
}

func invalidExportLifecycleTransition(current domain.ExportJobStatus, action domain.ExportJobAdvanceAction) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("cannot apply %s when export job status is %s", action, current), nil)
}

func buildAdvancedResultRef(job domain.ExportJob, params AdvanceExportJobParams, nextStatus domain.ExportJobStatus, now time.Time) *domain.ExportResultRef {
	template, ok := exportTemplateByKey(job.TemplateKey)
	if !ok {
		template = domain.ExportTemplate{
			Key:          job.TemplateKey,
			ResultFormat: "csv",
		}
	}

	current := cloneExportResultRef(job.ResultRef)
	if current == nil {
		current = buildPlaceholderResultRef(template, now)
	}

	ref := *current
	ref.RefType = "download_handoff_placeholder"
	ref.IsPlaceholder = true

	resultFileName := strings.TrimSpace(params.ResultFileName)
	if resultFileName == "" {
		resultFileName = strings.TrimSpace(ref.FileName)
	}
	if resultFileName == "" {
		resultFileName = buildPlaceholderResultRef(template, now).FileName
	}
	resultMimeType := strings.TrimSpace(params.ResultMimeType)
	if resultMimeType == "" {
		resultMimeType = strings.TrimSpace(ref.MimeType)
	}
	if resultMimeType == "" {
		resultMimeType = mediaTypeForResultFormat(template.ResultFormat)
	}

	ref.FileName = resultFileName
	ref.MimeType = resultMimeType

	switch nextStatus {
	case domain.ExportJobStatusQueued:
		ref.ExpiresAt = nil
		ref.Note = "Export job queued. Download handoff is reserved but no real downloadable artifact is available yet."
	case domain.ExportJobStatusRunning:
		ref.ExpiresAt = nil
		ref.Note = "Export job is processing in the placeholder lifecycle. No real file generation or storage is connected yet."
	case domain.ExportJobStatusReady:
		ref.RefType = "download_handoff"
		expiresAt := params.ExpiresAt
		if expiresAt == nil {
			defaultExpiry := now.Add(exportDownloadHandoffTTL)
			expiresAt = &defaultExpiry
		}
		expiresAtUTC := expiresAt.UTC()
		ref.ExpiresAt = &expiresAtUTC
		ref.Note = "Download handoff is ready as placeholder metadata only. This phase does not provide a real file service or signed URL."
	case domain.ExportJobStatusFailed:
		ref.ExpiresAt = nil
		failureReason := strings.TrimSpace(params.FailureReason)
		if failureReason == "" {
			ref.Note = "Export job failed before placeholder download handoff became ready."
		} else {
			ref.Note = "Export job failed: " + failureReason
		}
	case domain.ExportJobStatusCancelled:
		ref.ExpiresAt = nil
		ref.Note = "Export job cancelled. Placeholder download handoff is no longer considered ready."
	}

	return &ref
}

func buildRefreshedResultRef(current *domain.ExportResultRef, now time.Time) *domain.ExportResultRef {
	ref := cloneExportResultRef(current)
	if ref == nil {
		ref = &domain.ExportResultRef{}
	}
	ref.RefType = "download_handoff"
	ref.RefKey = "export-result/" + uuid.NewString()
	expiresAt := now.Add(exportDownloadHandoffTTL).UTC()
	ref.ExpiresAt = &expiresAt
	ref.IsPlaceholder = true
	ref.Note = "Placeholder download handoff refreshed. This phase still exposes metadata only and does not provide real file delivery."
	return ref
}

func exportTemplateByKey(templateKey string) (domain.ExportTemplate, bool) {
	for _, template := range exportTemplateCatalog() {
		if template.Key == templateKey {
			return template, true
		}
	}
	return domain.ExportTemplate{}, false
}

func cloneExportResultRef(value *domain.ExportResultRef) *domain.ExportResultRef {
	if value == nil {
		return nil
	}
	copyValue := *value
	if value.ExpiresAt != nil {
		expiresAtCopy := *value.ExpiresAt
		copyValue.ExpiresAt = &expiresAtCopy
	}
	return &copyValue
}

func buildSubmittedExportJobDispatch(exportJobID int64, triggerSource, adapterNote string, expiresAt *time.Time, now time.Time) *domain.ExportJobDispatch {
	dispatch := &domain.ExportJobDispatch{
		ExportJobID:   exportJobID,
		TriggerSource: normalizeExportJobDispatchTriggerSource(triggerSource),
		ExecutionMode: domain.ExportJobExecutionModeManualPlaceholderRunner,
		AdapterKey:    domain.ExportJobRunnerAdapterKeyManualPlaceholder,
		Status:        domain.ExportJobDispatchStatusSubmitted,
		SubmittedAt:   now,
		AdapterNote:   strings.TrimSpace(adapterNote),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if expiresAt != nil {
		value := expiresAt.UTC()
		dispatch.ExpiresAt = &value
	}
	return dispatch
}

func applyExportJobDispatchUpdate(dispatch domain.ExportJobDispatch, update repo.ExportJobDispatchUpdate, now time.Time) domain.ExportJobDispatch {
	dispatch.Status = update.Status
	dispatch.ReceivedAt = update.ReceivedAt
	dispatch.FinishedAt = update.FinishedAt
	dispatch.ExpiresAt = update.ExpiresAt
	dispatch.StatusReason = update.StatusReason
	dispatch.AdapterNote = update.AdapterNote
	dispatch.UpdatedAt = now
	return dispatch
}

func (s *exportCenterService) accessDownloadHandoff(ctx context.Context, id int64, eventType string) (*domain.ExportJobDownloadHandoff, *domain.AppError) {
	job, appErr := s.GetJob(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	now := s.nowFn().UTC()
	if appErr := validateExportDownloadAccess(job, eventType, now); appErr != nil {
		if shouldAppendExportDownloadExpired(job, now) {
			actor, _ := resolveWorkbenchActorScope(ctx)
			events, err := s.exportJobEventRepo.ListByExportJobID(ctx, id)
			if err != nil {
				return nil, infraError("list export job events for expired handoff", err)
			}
			if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
				return s.appendDownloadExpiredEventIfNeeded(ctx, tx, job, actor, events, now)
			}); err != nil {
				return nil, infraError("append export job download_expired event", err)
			}
		}
		return nil, appErr
	}

	actor, _ := resolveWorkbenchActorScope(ctx)
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		event, err := buildDownloadAccessExportJobEvent(*job, actor, eventType, now)
		if err != nil {
			return err
		}
		_, err = s.exportJobEventRepo.Append(ctx, tx, event)
		return err
	}); err != nil {
		return nil, infraError("append export job download handoff event", err)
	}

	return s.buildDownloadHandoff(ctx, id)
}

func (s *exportCenterService) hydrateExportJobAttemptSummaries(ctx context.Context, jobs []*domain.ExportJob) error {
	if len(jobs) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(jobs))
	for _, job := range jobs {
		if job == nil || job.ExportJobID <= 0 {
			continue
		}
		ids = append(ids, job.ExportJobID)
	}
	if len(ids) == 0 {
		return nil
	}
	summaries, err := s.exportJobAttemptRepo.SummariesByExportJobIDs(ctx, ids)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job == nil {
			continue
		}
		summary := summaries[job.ExportJobID]
		job.AttemptCount = summary.AttemptCount
		job.LatestAttempt = summary.LatestAttempt
		domain.HydrateExportJobDerived(job)
	}
	return nil
}

func (s *exportCenterService) hydrateExportJobDispatchSummaries(ctx context.Context, jobs []*domain.ExportJob) error {
	if len(jobs) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(jobs))
	for _, job := range jobs {
		if job == nil || job.ExportJobID <= 0 {
			continue
		}
		ids = append(ids, job.ExportJobID)
	}
	if len(ids) == 0 {
		return nil
	}
	summaries, err := s.exportJobDispatchRepo.SummariesByExportJobIDs(ctx, ids)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job == nil {
			continue
		}
		summary := summaries[job.ExportJobID]
		job.DispatchCount = summary.DispatchCount
		job.LatestDispatch = summary.LatestDispatch
		domain.HydrateExportJobDerived(job)
	}
	return nil
}

func (s *exportCenterService) hydrateExportJobEventSummaries(ctx context.Context, jobs []*domain.ExportJob) error {
	if len(jobs) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(jobs))
	for _, job := range jobs {
		if job == nil || job.ExportJobID <= 0 {
			continue
		}
		ids = append(ids, job.ExportJobID)
	}
	if len(ids) == 0 {
		return nil
	}
	summaries, err := s.exportJobEventRepo.SummariesByExportJobIDs(ctx, ids)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job == nil {
			continue
		}
		summary := summaries[job.ExportJobID]
		job.EventCount = summary.EventCount
		job.LatestEvent = summary.LatestEvent
	}
	dispatchSummaries, err := s.exportJobEventRepo.LatestSummariesByExportJobIDsAndTypes(ctx, ids, exportJobDispatchEventTypes())
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job == nil {
			continue
		}
		job.LatestDispatchEvent = dispatchSummaries[job.ExportJobID]
	}
	runnerSummaries, err := s.exportJobEventRepo.LatestSummariesByExportJobIDsAndTypes(ctx, ids, exportJobRunnerEventTypes())
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job == nil {
			continue
		}
		job.LatestRunnerEvent = runnerSummaries[job.ExportJobID]
	}
	return nil
}

func (s *exportCenterService) hydrateExportJobDownloadStates(jobs []*domain.ExportJob, now time.Time) {
	for _, job := range jobs {
		domain.HydrateExportJobDownloadState(job, now)
	}
}

func (s *exportCenterService) buildDownloadHandoff(ctx context.Context, id int64) (*domain.ExportJobDownloadHandoff, *domain.AppError) {
	job, appErr := s.GetJob(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	events, err := s.exportJobEventRepo.ListByExportJobID(ctx, id)
	if err != nil {
		return nil, infraError("list export job events for handoff", err)
	}
	handoff := domain.BuildExportJobDownloadHandoff(job, s.nowFn().UTC())
	applyExportDownloadAccessAudit(handoff, events)
	return handoff, nil
}

func buildCreatedExportJobEvent(job domain.ExportJob, actor domain.RequestActor, now time.Time) (*domain.ExportJobEvent, error) {
	payload, err := marshalExportJobEventPayload(map[string]interface{}{
		"template_key":       job.TemplateKey,
		"export_type":        job.ExportType,
		"source_query_type":  job.SourceQueryType,
		"requested_by":       job.RequestedBy,
		"progress_hint":      job.ProgressHint,
		"download_ready":     job.DownloadReady,
		"result_ref":         job.ResultRef,
		"normalized_filters": job.NormalizedFilters,
		"query_template":     job.QueryTemplate,
	})
	if err != nil {
		return nil, err
	}
	toStatus := domain.ExportJobStatusQueued
	return &domain.ExportJobEvent{
		ExportJobID: job.ExportJobID,
		EventType:   domain.ExportJobEventCreated,
		ToStatus:    &toStatus,
		ActorID:     normalizedExportJobActorID(actor),
		ActorType:   normalizedExportJobActorType(actor),
		Note:        exportJobCreatedEventNote(job.Remark),
		Payload:     payload,
		CreatedAt:   now,
	}, nil
}

func buildDispatchSubmittedExportJobEvent(job domain.ExportJob, dispatch domain.ExportJobDispatch, actor domain.RequestActor, now time.Time) (*domain.ExportJobEvent, error) {
	payload := map[string]interface{}{
		"status":           job.Status,
		"placeholder_only": true,
	}
	mergeDispatchEventPayload(payload, &dispatch)
	rawPayload, err := marshalExportJobEventPayload(payload)
	if err != nil {
		return nil, err
	}
	fromStatus := job.Status
	toStatus := job.Status
	return &domain.ExportJobEvent{
		ExportJobID: job.ExportJobID,
		EventType:   domain.ExportJobEventDispatchSubmitted,
		FromStatus:  &fromStatus,
		ToStatus:    &toStatus,
		ActorID:     normalizedExportJobActorID(actor),
		ActorType:   normalizedExportJobActorType(actor),
		Note:        "Placeholder adapter-dispatch handoff submitted.",
		Payload:     rawPayload,
		CreatedAt:   now,
	}, nil
}

func buildDispatchAdvancedExportJobEvent(job domain.ExportJob, dispatch domain.ExportJobDispatch, actor domain.RequestActor, eventType, note string, now time.Time) (*domain.ExportJobEvent, error) {
	payload := map[string]interface{}{
		"status":           job.Status,
		"placeholder_only": true,
	}
	mergeDispatchEventPayload(payload, &dispatch)
	rawPayload, err := marshalExportJobEventPayload(payload)
	if err != nil {
		return nil, err
	}
	fromStatus := job.Status
	toStatus := job.Status
	return &domain.ExportJobEvent{
		ExportJobID: job.ExportJobID,
		EventType:   eventType,
		FromStatus:  &fromStatus,
		ToStatus:    &toStatus,
		ActorID:     normalizedExportJobActorID(actor),
		ActorType:   normalizedExportJobActorType(actor),
		Note:        note,
		Payload:     rawPayload,
		CreatedAt:   now,
	}, nil
}

func buildRunnerInitiatedExportJobEvent(job domain.ExportJob, dispatch *domain.ExportJobDispatch, attempt domain.ExportJobAttempt, actor domain.RequestActor, initiationSource string, now time.Time) (*domain.ExportJobEvent, error) {
	payload := map[string]interface{}{
		"action":            domain.ExportJobAdvanceActionStart,
		"initiation_source": strings.TrimSpace(initiationSource),
		"start_mode":        domain.ExportJobStartModeExplicitInternal,
		"execution_mode":    domain.ExportJobExecutionModeManualPlaceholderRunner,
		"placeholder_only":  true,
		"status":            job.Status,
		"download_ready":    job.DownloadReady,
	}
	mergeDispatchEventPayload(payload, dispatch)
	mergeAttemptEventPayload(payload, &attempt)
	rawPayload, err := marshalExportJobEventPayload(payload)
	if err != nil {
		return nil, err
	}
	fromStatus := job.Status
	toStatus := job.Status
	return &domain.ExportJobEvent{
		ExportJobID: job.ExportJobID,
		EventType:   domain.ExportJobEventRunnerInitiated,
		FromStatus:  &fromStatus,
		ToStatus:    &toStatus,
		ActorID:     normalizedExportJobActorID(actor),
		ActorType:   normalizedExportJobActorType(actor),
		Note:        "Placeholder runner initiation accepted.",
		Payload:     rawPayload,
		CreatedAt:   now,
	}, nil
}

func buildStartedExportJobEvent(job domain.ExportJob, dispatch *domain.ExportJobDispatch, attempt domain.ExportJobAttempt, actor domain.RequestActor, initiationSource string, nextResultRef *domain.ExportResultRef, now time.Time) (*domain.ExportJobEvent, error) {
	nextStatus := domain.ExportJobStatusRunning
	payload := map[string]interface{}{
		"action":            domain.ExportJobAdvanceActionStart,
		"initiation_source": strings.TrimSpace(initiationSource),
		"start_mode":        domain.ExportJobStartModeExplicitInternal,
		"execution_mode":    domain.ExportJobExecutionModeManualPlaceholderRunner,
		"placeholder_only":  true,
		"progress_hint":     domain.ExportJobProgressForStatus(nextStatus),
		"download_ready":    domain.ExportJobDownloadReady(nextStatus, nextResultRef),
		"result_ref":        nextResultRef,
	}
	mergeDispatchEventPayload(payload, dispatch)
	mergeAttemptEventPayload(payload, &attempt)
	rawPayload, err := marshalExportJobEventPayload(payload)
	if err != nil {
		return nil, err
	}
	fromStatus := job.Status
	return &domain.ExportJobEvent{
		ExportJobID: job.ExportJobID,
		EventType:   domain.ExportJobEventStarted,
		FromStatus:  &fromStatus,
		ToStatus:    &nextStatus,
		ActorID:     normalizedExportJobActorID(actor),
		ActorType:   normalizedExportJobActorType(actor),
		Note:        "Placeholder runner started export job.",
		Payload:     rawPayload,
		CreatedAt:   now,
	}, nil
}

func buildLifecycleAdvancedExportJobEvent(job domain.ExportJob, dispatch *domain.ExportJobDispatch, nextStatus domain.ExportJobStatus, actor domain.RequestActor, params AdvanceExportJobParams, attempt *domain.ExportJobAttempt, now time.Time) (*domain.ExportJobEvent, error) {
	payload := map[string]interface{}{
		"action":         params.Action,
		"remark":         strings.TrimSpace(params.Remark),
		"progress_hint":  domain.ExportJobProgressForStatus(nextStatus),
		"download_ready": domain.ExportJobDownloadReady(nextStatus, buildAdvancedResultRef(job, params, nextStatus, now)),
	}
	mergeDispatchEventPayload(payload, dispatch)
	mergeAttemptEventPayload(payload, attempt)
	if reason := strings.TrimSpace(params.FailureReason); reason != "" {
		payload["failure_reason"] = reason
	}
	if fileName := strings.TrimSpace(params.ResultFileName); fileName != "" {
		payload["result_file_name"] = fileName
	}
	if mimeType := strings.TrimSpace(params.ResultMimeType); mimeType != "" {
		payload["result_mime_type"] = mimeType
	}
	if params.ExpiresAt != nil {
		payload["expires_at"] = params.ExpiresAt.UTC()
	}
	rawPayload, err := marshalExportJobEventPayload(payload)
	if err != nil {
		return nil, err
	}
	fromStatus := job.Status
	return &domain.ExportJobEvent{
		ExportJobID: job.ExportJobID,
		EventType:   exportJobLifecycleEventType(nextStatus),
		FromStatus:  &fromStatus,
		ToStatus:    &nextStatus,
		ActorID:     normalizedExportJobActorID(actor),
		ActorType:   normalizedExportJobActorType(actor),
		Note:        exportJobLifecycleEventNote(nextStatus, params),
		Payload:     rawPayload,
		CreatedAt:   now,
	}, nil
}

func buildAttemptFinishedExportJobEvent(job domain.ExportJob, dispatch *domain.ExportJobDispatch, attempt domain.ExportJobAttempt, actor domain.RequestActor, nextStatus domain.ExportJobStatus, params AdvanceExportJobParams, nextResultRef *domain.ExportResultRef, now time.Time) (*domain.ExportJobEvent, error) {
	eventType := exportJobAttemptFinishedEventType(nextStatus)
	if eventType == "" {
		return nil, nil
	}
	payload := map[string]interface{}{
		"action":          params.Action,
		"next_job_status": nextStatus,
		"download_ready":  domain.ExportJobDownloadReady(nextStatus, nextResultRef),
		"result_ref":      nextResultRef,
	}
	mergeDispatchEventPayload(payload, dispatch)
	mergeAttemptEventPayload(payload, &attempt)
	if reason := strings.TrimSpace(params.FailureReason); reason != "" {
		payload["failure_reason"] = reason
	}
	rawPayload, err := marshalExportJobEventPayload(payload)
	if err != nil {
		return nil, err
	}
	fromStatus := job.Status
	return &domain.ExportJobEvent{
		ExportJobID: job.ExportJobID,
		EventType:   eventType,
		FromStatus:  &fromStatus,
		ToStatus:    &nextStatus,
		ActorID:     normalizedExportJobActorID(actor),
		ActorType:   normalizedExportJobActorType(actor),
		Note:        exportJobAttemptFinishedNote(nextStatus, params),
		Payload:     rawPayload,
		CreatedAt:   now,
	}, nil
}

func buildResultRefUpdatedExportJobEvent(job domain.ExportJob, nextResultRef *domain.ExportResultRef, actor domain.RequestActor, nextStatus domain.ExportJobStatus, params AdvanceExportJobParams, now time.Time) (*domain.ExportJobEvent, error) {
	payload, err := marshalExportJobEventPayload(map[string]interface{}{
		"action":              params.Action,
		"previous_result_ref": job.ResultRef,
		"current_result_ref":  nextResultRef,
	})
	if err != nil {
		return nil, err
	}
	fromStatus := job.Status
	return &domain.ExportJobEvent{
		ExportJobID: job.ExportJobID,
		EventType:   domain.ExportJobEventResultRefUpdated,
		FromStatus:  &fromStatus,
		ToStatus:    &nextStatus,
		ActorID:     normalizedExportJobActorID(actor),
		ActorType:   normalizedExportJobActorType(actor),
		Note:        exportJobResultRefUpdatedNote(nextStatus),
		Payload:     payload,
		CreatedAt:   now,
	}, nil
}

func mergeAttemptEventPayload(payload map[string]interface{}, attempt *domain.ExportJobAttempt) {
	if payload == nil || attempt == nil {
		return
	}
	payload["attempt_id"] = attempt.AttemptID
	payload["attempt_no"] = attempt.AttemptNo
	payload["attempt_status"] = attempt.Status
	payload["trigger_source"] = strings.TrimSpace(attempt.TriggerSource)
	payload["execution_mode"] = attempt.ExecutionMode
	payload["adapter_key"] = attempt.AdapterKey
	payload["adapter_note"] = strings.TrimSpace(attempt.AdapterNote)
	payload["started_at"] = attempt.StartedAt.UTC()
	if attempt.FinishedAt != nil {
		payload["finished_at"] = attempt.FinishedAt.UTC()
	}
	if strings.TrimSpace(attempt.ErrorMessage) != "" {
		payload["error_message"] = strings.TrimSpace(attempt.ErrorMessage)
	}
}

func mergeDispatchEventPayload(payload map[string]interface{}, dispatch *domain.ExportJobDispatch) {
	if payload == nil || dispatch == nil {
		return
	}
	payload["dispatch_id"] = dispatch.DispatchID
	payload["dispatch_no"] = dispatch.DispatchNo
	payload["dispatch_status"] = dispatch.Status
	payload["dispatch_trigger_source"] = strings.TrimSpace(dispatch.TriggerSource)
	payload["dispatch_execution_mode"] = dispatch.ExecutionMode
	payload["dispatch_adapter_key"] = dispatch.AdapterKey
	payload["dispatch_submitted_at"] = dispatch.SubmittedAt.UTC()
	if dispatch.ReceivedAt != nil {
		payload["dispatch_received_at"] = dispatch.ReceivedAt.UTC()
	}
	if dispatch.FinishedAt != nil {
		payload["dispatch_finished_at"] = dispatch.FinishedAt.UTC()
	}
	if dispatch.ExpiresAt != nil {
		payload["dispatch_expires_at"] = dispatch.ExpiresAt.UTC()
	}
	if strings.TrimSpace(dispatch.StatusReason) != "" {
		payload["dispatch_status_reason"] = strings.TrimSpace(dispatch.StatusReason)
	}
	if strings.TrimSpace(dispatch.AdapterNote) != "" {
		payload["dispatch_adapter_note"] = strings.TrimSpace(dispatch.AdapterNote)
	}
}

func marshalExportJobEventPayload(payload interface{}) (json.RawMessage, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal export job event payload: %w", err)
	}
	return raw, nil
}

func exportResultRefAuditChanged(previous, current *domain.ExportResultRef) bool {
	if previous == nil && current == nil {
		return false
	}
	if previous == nil || current == nil {
		return true
	}
	if previous.RefType != current.RefType || previous.RefKey != current.RefKey || previous.FileName != current.FileName || previous.MimeType != current.MimeType || previous.IsPlaceholder != current.IsPlaceholder {
		return true
	}
	if previous.ExpiresAt == nil && current.ExpiresAt == nil {
		return false
	}
	if previous.ExpiresAt == nil || current.ExpiresAt == nil {
		return true
	}
	return !previous.ExpiresAt.UTC().Equal(current.ExpiresAt.UTC())
}

func exportJobLifecycleEventType(status domain.ExportJobStatus) string {
	switch status {
	case domain.ExportJobStatusRunning:
		return domain.ExportJobEventAdvancedToRunning
	case domain.ExportJobStatusReady:
		return domain.ExportJobEventAdvancedToReady
	case domain.ExportJobStatusFailed:
		return domain.ExportJobEventAdvancedToFailed
	case domain.ExportJobStatusCancelled:
		return domain.ExportJobEventAdvancedToCancelled
	default:
		return domain.ExportJobEventAdvancedToQueued
	}
}

func exportJobCreatedEventNote(remark string) string {
	if strings.TrimSpace(remark) != "" {
		return "Export job created: " + strings.TrimSpace(remark)
	}
	return "Export job created and queued."
}

func exportJobLifecycleEventNote(status domain.ExportJobStatus, params AdvanceExportJobParams) string {
	switch status {
	case domain.ExportJobStatusRunning:
		return "Export job advanced to running."
	case domain.ExportJobStatusReady:
		return "Export job advanced to ready placeholder handoff."
	case domain.ExportJobStatusFailed:
		if reason := strings.TrimSpace(params.FailureReason); reason != "" {
			return "Export job failed: " + reason
		}
		return "Export job advanced to failed."
	case domain.ExportJobStatusCancelled:
		return "Export job advanced to cancelled."
	default:
		return "Export job requeued."
	}
}

func exportJobAttemptStartNote(initiationSource string) string {
	if strings.TrimSpace(initiationSource) == exportJobInitiationSourceAdvanceAction {
		return "Placeholder runner-adapter accepted backward-compatible advance start."
	}
	return "Placeholder runner-adapter accepted explicit start boundary."
}

func normalizeExportJobDispatchTriggerSource(triggerSource string) string {
	trimmed := strings.TrimSpace(triggerSource)
	if trimmed == "" {
		return exportJobDispatchSourceManualSubmit
	}
	return trimmed
}

func exportJobDispatchAutoStartSubmitNote() string {
	return "Placeholder start boundary auto-submitted adapter-dispatch handoff."
}

func exportJobDispatchAutoStartReceiveNote() string {
	return "Placeholder start boundary auto-received adapter-dispatch handoff."
}

func exportJobAttemptFinishedEventType(nextStatus domain.ExportJobStatus) string {
	switch nextStatus {
	case domain.ExportJobStatusReady:
		return domain.ExportJobEventAttemptSucceeded
	case domain.ExportJobStatusFailed:
		return domain.ExportJobEventAttemptFailed
	case domain.ExportJobStatusCancelled:
		return domain.ExportJobEventAttemptCancelled
	default:
		return ""
	}
}

func exportJobAttemptFinishedNote(nextStatus domain.ExportJobStatus, params AdvanceExportJobParams) string {
	switch nextStatus {
	case domain.ExportJobStatusReady:
		return "Placeholder runner-adapter completed export attempt."
	case domain.ExportJobStatusFailed:
		if reason := strings.TrimSpace(params.FailureReason); reason != "" {
			return "Placeholder runner-adapter failed export attempt: " + reason
		}
		return "Placeholder runner-adapter failed export attempt."
	case domain.ExportJobStatusCancelled:
		return "Placeholder runner-adapter cancelled export attempt."
	default:
		return "Placeholder runner-adapter updated export attempt."
	}
}

func exportJobRunnerStateDetails(job *domain.ExportJob, latestDispatch *domain.ExportJobDispatch, latestAttempt *domain.ExportJobAttempt) map[string]interface{} {
	if job == nil {
		return nil
	}
	canStart, canStartReason := domain.ExportJobStartAdmission(job.Status, latestDispatch)
	canAttempt, canAttemptReason := domain.ExportJobAttemptAdmission(job.Status, latestDispatch)
	canDispatch, canDispatchReason := domain.ExportJobDispatchAdmission(job.Status, latestDispatch)
	canRedispatch, canRedispatchReason := domain.ExportJobRedispatchAdmission(job.Status, job.DispatchCount, latestDispatch)
	details := map[string]interface{}{
		"export_job_id":          job.ExportJobID,
		"status":                 job.Status,
		"can_start":              canStart,
		"can_start_reason":       canStartReason,
		"can_attempt":            canAttempt,
		"can_attempt_reason":     canAttemptReason,
		"can_retry":              domain.ExportJobCanRetry(job.Status, job.AttemptCount),
		"can_dispatch":           canDispatch,
		"can_dispatch_reason":    canDispatchReason,
		"can_redispatch":         canRedispatch,
		"can_redispatch_reason":  canRedispatchReason,
		"dispatchability_reason": canDispatchReason,
		"attemptability_reason":  canAttemptReason,
		"start_mode":             domain.ExportJobStartModeExplicitInternal,
		"execution_mode":         domain.ExportJobExecutionModeManualPlaceholderRunner,
		"adapter_mode":           domain.ExportJobAdapterModeDispatchThenAttempt,
		"execution_boundary":     domain.DefaultExportJobExecutionBoundary(),
		"attempt_count":          job.AttemptCount,
		"placeholder_only":       true,
	}
	if latestDispatch != nil {
		details["latest_dispatch"] = latestDispatch
	}
	if latestAttempt != nil {
		details["latest_attempt"] = latestAttempt
	}
	details["latest_admission_decision"] = domain.BuildExportJobLatestAdmissionDecision(&domain.ExportJob{
		Status:              job.Status,
		DispatchCount:       job.DispatchCount,
		CanDispatch:         canDispatch,
		CanDispatchReason:   canDispatchReason,
		CanRedispatch:       canRedispatch,
		CanRedispatchReason: canRedispatchReason,
		CanAttempt:          canAttempt,
		CanAttemptReason:    canAttemptReason,
		LatestDispatch:      latestDispatch,
		LatestAttempt:       latestAttempt,
	})
	return details
}

func exportJobDispatchStateDetails(job *domain.ExportJob, latestDispatch *domain.ExportJobDispatch) map[string]interface{} {
	var canDispatch bool
	var canDispatchReason string
	var canRedispatch bool
	var canRedispatchReason string
	var dispatchCount int64
	if job != nil {
		dispatchCount = job.DispatchCount
		canDispatch, canDispatchReason = domain.ExportJobDispatchAdmission(job.Status, latestDispatch)
		canRedispatch, canRedispatchReason = domain.ExportJobRedispatchAdmission(job.Status, dispatchCount, latestDispatch)
	}
	details := map[string]interface{}{
		"adapter_mode":       domain.ExportJobAdapterModeDispatchThenAttempt,
		"execution_boundary": domain.DefaultExportJobExecutionBoundary(),
		"placeholder_only":   true,
	}
	if job != nil {
		details["export_job_id"] = job.ExportJobID
		details["status"] = job.Status
		details["dispatch_count"] = dispatchCount
		details["can_dispatch"] = canDispatch
		details["can_dispatch_reason"] = canDispatchReason
		details["can_redispatch"] = canRedispatch
		details["can_redispatch_reason"] = canRedispatchReason
		details["dispatchability_reason"] = canDispatchReason
	}
	if latestDispatch != nil {
		details["latest_dispatch"] = latestDispatch
	}
	return details
}

func invalidExportJobDispatchTransition(job domain.ExportJob, dispatch domain.ExportJobDispatch, action string) *domain.AppError {
	return domain.NewAppError(
		domain.ErrCodeInvalidStateTransition,
		fmt.Sprintf("cannot apply dispatch action %s when latest dispatch status is %s", action, dispatch.Status),
		exportJobDispatchStateDetails(&job, &dispatch),
	)
}

func validateExportJobStart(job *domain.ExportJob, latestDispatch *domain.ExportJobDispatch) *domain.AppError {
	if job == nil {
		return domain.ErrNotFound
	}
	allowed, reason := domain.ExportJobStartAdmission(job.Status, latestDispatch)
	if !allowed {
		return domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("cannot start export job: %s", exportAdmissionReasonMessage(reason)),
			exportJobRunnerStateDetails(job, latestDispatch, job.LatestAttempt),
		)
	}
	return nil
}

func exportAdmissionReasonMessage(reason string) string {
	switch strings.TrimSpace(reason) {
	case domain.ExportJobAdmissionReasonQueuedWithoutDispatch:
		return "queued export job has no dispatch yet; dispatch submission is allowed"
	case domain.ExportJobAdmissionReasonNoHistoricalDispatch:
		return "redispatch requires at least one historical dispatch"
	case domain.ExportJobAdmissionReasonRunningDispatchBlocked:
		return "running export job cannot accept a new dispatch"
	case domain.ExportJobAdmissionReasonReadyDispatchBlocked:
		return "ready export job cannot accept a new dispatch"
	case domain.ExportJobAdmissionReasonFailedDispatchBlocked:
		return "failed export job must be requeued before dispatch"
	case domain.ExportJobAdmissionReasonCancelledDispatchBlocked:
		return "cancelled export job must be requeued before dispatch"
	case domain.ExportJobAdmissionReasonUnknownDispatchBlocked:
		return "only queued export jobs can accept dispatch submission"
	case domain.ExportJobAdmissionReasonLatestDispatchSubmittedPendingResolution:
		return "latest dispatch is still submitted and must be resolved before next admission"
	case domain.ExportJobAdmissionReasonLatestDispatchReceivedPendingStartOrResolution:
		return "latest dispatch is received and should be started or resolved before another dispatch"
	case domain.ExportJobAdmissionReasonLatestDispatchRejectedRedispatchAllowed:
		return "latest dispatch was rejected and redispatch is allowed"
	case domain.ExportJobAdmissionReasonLatestDispatchExpiredRedispatchAllowed:
		return "latest dispatch expired and redispatch is allowed"
	case domain.ExportJobAdmissionReasonLatestDispatchNotExecutedRedispatchAllowed:
		return "latest dispatch was marked not executed and redispatch is allowed"
	case domain.ExportJobAdmissionReasonRunningStartBlocked:
		return "running export job cannot start again"
	case domain.ExportJobAdmissionReasonReadyStartBlocked:
		return "ready export job cannot start again"
	case domain.ExportJobAdmissionReasonFailedStartBlocked:
		return "failed export job must be requeued before start"
	case domain.ExportJobAdmissionReasonCancelledStartBlocked:
		return "cancelled export job must be requeued before start"
	case domain.ExportJobAdmissionReasonUnknownStartBlocked:
		return "only queued export jobs can start"
	case domain.ExportJobAdmissionReasonLatestDispatchReceivedStartAllowed:
		return "latest dispatch is received and can be consumed by start"
	case domain.ExportJobAdmissionReasonNoDispatchAutoPlaceholderAllowed:
		return "no dispatch exists; start can auto-create compatibility placeholder dispatch"
	case domain.ExportJobAdmissionReasonLatestDispatchRejectedAutoPlaceholderAllowed:
		return "latest dispatch was rejected; start can auto-create compatibility placeholder dispatch"
	case domain.ExportJobAdmissionReasonLatestDispatchExpiredAutoPlaceholderAllowed:
		return "latest dispatch expired; start can auto-create compatibility placeholder dispatch"
	case domain.ExportJobAdmissionReasonLatestDispatchNotExecutedAutoPlaceholderAllowed:
		return "latest dispatch was not executed; start can auto-create compatibility placeholder dispatch"
	case domain.ExportJobAdmissionReasonRunningAttemptBlocked:
		return "running export job cannot create a new attempt"
	case domain.ExportJobAdmissionReasonReadyAttemptBlocked:
		return "ready export job cannot create a new attempt"
	case domain.ExportJobAdmissionReasonFailedAttemptBlocked:
		return "failed export job must be requeued before creating a new attempt"
	case domain.ExportJobAdmissionReasonCancelledAttemptBlocked:
		return "cancelled export job must be requeued before creating a new attempt"
	case domain.ExportJobAdmissionReasonUnknownAttemptBlocked:
		return "only queued export jobs can create a new attempt"
	default:
		return "admission rejected by current placeholder rule"
	}
}

func exportJobResultRefUpdatedNote(status domain.ExportJobStatus) string {
	switch status {
	case domain.ExportJobStatusReady:
		return "Placeholder result handoff updated for ready export job."
	default:
		return "Placeholder result handoff metadata updated."
	}
}

func normalizedExportJobActorID(actor domain.RequestActor) int64 {
	if actor.ID > 0 {
		return actor.ID
	}
	return 1
}

func normalizedExportJobActorType(actor domain.RequestActor) string {
	if strings.TrimSpace(actor.Source) != "" {
		return strings.TrimSpace(actor.Source)
	}
	return "system_fallback"
}

func buildDownloadAccessExportJobEvent(job domain.ExportJob, actor domain.RequestActor, eventType string, now time.Time) (*domain.ExportJobEvent, error) {
	payload, err := marshalExportJobEventPayload(map[string]interface{}{
		"status":           job.Status,
		"download_ready":   job.DownloadReady,
		"result_ref":       job.ResultRef,
		"ref_key":          exportResultRefKey(job.ResultRef),
		"placeholder_only": true,
	})
	if err != nil {
		return nil, err
	}
	fromStatus := job.Status
	toStatus := job.Status
	return &domain.ExportJobEvent{
		ExportJobID: job.ExportJobID,
		EventType:   eventType,
		FromStatus:  &fromStatus,
		ToStatus:    &toStatus,
		ActorID:     normalizedExportJobActorID(actor),
		ActorType:   normalizedExportJobActorType(actor),
		Note:        exportJobDownloadAccessEventNote(eventType),
		Payload:     payload,
		CreatedAt:   now,
	}, nil
}

func buildDownloadExpiredExportJobEvent(job domain.ExportJob, actor domain.RequestActor, now time.Time) (*domain.ExportJobEvent, error) {
	payload, err := marshalExportJobEventPayload(map[string]interface{}{
		"status":           job.Status,
		"download_ready":   job.DownloadReady,
		"result_ref":       job.ResultRef,
		"ref_key":          exportResultRefKey(job.ResultRef),
		"expires_at":       exportResultRefExpiresAt(job.ResultRef),
		"placeholder_only": true,
	})
	if err != nil {
		return nil, err
	}
	fromStatus := job.Status
	toStatus := job.Status
	return &domain.ExportJobEvent{
		ExportJobID: job.ExportJobID,
		EventType:   domain.ExportJobEventDownloadExpired,
		FromStatus:  &fromStatus,
		ToStatus:    &toStatus,
		ActorID:     normalizedExportJobActorID(actor),
		ActorType:   normalizedExportJobActorType(actor),
		Note:        "Placeholder download handoff expired.",
		Payload:     payload,
		CreatedAt:   now,
	}, nil
}

func buildDownloadRefreshedExportJobEvent(job domain.ExportJob, nextResultRef *domain.ExportResultRef, actor domain.RequestActor, now time.Time) (*domain.ExportJobEvent, error) {
	payload, err := marshalExportJobEventPayload(map[string]interface{}{
		"status":              job.Status,
		"download_ready":      job.DownloadReady,
		"previous_result_ref": job.ResultRef,
		"current_result_ref":  nextResultRef,
		"previous_ref_key":    exportResultRefKey(job.ResultRef),
		"current_ref_key":     exportResultRefKey(nextResultRef),
		"placeholder_only":    true,
	})
	if err != nil {
		return nil, err
	}
	fromStatus := job.Status
	toStatus := job.Status
	return &domain.ExportJobEvent{
		ExportJobID: job.ExportJobID,
		EventType:   domain.ExportJobEventDownloadRefreshed,
		FromStatus:  &fromStatus,
		ToStatus:    &toStatus,
		ActorID:     normalizedExportJobActorID(actor),
		ActorType:   normalizedExportJobActorType(actor),
		Note:        "Placeholder download handoff refreshed after expiry.",
		Payload:     payload,
		CreatedAt:   now,
	}, nil
}

func exportJobDownloadAccessEventNote(eventType string) string {
	switch eventType {
	case domain.ExportJobEventDownloadClaimed:
		return "Placeholder download handoff claimed."
	default:
		return "Placeholder download handoff read."
	}
}

func applyExportDownloadAccessAudit(handoff *domain.ExportJobDownloadHandoff, events []*domain.ExportJobEvent) {
	if handoff == nil || len(events) == 0 {
		return
	}
	currentRefKey := exportResultRefKey(handoff.ResultRef)
	for _, event := range events {
		if event == nil {
			continue
		}
		if currentRefKey != "" && exportJobEventRefKey(event) != currentRefKey {
			continue
		}
		switch event.EventType {
		case domain.ExportJobEventDownloadClaimed:
			claimedAt := event.CreatedAt.UTC()
			claimedByActorID := event.ActorID
			handoff.ClaimedAt = &claimedAt
			handoff.ClaimedByActorID = &claimedByActorID
			handoff.ClaimedByActorType = event.ActorType
		case domain.ExportJobEventDownloadRead:
			lastReadAt := event.CreatedAt.UTC()
			lastReadByActorID := event.ActorID
			handoff.LastReadAt = &lastReadAt
			handoff.LastReadByActorID = &lastReadByActorID
			handoff.LastReadByActorType = event.ActorType
		}
	}
}

func validateExportDownloadAccess(job *domain.ExportJob, eventType string, now time.Time) *domain.AppError {
	if job == nil {
		return domain.ErrNotFound
	}
	if domain.ExportJobCanAccessDownload(job.Status, job.ResultRef, now) {
		return nil
	}
	action := "read"
	if eventType == domain.ExportJobEventDownloadClaimed {
		action = "claim"
	}
	if domain.ExportJobDownloadExpired(job.Status, job.ResultRef, now) {
		return domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("cannot %s placeholder download handoff because the current handoff has expired; refresh is required", action),
			exportDownloadStateDetails(job, now),
		)
	}
	return domain.NewAppError(
		domain.ErrCodeInvalidStateTransition,
		fmt.Sprintf("cannot %s placeholder download handoff when export job status is %s", action, job.Status),
		exportDownloadStateDetails(job, now),
	)
}

func validateExportDownloadRefresh(job *domain.ExportJob, now time.Time) *domain.AppError {
	if job == nil {
		return domain.ErrNotFound
	}
	if domain.ExportJobCanRefreshDownload(job.Status, job.ResultRef, now) {
		return nil
	}
	if job.Status != domain.ExportJobStatusReady || !job.DownloadReady {
		return domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("cannot refresh placeholder download handoff when export job status is %s", job.Status),
			exportDownloadStateDetails(job, now),
		)
	}
	return domain.NewAppError(
		domain.ErrCodeInvalidStateTransition,
		"cannot refresh placeholder download handoff before the current handoff expires",
		exportDownloadStateDetails(job, now),
	)
}

func exportDownloadStateDetails(job *domain.ExportJob, now time.Time) map[string]interface{} {
	details := map[string]interface{}{
		"export_job_id":     job.ExportJobID,
		"status":            job.Status,
		"download_ready":    job.DownloadReady,
		"is_expired":        domain.ExportJobDownloadExpired(job.Status, job.ResultRef, now),
		"can_refresh":       domain.ExportJobCanRefreshDownload(job.Status, job.ResultRef, now),
		"required_status":   domain.ExportJobStatusReady,
		"storage_mode":      domain.ExportJobStorageModeLifecycleManagedResultRef,
		"delivery_mode":     domain.ExportJobDeliveryModeClaimReadRefreshHandoff,
		"storage_boundary":  domain.DefaultExportJobStorageBoundary(),
		"delivery_boundary": domain.DefaultExportJobDeliveryBoundary(),
		"placeholder_only":  true,
	}
	if job.ResultRef != nil {
		details["ref_key"] = exportResultRefKey(job.ResultRef)
		if expiresAt := exportResultRefExpiresAt(job.ResultRef); expiresAt != nil {
			details["expires_at"] = expiresAt
		}
		if strings.TrimSpace(job.ResultRef.Note) != "" {
			details["note"] = job.ResultRef.Note
		}
	}
	return details
}

func shouldAppendExportDownloadExpired(job *domain.ExportJob, now time.Time) bool {
	return job != nil && domain.ExportJobDownloadExpired(job.Status, job.ResultRef, now)
}

func (s *exportCenterService) appendDownloadExpiredEventIfNeeded(ctx context.Context, tx repo.Tx, job *domain.ExportJob, actor domain.RequestActor, events []*domain.ExportJobEvent, now time.Time) error {
	if !shouldAppendExportDownloadExpired(job, now) || exportDownloadExpiredEventExists(job.ResultRef, events) {
		return nil
	}
	event, err := buildDownloadExpiredExportJobEvent(*job, actor, now)
	if err != nil {
		return err
	}
	_, err = s.exportJobEventRepo.Append(ctx, tx, event)
	return err
}

func exportDownloadExpiredEventExists(resultRef *domain.ExportResultRef, events []*domain.ExportJobEvent) bool {
	refKey := exportResultRefKey(resultRef)
	if refKey == "" {
		return false
	}
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event == nil {
			continue
		}
		if event.EventType != domain.ExportJobEventDownloadExpired {
			continue
		}
		if exportJobEventRefKey(event) == refKey {
			return true
		}
	}
	return false
}

func exportJobEventRefKey(event *domain.ExportJobEvent) string {
	if event == nil || len(event.Payload) == 0 {
		return ""
	}
	var payload struct {
		RefKey           string                  `json:"ref_key"`
		ResultRef        *domain.ExportResultRef `json:"result_ref"`
		CurrentResultRef *domain.ExportResultRef `json:"current_result_ref"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return ""
	}
	if strings.TrimSpace(payload.RefKey) != "" {
		return strings.TrimSpace(payload.RefKey)
	}
	if payload.ResultRef != nil && strings.TrimSpace(payload.ResultRef.RefKey) != "" {
		return strings.TrimSpace(payload.ResultRef.RefKey)
	}
	if payload.CurrentResultRef != nil && strings.TrimSpace(payload.CurrentResultRef.RefKey) != "" {
		return strings.TrimSpace(payload.CurrentResultRef.RefKey)
	}
	return ""
}

func exportResultRefKey(resultRef *domain.ExportResultRef) string {
	if resultRef == nil {
		return ""
	}
	return strings.TrimSpace(resultRef.RefKey)
}

func exportResultRefExpiresAt(resultRef *domain.ExportResultRef) *time.Time {
	if resultRef == nil || resultRef.ExpiresAt == nil {
		return nil
	}
	expiresAt := resultRef.ExpiresAt.UTC()
	return &expiresAt
}

func exportJobRunnerEventTypes() []string {
	return []string{
		domain.ExportJobEventRunnerInitiated,
		domain.ExportJobEventStarted,
		domain.ExportJobEventAttemptSucceeded,
		domain.ExportJobEventAttemptFailed,
		domain.ExportJobEventAttemptCancelled,
	}
}

func exportJobDispatchEventTypes() []string {
	return []string{
		domain.ExportJobEventDispatchSubmitted,
		domain.ExportJobEventDispatchReceived,
		domain.ExportJobEventDispatchRejected,
		domain.ExportJobEventDispatchExpired,
		domain.ExportJobEventDispatchNotExecuted,
	}
}
