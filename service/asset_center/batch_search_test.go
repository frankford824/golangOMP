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

func TestBatchSearchScansBeyondFirstPageForDeliveryImage(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	scopeSKU := "NSKT000263"
	jpgMime := "image/jpeg"
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	rows := make([]*repo.TaskAssetSearchRow, 0, batchSearchPageSize+1)
	for i := 0; i < batchSearchPageSize; i++ {
		rows = append(rows, &repo.TaskAssetSearchRow{
			Asset: &domain.TaskAsset{
				ID:           int64(2000 + i),
				TaskID:       501,
				AssetID:      int64PtrExcelPkg(int64(3000 + i)),
				ScopeSKUCode: &scopeSKU,
				AssetType:    domain.TaskAssetTypeReference,
				FileName:     "NSKT000263参考图.jpg",
				MimeType:     &jpgMime,
				UploadStatus: &uploaded,
				CreatedAt:    now.Add(time.Duration(i) * time.Minute),
			},
			Task: &domain.Task{ID: 501, TaskNo: "RW-20260618-A-000003", SKUCode: scopeSKU, ProductNameSnapshot: "商品C"},
		})
	}
	rows = append(rows, &repo.TaskAssetSearchRow{
		Asset: &domain.TaskAsset{
			ID:           9001,
			TaskID:       501,
			AssetID:      int64PtrExcelPkg(9901),
			ScopeSKUCode: &scopeSKU,
			AssetType:    domain.TaskAssetTypeDelivery,
			FileName:     "NSKT000263成品图.jpg",
			MimeType:     &jpgMime,
			UploadStatus: &uploaded,
			CreatedAt:    now.Add(-time.Hour),
		},
		Task: &domain.Task{ID: 501, TaskNo: "RW-20260618-A-000003", SKUCode: scopeSKU, ProductNameSnapshot: "商品C"},
	})
	svc := NewService(
		&excelPackageRepoStub{
			rowsByKeyword: map[string][]*repo.TaskAssetSearchRow{scopeSKU: rows},
		},
		excelPackagePresignerStub{},
		nil,
	)

	result, appErr := svc.BatchSearch(context.Background(), BatchSearchRequest{
		Terms:        []string{scopeSKU},
		FormatFilter: "jpg_png",
	})
	if appErr != nil {
		t.Fatalf("BatchSearch error = %+v", appErr)
	}
	if len(result.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(result.Results))
	}
	item := result.Results[0]
	if item.Status != BatchSearchStatusMatched || item.Asset == nil {
		t.Fatalf("item = %+v, want matched delivery asset", item)
	}
	if item.Asset.ID != 9901 {
		t.Fatalf("asset id = %d, want delivery jpg 9901 from second page", item.Asset.ID)
	}
}

func TestBatchSearchDefaultAssetKindRequiresDelivery(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	scopeSKU := "NSKT000264"
	jpgMime := "image/jpeg"
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	svc := NewService(
		&excelPackageRepoStub{
			rowsByKeyword: map[string][]*repo.TaskAssetSearchRow{
				scopeSKU: {
					{
						Asset: &domain.TaskAsset{
							ID:           1001,
							TaskID:       501,
							AssetID:      int64PtrExcelPkg(101),
							ScopeSKUCode: &scopeSKU,
							AssetType:    domain.TaskAssetTypeReference,
							FileName:     "NSKT000264参考图.jpg",
							MimeType:     &jpgMime,
							UploadStatus: &uploaded,
							CreatedAt:    now,
						},
						Task: &domain.Task{ID: 501, TaskNo: "RW-20260618-A-000004", SKUCode: scopeSKU, ProductNameSnapshot: "商品D"},
					},
				},
			},
		},
		excelPackagePresignerStub{},
		nil,
	)

	result, appErr := svc.BatchSearch(context.Background(), BatchSearchRequest{
		Terms: []string{scopeSKU},
	})
	if appErr != nil {
		t.Fatalf("BatchSearch error = %+v", appErr)
	}
	if len(result.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(result.Results))
	}
	if result.Results[0].Status != BatchSearchStatusNotFound {
		t.Fatalf("status = %s, want not_found for default delivery-only search", result.Results[0].Status)
	}
}
