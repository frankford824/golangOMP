package asset_lifecycle

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestDeleteRepairsMissingLifecycleModuleAndWritesEvent(t *testing.T) {
	assetID := int64(8801)
	departmentID := int64(41)
	storageKey := "tasks/RW-LEGACY/delivery.psd"
	row := &repo.TaskAssetSearchRow{
		Asset: &domain.TaskAsset{
			ID:              9901,
			TaskID:          7701,
			AssetID:         &assetID,
			AssetType:       domain.TaskAssetTypeDelivery,
			SourceModuleKey: domain.ModuleKeyCustomization,
			StorageKey:      &storageKey,
		},
		Task: &domain.Task{ID: 7701, CreatorID: 404, OwnerDepartmentID: &departmentID, TaskStatus: domain.TaskStatusInProgress},
	}
	search := &deleteSearchRepoStub{current: row, versions: []*repo.TaskAssetSearchRow{row}}
	lifecycle := &deleteLifecycleRepoStub{current: row, resolvedModuleID: 6601}
	deleter := &recordingObjectDeleter{enabled: true}
	svc := NewService(search, lifecycle, fakeTxRunner{}, deleter).WithNow(func() time.Time {
		return time.Date(2026, 7, 13, 6, 0, 0, 0, time.UTC)
	})

	actor := assetManageActor(303, domain.AccessScopeOwnDepartment, &departmentID)
	if appErr := svc.Delete(context.Background(), actor, assetID, "文件错误"); appErr != nil {
		t.Fatalf("Delete() appErr = %+v", appErr)
	}
	if lifecycle.resolvedTaskID != row.Task.ID || lifecycle.resolvedModuleKey != domain.ModuleKeyCustomization {
		t.Fatalf("resolved module = task:%d key:%q", lifecycle.resolvedTaskID, lifecycle.resolvedModuleKey)
	}
	if !lifecycle.softDeleted {
		t.Fatal("Delete() did not soft-delete the asset")
	}
	if got := lifecycle.mutationOrder; len(got) != 2 || got[0] != "outbox" || got[1] != "soft_delete" {
		t.Fatalf("mutation order = %v, want [outbox soft_delete]", got)
	}
	if len(deleter.deletedKeys) != 0 {
		t.Fatalf("request path physically deleted objects before commit: %v", deleter.deletedKeys)
	}
	if search.currentCalls != 0 || search.versionCalls != 0 {
		t.Fatalf("delete performed pre-transaction search reads: current=%d versions=%d", search.currentCalls, search.versionCalls)
	}
	if lifecycle.eventModuleID != lifecycle.resolvedModuleID || lifecycle.eventType != "asset_deleted_by_admin" {
		t.Fatalf("event = module:%d type:%q", lifecycle.eventModuleID, lifecycle.eventType)
	}
}

func TestDeleteRequiresReopenForFinalizedTaskRegardlessOfLegacyRole(t *testing.T) {
	for _, status := range []domain.TaskStatus{domain.TaskStatusCompleted, domain.TaskStatusArchived} {
		t.Run(string(status), func(t *testing.T) {
			assetID := int64(8802)
			moduleID := int64(6602)
			row := &repo.TaskAssetSearchRow{
				Asset: &domain.TaskAsset{ID: 9902, TaskID: 7702, AssetID: &assetID, AssetType: domain.TaskAssetTypeDelivery, SourceTaskModuleID: &moduleID},
				Task:  &domain.Task{ID: 7702, CreatorID: 404, TaskStatus: status},
			}
			lifecycle := &deleteLifecycleRepoStub{current: row}
			svc := NewService(&deleteSearchRepoStub{}, lifecycle, fakeTxRunner{}, &recordingObjectDeleter{enabled: true})
			actor := assetManageActor(303, domain.AccessScopeGlobal, nil)
			actor.Roles = []domain.Role{domain.RoleSuperAdmin, domain.RoleAuditA, domain.RoleAssetManager}

			appErr := svc.Delete(context.Background(), actor, assetID, "文件错误")
			if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
				t.Fatalf("Delete() appErr = %+v, want invalid state", appErr)
			}
			details, _ := appErr.Details.(map[string]interface{})
			if details["deny_code"] != "finalized_resource_requires_reopen" {
				t.Fatalf("deny_code = %v", details["deny_code"])
			}
			if lifecycle.enqueued || lifecycle.softDeleted {
				t.Fatalf("finalized resource mutated: enqueued=%v soft_deleted=%v", lifecycle.enqueued, lifecycle.softDeleted)
			}
		})
	}
}

func TestDeleteRequiresAssetManageInStableTaskScope(t *testing.T) {
	assetID := int64(8803)
	moduleID := int64(6603)
	taskDepartmentID := int64(51)
	actorDepartmentID := int64(52)
	row := &repo.TaskAssetSearchRow{
		Asset: &domain.TaskAsset{ID: 9903, TaskID: 7703, AssetID: &assetID, AssetType: domain.TaskAssetTypeSource, SourceTaskModuleID: &moduleID},
		Task:  &domain.Task{ID: 7703, CreatorID: 404, OwnerDepartmentID: &taskDepartmentID, TaskStatus: domain.TaskStatusInProgress},
	}
	for _, tc := range []struct {
		name  string
		actor domain.RequestActor
	}{
		{name: "legacy role without effective access", actor: domain.RequestActor{ID: 303, Roles: []domain.Role{domain.RoleSuperAdmin, domain.RoleAssetManager}}},
		{name: "asset manage outside stable department", actor: assetManageActor(303, domain.AccessScopeOwnDepartment, &actorDepartmentID)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lifecycle := &deleteLifecycleRepoStub{current: row}
			svc := NewService(&deleteSearchRepoStub{}, lifecycle, fakeTxRunner{}, nil)
			appErr := svc.Delete(context.Background(), tc.actor, assetID, "文件错误")
			if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
				t.Fatalf("Delete() appErr = %+v, want permission denied", appErr)
			}
			if lifecycle.enqueued || lifecycle.softDeleted {
				t.Fatalf("out-of-scope resource mutated: enqueued=%v soft_deleted=%v", lifecycle.enqueued, lifecycle.softDeleted)
			}
		})
	}
}

func TestDeleteRejectsBoundOrHistoricallyReferencedResourceAfterReopen(t *testing.T) {
	assetID := int64(8804)
	moduleID := int64(6604)
	row := &repo.TaskAssetSearchRow{
		Asset: &domain.TaskAsset{ID: 9904, TaskID: 7704, AssetID: &assetID, AssetType: domain.TaskAssetTypeSource, SourceTaskModuleID: &moduleID},
		Task:  &domain.Task{ID: 7704, CreatorID: 404, TaskStatus: domain.TaskStatusInProgress},
	}
	for _, tc := range []struct {
		name     string
		guard    *repo.TaskAssetDeleteGuard
		denyCode string
	}{
		{
			name:     "derived resource is bound",
			guard:    &repo.TaskAssetDeleteGuard{DesignAssetIDs: []int64{8804, 8805}, TaskAssetIDs: []int64{9904, 9905}, AllStagedUnbound: false},
			denyCode: "asset_delete_requires_staged_unbound",
		},
		{
			name: "derived resource remains in old finalized revision after reopen",
			guard: &repo.TaskAssetDeleteGuard{
				DesignAssetIDs: []int64{8804, 8805}, TaskAssetIDs: []int64{9904, 9905}, AllStagedUnbound: true, RevisionReferenceIDs: []int64{4401},
			},
			denyCode: "asset_delete_resource_referenced",
		},
		{
			name: "derived resource has publication pin",
			guard: &repo.TaskAssetDeleteGuard{
				DesignAssetIDs: []int64{8804, 8805}, TaskAssetIDs: []int64{9904, 9905}, AllStagedUnbound: true, PublicationPinIDs: []int64{5501},
			},
			denyCode: "asset_delete_resource_referenced",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lifecycle := &deleteLifecycleRepoStub{current: row, deleteGuard: tc.guard}
			svc := NewService(&deleteSearchRepoStub{}, lifecycle, fakeTxRunner{}, nil)
			appErr := svc.Delete(context.Background(), assetManageActor(303, domain.AccessScopeGlobal, nil), assetID, "文件错误")
			if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
				t.Fatalf("Delete() appErr = %+v, want invalid state", appErr)
			}
			details, _ := appErr.Details.(map[string]interface{})
			if details["deny_code"] != tc.denyCode {
				t.Fatalf("deny_code = %v, want %s", details["deny_code"], tc.denyCode)
			}
			if lifecycle.enqueued || lifecycle.softDeleted {
				t.Fatalf("protected resource mutated: enqueued=%v soft_deleted=%v", lifecycle.enqueued, lifecycle.softDeleted)
			}
		})
	}
}

func TestLifecycleSourceModuleKeyDerivesMissingLegacyValues(t *testing.T) {
	tests := []struct {
		name  string
		asset *domain.TaskAsset
		task  *domain.Task
		want  string
	}{
		{name: "reference", asset: &domain.TaskAsset{AssetType: domain.TaskAssetTypeReference}, task: &domain.Task{}, want: domain.ModuleKeyBasicInfo},
		{name: "customization", asset: &domain.TaskAsset{AssetType: domain.TaskAssetTypeDelivery}, task: &domain.Task{CustomizationRequired: true}, want: domain.ModuleKeyCustomization},
		{name: "retouch", asset: &domain.TaskAsset{AssetType: domain.TaskAssetTypeSource}, task: &domain.Task{TaskType: domain.TaskTypeRetouchTask}, want: domain.ModuleKeyRetouch},
		{name: "explicit", asset: &domain.TaskAsset{AssetType: domain.TaskAssetTypeDelivery, SourceModuleKey: domain.ModuleKeyAudit}, task: &domain.Task{}, want: domain.ModuleKeyAudit},
		{name: "unknown falls back", asset: &domain.TaskAsset{AssetType: domain.TaskAssetTypeDelivery, SourceModuleKey: "legacy_unknown"}, task: &domain.Task{}, want: domain.ModuleKeyDesign},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lifecycleSourceModuleKey(tt.asset, tt.task); got != tt.want {
				t.Fatalf("lifecycleSourceModuleKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

type deleteSearchRepoStub struct {
	current      *repo.TaskAssetSearchRow
	versions     []*repo.TaskAssetSearchRow
	currentCalls int
	versionCalls int
}

func (s *deleteSearchRepoStub) Search(context.Context, domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	return nil, 0, nil
}

func (s *deleteSearchRepoStub) GetCurrentByAssetID(context.Context, int64) (*repo.TaskAssetSearchRow, error) {
	s.currentCalls++
	return s.current, nil
}
func (s *deleteSearchRepoStub) ListCurrentByAssetIDs(context.Context, []int64) ([]*repo.TaskAssetSearchRow, error) {
	return nil, nil
}
func (s *deleteSearchRepoStub) ListVersionsByAssetID(context.Context, int64) ([]*repo.TaskAssetSearchRow, error) {
	s.versionCalls++
	return s.versions, nil
}
func (s *deleteSearchRepoStub) GetVersion(context.Context, int64, int64) (*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

type deleteLifecycleRepoStub struct {
	current           *repo.TaskAssetSearchRow
	resolvedModuleID  int64
	resolvedTaskID    int64
	resolvedModuleKey string
	softDeleted       bool
	eventModuleID     int64
	eventType         domain.ModuleEventType
	enqueued          bool
	mutationOrder     []string
	deleteGuard       *repo.TaskAssetDeleteGuard
}

func (s *deleteLifecycleRepoStub) Archive(context.Context, repo.Tx, repo.TaskAssetLifecycleUpdate) error {
	return nil
}
func (s *deleteLifecycleRepoStub) Restore(context.Context, repo.Tx, repo.TaskAssetLifecycleUpdate) error {
	return nil
}
func (s *deleteLifecycleRepoStub) LockGenericDeleteGuard(context.Context, repo.Tx, int64) (*repo.TaskAssetDeleteGuard, error) {
	if s.deleteGuard != nil {
		return s.deleteGuard, nil
	}
	return &repo.TaskAssetDeleteGuard{TaskAssetIDs: []int64{9901}, AllStagedUnbound: true}, nil
}
func (s *deleteLifecycleRepoStub) LockCleanupObjectIDs(context.Context, repo.Tx, int64) ([]int64, error) {
	return nil, nil
}
func (s *deleteLifecycleRepoStub) EnqueueObjectDeletions(context.Context, repo.Tx, []int64) error {
	s.enqueued = true
	s.mutationOrder = append(s.mutationOrder, "outbox")
	return nil
}
func (s *deleteLifecycleRepoStub) SoftDelete(context.Context, repo.Tx, repo.TaskAssetLifecycleUpdate) error {
	s.softDeleted = true
	s.mutationOrder = append(s.mutationOrder, "soft_delete")
	return nil
}
func (s *deleteLifecycleRepoStub) MarkAutoCleaned(context.Context, repo.Tx, int64, time.Time) error {
	return nil
}
func (s *deleteLifecycleRepoStub) ListEligibleForCleanup(context.Context, time.Time, int) ([]*repo.TaskAssetCleanupCandidate, error) {
	return nil, nil
}
func (s *deleteLifecycleRepoStub) GetCurrentForUpdate(context.Context, repo.Tx, int64) (*repo.TaskAssetSearchRow, error) {
	return s.current, nil
}
func (s *deleteLifecycleRepoStub) InsertLifecycleEvent(_ context.Context, _ repo.Tx, moduleID int64, eventType domain.ModuleEventType, _ *int64, _ interface{}) error {
	s.eventModuleID = moduleID
	s.eventType = eventType
	return nil
}
func (s *deleteLifecycleRepoStub) ResolveOrCreateLifecycleEventModule(_ context.Context, _ repo.Tx, taskID int64, moduleKey string) (int64, error) {
	s.resolvedTaskID = taskID
	s.resolvedModuleKey = moduleKey
	return s.resolvedModuleID, nil
}

type recordingObjectDeleter struct {
	enabled     bool
	deletedKeys []string
}

func (d *recordingObjectDeleter) Enabled() bool { return d.enabled }

func (d *recordingObjectDeleter) DeleteObject(_ context.Context, key string) error {
	d.deletedKeys = append(d.deletedKeys, key)
	return nil
}

func assetManageActor(id int64, scope domain.AccessScopeMode, departmentID *int64) domain.RequestActor {
	assignment := domain.AccessAssignment{ID: 1, UserID: id, RoleID: 901, RoleCode: "asset_manager", ScopeMode: scope, SourceType: "direct"}
	effective := &domain.EffectiveAccess{
		UserID:      id,
		Permissions: []domain.PermissionCode{domain.PermissionAssetManage},
		Assignments: []domain.AccessAssignment{assignment},
		Sources: []domain.EffectiveAccessNote{{
			Permission: domain.PermissionAssetManage,
			RoleID:     assignment.RoleID,
			RoleCode:   assignment.RoleCode,
			SourceType: assignment.SourceType,
			ScopeMode:  assignment.ScopeMode,
		}},
	}
	return domain.RequestActor{ID: id, DepartmentID: departmentID, Permissions: effective.Permissions, EffectiveAccess: effective}
}
