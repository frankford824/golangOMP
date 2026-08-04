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
		TaskStatus: domain.TaskStatusPendingAudit,
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

func TestLoadTaskDesignAssetReadModelKeepsMigrationSourceAliasOutOfLegacyPointers(t *testing.T) {
	const (
		taskID        = int64(516)
		rootAssetID   = int64(483)
		originID      = int64(335)
		sourceAliasID = int64(36019)
	)
	versionNo := 1
	uploaded := "uploaded"
	now := time.Date(2026, 4, 23, 5, 17, 2, 0, time.UTC)
	designRepo := &designAssetReadModelStub{
		assetsByTask: map[int64][]*domain.DesignAsset{
			taskID: {
				{
					ID:               rootAssetID,
					TaskID:           taskID,
					AssetNo:          "AST-0001",
					AssetType:        domain.TaskAssetTypeDelivery,
					CurrentVersionID: int64Ptr(originID),
				},
			},
		},
	}
	origin := &domain.TaskAsset{
		ID:               originID,
		TaskID:           taskID,
		AssetID:          int64Ptr(rootAssetID),
		AssetType:        domain.TaskAssetTypeDelivery,
		VersionNo:        versionNo,
		AssetVersionNo:   intPtr(versionNo),
		FileName:         "1-35-175cm.psd",
		UploadStatus:     &uploaded,
		UploadedBy:       262,
		UploadedAt:       &now,
		FlowReviewStatus: domain.TaskAssetFlowReviewStatusPendingReview,
	}
	alias := *origin
	alias.ID = sourceAliasID
	alias.AssetType = domain.TaskAssetTypeSource
	alias.VersionNo = 68
	alias.SourceModuleKey = "migration"
	alias.Remark = "v8-source-alias:group=307:origin=335"
	alias.FlowReviewStatus = domain.TaskAssetFlowReviewStatusNotApplicable
	taskAssetRepo := &taskAssetReadModelStub{
		recordsByTask: map[int64][]*domain.TaskAsset{
			taskID: {origin, &alias},
		},
		recordsByAsset: map[int64][]*domain.TaskAsset{
			rootAssetID: {origin, &alias},
		},
	}
	task := &domain.Task{
		ID:         taskID,
		TaskNo:     "RW-20260423-A-000511",
		TaskStatus: domain.TaskStatusCompleted,
	}

	assets, versions, appErr := loadTaskDesignAssetReadModel(
		context.Background(), nil, designRepo, taskAssetRepo, task,
	)
	if appErr != nil {
		t.Fatalf("loadTaskDesignAssetReadModel() error = %+v", appErr)
	}
	if len(assets) != 1 || len(versions) != 1 {
		t.Fatalf("legacy read model assets/versions = %d/%d, want 1/1", len(assets), len(versions))
	}
	if versions[0].ID != originID {
		t.Fatalf("legacy version id = %d, want origin %d", versions[0].ID, originID)
	}
	if assets[0].CurrentVersion == nil || assets[0].CurrentVersion.ID != originID ||
		assets[0].ApprovedVersion == nil || assets[0].ApprovedVersion.ID != originID {
		t.Fatalf(
			"legacy current/approved = %+v/%+v, want origin %d for both",
			assets[0].CurrentVersion, assets[0].ApprovedVersion, originID,
		)
	}
	if assets[0].CurrentVersionID == nil || *assets[0].CurrentVersionID != originID ||
		assets[0].ApprovedVersionID == nil || *assets[0].ApprovedVersionID != originID {
		t.Fatalf(
			"legacy pointer ids = %v/%v, want origin %d for both",
			assets[0].CurrentVersionID, assets[0].ApprovedVersionID, originID,
		)
	}

	view := &taskAssetCenterService{taskAssetRepo: taskAssetRepo}
	root := *designRepo.assetsByTask[taskID][0]
	if err := view.hydrateDesignAssetReadModel(context.Background(), task, &root); err != nil {
		t.Fatalf("hydrateDesignAssetReadModel() error = %v", err)
	}
	if root.CurrentVersion == nil || root.CurrentVersion.ID != originID ||
		root.ApprovedVersion == nil || root.ApprovedVersion.ID != originID {
		t.Fatalf(
			"asset-center current/approved = %+v/%+v, want origin %d for both",
			root.CurrentVersion, root.ApprovedVersion, originID,
		)
	}
}

func TestWorkflowMigrationSourceAliasMarkerIsFailClosed(t *testing.T) {
	base := &domain.TaskAsset{
		AssetType:       domain.TaskAssetTypeSource,
		SourceModuleKey: "migration",
		Remark:          "v8-source-alias:group=307:origin=335",
	}
	if !isWorkflowMigrationSourceAlias(base) {
		t.Fatal("exact workflow migration source alias marker was not recognized")
	}
	for name, mutate := range map[string]func(*domain.TaskAsset){
		"ordinary source": func(row *domain.TaskAsset) { row.SourceModuleKey = "design" },
		"ordinary remark": func(row *domain.TaskAsset) { row.Remark = "source file" },
		"delivery row":    func(row *domain.TaskAsset) { row.AssetType = domain.TaskAssetTypeDelivery },
	} {
		t.Run(name, func(t *testing.T) {
			row := *base
			mutate(&row)
			if isWorkflowMigrationSourceAlias(&row) {
				t.Fatalf("non-alias row was hidden: %+v", row)
			}
		})
	}
}

func TestLoadTaskDesignAssetReadModelRejectsMigrationSourceAliasCurrentPointer(t *testing.T) {
	const (
		taskID        = int64(516)
		rootAssetID   = int64(483)
		sourceAliasID = int64(36019)
	)
	versionNo := 1
	designRepo := &designAssetReadModelStub{
		assetsByTask: map[int64][]*domain.DesignAsset{
			taskID: {
				{
					ID:               rootAssetID,
					TaskID:           taskID,
					AssetType:        domain.TaskAssetTypeDelivery,
					CurrentVersionID: int64Ptr(sourceAliasID),
				},
			},
		},
	}
	alias := &domain.TaskAsset{
		ID:               sourceAliasID,
		TaskID:           taskID,
		AssetID:          int64Ptr(rootAssetID),
		AssetType:        domain.TaskAssetTypeSource,
		AssetVersionNo:   intPtr(versionNo),
		SourceModuleKey:  "migration",
		Remark:           "v8-source-alias:group=307:origin=335",
		FlowReviewStatus: domain.TaskAssetFlowReviewStatusNotApplicable,
	}
	taskAssetRepo := &taskAssetReadModelStub{
		recordsByTask: map[int64][]*domain.TaskAsset{
			taskID: {alias},
		},
		recordsByAsset: map[int64][]*domain.TaskAsset{
			rootAssetID: {alias},
		},
	}
	task := &domain.Task{ID: taskID, TaskStatus: domain.TaskStatusCompleted}

	if _, _, appErr := loadTaskDesignAssetReadModel(
		context.Background(), nil, designRepo, taskAssetRepo, task,
	); appErr == nil {
		t.Fatal("task read model accepted migration source alias as current version")
	}

	view := &taskAssetCenterService{taskAssetRepo: taskAssetRepo}
	root := *designRepo.assetsByTask[taskID][0]
	if err := view.hydrateDesignAssetReadModel(context.Background(), task, &root); err == nil {
		t.Fatal("asset-center read model accepted migration source alias as current version")
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
		TaskStatus: domain.TaskStatusPendingAudit,
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
		TaskStatus: domain.TaskStatusPendingAudit,
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

func TestTaskServiceLoadTaskDesignAssetReadModelEnrichesUploaderNames(t *testing.T) {
	task := &domain.Task{ID: 15, TaskNo: "RW-15"}
	designRepo := &designAssetReadModelStub{
		assetsByTask: map[int64][]*domain.DesignAsset{
			15: {
				{
					ID:               601,
					TaskID:           15,
					AssetNo:          "AST-601",
					AssetType:        domain.TaskAssetTypeSource,
					CurrentVersionID: int64Ptr(701),
				},
			},
		},
	}
	taskAssetRepo := &taskAssetReadModelStub{
		recordsByTask: map[int64][]*domain.TaskAsset{
			15: {
				{
					ID:             701,
					TaskID:         15,
					AssetID:        int64Ptr(601),
					AssetType:      domain.TaskAssetTypeSource,
					AssetVersionNo: intPtr(1),
					UploadedBy:     88,
				},
			},
		},
	}
	svc := &taskService{
		designAssetRepo:         designRepo,
		taskAssetRepo:           taskAssetRepo,
		userDisplayNameResolver: &countingUploaderNameResolver{names: map[int64]string{88: "Designer 88"}},
	}

	assets, versions, appErr := svc.loadTaskDesignAssetReadModel(context.Background(), task)
	if appErr != nil {
		t.Fatalf("loadTaskDesignAssetReadModel() error = %+v", appErr)
	}
	if len(versions) != 1 || versions[0].UploadedByName != "Designer 88" {
		t.Fatalf("versions = %+v, want one enriched uploader", versions)
	}
	if len(assets) != 1 || assets[0].CurrentVersion == nil || assets[0].CurrentVersion.UploadedByName != "Designer 88" {
		t.Fatalf("assets = %+v, want enriched current version", assets)
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
	recordsByAsset   map[int64][]*domain.TaskAsset
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
func (r *taskAssetReadModelStub) ListByAssetID(_ context.Context, assetID int64) ([]*domain.TaskAsset, error) {
	r.listByAssetCalls++
	return append([]*domain.TaskAsset{}, r.recordsByAsset[assetID]...), nil
}
func (r *taskAssetReadModelStub) NextVersionNo(context.Context, repo.Tx, int64) (int, error) {
	return 0, nil
}
func (r *taskAssetReadModelStub) NextAssetVersionNo(context.Context, repo.Tx, int64) (int, error) {
	return 0, nil
}
