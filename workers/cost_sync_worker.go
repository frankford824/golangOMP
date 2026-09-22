package workers

import (
	"context"
	"go.uber.org/zap"
	"time"
	"workflow/service"
)

func RunCostSyncWorker(ctx context.Context, s *service.CostSyncService, logger *zap.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	logger.Info("cost sync worker started", zap.Duration("interval", 5*time.Second), zap.Int("batch_limit", 4))
	for {
		if err := s.RunOnce(ctx); err != nil {
			logger.Error("cost sync cycle failed", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.Wake():
		}
	}
}
