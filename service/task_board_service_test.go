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
	members  []domain.DesignDepartmentMember
	facts    []domain.DesignDepartmentTaskFact
	filter   domain.DesignDepartmentDashboardFilter
	err      error
}

func (s *taskOperationalDashboardRepoStub) ListDesignDepartmentMembers(_ context.Context, _ int64) ([]domain.DesignDepartmentMember, error) {
	return s.members, s.err
}

func (s *taskOperationalDashboardRepoStub) ListDesignDepartmentTaskFacts(_ context.Context, filter domain.DesignDepartmentDashboardFilter) ([]domain.DesignDepartmentTaskFact, error) {
	s.filter = filter
	return s.facts, s.err
}

func (s *taskOperationalDashboardRepoStub) GetTaskOperationalOverview(_ context.Context, now time.Time) (*domain.TaskOperationalOverview, error) {
	s.now = now
	return s.overview, s.err
}

func TestTaskBoardServiceDesignDepartmentDashboardScopesAndAggregates(t *testing.T) {
	departmentID := int64(14)
	designerID := int64(228)
	completedAt := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	deadlineAt := completedAt.Add(time.Hour)
	start := time.Date(2026, 9, 1, 16, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 1, 16, 0, 0, 0, time.UTC)
	repository := &taskOperationalDashboardRepoStub{
		members: []domain.DesignDepartmentMember{{UserID: designerID, DisplayName: "王亚琳", Username: "王亚琳", Status: "active"}},
		facts: []domain.DesignDepartmentTaskFact{{
			TaskID: 1, TaskNo: "RW-1", ProductName: "测试任务", TaskType: domain.TaskTypeNewProductDevelopment,
			TaskStatus: domain.TaskStatusCompleted, Priority: domain.TaskPriorityHigh, DesignerID: designerID,
			DesignerName: "王亚琳", DesignerStatus: "active", CreatedAt: completedAt.Add(-8 * time.Hour),
			UpdatedAt: completedAt, CompletedAt: &completedAt, DeadlineAt: &deadlineAt, PrimaryAt: completedAt,
			SKUCount: 2, ResourceUnitCount: 2, SourceFileCount: 2, FinalFileCount: 3,
			DesignSubmissionCount: 1, AuditApproveCount: 1,
		}},
	}
	svc := NewTaskBoardService(repository).(*taskBoardService)
	svc.nowFn = func() time.Time { return completedAt.Add(2 * time.Hour) }
	actor := domain.RequestActor{ID: designerID, Roles: []domain.Role{domain.RoleDeptAdmin}, Department: "视觉研创部", DepartmentID: &departmentID, EffectiveAccess: &domain.EffectiveAccess{Permissions: []domain.PermissionCode{domain.PermissionReportView}}}
	ctx := domain.WithRequestActor(context.Background(), actor)
	got, appErr := svc.GetDesignDepartmentDashboard(ctx, domain.DesignDepartmentDashboardFilter{StartAt: start, EndAt: end, DateBasis: domain.DesignDashboardDateCreated, Granularity: domain.DesignDashboardGranularityDay})
	if appErr != nil {
		t.Fatalf("dashboard error: %+v", appErr)
	}
	if repository.filter.DepartmentID != departmentID {
		t.Fatalf("department scope=%d", repository.filter.DepartmentID)
	}
	if got.Summary.TaskCount != 1 || got.Summary.DesignFileCount != 5 || got.Summary.SKUCount != 2 || got.Summary.FirstPassRate != 100 || got.Summary.OnTimeRate != 100 || got.Summary.AverageTurnaroundHours != 8 {
		t.Fatalf("summary=%+v", got.Summary)
	}
	if len(got.People) != 1 || got.People[0].TaskCount != 1 || got.People[0].WorkloadShare != 100 {
		t.Fatalf("people=%+v", got.People)
	}
	if len(got.Trend) != 1 || got.Trend[0].DesignFileCount != 5 {
		t.Fatalf("trend=%+v", got.Trend)
	}
}

func TestTaskBoardServiceDesignDepartmentDashboardRejectsNonAdmin(t *testing.T) {
	departmentID := int64(14)
	svc := NewTaskBoardService(&taskOperationalDashboardRepoStub{}).(*taskBoardService)
	for _, actor := range []domain.RequestActor{
		{ID: 1, DepartmentID: &departmentID, Roles: []domain.Role{domain.RoleDesigner}, EffectiveAccess: &domain.EffectiveAccess{Permissions: []domain.PermissionCode{domain.PermissionReportView}}},
		{ID: 1, DepartmentID: &departmentID, Roles: []domain.Role{domain.RoleDeptAdmin}, EffectiveAccess: &domain.EffectiveAccess{Permissions: []domain.PermissionCode{domain.PermissionTaskView}}},
	} {
		_, appErr := svc.GetDesignDepartmentDashboard(domain.WithRequestActor(context.Background(), actor), domain.DesignDepartmentDashboardFilter{StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), DateBasis: domain.DesignDashboardDateCreated, Granularity: domain.DesignDashboardGranularityDay})
		if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
			t.Fatalf("actor=%+v err=%+v", actor, appErr)
		}
	}
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
