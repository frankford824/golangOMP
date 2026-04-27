package task_lifecycle

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"workflow/repo"
)

type fakeArchiveRepo struct {
	candidates    []int64
	archived      int
	listCutoff    time.Time
	listLimit     int
	archiveCalled bool
	archiveIDs    []int64
}

func (r *fakeArchiveRepo) ListEligibleForArchive(_ context.Context, cutoff time.Time, limit int) ([]int64, error) {
	r.listCutoff = cutoff
	r.listLimit = limit
	return append([]int64{}, r.candidates...), nil
}

func (r *fakeArchiveRepo) ArchiveTasks(_ context.Context, _ repo.Tx, taskIDs []int64) (int, error) {
	r.archiveCalled = true
	r.archiveIDs = append([]int64{}, taskIDs...)
	return r.archived, nil
}

type fakeTx struct{}

func (fakeTx) IsTx() {}

type fakeTxRunner struct{}

func (fakeTxRunner) RunInTx(ctx context.Context, fn func(tx repo.Tx) error) error {
	if ctx == nil {
		return errors.New("nil context")
	}
	return fn(fakeTx{})
}

func TestAutoArchive_DryRun(t *testing.T) {
	fakeRepo := &fakeArchiveRepo{candidates: []int64{1, 2, 3, 4, 5}, archived: 5}
	result, appErr := NewAutoArchiveJob(fakeRepo, fakeTxRunner{}, nil).Run(context.Background(), AutoArchiveOptions{DryRun: true})
	if appErr != nil {
		t.Fatalf("Run dry-run: %v", appErr)
	}
	if result.Scanned != 5 || result.Archived != 0 {
		t.Fatalf("scanned/archived = %d/%d, want 5/0", result.Scanned, result.Archived)
	}
	if fakeRepo.archiveCalled {
		t.Fatal("ArchiveTasks called during dry-run")
	}
}

func TestAutoArchive_RealRun(t *testing.T) {
	fakeRepo := &fakeArchiveRepo{candidates: []int64{1, 2, 3, 4, 5}, archived: 5}
	result, appErr := NewAutoArchiveJob(fakeRepo, fakeTxRunner{}, nil).Run(context.Background(), AutoArchiveOptions{})
	if appErr != nil {
		t.Fatalf("Run real: %v", appErr)
	}
	if result.Scanned != 5 || result.Archived != 5 {
		t.Fatalf("scanned/archived = %d/%d, want 5/5", result.Scanned, result.Archived)
	}
	if !reflect.DeepEqual(fakeRepo.archiveIDs, []int64{1, 2, 3, 4, 5}) {
		t.Fatalf("archive IDs = %#v", fakeRepo.archiveIDs)
	}
}

func TestAutoArchive_Idempotent_NoCandidates(t *testing.T) {
	fakeRepo := &fakeArchiveRepo{}
	result, appErr := NewAutoArchiveJob(fakeRepo, fakeTxRunner{}, nil).Run(context.Background(), AutoArchiveOptions{})
	if appErr != nil {
		t.Fatalf("Run no candidates: %v", appErr)
	}
	if result.Scanned != 0 || result.Archived != 0 {
		t.Fatalf("scanned/archived = %d/%d, want 0/0", result.Scanned, result.Archived)
	}
	if fakeRepo.archiveCalled {
		t.Fatal("ArchiveTasks called with no candidates")
	}
}

func TestAutoArchive_DefaultCutoffDays(t *testing.T) {
	fixed := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	fakeRepo := &fakeArchiveRepo{}
	_, appErr := NewAutoArchiveJob(fakeRepo, fakeTxRunner{}, nil).WithNow(func() time.Time { return fixed }).Run(context.Background(), AutoArchiveOptions{})
	if appErr != nil {
		t.Fatalf("Run default cutoff: %v", appErr)
	}
	want := fixed.AddDate(0, 0, -90)
	if !fakeRepo.listCutoff.Equal(want) {
		t.Fatalf("cutoff = %s, want %s", fakeRepo.listCutoff, want)
	}
}

func TestAutoArchive_CutoffDaysOverride(t *testing.T) {
	fixed := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	fakeRepo := &fakeArchiveRepo{}
	_, appErr := NewAutoArchiveJob(fakeRepo, fakeTxRunner{}, nil).WithNow(func() time.Time { return fixed }).Run(context.Background(), AutoArchiveOptions{CutoffDays: 30})
	if appErr != nil {
		t.Fatalf("Run override cutoff: %v", appErr)
	}
	want := fixed.AddDate(0, 0, -30)
	if !fakeRepo.listCutoff.Equal(want) {
		t.Fatalf("cutoff = %s, want %s", fakeRepo.listCutoff, want)
	}
}

func TestAutoArchive_LimitDefault(t *testing.T) {
	fakeRepo := &fakeArchiveRepo{}
	_, appErr := NewAutoArchiveJob(fakeRepo, fakeTxRunner{}, nil).Run(context.Background(), AutoArchiveOptions{})
	if appErr != nil {
		t.Fatalf("Run default limit: %v", appErr)
	}
	if fakeRepo.listLimit != 1000 {
		t.Fatalf("limit = %d, want 1000", fakeRepo.listLimit)
	}
}
