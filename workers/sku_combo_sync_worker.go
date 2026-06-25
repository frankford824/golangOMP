package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"workflow/service"
)

type SKUComboSyncWorker struct {
	svc      service.SKUComboSyncService
	logger   *zap.Logger
	interval time.Duration
}

func NewSKUComboSyncWorker(svc service.SKUComboSyncService, logger *zap.Logger, interval time.Duration) *SKUComboSyncWorker {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SKUComboSyncWorker{svc: svc, logger: logger.Named("sku_combo_sync"), interval: interval}
}

func (w *SKUComboSyncWorker) Run(ctx context.Context) {
	if w == nil || w.svc == nil {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("SKUComboSyncWorker started", zap.Duration("interval", w.interval))
	w.process(ctx)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("SKUComboSyncWorker stopped")
			return
		case <-ticker.C:
			w.process(ctx)
		}
	}
}

func (w *SKUComboSyncWorker) process(ctx context.Context) {
	processed, appErr := w.svc.ProcessNextPage(ctx)
	if appErr != nil {
		w.logger.Warn("SKUComboSyncWorker failed", zap.String("code", appErr.Code), zap.String("message", appErr.Message))
		return
	}
	if processed > 0 {
		w.logger.Info("SKUComboSyncWorker processed page", zap.Int("processed", processed))
	}
}
