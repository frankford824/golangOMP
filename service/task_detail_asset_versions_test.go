package service

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestTaskDetailAggregateIncludesCompletedUploadVersionsWithoutSubmitDesign(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			401: {
				ID:         401,
				TaskNo:     "T-401",
				TaskType:   domain.TaskTypeOriginalProductDevelopment,
				TaskStatus: domain.TaskStatusPendingAuditA,
			},
		},
		details: map[int64]*domain.TaskDetail{
			401: {TaskID: 401},
		},
	}
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := &prdTaskAssetRepo{}

	assetID, err := designAssetRepo.Create(context.Background(), step04Tx{}, &domain.DesignAsset{
		TaskID:    401,
		AssetNo:   "AST-0401",
		AssetType: domain.TaskAssetTypeDelivery,
		CreatedBy: 66,
	})
	if err != nil {
		t.Fatalf("Create() design asset error = %v", err)
	}
	versionNo := 1
	versionID, err := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
		TaskID:         401,
		AssetID:        &assetID,
		AssetType:      domain.TaskAssetTypeDelivery,
		VersionNo:      1,
		AssetVersionNo: &versionNo,
		UploadMode:     strPtr("multipart"),
		FileName:       "delivery-v1.zip",
		OriginalName:   strPtr("delivery-v1.zip"),
		StorageKey:     strPtr("tasks/T-401/assets/AST-0401/v1/delivery/delivery-v1.zip"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     66,
		UploadedAt:     timeValuePtr(time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Create() task asset error = %v", err)
	}
	if err := designAssetRepo.UpdateCurrentVersionID(context.Background(), step04Tx{}, assetID, &versionID); err != nil {
		t.Fatalf("UpdateCurrentVersionID() error = %v", err)
	}

	svc := NewTaskDetailAggregateService(
		taskRepo,
		&prdProcurementRepo{},
		nil,
		nil,
		&auditV7RepoStub{},
		&taskDetailOutsourceRepoStub{},
		taskAssetRepo,
		&prdWarehouseRepo{},
		&prdTaskEventRepo{},
		nil,
		nil,
		nil,
		WithTaskDetailDesignAssetReadModel(designAssetRepo),
	)

	aggregate, appErr := svc.GetByTaskID(context.Background(), 401)
	if appErr != nil {
		t.Fatalf("GetByTaskID() unexpected error: %+v", appErr)
	}
	if len(aggregate.DesignAssets) != 1 {
		t.Fatalf("design_assets len = %d, want 1", len(aggregate.DesignAssets))
	}
	if len(aggregate.AssetVersions) != 1 {
		t.Fatalf("asset_versions len = %d, want 1", len(aggregate.AssetVersions))
	}
	if aggregate.DesignAssets[0].CurrentVersion == nil || aggregate.DesignAssets[0].CurrentVersion.ID != versionID {
		t.Fatalf("design_assets[0].current_version = %+v, want id=%d", aggregate.DesignAssets[0].CurrentVersion, versionID)
	}
	if aggregate.AssetVersions[0].ID != versionID {
		t.Fatalf("asset_versions[0].id = %d, want %d", aggregate.AssetVersions[0].ID, versionID)
	}
	if !aggregate.AssetVersions[0].IsDeliveryFile || aggregate.AssetVersions[0].UploadStatus != domain.DesignAssetUploadStatusUploaded {
		t.Fatalf("asset_versions[0] semantics = %+v", aggregate.AssetVersions[0])
	}
	if aggregate.AssetVersions[0].CurrentVersionRole != "current_version" {
		t.Fatalf("asset_versions[0].current_version_role = %q, want current_version", aggregate.AssetVersions[0].CurrentVersionRole)
	}
}

func TestTaskDetailAggregateBatchIncludesFallbackAssetVersions(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			402: {
				ID:             402,
				TaskNo:         "T-402",
				TaskType:       domain.TaskTypeNewProductDevelopment,
				TaskStatus:     domain.TaskStatusPendingAuditA,
				IsBatchTask:    true,
				BatchItemCount: 2,
				BatchMode:      domain.TaskBatchModeMultiSKU,
				PrimarySKUCode: "BATCH-402-A",
				SKUCode:        "BATCH-402-A",
			},
		},
		details: map[int64]*domain.TaskDetail{
			402: {
				TaskID:                402,
				ReferenceFileRefsJSON: `[{"asset_id":"ref-batch-402","download_url":"/v1/assets/files/tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/reference/batch-402.png"}]`,
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			402: {
				{ID: 11, TaskID: 402, SequenceNo: 1, SKUCode: "BATCH-402-A"},
				{ID: 12, TaskID: 402, SequenceNo: 2, SKUCode: "BATCH-402-B"},
			},
		},
	}
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := &prdTaskAssetRepo{}

	v1No := 1
	v1ID, err := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
		TaskID:         402,
		AssetID:        int64Ptr(9301),
		AssetType:      domain.TaskAssetTypeDelivery,
		VersionNo:      1,
		AssetVersionNo: &v1No,
		UploadMode:     strPtr("multipart"),
		FileName:       "batch-detail-v1.zip",
		OriginalName:   strPtr("batch-detail-v1.zip"),
		StorageKey:     strPtr("tasks/T-402/AST-9301/v1.zip"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     7,
		UploadedAt:     timeValuePtr(time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Create() task asset v1 error = %v", err)
	}
	v2No := 2
	v2ID, err := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
		TaskID:         402,
		AssetID:        int64Ptr(9301),
		AssetType:      domain.TaskAssetTypeDelivery,
		VersionNo:      2,
		AssetVersionNo: &v2No,
		UploadMode:     strPtr("multipart"),
		FileName:       "batch-detail-v2.zip",
		OriginalName:   strPtr("batch-detail-v2.zip"),
		StorageKey:     strPtr("tasks/T-402/AST-9301/v2.zip"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     7,
		UploadedAt:     timeValuePtr(time.Date(2026, 4, 2, 11, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Create() task asset v2 error = %v", err)
	}

	svc := NewTaskDetailAggregateService(
		taskRepo,
		&prdProcurementRepo{},
		nil,
		nil,
		&auditV7RepoStub{},
		&taskDetailOutsourceRepoStub{},
		taskAssetRepo,
		&prdWarehouseRepo{},
		&prdTaskEventRepo{},
		nil,
		nil,
		nil,
		WithTaskDetailDesignAssetReadModel(designAssetRepo),
	)

	aggregate, appErr := svc.GetByTaskID(context.Background(), 402)
	if appErr != nil {
		t.Fatalf("GetByTaskID() unexpected error: %+v", appErr)
	}
	if len(aggregate.DesignAssets) != 1 {
		t.Fatalf("design_assets len = %d, want 1", len(aggregate.DesignAssets))
	}
	if len(aggregate.AssetVersions) != 2 {
		t.Fatalf("asset_versions len = %d, want 2", len(aggregate.AssetVersions))
	}
	if aggregate.DesignAssets[0].CurrentVersion == nil || aggregate.DesignAssets[0].CurrentVersion.ID != v2ID {
		t.Fatalf("design_assets[0].current_version = %+v, want id=%d", aggregate.DesignAssets[0].CurrentVersion, v2ID)
	}
	if aggregate.AssetVersions[0].ID != v1ID || aggregate.AssetVersions[1].ID != v2ID {
		t.Fatalf("asset_versions ids = [%d,%d], want [%d,%d]", aggregate.AssetVersions[0].ID, aggregate.AssetVersions[1].ID, v1ID, v2ID)
	}
}

type taskDetailOutsourceRepoStub struct{}

func (taskDetailOutsourceRepoStub) Create(_ context.Context, _ repo.Tx, _ *domain.OutsourceOrder) (int64, error) {
	return 0, nil
}

func (taskDetailOutsourceRepoStub) GetByID(_ context.Context, _ int64) (*domain.OutsourceOrder, error) {
	return nil, nil
}

func (taskDetailOutsourceRepoStub) List(_ context.Context, _ repo.OutsourceListFilter) ([]*domain.OutsourceOrder, int64, error) {
	return []*domain.OutsourceOrder{}, 0, nil
}

func (taskDetailOutsourceRepoStub) Update(_ context.Context, _ repo.Tx, _ *domain.OutsourceOrder) error {
	return nil
}
