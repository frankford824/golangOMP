package asset_center

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestBatchSearchMatchesDeliveryImageBySKU(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	scopeSKU := "NSKT000261"
	jpgMime := "image/jpeg"
	psdMime := "image/vnd.adobe.photoshop"
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	svc := NewService(
		&excelPackageRepoStub{
			rowsByKeyword: map[string][]*repo.TaskAssetSearchRow{
				"NSKT000261": {
					{
						Asset: &domain.TaskAsset{
							ID:           1001,
							TaskID:       501,
							AssetID:      int64PtrExcelPkg(101),
							ScopeSKUCode: &scopeSKU,
							AssetType:    domain.TaskAssetTypeSource,
							FileName:     "NSKT000261设计源文件.psd",
							MimeType:     &psdMime,
							UploadStatus: &uploaded,
							CreatedAt:    now.Add(time.Minute),
						},
						Task: &domain.Task{ID: 501, TaskNo: "RW-20260618-A-000001", SKUCode: "NSKT000261", ProductNameSnapshot: "商品A"},
					},
					{
						Asset: &domain.TaskAsset{
							ID:           1002,
							TaskID:       501,
							AssetID:      int64PtrExcelPkg(102),
							ScopeSKUCode: &scopeSKU,
							AssetType:    domain.TaskAssetTypeDelivery,
							FileName:     "NSKT000261成品图.jpg",
							MimeType:     &jpgMime,
							UploadStatus: &uploaded,
							CreatedAt:    now,
						},
						Task: &domain.Task{ID: 501, TaskNo: "RW-20260618-A-000001", SKUCode: "NSKT000261", ProductNameSnapshot: "商品A"},
					},
				},
			},
		},
		excelPackagePresignerStub{},
		nil,
	)

	result, appErr := svc.BatchSearch(context.Background(), BatchSearchRequest{
		Terms:        []string{"NSKT000261", "NSKT000261"},
		FormatFilter: "jpg_png",
		AssetKind:    "auto",
	})
	if appErr != nil {
		t.Fatalf("BatchSearch error = %+v", appErr)
	}
	if len(result.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(result.Results))
	}
	item := result.Results[0]
	if item.Status != BatchSearchStatusMatched || item.Asset == nil {
		t.Fatalf("item = %+v, want matched asset", item)
	}
	if item.Asset.ID != 102 {
		t.Fatalf("asset id = %d, want delivery jpg 102", item.Asset.ID)
	}
	if item.Candidates != 1 {
		t.Fatalf("candidates = %d, want 1", item.Candidates)
	}
}

func TestBatchSearchFiltersAssetKind(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	scopeSKU := "NSKT000262"
	jpgMime := "image/jpeg"
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	svc := NewService(
		&excelPackageRepoStub{
			rowsByKeyword: map[string][]*repo.TaskAssetSearchRow{
				"NSKT000262": {
					{
						Asset: &domain.TaskAsset{
							ID:           1001,
							TaskID:       501,
							AssetID:      int64PtrExcelPkg(101),
							ScopeSKUCode: &scopeSKU,
							AssetType:    domain.TaskAssetTypeReference,
							FileName:     "NSKT000262参考图.jpg",
							MimeType:     &jpgMime,
							UploadStatus: &uploaded,
							CreatedAt:    now,
						},
						Task: &domain.Task{ID: 501, TaskNo: "RW-20260618-A-000002", SKUCode: "NSKT000262", ProductNameSnapshot: "商品B"},
					},
				},
			},
		},
		excelPackagePresignerStub{},
		nil,
	)

	result, appErr := svc.BatchSearch(context.Background(), BatchSearchRequest{
		Terms:        []string{"NSKT000262"},
		FormatFilter: "jpg_png",
		AssetKind:    "delivery",
	})
	if appErr != nil {
		t.Fatalf("BatchSearch error = %+v", appErr)
	}
	if len(result.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(result.Results))
	}
	if result.Results[0].Status != BatchSearchStatusNotFound {
		t.Fatalf("status = %s, want not_found", result.Results[0].Status)
	}
	if result.MatchedCount != 0 || result.FailedCount != 1 {
		t.Fatalf("counts = %+v, want 0 matched / 1 failed", result)
	}
}
