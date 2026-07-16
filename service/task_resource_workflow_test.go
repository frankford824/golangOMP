package service

import (
	"context"
	"encoding/json"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

type resourceWorkflowRepoStub struct {
	TaskResourceGroupRepository
	workflow      *domain.TaskWorkflowLock
	groups        []domain.TaskAssetGroup
	expected      int64
	listFn        func(domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64)
	staged        map[int64]domain.StagedTaskAssetBinding
	workflowReads int
	groupReads    int
	groupByID     map[int64]*domain.TaskAssetGroup
	subjectByTask map[int64]domain.TaskAccessSubject
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
	return append([]domain.TaskAssetGroup(nil), s.groups...), nil
}

func (s *resourceWorkflowRepoStub) ListResourceGroups(_ context.Context, params domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64, error) {
	items, total := s.listFn(params)
	return items, total, nil
}

func (s *resourceWorkflowRepoStub) ListStagedAssetsForUpdate(context.Context, repo.Tx, []int64) (map[int64]domain.StagedTaskAssetBinding, error) {
	return s.staged, nil
}

func (s *resourceWorkflowRepoStub) GetResourceGroup(_ context.Context, groupID int64) (*domain.TaskAssetGroup, error) {
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

func (r *bindingRollbackRepo) CreateRevision(context.Context, repo.Tx, domain.TaskAssetGroup, domain.SubmitResourceGroupInput, domain.TaskAssetGroupRevisionStatus, domain.TaskAssetSourceStage, int64, string) (int64, error) {
	r.createCalls++
	return 100, nil
}

func (r *bindingRollbackRepo) CompleteIdempotency(context.Context, repo.Tx, int64, int64, string, string, interface{}) error {
	r.completeCalls++
	return nil
}

func TestSubmitDesignRollsBackEntireTransactionOnScopedBindingFailure(t *testing.T) {
	actorID, skuID := int64(7), int64(22)
	sourceID, finalID := int64(1), int64(2)
	repository := &bindingRollbackRepo{
		task:  &domain.TaskWorkflowLock{TaskID: 10, TaskType: domain.TaskTypeNewProductDevelopment, Status: domain.TaskStatusInProgress, WorkflowRevision: 3, CreatorID: actorID},
		group: domain.TaskAssetGroup{ID: 20, TaskID: 10, ScopeKind: domain.TaskAssetGroupScopeTask, LockVersion: 0},
		staged: map[int64]domain.StagedTaskAssetBinding{
			sourceID: {TaskAssetID: sourceID, TaskID: 10, BindingState: "staged", StagedRole: "source", StagedBy: &actorID},
			finalID:  {TaskAssetID: finalID, TaskID: 10, BindingState: "staged", StagedRole: "final", StagedBy: &actorID, StagedTaskSKUItemID: &skuID},
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
