package asset_center

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	externalassets "workflow/service/external_assets"
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
	if len(item.Assets) != 1 || item.Assets[0].ID != 102 {
		t.Fatalf("assets = %+v, want one delivery jpg 102", item.Assets)
	}
}

func TestBatchSearchReturnsAllMatchingDeliveryImagesForTerm(t *testing.T) {
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	scopeSKU := "CGP000155"
	jpgMime := "image/jpeg"
	psdMime := "image/vnd.adobe.photoshop"
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	rows := []*repo.TaskAssetSearchRow{
		{
			Asset: &domain.TaskAsset{
				ID:           11697,
				TaskID:       2113,
				AssetID:      int64PtrExcelPkg(11697),
				ScopeSKUCode: &scopeSKU,
				AssetType:    domain.TaskAssetTypeDelivery,
				FileName:     "万事顺意【30x170cm】.jpg",
				MimeType:     &jpgMime,
				UploadStatus: &uploaded,
				CreatedAt:    now.Add(3 * time.Minute),
			},
			Task: &domain.Task{ID: 2113, TaskNo: "RW-20260706-A-002110", SKUCode: scopeSKU, ProductNameSnapshot: "露素常规海报"},
		},
		{
			Asset: &domain.TaskAsset{
				ID:           11696,
				TaskID:       2113,
				AssetID:      int64PtrExcelPkg(11696),
				ScopeSKUCode: &scopeSKU,
				AssetType:    domain.TaskAssetTypeDelivery,
				FileName:     "生日快乐【30x130cm】.jpg",
				MimeType:     &jpgMime,
				UploadStatus: &uploaded,
				CreatedAt:    now.Add(2 * time.Minute),
			},
			Task: &domain.Task{ID: 2113, TaskNo: "RW-20260706-A-002110", SKUCode: scopeSKU, ProductNameSnapshot: "露素常规海报"},
		},
		{
			Asset: &domain.TaskAsset{
				ID:           11631,
				TaskID:       2113,
				AssetID:      int64PtrExcelPkg(11631),
				ScopeSKUCode: &scopeSKU,
				AssetType:    domain.TaskAssetTypeDelivery,
				FileName:     "CGP000155露素常规海报-源文件.psd",
				MimeType:     &psdMime,
				UploadStatus: &uploaded,
				CreatedAt:    now.Add(time.Minute),
			},
			Task: &domain.Task{ID: 2113, TaskNo: "RW-20260706-A-002110", SKUCode: scopeSKU, ProductNameSnapshot: "露素常规海报"},
		},
	}
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
		AssetKind:    "delivery",
	})
	if appErr != nil {
		t.Fatalf("BatchSearch error = %+v", appErr)
	}
	if len(result.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(result.Results))
	}
	item := result.Results[0]
	if item.Status != BatchSearchStatusMatched || item.Asset == nil {
		t.Fatalf("item = %+v, want matched assets", item)
	}
	if item.Candidates != 2 {
		t.Fatalf("candidates = %d, want 2", item.Candidates)
	}
	if len(item.Assets) != 2 {
		t.Fatalf("assets len = %d, want 2", len(item.Assets))
	}
	if item.Asset.ID != 11697 || item.Assets[0].ID != 11697 || item.Assets[1].ID != 11696 {
		t.Fatalf("asset ordering = primary %+v assets %+v, want all matching JPG deliveries newest first", item.Asset, item.Assets)
	}
}

func TestBatchSearchMatchesExternalAssetsBySKU(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	externalRepo := &assetCenterExternalRepoStub{
		searchRows: []*domain.ExternalAssetRecord{
			{
				ID:            77,
				ResourceID:    domain.ExternalAssetResourceID(77),
				Provider:      "alist",
				Kind:          domain.ExternalAssetKindNASLocal,
				MountPath:     "/p3",
				OriginPath:    "/p3/HSC12654/HSC12654主图.jpg",
				ParentPath:    "/p3/HSC12654",
				FileName:      "HSC12654主图.jpg",
				FileExt:       ".jpg",
				MimeType:      "image/jpeg",
				FileSize:      12345,
				Status:        domain.ExternalAssetStatusIndexed,
				OSSSyncStatus: domain.ExternalAssetOSSStatusNone,
				PreviewStatus: domain.ExternalAssetPreviewStatusNone,
				CreatedAt:     now,
				UpdatedAt:     now.Add(time.Minute),
			},
		},
	}
	svc := NewService(&excelPackageRepoStub{}, excelPackagePresignerStub{}, nil)
	svc.SetExternalAssetService(externalassets.NewService(externalRepo, externalassets.Config{
		Enabled: true,
		Mounts:  externalassets.ParseMounts("/p3:nas_local"),
	}, nil))

	result, appErr := svc.BatchSearch(context.Background(), BatchSearchRequest{
		Terms:        []string{"HSC12654"},
		FormatFilter: "jpg_png",
		AssetKind:    "delivery",
	})
	if appErr != nil {
		t.Fatalf("BatchSearch error = %+v", appErr)
	}
	if len(result.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(result.Results))
	}
	item := result.Results[0]
	if item.Status != BatchSearchStatusMatched || item.Asset == nil {
		t.Fatalf("item = %+v, want matched external asset", item)
	}
	if item.Asset.ResourceID != "ext-77" || item.Asset.SourceType != string(domain.AssetResourceSourceExternal) {
		t.Fatalf("asset = %+v, want external ext-77", item.Asset)
	}
	if item.Candidates != 1 || len(item.Assets) != 1 || item.Assets[0].ResourceID != "ext-77" {
		t.Fatalf("candidates/assets = %d/%+v, want one external candidate", item.Candidates, item.Assets)
	}
	if len(externalRepo.searchQueries) != 1 || externalRepo.searchQueries[0].Keyword != "HSC12654" {
		t.Fatalf("external search queries = %+v, want keyword HSC12654", externalRepo.searchQueries)
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
