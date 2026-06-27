package workers

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type AssetWorkbenchPreviewWorker struct {
	svc      AssetWorkbenchPreviewProcessor
	logger   *zap.Logger
	interval time.Duration
	limit    int
}

func NewAssetWorkbenchPreviewWorker(svc AssetWorkbenchPreviewProcessor, logger *zap.Logger, interval time.Duration, limit int) *AssetWorkbenchPreviewWorker {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if limit <= 0 {
		limit = 8
	}
	return &AssetWorkbenchPreviewWorker{
		svc:      svc,
		logger:   logger.Named("asset_workbench_preview"),
		interval: interval,
		limit:    limit,
	}
}

func (w *AssetWorkbenchPreviewWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("AssetWorkbenchPreviewWorker started", zap.Duration("interval", w.interval), zap.Int("limit", w.limit))
	w.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("AssetWorkbenchPreviewWorker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *AssetWorkbenchPreviewWorker) runOnce(ctx context.Context) {
	processed, appErr := w.svc.ProcessPendingPreviews(ctx, w.limit)
	if appErr != nil {
		w.logger.Error("AssetWorkbenchPreviewWorker failed", zap.String("code", appErr.Code), zap.String("message", appErr.Message))
		return
	}
	if processed > 0 {
		w.logger.Info("AssetWorkbenchPreviewWorker processed queue", zap.Int("processed", processed))
	}
}
