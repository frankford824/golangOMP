package workers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"workflow/repo"
	"workflow/service"
)

type AsyncOutboxWorkerConfig struct {
	Interval          time.Duration
	LeaseTTL          time.Duration
	RetryBase         time.Duration
	RetryMax          time.Duration
	AlertAfterAttempt int
	Limit             int
}

func normalizeAsyncOutboxConfig(config AsyncOutboxWorkerConfig) AsyncOutboxWorkerConfig {
	if config.Interval <= 0 {
		config.Interval = 5 * time.Second
	}
	if config.LeaseTTL <= 0 {
		config.LeaseTTL = 2 * time.Minute
	}
	if config.RetryBase <= 0 {
		config.RetryBase = 30 * time.Second
	}
	if config.RetryMax <= 0 {
		config.RetryMax = 6 * time.Hour
	}
	if config.AlertAfterAttempt <= 0 {
		config.AlertAfterAttempt = 5
	}
	if config.Limit <= 0 || config.Limit > 500 {
		config.Limit = 50
	}
	return config
}

type TaskERPOutboxWorker struct {
	repository repo.AsyncProjectionOutboxRepo
	txRunner   repo.TxRunner
	processor  service.TaskERPOutboxProcessor
	config     AsyncOutboxWorkerConfig
	now        func() time.Time
	token      func() string
	logger     *zap.Logger
}

func NewTaskERPOutboxWorker(repository repo.AsyncProjectionOutboxRepo, txRunner repo.TxRunner, processor service.TaskERPOutboxProcessor, config AsyncOutboxWorkerConfig, logger *zap.Logger) *TaskERPOutboxWorker {
	return &TaskERPOutboxWorker{repository: repository, txRunner: txRunner, processor: processor, config: normalizeAsyncOutboxConfig(config), now: time.Now, token: asyncOutboxLeaseToken, logger: logger}
}

func (w *TaskERPOutboxWorker) Run(ctx context.Context) {
	w.runOnceAndLog(ctx)
	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnceAndLog(ctx)
		}
	}
}

func (w *TaskERPOutboxWorker) runOnceAndLog(ctx context.Context) {
	processed, err := w.RunOnce(ctx)
	if err != nil && w.logger != nil {
		w.logger.Error("task ERP outbox worker failed", zap.Error(err))
	} else if processed > 0 && w.logger != nil {
		w.logger.Info("task ERP outbox worker processed", zap.Int("count", processed))
	}
}

func (w *TaskERPOutboxWorker) RunOnce(ctx context.Context) (int, error) {
	if w == nil || w.repository == nil || w.txRunner == nil || w.processor == nil {
		return 0, fmt.Errorf("task ERP outbox worker is not configured")
	}
	now := w.now().UTC()
	leaseToken := strings.TrimSpace(w.token())
	if leaseToken == "" {
		return 0, fmt.Errorf("task ERP outbox lease token is empty")
	}
	var items []repo.TaskERPOutboxItem
	if err := w.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		items, err = w.repository.ClaimTaskERPOutbox(ctx, tx, leaseToken, now, now.Add(w.config.LeaseTTL), w.config.Limit)
		return err
	}); err != nil {
		return 0, err
	}
	for _, item := range items {
		processErr := w.processor.ProcessTaskERPOutbox(ctx, item)
		if processErr == nil {
			if err := w.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
				return w.repository.MarkTaskERPOutboxSucceeded(ctx, tx, item.ID, leaseToken)
			}); err != nil {
				return 0, err
			}
			continue
		}
		nextRetry := w.now().UTC().Add(asyncOutboxRetryDelay(w.config, item.Attempt))
		alert := item.Attempt >= w.config.AlertAfterAttempt
		if err := w.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			return w.repository.MarkTaskERPOutboxRetry(ctx, tx, item.ID, leaseToken, processErr.Error(), nextRetry, alert)
		}); err != nil {
			return 0, err
		}
		if w.logger != nil {
			w.logger.Warn("task ERP outbox item scheduled for retry", zap.Int64("outbox_id", item.ID), zap.String("job_type", item.JobType), zap.Int("attempt", item.Attempt), zap.Bool("alert", alert), zap.Error(processErr))
		}
	}
	return len(items), nil
}

type SearchReindexOutboxWorker struct {
	repository repo.AsyncProjectionOutboxRepo
	txRunner   repo.TxRunner
	config     AsyncOutboxWorkerConfig
	now        func() time.Time
	token      func() string
	logger     *zap.Logger
}

func NewSearchReindexOutboxWorker(repository repo.AsyncProjectionOutboxRepo, txRunner repo.TxRunner, config AsyncOutboxWorkerConfig, logger *zap.Logger) *SearchReindexOutboxWorker {
	config = normalizeAsyncOutboxConfig(config)
	if config.Limit < 100 {
		config.Limit = 100
	}
	return &SearchReindexOutboxWorker{repository: repository, txRunner: txRunner, config: config, now: time.Now, token: asyncOutboxLeaseToken, logger: logger}
}

func (w *SearchReindexOutboxWorker) Run(ctx context.Context) {
	w.runOnceAndLog(ctx)
	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnceAndLog(ctx)
		}
	}
}

func (w *SearchReindexOutboxWorker) runOnceAndLog(ctx context.Context) {
	processed, err := w.RunOnce(ctx)
	if err != nil && w.logger != nil {
		w.logger.Error("search reindex outbox worker failed", zap.Error(err))
	} else if processed > 0 && w.logger != nil {
		w.logger.Info("search reindex outbox worker processed", zap.Int("count", processed))
	}
}

func (w *SearchReindexOutboxWorker) RunOnce(ctx context.Context) (int, error) {
	if w == nil || w.repository == nil || w.txRunner == nil {
		return 0, fmt.Errorf("search reindex outbox worker is not configured")
	}
	now := w.now().UTC()
	leaseToken := strings.TrimSpace(w.token())
	if leaseToken == "" {
		return 0, fmt.Errorf("search reindex outbox lease token is empty")
	}
	var items []repo.SearchReindexOutboxItem
	if err := w.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		items, err = w.repository.ClaimSearchReindexOutbox(ctx, tx, leaseToken, now, now.Add(w.config.LeaseTTL), w.config.Limit)
		return err
	}); err != nil {
		return 0, err
	}
	for _, item := range items {
		applyErr := w.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			if err := w.repository.ApplySearchReindex(ctx, tx, item); err != nil {
				return err
			}
			return w.repository.MarkSearchReindexOutboxSucceeded(ctx, tx, item.ID, leaseToken)
		})
		if applyErr == nil {
			continue
		}
		nextRetry := w.now().UTC().Add(asyncOutboxRetryDelay(w.config, item.Attempt))
		if err := w.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			return w.repository.MarkSearchReindexOutboxRetry(ctx, tx, item.ID, leaseToken, applyErr.Error(), nextRetry)
		}); err != nil {
			return 0, err
		}
		if w.logger != nil {
			w.logger.Warn("search reindex outbox item scheduled for retry", zap.Int64("outbox_id", item.ID), zap.String("entity_type", item.EntityType), zap.Int("attempt", item.Attempt), zap.Error(applyErr))
		}
	}
	return len(items), nil
}

func asyncOutboxRetryDelay(config AsyncOutboxWorkerConfig, attempt int) time.Duration {
	exponent := attempt - 1
	if exponent < 0 {
		exponent = 0
	}
	if exponent > 16 {
		exponent = 16
	}
	delay := config.RetryBase * time.Duration(1<<exponent)
	if delay > config.RetryMax {
		return config.RetryMax
	}
	return delay
}

func asyncOutboxLeaseToken() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(raw)
}
