package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type UpsertTaskCostOverrideReviewParams struct {
	TaskID          int64
	OverrideEventID string
	ReviewRequired  *bool
	ReviewStatus    domain.TaskCostOverrideReviewStatus
	ReviewNote      string
	ReviewActor     string
	ReviewedAt      *time.Time
}

type UpsertTaskCostFinanceFlagParams struct {
	TaskID          int64
	OverrideEventID string
	FinanceRequired *bool
	FinanceStatus   domain.TaskCostOverrideFinanceStatus
	FinanceNote     string
	FinanceMarkedBy string
	FinanceMarkedAt *time.Time
}

type TaskCostOverrideAuditService interface {
	ListByTaskID(ctx context.Context, taskID int64) (*domain.TaskCostOverrideAuditTimeline, *domain.AppError)
	UpsertReview(ctx context.Context, p UpsertTaskCostOverrideReviewParams) (*domain.TaskCostOverrideGovernanceBoundary, *domain.AppError)
	UpsertFinanceFlag(ctx context.Context, p UpsertTaskCostFinanceFlagParams) (*domain.TaskCostOverrideGovernanceBoundary, *domain.AppError)
}

type taskCostOverrideAuditService struct {
	taskRepo               repo.TaskRepo
	costOverrideEventRepo  repo.TaskCostOverrideEventRepo
	taskEventRepo          repo.TaskEventRepo
	costOverrideReviewRepo repo.TaskCostOverrideReviewRepo
	costFinanceFlagRepo    repo.TaskCostFinanceFlagRepo
}

func NewTaskCostOverrideAuditService(
	taskRepo repo.TaskRepo,
	costOverrideEventRepo repo.TaskCostOverrideEventRepo,
	taskEventRepo repo.TaskEventRepo,
	costOverrideReviewRepo repo.TaskCostOverrideReviewRepo,
	costFinanceFlagRepo repo.TaskCostFinanceFlagRepo,
) TaskCostOverrideAuditService {
	return &taskCostOverrideAuditService{
		taskRepo:               taskRepo,
		costOverrideEventRepo:  costOverrideEventRepo,
		taskEventRepo:          taskEventRepo,
		costOverrideReviewRepo: costOverrideReviewRepo,
		costFinanceFlagRepo:    costFinanceFlagRepo,
	}
}

func (s *taskCostOverrideAuditService) ListByTaskID(ctx context.Context, taskID int64) (*domain.TaskCostOverrideAuditTimeline, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task for cost override audit", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}

	detail, err := s.taskRepo.GetDetailByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task detail for cost override audit", err)
	}

	var events []*domain.TaskCostOverrideAuditEvent
	if s.costOverrideEventRepo != nil {
		events, err = s.costOverrideEventRepo.ListByTaskID(ctx, taskID)
		if err != nil {
			return nil, infraError("list cost override audit events", err)
		}
	}
	if events == nil {
		events = []*domain.TaskCostOverrideAuditEvent{}
	}

	var taskEvents []*domain.TaskEvent
	if len(events) == 0 && s.taskEventRepo != nil {
		taskEvents, err = s.taskEventRepo.ListByTaskID(ctx, taskID)
		if err != nil {
			return nil, infraError("list fallback task events for cost override audit", err)
		}
	}

	reviewRecords, appErr := s.listReviewRecords(ctx, taskID)
	if appErr != nil {
		return nil, appErr
	}
	financeFlags, appErr := s.listFinanceFlags(ctx, taskID)
	if appErr != nil {
		return nil, appErr
	}

	overrideSummary, governanceAuditSummary, overrideBoundary := buildTaskCostOverrideReadModels(detail, taskEvents, events, reviewRecords, financeFlags)
	timeline := &domain.TaskCostOverrideAuditTimeline{
		TaskID:                 taskID,
		Events:                 events,
		GovernanceAuditSummary: governanceAuditSummary,
		OverrideBoundary:       overrideBoundary,
	}
	if timeline.GovernanceAuditSummary == nil && overrideSummary != nil {
		timeline.GovernanceAuditSummary = buildTaskGovernanceAuditSummary(detail, overrideSummary, len(events))
	}
	return timeline, nil
}

func (s *taskCostOverrideAuditService) UpsertReview(ctx context.Context, p UpsertTaskCostOverrideReviewParams) (*domain.TaskCostOverrideGovernanceBoundary, *domain.AppError) {
	if s.costOverrideReviewRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "cost override review placeholder repo is unavailable", nil)
	}
	event, appErr := s.loadOverrideEvent(ctx, p.TaskID, p.OverrideEventID)
	if appErr != nil {
		return nil, appErr
	}

	existing, err := s.costOverrideReviewRepo.GetByEventID(ctx, p.OverrideEventID)
	if err != nil {
		return nil, infraError("get cost override review placeholder", err)
	}

	reviewRequired := event.EventType != domain.TaskCostOverrideAuditEventReleased
	if existing != nil {
		reviewRequired = existing.ReviewRequired
	}
	if p.ReviewRequired != nil {
		reviewRequired = *p.ReviewRequired
	}

	reviewStatus := normalizeTaskCostOverrideReviewStatus(p.ReviewStatus, reviewRequired)
	if p.ReviewStatus == "" && existing != nil {
		reviewStatus = normalizeTaskCostOverrideReviewStatus(existing.ReviewStatus, reviewRequired)
	}

	reviewActor := strings.TrimSpace(p.ReviewActor)
	if reviewActor == "" && existing != nil {
		reviewActor = existing.ReviewActor
	}
	if reviewActor == "" {
		reviewActor = placeholderBoundaryActor(ctx)
	}

	reviewNote := strings.TrimSpace(p.ReviewNote)
	if reviewNote == "" && existing != nil {
		reviewNote = existing.ReviewNote
	}

	reviewedAt := cloneTimePtr(p.ReviewedAt)
	if reviewedAt == nil && existing != nil {
		reviewedAt = cloneTimePtr(existing.ReviewedAt)
	}
	if reviewedAt == nil {
		now := time.Now().UTC()
		reviewedAt = &now
	}

	record, err := s.costOverrideReviewRepo.Upsert(ctx, nil, &domain.TaskCostOverrideReviewRecord{
		OverrideEventID: p.OverrideEventID,
		TaskID:          p.TaskID,
		ReviewRequired:  reviewRequired,
		ReviewStatus:    reviewStatus,
		ReviewNote:      reviewNote,
		ReviewActor:     reviewActor,
		ReviewedAt:      reviewedAt,
	})
	if err != nil {
		return nil, infraError("upsert cost override review placeholder", err)
	}

	financeFlag, err := s.financeFlagByEventID(ctx, p.OverrideEventID)
	if err != nil {
		return nil, infraError("get finance flag after review upsert", err)
	}
	return buildTaskCostOverrideGovernanceBoundary(event, record, financeFlag), nil
}

func (s *taskCostOverrideAuditService) UpsertFinanceFlag(ctx context.Context, p UpsertTaskCostFinanceFlagParams) (*domain.TaskCostOverrideGovernanceBoundary, *domain.AppError) {
	if s.costFinanceFlagRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "cost finance placeholder repo is unavailable", nil)
	}
	event, appErr := s.loadOverrideEvent(ctx, p.TaskID, p.OverrideEventID)
	if appErr != nil {
		return nil, appErr
	}

	reviewRecord, err := s.reviewRecordByEventID(ctx, p.OverrideEventID)
	if err != nil {
		return nil, infraError("get cost override review before finance upsert", err)
	}
	existing, err := s.costFinanceFlagRepo.GetByEventID(ctx, p.OverrideEventID)
	if err != nil {
		return nil, infraError("get cost finance flag placeholder", err)
	}

	financeRequired := event.EventType != domain.TaskCostOverrideAuditEventReleased
	if existing != nil {
		financeRequired = existing.FinanceRequired
	}
	if p.FinanceRequired != nil {
		financeRequired = *p.FinanceRequired
	}

	reviewRequired := event.EventType != domain.TaskCostOverrideAuditEventReleased
	reviewStatus := normalizeTaskCostOverrideReviewStatus("", reviewRequired)
	if reviewRecord != nil {
		reviewRequired = reviewRecord.ReviewRequired
		reviewStatus = normalizeTaskCostOverrideReviewStatus(reviewRecord.ReviewStatus, reviewRequired)
	}

	financeStatus := normalizeTaskCostOverrideFinanceStatus(p.FinanceStatus, financeRequired, reviewRequired, reviewStatus)
	if p.FinanceStatus == "" && existing != nil {
		financeStatus = normalizeTaskCostOverrideFinanceStatus(existing.FinanceStatus, financeRequired, reviewRequired, reviewStatus)
	}

	financeMarkedBy := strings.TrimSpace(p.FinanceMarkedBy)
	if financeMarkedBy == "" && existing != nil {
		financeMarkedBy = existing.FinanceMarkedBy
	}
	if financeMarkedBy == "" {
		financeMarkedBy = placeholderBoundaryActor(ctx)
	}

	financeNote := strings.TrimSpace(p.FinanceNote)
	if financeNote == "" && existing != nil {
		financeNote = existing.FinanceNote
	}

	financeMarkedAt := cloneTimePtr(p.FinanceMarkedAt)
	if financeMarkedAt == nil && existing != nil {
		financeMarkedAt = cloneTimePtr(existing.FinanceMarkedAt)
	}
	if financeMarkedAt == nil {
		now := time.Now().UTC()
		financeMarkedAt = &now
	}

	flag, err := s.costFinanceFlagRepo.Upsert(ctx, nil, &domain.TaskCostFinanceFlag{
		OverrideEventID: p.OverrideEventID,
		TaskID:          p.TaskID,
		FinanceRequired: financeRequired,
		FinanceStatus:   financeStatus,
		FinanceNote:     financeNote,
		FinanceMarkedBy: financeMarkedBy,
		FinanceMarkedAt: financeMarkedAt,
	})
	if err != nil {
		return nil, infraError("upsert cost finance placeholder", err)
	}

	return buildTaskCostOverrideGovernanceBoundary(event, reviewRecord, flag), nil
}

func (s *taskCostOverrideAuditService) loadOverrideEvent(ctx context.Context, taskID int64, eventID string) (*domain.TaskCostOverrideAuditEvent, *domain.AppError) {
	if strings.TrimSpace(eventID) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "override_event_id is required", nil)
	}
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task for override placeholder", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if s.costOverrideEventRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "cost override audit repo is unavailable", nil)
	}
	event, err := s.costOverrideEventRepo.GetByEventID(ctx, eventID)
	if err != nil {
		return nil, infraError("get cost override audit event", err)
	}
	if event == nil || event.TaskID != taskID {
		return nil, domain.ErrNotFound
	}
	return event, nil
}

func (s *taskCostOverrideAuditService) listReviewRecords(ctx context.Context, taskID int64) ([]*domain.TaskCostOverrideReviewRecord, *domain.AppError) {
	if s.costOverrideReviewRepo == nil {
		return nil, nil
	}
	records, err := s.costOverrideReviewRepo.ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list cost override reviews", err)
	}
	return records, nil
}

func (s *taskCostOverrideAuditService) listFinanceFlags(ctx context.Context, taskID int64) ([]*domain.TaskCostFinanceFlag, *domain.AppError) {
	if s.costFinanceFlagRepo == nil {
		return nil, nil
	}
	flags, err := s.costFinanceFlagRepo.ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list cost finance flags", err)
	}
	return flags, nil
}

func (s *taskCostOverrideAuditService) reviewRecordByEventID(ctx context.Context, eventID string) (*domain.TaskCostOverrideReviewRecord, error) {
	if s.costOverrideReviewRepo == nil {
		return nil, nil
	}
	return s.costOverrideReviewRepo.GetByEventID(ctx, eventID)
}

func (s *taskCostOverrideAuditService) financeFlagByEventID(ctx context.Context, eventID string) (*domain.TaskCostFinanceFlag, error) {
	if s.costFinanceFlagRepo == nil {
		return nil, nil
	}
	return s.costFinanceFlagRepo.GetByEventID(ctx, eventID)
}

func placeholderBoundaryActor(ctx context.Context) string {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return ""
	}
	return fmt.Sprintf("actor:%d", actor.ID)
}
