package asset_lifecycle

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestCleanupJobDryRunRealRunIdempotent(t *testing.T) {
	now := time.Date(2026, 4, 24, 8, 0, 0, 0, time.UTC)
	repo := &fakeLifecycleRepo{
		candidates: []*repo.TaskAssetCleanupCandidate{{
			AssetID:            11,
			VersionID:          101,
			TaskID:             20009,
			SourceTaskModuleID: int64PtrLifecycle(901),
			StorageKey:         "tasks/20009/source.psd",
			SourceModuleKey:    domain.ModuleKeyDesign,
			TaskUpdatedAt:      now.AddDate(0, 0, -400),
		}},
	}
	job := NewCleanupJob(repo, fakeTxRunner{}, nil).WithNow(func() time.Time { return now })
	dry, appErr := job.Run(context.Background(), CleanupOptions{DryRun: true})
	if appErr != nil {
		t.Fatalf("dry-run error = %v", appErr)
	}
	if dry.Scanned != 1 || dry.Cleaned != 0 || repo.marked != 0 || repo.events != 0 {
		t.Fatalf("dry-run scanned/cleaned/marked/events = %d/%d/%d/%d", dry.Scanned, dry.Cleaned, repo.marked, repo.events)
	}
	real, appErr := job.Run(context.Background(), CleanupOptions{})
	if appErr != nil {
		t.Fatalf("real run error = %v", appErr)
	}
	if real.Cleaned != 1 || repo.marked != 1 || repo.events != 1 || len(repo.enqueued) != 1 {
		t.Fatalf("real cleaned/marked/events/outbox = %d/%d/%d/%d", real.Cleaned, repo.marked, repo.events, len(repo.enqueued))
	}
	if got := repo.enqueued[0]; len(got) != 2 || got[0] != 101 || got[1] != 102 {
		t.Fatalf("outbox object ids = %v", got)
	}
	again, appErr := job.Run(context.Background(), CleanupOptions{})
	if appErr != nil {
		t.Fatalf("again error = %v", appErr)
	}
	if again.Cleaned != 0 || repo.marked != 1 || repo.events != 1 || len(repo.enqueued) != 1 {
		t.Fatalf("again cleaned/marked/events/outbox = %d/%d/%d/%d", again.Cleaned, repo.marked, repo.events, len(repo.enqueued))
	}
}

type fakeLifecycleRepo struct {
	candidates []*repo.TaskAssetCleanupCandidate
	marked     int
	events     int
	enqueued   [][]int64
}

func (f *fakeLifecycleRepo) LockGenericDeleteGuard(context.Context, repo.Tx, int64) (*repo.TaskAssetDeleteGuard, error) {
	return nil, nil
}

func (f *fakeLifecycleRepo) LockCleanupObjectIDs(_ context.Context, _ repo.Tx, versionID int64) ([]int64, error) {
	return []int64{versionID, versionID + 1}, nil
}

func (f *fakeLifecycleRepo) EnqueueObjectDeletions(_ context.Context, _ repo.Tx, ids []int64) error {
	f.enqueued = append(f.enqueued, append([]int64(nil), ids...))
	return nil
}

func (f *fakeLifecycleRepo) SoftDelete(context.Context, repo.Tx, repo.TaskAssetLifecycleUpdate) error {
	return nil
}

func (f *fakeLifecycleRepo) MarkAutoCleaned(context.Context, repo.Tx, int64, time.Time) error {
	f.marked++
	f.candidates = nil
	return nil
}

func (f *fakeLifecycleRepo) ListEligibleForCleanup(context.Context, time.Time, int) ([]*repo.TaskAssetCleanupCandidate, error) {
	return append([]*repo.TaskAssetCleanupCandidate(nil), f.candidates...), nil
}

func (f *fakeLifecycleRepo) GetCurrentForUpdate(context.Context, repo.Tx, int64) (*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

func (f *fakeLifecycleRepo) InsertLifecycleEvent(context.Context, repo.Tx, int64, domain.ModuleEventType, *int64, interface{}) error {
	f.events++
	return nil
}

type fakeTxRunner struct{}

func (fakeTxRunner) RunInTx(ctx context.Context, fn func(tx repo.Tx) error) error {
	return fn(fakeTx{})
}

type fakeTx struct{}

func (fakeTx) IsTx() {}

func int64PtrLifecycle(v int64) *int64 { return &v }
