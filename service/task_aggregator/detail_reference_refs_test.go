package task_aggregator

import (
	"context"
	"testing"

	"workflow/domain"
	"workflow/repo"
	parentservice "workflow/service"
)

func TestBuildDetailReferenceFileRefsPrefersTaskDetailJSON(t *testing.T) {
	detail := &domain.TaskDetail{
		ReferenceFileRefsJSON: `[{"asset_id":"ref-1","ref_id":"ref-1","storage_key":"tasks/ref-1.png","download_url":"/v1/assets/files/tasks/ref-1.png"}]`,
	}

	refs := parentservice.BuildTaskLevelDetailReferenceFileRefs(detail, []*domain.ReferenceFileRefFlat{{RefID: "flat-ref"}})
	if len(refs) != 1 {
		t.Fatalf("refs len = %d, want 1", len(refs))
	}
	if refs[0].AssetID != "ref-1" || refs[0].StorageKey != "tasks/ref-1.png" || refs[0].DownloadURL == nil {
		t.Fatalf("refs[0] = %+v, want formal ref object from task_detail JSON", refs[0])
	}
}

func TestBuildDetailReferenceFileRefsFallsBackToFlatRefs(t *testing.T) {
	refs := parentservice.BuildTaskLevelDetailReferenceFileRefs(&domain.TaskDetail{ReferenceFileRefsJSON: "[]"}, []*domain.ReferenceFileRefFlat{{RefID: "flat-ref"}})
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
}

func TestBuildDetailUsesV8TaskStatusWhenModuleStateIsStale(t *testing.T) {
	designerID := int64(203)
	task := &domain.Task{
		ID:         629,
		TaskType:   domain.TaskTypeOriginalProductDevelopment,
		TaskStatus: domain.TaskStatusPendingAudit,
		CreatorID:  1,
		DesignerID: &designerID,
	}
	svc := &DetailService{nameResolver: detailNameResolverStub{names: map[int64]string{1: "系统管理员", 203: "设计测试账号2"}}}

	detail := svc.buildDetail(context.Background(), task, &domain.TaskDetail{
		TaskID:       629,
		FilingStatus: domain.FilingStatusFiled,
	}, []*domain.TaskModule{{
		ID:        1,
		TaskID:    629,
		ModuleKey: domain.ModuleKeyDesign,
		State:     domain.ModuleStateInProgress,
		ClaimedBy: &designerID,
	}}, nil, nil)

	if detail.DesignSubStatus != string(domain.TaskSubStatusPendingAudit) {
		t.Fatalf("design_sub_status = %q, want pending_audit", detail.DesignSubStatus)
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
	if detail.AssetVersions[0].DownloadURL == nil || *detail.AssetVersions[0].DownloadURL != "/v1/assets/files/tasks/RW-617/assets/AST-0001/v1/delivery/file.jpg" {
		t.Fatalf("asset_versions[0].download_url = %+v", detail.AssetVersions[0].DownloadURL)
	}
	if !detail.AssetVersions[0].PreviewAvailable {
		t.Fatalf("asset_versions[0].preview_available = false, want true")
	}
	if !detail.AssetVersions[0].PublicDownloadAllowed || !detail.AssetVersions[0].PreviewPublicAllowed {
		t.Fatalf("asset_versions[0] access flags = %+v", detail.AssetVersions[0])
	}
}

func TestDetailServiceUsesSingleReadBundleWithoutFallbackQueries(t *testing.T) {
	taskID := int64(618)
	designerID := int64(203)
	designAssetID := int64(990)
	assetVersion := 1
	scopeSKU := "SKU-618"
	storageKey := "tasks/RW-618/assets/final.jpg"
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	bundle := &domain.TaskDetailReadBundle{
		Task:       &domain.Task{ID: taskID, TaskNo: "RW-618", TaskType: domain.TaskTypeNewProductDevelopment, TaskStatus: domain.TaskStatusInProgress, CreatorID: 1, DesignerID: &designerID},
		TaskDetail: &domain.TaskDetail{TaskID: taskID},
		SKUItems:   []*domain.TaskSKUItem{{ID: 88, TaskID: taskID, SKUCode: "SKU-618"}},
		TaskAssets: []*domain.TaskAsset{{ID: 99, TaskID: taskID, AssetID: &designAssetID, AssetVersionNo: &assetVersion, ScopeSKUCode: &scopeSKU, AssetType: domain.TaskAssetTypeDelivery, FileName: "final.jpg", StorageKey: &storageKey, UploadStatus: &uploaded}},
		UserNames:  map[int64]string{1: "创建人", designerID: "设计师"},
	}
	svc := NewDetailService(
		bundledTaskRepoStub{detailTaskRepoStub: detailTaskRepoStub{}, bundle: bundle},
		panicDetailModuleRepo{}, panicDetailEventRepo{}, panicDetailReferenceRepo{},
		WithTaskAssetRepo(panicDetailAssetRepo{}),
		WithUserDisplayNameResolver(panicDetailNameResolver{}),
	)

	detail, err := svc.Get(context.Background(), taskID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if detail.CreatorName != "创建人" || detail.DesignerName != "设计师" {
		t.Fatalf("bundled names = %q/%q", detail.CreatorName, detail.DesignerName)
	}
	if len(detail.SKUItems) != 1 || len(detail.AssetVersions) != 1 {
		t.Fatalf("bundled sku/assets = %d/%d", len(detail.SKUItems), len(detail.AssetVersions))
	}
}

func TestDetailServiceEnforcesEffectiveTaskScopeBeforeHydratingBundle(t *testing.T) {
	const (
		taskID  = int64(1000)
		actorID = int64(231)
	)
	bundle := &domain.TaskDetailReadBundle{
		Task: &domain.Task{
			ID:         taskID,
			TaskType:   domain.TaskTypeNewProductDevelopment,
			TaskStatus: domain.TaskStatusCompleted,
			CreatorID:  999,
		},
		TaskDetail: &domain.TaskDetail{
			TaskID:                taskID,
			ReferenceFileRefsJSON: `[{"asset_id":"sensitive-ref","download_url":"https://controlled.example/ref"}]`,
		},
		TaskAssets: []*domain.TaskAsset{{
			ID:         501,
			TaskID:     taskID,
			AssetType:  domain.TaskAssetTypeDelivery,
			FileName:   "sensitive.png",
			StorageKey: strPtr("tasks/1000/sensitive.png"),
		}},
	}
	svc := NewDetailService(
		bundledTaskRepoStub{detailTaskRepoStub: detailTaskRepoStub{}, bundle: bundle},
		panicDetailModuleRepo{}, panicDetailEventRepo{}, panicDetailReferenceRepo{},
		WithTaskAssetRepo(panicDetailAssetRepo{}),
		WithReferenceFileRefEnricher(panicDetailReferenceEnricher{}),
	)

	detail, err := svc.Get(detailScopeContext(actorID, domain.AccessScopeSelf), taskID)
	if detail != nil {
		t.Fatalf("Get() detail = %+v, want nil for out-of-scope actor", detail)
	}
	appErr, ok := err.(*domain.AppError)
	if !ok || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Get() error = %#v, want PERMISSION_DENIED", err)
	}

	bundle.Task.CreatorID = actorID
	allowedSvc := NewDetailService(
		bundledTaskRepoStub{detailTaskRepoStub: detailTaskRepoStub{}, bundle: bundle},
		panicDetailModuleRepo{}, panicDetailEventRepo{}, panicDetailReferenceRepo{},
		WithTaskAssetRepo(panicDetailAssetRepo{}),
	)
	detail, err = allowedSvc.Get(detailScopeContext(actorID, domain.AccessScopeSelf), taskID)
	if err != nil {
		t.Fatalf("Get() legal self-scope error = %v", err)
	}
	if detail == nil || detail.Task == nil || detail.Task.ID != taskID {
		t.Fatalf("Get() legal self-scope detail = %+v", detail)
	}
}

func detailScopeContext(actorID int64, scope domain.AccessScopeMode) context.Context {
	const roleID int64 = 901
	effective := &domain.EffectiveAccess{
		UserID:      actorID,
		Permissions: []domain.PermissionCode{domain.PermissionTaskView},
		Assignments: []domain.AccessAssignment{{RoleID: roleID, UserID: actorID, ScopeMode: scope}},
		Sources:     []domain.EffectiveAccessNote{{RoleID: roleID, Permission: domain.PermissionTaskView, ScopeMode: scope}},
	}
	return domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:              actorID,
		Permissions:     effective.Permissions,
		EffectiveAccess: effective,
	})
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

type bundledTaskRepoStub struct {
	detailTaskRepoStub
	bundle *domain.TaskDetailReadBundle
}

func (r bundledTaskRepoStub) GetTaskDetailReadBundle(context.Context, int64, int) (*domain.TaskDetailReadBundle, error) {
	return r.bundle, nil
}

type panicDetailModuleRepo struct{ repo.TaskModuleRepo }

func (panicDetailModuleRepo) ListByTask(context.Context, int64) ([]*domain.TaskModule, error) {
	panic("fallback module query must not run")
}

type panicDetailEventRepo struct{ repo.TaskModuleEventRepo }

func (panicDetailEventRepo) ListRecentByTask(context.Context, int64, int) ([]*domain.TaskModuleEvent, error) {
	panic("fallback event query must not run")
}

type panicDetailReferenceRepo struct{ repo.ReferenceFileRefFlatRepo }

func (panicDetailReferenceRepo) ListByTask(context.Context, int64) ([]*domain.ReferenceFileRefFlat, error) {
	panic("fallback reference query must not run")
}

type panicDetailAssetRepo struct{ repo.TaskAssetRepo }

func (panicDetailAssetRepo) ListByTaskID(context.Context, int64) ([]*domain.TaskAsset, error) {
	panic("fallback asset query must not run")
}

type panicDetailNameResolver struct{}

func (panicDetailNameResolver) GetDisplayName(context.Context, int64) string {
	panic("fallback user query must not run")
}

type panicDetailReferenceEnricher struct{}

func (panicDetailReferenceEnricher) EnrichAll([]domain.ReferenceFileRef) []domain.ReferenceFileRef {
	panic("out-of-scope detail must not hydrate controlled reference URLs")
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
