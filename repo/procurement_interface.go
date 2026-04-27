package repo

import (
	"context"

	"workflow/domain"
)

// ProcurementRepo handles procurement_records table for purchase-task preparation.
type ProcurementRepo interface {
	GetByTaskID(ctx context.Context, taskID int64) (*domain.ProcurementRecord, error)
	ListItemsByTaskID(ctx context.Context, taskID int64) ([]*domain.ProcurementRecordItem, error)
	Upsert(ctx context.Context, tx Tx, record *domain.ProcurementRecord) error
	CreateItems(ctx context.Context, tx Tx, items []*domain.ProcurementRecordItem) error
}
