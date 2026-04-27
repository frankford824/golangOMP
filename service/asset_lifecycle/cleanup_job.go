package asset_lifecycle

import (
	"context"
	"log"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const CleanupLogPrefix = "[ASSET-CLEANUP]"

type CleanupJob struct {
	lifecycleRepo repo.TaskAssetLifecycleRepo
	txRunner      repo.TxRunner
	deleter       ObjectDeleter
	now           func() time.Time
	logger        *log.Logger
}

type CleanupOptions struct {
	DryRun bool
	Limit  int
}

type CleanupResult struct {
	DryRun     bool
	Scanned    int
	Cleaned    int
	Candidates []*repo.TaskAssetCleanupCandidate
}

func NewCleanupJob(lifecycleRepo repo.TaskAssetLifecycleRepo, txRunner repo.TxRunner, deleter ObjectDeleter, logger *log.Logger) *CleanupJob {
	return &CleanupJob{lifecycleRepo: lifecycleRepo, txRunner: txRunner, deleter: deleter, logger: logger, now: time.Now}
}

func (j *CleanupJob) WithNow(now func() time.Time) *CleanupJob {
	if now != nil {
		j.now = now
	}
	return j
}

func (j *CleanupJob) Run(ctx context.Context, opts CleanupOptions) (*CleanupResult, *domain.AppError) {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	cutoff := j.now().UTC().AddDate(0, 0, -365)
	candidates, err := j.lifecycleRepo.ListEligibleForCleanup(ctx, cutoff, opts.Limit)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	result := &CleanupResult{DryRun: opts.DryRun, Scanned: len(candidates), Candidates: candidates}
	j.logf("dry_run=%t scanned=%d cutoff=%s", opts.DryRun, len(candidates), cutoff.Format(time.RFC3339))
	if opts.DryRun {
		return result, nil
	}
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if j.deleter != nil && j.deleter.Enabled() && candidate.StorageKey != "" {
			if err := j.deleter.DeleteObject(ctx, candidate.StorageKey); err != nil {
				return result, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
			}
		}
		err := j.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			if err := j.lifecycleRepo.MarkAutoCleaned(ctx, tx, candidate.VersionID, j.now().UTC()); err != nil {
				return err
			}
			if candidate.SourceTaskModuleID == nil || *candidate.SourceTaskModuleID <= 0 {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "cleanup candidate missing source_task_module_id", map[string]interface{}{"version_id": candidate.VersionID})
			}
			return j.lifecycleRepo.InsertLifecycleEvent(ctx, tx, *candidate.SourceTaskModuleID, domain.ModuleEventType("asset_auto_cleaned"), nil, map[string]interface{}{
				"asset_id":             candidate.AssetID,
				"version_id":           candidate.VersionID,
				"task_id":              candidate.TaskID,
				"source_module_key":    candidate.SourceModuleKey,
				"original_storage_key": candidate.StorageKey,
				"task_updated_at":      candidate.TaskUpdatedAt,
			})
		})
		if err != nil {
			return result, toAppError(err)
		}
		result.Cleaned++
	}
	j.logf("cleaned=%d", result.Cleaned)
	return result, nil
}

func (j *CleanupJob) logf(format string, args ...interface{}) {
	if j.logger != nil {
		j.logger.Printf(CleanupLogPrefix+" "+format, args...)
	}
}
