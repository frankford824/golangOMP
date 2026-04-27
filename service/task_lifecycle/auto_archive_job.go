package task_lifecycle

import (
	"context"
	"log"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const AutoArchiveLogPrefix = "[TASK-AUTO-ARCHIVE]"

type AutoArchiveJob struct {
	archiveRepo repo.TaskAutoArchiveRepo
	txRunner    repo.TxRunner
	now         func() time.Time
	logger      *log.Logger
}

type AutoArchiveOptions struct {
	DryRun     bool
	Limit      int
	CutoffDays int
}

type AutoArchiveResult struct {
	DryRun     bool
	Scanned    int
	Archived   int
	Candidates []int64
	Cutoff     time.Time
}

func NewAutoArchiveJob(archiveRepo repo.TaskAutoArchiveRepo, txRunner repo.TxRunner, logger *log.Logger) *AutoArchiveJob {
	return &AutoArchiveJob{archiveRepo: archiveRepo, txRunner: txRunner, logger: logger, now: time.Now}
}

func (j *AutoArchiveJob) WithNow(now func() time.Time) *AutoArchiveJob {
	if now != nil {
		j.now = now
	}
	return j
}

func (j *AutoArchiveJob) Run(ctx context.Context, opts AutoArchiveOptions) (*AutoArchiveResult, *domain.AppError) {
	if opts.Limit <= 0 {
		opts.Limit = 1000
	}
	if opts.CutoffDays <= 0 {
		opts.CutoffDays = 90
	}
	cutoff := j.now().UTC().AddDate(0, 0, -opts.CutoffDays)
	candidates, err := j.archiveRepo.ListEligibleForArchive(ctx, cutoff, opts.Limit)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	result := &AutoArchiveResult{DryRun: opts.DryRun, Scanned: len(candidates), Candidates: candidates, Cutoff: cutoff}
	j.logf("dry_run=%t scanned=%d cutoff=%s limit=%d", opts.DryRun, len(candidates), cutoff.Format(time.RFC3339), opts.Limit)
	if opts.DryRun || len(candidates) == 0 {
		return result, nil
	}
	err = j.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		archived, err := j.archiveRepo.ArchiveTasks(ctx, tx, candidates)
		if err != nil {
			return err
		}
		result.Archived = archived
		return nil
	})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	j.logf("archived=%d", result.Archived)
	return result, nil
}

func (j *AutoArchiveJob) logf(format string, args ...interface{}) {
	if j.logger != nil {
		j.logger.Printf(AutoArchiveLogPrefix+" "+format, args...)
	}
}
