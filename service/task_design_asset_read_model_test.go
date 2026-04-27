package service

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestLoadTaskDesignAssetReadModelUsesTaskLevelBatchVersionRead(t *testing.T) {
	designRepo := &designAssetReadModelStub{
		assetsByTask: map[int64][]*domain.DesignAsset{
			11: {
				{ID: 101, TaskID: 11, AssetNo: "AST-101", AssetType: domain.TaskAssetTypeDelivery, CurrentVersionID: int64Ptr(201)},
				{ID: 102, TaskID: 11, AssetNo: "AST-102", AssetType: domain.TaskAssetTypePreview, CurrentVersionID: int64Ptr(202)},
			},
		},
	}
	taskAssetRepo := &taskAssetReadModelStub{
		recordsByTask: map[int64][]*domain.TaskAsset{
			11: {
				{
					ID:             201,
					TaskID:         11,
					AssetID:        int64Ptr(101),
					AssetType:      domain.TaskAssetTypeDelivery,
					AssetVersionNo: intPtr(2),
					FileName:       "delivery-v2.png",
					OriginalName:   strPtr("delivery-v2.png"),
					MimeType:       strPtr("image/png"),
					StorageKey:     strPtr("tasks/T-11/AST-101/v2.png"),
					UploadStatus:   strPtr("uploaded"),
					UploadedBy:     1,
					UploadedAt:     timeValuePtr(time.Now().UTC()),
				},
				{
					ID:             200,
					TaskID:         11,
					AssetID:        int64Ptr(101),
					AssetType:      domain.TaskAssetTypeDelivery,
					AssetVersionNo: intPtr(1),
					FileName:       "delivery-v1.png",
					OriginalName:   strPtr("delivery-v1.png"),
					MimeType:       strPtr("image/png"),
					StorageKey:     strPtr("tasks/T-11/AST-101/v1.png"),
					UploadStatus:   strPtr("uploaded"),
					UploadedBy:     1,
					UploadedAt:     timeValuePtr(time.Now().UTC()),
				},
				{
					ID:             202,
					TaskID:         11,
					AssetID:        int64Ptr(102),
					AssetType:      domain.TaskAssetTypePreview,
					AssetVersionNo: intPtr(1),
					FileName:       "preview-v1.png",
					OriginalName:   strPtr("preview-v1.png"),
					MimeType:       strPtr("image/png"),
					StorageKey:     strPtr("tasks/T-11/AST-102/v1.png"),
					UploadStatus:   strPtr("uploaded"),
					UploadedBy:     1,
					UploadedAt:     timeValuePtr(time.Now().UTC()),
				},
			},
		},
	}

	task := &domain.Task{
		ID:         11,
		TaskNo:     "T-11",
		TaskStatus: domain.TaskStatusPendingAuditA,
	}
	assets, versions, appErr := loadTaskDesignAssetReadModel(context.Background(), nil, designRepo, taskAssetRepo, task)
	if appErr != nil {
		t.Fatalf("loadTaskDesignAssetReadModel() error = %+v", appErr)
	}
	if len(assets) != 2 {
		t.Fatalf("design assets len = %d, want 2", len(assets))
	}
	if len(versions) != 3 {
		t.Fatalf("asset versions len = %d, want 3", len(versions))
	}
	if taskAssetRepo.listByTaskCalls != 1 {
		t.Fatalf("ListByTaskID calls = %d, want 1", taskAssetRepo.listByTaskCalls)
	}
	if taskAssetRepo.listByAssetCalls != 0 {
		t.Fatalf("ListByAssetID calls = %d, want 0", taskAssetRepo.listByAssetCalls)
	}
	if versions[0].ID != 200 || versions[1].ID != 201 {
		t.Fatalf("delivery version order = [%d,%d], want [200,201]", versions[0].ID, versions[1].ID)
	}
}

func TestDesignAssets_OrphanShellsAreFiltered(t *testing.T) {
	designRepo := &designAssetReadModelStub{
		assetsByTask: map[int64][]*domain.DesignAsset{
			13: {
				{ID: 401, TaskID: 13, AssetNo: "AST-401", AssetType: domain.TaskAssetTypeDelivery, CurrentVersionID: int64Ptr(501)},
				{ID: 402, TaskID: 13, AssetNo: "AST-402", AssetType: domain.TaskAssetTypePreview},
				{ID: 403, TaskID: 13, AssetNo: "AST-403", AssetType: domain.TaskAssetTypeSource, CurrentVersionID: int64Ptr(503)},
			},
		},
	}
	taskAssetRepo := &taskAssetReadModelStub{
		recordsByTask: map[int64][]*domain.TaskAsset{
			13: {
				{
					ID:             501,
					TaskID:         13,
					AssetID:        int64Ptr(401),
					AssetType:      domain.TaskAssetTypeDelivery,
					AssetVersionNo: intPtr(1),
					FileName:       "delivery.png",
					StorageKey:     strPtr("tasks/T-13/AST-401/v1.png"),
					UploadStatus:   strPtr("uploaded"),
					UploadedBy:     1,
				},
				{
					ID:             503,
					TaskID:         13,
					AssetID:        int64Ptr(403),
					AssetType:      domain.TaskAssetTypeSource,
					AssetVersionNo: intPtr(1),
					FileName:       "source.png",
					StorageKey:     strPtr("tasks/T-13/AST-403/v1.png"),
					UploadStatus:   strPtr("uploaded"),
					UploadedBy:     1,
				},
			},
		},
	}

	task := &domain.Task{
		ID:         13,
		TaskNo:     "T-13",
		TaskStatus: domain.TaskStatusPendingAuditA,
	}
	assets, versions, appErr := loadTaskDesignAssetReadModel(context.Background(), nil, designRepo, taskAssetRepo, task)
	if appErr != nil {
		t.Fatalf("loadTaskDesignAssetReadModel() error = %+v", appErr)
	}
	if len(assets) != 2 {
		t.Fatalf("design assets len = %d, want 2", len(assets))
	}
	for _, asset := range assets {
		if asset.ID == 402 {
			t.Fatalf("orphan shell asset id 402 was returned: %+v", assets)
		}
	}
	if len(versions) != 2 {
		t.Fatalf("asset versions len = %d, want 2", len(versions))
	}
}

func TestLoadTaskDesignAssetReadModelFallsBackWhenRootsMissing(t *testing.T) {
	designRepo := &designAssetReadModelStub{
		assetsByTask: map[int64][]*domain.DesignAsset{
			12: {},
		},
	}
	taskAssetRepo := &taskAssetReadModelStub{
		recordsByTask: map[int64][]*domain.TaskAsset{
			12: {
				{
					ID:             301,
					TaskID:         12,
					AssetID:        int64Ptr(9001),
					AssetType:      domain.TaskAssetTypeDelivery,
					VersionNo:      1,
					AssetVersionNo: intPtr(1),
					FileName:       "delivery-v1.png",
					OriginalName:   strPtr("delivery-v1.png"),
					MimeType:       strPtr("image/png"),
					StorageKey:     strPtr("tasks/T-12/AST-9001/v1.png"),
					UploadStatus:   strPtr("uploaded"),
					UploadedBy:     7,
					UploadedAt:     timeValuePtr(time.Now().UTC()),
				},
				{
					ID:             302,
					TaskID:         12,
					AssetID:        int64Ptr(9001),
					AssetType:      domain.TaskAssetTypeDelivery,
					VersionNo:      2,
					AssetVersionNo: intPtr(2),
					FileName:       "delivery-v2.png",
					OriginalName:   strPtr("delivery-v2.png"),
					MimeType:       strPtr("image/png"),
					StorageKey:     strPtr("tasks/T-12/AST-9001/v2.png"),
					UploadStatus:   strPtr("uploaded"),
					UploadedBy:     7,
					UploadedAt:     timeValuePtr(time.Now().UTC()),
				},
			},
		},
	}

	task := &domain.Task{
		ID:         12,
		TaskNo:     "T-12",
		TaskStatus: domain.TaskStatusPendingAuditA,
	}
	assets, versions, appErr := loadTaskDesignAssetReadModel(context.Background(), nil, designRepo, taskAssetRepo, task)
	if appErr != nil {
		t.Fatalf("loadTaskDesignAssetReadModel() error = %+v", appErr)
	}
	if len(assets) != 1 {
		t.Fatalf("design assets len = %d, want 1", len(assets))
	}
	if len(versions) != 2 {
		t.Fatalf("asset versions len = %d, want 2", len(versions))
	}
	if assets[0].CurrentVersion == nil || assets[0].CurrentVersion.ID != 302 {
		t.Fatalf("fallback current_version = %+v, want id=302", assets[0].CurrentVersion)
	}
	if versions[0].ID != 301 || versions[1].ID != 302 {
		t.Fatalf("fallback version order = [%d,%d], want [301,302]", versions[0].ID, versions[1].ID)
	}
}

type designAssetReadModelStub struct {
	assetsByTask    map[int64][]*domain.DesignAsset
	listByTaskCalls int
}

func (r *designAssetReadModelStub) Create(context.Context, repo.Tx, *domain.DesignAsset) (int64, error) {
	return 0, nil
}
func (r *designAssetReadModelStub) GetByID(context.Context, int64) (*domain.DesignAsset, error) {
	return nil, nil
}
func (r *designAssetReadModelStub) List(_ context.Context, filter repo.DesignAssetListFilter) ([]*domain.DesignAsset, error) {
	if filter.TaskID == nil {
		return []*domain.DesignAsset{}, nil
	}
	return r.ListByTaskID(context.Background(), *filter.TaskID)
}
func (r *designAssetReadModelStub) ListByTaskID(_ context.Context, taskID int64) ([]*domain.DesignAsset, error) {
	r.listByTaskCalls++
	return append([]*domain.DesignAsset{}, r.assetsByTask[taskID]...), nil
}
func (r *designAssetReadModelStub) NextAssetNo(context.Context, repo.Tx, int64) (string, error) {
	return "", nil
}
func (r *designAssetReadModelStub) UpdateCurrentVersionID(context.Context, repo.Tx, int64, *int64) error {
	return nil
}

type taskAssetReadModelStub struct {
	recordsByTask    map[int64][]*domain.TaskAsset
	listByTaskCalls  int
	listByAssetCalls int
}

func (r *taskAssetReadModelStub) Create(context.Context, repo.Tx, *domain.TaskAsset) (int64, error) {
	return 0, nil
}
func (r *taskAssetReadModelStub) GetByID(context.Context, int64) (*domain.TaskAsset, error) {
	return nil, nil
}
func (r *taskAssetReadModelStub) ListByTaskID(_ context.Context, taskID int64) ([]*domain.TaskAsset, error) {
	r.listByTaskCalls++
	return append([]*domain.TaskAsset{}, r.recordsByTask[taskID]...), nil
}
func (r *taskAssetReadModelStub) ListByAssetID(context.Context, int64) ([]*domain.TaskAsset, error) {
	r.listByAssetCalls++
	return []*domain.TaskAsset{}, nil
}
func (r *taskAssetReadModelStub) NextVersionNo(context.Context, repo.Tx, int64) (int, error) {
	return 0, nil
}
func (r *taskAssetReadModelStub) NextAssetVersionNo(context.Context, repo.Tx, int64) (int, error) {
	return 0, nil
}
