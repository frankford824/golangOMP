package asset_center

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
)

func TestBuildExcelPackageManifestMatchesOnlyJPGPNG(t *testing.T) {
	jpgKey := "tasks/RW-1/assets/AST-1/v1/delivery/sku-a.jpg"
	psdKey := "tasks/RW-1/assets/AST-2/v1/delivery/sku-b.psd"
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	scopeA := "SKU-A"
	scopeB := "SKU-B"
	jpgMime := "image/jpeg"
	psdMime := "image/vnd.adobe.photoshop"
	jpgSize := int64(12)
	psdSize := int64(99)
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)

	svc := NewService(
		&excelPackageRepoStub{
			rowsByKeyword: map[string][]*repo.TaskAssetSearchRow{
				"SKU-A": {
					{
						Asset: &domain.TaskAsset{
							ID:           1001,
							TaskID:       501,
							AssetID:      int64PtrExcelPkg(101),
							ScopeSKUCode: &scopeA,
							AssetType:    domain.TaskAssetTypeDelivery,
							FileName:     "SKU-A成品图.jpg",
							MimeType:     &jpgMime,
							FileSize:     &jpgSize,
							StorageKey:   &jpgKey,
							UploadStatus: &uploaded,
							CreatedAt:    now,
						},
						Task: &domain.Task{ID: 501, TaskNo: "RW-1", SKUCode: "SKU-A", ProductNameSnapshot: "商品A"},
					},
				},
				"SKU-B": {
					{
						Asset: &domain.TaskAsset{
							ID:           1002,
							TaskID:       502,
							AssetID:      int64PtrExcelPkg(102),
							ScopeSKUCode: &scopeB,
							AssetType:    domain.TaskAssetTypeDelivery,
							FileName:     "SKU-B源文件.psd",
							MimeType:     &psdMime,
							FileSize:     &psdSize,
							StorageKey:   &psdKey,
							UploadStatus: &uploaded,
							CreatedAt:    now.Add(time.Minute),
						},
						Task: &domain.Task{ID: 502, TaskNo: "RW-2", SKUCode: "SKU-B", ProductNameSnapshot: "商品B"},
					},
				},
			},
		},
		excelPackagePresignerStub{},
		nil,
	)

	manifest, appErr := svc.BuildExcelPackageManifest(context.Background(), []ExcelPackageRow{
		{RowNumber: 2, OrderNo: "DD001", SKUCode: "SKU-A", Quantity: 3},
		{RowNumber: 3, OrderNo: "DD002", SKUCode: "SKU-B", Quantity: 1},
	})
	if appErr != nil {
		t.Fatalf("BuildExcelPackageManifest error = %+v", appErr)
	}
	if len(manifest.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(manifest.Items))
	}
	item := manifest.Items[0]
	if item.AssetID != 101 || item.Quantity != 3 || item.TotalFilenameForTest() == "" {
		t.Fatalf("item = %+v", item)
	}
	if manifest.TotalFiles != 3 {
		t.Fatalf("TotalFiles = %d, want 3", manifest.TotalFiles)
	}
	if manifest.TotalSize != 36 {
		t.Fatalf("TotalSize = %d, want 36", manifest.TotalSize)
	}
	if len(manifest.Failures) != 1 || manifest.Failures[0].Reason != "asset_not_found" {
		t.Fatalf("failures = %+v, want psd filtered as not found", manifest.Failures)
	}
}

func TestBuildExcelPackageManifestRequiresSuccessfulRows(t *testing.T) {
	svc := NewService(&excelPackageRepoStub{}, excelPackagePresignerStub{}, nil)
	_, appErr := svc.BuildExcelPackageManifest(context.Background(), []ExcelPackageRow{
		{RowNumber: 2, OrderNo: "DD001", SKUCode: "", Quantity: 1},
	})
	if appErr == nil {
		t.Fatal("BuildExcelPackageManifest appErr = nil, want all rows unavailable")
	}
}

type excelPackageRepoStub struct {
	rowsByKeyword map[string][]*repo.TaskAssetSearchRow
}

func (s *excelPackageRepoStub) Search(_ context.Context, query domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	rows := s.rowsByKeyword[strings.ToUpper(strings.TrimSpace(query.Keyword))]
	return rows, int64(len(rows)), nil
}

func (s *excelPackageRepoStub) GetCurrentByAssetID(context.Context, int64) (*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

func (s *excelPackageRepoStub) ListCurrentByAssetIDs(context.Context, []int64) ([]*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

func (s *excelPackageRepoStub) ListVersionsByAssetID(context.Context, int64) ([]*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

func (s *excelPackageRepoStub) GetVersion(context.Context, int64, int64) (*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

type excelPackagePresignerStub struct{}

func (excelPackagePresignerStub) Enabled() bool { return true }

func (excelPackagePresignerStub) PresignDownloadURL(objectKey string) *baseservice.OSSDirectDownloadInfo {
	return &baseservice.OSSDirectDownloadInfo{
		DownloadURL: fmt.Sprintf("https://oss.test/%s", objectKey),
		ExpiresAt:   time.Date(2026, 5, 13, 11, 0, 0, 0, time.UTC),
	}
}

func int64PtrExcelPkg(v int64) *int64 { return &v }

func (i ExcelPackageItem) TotalFilenameForTest() string {
	return strings.TrimSpace(i.Filename)
}
