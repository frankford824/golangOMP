package asset_center

import (
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
)

func TestBuildAssetDetailUsesDerivedPreviewFromSearchRow(t *testing.T) {
	svc := NewService(&fakeSearchRepo{}, testAssetCenterPreviewPresigner(), nil)
	row := previewSearchRow("poster.psd", "image/vnd.adobe.photoshop")
	row.DerivedPreviewStorageKey = "tasks/previews/poster.webp"
	row.DerivedPreviewFilename = "preview.webp"
	row.DerivedPreviewMimeType = "image/webp"

	detail := svc.buildAssetDetail(row, nil)
	if detail == nil || !detail.PreviewAvailable || detail.PreviewURL == nil {
		t.Fatalf("detail = %+v, want ready derived preview", detail)
	}
	if !strings.Contains(*detail.PreviewURL, row.DerivedPreviewStorageKey) {
		t.Fatalf("preview_url = %q, want derived key", *detail.PreviewURL)
	}
}

func TestBuildAssetDetailUsesOSSImageProcessForTIFF(t *testing.T) {
	svc := NewService(&fakeSearchRepo{}, testAssetCenterPreviewPresigner(), nil)
	row := previewSearchRow("poster.tif", "image/tiff")
	storageKey := "tasks/source/poster.tif"
	row.Asset.StorageKey = &storageKey

	detail := svc.buildAssetDetail(row, nil)
	if detail == nil || detail.PreviewURL == nil {
		t.Fatalf("detail = %+v, want direct OSS image preview", detail)
	}
	if !strings.Contains(*detail.PreviewURL, "x-oss-process") || !strings.Contains(*detail.PreviewURL, "format%2Cjpg") {
		t.Fatalf("preview_url = %q, want OSS image process", *detail.PreviewURL)
	}
}

func previewSearchRow(filename, mimeType string) *repo.TaskAssetSearchRow {
	now := time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC)
	assetID := int64(42)
	return &repo.TaskAssetSearchRow{
		Asset: &domain.TaskAsset{
			ID:        84,
			AssetID:   &assetID,
			TaskID:    21,
			AssetType: domain.TaskAssetTypeSource,
			FileName:  filename,
			MimeType:  &mimeType,
			CreatedAt: now,
		},
		Task: &domain.Task{
			ID:         21,
			TaskNo:     "RW-20260710-A-000001",
			TaskStatus: domain.TaskStatusInProgress,
			CreatedAt:  now,
		},
		DesignCreatedAt: now,
		DesignUpdatedAt: now,
	}
}

func testAssetCenterPreviewPresigner() *baseservice.OSSDirectService {
	return baseservice.NewOSSDirectService(baseservice.OSSDirectConfig{
		Enabled:         true,
		Endpoint:        "oss-cn-hangzhou.aliyuncs.com",
		PublicEndpoint:  "oss-cn-hangzhou.aliyuncs.com",
		Bucket:          "test-bucket",
		AccessKeyID:     "test-key",
		AccessKeySecret: "test-secret",
		PresignExpiry:   15 * time.Minute,
	})
}
