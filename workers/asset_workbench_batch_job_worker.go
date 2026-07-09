package workers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AssetWorkbenchBatchJobWorker struct {
	svc      AssetWorkbenchBatchJobProcessor
	logger   *zap.Logger
	interval time.Duration
	limit    int
	workerID string
}

func NewAssetWorkbenchBatchJobWorker(svc AssetWorkbenchBatchJobProcessor, logger *zap.Logger, interval time.Duration, limit int) *AssetWorkbenchBatchJobWorker {
	if logger == nil {
		logger = zap.NewNop()
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if limit <= 0 {
		limit = 2
	}
	return &AssetWorkbenchBatchJobWorker{
		svc:      svc,
		logger:   logger.Named("asset_workbench_batch_job_worker"),
		interval: interval,
		limit:    limit,
		workerID: "asset-workbench-batch-" + uuid.NewString(),
	}
}

func (w *AssetWorkbenchBatchJobWorker) Run(ctx context.Context) {
	if w.svc == nil {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("AssetWorkbenchBatchJobWorker started", zap.Duration("interval", w.interval), zap.Int("limit", w.limit))
	w.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("AssetWorkbenchBatchJobWorker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *AssetWorkbenchBatchJobWorker) runOnce(ctx context.Context) {
	processed, appErr := w.svc.ProcessPendingBatchJobs(ctx, w.workerID, w.limit)
	if appErr != nil {
		w.logger.Error("AssetWorkbenchBatchJobWorker failed", zap.String("code", appErr.Code), zap.String("message", appErr.Message))
		return
	}
	if processed > 0 {
		w.logger.Info("AssetWorkbenchBatchJobWorker processed jobs", zap.Int("processed", processed))
	}
}
