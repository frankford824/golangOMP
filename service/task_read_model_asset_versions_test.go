package service

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
)

func TestTaskReadModelIncludesCompletedUploadVersionsWithoutSubmitDesign(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			301: {
				ID:         301,
				TaskNo:     "T-301",
				TaskType:   domain.TaskTypeOriginalProductDevelopment,
				TaskStatus: domain.TaskStatusPendingAuditA,
			},
		},
		details: map[int64]*domain.TaskDetail{
			301: {TaskID: 301},
		},
	}
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := &prdTaskAssetRepo{}

	assetID, err := designAssetRepo.Create(context.Background(), step04Tx{}, &domain.DesignAsset{
		TaskID:    301,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeDelivery,
		CreatedBy: 88,
	})
	if err != nil {
		t.Fatalf("Create() design asset error = %v", err)
	}
	versionNo := 1
	versionID, err := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
		TaskID:         301,
		AssetID:        &assetID,
		AssetType:      domain.TaskAssetTypeDelivery,
		VersionNo:      1,
		AssetVersionNo: &versionNo,
		UploadMode:     strPtr("multipart"),
		FileName:       "delivery-v1.zip",
		OriginalName:   strPtr("delivery-v1.zip"),
		StorageKey:     strPtr("nas/design-assets/file-301"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     88,
		UploadedAt:     timeValuePtr(time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Create() task asset error = %v", err)
	}
	if err := designAssetRepo.UpdateCurrentVersionID(context.Background(), step04Tx{}, assetID, &versionID); err != nil {
		t.Fatalf("UpdateCurrentVersionID() error = %v", err)
	}

	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		taskAssetRepo,
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		nil,
		nil,
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskDesignAssetReadModel(designAssetRepo),
	)

	readModel, appErr := svc.GetByID(context.Background(), 301)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if len(readModel.DesignAssets) != 1 {
		t.Fatalf("design_assets len = %d, want 1", len(readModel.DesignAssets))
	}
	if len(readModel.AssetVersions) != 1 {
		t.Fatalf("asset_versions len = %d, want 1", len(readModel.AssetVersions))
	}
	if readModel.DesignAssets[0].CurrentVersion == nil || readModel.DesignAssets[0].CurrentVersion.ID != versionID {
		t.Fatalf("design_assets[0].current_version = %+v, want id=%d", readModel.DesignAssets[0].CurrentVersion, versionID)
	}
	if readModel.AssetVersions[0].ID != versionID {
		t.Fatalf("asset_versions[0].id = %d, want %d", readModel.AssetVersions[0].ID, versionID)
	}
	if !readModel.AssetVersions[0].IsDeliveryFile || readModel.AssetVersions[0].UploadStatus != domain.DesignAssetUploadStatusUploaded {
		t.Fatalf("asset_versions[0] semantics = %+v", readModel.AssetVersions[0])
	}
	if readModel.AssetVersions[0].CurrentVersionRole != "current_version" {
		t.Fatalf("asset_versions[0].current_version_role = %q, want current_version", readModel.AssetVersions[0].CurrentVersionRole)
	}
}

func TestTaskReadModelBatchIncludesReferenceFileRefsAndFallbackAssetVersions(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			302: {
				ID:              302,
				TaskNo:          "T-302",
				TaskType:        domain.TaskTypeNewProductDevelopment,
				TaskStatus:      domain.TaskStatusPendingAuditA,
				IsBatchTask:     true,
				BatchItemCount:  2,
				BatchMode:       domain.TaskBatchModeMultiSKU,
				PrimarySKUCode:  "BATCH-302-A",
				SKUCode:         "BATCH-302-A",
				OwnerTeam:       domain.AllValidTeams()[0],
				OwnerDepartment: "运营部",
				OwnerOrgTeam:    "运营三组",
			},
		},
		details: map[int64]*domain.TaskDetail{
			302: {
				TaskID:                302,
				ReferenceImagesJSON:   `[{"asset_id":"legacy-should-not-win"}]`,
				ReferenceFileRefsJSON: `[{"asset_id":"ref-batch-302","download_url":"/v1/assets/files/tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/reference/batch-302.png"}]`,
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			302: {
				{ID: 1, TaskID: 302, SequenceNo: 1, SKUCode: "BATCH-302-A", ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: "ref-a"}}},
				{ID: 2, TaskID: 302, SequenceNo: 2, SKUCode: "BATCH-302-B", ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: "ref-b"}}},
			},
		},
	}
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := &prdTaskAssetRepo{}

	v1No := 1
	v1ID, err := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
		TaskID:         302,
		AssetID:        int64Ptr(9201),
		ScopeSKUCode:   strPtr("BATCH-302-A"),
		AssetType:      domain.TaskAssetTypeDelivery,
		VersionNo:      1,
		AssetVersionNo: &v1No,
		UploadMode:     strPtr("multipart"),
		FileName:       "batch-delivery-v1.png",
		OriginalName:   strPtr("batch-delivery-v1.png"),
		StorageKey:     strPtr("tasks/T-302/AST-9201/v1.png"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     9,
		UploadedAt:     timeValuePtr(time.Date(2026, 4, 2, 8, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Create() task asset v1 error = %v", err)
	}
	v2No := 2
	v2ID, err := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
		TaskID:         302,
		AssetID:        int64Ptr(9201),
		ScopeSKUCode:   strPtr("BATCH-302-B"),
		AssetType:      domain.TaskAssetTypeDelivery,
		VersionNo:      2,
		AssetVersionNo: &v2No,
		UploadMode:     strPtr("multipart"),
		FileName:       "batch-delivery-v2.png",
		OriginalName:   strPtr("batch-delivery-v2.png"),
		StorageKey:     strPtr("tasks/T-302/AST-9201/v2.png"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     9,
		UploadedAt:     timeValuePtr(time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Create() task asset v2 error = %v", err)
	}

	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		taskAssetRepo,
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		nil,
		nil,
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskDesignAssetReadModel(designAssetRepo),
	)

	readModel, appErr := svc.GetByID(context.Background(), 302)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if len(readModel.ReferenceFileRefs) != 1 || readModel.ReferenceFileRefs[0].AssetID != "ref-batch-302" {
		t.Fatalf("reference_file_refs = %+v, want asset_id=ref-batch-302", readModel.ReferenceFileRefs)
	}
	if len(readModel.DesignAssets) != 1 {
		t.Fatalf("design_assets len = %d, want 1", len(readModel.DesignAssets))
	}
	if len(readModel.AssetVersions) != 2 {
		t.Fatalf("asset_versions len = %d, want 2", len(readModel.AssetVersions))
	}
	if readModel.DesignAssets[0].CurrentVersion == nil || readModel.DesignAssets[0].CurrentVersion.ID != v2ID {
		t.Fatalf("design_assets[0].current_version = %+v, want id=%d", readModel.DesignAssets[0].CurrentVersion, v2ID)
	}
	if readModel.AssetVersions[0].ID != v1ID || readModel.AssetVersions[1].ID != v2ID {
		t.Fatalf("asset_versions ids = [%d,%d], want [%d,%d]", readModel.AssetVersions[0].ID, readModel.AssetVersions[1].ID, v1ID, v2ID)
	}
	if readModel.AssetVersions[0].ScopeSKUCode != "BATCH-302-A" || readModel.AssetVersions[1].ScopeSKUCode != "BATCH-302-B" {
		t.Fatalf("asset_versions scope_sku_code = [%q,%q]", readModel.AssetVersions[0].ScopeSKUCode, readModel.AssetVersions[1].ScopeSKUCode)
	}
	if len(readModel.SKUItems[0].ReferenceFileRefs) != 1 || len(readModel.SKUItems[1].ReferenceFileRefs) != 1 {
		t.Fatalf("sku_items reference_file_refs = %+v", readModel.SKUItems)
	}
}
