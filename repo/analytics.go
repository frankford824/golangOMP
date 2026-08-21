package repo

import (
	"context"

	"workflow/domain"
)

// AnalyticsRepo is the read-only execution boundary shared by Data Center AI
// and the Streamable HTTP MCP endpoint. Implementations must compile only
// allow-listed metric definitions and groupings; callers never provide SQL.
type AnalyticsRepo interface {
	QueryMetric(ctx context.Context, access domain.ResourceGroupAccessFilter, definition domain.AnalyticsMetricDefinition, query domain.AnalyticsMetricQuery) (*domain.AnalyticsMetricResult, error)
	TraceEntity(ctx context.Context, access domain.ResourceGroupAccessFilter, query domain.AnalyticsTraceQuery) ([]domain.AIRetrievalHit, error)
}
