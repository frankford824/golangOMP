package asset_center

import (
	"context"
	"net/url"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
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

func TestDownloadLatestAppendsProxyDownloadFilename(t *testing.T) {
	storageKey := "tasks/x/source.psd"
	originalName := "交付 文件.psd"
	svc := NewService(&fakeSearchRepo{
		current: &repo.TaskAssetSearchRow{
			Asset: &domain.TaskAsset{
				ID:           10,
				AssetID:      int64Ptr(5),
				FileName:     "source.psd",
				OriginalName: &originalName,
				StorageKey:   &storageKey,
			},
			Task: &domain.Task{TaskStatus: domain.TaskStatusCompleted},
		},
	}, nil, fakeBrowserURLBuilder{baseURL: "/v1/assets/files/"})

	info, appErr := svc.DownloadLatest(context.Background(), 5)
	if appErr != nil {
		t.Fatalf("DownloadLatest error = %#v", appErr)
	}
	if info == nil || info.DownloadURL == nil {
		t.Fatalf("DownloadLatest info = %#v, want download_url", info)
	}
	parsed, err := url.Parse(*info.DownloadURL)
	if err != nil {
		t.Fatalf("download_url parse error: %v", err)
	}
	if got := parsed.Query().Get(baseservice.DownloadFilenameQueryParam); got != originalName {
		t.Fatalf("download_filename = %q, want %q", got, originalName)
	}
	if got := info.Filename; got != originalName {
		t.Fatalf("Filename = %q, want %q", got, originalName)
	}
}

type fakeBrowserURLBuilder struct {
	baseURL string
}

func (f fakeBrowserURLBuilder) BuildBrowserFileURL(storageKey string) *string {
	value := f.baseURL + storageKey
	return &value
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

func (f *fakeSearchRepo) ListCurrentByAssetIDs(context.Context, []int64) ([]*repo.TaskAssetSearchRow, error) {
	if f.current == nil {
		return []*repo.TaskAssetSearchRow{}, nil
	}
	return []*repo.TaskAssetSearchRow{f.current}, nil
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
