package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"workflow/service"
)

type ProductManagementSyncWorker struct {
	svc      service.ProductManagementService
	logger   *zap.Logger
	interval time.Duration
	limit    int
}

func NewProductManagementSyncWorker(svc service.ProductManagementService, logger *zap.Logger, interval time.Duration, limit int) *ProductManagementSyncWorker {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if limit <= 0 {
		limit = 10
	}
	return &ProductManagementSyncWorker{
		svc:      svc,
		logger:   logger.Named("product_management_sync"),
		interval: interval,
		limit:    limit,
	}
}

func (w *ProductManagementSyncWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("ProductManagementSyncWorker started", zap.Duration("interval", w.interval), zap.Int("limit", w.limit))
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("ProductManagementSyncWorker stopped")
			return
		case <-ticker.C:
			processed, appErr := w.svc.ProcessQueuedERPSync(ctx, w.limit)
			if appErr != nil {
				w.logger.Error("ProductManagementSyncWorker failed", zap.String("code", appErr.Code), zap.String("message", appErr.Message))
				continue
			}
			if processed > 0 {
				w.logger.Info("ProductManagementSyncWorker processed queue", zap.Int("processed", processed))
			}
		}
	}
}
