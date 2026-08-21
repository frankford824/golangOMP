package aichat

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"workflow/domain"
	"workflow/service/aiagent"
)

type toolProviderStub struct {
	ready bool
	plan  string
	err   error
}

func (s toolProviderStub) Ready() bool { return s.ready }
func (s toolProviderStub) CompleteText(context.Context, aiagent.ChatRequest) (string, aiagent.ChatStreamResult, error) {
	return s.plan, aiagent.ChatStreamResult{}, s.err
}
func (s toolProviderStub) Stream(context.Context, aiagent.ChatRequest, func(string) error) (aiagent.ChatStreamResult, error) {
	return aiagent.ChatStreamResult{}, nil
}

type evidenceRetrieverStub struct {
	hits  []domain.AIRetrievalHit
	err   error
	calls int
}

func (s *evidenceRetrieverStub) HybridReady() bool { return true }
func (s *evidenceRetrieverStub) Search(context.Context, domain.RequestActor, string, int) ([]domain.AIRetrievalHit, domain.AIRetrievalMeta, error) {
	s.calls++
	return append([]domain.AIRetrievalHit{}, s.hits...), domain.AIRetrievalMeta{Mode: "hybrid", Candidates: len(s.hits)}, s.err
}

type analysisRepoStub struct {
	mu                                   sync.Mutex
	task, group, kpi, trends, experience []domain.AIRetrievalHit
	kpiCalls                             int
	taskCalls                            int
	groupCalls                           int
	lastAccess                           domain.ResourceGroupAccessFilter
	err                                  error
}

func (s *analysisRepoStub) GetTaskDetailEvidence(_ context.Context, access domain.ResourceGroupAccessFilter, _ int64) ([]domain.AIRetrievalHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskCalls++
	s.lastAccess = access
	return append([]domain.AIRetrievalHit{}, s.task...), s.err
}
func (s *analysisRepoStub) GetResourceGroupDetailEvidence(_ context.Context, access domain.ResourceGroupAccessFilter, _ int64) ([]domain.AIRetrievalHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.groupCalls++
	s.lastAccess = access
	return append([]domain.AIRetrievalHit{}, s.group...), s.err
}

func (s *analysisRepoStub) ListKPIEvidence(_ context.Context, access domain.ResourceGroupAccessFilter, _, _ time.Time, _ int) ([]domain.AIRetrievalHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kpiCalls++
	s.lastAccess = access
	return append([]domain.AIRetrievalHit{}, s.kpi...), s.err
}
func (s *analysisRepoStub) ListBusinessTrendEvidence(context.Context, domain.ResourceGroupAccessFilter, time.Time, time.Time, int) ([]domain.AIRetrievalHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.AIRetrievalHit{}, s.trends...), s.err
}
func (s *analysisRepoStub) ListExperienceEvidence(context.Context, domain.ResourceGroupAccessFilter, time.Time, time.Time, int) ([]domain.AIRetrievalHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.AIRetrievalHit{}, s.experience...), s.err
}

func TestValidateAnalysisPlanRejectsUnknownTooManyAndInvalidDates(t *testing.T) {
	cases := []AnalysisPlan{
		{Tools: []AnalysisToolCall{{Name: "sql"}}},
		{Tools: []AnalysisToolCall{{Name: "global_search"}, {Name: "task_kpi"}, {Name: "business_trends"}, {Name: "experience_summary"}}},
		{Tools: []AnalysisToolCall{{Name: "task_kpi", From: "2026-07-10", To: "2026-07-01"}}},
	}
	for _, item := range cases {
		if err := validateAnalysisPlan(&item); err == nil {
			t.Fatalf("plan should be rejected: %+v", item)
		}
	}
}

func TestAnalysisDateRangeUsesBeijingBusinessDays(t *testing.T) {
	from, to := analysisDateRange(AnalysisToolCall{From: "2026-08-15", To: "2026-08-21"}, time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC))
	if got := from.Format(time.RFC3339); got != "2026-08-15T00:00:00+08:00" {
		t.Fatalf("from = %s", got)
	}
	if got := to.Format(time.RFC3339); got != "2026-08-22T00:00:00+08:00" {
		t.Fatalf("to = %s", got)
	}
}

func TestToolOrchestratorUsesScopedMySQLAnalysisEvidence(t *testing.T) {
	provider := toolProviderStub{ready: true, plan: `{"tools":[{"name":"task_kpi","from":"2026-07-01","to":"2026-07-10"}]}`}
	retriever := &evidenceRetrieverStub{}
	analytics := &analysisRepoStub{kpi: []domain.AIRetrievalHit{{DocumentID: "kpi:1", EntityType: "task_kpi", Score: 1}}}
	actor := actorWithGlobalPermission(9, domain.PermissionReportView, domain.PermissionTaskView)
	hits, meta, err := NewToolOrchestrator(provider, retriever, analytics).Gather(context.Background(), actor, "最近任务完成情况", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].DocumentID != "kpi:1" || analytics.kpiCalls != 1 || retriever.calls != 0 {
		t.Fatalf("hits=%+v analytics=%d retrieval=%d", hits, analytics.kpiCalls, retriever.calls)
	}
	if !analytics.lastAccess.Global || meta.Mode != "hybrid" {
		t.Fatalf("access=%+v meta=%+v", analytics.lastAccess, meta)
	}
}

func TestToolOrchestratorLoadsEntityDetailThroughExactScopedTool(t *testing.T) {
	provider := toolProviderStub{ready: true, plan: `{"tools":[{"name":"task_detail","entity_id":"42"},{"name":"resource_group_detail","entity_id":"81"}]}`}
	retriever := &evidenceRetrieverStub{}
	analytics := &analysisRepoStub{
		task:  []domain.AIRetrievalHit{{DocumentID: "task:42", EntityType: "task", Score: 1}},
		group: []domain.AIRetrievalHit{{DocumentID: "task_resource_group:81", EntityType: "task_resource_group", Score: 1}},
	}
	actor := actorWithGlobalPermission(9, domain.PermissionReportView, domain.PermissionTaskView, domain.PermissionAssetView)
	hits, _, err := NewToolOrchestrator(provider, retriever, analytics).Gather(context.Background(), actor, "查看任务和资源组", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || analytics.taskCalls != 1 || analytics.groupCalls != 1 || retriever.calls != 0 {
		t.Fatalf("hits=%+v task_calls=%d group_calls=%d retrieval_calls=%d", hits, analytics.taskCalls, analytics.groupCalls, retriever.calls)
	}
}

func TestToolOrchestratorFailsClosedWithoutTaskView(t *testing.T) {
	provider := toolProviderStub{ready: true, plan: `{"tools":[{"name":"task_kpi"}]}`}
	analytics := &analysisRepoStub{kpi: []domain.AIRetrievalHit{{DocumentID: "must-not-leak"}}}
	hits, meta, err := NewToolOrchestrator(provider, &evidenceRetrieverStub{}, analytics).Gather(context.Background(), actorWithGlobalPermission(9, domain.PermissionReportView), "KPI", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 || analytics.kpiCalls != 0 || meta.Reason != "" {
		t.Fatalf("hits=%+v calls=%d meta=%+v", hits, analytics.kpiCalls, meta)
	}
}

func TestToolOrchestratorPlannerFailureFallsBackToSharedRetrieval(t *testing.T) {
	retriever := &evidenceRetrieverStub{hits: []domain.AIRetrievalHit{{DocumentID: "task:1"}}}
	o := NewToolOrchestrator(toolProviderStub{ready: true, err: errors.New("provider down")}, retriever)
	hits, meta, err := o.Gather(context.Background(), actorWithGlobalPermission(9, domain.PermissionReportView, domain.PermissionTaskView), "查任务", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || retriever.calls != 1 || meta.Reason != "planner_fallback" {
		t.Fatalf("hits=%+v calls=%d meta=%+v", hits, retriever.calls, meta)
	}
}

func TestToolOrchestratorRuntimeFailureFallsBackToSharedRetrieval(t *testing.T) {
	retriever := &evidenceRetrieverStub{hits: []domain.AIRetrievalHit{{DocumentID: "task:fallback"}}}
	analytics := &analysisRepoStub{err: errors.New("analytics unavailable")}
	provider := toolProviderStub{ready: true, plan: `{"tools":[{"name":"task_kpi"},{"name":"business_trends"}]}`}
	hits, meta, err := NewToolOrchestrator(provider, retriever, analytics).Gather(context.Background(), actorWithGlobalPermission(9, domain.PermissionReportView, domain.PermissionTaskView), "经营情况", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].DocumentID != "task:fallback" || retriever.calls != 1 || !meta.Degraded || meta.Reason != "tool_fallback" {
		t.Fatalf("hits=%+v calls=%d meta=%+v", hits, retriever.calls, meta)
	}
}

func actorWithGlobalPermission(id int64, permissions ...domain.PermissionCode) domain.RequestActor {
	assignments := make([]domain.AccessAssignment, 0, len(permissions))
	sources := make([]domain.EffectiveAccessNote, 0, len(permissions))
	for index, permission := range permissions {
		roleID := int64(index + 1)
		assignments = append(assignments, domain.AccessAssignment{UserID: id, RoleID: roleID, ScopeMode: domain.AccessScopeGlobal})
		sources = append(sources, domain.EffectiveAccessNote{Permission: permission, RoleID: roleID, ScopeMode: domain.AccessScopeGlobal})
	}
	return domain.RequestActor{ID: id, Permissions: permissions, EffectiveAccess: &domain.EffectiveAccess{UserID: id, Permissions: permissions, Assignments: assignments, Sources: sources}}
}
