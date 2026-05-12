package asset_center

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestBuildBatchDownloadZipSuccessWithTaskPathAndFilenameFallback(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	repoRows := []*repo.TaskAssetSearchRow{
		{
			Asset: &domain.TaskAsset{
				ID:           11,
				AssetID:      int64Ptr(101),
				TaskID:       9001,
				FileName:     "fallback.psd",
				StorageKey:   strPtr("k-101"),
				FileSize:     int64Ptr(4),
				UploadStatus: &uploaded,
				OriginalName: strPtr("原始文件.psd"),
			},
			Task: &domain.Task{ID: 9001},
		},
		{
			Asset: &domain.TaskAsset{
				ID:           12,
				AssetID:      int64Ptr(102),
				TaskID:       9001,
				FileName:     "fallback2.psd",
				StorageKey:   strPtr("k-102"),
				FileSize:     int64Ptr(4),
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{ID: 9001},
		},
	}
	svc := NewService(&batchRepoStub{rowsByIDs: repoRows}, nil, nil)
	svc.SetStorageStreamOpener(&streamOpenerStub{contentByKey: map[string]string{
		"k-101": "A101",
		"k-102": "A102",
	}})

	result, appErr := svc.BuildBatchDownloadZip(context.Background(), []int64{101, 102})
	if appErr != nil {
		t.Fatalf("BuildBatchDownloadZip error = %+v", appErr)
	}
	if result.SuccessCount != 2 || result.FailureCount != 0 {
		t.Fatalf("result counts = %+v", result)
	}
	entries := unzipEntries(t, result.ZipBytes)
	if _, ok := entries["task-9001/原始文件.psd"]; !ok {
		t.Fatalf("missing original filename entry, got keys=%v", mapKeys(entries))
	}
	if _, ok := entries["task-9001/fallback2.psd"]; !ok {
		t.Fatalf("missing fallback filename entry, got keys=%v", mapKeys(entries))
	}
}

func TestBuildBatchDownloadZipDuplicateNamesAddSuffix(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	repoRows := []*repo.TaskAssetSearchRow{
		{
			Asset: &domain.TaskAsset{
				ID:           21,
				AssetID:      int64Ptr(201),
				TaskID:       9100,
				FileName:     "same.psd",
				StorageKey:   strPtr("k-201"),
				FileSize:     int64Ptr(1),
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{ID: 9100},
		},
		{
			Asset: &domain.TaskAsset{
				ID:           22,
				AssetID:      int64Ptr(202),
				TaskID:       9100,
				FileName:     "same.psd",
				StorageKey:   strPtr("k-202"),
				FileSize:     int64Ptr(1),
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{ID: 9100},
		},
	}
	svc := NewService(&batchRepoStub{rowsByIDs: repoRows}, nil, nil)
	svc.SetStorageStreamOpener(&streamOpenerStub{contentByKey: map[string]string{
		"k-201": "x",
		"k-202": "y",
	}})

	result, appErr := svc.BuildBatchDownloadZip(context.Background(), []int64{201, 202})
	if appErr != nil {
		t.Fatalf("BuildBatchDownloadZip error = %+v", appErr)
	}
	entries := unzipEntries(t, result.ZipBytes)
	if _, ok := entries["task-9100/same.psd"]; !ok {
		t.Fatalf("missing first duplicate entry")
	}
	if _, ok := entries["task-9100/same (2).psd"]; !ok {
		t.Fatalf("missing second duplicate entry")
	}
}

func TestBuildBatchDownloadZipPartialFailureWritesDownloadErrors(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	repoRows := []*repo.TaskAssetSearchRow{
		{
			Asset: &domain.TaskAsset{
				ID:           31,
				AssetID:      int64Ptr(301),
				TaskID:       9200,
				FileName:     "ok.psd",
				StorageKey:   strPtr("k-301"),
				FileSize:     int64Ptr(2),
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{ID: 9200},
		},
		{
			Asset: &domain.TaskAsset{
				ID:           32,
				AssetID:      int64Ptr(302),
				TaskID:       9200,
				FileName:     "bad.psd",
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{ID: 9200},
		},
	}
	svc := NewService(&batchRepoStub{rowsByIDs: repoRows}, nil, nil)
	svc.SetStorageStreamOpener(&streamOpenerStub{contentByKey: map[string]string{
		"k-301": "ok",
	}})

	result, appErr := svc.BuildBatchDownloadZip(context.Background(), []int64{301, 302})
	if appErr != nil {
		t.Fatalf("BuildBatchDownloadZip error = %+v", appErr)
	}
	if result.SuccessCount != 1 || result.FailureCount != 1 {
		t.Fatalf("result counts = %+v", result)
	}
	entries := unzipEntries(t, result.ZipBytes)
	errFile := entries["download_errors.txt"]
	if !strings.Contains(errFile, "asset_id=302") || !strings.Contains(errFile, "reason=missing_storage_key") {
		t.Fatalf("download_errors.txt = %q", errFile)
	}
}

func TestBuildBatchDownloadZipAllFailedReturnsError(t *testing.T) {
	repoRows := []*repo.TaskAssetSearchRow{
		{
			Asset: &domain.TaskAsset{
				ID:       41,
				AssetID:  int64Ptr(401),
				TaskID:   9300,
				FileName: "x.psd",
			},
			Task: &domain.Task{ID: 9300},
		},
	}
	svc := NewService(&batchRepoStub{rowsByIDs: repoRows}, nil, nil)
	svc.SetStorageStreamOpener(&streamOpenerStub{})

	result, appErr := svc.BuildBatchDownloadZip(context.Background(), []int64{401})
	if appErr == nil {
		t.Fatalf("expected error, result=%+v", result)
	}
	if appErr.Code != domain.ErrCodeAssetMissing {
		t.Fatalf("error code = %s, want %s", appErr.Code, domain.ErrCodeAssetMissing)
	}
}

func TestBuildBatchDownloadZipAssetCountLimit(t *testing.T) {
	svc := NewService(&batchRepoStub{}, nil, nil)
	svc.SetStorageStreamOpener(&streamOpenerStub{})
	assetIDs := make([]int64, MaxBatchDownloadAssets+1)
	for i := range assetIDs {
		assetIDs[i] = int64(i + 1)
	}

	result, appErr := svc.BuildBatchDownloadZip(context.Background(), assetIDs)
	if appErr == nil {
		t.Fatalf("expected limit error, result=%+v", result)
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
}

type batchRepoStub struct {
	rowsByIDs []*repo.TaskAssetSearchRow
}

func (b *batchRepoStub) Search(context.Context, domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	return nil, 0, nil
}

func (b *batchRepoStub) GetCurrentByAssetID(context.Context, int64) (*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

func (b *batchRepoStub) ListCurrentByAssetIDs(_ context.Context, assetIDs []int64) ([]*repo.TaskAssetSearchRow, error) {
	if len(assetIDs) == 0 {
		return []*repo.TaskAssetSearchRow{}, nil
	}
	return b.rowsByIDs, nil
}

func (b *batchRepoStub) ListVersionsByAssetID(context.Context, int64) ([]*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

func (b *batchRepoStub) GetVersion(context.Context, int64, int64) (*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

type streamOpenerStub struct {
	contentByKey map[string]string
	errByKey     map[string]error
}

func (s *streamOpenerStub) Open(_ context.Context, storageKey string) (io.ReadCloser, error) {
	if err := s.errByKey[storageKey]; err != nil {
		return nil, err
	}
	content, ok := s.contentByKey[storageKey]
	if !ok {
		return nil, errors.New("missing stream content")
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func unzipEntries(t *testing.T, raw []byte) map[string]string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("zip reader error: %v", err)
	}
	out := map[string]string{}
	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("zip open file error: %v", err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("zip read file error: %v", err)
		}
		out[f.Name] = string(body)
	}
	return out
}

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func strPtr(v string) *string { return &v }
