package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type WorkflowTraceEventRepo interface {
	Create(ctx context.Context, tx Tx, event *domain.WorkflowTraceEvent) (int64, error)
	List(ctx context.Context, filter WorkflowTraceEventListFilter) ([]*domain.WorkflowTraceEvent, int64, error)
}

type WorkflowTraceEventListFilter struct {
	TraceID              string
	EventSource          string
	EventType            string
	Action               string
	ActorID              *int64
	ActorUsername        string
	ActorSource          string
	ActorDepartment      string
	ActorTeam            string
	RoutePath            string
	TaskID               *int64
	ModuleKey            string
	SKUCode              string
	AssetID              *int64
	DesignAssetID        *int64
	TaskAssetID          *int64
	IntegrationCallLogID *int64
	ResourceType         string
	ResourceID           string
	Outcome              string
	BusinessOnly         bool
	From                 *time.Time
	To                   *time.Time
	Page                 int
	PageSize             int
}
