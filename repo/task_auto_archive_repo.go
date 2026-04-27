package repo

import (
	"context"
	"time"
)

type TaskAutoArchiveRepo interface {
	// ListEligibleForArchive returns task IDs with terminal statuses older than cutoff.
	ListEligibleForArchive(ctx context.Context, cutoff time.Time, limit int) ([]int64, error)

	// ArchiveTasks updates matching Completed/Cancelled tasks to Archived and returns affected rows.
	ArchiveTasks(ctx context.Context, tx Tx, taskIDs []int64) (int, error)
}
