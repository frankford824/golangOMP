package asset_lifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type ObjectDeletionWorkerConfig struct {
	LeaseTTL          time.Duration
	RetryBase         time.Duration
	RetryMax          time.Duration
	AlertAfterAttempt int
}

type ObjectDeletionWorkerResult struct {
	Claimed   int
	Succeeded int
	Retained  int
	Retried   int
	Alerted   int
}

type ObjectDeletionWorker struct {
	repository repo.AssetObjectDeletionOutboxRepo
	txRunner   repo.TxRunner
	deleter    ObjectDeleter
	config     ObjectDeletionWorkerConfig
	now        func() time.Time
	leaseToken func() string
	logger     *log.Logger
}

func NewObjectDeletionWorker(repository repo.AssetObjectDeletionOutboxRepo, txRunner repo.TxRunner, deleter ObjectDeleter, config ObjectDeletionWorkerConfig, logger *log.Logger) *ObjectDeletionWorker {
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
	return &ObjectDeletionWorker{
		repository: repository,
		txRunner:   txRunner,
		deleter:    deleter,
		config:     config,
		now:        time.Now,
		leaseToken: newObjectDeletionLeaseToken,
		logger:     logger,
	}
}

func (w *ObjectDeletionWorker) Enabled() bool {
	return w != nil && w.repository != nil && w.txRunner != nil
}

func (w *ObjectDeletionWorker) WithNow(now func() time.Time) *ObjectDeletionWorker {
	if now != nil {
		w.now = now
	}
	return w
}

func (w *ObjectDeletionWorker) WithLeaseTokenGenerator(generator func() string) *ObjectDeletionWorker {
	if generator != nil {
		w.leaseToken = generator
	}
	return w
}

func (w *ObjectDeletionWorker) RunOnce(ctx context.Context, limit int) (*ObjectDeletionWorkerResult, *domain.AppError) {
	result := &ObjectDeletionWorkerResult{}
	if !w.Enabled() {
		return result, domain.NewAppError(domain.ErrCodeInternalError, "asset object deletion worker is not configured", nil)
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	now := w.now().UTC()
	leaseToken := strings.TrimSpace(w.leaseToken())
	if leaseToken == "" {
		return result, domain.NewAppError(domain.ErrCodeInternalError, "asset object deletion lease token is empty", nil)
	}
	var items []repo.AssetObjectDeletionOutboxItem
	if err := w.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		items, err = w.repository.ClaimObjectDeletions(ctx, tx, leaseToken, now, now.Add(w.config.LeaseTTL), limit)
		return err
	}); err != nil {
		return result, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	result.Claimed = len(items)
	for _, item := range items {
		if item.RetainPhysicalObject {
			result.Retained++
			w.logf("object deletion retained shared object id=%d task_asset_id=%v storage_key=%s", item.ID, item.TaskAssetID, item.StorageKey)
		}
		deleteErr, forceAlert := w.deleteObject(ctx, item)
		if deleteErr == nil || objectDeletionNotFound(w.deleter, deleteErr) {
			if err := w.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
				return w.repository.MarkObjectDeletionSucceeded(ctx, tx, item, leaseToken, w.now().UTC())
			}); err != nil {
				return result, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
			}
			result.Succeeded++
			continue
		}

		alert := forceAlert || item.Attempt >= w.config.AlertAfterAttempt
		nextRetry := w.now().UTC().Add(w.retryDelay(item.Attempt))
		lastError := truncateObjectDeletionError(deleteErr.Error())
		if err := w.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			return w.repository.MarkObjectDeletionRetry(ctx, tx, item, leaseToken, lastError, nextRetry, alert)
		}); err != nil {
			return result, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
		}
		result.Retried++
		if alert {
			result.Alerted++
		}
		w.logf("object deletion retry id=%d task_asset_id=%v attempt=%d alert=%t next_retry_at=%s error=%s", item.ID, item.TaskAssetID, item.Attempt, alert, nextRetry.Format(time.RFC3339), lastError)
	}
	return result, nil
}

func (w *ObjectDeletionWorker) deleteObject(ctx context.Context, item repo.AssetObjectDeletionOutboxItem) (error, bool) {
	if item.RetainPhysicalObject {
		return nil, false
	}
	switch item.StorageAdapter {
	case domain.AssetStorageAdapterOSSUploadService:
		if item.StorageIsPlaceholder {
			return nil, false
		}
		if w.deleter == nil || !w.deleter.Enabled() {
			return fmt.Errorf("storage adapter %q deletion backend is unavailable", item.StorageAdapter), true
		}
		return w.deleter.DeleteObject(ctx, item.StorageKey), false
	case domain.AssetStorageAdapterPlaceholderStorage,
		domain.AssetStorageAdapterMockUpload,
		domain.AssetStorageAdapterExportPlaceholder:
		// These adapters never own a physical object. Completing the outbox row
		// is the deletion operation and must not call the OSS backend.
		return nil, false
	default:
		// Unknown adapters are deliberately not guessed. In particular, a NAS or
		// upload-service key must never be sent to OSS merely because it looks
		// like an object key.
		return fmt.Errorf("unsupported storage adapter %q; refusing physical deletion", item.StorageAdapter), true
	}
}

func (w *ObjectDeletionWorker) retryDelay(attempt int) time.Duration {
	exponent := attempt - 1
	if exponent < 0 {
		exponent = 0
	}
	if exponent > 16 {
		exponent = 16
	}
	delay := w.config.RetryBase * time.Duration(1<<exponent)
	if delay > w.config.RetryMax {
		return w.config.RetryMax
	}
	return delay
}

func (w *ObjectDeletionWorker) logf(format string, args ...interface{}) {
	if w.logger != nil {
		w.logger.Printf(format, args...)
	}
}

type objectDeleteNotFoundClassifier interface {
	IsObjectNotFound(error) bool
}

func objectDeletionNotFound(deleter ObjectDeleter, err error) bool {
	if err == nil {
		return false
	}
	if classifier, ok := deleter.(objectDeleteNotFoundClassifier); ok && classifier.IsObjectNotFound(err) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "status=404") || strings.Contains(message, "no such key") || strings.Contains(message, "nosuchkey")
}

func truncateObjectDeletionError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= 4000 {
		return message
	}
	return message[:4000]
}

func newObjectDeletionLeaseToken() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(raw)
}
