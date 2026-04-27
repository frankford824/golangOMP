package asset_center

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestDownloadAutoCleanedReturnsGone(t *testing.T) {
	now := time.Now()
	storageKey := "tasks/x/source.psd"
	svc := NewService(&fakeSearchRepo{
		current: &repo.TaskAssetSearchRow{
			Asset: &domain.TaskAsset{ID: 10, AssetID: int64Ptr(5), StorageKey: &storageKey, CleanedAt: &now},
			Task:  &domain.Task{TaskStatus: domain.TaskStatusCompleted},
		},
	}, nil, nil)
	_, appErr := svc.DownloadLatest(context.Background(), 5)
	if appErr == nil || appErr.Code != ErrCodeAssetGone {
		t.Fatalf("DownloadLatest error = %#v, want %s", appErr, ErrCodeAssetGone)
	}
}

func TestDownloadDeletedReturnsNotFound(t *testing.T) {
	now := time.Now()
	svc := NewService(&fakeSearchRepo{
		current: &repo.TaskAssetSearchRow{
			Asset: &domain.TaskAsset{ID: 10, AssetID: int64Ptr(5), DeletedAt: &now},
			Task:  &domain.Task{TaskStatus: domain.TaskStatusCompleted},
		},
	}, nil, nil)
	_, appErr := svc.DownloadLatest(context.Background(), 5)
	if appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("DownloadLatest error = %#v, want not found", appErr)
	}
}

type fakeSearchRepo struct {
	current *repo.TaskAssetSearchRow
}

func (f *fakeSearchRepo) Search(context.Context, domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	if f.current == nil {
		return nil, 0, nil
	}
	return []*repo.TaskAssetSearchRow{f.current}, 1, nil
}

func (f *fakeSearchRepo) GetCurrentByAssetID(context.Context, int64) (*repo.TaskAssetSearchRow, error) {
	return f.current, nil
}

func (f *fakeSearchRepo) ListVersionsByAssetID(context.Context, int64) ([]*repo.TaskAssetSearchRow, error) {
	if f.current == nil {
		return nil, nil
	}
	return []*repo.TaskAssetSearchRow{f.current}, nil
}

func (f *fakeSearchRepo) GetVersion(context.Context, int64, int64) (*repo.TaskAssetSearchRow, error) {
	return f.current, nil
}

func int64Ptr(v int64) *int64 { return &v }
