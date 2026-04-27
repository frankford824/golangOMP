package service

import (
	"context"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

// IncidentFilter for list queries.
type IncidentFilter struct {
	Status   string
	SKUID    *int64
	Page     int
	PageSize int
}

// IncidentService manages incident lifecycle (spec §2.1 module 6, §3.3).
type IncidentService interface {
	List(ctx context.Context, filter IncidentFilter) ([]*domain.Incident, *domain.AppError)
	Assign(ctx context.Context, id, assigneeID int64, reason string) *domain.AppError
	Resolve(ctx context.Context, id int64, reason string) *domain.AppError
}

type incidentService struct {
	incidentRepo repo.IncidentRepo
	eventRepo    repo.EventRepo
	txRunner     repo.TxRunner
}

func NewIncidentService(
	incidentRepo repo.IncidentRepo,
	eventRepo repo.EventRepo,
	txRunner repo.TxRunner,
) IncidentService {
	return &incidentService{
		incidentRepo: incidentRepo,
		eventRepo:    eventRepo,
		txRunner:     txRunner,
	}
}

func (s *incidentService) List(ctx context.Context, filter IncidentFilter) ([]*domain.Incident, *domain.AppError) {
	repoFilter := repo.IncidentListFilter{
		SKUID:    filter.SKUID,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}
	if repoFilter.Page <= 0 {
		repoFilter.Page = 1
	}
	if repoFilter.PageSize <= 0 {
		repoFilter.PageSize = 20
	}
	if strings.TrimSpace(filter.Status) != "" {
		status := domain.IncidentStatus(filter.Status)
		if !isValidIncidentStatus(status) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid incident status filter", nil)
		}
		repoFilter.Status = &status
	}

	incidents, err := s.incidentRepo.List(ctx, repoFilter)
	if err != nil {
		return nil, infraError("list incidents", err)
	}
	return incidents, nil
}

func (s *incidentService) Assign(ctx context.Context, id, assigneeID int64, reason string) *domain.AppError {
	if strings.TrimSpace(reason) == "" {
		return domain.ErrReasonRequired
	}

	inc, err := s.incidentRepo.GetByID(ctx, id)
	if err != nil {
		return infraError("get incident", err)
	}
	if inc == nil {
		return domain.ErrNotFound
	}
	if inc.Status != domain.IncidentStatusOpen && inc.Status != domain.IncidentStatusInProgress {
		return domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("incident %d cannot be assigned from status %q", id, inc.Status),
			nil,
		)
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err = s.incidentRepo.UpdateStatus(ctx, tx, id, domain.IncidentStatusInProgress); err != nil {
			return fmt.Errorf("update incident status in progress: %w", err)
		}
		_, err = s.eventRepo.Append(ctx, tx, inc.SKUID, domain.EventIncidentAssigned, map[string]interface{}{
			"incident_id":  id,
			"assignee_id":  assigneeID,
			"reason":       reason,
			"triggered_by": "ops",
		})
		if err != nil {
			return fmt.Errorf("append incident.assigned: %w", err)
		}
		return nil
	})
	if txErr != nil {
		return infraError("assign incident tx", txErr)
	}

	if err = s.incidentRepo.Assign(ctx, id, assigneeID); err != nil {
		return infraError("assign incident owner", err)
	}
	return nil
}

func (s *incidentService) Resolve(ctx context.Context, id int64, reason string) *domain.AppError {
	if strings.TrimSpace(reason) == "" {
		return domain.ErrReasonRequired
	}

	inc, err := s.incidentRepo.GetByID(ctx, id)
	if err != nil {
		return infraError("get incident", err)
	}
	if inc == nil {
		return domain.ErrNotFound
	}
	if inc.Status != domain.IncidentStatusInProgress {
		return domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("incident %d cannot be resolved from status %q", id, inc.Status),
			nil,
		)
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err = s.incidentRepo.UpdateStatus(ctx, tx, id, domain.IncidentStatusResolved); err != nil {
			return fmt.Errorf("update incident status resolved: %w", err)
		}
		_, err = s.eventRepo.Append(ctx, tx, inc.SKUID, domain.EventIncidentResolved, map[string]interface{}{
			"incident_id":  id,
			"reason":       reason,
			"triggered_by": "ops",
		})
		if err != nil {
			return fmt.Errorf("append incident.resolved: %w", err)
		}
		return nil
	})
	if txErr != nil {
		return infraError("resolve incident tx", txErr)
	}

	if err = s.incidentRepo.Resolve(ctx, id, callerFromCtx(ctx)); err != nil {
		return infraError("resolve incident owner", err)
	}
	return nil
}

func isValidIncidentStatus(status domain.IncidentStatus) bool {
	switch status {
	case domain.IncidentStatusOpen, domain.IncidentStatusInProgress, domain.IncidentStatusResolved, domain.IncidentStatusClosed:
		return true
	default:
		return false
	}
}
