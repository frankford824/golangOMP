package service

import (
	"context"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type TaskBoardService interface {
	GetOperationalOverview(ctx context.Context) (*domain.TaskOperationalOverview, *domain.AppError)
}

type taskBoardService struct {
	dashboardRepo repo.TaskOperationalDashboardRepo
	nowFn         func() time.Time
}

func NewTaskBoardService(dashboardRepo repo.TaskOperationalDashboardRepo) TaskBoardService {
	return &taskBoardService{dashboardRepo: dashboardRepo, nowFn: time.Now}
}

func (s *taskBoardService) GetOperationalOverview(ctx context.Context) (*domain.TaskOperationalOverview, *domain.AppError) {
	if s.dashboardRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "task operational dashboard repository is not configured", nil)
	}
	overview, err := s.dashboardRepo.GetTaskOperationalOverview(ctx, s.nowFn().UTC())
	if err != nil {
		return nil, infraError("get task operational overview", err)
	}
	return overview, nil
}
