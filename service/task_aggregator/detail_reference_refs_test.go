package task_aggregator

import (
	"context"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestBuildDetailReferenceFileRefsPrefersTaskDetailJSON(t *testing.T) {
	detail := &domain.TaskDetail{
		ReferenceFileRefsJSON: `[{"asset_id":"ref-1","ref_id":"ref-1","storage_key":"tasks/ref-1.png","download_url":"/v1/assets/files/tasks/ref-1.png"}]`,
	}

	refs := buildDetailReferenceFileRefs(detail, []*domain.ReferenceFileRefFlat{{RefID: "flat-ref"}})
	if len(refs) != 1 {
		t.Fatalf("refs len = %d, want 1", len(refs))
	}
	if refs[0].AssetID != "ref-1" || refs[0].StorageKey != "tasks/ref-1.png" || refs[0].DownloadURL == nil {
		t.Fatalf("refs[0] = %+v, want formal ref object from task_detail JSON", refs[0])
	}
}

func TestBuildDetailReferenceFileRefsFallsBackToFlatRefs(t *testing.T) {
	refs := buildDetailReferenceFileRefs(&domain.TaskDetail{ReferenceFileRefsJSON: "[]"}, []*domain.ReferenceFileRefFlat{{RefID: "flat-ref"}})
	if len(refs) != 1 {
		t.Fatalf("refs len = %d, want 1", len(refs))
	}
	if refs[0].AssetID != "flat-ref" || refs[0].RefID != "flat-ref" {
		t.Fatalf("refs[0] = %+v, want flat-ref fallback", refs[0])
	}
}

func TestBuildDetailEnrichesActorNamesAndDesignWorkflow(t *testing.T) {
	designerID := int64(203)
	task := &domain.Task{
		ID:               606,
		TaskType:         domain.TaskTypeNewProductDevelopment,
		TaskStatus:       domain.TaskStatusInProgress,
		CreatorID:        1,
		DesignerID:       &designerID,
		CurrentHandlerID: &designerID,
	}
	svc := &DetailService{nameResolver: detailNameResolverStub{names: map[int64]string{1: "系统管理员", 203: "设计测试账号2"}}}

	detail := svc.buildDetail(context.Background(), task, &domain.TaskDetail{}, []*domain.TaskModule{{
		ID:        1,
		TaskID:    606,
		ModuleKey: domain.ModuleKeyDesign,
		State:     domain.ModuleStateInProgress,
		ClaimedBy: &designerID,
	}}, nil, nil)

	if detail.DesignerName != "设计测试账号2" || detail.AssigneeName != "设计测试账号2" {
		t.Fatalf("designer/assignee names = %q/%q, want 设计测试账号2", detail.DesignerName, detail.AssigneeName)
	}
	if detail.DesignSubStatus != string(domain.TaskSubStatusInProgress) {
		t.Fatalf("design_sub_status = %q, want in_progress", detail.DesignSubStatus)
	}
	if detail.Workflow.SubStatus.Design.Code != domain.TaskSubStatusInProgress {
		t.Fatalf("workflow.sub_status.design = %+v, want in_progress", detail.Workflow.SubStatus.Design)
	}
}

func TestDetailServiceReturnsSKUItemsAndScopedAssetVersions(t *testing.T) {
	taskID := int64(617)
	assetID := int64(9001)
	assetVersionNo := 1
	uploadMode := string(domain.DesignAssetUploadModeMultipart)
	uploadStatus := string(domain.DesignAssetUploadStatusUploaded)
	previewStatus := string(domain.DesignAssetPreviewStatusNotApplicable)
	storageKey := "tasks/RW-617/assets/AST-0001/v1/delivery/file.jpg"

	svc := NewDetailService(
		detailTaskRepoStub{
			task: &domain.Task{
				ID:          taskID,
				TaskNo:      "RW-617",
				TaskType:    domain.TaskTypeNewProductDevelopment,
				IsBatchTask: true,
				BatchMode:   domain.TaskBatchModeMultiSKU,
			},
			detail: &domain.TaskDetail{TaskID: taskID},
			skuItems: []*domain.TaskSKUItem{
				{TaskID: taskID, SequenceNo: 1, SKUCode: "NSGE000004", ProductNameSnapshot: "新品样品1"},
				{TaskID: taskID, SequenceNo: 2, SKUCode: "NSGE000005", ProductNameSnapshot: "新品样品2"},
			},
		},
		detailModuleRepoStub{},
		detailModuleEventRepoStub{},
		detailReferenceRepoStub{},
		WithTaskAssetRepo(detailTaskAssetRepoStub{assets: []*domain.TaskAsset{{
			ID:             7001,
			TaskID:         taskID,
			AssetID:        &assetID,
			ScopeSKUCode:   strPtr("NSGE000005"),
			AssetType:      domain.TaskAssetTypeDelivery,
			VersionNo:      1,
			AssetVersionNo: &assetVersionNo,
			UploadMode:     &uploadMode,
			FileName:       "delivery.jpg",
			StorageKey:     &storageKey,
			UploadStatus:   &uploadStatus,
			PreviewStatus:  &previewStatus,
			UploadedBy:     1,
		}}}),
	)

	detail, err := svc.Get(context.Background(), taskID)
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if len(detail.SKUItems) != 2 {
		t.Fatalf("sku_items len = %d, want 2", len(detail.SKUItems))
	}
	if len(detail.AssetVersions) != 1 {
		t.Fatalf("asset_versions len = %d, want 1", len(detail.AssetVersions))
	}
	if detail.AssetVersions[0].ScopeSKUCode != "NSGE000005" {
		t.Fatalf("asset_versions[0].scope_sku_code = %q, want NSGE000005", detail.AssetVersions[0].ScopeSKUCode)
	}
}

type detailNameResolverStub struct {
	names map[int64]string
}

func (r detailNameResolverStub) GetDisplayName(_ context.Context, id int64) string {
	return r.names[id]
}

type detailTaskRepoStub struct {
	repo.TaskRepo
	task     *domain.Task
	detail   *domain.TaskDetail
	skuItems []*domain.TaskSKUItem
}

func (r detailTaskRepoStub) GetByID(context.Context, int64) (*domain.Task, error) {
	return r.task, nil
}

func (r detailTaskRepoStub) GetDetailByTaskID(context.Context, int64) (*domain.TaskDetail, error) {
	return r.detail, nil
}

func (r detailTaskRepoStub) ListSKUItemsByTaskID(context.Context, int64) ([]*domain.TaskSKUItem, error) {
	return r.skuItems, nil
}

type detailModuleRepoStub struct{ repo.TaskModuleRepo }

func (detailModuleRepoStub) ListByTask(context.Context, int64) ([]*domain.TaskModule, error) {
	return []*domain.TaskModule{}, nil
}

type detailModuleEventRepoStub struct{ repo.TaskModuleEventRepo }

func (detailModuleEventRepoStub) ListRecentByTask(context.Context, int64, int) ([]*domain.TaskModuleEvent, error) {
	return []*domain.TaskModuleEvent{}, nil
}

type detailReferenceRepoStub struct{ repo.ReferenceFileRefFlatRepo }

func (detailReferenceRepoStub) ListByTask(context.Context, int64) ([]*domain.ReferenceFileRefFlat, error) {
	return []*domain.ReferenceFileRefFlat{}, nil
}

type detailTaskAssetRepoStub struct {
	repo.TaskAssetRepo
	assets []*domain.TaskAsset
}

func (r detailTaskAssetRepoStub) ListByTaskID(context.Context, int64) ([]*domain.TaskAsset, error) {
	return r.assets, nil
}

func strPtr(value string) *string {
	return &value
}
