package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

type resourceWorkflowRepoStub struct {
	TaskResourceGroupRepository
	workflow             *domain.TaskWorkflowLock
	groups               []domain.TaskAssetGroup
	expected             int64
	listFn               func(domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64)
	flatListFn           func(domain.ResourceGroupListParams) ([]domain.FlatResourceItem, int64)
	listByTaskErr        error
	listErr              error
	flatListErr          error
	groupErr             error
	revisionsErr         error
	staged               map[int64]domain.StagedTaskAssetBinding
	workflowReads        int
	groupReads           int
	groupByID            map[int64]*domain.TaskAssetGroup
	subjectByTask        map[int64]domain.TaskAccessSubject
	revisionsByGroup     map[int64][]domain.TaskAssetGroupRevision
	revisionTotalByGroup map[int64]int64
}

type taskResourceProfileProviderStub struct {
	records []*domain.ProductManagementRecord
	taskIDs []int64
	appErr  *domain.AppError
}

func (s *taskResourceProfileProviderStub) GetByTaskIDs(_ context.Context, taskIDs []int64) ([]*domain.ProductManagementRecord, *domain.AppError) {
	s.taskIDs = append([]int64(nil), taskIDs...)
	return s.records, s.appErr
}

func (s *resourceWorkflowRepoStub) GetWorkflow(context.Context, int64) (*domain.TaskWorkflowLock, error) {
	s.workflowReads++
	return s.workflow, nil
}

func (s *resourceWorkflowRepoStub) ExpectedResourceGroupCount(context.Context, int64, domain.TaskType) (int64, error) {
	return s.expected, nil
}

func (s *resourceWorkflowRepoStub) ListByTaskID(context.Context, int64) ([]domain.TaskAssetGroup, error) {
	s.groupReads++
	return append([]domain.TaskAssetGroup(nil), s.groups...), s.listByTaskErr
}

func (s *resourceWorkflowRepoStub) ListResourceGroups(_ context.Context, params domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	items, total := s.listFn(params)
	return items, total, nil
}

func (s *resourceWorkflowRepoStub) ListFlatResourceItems(_ context.Context, params domain.ResourceGroupListParams) ([]domain.FlatResourceItem, int64, error) {
	if s.flatListErr != nil {
		return nil, 0, s.flatListErr
	}
	items, total := s.flatListFn(params)
	return items, total, nil
}

func (s *resourceWorkflowRepoStub) ListStagedAssetsForUpdate(context.Context, repo.Tx, []int64) (map[int64]domain.StagedTaskAssetBinding, error) {
	return s.staged, nil
}

func (s *resourceWorkflowRepoStub) GetResourceGroup(_ context.Context, groupID int64) (*domain.TaskAssetGroup, error) {
	if s.groupErr != nil {
		return nil, s.groupErr
	}
	group := s.groupByID[groupID]
	if group == nil {
		return nil, repo.ErrNotFound
	}
	copyGroup := *group
	return &copyGroup, nil
}

func (s *resourceWorkflowRepoStub) GetTaskAccessSubject(_ context.Context, taskID int64) (domain.TaskAccessSubject, error) {
	subject, ok := s.subjectByTask[taskID]
	if !ok {
		return domain.TaskAccessSubject{}, repo.ErrNotFound
	}
	return subject, nil
}

func (s *resourceWorkflowRepoStub) ListResourceGroupRevisions(_ context.Context, groupID int64, page, pageSize int) ([]domain.TaskAssetGroupRevision, int64, error) {
	items := append([]domain.TaskAssetGroupRevision(nil), s.revisionsByGroup[groupID]...)
	return items, s.revisionTotalByGroup[groupID], s.revisionsErr
}

func TestResourceBundleIsPureReadAndReturnsStableGroupIDs(t *testing.T) {
	workflow := &domain.TaskWorkflowLock{TaskID: 10, TaskType: domain.TaskTypeNewProductDevelopment, Status: domain.TaskStatusInProgress, CreatorID: 7}
	repository := &resourceWorkflowRepoStub{workflow: workflow, expected: 2, groups: []domain.TaskAssetGroup{{ID: 101, TaskID: 10}, {ID: 102, TaskID: 10}}}
	svc := NewTaskResourceWorkflowService(repository, nil, nil)
	actor := globalCapabilityActor(7, domain.PermissionTaskView)
	bundle, appErr := svc.ResourceBundle(context.Background(), 10, actor)
	if appErr != nil {
		t.Fatalf("ResourceBundle() error = %+v", appErr)
	}
	if len(bundle.Groups) != 2 || bundle.Groups[0].ID != 101 || bundle.Groups[1].ID != 102 {
		t.Fatalf("stable group ids = %+v", bundle.Groups)
	}
	if repository.workflowReads != 1 || repository.groupReads != 1 {
		t.Fatalf("read counts workflow/groups = %d/%d", repository.workflowReads, repository.groupReads)
	}
}

func TestCurrentResourceGroupURLsRespectScopedViewAndDownloadPermissions(t *testing.T) {
	const (
		taskID  = int64(10)
		groupID = int64(101)
	)
	departmentID := int64(55)
	newGroup := func() *domain.TaskAssetGroup {
		return &domain.TaskAssetGroup{
			ID: groupID, TaskID: taskID,
			WorkingRevision: &domain.TaskAssetGroupRevision{
				ID: 201, GroupID: groupID,
				SourceFile: &domain.TaskResourceFile{
					TaskAssetID: 301, FileName: "source.psd", StorageKey: "tasks/10/source.psd",
				},
				Items: []domain.TaskAssetGroupRevisionItem{{
					ID: 401, TaskAssetID: 302,
					File: &domain.TaskResourceFile{
						TaskAssetID: 302, FileName: "final.png", StorageKey: "tasks/10/final.png",
					},
				}},
				References: []domain.TaskAssetGroupRevisionReference{{
					ID: 501, FormalTaskAssetID: int64PtrForResourceWorkflowTest(303),
					FileNameSnapshot: "reference.png", StorageKey: "tasks/10/reference.png",
				}},
			},
		}
	}
	actor := multiCapabilityActor(7, &departmentID,
		capabilityScope{permission: domain.PermissionTaskView, scope: domain.AccessScopeOwnDepartment},
		capabilityScope{permission: domain.PermissionAssetView, scope: domain.AccessScopeOwnDepartment},
	)

	t.Run("bundle view-only actor gets previews without download urls", func(t *testing.T) {
		group := newGroup()
		repository := &resourceWorkflowRepoStub{
			workflow: &domain.TaskWorkflowLock{
				TaskID: taskID, CreatorID: 7, OwnerDepartmentID: &departmentID,
			},
			expected: 1,
			groups:   []domain.TaskAssetGroup{*group},
		}
		result, appErr := NewTaskResourceWorkflowService(repository, nil, nil).
			ResourceBundle(context.Background(), taskID, actor)
		if appErr != nil {
			t.Fatalf("ResourceBundle() error = %+v", appErr)
		}
		assertCurrentResourceURLsAreViewOnly(t, result.Groups[0].WorkingRevision)
	})

	t.Run("detail view-only actor gets previews without download urls", func(t *testing.T) {
		group := newGroup()
		repository := &resourceWorkflowRepoStub{
			groupByID: map[int64]*domain.TaskAssetGroup{groupID: group},
			subjectByTask: map[int64]domain.TaskAccessSubject{taskID: {
				TaskID: taskID, CreatorID: 7, OwnerDepartmentID: &departmentID,
			}},
		}
		result, appErr := NewTaskResourceWorkflowService(repository, nil, nil).
			ResourceGroup(context.Background(), actor, groupID)
		if appErr != nil {
			t.Fatalf("ResourceGroup() error = %+v", appErr)
		}
		assertCurrentResourceURLsAreViewOnly(t, result.WorkingRevision)
	})
}

func assertCurrentResourceURLsAreViewOnly(t *testing.T, revision *domain.TaskAssetGroupRevision) {
	t.Helper()
	if revision == nil || revision.SourceFile == nil ||
		revision.SourceFile.PreviewURL != "/v1/task-assets/301/preview" || revision.SourceFile.DownloadURL != "" {
		t.Fatalf("source urls = %+v", revision)
	}
	if len(revision.Items) != 1 || revision.Items[0].File == nil ||
		revision.Items[0].File.PreviewURL != "/v1/task-assets/302/preview" || revision.Items[0].File.DownloadURL != "" {
		t.Fatalf("final urls = %+v", revision)
	}
	if len(revision.References) != 1 ||
		revision.References[0].PreviewURL != "/v1/task-assets/303/preview" || revision.References[0].DownloadURL != "" {
		t.Fatalf("reference urls = %+v", revision)
	}
}

func TestCurrentResourceViewDoesNotUseDownloadOnlyRawFileRoute(t *testing.T) {
	svc := &taskResourceWorkflowService{repo: &resourceWorkflowRepoStub{}}
	file := &domain.TaskResourceFile{
		FileName:   "legacy-reference.png",
		StorageKey: "tasks/10/legacy-reference.png",
	}
	svc.hydrateCurrentResourceFileURL(file, true, false)
	if file.PreviewURL != "" || file.DownloadURL != "" {
		t.Fatalf("view-only raw file urls = preview %q download %q", file.PreviewURL, file.DownloadURL)
	}
	svc.hydrateCurrentResourceFileURL(file, true, true)
	if file.PreviewURL == "" || file.DownloadURL == "" {
		t.Fatalf("download-capable raw file urls = preview %q download %q", file.PreviewURL, file.DownloadURL)
	}
}

func TestResourceGroupReadSurfacesMapIntegrityFailureToControlledConflict(t *testing.T) {
	group := &domain.TaskAssetGroup{ID: 8, TaskID: 10}
	subject := domain.TaskAccessSubject{TaskID: 10, CreatorID: 7}
	viewActor := globalCapabilityActor(7, domain.PermissionAssetView)
	downloadActor := globalCapabilityActor(7, domain.PermissionAssetDownload)

	cases := []struct {
		name string
		run  func() *domain.AppError
	}{
		{
			name: "resource bundle",
			run: func() *domain.AppError {
				repository := &resourceWorkflowRepoStub{
					workflow:      &domain.TaskWorkflowLock{TaskID: 10},
					listByTaskErr: repo.ErrDataIntegrity,
				}
				_, appErr := NewTaskResourceWorkflowService(repository, nil, nil).
					ResourceBundle(context.Background(), 10, viewActor)
				return appErr
			},
		},
		{
			name: "resource group list",
			run: func() *domain.AppError {
				repository := &resourceWorkflowRepoStub{listErr: repo.ErrDataIntegrity}
				_, appErr := NewTaskResourceWorkflowService(repository, nil, nil).
					ListResourceGroups(context.Background(), viewActor, domain.ResourceGroupListParams{Page: 1, PageSize: 20})
				return appErr
			},
		},
		{
			name: "flat resource group list",
			run: func() *domain.AppError {
				repository := &resourceWorkflowRepoStub{flatListErr: repo.ErrDataIntegrity}
				_, appErr := NewTaskResourceWorkflowService(repository, nil, nil).
					ListResourceGroups(context.Background(), viewActor, domain.ResourceGroupListParams{
						ResourceRole: domain.ResourceRoleFilterFinal,
						Page:         1,
						PageSize:     20,
					})
				return appErr
			},
		},
		{
			name: "resource group detail",
			run: func() *domain.AppError {
				repository := &resourceWorkflowRepoStub{groupErr: repo.ErrDataIntegrity}
				_, appErr := NewTaskResourceWorkflowService(repository, nil, nil).
					ResourceGroup(context.Background(), viewActor, group.ID)
				return appErr
			},
		},
		{
			name: "resource group history current pointer",
			run: func() *domain.AppError {
				repository := &resourceWorkflowRepoStub{groupErr: repo.ErrDataIntegrity}
				_, appErr := NewTaskResourceWorkflowService(repository, nil, nil).
					ResourceGroupRevisions(context.Background(), viewActor, group.ID, 1, 20)
				return appErr
			},
		},
		{
			name: "resource group history page",
			run: func() *domain.AppError {
				repository := &resourceWorkflowRepoStub{
					groupByID:        map[int64]*domain.TaskAssetGroup{group.ID: group},
					subjectByTask:    map[int64]domain.TaskAccessSubject{group.TaskID: subject},
					revisionsErr:     repo.ErrDataIntegrity,
					revisionsByGroup: map[int64][]domain.TaskAssetGroupRevision{},
				}
				_, appErr := NewTaskResourceWorkflowService(repository, nil, nil).
					ResourceGroupRevisions(context.Background(), viewActor, group.ID, 1, 20)
				return appErr
			},
		},
		{
			name: "batch download",
			run: func() *domain.AppError {
				repository := &resourceWorkflowRepoStub{groupErr: repo.ErrDataIntegrity}
				_, appErr := NewTaskResourceWorkflowService(repository, nil, nil).
					BatchDownloadResourceGroups(context.Background(), downloadActor, domain.ResourceGroupBatchDownloadRequest{GroupIDs: []int64{group.ID}})
				return appErr
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			appErr := test.run()
			if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
				t.Fatalf("controlled integrity error = %+v", appErr)
			}
			details, ok := appErr.Details.(map[string]interface{})
			if !ok || details["integrity_violation"] != true {
				t.Fatalf("integrity details = %#v", appErr.Details)
			}
		})
	}
}

func TestCurrentResourceGroupReadsHydrateMigrationEvidence(t *testing.T) {
	const (
		taskID  = int64(1264)
		groupID = int64(45)
	)
	reason := "reviewed retouch reopen [migration_v2 manifest=" + strings.Repeat("a", 64) +
		" confidence=confirmed_auto confirmed_by=1 confirmed_at=2026-07-23T06:04:15Z" +
		" evidence_count=3 first_evidence=task_event_log:2ced77db-2d6d-40e3-bd56-208d259bbe51]"
	newGroup := func() *domain.TaskAssetGroup {
		working := &domain.TaskAssetGroupRevision{
			ID: 12312, GroupID: groupID, RevisionNo: 2,
			Status: domain.TaskAssetGroupRevisionFinalized, Reason: reason,
		}
		finalized := *working
		return &domain.TaskAssetGroup{
			ID: groupID, TaskID: taskID,
			WorkingRevision: working, FinalizedRevision: &finalized,
		}
	}
	bundleGroup := newGroup()
	detailGroup := newGroup()
	repository := &resourceWorkflowRepoStub{
		workflow:      &domain.TaskWorkflowLock{TaskID: taskID, TaskType: domain.TaskTypeRetouchTask, Status: domain.TaskStatusCompleted, CreatorID: 1},
		expected:      1,
		groups:        []domain.TaskAssetGroup{*bundleGroup},
		groupByID:     map[int64]*domain.TaskAssetGroup{groupID: detailGroup},
		subjectByTask: map[int64]domain.TaskAccessSubject{taskID: {TaskID: taskID}},
	}
	svc := NewTaskResourceWorkflowService(repository, nil, nil)
	actor := globalCapabilityActor(1, domain.PermissionAssetView)
	assertEvidence := func(t *testing.T, group domain.TaskAssetGroup) {
		t.Helper()
		for label, revision := range map[string]*domain.TaskAssetGroupRevision{
			"working": group.WorkingRevision, "finalized": group.FinalizedRevision,
		} {
			if revision == nil || !revision.LegacyMigration || revision.EvidenceSummary == nil {
				t.Fatalf("%s revision evidence = %+v", label, revision)
			}
			if revision.EvidenceSummary.Confidence != "confirmed_auto" ||
				revision.EvidenceSummary.EvidenceEventCount != 3 ||
				revision.EvidenceSummary.EvidenceEventIDsComplete ||
				!reflect.DeepEqual(revision.EvidenceSummary.EvidenceEventIDs, []string{"task_event_log:2ced77db-2d6d-40e3-bd56-208d259bbe51"}) {
				t.Fatalf("%s evidence summary = %+v", label, revision.EvidenceSummary)
			}
		}
	}

	bundle, appErr := svc.ResourceBundle(context.Background(), taskID, actor)
	if appErr != nil {
		t.Fatalf("ResourceBundle() error = %+v", appErr)
	}
	if len(bundle.Groups) != 1 {
		t.Fatalf("ResourceBundle() groups = %+v", bundle.Groups)
	}
	assertEvidence(t, bundle.Groups[0])

	group, appErr := svc.ResourceGroup(context.Background(), actor, groupID)
	if appErr != nil {
		t.Fatalf("ResourceGroup() error = %+v", appErr)
	}
	assertEvidence(t, *group)
}

func TestResourceBundleRejectsHistoricalMissingGroupWithoutWriting(t *testing.T) {
	workflow := &domain.TaskWorkflowLock{TaskID: 10, TaskType: domain.TaskTypeNewProductDevelopment, Status: domain.TaskStatusInProgress, CreatorID: 7}
	repository := &resourceWorkflowRepoStub{workflow: workflow, expected: 2, groups: []domain.TaskAssetGroup{{ID: 101, TaskID: 10}}}
	svc := NewTaskResourceWorkflowService(repository, nil, nil)
	_, appErr := svc.ResourceBundle(context.Background(), 10, globalCapabilityActor(7, domain.PermissionTaskView))
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("ResourceBundle() appErr = %+v", appErr)
	}
	if repository.workflowReads != 1 || repository.groupReads != 1 {
		t.Fatalf("read counts workflow/groups = %d/%d", repository.workflowReads, repository.groupReads)
	}
}

func TestListResourceGroupsPushesScopeBeforePagination(t *testing.T) {
	departmentID := int64(101)
	actor := scopedCapabilityActor(7, domain.PermissionAssetView, domain.AccessScopeOwnDepartment, &departmentID, nil, nil)
	all := []domain.TaskAssetGroup{{ID: 1, TaskID: 11}, {ID: 2, TaskID: 12}, {ID: 3, TaskID: 13}}
	repository := &resourceWorkflowRepoStub{listFn: func(params domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64) {
		if len(params.Access.DepartmentIDs) != 1 || params.Access.DepartmentIDs[0] != departmentID {
			t.Fatalf("SQL scope params = %+v", params.Access)
		}
		// Simulate the repository's scoped result: the unauthorized global first
		// row is removed before LIMIT/OFFSET, so page one still returns group 2.
		visible := all[1:]
		start := (params.Page - 1) * params.PageSize
		end := start + params.PageSize
		if end > len(visible) {
			end = len(visible)
		}
		return visible[start:end], int64(len(visible))
	}}
	svc := NewTaskResourceWorkflowService(repository, nil, nil)
	result, appErr := svc.ListResourceGroups(context.Background(), actor, domain.ResourceGroupListParams{Page: 1, PageSize: 1})
	if appErr != nil {
		t.Fatalf("ListResourceGroups() error = %+v", appErr)
	}
	if result.Total != 2 || len(result.Items) != 1 || result.Items[0].ID != 2 {
		t.Fatalf("scoped page = %+v", result)
	}
}

func TestListResourceGroupsDoesNotExposeDownloadURLsFromViewScope(t *testing.T) {
	group := domain.TaskAssetGroup{
		ID:     1,
		TaskID: 11,
		FinalizedRevision: &domain.TaskAssetGroupRevision{
			ID:     21,
			Status: domain.TaskAssetGroupRevisionFinalized,
			Items: []domain.TaskAssetGroupRevisionItem{{
				ID:          31,
				TaskAssetID: 41,
				File: &domain.TaskResourceFile{
					TaskAssetID: 41,
					FileName:    "final.png",
					StorageKey:  "tasks/11/final.png",
				},
			}},
		},
	}
	repository := &resourceWorkflowRepoStub{listFn: func(domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64) {
		return []domain.TaskAssetGroup{group}, 1
	}}
	svc := NewTaskResourceWorkflowService(repository, nil, nil)
	result, appErr := svc.ListResourceGroups(
		context.Background(),
		globalCapabilityActor(7, domain.PermissionAssetView),
		domain.ResourceGroupListParams{Page: 1, PageSize: 20},
	)
	if appErr != nil {
		t.Fatalf("ListResourceGroups() error = %+v", appErr)
	}
	file := result.Items[0].FinalizedRevision.Items[0].File
	if file.PreviewURL != "/v1/task-assets/41/preview" {
		t.Fatalf("view-scoped list preview_url = %q", file.PreviewURL)
	}
	if file.DownloadURL != "" {
		t.Fatalf("view-scoped list leaked download_url = %q", file.DownloadURL)
	}
}

func TestListResourceGroupsHydratesExactSKUProfilesInOneBatch(t *testing.T) {
	skuItemID := int64(41)
	repository := &resourceWorkflowRepoStub{listFn: func(params domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64) {
		return []domain.TaskAssetGroup{
			{ID: 1, TaskID: 11, TaskSKUItemID: &skuItemID, SKUCode: "SKU-11"},
			{ID: 2, TaskID: 12, SKUCode: "SKU-MAIN"},
		}, 2
	}}
	provider := &taskResourceProfileProviderStub{records: []*domain.ProductManagementRecord{
		{ID: 101, TaskID: 11, TaskSKUItemID: &skuItemID, SKUCode: "SKU-11", ComboSKUCodes: []string{"COMBO-1"}},
		{ID: 102, TaskID: 12, SKUCode: "SKU-MAIN"},
	}}
	svc := NewTaskResourceWorkflowService(repository, nil, nil, WithTaskResourceWorkflowSKUProfiles(provider))
	result, appErr := svc.ListResourceGroups(context.Background(), globalCapabilityActor(7, domain.PermissionAssetView), domain.ResourceGroupListParams{Page: 1, PageSize: 20})
	if appErr != nil {
		t.Fatalf("ListResourceGroups() error = %+v", appErr)
	}
	if len(provider.taskIDs) != 2 || provider.taskIDs[0] != 11 || provider.taskIDs[1] != 12 {
		t.Fatalf("profile batch task ids = %+v", provider.taskIDs)
	}
	if len(result.Items) != 2 || result.Items[0].SKUProfile == nil || result.Items[0].SKUProfile.ID != 101 {
		t.Fatalf("first sku profile = %+v", result.Items)
	}
	if result.Items[1].SKUProfile == nil || result.Items[1].SKUProfile.ID != 102 {
		t.Fatalf("task-scope profile = %+v", result.Items[1].SKUProfile)
	}
}

func TestListResourceGroupsKeepsGroupsWhenProfilesAreUnavailable(t *testing.T) {
	repository := &resourceWorkflowRepoStub{listFn: func(domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64) {
		return []domain.TaskAssetGroup{{ID: 1, TaskID: 11, SKUCode: "SKU-MAIN"}}, 1
	}}
	provider := &taskResourceProfileProviderStub{appErr: domain.NewAppError(domain.ErrCodeInternalError, "profile unavailable", nil)}
	svc := NewTaskResourceWorkflowService(repository, nil, nil, WithTaskResourceWorkflowSKUProfiles(provider))
	result, appErr := svc.ListResourceGroups(context.Background(), globalCapabilityActor(7, domain.PermissionAssetView), domain.ResourceGroupListParams{Page: 1, PageSize: 20})
	if appErr != nil {
		t.Fatalf("ListResourceGroups() must not fail when optional profiles are unavailable: %+v", appErr)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != 1 {
		t.Fatalf("result = %+v", result)
	}
	if result.Items[0].SKUProfile != nil {
		t.Fatalf("sku profile = %+v", result.Items[0].SKUProfile)
	}
}

func TestResourceGroupRevisionsUsesPreferredScopedPermissionAndSafeURLs(t *testing.T) {
	departmentID, otherDepartmentID := int64(101), int64(202)
	workingID, finalizedID := int64(31), int64(30)
	group := &domain.TaskAssetGroup{ID: 10, TaskID: 20, WorkingRevisionID: &workingID, FinalizedRevisionID: &finalizedID}
	revisions := []domain.TaskAssetGroupRevision{{
		ID: 30, GroupID: group.ID, RevisionNo: 2, Status: domain.TaskAssetGroupRevisionFinalized,
		CreatedBy: 9, CreatedByName: "审核员",
		Reason:     "审核确认 [migration_v2 manifest=" + strings.Repeat("a", 64) + " confidence=confirmed_auto confirmed_by=9 confirmed_at=2026-07-22T08:00:00Z evidence=task_event_log:00000000-0000-0000-0000-000000000001,task_module_event:42 upload_sessions=session-a,session-b]",
		SourceFile: &domain.TaskResourceFile{TaskAssetID: 40, FileName: "source.psd", StorageKey: "tasks/20/source.psd"},
		Items:      []domain.TaskAssetGroupRevisionItem{{ID: 50, TaskAssetID: 60, File: &domain.TaskResourceFile{TaskAssetID: 60, FileName: "final.png", StorageKey: "tasks/20/final.png"}}},
		References: []domain.TaskAssetGroupRevisionReference{
			{ID: 70, FormalTaskAssetID: int64PtrForResourceWorkflowTest(80), FileNameSnapshot: "reference.jpg", StorageKey: "tasks/20/reference.jpg"},
			{ID: 71, FileNameSnapshot: "snapshot-only.jpg", StorageKey: "objects/reference/snapshot-only.jpg"},
		},
	}}
	repository := &resourceWorkflowRepoStub{
		groupByID:            map[int64]*domain.TaskAssetGroup{group.ID: group},
		subjectByTask:        map[int64]domain.TaskAccessSubject{group.TaskID: {TaskID: group.TaskID, OwnerDepartmentID: &departmentID}},
		revisionsByGroup:     map[int64][]domain.TaskAssetGroupRevision{group.ID: revisions},
		revisionTotalByGroup: map[int64]int64{group.ID: 1},
	}
	svc := NewTaskResourceWorkflowService(repository, nil, nil)

	t.Run("asset view gets controlled previews without download urls", func(t *testing.T) {
		actor := multiCapabilityActor(7, &departmentID,
			capabilityScope{permission: domain.PermissionAssetView, scope: domain.AccessScopeOwnDepartment},
			capabilityScope{permission: domain.PermissionTaskView, scope: domain.AccessScopeGlobal},
		)
		result, appErr := svc.ResourceGroupRevisions(context.Background(), actor, group.ID, 1, 20)
		if appErr != nil {
			t.Fatalf("ResourceGroupRevisions() error = %+v", appErr)
		}
		revision := result.Items[0]
		if revision.SourceFile.PreviewURL != "/v1/task-assets/40/preview" || revision.SourceFile.DownloadURL != "" {
			t.Fatalf("source urls = preview %q download %q", revision.SourceFile.PreviewURL, revision.SourceFile.DownloadURL)
		}
		if revision.Items[0].File.PreviewURL != "/v1/task-assets/60/preview" || revision.Items[0].File.DownloadURL != "" {
			t.Fatalf("item urls = %+v", revision.Items[0].File)
		}
		if revision.References[0].PreviewURL != "/v1/task-assets/80/preview" || revision.References[0].DownloadURL != "" {
			t.Fatalf("reference urls = %+v", revision.References[0])
		}
		if revision.References[1].PreviewURL != "" || revision.References[1].DownloadURL != "" {
			t.Fatalf("snapshot-only reference leaked urls = %+v", revision.References[1])
		}
		if !revision.LegacyMigration || revision.EvidenceSummary == nil || revision.EvidenceSummary.BusinessReason != "审核确认" || revision.EvidenceSummary.ManifestSHA256 != strings.Repeat("a", 64) {
			t.Fatalf("migration evidence summary = %+v", revision.EvidenceSummary)
		}
		if !revision.EvidenceSummary.UploadSessionsKnown || !reflect.DeepEqual(revision.EvidenceSummary.UploadSessionIDs, []string{"session-a", "session-b"}) {
			t.Fatalf("upload session evidence = %+v", revision.EvidenceSummary)
		}
		if revision.EvidenceSummary.EvidenceEventCount != 2 || !revision.EvidenceSummary.EvidenceEventIDsComplete ||
			len(revision.EvidenceSummary.EvidenceEventIDs) != 2 {
			t.Fatalf("full evidence completeness = %+v", revision.EvidenceSummary)
		}
	})

	t.Run("broader task view cannot override narrow asset scope", func(t *testing.T) {
		actor := multiCapabilityActor(7, &otherDepartmentID,
			capabilityScope{permission: domain.PermissionAssetView, scope: domain.AccessScopeOwnDepartment},
			capabilityScope{permission: domain.PermissionTaskView, scope: domain.AccessScopeGlobal},
		)
		result, appErr := svc.ResourceGroupRevisions(context.Background(), actor, group.ID, 1, 20)
		if result != nil || appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
			t.Fatalf("combined scope result/error = %+v/%+v", result, appErr)
		}
	})

	t.Run("pure task view cannot read asset revision metadata", func(t *testing.T) {
		actor := multiCapabilityActor(7, nil, capabilityScope{permission: domain.PermissionTaskView, scope: domain.AccessScopeGlobal})
		result, appErr := svc.ResourceGroupRevisions(context.Background(), actor, group.ID, 1, 20)
		if result != nil || appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
			t.Fatalf("task-view-only result/error = %+v/%+v", result, appErr)
		}
	})

	t.Run("scoped asset download adds download urls", func(t *testing.T) {
		actor := multiCapabilityActor(7, &departmentID,
			capabilityScope{permission: domain.PermissionAssetView, scope: domain.AccessScopeOwnDepartment},
			capabilityScope{permission: domain.PermissionAssetDownload, scope: domain.AccessScopeOwnDepartment},
		)
		result, appErr := svc.ResourceGroupRevisions(context.Background(), actor, group.ID, 1, 20)
		if appErr != nil {
			t.Fatalf("ResourceGroupRevisions() error = %+v", appErr)
		}
		if result.Items[0].SourceFile.DownloadURL != "/v1/task-assets/40/download" || result.Items[0].References[0].DownloadURL != "/v1/task-assets/80/download" {
			t.Fatalf("download-capable response omitted urls: %+v", result.Items[0])
		}
		if result.Items[0].References[1].DownloadURL != "" || result.Items[0].References[1].PreviewURL != "" {
			t.Fatalf("snapshot-only reference leaked authorized urls: %+v", result.Items[0].References[1])
		}
	})
}

func TestResourceGroupRevisionsDoesNotExposeURLsForHistoricalUnavailableFile(t *testing.T) {
	group := &domain.TaskAssetGroup{ID: 10, TaskID: 20}
	repository := &resourceWorkflowRepoStub{
		groupByID:     map[int64]*domain.TaskAssetGroup{group.ID: group},
		subjectByTask: map[int64]domain.TaskAccessSubject{group.TaskID: {TaskID: group.TaskID}},
		revisionsByGroup: map[int64][]domain.TaskAssetGroupRevision{group.ID: {{
			ID: 30, GroupID: group.ID, RevisionNo: 1, Status: domain.TaskAssetGroupRevisionSuperseded,
			Items: []domain.TaskAssetGroupRevisionItem{{
				ID: 50,
				File: &domain.TaskResourceFile{
					TaskAssetID:       12323,
					FileName:          "lost.psd",
					Availability:      domain.TaskResourceFileHistoricalUnavailable,
					UnavailableReason: "original object was unavailable before V8 migration",
				},
			}},
			References: []domain.TaskAssetGroupRevisionReference{{
				ID:                51,
				FormalTaskAssetID: int64PtrForResourceWorkflowTest(12324),
				FileNameSnapshot:  "lost-reference.png",
				Availability:      domain.TaskResourceFileHistoricalUnavailable,
				UnavailableReason: "legacy_original_object_missing",
			}},
		}}},
		revisionTotalByGroup: map[int64]int64{group.ID: 1},
	}
	svc := NewTaskResourceWorkflowService(repository, nil, nil)
	actor := multiCapabilityActor(7, nil,
		capabilityScope{permission: domain.PermissionAssetView, scope: domain.AccessScopeGlobal},
		capabilityScope{permission: domain.PermissionAssetDownload, scope: domain.AccessScopeGlobal},
	)

	result, appErr := svc.ResourceGroupRevisions(context.Background(), actor, group.ID, 1, 20)
	if appErr != nil {
		t.Fatalf("ResourceGroupRevisions() error = %+v", appErr)
	}
	file := result.Items[0].Items[0].File
	if file == nil || file.PreviewURL != "" || file.DownloadURL != "" ||
		file.Availability != domain.TaskResourceFileHistoricalUnavailable {
		t.Fatalf("historical unavailable file = %+v", file)
	}
	reference := result.Items[0].References[0]
	if reference.PreviewURL != "" || reference.DownloadURL != "" ||
		reference.Availability != domain.TaskResourceFileHistoricalUnavailable {
		t.Fatalf("historical unavailable reference = %+v", reference)
	}
}

func TestParseLegacyMigrationEvidenceFailsSafe(t *testing.T) {
	tests := []struct {
		name             string
		reason           string
		wantMarked       bool
		wantParsed       bool
		wantKnown        bool
		wantConfidence   string
		wantCount        int64
		wantIDs          []string
		wantIDsComplete  bool
		wantReasonSHA256 string
	}{
		{name: "ordinary reason", reason: "ordinary review"},
		{name: "malformed marked reason", reason: "legacy [migration_v2 manifest=bad]", wantMarked: true},
		{
			name:            "legacy full metadata without upload proof",
			reason:          "legacy [migration_v2 manifest=" + strings.Repeat("b", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence=task_event_log:00000000-0000-0000-0000-000000000007]",
			wantMarked:      true,
			wantParsed:      true,
			wantConfidence:  "confirmed_auto",
			wantCount:       1,
			wantIDs:         []string{"task_event_log:00000000-0000-0000-0000-000000000007"},
			wantIDsComplete: true,
		},
		{
			name:            "compact metadata exposes partial ids",
			reason:          "legacy [migration_v2 manifest=" + strings.Repeat("c", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=3 first_evidence=task_event_log:00000000-0000-0000-0000-000000000007]",
			wantMarked:      true,
			wantParsed:      true,
			wantConfidence:  "confirmed_auto",
			wantCount:       3,
			wantIDs:         []string{"task_event_log:00000000-0000-0000-0000-000000000007"},
			wantIDsComplete: false,
		},
		{
			name:             "oversized compact metadata exposes reason hash and hard blocker",
			reason:           "[migration_v2 manifest=" + strings.Repeat("d", 64) + " reason_sha256=" + strings.Repeat("e", 64) + " confidence=hard_blocked confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=1 first_evidence=task_module_event:42]",
			wantMarked:       true,
			wantParsed:       true,
			wantConfidence:   "hard_blocked",
			wantCount:        1,
			wantIDs:          []string{"task_module_event:42"},
			wantIDsComplete:  true,
			wantReasonSHA256: strings.Repeat("e", 64),
		},
		{name: "reject mixed full and compact evidence", reason: "[migration_v2 manifest=" + strings.Repeat("f", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence=task_module_event:42 evidence_count=1 first_evidence=task_module_event:42]", wantMarked: true},
		{name: "reject compact count without first id", reason: "[migration_v2 manifest=" + strings.Repeat("a", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=2]", wantMarked: true},
		{name: "reject compact first id with zero count", reason: "[migration_v2 manifest=" + strings.Repeat("a", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=0 first_evidence=task_module_event:42]", wantMarked: true},
		{name: "reject compact zero count without first id", reason: "[migration_v2 manifest=" + strings.Repeat("a", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=0]", wantMarked: true},
		{name: "reject noncanonical count", reason: "[migration_v2 manifest=" + strings.Repeat("a", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=01 first_evidence=task_module_event:42]", wantMarked: true},
		{name: "reject malformed task event id", reason: "[migration_v2 manifest=" + strings.Repeat("a", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=1 first_evidence=task_event_log:event-7]", wantMarked: true},
		{name: "reject uppercase task event id", reason: "[migration_v2 manifest=" + strings.Repeat("a", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=1 first_evidence=task_event_log:00000000-0000-0000-0000-00000000000A]", wantMarked: true},
		{name: "reject malformed module event id", reason: "[migration_v2 manifest=" + strings.Repeat("a", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=1 first_evidence=task_module_event:0042]", wantMarked: true},
		{name: "reject uppercase hash", reason: "[migration_v2 manifest=" + strings.Repeat("A", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=1 first_evidence=task_module_event:42]", wantMarked: true},
		{name: "reject malformed reason hash", reason: "[migration_v2 manifest=" + strings.Repeat("a", 64) + " reason_sha256=bad confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=1 first_evidence=task_module_event:42]", wantMarked: true},
		{name: "reject hashed reason with inline prose", reason: "inline [migration_v2 manifest=" + strings.Repeat("a", 64) + " reason_sha256=" + strings.Repeat("b", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=1 first_evidence=task_module_event:42]", wantMarked: true},
		{name: "reject duplicate token", reason: "[migration_v2 manifest=" + strings.Repeat("a", 64) + " confidence=confirmed_auto confidence=hard_blocked confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=1 first_evidence=task_module_event:42]", wantMarked: true},
		{name: "reject unknown token", reason: "[migration_v2 manifest=" + strings.Repeat("a", 64) + " confidence=confirmed_auto confirmed_by=7 confirmed_at=2026-07-22T08:00:00Z evidence_count=1 first_evidence=task_module_event:42 extra=x]", wantMarked: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			summary, marked := ParseLegacyMigrationEvidence(test.reason)
			if marked != test.wantMarked || (summary != nil) != test.wantParsed {
				t.Fatalf("ParseLegacyMigrationEvidence() = %+v/%v", summary, marked)
			}
			if summary != nil && summary.UploadSessionsKnown != test.wantKnown {
				t.Fatalf("upload sessions known = %v", summary.UploadSessionsKnown)
			}
			if summary != nil && (summary.Confidence != test.wantConfidence ||
				summary.EvidenceEventCount != test.wantCount ||
				!reflect.DeepEqual(summary.EvidenceEventIDs, test.wantIDs) ||
				summary.EvidenceEventIDsComplete != test.wantIDsComplete ||
				summary.BusinessReasonSHA256 != test.wantReasonSHA256) {
				t.Fatalf("migration evidence = %+v", summary)
			}
		})
	}
}

func TestResourceGroupRevisionsRejectsInvalidPagination(t *testing.T) {
	group := &domain.TaskAssetGroup{ID: 10, TaskID: 20}
	repository := &resourceWorkflowRepoStub{
		groupByID:        map[int64]*domain.TaskAssetGroup{group.ID: group},
		subjectByTask:    map[int64]domain.TaskAccessSubject{group.TaskID: {TaskID: group.TaskID}},
		revisionsByGroup: map[int64][]domain.TaskAssetGroupRevision{group.ID: {}},
	}
	svc := NewTaskResourceWorkflowService(repository, nil, nil)
	actor := multiCapabilityActor(7, nil, capabilityScope{permission: domain.PermissionAssetView, scope: domain.AccessScopeGlobal})

	result, appErr := svc.ResourceGroupRevisions(context.Background(), actor, group.ID, 0, 50)
	if result != nil || appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("invalid page result/error = %+v/%+v", result, appErr)
	}
	result, appErr = svc.ResourceGroupRevisions(context.Background(), actor, group.ID, 2, 201)
	if result != nil || appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("oversized page result/error = %+v/%+v", result, appErr)
	}
}

type capabilityScope struct {
	permission domain.PermissionCode
	scope      domain.AccessScopeMode
}

func multiCapabilityActor(id int64, departmentID *int64, capabilities ...capabilityScope) domain.RequestActor {
	view := &domain.EffectiveAccess{UserID: id}
	for index, capability := range capabilities {
		roleID := int64(index + 1)
		assignment := domain.AccessAssignment{ID: roleID, UserID: id, RoleID: roleID, ScopeMode: capability.scope, SourceType: "org_policy"}
		view.Permissions = append(view.Permissions, capability.permission)
		view.Assignments = append(view.Assignments, assignment)
		view.Sources = append(view.Sources, domain.EffectiveAccessNote{Permission: capability.permission, RoleID: roleID, SourceType: assignment.SourceType, ScopeMode: capability.scope})
	}
	return domain.RequestActor{ID: id, DepartmentID: departmentID, Permissions: view.Permissions, EffectiveAccess: view}
}

func TestListResourceGroupsUsesFileLevelPaginationForFlatMode(t *testing.T) {
	departmentID := int64(101)
	actor := scopedCapabilityActor(7, domain.PermissionAssetView, domain.AccessScopeOwnDepartment, &departmentID, nil, nil)
	repository := &resourceWorkflowRepoStub{
		listFn: func(domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64) {
			t.Fatal("group-level list must not run for flat mode")
			return nil, 0
		},
		flatListFn: func(params domain.ResourceGroupListParams) ([]domain.FlatResourceItem, int64) {
			if params.Page != 3 || params.PageSize != 2 || params.ResourceRole != domain.ResourceRoleFilterFinal {
				t.Fatalf("flat params = %+v", params)
			}
			if len(params.Access.DepartmentIDs) != 1 || params.Access.DepartmentIDs[0] != departmentID {
				t.Fatalf("flat SQL scope params = %+v", params.Access)
			}
			return []domain.FlatResourceItem{{
				GroupID:      8,
				TaskID:       18,
				TaskAssetID:  81,
				FileName:     "fifth.png",
				StorageKey:   "tasks/18/fifth.png",
				ResourceRole: domain.ResourceRoleFilterFinal,
			}}, 5
		},
	}
	svc := NewTaskResourceWorkflowService(repository, nil, nil)
	result, appErr := svc.ListResourceGroups(context.Background(), actor, domain.ResourceGroupListParams{
		Page: 3, PageSize: 2, ResourceRole: domain.ResourceRoleFilterFinal,
	})
	if appErr != nil {
		t.Fatalf("ListResourceGroups() error = %+v", appErr)
	}
	if result.ViewMode != "flat" || result.Total != 5 || len(result.FlatItems) != 1 || len(result.Items) != 0 {
		t.Fatalf("flat page = %+v", result)
	}
	if result.FlatItems[0].PreviewURL != "/v1/task-assets/81/preview" {
		t.Fatalf("view-scoped flat list preview_url = %q", result.FlatItems[0].PreviewURL)
	}
	if result.FlatItems[0].DownloadURL != "" {
		t.Fatalf("view-scoped flat list leaked download_url = %q", result.FlatItems[0].DownloadURL)
	}
}

func TestOrgPolicyScopesMatchSingleTaskAndListFiltering(t *testing.T) {
	departmentID, teamID := int64(101), int64(202)
	cases := []struct {
		name       string
		scope      domain.AccessScopeMode
		subjects   []domain.AccessScopeSubject
		wantGlobal bool
		wantDept   bool
		wantTeam   bool
	}{
		{name: "global", scope: domain.AccessScopeGlobal, wantGlobal: true},
		{name: "own department", scope: domain.AccessScopeOwnDepartment, wantDept: true},
		{name: "own team", scope: domain.AccessScopeOwnTeam, wantTeam: true},
		{name: "selected department", scope: domain.AccessScopeSelectedOrg, subjects: []domain.AccessScopeSubject{{SubjectType: domain.AccessSubjectDepartment, SubjectID: departmentID}}, wantDept: true},
		{name: "selected team", scope: domain.AccessScopeSelectedOrg, subjects: []domain.AccessScopeSubject{{SubjectType: domain.AccessSubjectTeam, SubjectID: teamID}}, wantTeam: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actor := scopedCapabilityActor(7, domain.PermissionAssetView, tc.scope, &departmentID, &teamID, tc.subjects)
			subject := domain.TaskAccessSubject{TaskID: 1, CreatorID: 8, OwnerDepartmentID: &departmentID, OwnerTeamID: &teamID}
			if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetView, subject) {
				t.Fatal("single-task effective access denied")
			}
			filter := resourceGroupAccessFilter(actor, domain.PermissionAssetView)
			if filter.Global != tc.wantGlobal || (len(filter.DepartmentIDs) > 0) != tc.wantDept || (len(filter.TeamIDs) > 0) != tc.wantTeam {
				t.Fatalf("list scope = %+v", filter)
			}
		})
	}
}

func TestBindingValidationRejectsCrossDiscriminatorAndSourceFinalOverlap(t *testing.T) {
	actorID, skuID, retouchID := int64(7), int64(22), int64(33)
	service := &taskResourceWorkflowService{repo: &resourceWorkflowRepoStub{staged: map[int64]domain.StagedTaskAssetBinding{
		1: {TaskAssetID: 1, TaskID: 10, BindingState: "staged", StagedRole: "final", StagedBy: &actorID, StagedTaskSKUItemID: &skuID},
	}}}
	err := service.validateBindingAssets(context.Background(), nil, actorID, domain.TaskAssetGroup{ID: 3, TaskID: 10, ScopeKind: domain.TaskAssetGroupScopeTask}, domain.SubmitResourceGroupInput{GroupID: 3, FinalTaskAssetIDs: []int64{1}})
	if err == nil {
		t.Fatal("task scope accepted a SKU-scoped staged file")
	}
	if appErr := validateGroupInput(domain.SubmitResourceGroupInput{GroupID: 3, Mode: domain.TaskAssetGroupModeSingle, SourceTaskAssetID: &retouchID, FinalTaskAssetIDs: []int64{retouchID}}, true); appErr == nil {
		t.Fatal("same file accepted as source and final")
	}
}

func TestDesignSubmissionAcceptsModeAndSourceButRejectsFinalOutput(t *testing.T) {
	sourceID := int64(17)
	for _, mode := range []domain.TaskAssetGroupMode{domain.TaskAssetGroupModeSingle, domain.TaskAssetGroupModeSet} {
		input := domain.SubmitResourceGroupInput{
			GroupID: 9, ExpectedGroupLockVersion: 0, Mode: mode, SourceTaskAssetID: &sourceID,
		}
		if appErr := validateDesignGroupInput(input); appErr != nil {
			t.Fatalf("mode %s rejected: %+v", mode, appErr)
		}
		input.FinalTaskAssetIDs = []int64{18}
		appErr := validateDesignGroupInput(input)
		if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
			t.Fatalf("mode %s final output error = %+v", mode, appErr)
		}
	}
}

func TestFinalOutputValidationUsesDesignerSelectedMode(t *testing.T) {
	sourceID := int64(17)
	if appErr := validateGroupInput(domain.SubmitResourceGroupInput{
		GroupID: 9, Mode: domain.TaskAssetGroupModeSingle, SourceTaskAssetID: &sourceID, FinalTaskAssetIDs: []int64{18},
	}, true); appErr != nil {
		t.Fatalf("single final rejected: %+v", appErr)
	}
	if appErr := validateGroupInput(domain.SubmitResourceGroupInput{
		GroupID: 9, Mode: domain.TaskAssetGroupModeSet, SourceTaskAssetID: &sourceID, FinalTaskAssetIDs: []int64{18},
	}, true); appErr == nil {
		t.Fatal("set mode accepted fewer than two final outputs")
	}
}

func TestAuditRevisionSourceStagePreservesInheritedDesignSource(t *testing.T) {
	designSourceID := int64(17)
	replacementSourceID := int64(18)
	current := &domain.TaskAssetGroupRevision{
		SourceTaskAssetID: &designSourceID,
		SourceStage:       domain.TaskAssetSourceDesign,
	}
	if got := auditRevisionSourceStage(current, nil); got != domain.TaskAssetSourceDesign {
		t.Fatalf("omitted audit source stage = %s, want design", got)
	}
	if got := auditRevisionSourceStage(current, &designSourceID); got != domain.TaskAssetSourceDesign {
		t.Fatalf("same audit source stage = %s, want design", got)
	}
	if got := auditRevisionSourceStage(current, &replacementSourceID); got != domain.TaskAssetSourceAudit {
		t.Fatalf("replacement audit source stage = %s, want audit", got)
	}
}

type resourceWorkflowTestTx struct{}

func (resourceWorkflowTestTx) IsTx() {}

type recordingResourceWorkflowTxRunner struct {
	committed  int
	rolledBack int
}

func (r *recordingResourceWorkflowTxRunner) RunInTx(_ context.Context, fn func(repo.Tx) error) error {
	err := fn(resourceWorkflowTestTx{})
	if err != nil {
		r.rolledBack++
		return err
	}
	r.committed++
	return nil
}

type bindingRollbackRepo struct {
	TaskResourceGroupRepository
	task                     *domain.TaskWorkflowLock
	group                    domain.TaskAssetGroup
	staged                   map[int64]domain.StagedTaskAssetBinding
	createCalls              int
	completeCalls            int
	customizationReadyErr    error
	customizationReadyChecks int
	revision                 *domain.TaskAssetGroupRevision
}

func (r *bindingRollbackRepo) StoreIdempotency(context.Context, repo.Tx, int64, int64, string, string, string, interface{}) (bool, json.RawMessage, error) {
	return true, nil, nil
}

func (r *bindingRollbackRepo) GetWorkflowForUpdate(context.Context, repo.Tx, int64) (*domain.TaskWorkflowLock, error) {
	return r.task, nil
}

func (r *bindingRollbackRepo) RequireCustomizationReadyForSubmit(context.Context, repo.Tx, int64) error {
	r.customizationReadyChecks++
	return r.customizationReadyErr
}

func (r *bindingRollbackRepo) ResetCustomizationReadyForSubmit(context.Context, repo.Tx, int64) error {
	return nil
}

func (r *bindingRollbackRepo) ListGroupsForUpdate(context.Context, repo.Tx, int64) ([]domain.TaskAssetGroup, error) {
	return []domain.TaskAssetGroup{r.group}, nil
}

func (r *bindingRollbackRepo) ExpectedResourceGroupCountForUpdate(context.Context, repo.Tx, int64, domain.TaskType) (int64, error) {
	return 1, nil
}

func (r *bindingRollbackRepo) LockGroup(context.Context, repo.Tx, int64, int64, int64) (*domain.TaskAssetGroup, error) {
	group := r.group
	return &group, nil
}

func (r *bindingRollbackRepo) ListStagedAssetsForUpdate(context.Context, repo.Tx, []int64) (map[int64]domain.StagedTaskAssetBinding, error) {
	return r.staged, nil
}

func (r *bindingRollbackRepo) GetRevisionForUpdate(context.Context, repo.Tx, int64) (*domain.TaskAssetGroupRevision, error) {
	if r.revision == nil {
		return nil, repo.ErrNotFound
	}
	copyRevision := *r.revision
	return &copyRevision, nil
}

func (r *bindingRollbackRepo) CreateRevision(context.Context, repo.Tx, domain.TaskAssetGroup, domain.SubmitResourceGroupInput, domain.TaskAssetGroupRevisionStatus, domain.TaskAssetSourceStage, int64, string) (int64, error) {
	r.createCalls++
	return 100, nil
}

func (r *bindingRollbackRepo) CompleteIdempotency(context.Context, repo.Tx, int64, int64, string, string, interface{}) error {
	r.completeCalls++
	return nil
}

func TestSubmitDesignRollsBackEntireTransactionWhenFinalOutputIsSentDuringDesign(t *testing.T) {
	actorID := int64(7)
	sourceID, finalID := int64(1), int64(2)
	repository := &bindingRollbackRepo{
		task:  &domain.TaskWorkflowLock{TaskID: 10, TaskType: domain.TaskTypeNewProductDevelopment, Status: domain.TaskStatusInProgress, WorkflowRevision: 3, CreatorID: actorID},
		group: domain.TaskAssetGroup{ID: 20, TaskID: 10, ScopeKind: domain.TaskAssetGroupScopeTask, LockVersion: 0},
		staged: map[int64]domain.StagedTaskAssetBinding{
			sourceID: {TaskAssetID: sourceID, TaskID: 10, BindingState: "staged", StagedRole: "source", StagedBy: &actorID},
			finalID:  {TaskAssetID: finalID, TaskID: 10, BindingState: "staged", StagedRole: "final", StagedBy: &actorID},
		},
	}
	runner := &recordingResourceWorkflowTxRunner{}
	svc := NewTaskResourceWorkflowService(repository, runner, nil)
	_, appErr := svc.SubmitDesign(context.Background(), 10, globalCapabilityActor(actorID, domain.PermissionTaskDesignSubmit), domain.SubmitDesignV2Request{
		ExpectedWorkflowRevision: 3, IdempotencyKey: "binding-rollback",
		Groups: []domain.SubmitResourceGroupInput{{GroupID: 20, ExpectedGroupLockVersion: 0, Mode: domain.TaskAssetGroupModeSingle, SourceTaskAssetID: &sourceID, FinalTaskAssetIDs: []int64{finalID}}},
	})
	if appErr == nil || runner.rolledBack != 1 || runner.committed != 0 || repository.createCalls != 0 || repository.completeCalls != 0 {
		t.Fatalf("error/tx/create/complete = %+v/%d/%d/%d/%d", appErr, runner.rolledBack, runner.committed, repository.createCalls, repository.completeCalls)
	}
}

func TestAuditDecisionRejectsModeChangeAndBoundDesignFinals(t *testing.T) {
	actorID, sourceID, finalID, revisionID := int64(7), int64(1), int64(2), int64(90)
	base := func() (*bindingRollbackRepo, *recordingResourceWorkflowTxRunner) {
		groupID := int64(20)
		return &bindingRollbackRepo{
			task:     &domain.TaskWorkflowLock{TaskID: 10, TaskType: domain.TaskTypeNewProductDevelopment, Status: domain.TaskStatusPendingAudit, WorkflowRevision: 4, CreatorID: actorID},
			group:    domain.TaskAssetGroup{ID: groupID, TaskID: 10, ScopeKind: domain.TaskAssetGroupScopeTask, LockVersion: 2, WorkingRevisionID: &revisionID},
			revision: &domain.TaskAssetGroupRevision{ID: revisionID, GroupID: groupID, Status: domain.TaskAssetGroupRevisionSubmitted, Mode: domain.TaskAssetGroupModeSingle, SourceTaskAssetID: &sourceID},
			staged: map[int64]domain.StagedTaskAssetBinding{
				sourceID: {TaskAssetID: sourceID, TaskID: 10, BindingState: "bound", BoundGroupID: &groupID, BoundRole: "source"},
				finalID:  {TaskAssetID: finalID, TaskID: 10, BindingState: "bound", BoundGroupID: &groupID, BoundRole: "final"},
			},
		}, &recordingResourceWorkflowTxRunner{}
	}

	t.Run("auditor cannot change designer mode", func(t *testing.T) {
		repository, runner := base()
		svc := NewTaskResourceWorkflowService(repository, runner, nil)
		_, appErr := svc.AuditDecision(context.Background(), 10, globalCapabilityActor(actorID, domain.PermissionTaskAuditDecision), domain.AuditDecisionRequest{
			Decision: domain.TaskAuditDecisionApprove, ExpectedWorkflowRevision: 4, IdempotencyKey: "audit-mode-mismatch",
			Groups: []domain.SubmitResourceGroupInput{{GroupID: 20, ExpectedGroupLockVersion: 2, Mode: domain.TaskAssetGroupModeSet, FinalTaskAssetIDs: []int64{finalID, finalID + 1}}},
		})
		if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest || repository.createCalls != 0 || runner.rolledBack != 1 {
			t.Fatalf("error/create/rollback = %+v/%d/%d", appErr, repository.createCalls, runner.rolledBack)
		}
	})

	t.Run("audit finals must be newly staged by current auditor", func(t *testing.T) {
		repository, runner := base()
		svc := NewTaskResourceWorkflowService(repository, runner, nil)
		_, appErr := svc.AuditDecision(context.Background(), 10, globalCapabilityActor(actorID, domain.PermissionTaskAuditDecision), domain.AuditDecisionRequest{
			Decision: domain.TaskAuditDecisionApprove, ExpectedWorkflowRevision: 4, IdempotencyKey: "audit-bound-final",
			Groups: []domain.SubmitResourceGroupInput{{GroupID: 20, ExpectedGroupLockVersion: 2, Mode: domain.TaskAssetGroupModeSingle, FinalTaskAssetIDs: []int64{finalID}}},
		})
		if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest || repository.createCalls != 0 || runner.rolledBack != 1 {
			t.Fatalf("error/create/rollback = %+v/%d/%d", appErr, repository.createCalls, runner.rolledBack)
		}
	})
}

func TestSubmitDesignRejectsCustomizationBeforeReadyForSubmit(t *testing.T) {
	actorID := int64(7)
	repository := &bindingRollbackRepo{
		task: &domain.TaskWorkflowLock{
			TaskID: 10, TaskType: domain.TaskTypeNewProductDevelopment, Status: domain.TaskStatusInProgress,
			WorkflowRevision: 3, CreatorID: actorID, Customization: true,
		},
		customizationReadyErr: domain.NewAppError(domain.ErrCodeInvalidStateTransition, "定制任务尚未完成设计准备", nil),
	}
	runner := &recordingResourceWorkflowTxRunner{}
	svc := NewTaskResourceWorkflowService(repository, runner, nil)
	_, appErr := svc.SubmitDesign(context.Background(), 10, globalCapabilityActor(actorID, domain.PermissionTaskDesignSubmit), domain.SubmitDesignV2Request{
		ExpectedWorkflowRevision: 3, IdempotencyKey: "customization-not-ready",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("SubmitDesign() error = %+v", appErr)
	}
	if repository.customizationReadyChecks != 1 || repository.createCalls != 0 || repository.completeCalls != 0 {
		t.Fatalf("ready/create/complete calls = %d/%d/%d", repository.customizationReadyChecks, repository.createCalls, repository.completeCalls)
	}
	if runner.rolledBack != 1 || runner.committed != 0 {
		t.Fatalf("transaction rolled back/committed = %d/%d", runner.rolledBack, runner.committed)
	}
}

type finalizerEventRepo struct {
	repo.TaskEventRepo
	eventTypes []string
}

func (r *finalizerEventRepo) Append(_ context.Context, _ repo.Tx, _ int64, eventType string, _ *int64, _ interface{}) (*domain.TaskEvent, error) {
	r.eventTypes = append(r.eventTypes, eventType)
	return &domain.TaskEvent{EventType: eventType}, nil
}

type finalizerRepoStub struct {
	TaskResourceGroupRepository
	revision      domain.TaskAssetGroupRevision
	finalizeCalls int
	enqueueCalls  int
}

func (r *finalizerRepoStub) GetRevisionForUpdate(context.Context, repo.Tx, int64) (*domain.TaskAssetGroupRevision, error) {
	copyRevision := r.revision
	return &copyRevision, nil
}
func (r *finalizerRepoStub) FinalizeGroup(context.Context, repo.Tx, int64, int64, int64, int64) error {
	r.finalizeCalls++
	return nil
}
func (r *finalizerRepoStub) CompleteModules(context.Context, repo.Tx, int64) error { return nil }
func (r *finalizerRepoStub) CASTaskStatus(_ context.Context, _ repo.Tx, _ int64, expectedRevision int64, _, _ domain.TaskStatus, _ bool) (int64, error) {
	return expectedRevision + 1, nil
}
func (r *finalizerRepoStub) EnqueueTaskFinalized(context.Context, repo.Tx, int64, int64, bool, bool) error {
	r.enqueueCalls++
	return nil
}

func TestTaskFinalizerUsesAuthoritativeTaskClosedEvent(t *testing.T) {
	sourceID := int64(70)
	repository := &finalizerRepoStub{revision: domain.TaskAssetGroupRevision{
		ID: 80, Status: domain.TaskAssetGroupRevisionSubmitted, Mode: domain.TaskAssetGroupModeSingle,
		SourceTaskAssetID: &sourceID, Items: []domain.TaskAssetGroupRevisionItem{{ID: 90, TaskAssetID: 91}},
	}}
	events := &finalizerEventRepo{}
	finalizer := NewTaskFinalizer(repository, events)
	next, err := finalizer.FinalizeInTx(context.Background(), resourceWorkflowTestTx{}, &domain.TaskWorkflowLock{
		TaskID: 10, TaskType: domain.TaskTypeNewProductDevelopment, Status: domain.TaskStatusPendingAudit, WorkflowRevision: 4,
	}, []domain.TaskAssetGroup{{ID: 20, WorkingRevisionID: int64PtrForResourceWorkflowTest(80), LockVersion: 2}}, FinalizeModeDesignAudit, 7)
	if err != nil {
		t.Fatalf("FinalizeInTx() error = %v", err)
	}
	if next != 5 || repository.finalizeCalls != 1 || repository.enqueueCalls != 1 {
		t.Fatalf("next/finalize/enqueue = %d/%d/%d", next, repository.finalizeCalls, repository.enqueueCalls)
	}
	if len(events.eventTypes) != 1 || events.eventTypes[0] != domain.TaskEventClosed {
		t.Fatalf("event types = %+v", events.eventTypes)
	}
}

func int64PtrForResourceWorkflowTest(value int64) *int64 { return &value }

func TestBatchDownloadResourceGroupsUsesDownloadCapabilityAndTaskScope(t *testing.T) {
	departmentID, otherDepartmentID := int64(101), int64(202)
	group := &domain.TaskAssetGroup{
		ID: 10, TaskID: 20, SKUCode: "SKU-20",
		FinalizedRevision: &domain.TaskAssetGroupRevision{ID: 30, Items: []domain.TaskAssetGroupRevisionItem{{
			ID: 40, SortOrder: 1, File: &domain.TaskResourceFile{TaskAssetID: 50, FileName: "final.png", StorageKey: "tasks/20/final.png"},
		}}},
	}
	repository := &resourceWorkflowRepoStub{
		groupByID: map[int64]*domain.TaskAssetGroup{group.ID: group},
		subjectByTask: map[int64]domain.TaskAccessSubject{
			group.TaskID: {TaskID: group.TaskID, CreatorID: 8, OwnerDepartmentID: &departmentID},
		},
	}
	svc := NewTaskResourceWorkflowService(repository, nil, nil)

	t.Run("download-only actor succeeds", func(t *testing.T) {
		manifest, appErr := svc.BatchDownloadResourceGroups(context.Background(), globalCapabilityActor(7, domain.PermissionAssetDownload), domain.ResourceGroupBatchDownloadRequest{GroupIDs: []int64{group.ID}})
		if appErr != nil || len(manifest.Items) != 1 || manifest.Items[0].RevisionItemID != 40 {
			t.Fatalf("manifest/error = %+v/%+v", manifest, appErr)
		}
	})

	t.Run("view-only actor is denied", func(t *testing.T) {
		_, appErr := svc.BatchDownloadResourceGroups(context.Background(), globalCapabilityActor(7, domain.PermissionAssetView), domain.ResourceGroupBatchDownloadRequest{GroupIDs: []int64{group.ID}})
		if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
			t.Fatalf("view-only error = %+v", appErr)
		}
	})

	t.Run("out-of-scope download actor is denied", func(t *testing.T) {
		actor := scopedCapabilityActor(7, domain.PermissionAssetDownload, domain.AccessScopeOwnDepartment, &otherDepartmentID, nil, nil)
		_, appErr := svc.BatchDownloadResourceGroups(context.Background(), actor, domain.ResourceGroupBatchDownloadRequest{GroupIDs: []int64{group.ID}})
		if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
			t.Fatalf("out-of-scope error = %+v", appErr)
		}
	})
}

func globalCapabilityActor(id int64, permission domain.PermissionCode) domain.RequestActor {
	return scopedCapabilityActor(id, permission, domain.AccessScopeGlobal, nil, nil, nil)
}

func scopedCapabilityActor(id int64, permission domain.PermissionCode, scope domain.AccessScopeMode, departmentID, teamID *int64, subjects []domain.AccessScopeSubject) domain.RequestActor {
	assignment := domain.AccessAssignment{ID: 1, UserID: id, RoleID: 9, ScopeMode: scope, Subjects: subjects, SourceType: "org_policy"}
	view := &domain.EffectiveAccess{
		UserID: id, Permissions: []domain.PermissionCode{permission}, Assignments: []domain.AccessAssignment{assignment},
		Sources: []domain.EffectiveAccessNote{{Permission: permission, RoleID: assignment.RoleID, SourceType: assignment.SourceType, ScopeMode: scope}},
	}
	return domain.RequestActor{ID: id, DepartmentID: departmentID, TeamID: teamID, Permissions: view.Permissions, EffectiveAccess: view}
}
