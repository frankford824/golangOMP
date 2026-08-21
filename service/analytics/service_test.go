package analytics

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
)

type analyticsRepoStub struct {
	definition domain.AnalyticsMetricDefinition
	query      domain.AnalyticsMetricQuery
	result     *domain.AnalyticsMetricResult
}

func (s *analyticsRepoStub) QueryMetric(_ context.Context, _ domain.ResourceGroupAccessFilter, definition domain.AnalyticsMetricDefinition, query domain.AnalyticsMetricQuery) (*domain.AnalyticsMetricResult, error) {
	s.definition, s.query = definition, query
	return s.result, nil
}
func (s *analyticsRepoStub) TraceEntity(context.Context, domain.ResourceGroupAccessFilter, domain.AnalyticsTraceQuery) ([]domain.AIRetrievalHit, error) {
	return []domain.AIRetrievalHit{{DocumentID: "trace:1", EntityType: "analytics_trace", Score: 1}}, nil
}

type legacyAnalyticsStub struct{ hits []domain.AIRetrievalHit }

func (s *legacyAnalyticsStub) GetTaskDetailEvidence(context.Context, domain.ResourceGroupAccessFilter, int64) ([]domain.AIRetrievalHit, error) {
	return nil, nil
}
func (s *legacyAnalyticsStub) GetResourceGroupDetailEvidence(context.Context, domain.ResourceGroupAccessFilter, int64) ([]domain.AIRetrievalHit, error) {
	return nil, nil
}
func (s *legacyAnalyticsStub) ListKPIEvidence(context.Context, domain.ResourceGroupAccessFilter, time.Time, time.Time, int) ([]domain.AIRetrievalHit, error) {
	return s.hits, nil
}
func (s *legacyAnalyticsStub) ListBusinessTrendEvidence(context.Context, domain.ResourceGroupAccessFilter, time.Time, time.Time, int) ([]domain.AIRetrievalHit, error) {
	return nil, nil
}
func (s *legacyAnalyticsStub) ListExperienceEvidence(context.Context, domain.ResourceGroupAccessFilter, time.Time, time.Time, int) ([]domain.AIRetrievalHit, error) {
	return nil, nil
}

func TestAnalyticsServiceCatalogAndGenericQuery(t *testing.T) {
	repository := &analyticsRepoStub{result: &domain.AnalyticsMetricResult{MetricID: "page_views", MetricName: "页面访问", Rows: []domain.AnalyticsMetricRow{{Key: "数据中心", Label: "数据中心", EventCount: 8}}}}
	service := NewService(repository, &legacyAnalyticsStub{})
	actor := analyticsActor()
	catalog, appErr := service.Call(context.Background(), actor, "list_metrics", nil)
	if appErr != nil || len(catalog.Hits) != 1 || catalog.Text == "" {
		t.Fatalf("catalog=%+v err=%+v", catalog, appErr)
	}
	output, appErr := service.Call(context.Background(), actor, "query_distribution", map[string]interface{}{
		"metric_id": "page_views", "group_by": "page", "days": 7, "limit": 20,
	})
	if appErr != nil || len(output.Hits) != 1 || repository.definition.ID != "page_views" || repository.query.GroupBy != "page" {
		t.Fatalf("output=%+v definition=%+v query=%+v err=%+v", output, repository.definition, repository.query, appErr)
	}
}

func TestAnalyticsServiceDerivedMetricUsesCompatibilityExecutor(t *testing.T) {
	legacy := &legacyAnalyticsStub{hits: []domain.AIRetrievalHit{{DocumentID: "kpi:summary", EntityType: "task_kpi", Score: 1}}}
	service := NewService(&analyticsRepoStub{}, legacy)
	output, appErr := service.Call(context.Background(), analyticsActor(), "query_metric", map[string]interface{}{"metric_id": "design_productivity", "days": 7})
	if appErr != nil || len(output.Hits) != 1 || output.Hits[0].DocumentID != "kpi:summary" {
		t.Fatalf("output=%+v err=%+v", output, appErr)
	}
}

func TestAnalyticsServiceRequiresExplicitPermissions(t *testing.T) {
	service := NewService(&analyticsRepoStub{}, &legacyAnalyticsStub{})
	if _, appErr := service.Call(context.Background(), domain.RequestActor{ID: 9}, "list_metrics", nil); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("appErr=%+v", appErr)
	}
}

func analyticsActor() domain.RequestActor {
	permissions := []domain.PermissionCode{domain.PermissionReportView, domain.PermissionTaskView}
	return domain.RequestActor{ID: 9, Permissions: permissions, EffectiveAccess: &domain.EffectiveAccess{
		UserID: 9, Permissions: permissions,
		Assignments: []domain.AccessAssignment{{UserID: 9, RoleID: 1, ScopeMode: domain.AccessScopeGlobal}},
		Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionReportView, RoleID: 1, ScopeMode: domain.AccessScopeGlobal}, {Permission: domain.PermissionTaskView, RoleID: 1, ScopeMode: domain.AccessScopeGlobal}},
	}}
}
