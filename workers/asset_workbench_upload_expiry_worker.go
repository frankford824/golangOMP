package workers

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type AssetWorkbenchUploadExpiryWorker struct {
	svc      AssetWorkbenchMaintenanceProcessor
	logger   *zap.Logger
	interval time.Duration
	limit    int
}

func NewAssetWorkbenchUploadExpiryWorker(svc AssetWorkbenchMaintenanceProcessor, logger *zap.Logger, interval time.Duration, limit int) *AssetWorkbenchUploadExpiryWorker {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	if limit <= 0 {
		limit = 100
	}
	return &AssetWorkbenchUploadExpiryWorker{
		svc:      svc,
		logger:   logger.Named("asset_workbench_upload_expiry"),
		interval: interval,
		limit:    limit,
	}
}

func (w *AssetWorkbenchUploadExpiryWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("AssetWorkbenchUploadExpiryWorker started", zap.Duration("interval", w.interval), zap.Int("limit", w.limit))
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("AssetWorkbenchUploadExpiryWorker stopped")
			return
		case <-ticker.C:
			processed, appErr := w.svc.ExpireUploadSessions(ctx, w.limit)
			if appErr != nil {
				w.logger.Error("AssetWorkbenchUploadExpiryWorker failed", zap.String("code", appErr.Code), zap.String("message", appErr.Message))
				continue
			}
			if processed > 0 {
				w.logger.Info("AssetWorkbenchUploadExpiryWorker expired sessions", zap.Int("processed", processed))
			}
		}
	}
}
