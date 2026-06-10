package report_l1

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/aiagent"
)

type stubReportRepo struct {
	cards      []domain.L1Card
	throughput []domain.L1ThroughputPoint
	dwell      []domain.L1ModuleDwellPoint
}

type stubKPIAnalysisRepo struct {
	events []domain.KPIAnalysisEvent
	assets []domain.KPIAnalysisAsset
}

type failingKPIAnalysisGenerator struct{}

func (s *stubReportRepo) GetCards(context.Context) ([]domain.L1Card, error) { return s.cards, nil }
func (s *stubReportRepo) GetThroughput(context.Context, repo.ReportL1Filter) ([]domain.L1ThroughputPoint, error) {
	return s.throughput, nil
}
func (s *stubReportRepo) GetModuleDwell(context.Context, repo.ReportL1Filter) ([]domain.L1ModuleDwellPoint, error) {
	return s.dwell, nil
}

func (s *stubKPIAnalysisRepo) ListTaskEvents(context.Context, repo.KPIAnalysisFilter) ([]domain.KPIAnalysisEvent, error) {
	return s.events, nil
}

func (s *stubKPIAnalysisRepo) ListTaskAssets(context.Context, repo.KPIAnalysisFilter) ([]domain.KPIAnalysisAsset, error) {
	return s.assets, nil
}

func (failingKPIAnalysisGenerator) GenerateKPIAnalysis(context.Context, any) (*aiagent.KPIAnalysis, error) {
	return nil, errors.New("provider timeout")
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
	if _, appErr := svc.KPIEvents(context.Background(), reportActor(domain.RoleMember), KPIEventsParams{From: from, To: to}); denyCode(appErr) != domain.ErrDenyCodeReportsSuperAdminOnly {
		t.Fatalf("kpi events deny=%+v", appErr)
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
	if _, appErr := svc.KPIEvents(context.Background(), reportActor(domain.RoleSuperAdmin), KPIEventsParams{From: from, To: to}); appErr == nil || appErr.Code != CodeInvalidDateRange {
		t.Fatalf("kpi events appErr=%+v", appErr)
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

func TestKPIAIAnalysisFallsBackWhenGeneratorFails(t *testing.T) {
	from := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)
	kpiRepo := &stubKPIAnalysisRepo{
		events: []domain.KPIAnalysisEvent{
			{TaskID: 1, TaskNo: "RW-1", ProductName: "测试任务", EventType: "task.created", OperatorName: "运营甲", CreatedAt: from.Add(time.Hour)},
			{TaskID: 1, TaskNo: "RW-1", ProductName: "测试任务", EventType: "task.design.submitted", OperatorName: "设计甲", CreatedAt: from.Add(2 * time.Hour)},
			{TaskID: 1, TaskNo: "RW-1", ProductName: "测试任务", EventType: "task.audit.rejected", OperatorName: "审核甲", CreatedAt: from.Add(3 * time.Hour)},
			{TaskID: 1, TaskNo: "RW-1", ProductName: "测试任务", EventType: "task.warehouse.completed", OperatorName: "仓库甲", CreatedAt: from.Add(4 * time.Hour)},
		},
		assets: []domain.KPIAnalysisAsset{
			{TaskID: 1, TaskNo: "RW-1", ProductName: "测试任务", AssetType: "delivery", OriginalName: "delivery.psd", UploadedByName: "设计甲", CreatedAt: from.Add(2 * time.Hour)},
		},
	}
	svc := NewService(&stubReportRepo{}, WithKPIAnalysisRepo(kpiRepo), WithKPIAnalysisGenerator(failingKPIAnalysisGenerator{}))

	analysis, appErr := svc.KPIAIAnalysis(context.Background(), reportActor(domain.RoleSuperAdmin), KPIAIAnalysisParams{From: from, To: from})
	if appErr != nil {
		t.Fatalf("appErr=%+v", appErr)
	}
	if analysis == nil || analysis.Provider != "system_fallback" {
		t.Fatalf("analysis=%+v", analysis)
	}
	if analysis.Headline == "" || analysis.Overview == "" || len(analysis.Highlights) == 0 {
		t.Fatalf("fallback missing readable content: %+v", analysis)
	}
	if !strings.Contains(analysis.Overview, "仓库完成 1 次") {
		t.Fatalf("fallback overview missing warehouse completion metric: %s", analysis.Overview)
	}
	if !strings.Contains(analysis.Overview, "最终成品图 1 个") {
		t.Fatalf("fallback overview did not count delivery as final asset: %s", analysis.Overview)
	}
}

func TestKPIEventsPassThrough(t *testing.T) {
	from := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	kpiRepo := &stubKPIAnalysisRepo{
		events: []domain.KPIAnalysisEvent{
			{TaskID: 1, TaskNo: "RW-1", EventType: "task.assigned", Priority: "critical", CreatedAt: from.Add(time.Hour)},
		},
	}
	svc := NewService(&stubReportRepo{}, WithKPIAnalysisRepo(kpiRepo))

	events, appErr := svc.KPIEvents(context.Background(), reportActor(domain.RoleSuperAdmin), KPIEventsParams{From: from, To: from, Limit: 5000})
	if appErr != nil {
		t.Fatalf("appErr=%+v", appErr)
	}
	if len(events) != 1 || events[0].Priority != "critical" {
		t.Fatalf("events=%+v", events)
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
