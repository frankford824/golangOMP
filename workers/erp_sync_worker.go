package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"workflow/service"
)

// ERPSyncWorker periodically runs the ERP sync placeholder flow.
type ERPSyncWorker struct {
	svc      service.ERPSyncService
	logger   *zap.Logger
	interval time.Duration
}

func NewERPSyncWorker(svc service.ERPSyncService, logger *zap.Logger, interval time.Duration) *ERPSyncWorker {
	return &ERPSyncWorker{
		svc:      svc,
		logger:   logger,
		interval: interval,
	}
}

func (w *ERPSyncWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("ERPSyncWorker started", zap.Duration("interval", w.interval))
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("ERPSyncWorker stopped")
			return
		case <-ticker.C:
			result, appErr := w.svc.RunScheduled(ctx)
			if appErr != nil {
				w.logger.Error("ERPSyncWorker internal error", zap.Error(appErr))
				continue
			}
			if result != nil && result.Status == "failed" {
				fields := []zap.Field{
					zap.String("status", string(result.Status)),
					zap.Int64("total_received", result.TotalReceived),
				}
				if result.ErrorMessage != nil {
					fields = append(fields, zap.String("error_message", *result.ErrorMessage))
				}
				w.logger.Warn("ERPSyncWorker sync failed", fields...)
			}
		}
	}
}
