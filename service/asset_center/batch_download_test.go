package asset_center

import (
	"context"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
)

func TestBuildBatchDownloadManifestReturnsDirectOSSURLs(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	expiresAt := time.Date(2026, 5, 12, 10, 30, 0, 0, time.UTC)
	repoRows := []*repo.TaskAssetSearchRow{
		{
			Asset: &domain.TaskAsset{
				ID:           11,
				AssetID:      int64PtrBatchSvc(101),
				TaskID:       9001,
				FileName:     "fallback.psd",
				StorageKey:   strPtr("k-101"),
				FileSize:     int64PtrBatchSvc(4),
				MimeType:     strPtr("image/vnd.adobe.photoshop"),
				UploadStatus: &uploaded,
				OriginalName: strPtr("原始文件.psd"),
			},
			Task: &domain.Task{ID: 9001},
		},
		{
			Asset: &domain.TaskAsset{
				ID:           12,
				AssetID:      int64PtrBatchSvc(102),
				TaskID:       9001,
				FileName:     "fallback2.psd",
				StorageKey:   strPtr("k-102"),
				FileSize:     int64PtrBatchSvc(5),
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{ID: 9001},
		},
	}
	presigner := &batchPresignerStub{
		enabled:   true,
		expiresAt: expiresAt,
		urlByKey: map[string]string{
			"k-101": "https://oss.example/k-101",
			"k-102": "https://oss.example/k-102",
		},
	}
	svc := NewService(&batchRepoStub{rowsByIDs: repoRows}, presigner, nil)

	result, appErr := svc.BuildBatchDownloadManifest(context.Background(), []int64{101, 102})
	if appErr != nil {
		t.Fatalf("BuildBatchDownloadManifest error = %+v", appErr)
	}
	if result.SuccessCount != 2 || result.FailureCount != 0 || result.TotalSize != 9 {
		t.Fatalf("manifest summary = %+v", result)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items len = %d", len(result.Items))
	}
	if result.Items[0].Filename != "原始文件.psd" || result.Items[0].DownloadURL != "https://oss.example/k-101" {
		t.Fatalf("first item = %+v", result.Items[0])
	}
	if result.Items[1].Filename != "fallback2.psd" || result.Items[1].DownloadURL != "https://oss.example/k-102" {
		t.Fatalf("second item = %+v", result.Items[1])
	}
	if got := presigner.filenameByKey["k-101"]; got != "原始文件.psd" {
		t.Fatalf("presigned filename = %q", got)
	}
	if result.ExpiresAt == nil || !result.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("manifest expires_at = %v", result.ExpiresAt)
	}
}

func TestBuildBatchDownloadManifestDuplicateNamesAddSuffix(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	repoRows := []*repo.TaskAssetSearchRow{
		{
			Asset: &domain.TaskAsset{
				ID:           21,
				AssetID:      int64PtrBatchSvc(201),
				TaskID:       9100,
				FileName:     "same.psd",
				StorageKey:   strPtr("k-201"),
				FileSize:     int64PtrBatchSvc(1),
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{ID: 9100},
		},
		{
			Asset: &domain.TaskAsset{
				ID:           22,
				AssetID:      int64PtrBatchSvc(202),
				TaskID:       9100,
				FileName:     "same.psd",
				StorageKey:   strPtr("k-202"),
				FileSize:     int64PtrBatchSvc(1),
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{ID: 9100},
		},
	}
	svc := NewService(&batchRepoStub{rowsByIDs: repoRows}, &batchPresignerStub{
		enabled: true,
		urlByKey: map[string]string{
			"k-201": "https://oss.example/k-201",
			"k-202": "https://oss.example/k-202",
		},
	}, nil)

	result, appErr := svc.BuildBatchDownloadManifest(context.Background(), []int64{201, 202})
	if appErr != nil {
		t.Fatalf("BuildBatchDownloadManifest error = %+v", appErr)
	}
	if result.Items[0].Filename != "same.psd" || result.Items[1].Filename != "same (2).psd" {
		t.Fatalf("filenames = %+v", result.Items)
	}
}

func TestBuildBatchDownloadManifestBusinessNamingModeUsesSKUAndProductName(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	scopeSKU := "NSKT000277"
	repoRows := []*repo.TaskAssetSearchRow{
		{
			Asset: &domain.TaskAsset{
				ID:           25,
				AssetID:      int64PtrBatchSvc(205),
				TaskID:       9105,
				ScopeSKUCode: &scopeSKU,
				AssetType:    domain.TaskAssetTypeDelivery,
				FileName:     "opaque-storage-name.jpg",
				OriginalName: strPtr("原始稿.jpg"),
				StorageKey:   strPtr("k-205"),
				FileSize:     int64PtrBatchSvc(1),
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{
				ID:                  9105,
				SKUCode:             "TASK-SKU",
				ProductNameSnapshot: "端午节保龄球/10.5x15.5cm",
			},
		},
	}
	svc := NewService(&batchRepoStub{rowsByIDs: repoRows}, &batchPresignerStub{
		enabled:  true,
		urlByKey: map[string]string{"k-205": "https://oss.example/k-205"},
	}, nil)

	if got := resolveBatchFilenameForMode(repoRows[0], 205, BatchDownloadNamingBusiness); got != "NSKT000277-端午节保龄球_10.5x15.5cm.jpg" {
		t.Fatalf("resolveBatchFilenameForMode() = %q, want business filename", got)
	}

	result, appErr := svc.BuildBatchDownloadManifest(
		context.Background(),
		[]int64{205},
		WithBatchDownloadNamingMode(BatchDownloadNamingBusiness),
	)
	if appErr != nil {
		t.Fatalf("BuildBatchDownloadManifest error = %+v", appErr)
	}
	if result.Items[0].Filename != "NSKT000277-端午节保龄球_10.5x15.5cm.jpg" {
		t.Fatalf("filename = %q", result.Items[0].Filename)
	}
}

func TestBuildBatchDownloadManifestBusinessNamingModeKeepsReferenceOriginalName(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	scopeSKU := "NSKT000277"
	repoRows := []*repo.TaskAssetSearchRow{
		{
			Asset: &domain.TaskAsset{
				ID:           26,
				AssetID:      int64PtrBatchSvc(206),
				TaskID:       9106,
				ScopeSKUCode: &scopeSKU,
				AssetType:    domain.TaskAssetTypeReference,
				FileName:     "stored-reference.jpg",
				OriginalName: strPtr("运营参考原图.jpg"),
				StorageKey:   strPtr("k-206"),
				FileSize:     int64PtrBatchSvc(1),
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{
				ID:                  9106,
				SKUCode:             "TASK-SKU",
				ProductNameSnapshot: "端午节保龄球",
			},
		},
	}
	svc := NewService(&batchRepoStub{rowsByIDs: repoRows}, &batchPresignerStub{
		enabled:  true,
		urlByKey: map[string]string{"k-206": "https://oss.example/k-206"},
	}, nil)

	result, appErr := svc.BuildBatchDownloadManifest(
		context.Background(),
		[]int64{206},
		WithBatchDownloadNamingMode(BatchDownloadNamingBusiness),
	)
	if appErr != nil {
		t.Fatalf("BuildBatchDownloadManifest error = %+v", appErr)
	}
	if result.Items[0].Filename != "运营参考原图.jpg" {
		t.Fatalf("filename = %q, want reference original name", result.Items[0].Filename)
	}
}

func TestBuildBatchDownloadManifestPartialFailure(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	repoRows := []*repo.TaskAssetSearchRow{
		{
			Asset: &domain.TaskAsset{
				ID:           31,
				AssetID:      int64PtrBatchSvc(301),
				TaskID:       9200,
				FileName:     "ok.psd",
				StorageKey:   strPtr("k-301"),
				FileSize:     int64PtrBatchSvc(2),
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{ID: 9200},
		},
		{
			Asset: &domain.TaskAsset{
				ID:           32,
				AssetID:      int64PtrBatchSvc(302),
				TaskID:       9200,
				FileName:     "bad.psd",
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{ID: 9200},
		},
	}
	svc := NewService(&batchRepoStub{rowsByIDs: repoRows}, &batchPresignerStub{
		enabled:  true,
		urlByKey: map[string]string{"k-301": "https://oss.example/k-301"},
	}, nil)

	result, appErr := svc.BuildBatchDownloadManifest(context.Background(), []int64{301, 302})
	if appErr != nil {
		t.Fatalf("BuildBatchDownloadManifest error = %+v", appErr)
	}
	if result.SuccessCount != 1 || result.FailureCount != 1 {
		t.Fatalf("result counts = %+v", result)
	}
	if result.Failures[0].AssetID != 302 || result.Failures[0].Reason != "missing_storage_key" {
		t.Fatalf("failure = %+v", result.Failures[0])
	}
}

func TestBuildBatchDownloadManifestAllFailedReturnsError(t *testing.T) {
	repoRows := []*repo.TaskAssetSearchRow{
		{
			Asset: &domain.TaskAsset{
				ID:       41,
				AssetID:  int64PtrBatchSvc(401),
				TaskID:   9300,
				FileName: "x.psd",
			},
			Task: &domain.Task{ID: 9300},
		},
	}
	svc := NewService(&batchRepoStub{rowsByIDs: repoRows}, &batchPresignerStub{enabled: true}, nil)

	result, appErr := svc.BuildBatchDownloadManifest(context.Background(), []int64{401})
	if appErr == nil {
		t.Fatalf("expected error, result=%+v", result)
	}
	if appErr.Code != domain.ErrCodeAssetMissing {
		t.Fatalf("error code = %s, want %s", appErr.Code, domain.ErrCodeAssetMissing)
	}
}

func TestBuildBatchDownloadManifestAssetCountLimit(t *testing.T) {
	svc := NewService(&batchRepoStub{}, &batchPresignerStub{enabled: true}, nil)
	assetIDs := make([]int64, MaxBatchDownloadAssets+1)
	for i := range assetIDs {
		assetIDs[i] = int64(i + 1)
	}

	result, appErr := svc.BuildBatchDownloadManifest(context.Background(), assetIDs)
	if appErr == nil {
		t.Fatalf("expected limit error, result=%+v", result)
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
}

func TestBuildBatchDownloadManifestSizeLimitUsesMetadata(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	repoRows := []*repo.TaskAssetSearchRow{
		{
			Asset: &domain.TaskAsset{
				ID:           51,
				AssetID:      int64PtrBatchSvc(501),
				TaskID:       9500,
				FileName:     "ok.psd",
				StorageKey:   strPtr("k-501"),
				FileSize:     int64PtrBatchSvc(2),
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{ID: 9500},
		},
		{
			Asset: &domain.TaskAsset{
				ID:           52,
				AssetID:      int64PtrBatchSvc(502),
				TaskID:       9500,
				FileName:     "too-large.psd",
				StorageKey:   strPtr("k-502"),
				FileSize:     int64PtrBatchSvc(MaxBatchDownloadTotalBytes),
				UploadStatus: &uploaded,
			},
			Task: &domain.Task{ID: 9500},
		},
	}
	svc := NewService(&batchRepoStub{rowsByIDs: repoRows}, &batchPresignerStub{
		enabled: true,
		urlByKey: map[string]string{
			"k-501": "https://oss.example/k-501",
			"k-502": "https://oss.example/k-502",
		},
	}, nil)

	result, appErr := svc.BuildBatchDownloadManifest(context.Background(), []int64{501, 502})
	if appErr != nil {
		t.Fatalf("BuildBatchDownloadManifest error = %+v", appErr)
	}
	if result.SuccessCount != 1 || result.FailureCount != 1 {
		t.Fatalf("result counts = %+v", result)
	}
	if result.Failures[0].AssetID != 502 || result.Failures[0].Reason != "total_size_limit_exceeded" {
		t.Fatalf("failure = %+v", result.Failures[0])
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

type batchPresignerStub struct {
	enabled       bool
	expiresAt     time.Time
	urlByKey      map[string]string
	filenameByKey map[string]string
}

func (p *batchPresignerStub) Enabled() bool {
	return p.enabled
}

func (p *batchPresignerStub) PresignDownloadURL(objectKey string) *baseservice.OSSDirectDownloadInfo {
	return p.PresignDownloadURLWithFilename(objectKey, "")
}

func (p *batchPresignerStub) PresignDownloadURLWithFilename(objectKey, filename string) *baseservice.OSSDirectDownloadInfo {
	if p.filenameByKey == nil {
		p.filenameByKey = map[string]string{}
	}
	p.filenameByKey[objectKey] = filename
	url := strings.TrimSpace(p.urlByKey[objectKey])
	if url == "" {
		return nil
	}
	expiresAt := p.expiresAt
	if expiresAt.IsZero() {
		expiresAt = time.Date(2026, 5, 12, 11, 0, 0, 0, time.UTC)
	}
	return &baseservice.OSSDirectDownloadInfo{DownloadURL: url, ExpiresAt: expiresAt}
}

func strPtr(v string) *string         { return &v }
func int64PtrBatchSvc(v int64) *int64 { return &v }
