package repo

import (
	"context"
	"workflow/domain"
)

type CostMaintenanceFilter struct {
	Keyword, Status, Reason string
	ReviewState             string
	BeforeID                int64
	Page, PageSize          int
}

type CostMaintenanceRepo interface {
	List(context.Context, CostMaintenanceFilter) ([]*domain.CostMaintenanceRow, error)
	LockRow(context.Context, Tx, int64) (*domain.CostMaintenanceRow, error)
	ListLogs(context.Context, CostMaintenanceFilter) ([]*domain.CostMaintenanceLog, int64, error)
	Project(context.Context, Tx, *domain.TaskSKUItem) error
	Save(context.Context, Tx, *domain.TaskSKUItem, bool, bool) error
	AppendLog(context.Context, Tx, int64, int64, string, string, string, int64, interface{}) error
}
