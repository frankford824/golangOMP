package asset_center

import (
	"context"
	"fmt"
	pathpkg "path"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
	externalassets "workflow/service/external_assets"
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
	manifest, appErr := svc.BuildExcelPackageManifest(context.Background(), []ExcelPackageRow{
		{RowNumber: 2, OrderNo: "DD001", SKUCode: "", Quantity: 1},
	})
	if appErr != nil {
		t.Fatalf("BuildExcelPackageManifest error = %+v", appErr)
	}
	if manifest.SuccessCount != 0 || manifest.FailureCount != 1 || len(manifest.Failures) != 1 {
		t.Fatalf("manifest = %+v, want failure-only manifest", manifest)
	}
}

func TestBuildExcelPackageManifestMatchesExternalP3Image(t *testing.T) {
	now := time.Date(2026, 7, 8, 15, 30, 0, 0, time.UTC)
	ossDirect := baseservice.NewOSSDirectService(baseservice.OSSDirectConfig{
		Enabled:         true,
		Endpoint:        "oss-cn-hangzhou.aliyuncs.com",
		PublicEndpoint:  "oss-cn-hangzhou.aliyuncs.com",
		Bucket:          "test-bucket",
		AccessKeyID:     "test-key",
		AccessKeySecret: "test-secret",
		PresignExpiry:   15 * time.Minute,
	})
	externalRepo := &assetCenterExternalRepoStub{
		searchRows: []*domain.ExternalAssetRecord{
			{
				ID:             77,
				ResourceID:     domain.ExternalAssetResourceID(77),
				Provider:       "alist",
				Kind:           domain.ExternalAssetKindNASLocal,
				MountPath:      "/p3",
				OriginPath:     "/p3/仓库素材区/徐凯/KT/HSC12654.jpg",
				ParentPath:     "/p3/仓库素材区/徐凯/KT",
				FileName:       "HSC12654.jpg",
				FileExt:        ".jpg",
				MimeType:       "image/jpeg",
				FileSize:       2048,
				Status:         domain.ExternalAssetStatusIndexed,
				OSSOriginalKey: "external-assets/alist/original/p3/HSC12654.jpg",
				OSSSyncStatus:  domain.ExternalAssetOSSStatusReady,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}
	svc := NewService(&excelPackageRepoStub{}, excelPackagePresignerStub{}, nil)
	svc.SetExternalAssetService(externalassets.NewService(externalRepo, externalassets.Config{
		Enabled: true,
		Mounts:  externalassets.ParseMounts("/p3:nas_local"),
	}, ossDirect))

	manifest, appErr := svc.BuildExcelPackageManifest(context.Background(), []ExcelPackageRow{
		{RowNumber: 2, OrderNo: "SO-1", SKUCode: "HSC12654", Quantity: 2, Address: "张三*敏感地址"},
	})
	if appErr != nil {
		t.Fatalf("BuildExcelPackageManifest error = %+v", appErr)
	}
	if manifest.SuccessCount != 1 || manifest.TotalFiles != 2 || manifest.TotalSize != 4096 {
		t.Fatalf("manifest summary = %+v", manifest)
	}
	item := manifest.Items[0]
	if item.ResourceID != "ext-77" || item.SourceType != string(domain.AssetResourceSourceExternal) || item.AssetID != 77 {
		t.Fatalf("item identity = %+v, want external ext-77", item)
	}
	if item.Address != "张三*敏感地址" || item.OriginPath == "" || !strings.Contains(item.DownloadURL, "external-assets/alist/original/p3/") {
		t.Fatalf("item fields = %+v", item)
	}
}

func TestBuildExcelPackageManifestReusesDuplicateSKUMatch(t *testing.T) {
	jpgKey := "tasks/RW-1/assets/AST-1/v1/delivery/HSC12654.jpg"
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	scope := "HSC12654"
	jpgMime := "image/jpeg"
	jpgSize := int64(10)
	now := time.Date(2026, 7, 8, 16, 0, 0, 0, time.UTC)
	repoStub := &excelPackageRepoStub{
		rowsByKeyword: map[string][]*repo.TaskAssetSearchRow{
			"HSC12654": {
				{
					Asset: &domain.TaskAsset{
						ID:           1001,
						TaskID:       501,
						AssetID:      int64PtrExcelPkg(101),
						ScopeSKUCode: &scope,
						AssetType:    domain.TaskAssetTypeDelivery,
						FileName:     "HSC12654.jpg",
						MimeType:     &jpgMime,
						FileSize:     &jpgSize,
						StorageKey:   &jpgKey,
						UploadStatus: &uploaded,
						CreatedAt:    now,
					},
					Task: &domain.Task{ID: 501, TaskNo: "RW-1", SKUCode: "HSC12654", ProductNameSnapshot: "商品"},
				},
			},
		},
	}
	svc := NewService(repoStub, excelPackagePresignerStub{}, nil)

	manifest, appErr := svc.BuildExcelPackageManifest(context.Background(), []ExcelPackageRow{
		{RowNumber: 2, OrderNo: "SO-1", SKUCode: "HSC12654", Quantity: 1},
		{RowNumber: 3, OrderNo: "SO-2", SKUCode: "HSC12654", Quantity: 2},
	})
	if appErr != nil {
		t.Fatalf("BuildExcelPackageManifest error = %+v", appErr)
	}
	if manifest.SuccessCount != 2 || manifest.TotalFiles != 3 {
		t.Fatalf("manifest summary = %+v", manifest)
	}
	if got := repoStub.callsByKeyword["HSC12654"]; got != 1 {
		t.Fatalf("Search calls for duplicate SKU = %d, want 1", got)
	}
}

func TestBuildExcelPackageManifestFreezesDuplicateExternalPreparation(t *testing.T) {
	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	externalRepo := &assetCenterExternalRepoStub{
		searchRows: []*domain.ExternalAssetRecord{
			{
				ID:            88,
				ResourceID:    domain.ExternalAssetResourceID(88),
				Provider:      "alist",
				Kind:          domain.ExternalAssetKindNASLocal,
				MountPath:     "/p3",
				OriginPath:    "/p3/仓库素材区/徐凯/HSC04325.jpg",
				FileName:      "HSC04325.jpg",
				FileExt:       ".jpg",
				MimeType:      "image/jpeg",
				FileSize:      1024,
				Status:        domain.ExternalAssetStatusIndexed,
				OSSSyncStatus: domain.ExternalAssetOSSStatusNone,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
	}
	svc := NewService(&excelPackageRepoStub{}, excelPackagePresignerStub{}, nil)
	svc.SetExternalAssetService(externalassets.NewService(externalRepo, externalassets.Config{
		Enabled: true,
		Mounts:  externalassets.ParseMounts("/p3:nas_local"),
	}, nil))

	rows := make([]ExcelPackageRow, 0, 6)
	for idx := 0; idx < 6; idx++ {
		rows = append(rows, ExcelPackageRow{RowNumber: idx + 2, OrderNo: "HSC04325", SKUCode: "HSC04325", Quantity: 1})
	}
	manifest, appErr := svc.BuildExcelPackageManifest(context.Background(), rows)
	if appErr != nil {
		t.Fatalf("BuildExcelPackageManifest error = %+v", appErr)
	}
	if manifest.SuccessCount != 0 || manifest.FailureCount != 6 {
		t.Fatalf("manifest summary = %+v, want all six rows to share one stable pending result", manifest)
	}
	if len(externalRepo.ossPendingIDs) != 1 || len(externalRepo.getIDs) != 1 {
		t.Fatalf("prepare calls=%v get calls=%v, want one source preparation for duplicate SKU", externalRepo.ossPendingIDs, externalRepo.getIDs)
	}
}

func TestBuildExcelPackageManifestPrefersOSSReadyExternalCandidate(t *testing.T) {
	now := time.Date(2026, 7, 9, 10, 30, 0, 0, time.UTC)
	ready := &domain.ExternalAssetRecord{
		ID:             91,
		ResourceID:     domain.ExternalAssetResourceID(91),
		Provider:       "alist",
		Kind:           domain.ExternalAssetKindNASLocal,
		MountPath:      "/p3",
		OriginPath:     "/p3/仓库素材区/徐凯/HSC06122.jpg",
		FileName:       "HSC06122.jpg",
		FileExt:        ".jpg",
		MimeType:       "image/jpeg",
		FileSize:       2048,
		Status:         domain.ExternalAssetStatusIndexed,
		OSSOriginalKey: "external-assets/alist/original/p3/HSC06122.jpg",
		OSSSyncStatus:  domain.ExternalAssetOSSStatusReady,
		CreatedAt:      now.Add(-time.Hour),
		UpdatedAt:      now.Add(-time.Hour),
	}
	pending := *ready
	pending.ID = 92
	pending.ResourceID = domain.ExternalAssetResourceID(92)
	pending.OriginPath = "/p3/其他目录/HSC06122.jpg"
	pending.OSSOriginalKey = ""
	pending.OSSSyncStatus = domain.ExternalAssetOSSStatusNone
	pending.UpdatedAt = now
	externalRepo := &assetCenterExternalRepoStub{searchRows: []*domain.ExternalAssetRecord{&pending, ready}}
	ossDirect := baseservice.NewOSSDirectService(baseservice.OSSDirectConfig{
		Enabled: true, Endpoint: "oss-cn-hangzhou.aliyuncs.com", PublicEndpoint: "oss-cn-hangzhou.aliyuncs.com",
		Bucket: "test-bucket", AccessKeyID: "test-key", AccessKeySecret: "test-secret", PresignExpiry: 15 * time.Minute,
	})
	svc := NewService(&excelPackageRepoStub{}, excelPackagePresignerStub{}, nil)
	svc.SetExternalAssetService(externalassets.NewService(externalRepo, externalassets.Config{
		Enabled: true,
		Mounts:  externalassets.ParseMounts("/p3:nas_local"),
	}, ossDirect))

	manifest, appErr := svc.BuildExcelPackageManifest(context.Background(), []ExcelPackageRow{
		{RowNumber: 2, OrderNo: "HSC06122", SKUCode: "HSC06122", Quantity: 1},
	})
	if appErr != nil {
		t.Fatalf("BuildExcelPackageManifest error = %+v", appErr)
	}
	if manifest.SuccessCount != 1 || manifest.Items[0].AssetID != ready.ID {
		t.Fatalf("manifest=%+v, want OSS-ready candidate %d", manifest, ready.ID)
	}
	if len(externalRepo.ossPendingIDs) != 0 {
		t.Fatalf("pending IDs=%v, ready candidate must not enqueue preparation", externalRepo.ossPendingIDs)
	}
}

func TestBuildExcelPackageManifestPackagesXuKaiMultiImageFolderAsCompleteSet(t *testing.T) {
	now := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	parentPath := "/p3/仓库素材区/徐凯/水晶标/叶真婚庆水晶标/HSC33333——真-常规水晶标-卡通喜字款"
	filenames := []string{"第三张【25x35cm】.jpg", "第一张【24x32cm】.jpg", "第二张【19x15cm】.jpg"}
	searchRows := make([]*domain.ExternalAssetRecord, 0, len(filenames))
	for index, filename := range filenames {
		id := int64(101 + index)
		searchRows = append(searchRows, &domain.ExternalAssetRecord{
			ID:             id,
			ResourceID:     domain.ExternalAssetResourceID(id),
			Provider:       "alist",
			Kind:           domain.ExternalAssetKindNASLocal,
			MountPath:      "/p3",
			OriginPath:     parentPath + "/" + filename,
			ParentPath:     parentPath,
			FileName:       filename,
			FileExt:        ".jpg",
			MimeType:       "image/jpeg",
			FileSize:       int64(1000 + index),
			Status:         domain.ExternalAssetStatusIndexed,
			OSSOriginalKey: fmt.Sprintf("external-assets/alist/original/p3/HSC33333/%d.jpg", index+1),
			OSSSyncStatus:  domain.ExternalAssetOSSStatusReady,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	externalRepo := &assetCenterExternalRepoStub{searchRows: searchRows}
	ossDirect := baseservice.NewOSSDirectService(baseservice.OSSDirectConfig{
		Enabled: true, Endpoint: "oss-cn-hangzhou.aliyuncs.com", PublicEndpoint: "oss-cn-hangzhou.aliyuncs.com",
		Bucket: "test-bucket", AccessKeyID: "test-key", AccessKeySecret: "test-secret", PresignExpiry: 15 * time.Minute,
	})
	svc := NewService(&excelPackageRepoStub{}, excelPackagePresignerStub{}, nil)
	svc.SetExternalAssetService(externalassets.NewService(externalRepo, externalassets.Config{
		Enabled: true,
		Mounts:  externalassets.ParseMounts("/p3:nas_local"),
	}, ossDirect))

	manifest, appErr := svc.BuildExcelPackageManifest(context.Background(), []ExcelPackageRow{
		{RowNumber: 2, OrderNo: "HSC33333", SKUCode: "HSC33333", Quantity: 2},
	})
	if appErr != nil {
		t.Fatalf("BuildExcelPackageManifest error = %+v", appErr)
	}
	if manifest.SuccessCount != 1 || manifest.FailureCount != 0 || len(manifest.Items) != 3 {
		t.Fatalf("manifest summary = %+v, want one successful set row with three components", manifest)
	}
	if manifest.TotalFiles != 6 || manifest.TotalSize != 6006 {
		t.Fatalf("manifest totals = files:%d size:%d, want 6 files and 6006 bytes", manifest.TotalFiles, manifest.TotalSize)
	}
	wantOrder := []string{"第一张【24x32cm】.jpg", "第二张【19x15cm】.jpg", "第三张【25x35cm】.jpg"}
	for index, item := range manifest.Items {
		if item.Filename != wantOrder[index] || item.Quantity != 2 || pathpkg.Dir(item.OriginPath) != parentPath || item.PackageFolder != pathpkg.Base(parentPath) {
			t.Fatalf("items[%d]=%+v, want ordered component %q", index, item, wantOrder[index])
		}
	}

	searchRows[1].OSSOriginalKey = ""
	searchRows[1].OSSSyncStatus = domain.ExternalAssetOSSStatusNone
	incomplete, appErr := svc.BuildExcelPackageManifest(context.Background(), []ExcelPackageRow{
		{RowNumber: 2, OrderNo: "HSC33333", SKUCode: "HSC33333", Quantity: 1},
	})
	if appErr != nil {
		t.Fatalf("BuildExcelPackageManifest incomplete set error = %+v", appErr)
	}
	if incomplete.SuccessCount != 0 || incomplete.FailureCount != 1 || len(incomplete.Items) != 0 {
		t.Fatalf("incomplete manifest = %+v, want the whole set rejected", incomplete)
	}
	if len(externalRepo.ossPendingIDs) != 1 || externalRepo.ossPendingIDs[0] != searchRows[1].ID {
		t.Fatalf("pending IDs=%v, want unavailable set component %d queued", externalRepo.ossPendingIDs, searchRows[1].ID)
	}
}

func TestExcelPackageSetParentRequiresExactXuKaiSKUFolder(t *testing.T) {
	valid := "/p3/仓库素材区/徐凯/水晶标/HSC33333——套装/第一张.jpg"
	if parent, ok := excelPackageSetParent(valid, "HSC33333"); !ok || !strings.HasSuffix(parent, "HSC33333——套装") {
		t.Fatalf("excelPackageSetParent(valid) = (%q, %v), want XuKai SKU folder", parent, ok)
	}
	for _, invalid := range []string{
		"/p3/仓库素材区/徐凯/水晶标/HSC333330——其他/第一张.jpg",
		"/p3/其他素材区/HSC33333——套装/第一张.jpg",
	} {
		if parent, ok := excelPackageSetParent(invalid, "HSC33333"); ok {
			t.Fatalf("excelPackageSetParent(%q) = (%q, true), want rejected", invalid, parent)
		}
	}
}

type excelPackageRepoStub struct {
	rowsByKeyword  map[string][]*repo.TaskAssetSearchRow
	callsByKeyword map[string]int
	mu             sync.Mutex
	delay          time.Duration
	active         atomic.Int32
	maxActive      atomic.Int32
}

func (s *excelPackageRepoStub) Search(_ context.Context, query domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	active := s.active.Add(1)
	defer s.active.Add(-1)
	for {
		previous := s.maxActive.Load()
		if active <= previous || s.maxActive.CompareAndSwap(previous, active) {
			break
		}
	}
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	keyword := strings.ToUpper(strings.TrimSpace(query.Keyword))
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.callsByKeyword == nil {
		s.callsByKeyword = map[string]int{}
	}
	s.callsByKeyword[keyword]++
	rows := s.rowsByKeyword[keyword]
	total := int64(len(rows))
	if query.Page <= 0 || query.Size <= 0 {
		return rows, total, nil
	}
	start := (query.Page - 1) * query.Size
	if start >= len(rows) {
		return []*repo.TaskAssetSearchRow{}, total, nil
	}
	end := start + query.Size
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end], total, nil
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
