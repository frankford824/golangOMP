package aichat

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"workflow/domain"
	"workflow/service/aiagent"
	analyticssvc "workflow/service/analytics"
)

type unifiedAnalyticsRepoStub struct{}

func (unifiedAnalyticsRepoStub) QueryMetric(context.Context, domain.ResourceGroupAccessFilter, domain.AnalyticsMetricDefinition, domain.AnalyticsMetricQuery) (*domain.AnalyticsMetricResult, error) {
	return &domain.AnalyticsMetricResult{MetricID: "page_views", MetricName: "页面访问", Rows: []domain.AnalyticsMetricRow{{Key: "数据中心", Label: "数据中心", EventCount: 8}}}, nil
}

type recordingUnifiedAnalyticsRepoStub struct{ query domain.AnalyticsMetricQuery }

func (s *recordingUnifiedAnalyticsRepoStub) QueryMetric(_ context.Context, _ domain.ResourceGroupAccessFilter, definition domain.AnalyticsMetricDefinition, query domain.AnalyticsMetricQuery) (*domain.AnalyticsMetricResult, error) {
	s.query = query
	return &domain.AnalyticsMetricResult{MetricID: definition.ID, MetricName: definition.Name, From: query.From, To: query.To, GroupBy: query.GroupBy, Rows: []domain.AnalyticsMetricRow{{Key: "9", Label: "王亚琳", EventCount: 3, TaskCount: 3, ActorCount: 1}}}, nil
}
func (*recordingUnifiedAnalyticsRepoStub) TraceEntity(context.Context, domain.ResourceGroupAccessFilter, domain.AnalyticsTraceQuery) ([]domain.AIRetrievalHit, error) {
	return nil, nil
}
func (unifiedAnalyticsRepoStub) TraceEntity(context.Context, domain.ResourceGroupAccessFilter, domain.AnalyticsTraceQuery) ([]domain.AIRetrievalHit, error) {
	return []domain.AIRetrievalHit{}, nil
}

type toolProviderStub struct {
	ready bool
	plan  string
	err   error
}

type capturingToolProviderStub struct {
	plan    string
	request aiagent.ChatRequest
}

func (*capturingToolProviderStub) Ready() bool { return true }
func (s *capturingToolProviderStub) CompleteText(_ context.Context, request aiagent.ChatRequest) (string, aiagent.ChatStreamResult, error) {
	s.request = request
	return s.plan, aiagent.ChatStreamResult{}, nil
}
func (*capturingToolProviderStub) Stream(context.Context, aiagent.ChatRequest, func(string) error) (aiagent.ChatStreamResult, error) {
	return aiagent.ChatStreamResult{}, nil
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
	from, to := analysisDateRange(AnalysisToolCall{From: "2026-08-15", To: "2026-08-21"}, "", time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC))
	if got := from.Format(time.RFC3339); got != "2026-08-15T00:00:00+08:00" {
		t.Fatalf("from = %s", got)
	}
	if got := to.Format(time.RFC3339); got != "2026-08-22T00:00:00+08:00" {
		t.Fatalf("to = %s", got)
	}
	from, to = analysisDateRange(AnalysisToolCall{}, "统计最近7天每天的设计产能", time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC))
	if from.Format("2006-01-02") != "2026-08-15" || to.Format("2006-01-02") != "2026-08-22" {
		t.Fatalf("relative range = %s..%s", from, to)
	}
	from, to = analysisDateRange(
		AnalysisToolCall{From: "2025-04-02", To: "2025-04-08"},
		"最近七天的设计师提交任务分布",
		time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC),
	)
	if from.Format("2006-01-02") != "2026-08-15" || to.Format("2006-01-02") != "2026-08-22" {
		t.Fatalf("hallucinated absolute range overrode Chinese relative range: %s..%s", from, to)
	}
}

func TestParseRelativeAnalysisDays(t *testing.T) {
	for input, want := range map[string]int{"7": 7, "七": 7, "两": 2, "十四": 14, "三十": 30, "一百二十": 120, "三百六十六": 366} {
		if got := parseRelativeAnalysisDays(input); got != want {
			t.Fatalf("parseRelativeAnalysisDays(%q)=%d want=%d", input, got, want)
		}
	}
}

func TestToolOrchestratorUsesScopedMySQLAnalysisEvidence(t *testing.T) {
	provider := toolProviderStub{ready: true, plan: `{"tools":[{"name":"task_kpi","from":"2026-07-01","to":"2026-07-10"}]}`}
	retriever := &evidenceRetrieverStub{}
	analytics := &analysisRepoStub{kpi: []domain.AIRetrievalHit{{DocumentID: "kpi:1", EntityType: "task_kpi", Score: 1}}}
	actor := actorWithGlobalPermission(9, domain.PermissionReportView, domain.PermissionTaskView)
	hits, meta, err := NewToolOrchestrator(provider, retriever, analytics).Gather(context.Background(), actor, "最近任务完成情况", nil, 20)
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

func TestToolOrchestratorUsesUnifiedAnalyticsRegistry(t *testing.T) {
	provider := toolProviderStub{ready: true, plan: `{"tools":[{"name":"query_distribution","arguments":{"metric_id":"page_views","group_by":"page","days":7}}]}`}
	retriever := &evidenceRetrieverStub{}
	legacy := &analysisRepoStub{}
	orchestrator := NewToolOrchestrator(provider, retriever, legacy)
	orchestrator.SetAnalyticsTools(analyticssvc.NewService(unifiedAnalyticsRepoStub{}, legacy))
	hits, _, err := orchestrator.Gather(context.Background(), actorWithGlobalPermission(9, domain.PermissionReportView, domain.PermissionTaskView), "最近七天页面访问分布", nil, 20)
	if err != nil || len(hits) != 1 || hits[0].EntityType != "analytics_metric" || retriever.calls != 0 {
		t.Fatalf("hits=%+v retrieval_calls=%d err=%v", hits, retriever.calls, err)
	}
}

func TestToolOrchestratorCarriesConversationContextAndCorrectsPersonDimension(t *testing.T) {
	provider := &capturingToolProviderStub{plan: `{"tools":[{"name":"query_distribution","arguments":{"metric_id":"task_design_submitted","group_by":"task_type","from":"2025-04-02","to":"2025-04-08"}}]}`}
	repository := &recordingUnifiedAnalyticsRepoStub{}
	orchestrator := NewToolOrchestrator(provider, &evidenceRetrieverStub{}, &analysisRepoStub{})
	orchestrator.SetAnalyticsTools(analyticssvc.NewService(repository, &analysisRepoStub{}))
	history := []domain.AIMessage{
		{Role: domain.AIMessageRoleUser, Content: "最近七天的设计师提交任务分布"},
		{Role: domain.AIMessageRoleAssistant, Content: "已有汇总，但未列姓名"},
		{Role: domain.AIMessageRoleUser, Content: "具体人员"},
	}
	hits, _, err := orchestrator.Gather(context.Background(), actorWithGlobalPermission(9, domain.PermissionReportView, domain.PermissionTaskView), "姓名：王亚琳", history, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.request.Messages) != 1 || !strings.Contains(provider.request.Messages[0].Content, "最近七天的设计师提交任务分布") || !strings.Contains(provider.request.Messages[0].Content, "姓名：王亚琳") {
		t.Fatalf("planner request lost conversation context: %+v", provider.request.Messages)
	}
	if repository.query.GroupBy != "person" {
		t.Fatalf("group_by=%q want person", repository.query.GroupBy)
	}
	if got := repository.query.From.Format("2006-01-02"); got == "2025-04-02" {
		t.Fatalf("stale planner date was accepted: %+v", repository.query)
	}
	if len(hits) != 1 || !strings.Contains(hits[0].Title, "王亚琳") {
		t.Fatalf("hits=%+v", hits)
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
	hits, _, err := NewToolOrchestrator(provider, retriever, analytics).Gather(context.Background(), actor, "查看任务和资源组", nil, 20)
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
	hits, meta, err := NewToolOrchestrator(provider, &evidenceRetrieverStub{}, analytics).Gather(context.Background(), actorWithGlobalPermission(9, domain.PermissionReportView), "KPI", nil, 20)
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
	hits, meta, err := o.Gather(context.Background(), actorWithGlobalPermission(9, domain.PermissionReportView, domain.PermissionTaskView), "查任务", nil, 20)
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
	hits, meta, err := NewToolOrchestrator(provider, retriever, analytics).Gather(context.Background(), actorWithGlobalPermission(9, domain.PermissionReportView, domain.PermissionTaskView), "经营情况", nil, 20)
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
