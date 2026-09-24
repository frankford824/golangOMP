package repo

import (
	"context"
	"encoding/json"
	"time"

	"workflow/domain"
)

type AssetMediaJobRepo interface {
	Enqueue(context.Context, *domain.AssetMediaJob, int64) (*domain.AssetMediaJob, string, error)
	Get(context.Context, string) (*domain.AssetMediaJob, error)
	Claim(context.Context, string, string, int, ...string) ([]*domain.AssetMediaJob, error)
	Heartbeat(context.Context, string, string, int64, string, int64, int64) error
	AssertLease(context.Context, Tx, string, string, int64) error
	SaveCheckpoint(context.Context, string, string, int64, json.RawMessage) error
	Finish(context.Context, string, string, int64, string, json.RawMessage, string, bool) error
	ListRequests(context.Context, int64, time.Time) ([]domain.AssetMediaRequest, error)
	HasRequest(context.Context, string, int64) (bool, error)
	GetRequestAccess(context.Context, string, int64) (*domain.AssetMediaAccessReference, error)
	CancelRequest(context.Context, string, int64) error
	Retry(context.Context, string, int64) error
}
