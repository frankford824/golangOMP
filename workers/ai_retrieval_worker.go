package workers

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

type AIRetrievalProcessor interface {
	IndexDocument(ctx context.Context, item domain.AIRetrievalOutboxItem) error
}

type AIRetrievalWorker struct {
	repository repo.AIRetrievalRepo
	tx         repo.TxRunner
	processor  AIRetrievalProcessor
	config     AsyncOutboxWorkerConfig
	now        func() time.Time
	token      func() string
	logger     *zap.Logger
}

func NewAIRetrievalWorker(repository repo.AIRetrievalRepo, tx repo.TxRunner, processor AIRetrievalProcessor, config AsyncOutboxWorkerConfig, logger *zap.Logger) *AIRetrievalWorker {
	return &AIRetrievalWorker{repository: repository, tx: tx, processor: processor, config: normalizeAsyncOutboxConfig(config), now: time.Now, token: asyncOutboxLeaseToken, logger: logger}
}

func (w *AIRetrievalWorker) Run(ctx context.Context) {
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

func (w *AIRetrievalWorker) runOnceAndLog(ctx context.Context) {
	count, err := w.RunOnce(ctx)
	if err != nil && w.logger != nil {
		w.logger.Error("ai retrieval outbox worker failed", zap.Error(err))
	} else if count > 0 && w.logger != nil {
		w.logger.Info("ai retrieval outbox worker processed", zap.Int("count", count))
	}
}

func (w *AIRetrievalWorker) RunOnce(ctx context.Context) (int, error) {
	if w == nil || w.repository == nil || w.tx == nil || w.processor == nil {
		return 0, fmt.Errorf("ai retrieval worker is not configured")
	}
	now := w.now().UTC()
	token := w.token()
	var items []domain.AIRetrievalOutboxItem
	if err := w.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		items, err = w.repository.ClaimRetrievalOutbox(ctx, tx, token, now, now.Add(w.config.LeaseTTL), w.config.Limit)
		return err
	}); err != nil {
		return 0, err
	}
	for _, item := range items {
		processErr := w.processor.IndexDocument(ctx, item)
		if processErr == nil {
			if err := w.tx.RunInTx(ctx, func(tx repo.Tx) error {
				if item.Operation == "upsert" {
					if err := w.repository.MarkRetrievalDocumentIndexed(ctx, tx, item.DocumentID, item.ContentHash, item.EmbeddingVersion, w.now().UTC()); err != nil {
						return err
					}
				}
				return w.repository.MarkRetrievalOutboxSucceeded(ctx, tx, item.ID, token, w.now().UTC())
			}); err != nil {
				return 0, err
			}
			continue
		}
		nextRetry := w.now().UTC().Add(asyncOutboxRetryDelay(w.config, item.Attempt))
		alert := item.Attempt >= w.config.AlertAfterAttempt
		if err := w.tx.RunInTx(ctx, func(tx repo.Tx) error {
			return w.repository.MarkRetrievalOutboxRetry(ctx, tx, item.ID, token, processErr.Error(), nextRetry, alert)
		}); err != nil {
			return 0, err
		}
		if w.logger != nil {
			w.logger.Warn("ai retrieval document scheduled for retry", zap.Int64("outbox_id", item.ID), zap.String("document_id", item.DocumentID), zap.Int("attempt", item.Attempt), zap.Bool("alert", alert), zap.Error(processErr))
		}
	}
	return len(items), nil
}

type AIConversationPurger interface {
	PurgeExpired(ctx context.Context, limit int) (int64, error)
}

func RunAIConversationPurgeWorker(ctx context.Context, purger AIConversationPurger, interval time.Duration, limit int, logger *zap.Logger) {
	if purger == nil {
		return
	}
	if interval <= 0 {
		interval = time.Hour
	}
	if limit <= 0 {
		limit = 200
	}
	run := func() {
		count, err := purger.PurgeExpired(ctx, limit)
		if err != nil && logger != nil {
			logger.Error("ai conversation purge failed", zap.Error(err))
		} else if count > 0 && logger != nil {
			logger.Info("ai conversations purged", zap.Int64("count", count))
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
