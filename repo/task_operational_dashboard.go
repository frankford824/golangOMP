package repo

import (
	"context"
	"time"

	"workflow/domain"
)

// TaskOperationalDashboardRepo owns the direct, uncached aggregate read used by the
// main operations dashboard. It intentionally does not reuse paginated task-list data.
type TaskOperationalDashboardRepo interface {
	GetTaskOperationalOverview(ctx context.Context, now time.Time) (*domain.TaskOperationalOverview, error)
}
