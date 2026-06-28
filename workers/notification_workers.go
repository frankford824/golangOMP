package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
)

type NotificationProcessor interface {
	ProcessWebPushOutbox(ctx context.Context, limit int) (int, *domain.AppError)
	ReconcileSKUSyncFailureNotifications(ctx context.Context, limit int) (int, *domain.AppError)
}

type NotificationWebPushWorker struct {
	processor NotificationProcessor
	logger    *zap.Logger
	interval  time.Duration
	limit     int
}

func NewNotificationWebPushWorker(processor NotificationProcessor, logger *zap.Logger, interval time.Duration, limit int) *NotificationWebPushWorker {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if limit <= 0 {
		limit = 20
	}
	return &NotificationWebPushWorker{
		processor: processor,
		logger:    logger.Named("notification_web_push"),
		interval:  interval,
		limit:     limit,
	}
}

func (w *NotificationWebPushWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("NotificationWebPushWorker started", zap.Duration("interval", w.interval), zap.Int("limit", w.limit))
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("NotificationWebPushWorker stopped")
			return
		case <-ticker.C:
			processed, appErr := w.processor.ProcessWebPushOutbox(ctx, w.limit)
			if appErr != nil {
				w.logger.Error("NotificationWebPushWorker failed", zap.String("code", appErr.Code), zap.String("message", appErr.Message))
				continue
			}
			if processed > 0 {
				w.logger.Info("NotificationWebPushWorker processed queue", zap.Int("processed", processed))
			}
		}
	}
}

type SKUSyncFailureNotificationWorker struct {
	processor NotificationProcessor
	logger    *zap.Logger
	interval  time.Duration
	limit     int
}

func NewSKUSyncFailureNotificationWorker(processor NotificationProcessor, logger *zap.Logger, interval time.Duration, limit int) *SKUSyncFailureNotificationWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if limit <= 0 {
		limit = 50
	}
	return &SKUSyncFailureNotificationWorker{
		processor: processor,
		logger:    logger.Named("sku_sync_failure_notification"),
		interval:  interval,
		limit:     limit,
	}
}

func (w *SKUSyncFailureNotificationWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("SKUSyncFailureNotificationWorker started", zap.Duration("interval", w.interval), zap.Int("limit", w.limit))
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("SKUSyncFailureNotificationWorker stopped")
			return
		case <-ticker.C:
			processed, appErr := w.processor.ReconcileSKUSyncFailureNotifications(ctx, w.limit)
			if appErr != nil {
				w.logger.Error("SKUSyncFailureNotificationWorker failed", zap.String("code", appErr.Code), zap.String("message", appErr.Message))
				continue
			}
			if processed > 0 {
				w.logger.Info("SKUSyncFailureNotificationWorker scanned failures", zap.Int("processed", processed))
			}
		}
	}
}
