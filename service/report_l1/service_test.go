package report_l1

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type stubReportRepo struct {
	cards      []domain.L1Card
	throughput []domain.L1ThroughputPoint
	dwell      []domain.L1ModuleDwellPoint
}

func (s *stubReportRepo) GetCards(context.Context) ([]domain.L1Card, error) { return s.cards, nil }
func (s *stubReportRepo) GetThroughput(context.Context, repo.ReportL1Filter) ([]domain.L1ThroughputPoint, error) {
	return s.throughput, nil
}
func (s *stubReportRepo) GetModuleDwell(context.Context, repo.ReportL1Filter) ([]domain.L1ModuleDwellPoint, error) {
	return s.dwell, nil
}

func TestReportL1ServiceRBAC(t *testing.T) {
	svc := NewService(&stubReportRepo{})
	from := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	to := from
	if _, appErr := svc.Cards(context.Background(), reportActor(domain.RoleMember)); denyCode(appErr) != domain.ErrDenyCodeReportsSuperAdminOnly {
		t.Fatalf("cards deny=%+v", appErr)
	}
	if _, appErr := svc.Throughput(context.Background(), reportActor(domain.RoleMember), from, to, nil, nil); denyCode(appErr) != domain.ErrDenyCodeReportsSuperAdminOnly {
		t.Fatalf("throughput deny=%+v", appErr)
	}
	if _, appErr := svc.ModuleDwell(context.Background(), reportActor(domain.RoleMember), from, to, nil, nil); denyCode(appErr) != domain.ErrDenyCodeReportsSuperAdminOnly {
		t.Fatalf("dwell deny=%+v", appErr)
	}
}

func TestReportL1ServiceDateRange(t *testing.T) {
	svc := NewService(&stubReportRepo{})
	from := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	if _, appErr := svc.Throughput(context.Background(), reportActor(domain.RoleSuperAdmin), from, to, nil, nil); appErr == nil || appErr.Code != CodeInvalidDateRange {
		t.Fatalf("throughput appErr=%+v", appErr)
	}
	if _, appErr := svc.ModuleDwell(context.Background(), reportActor(domain.RoleSuperAdmin), from, to, nil, nil); appErr == nil || appErr.Code != CodeInvalidDateRange {
		t.Fatalf("dwell appErr=%+v", appErr)
	}
}

func TestReportL1ServicePassThrough(t *testing.T) {
	from := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	repo := &stubReportRepo{
		cards:      []domain.L1Card{{Key: "tasks_in_progress", Title: "Tasks in progress", Value: 1}},
		throughput: []domain.L1ThroughputPoint{{Date: "2026-04-20", Created: 3}, {Date: "2026-04-21", Created: 3}, {Date: "2026-04-22", Created: 4}},
		dwell:      []domain.L1ModuleDwellPoint{{ModuleKey: "design", AvgDwellSeconds: 10, P95DwellSeconds: 20, Samples: 2}},
	}
	svc := NewService(repo)
	cards, appErr := svc.Cards(context.Background(), reportActor(domain.RoleSuperAdmin))
	if appErr != nil || len(cards) != 1 {
		t.Fatalf("cards=%+v err=%+v", cards, appErr)
	}
	points, appErr := svc.Throughput(context.Background(), reportActor(domain.RoleSuperAdmin), from, from.AddDate(0, 0, 2), nil, nil)
	if appErr != nil || len(points) != 3 || points[2].Created != 4 {
		t.Fatalf("throughput=%+v err=%+v", points, appErr)
	}
	dwell, appErr := svc.ModuleDwell(context.Background(), reportActor(domain.RoleSuperAdmin), from, from, nil, nil)
	if appErr != nil || len(dwell) != 1 || dwell[0].Samples != 2 {
		t.Fatalf("dwell=%+v err=%+v", dwell, appErr)
	}
}

func reportActor(role domain.Role) domain.RequestActor {
	return domain.RequestActor{ID: 1, Roles: []domain.Role{role}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced}
}

func denyCode(appErr *domain.AppError) string {
	if appErr == nil {
		return ""
	}
	if details, ok := appErr.Details.(map[string]string); ok {
		return details["deny_code"]
	}
	return ""
}
