package service

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type taskOperationalDashboardRepoStub struct {
	now      time.Time
	overview *domain.TaskOperationalOverview
	err      error
}

func (s *taskOperationalDashboardRepoStub) GetTaskOperationalOverview(_ context.Context, now time.Time) (*domain.TaskOperationalOverview, error) {
	s.now = now
	return s.overview, s.err
}

var _ repo.TaskOperationalDashboardRepo = (*taskOperationalDashboardRepoStub)(nil)

func TestTaskBoardServiceOperationalOverviewUsesAuthoritativeRepository(t *testing.T) {
	wantNow := time.Date(2026, 7, 13, 1, 30, 0, 0, time.UTC)
	want := &domain.TaskOperationalOverview{GeneratedAt: wantNow, HealthStatus: "ok"}
	dashboardRepo := &taskOperationalDashboardRepoStub{overview: want}
	svc := NewTaskBoardService(dashboardRepo).(*taskBoardService)
	svc.nowFn = func() time.Time { return wantNow }

	got, appErr := svc.GetOperationalOverview(context.Background())
	if appErr != nil {
		t.Fatalf("GetOperationalOverview() unexpected error: %+v", appErr)
	}
	if got != want {
		t.Fatalf("GetOperationalOverview() = %+v, want repository result", got)
	}
	if !dashboardRepo.now.Equal(wantNow) {
		t.Fatalf("repository now = %s, want %s", dashboardRepo.now, wantNow)
	}
}
