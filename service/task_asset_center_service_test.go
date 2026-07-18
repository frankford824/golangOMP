package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"workflow/domain"
	"workflow/repo"
)

func taskAssetMutationTestContext() context.Context {
	const (
		actorID = int64(99001)
		roleID  = int64(99002)
	)
	permissions := []domain.PermissionCode{
		domain.PermissionTaskDesignSubmit,
		domain.PermissionTaskAuditDecision,
		domain.PermissionAssetView,
		domain.PermissionAssetManage,
	}
	assignment := domain.AccessAssignment{ID: 1, UserID: actorID, RoleID: roleID, ScopeMode: domain.AccessScopeGlobal, SourceType: "direct"}
	sources := make([]domain.EffectiveAccessNote, 0, len(permissions))
	for _, permission := range permissions {
		sources = append(sources, domain.EffectiveAccessNote{Permission: permission, RoleID: roleID, SourceType: "direct", ScopeMode: domain.AccessScopeGlobal})
	}
	effective := &domain.EffectiveAccess{UserID: actorID, Permissions: permissions, Assignments: []domain.AccessAssignment{assignment}, Sources: sources}
	return domain.WithRequestActor(context.Background(), domain.RequestActor{ID: actorID, Permissions: permissions, EffectiveAccess: effective})
}

func taskAssetPreviewTestContext(actorID int64, permission domain.PermissionCode, scope domain.AccessScopeMode) context.Context {
	roleID := actorID + 100000
	assignment := domain.AccessAssignment{ID: roleID, UserID: actorID, RoleID: roleID, ScopeMode: scope, SourceType: "direct"}
	effective := &domain.EffectiveAccess{
		UserID:      actorID,
		Permissions: []domain.PermissionCode{permission},
		Assignments: []domain.AccessAssignment{assignment},
		Sources: []domain.EffectiveAccessNote{{
			Permission: permission,
			RoleID:     roleID,
			SourceType: "direct",
			ScopeMode:  scope,
		}},
	}
	return domain.WithRequestActor(context.Background(), domain.RequestActor{ID: actorID, Permissions: effective.Permissions, EffectiveAccess: effective})
}

func TestTaskAssetCenterServiceCreateAndCompleteMultipartUploadSession(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2001, TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)
	}

	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2001,
		CreatedBy:    501,
		AssetType:    domain.TaskAssetTypeOriginal,
		Filename:     "hero.psd",
		ExpectedSize: uploadRequestInt64Ptr(12345),
		MimeType:     "image/vnd.adobe.photoshop",
		FileHash:     "hash-hero",
		Remark:       "prepare source upload",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}
	if createResult.Session == nil || createResult.Session.UploadMode != domain.DesignAssetUploadModeMultipart {
		t.Fatalf("CreateMultipartUploadSession() session = %+v", createResult.Session)
	}
	if createResult.Remote == nil || createResult.Remote.UploadID == "" {
		t.Fatalf("CreateMultipartUploadSession() remote = %+v", createResult.Remote)
	}
	if createResult.Remote.Headers != nil {
		t.Fatalf("CreateMultipartUploadSession() headers = %+v, want no internal auth headers for browser", createResult.Remote.Headers)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 14, 9, 30, 0, 0, time.UTC)
	}
	completeResult, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2001,
		SessionID:   createResult.Session.ID,
		CompletedBy: 501,
		Remark:      "upload recorded",
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession() unexpected error: %+v", appErr)
	}
	if completeResult.Asset == nil || completeResult.Asset.CurrentVersionID == nil {
		t.Fatalf("CompleteUploadSession() asset = %+v", completeResult.Asset)
	}
	if completeResult.Version == nil || completeResult.Version.VersionNo != 1 {
		t.Fatalf("CompleteUploadSession() version = %+v", completeResult.Version)
	}
	if completeResult.Version.UploadStatus != domain.DesignAssetUploadStatusUploaded {
		t.Fatalf("CompleteUploadSession() upload_status = %s", completeResult.Version.UploadStatus)
	}
	if completeResult.Version.AssetType != domain.TaskAssetTypeSource || !completeResult.Version.IsSourceFile {
		t.Fatalf("CompleteUploadSession() source semantics = %+v", completeResult.Version)
	}
	if completeResult.Version.AccessPolicy != domain.DesignAssetAccessPolicySourceControlled {
		t.Fatalf("CompleteUploadSession() source access policy = %+v", completeResult.Version)
	}
	if completeResult.Session.SessionStatus != domain.DesignAssetSessionStatusCompleted {
		t.Fatalf("CompleteUploadSession() session = %+v", completeResult.Session)
	}
	if uploadClient.completeCalls != 0 {
		t.Fatalf("CompleteUploadSession() remote complete calls = %d, want 0", uploadClient.completeCalls)
	}
	if uploadClient.getFileMetaCalls == 0 {
		t.Fatalf("CompleteUploadSession() get file meta calls = %d, want > 0", uploadClient.getFileMetaCalls)
	}
	if len(taskEventRepo.events) != 3 || taskEventRepo.events[0].EventType != domain.TaskEventAssetUploadSessionCreated || taskEventRepo.events[1].EventType != domain.TaskEventAssetVersionCreated || taskEventRepo.events[2].EventType != domain.TaskEventAssetUploadSessionCompleted {
		t.Fatalf("task events = %+v", taskEventRepo.events)
	}
}

func TestTaskAssetCenterServiceCompletedTaskRequiresReopenBeforeAnyResourceReplacement(t *testing.T) {
	const (
		taskID     = int64(2099)
		uploaderID = int64(509)
	)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:              taskID,
		TaskStatus:      domain.TaskStatusCompleted,
		OwnerDepartment: string(domain.DepartmentOperations),
		OwnerOrgTeam:    "淘系运营三部",
	})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	moduleRepo := newStep04TaskModuleRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted

	assetID, err := designAssetRepo.Create(taskAssetMutationTestContext(), nil, &domain.DesignAsset{
		TaskID:    taskID,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeDelivery,
		CreatedBy: uploaderID,
	})
	if err != nil {
		t.Fatalf("create design asset: %v", err)
	}
	oldVersionNo := 1
	oldVersionID, err := taskAssetRepo.Create(taskAssetMutationTestContext(), nil, &domain.TaskAsset{
		TaskID:           taskID,
		AssetID:          &assetID,
		AssetType:        domain.TaskAssetTypeDelivery,
		VersionNo:        1,
		AssetVersionNo:   &oldVersionNo,
		FileName:         "old.psd",
		FlowReviewStatus: domain.TaskAssetFlowReviewStatusApproved,
	})
	if err != nil {
		t.Fatalf("create old asset version: %v", err)
	}
	if err := designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), nil, assetID, &oldVersionID); err != nil {
		t.Fatalf("set old current version: %v", err)
	}

	svc := NewTaskAssetCenterService(
		taskRepo,
		designAssetRepo,
		taskAssetRepo,
		uploadRequestRepo,
		storageRefRepo,
		taskEventRepo,
		step04TxRunner{},
		uploadClient,
		WithTaskAssetCenterModuleRepo(moduleRepo),
	).(*taskAssetCenterService)
	svc.nowFn = func() time.Time { return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC) }
	ctx := domain.WithRequestActor(taskAssetMutationTestContext(), domain.RequestActor{
		ID:       uploaderID,
		Roles:    []domain.Role{domain.RoleAssetManager},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	emptyAssetID, err := designAssetRepo.Create(taskAssetMutationTestContext(), nil, &domain.DesignAsset{
		TaskID:    taskID,
		AssetNo:   "AST-EMPTY",
		AssetType: domain.TaskAssetTypeDelivery,
		CreatedBy: uploaderID,
	})
	if err != nil {
		t.Fatalf("create empty design asset: %v", err)
	}

	_, appErr := svc.CreateMultipartUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:       taskID,
		CreatedBy:    uploaderID,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "new-without-asset.psd",
		ExpectedSize: uploadRequestInt64Ptr(2048),
	})
	if appErr == nil {
		t.Fatal("completed task create without asset_id error = nil")
	}
	details, _ := appErr.Details.(map[string]interface{})
	if details["deny_code"] != "completed_resource_requires_reopen" {
		t.Fatalf("completed task create without asset_id error = %+v", appErr)
	}
	_, appErr = svc.CreateMultipartUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:       taskID,
		AssetID:      &emptyAssetID,
		CreatedBy:    uploaderID,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "new-against-empty-root.psd",
		ExpectedSize: uploadRequestInt64Ptr(2048),
	})
	if appErr == nil {
		t.Fatal("completed task create against empty asset root error = nil")
	}
	details, _ = appErr.Details.(map[string]interface{})
	if details["deny_code"] != "completed_resource_requires_reopen" {
		t.Fatalf("completed task create against empty asset root error = %+v", appErr)
	}

	_, appErr = svc.CreateMultipartUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:       taskID,
		AssetID:      &assetID,
		CreatedBy:    uploaderID,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "new-current.psd",
		ExpectedSize: uploadRequestInt64Ptr(4096),
		MimeType:     "image/vnd.adobe.photoshop",
		Remark:       "post-close correction",
	})
	if appErr == nil {
		t.Fatal("completed task replacement against current resource error = nil")
	}
	details, _ = appErr.Details.(map[string]interface{})
	if details["deny_code"] != "completed_resource_requires_reopen" {
		t.Fatalf("completed task replacement against current resource error = %+v", appErr)
	}
}

func TestAuthorizeV8TaskAssetMutationUsesCapabilityAndStableScope(t *testing.T) {
	const actorID = int64(610)
	departmentID, otherDepartmentID := int64(41), int64(42)
	task := &domain.Task{ID: 2100, TaskStatus: domain.TaskStatusInProgress, OwnerDepartmentID: &departmentID}

	legacyOnly := domain.WithRequestActor(taskAssetMutationTestContext(), domain.RequestActor{ID: actorID, Roles: []domain.Role{domain.RoleAdmin, domain.RoleAuditA}})
	if appErr := authorizeV8TaskAssetMutation(legacyOnly, task, domain.TaskAssetTypeSource); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("legacy role-only authorization = %+v, want permission denied", appErr)
	}

	outOfScopeActor := scopedCapabilityActor(actorID, domain.PermissionTaskDesignSubmit, domain.AccessScopeOwnDepartment, &otherDepartmentID, nil, nil)
	outOfScope := domain.WithRequestActor(taskAssetMutationTestContext(), outOfScopeActor)
	if appErr := authorizeV8TaskAssetMutation(outOfScope, task, domain.TaskAssetTypeSource); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("out-of-scope authorization = %+v, want permission denied", appErr)
	}

	inScopeActor := scopedCapabilityActor(actorID, domain.PermissionTaskDesignSubmit, domain.AccessScopeOwnDepartment, &departmentID, nil, nil)
	if appErr := authorizeV8TaskAssetMutation(domain.WithRequestActor(taskAssetMutationTestContext(), inScopeActor), task, domain.TaskAssetTypeSource); appErr != nil {
		t.Fatalf("in-scope design capability rejected: %+v", appErr)
	}

	task.TaskStatus = domain.TaskStatusPendingAudit
	auditActor := scopedCapabilityActor(actorID, domain.PermissionTaskAuditDecision, domain.AccessScopeOwnDepartment, &departmentID, nil, nil)
	if appErr := authorizeV8TaskAssetMutation(domain.WithRequestActor(taskAssetMutationTestContext(), auditActor), task, domain.TaskAssetTypeSource); appErr != nil {
		t.Fatalf("in-scope audit capability rejected: %+v", appErr)
	}
	if appErr := authorizeV8TaskAssetMutation(domain.WithRequestActor(taskAssetMutationTestContext(), inScopeActor), task, domain.TaskAssetTypeSource); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("design capability accepted during audit: %+v", appErr)
	}

	task.TaskStatus = domain.TaskStatusInProgress
	manageActor := scopedCapabilityActor(actorID, domain.PermissionTaskCreate, domain.AccessScopeOwnDepartment, &departmentID, nil, nil)
	manageCtx := domain.WithRequestActor(taskAssetMutationTestContext(), manageActor)
	if appErr := authorizeV8TaskAssetMutation(manageCtx, task, domain.TaskAssetTypeReference); appErr != nil {
		t.Fatalf("in-scope task.create reference mutation rejected: %+v", appErr)
	}
	if appErr := authorizeV8TaskAssetMutation(manageCtx, task, domain.TaskAssetTypeSource); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("task.create unexpectedly authorized source upload: %+v", appErr)
	}
}

type capturingReferenceFileRefRepo struct {
	inserted []*domain.ReferenceFileRefFlat
}

func (r *capturingReferenceFileRefRepo) InsertFlat(_ context.Context, _ repo.Tx, ref *domain.ReferenceFileRefFlat) (int64, error) {
	copied := *ref
	copied.SKUItemID = domain.CloneInt64Ptr(ref.SKUItemID)
	copied.RetouchRequirementID = domain.CloneInt64Ptr(ref.RetouchRequirementID)
	r.inserted = append(r.inserted, &copied)
	return int64(len(r.inserted)), nil
}

func (*capturingReferenceFileRefRepo) ListByTask(context.Context, int64) ([]*domain.ReferenceFileRefFlat, error) {
	return nil, nil
}

func (*capturingReferenceFileRefRepo) DeleteByTaskAndRef(context.Context, repo.Tx, int64, string) error {
	return nil
}

func TestTaskAssetCenterServiceCompletingTaskReferencePersistsFlatRelation(t *testing.T) {
	const taskID = int64(2104)
	taskRepo := newStep04TaskRepo(&domain.Task{ID: taskID, TaskNo: "T-2104", TaskStatus: domain.TaskStatusInProgress})
	referenceRepo := &capturingReferenceFileRefRepo{}
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted
	svc := NewTaskAssetCenterService(
		taskRepo,
		newStep67DesignAssetRepo(),
		newStep04TaskAssetRepo(),
		newStep37UploadRequestRepo(),
		newStep37AssetStorageRefRepo(),
		&step04TaskEventRepo{},
		step04TxRunner{},
		uploadClient,
		WithTaskAssetCenterReferenceFileRefFlatRepo(referenceRepo),
	).(*taskAssetCenterService)
	actor := scopedCapabilityActor(610, domain.PermissionTaskCreate, domain.AccessScopeGlobal, nil, nil, nil)
	ctx := domain.WithRequestActor(context.Background(), actor)

	created, appErr := svc.CreateUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:         taskID,
		CreatedBy:      actor.ID,
		AssetType:      domain.TaskAssetTypeReference,
		Filename:       "运营参考.png",
		ExpectedSize:   uploadRequestInt64Ptr(1024),
		MimeType:       "image/png",
		OwnerModuleKey: string(domain.ModuleKeyBasicInfo),
		UploadPolicy:   "append_only",
	})
	if appErr != nil {
		t.Fatalf("CreateUploadSession() error = %+v", appErr)
	}
	if _, appErr = svc.CompleteUploadSession(ctx, CompleteTaskAssetUploadSessionParams{
		TaskID: taskID, SessionID: created.Session.ID, CompletedBy: actor.ID,
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession() error = %+v", appErr)
	}
	if len(referenceRepo.inserted) != 1 {
		t.Fatalf("reference rows = %+v, want one", referenceRepo.inserted)
	}
	ref := referenceRepo.inserted[0]
	if ref.TaskID != taskID || ref.SKUItemID != nil || ref.RetouchRequirementID != nil || ref.RefID == "" || ref.Context == nil || *ref.Context != "task_reference" {
		t.Fatalf("task reference relation = %+v", ref)
	}
}

func TestRejectCompletedTaskAssetMutationAlwaysRequiresReopen(t *testing.T) {
	for _, status := range []domain.TaskStatus{domain.TaskStatusCompleted, domain.TaskStatusArchived} {
		t.Run(string(status), func(t *testing.T) {
			appErr := rejectCompletedTaskAssetMutation(&domain.Task{ID: 2101, TaskStatus: status})
			if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition || appErrorDenyCode(appErr) != "completed_resource_requires_reopen" {
				t.Fatalf("finalized mutation rejection = %+v", appErr)
			}
		})
	}
}

func TestTaskAssetCenterServiceGetUploadSessionRejectsFinalizedWithoutRemoteSync(t *testing.T) {
	for _, status := range []domain.TaskStatus{domain.TaskStatusCompleted, domain.TaskStatusArchived} {
		t.Run(string(status), func(t *testing.T) {
			const taskID = int64(2103)
			taskRepo := newStep04TaskRepo(&domain.Task{ID: taskID, TaskNo: "T-2103", TaskStatus: status})
			uploadRequestRepo := newStep37UploadRequestRepo()
			uploadRequestRepo.requests["finalized-session"] = &domain.UploadRequest{
				RequestID:      "finalized-session",
				TaskID:         taskID,
				OwnerType:      domain.AssetOwnerTypeTask,
				OwnerID:        taskID,
				Status:         domain.UploadRequestStatusRequested,
				SessionStatus:  domain.DesignAssetSessionStatusCreated,
				RemoteUploadID: "remote-finalized-session",
			}
			uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
			svc := NewTaskAssetCenterService(
				taskRepo,
				newStep67DesignAssetRepo(),
				newStep04TaskAssetRepo(),
				uploadRequestRepo,
				newStep37AssetStorageRefRepo(),
				&step04TaskEventRepo{},
				step04TxRunner{},
				uploadClient,
			).(*taskAssetCenterService)

			_, appErr := svc.GetUploadSession(taskAssetMutationTestContext(), taskID, "finalized-session")
			if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition || appErrorDenyCode(appErr) != "completed_resource_requires_reopen" {
				t.Fatalf("GetUploadSession() finalized rejection = %+v", appErr)
			}
			if uploadClient.getUploadSessionCalls != 0 {
				t.Fatalf("GetUploadSession() remote calls = %d, want 0", uploadClient.getUploadSessionCalls)
			}
		})
	}
}

func TestTaskAssetCenterServiceGetUploadSessionRechecksFinalizedStateInsideTransaction(t *testing.T) {
	const taskID = int64(2104)
	taskRepo := newStep04TaskRepo(&domain.Task{ID: taskID, TaskNo: "T-2104", TaskStatus: domain.TaskStatusInProgress})
	taskRepo.forUpdateTask = &domain.Task{ID: taskID, TaskNo: "T-2104", TaskStatus: domain.TaskStatusCompleted}
	uploadRequestRepo := newStep37UploadRequestRepo()
	uploadRequestRepo.requests["concurrent-finalize-session"] = &domain.UploadRequest{
		RequestID:      "concurrent-finalize-session",
		TaskID:         taskID,
		OwnerType:      domain.AssetOwnerTypeTask,
		OwnerID:        taskID,
		Status:         domain.UploadRequestStatusRequested,
		SessionStatus:  domain.DesignAssetSessionStatusCreated,
		RemoteUploadID: "remote-concurrent-finalize-session",
	}
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	svc := NewTaskAssetCenterService(
		taskRepo,
		newStep67DesignAssetRepo(),
		newStep04TaskAssetRepo(),
		uploadRequestRepo,
		newStep37AssetStorageRefRepo(),
		&step04TaskEventRepo{},
		step04TxRunner{},
		uploadClient,
	).(*taskAssetCenterService)

	_, appErr := svc.GetUploadSession(taskAssetMutationTestContext(), taskID, "concurrent-finalize-session")
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition || appErrorDenyCode(appErr) != "completed_resource_requires_reopen" {
		t.Fatalf("GetUploadSession() concurrent finalized rejection = %+v", appErr)
	}
	if uploadClient.getUploadSessionCalls != 1 {
		t.Fatalf("GetUploadSession() remote calls = %d, want 1 read before locked recheck", uploadClient.getUploadSessionCalls)
	}
	request := uploadRequestRepo.requests["concurrent-finalize-session"]
	if request.LastSyncedAt != nil || request.SessionStatus != domain.DesignAssetSessionStatusCreated {
		t.Fatalf("concurrent finalized session mutated = %+v", request)
	}
}

func TestTaskAssetCenterServiceCreateUploadSessionRechecksCompletedStateInsideTransaction(t *testing.T) {
	const taskID = int64(2102)
	taskRepo := newStep04TaskRepo(&domain.Task{ID: taskID, TaskNo: "T-2102", TaskStatus: domain.TaskStatusInProgress})
	taskRepo.forUpdateTask = &domain.Task{ID: taskID, TaskNo: "T-2102", TaskStatus: domain.TaskStatusCompleted}
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssetCenterService(
		taskRepo,
		newStep67DesignAssetRepo(),
		newStep04TaskAssetRepo(),
		uploadRequestRepo,
		newStep37AssetStorageRefRepo(),
		taskEventRepo,
		step04TxRunner{},
		newStubUploadServiceClient(),
	).(*taskAssetCenterService)

	_, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       taskID,
		CreatedBy:    99001,
		AssetType:    domain.TaskAssetTypeSource,
		Filename:     "concurrent-close.psd",
		ExpectedSize: uploadRequestInt64Ptr(1024),
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition || appErrorDenyCode(appErr) != "completed_resource_requires_reopen" {
		t.Fatalf("concurrent Completed recheck = %+v", appErr)
	}
	if len(uploadRequestRepo.requests) != 0 || len(taskEventRepo.events) != 0 {
		t.Fatalf("concurrent Completed mutation wrote request/event: requests=%+v events=%+v", uploadRequestRepo.requests, taskEventRepo.events)
	}
}

func TestTaskAssetCenterServiceCompleteMultipartSkipsSecondRemoteCompleteAfterBrowserRemoteComplete(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2011, TaskStatus: domain.TaskStatusPendingAudit})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted
	uploadClient.failComplete = true

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)
	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2011,
		CreatedBy:    511,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "delivery-final.zip",
		ExpectedSize: uploadRequestInt64Ptr(2048),
		MimeType:     "application/zip",
		FileHash:     "zip-hash",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}

	completeResult, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2011,
		SessionID:   createResult.Session.ID,
		CompletedBy: 511,
		Remark:      "browser complete already succeeded",
		FileHash:    "zip-hash",
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession() unexpected error: %+v", appErr)
	}
	if completeResult.Session == nil || completeResult.Session.SessionStatus != domain.DesignAssetSessionStatusCompleted {
		t.Fatalf("CompleteUploadSession() session = %+v", completeResult.Session)
	}
	if completeResult.Asset == nil || completeResult.Version == nil {
		t.Fatalf("CompleteUploadSession() result = %+v", completeResult)
	}
	if uploadClient.completeCalls != 0 {
		t.Fatalf("CompleteUploadSession() remote complete calls = %d, want 0", uploadClient.completeCalls)
	}
	if uploadClient.getUploadSessionCalls == 0 {
		t.Fatalf("CompleteUploadSession() get upload session calls = %d, want > 0", uploadClient.getUploadSessionCalls)
	}
	if uploadClient.getFileMetaCalls == 0 {
		t.Fatalf("CompleteUploadSession() get file meta calls = %d, want > 0", uploadClient.getFileMetaCalls)
	}
	if len(taskEventRepo.events) != 3 {
		t.Fatalf("task events = %+v, want create/version/completed", taskEventRepo.events)
	}
	if taskEventRepo.events[1].EventType != domain.TaskEventAssetVersionCreated ||
		taskEventRepo.events[2].EventType != domain.TaskEventAssetUploadSessionCompleted {
		t.Fatalf("event order = %s/%s, want asset.version.created/upload_session.completed",
			taskEventRepo.events[1].EventType, taskEventRepo.events[2].EventType)
	}

	reloadedSession, appErr := svc.GetUploadSession(taskAssetMutationTestContext(), 2011, createResult.Session.ID)
	if appErr != nil {
		t.Fatalf("GetUploadSession() unexpected error: %+v", appErr)
	}
	if reloadedSession.SessionStatus != domain.DesignAssetSessionStatusCompleted || reloadedSession.CurrentVersionID == nil {
		t.Fatalf("GetUploadSession() session = %+v", reloadedSession)
	}
}

func TestTaskAssetCenterServiceCompleteMultipartFallsBackToBackendRemoteComplete(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2013, TaskStatus: domain.TaskStatusPendingAudit})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCreated

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)
	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2013,
		CreatedBy:    513,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "delivery-fallback.zip",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "application/zip",
		FileHash:     "delivery-fallback-hash",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}

	completeResult, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2013,
		SessionID:   createResult.Session.ID,
		CompletedBy: 513,
		FileHash:    "delivery-fallback-hash",
		Remark:      "fallback remote complete by backend",
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession() unexpected error: %+v", appErr)
	}
	if completeResult.Session == nil || completeResult.Session.SessionStatus != domain.DesignAssetSessionStatusCompleted {
		t.Fatalf("CompleteUploadSession() session = %+v", completeResult.Session)
	}
	if completeResult.Asset == nil || completeResult.Version == nil {
		t.Fatalf("CompleteUploadSession() result = %+v", completeResult)
	}
	if uploadClient.completeCalls != 1 {
		t.Fatalf("CompleteUploadSession() remote complete calls = %d, want 1", uploadClient.completeCalls)
	}
}

func TestTaskAssetCenterServiceCompleteUploadSessionIsIdempotentAfterFinalize(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2012, TaskStatus: domain.TaskStatusPendingAudit})
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted

	svc := NewTaskAssetCenterService(taskRepo, newStep67DesignAssetRepo(), newStep04TaskAssetRepo(), newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, uploadClient).(*taskAssetCenterService)
	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2012,
		CreatedBy:    512,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "preview.png",
		ExpectedSize: uploadRequestInt64Ptr(4096),
		MimeType:     "image/png",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}

	firstResult, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2012,
		SessionID:   createResult.Session.ID,
		CompletedBy: 512,
		Remark:      "first finalize",
	})
	if appErr != nil {
		t.Fatalf("first CompleteUploadSession() unexpected error: %+v", appErr)
	}
	secondResult, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2012,
		SessionID:   createResult.Session.ID,
		CompletedBy: 512,
		Remark:      "second finalize",
	})
	if appErr != nil {
		t.Fatalf("second CompleteUploadSession() unexpected error: %+v", appErr)
	}
	if firstResult.Version == nil || secondResult.Version == nil || firstResult.Version.ID != secondResult.Version.ID {
		t.Fatalf("idempotent CompleteUploadSession() version mismatch: first=%+v second=%+v", firstResult.Version, secondResult.Version)
	}
}

func TestTaskAssetCenterServiceCompleteOSSDirectMultipartWithoutRemoteSync(t *testing.T) {
	ossServer := newFakeOSSDirectServer(t)
	defer ossServer.Close()

	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2040, TaskNo: "T-2040", TaskStatus: domain.TaskStatusPendingAudit})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.getUploadSessionErr = fmt.Errorf("upload service get session unavailable")

	ossDirect := NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        ossServer.EndpointHost(),
		PublicEndpoint:  ossServer.EndpointHost(),
		Bucket:          "test-bucket",
		AccessKeyID:     "test-key",
		AccessKeySecret: "test-secret",
		PresignExpiry:   15 * time.Minute,
		PartSize:        10 * 1024 * 1024,
	})
	ossDirect.httpClient = ossServer.Client()

	svc := NewTaskAssetCenterService(
		taskRepo,
		designAssetRepo,
		taskAssetRepo,
		uploadRequestRepo,
		storageRefRepo,
		taskEventRepo,
		step04TxRunner{},
		uploadClient,
		WithOSSDirectService(ossDirect),
		WithTaskAssetCenterPreviewRenderer(testPreviewRenderer{}),
	).(*taskAssetCenterService)

	body := []byte("canonical oss direct payload")
	bodyHash := sha256.Sum256(body)
	fileHash := hex.EncodeToString(bodyHash[:])

	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2040,
		CreatedBy:    640,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "delivery-proof.zip",
		ExpectedSize: uploadRequestInt64Ptr(int64(len(body))),
		MimeType:     "application/zip",
		FileHash:     fileHash,
		Remark:       "canonical create",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}
	if createResult == nil || createResult.OSSDirect == nil {
		t.Fatalf("CreateMultipartUploadSession() oss_direct = %+v", createResult)
	}
	if len(createResult.OSSDirect.Parts) != 1 {
		t.Fatalf("OSS direct parts = %d, want 1", len(createResult.OSSDirect.Parts))
	}

	uploadedParts := make([]OSSCompletePart, 0, len(createResult.OSSDirect.Parts))
	for _, part := range createResult.OSSDirect.Parts {
		req, err := http.NewRequest(part.Method, part.UploadURL, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Content-Type", createResult.OSSDirect.RequiredContentType)
		resp, err := ossServer.Client().Do(req)
		if err != nil {
			t.Fatalf("direct PUT error = %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("direct PUT status = %d", resp.StatusCode)
		}
		uploadedParts = append(uploadedParts, OSSCompletePart{
			PartNumber: part.PartNumber,
			ETag:       strings.TrimSpace(resp.Header.Get("ETag")),
		})
	}

	completeResult, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:            2040,
		SessionID:         createResult.Session.ID,
		CompletedBy:       640,
		FileHash:          fileHash,
		Remark:            "canonical complete",
		UploadContentType: createResult.OSSDirect.RequiredContentType,
		OSSParts:          uploadedParts,
		OSSUploadID:       createResult.OSSDirect.UploadID,
		OSSObjectKey:      createResult.OSSDirect.ObjectKey,
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession() unexpected error: %+v", appErr)
	}
	if uploadClient.getUploadSessionCalls != 0 {
		t.Fatalf("GetUploadSession() calls = %d, want 0", uploadClient.getUploadSessionCalls)
	}
	if uploadClient.completeCalls != 0 {
		t.Fatalf("remote complete calls = %d, want 0", uploadClient.completeCalls)
	}
	if uploadClient.getFileMetaCalls != 0 {
		t.Fatalf("get file meta calls = %d, want 0", uploadClient.getFileMetaCalls)
	}
	if completeResult == nil || completeResult.Asset == nil || completeResult.Version == nil {
		t.Fatalf("CompleteUploadSession() result = %+v", completeResult)
	}
	if completeResult.Version.StorageKey != createResult.OSSDirect.ObjectKey {
		t.Fatalf("version storage key = %q, want %q", completeResult.Version.StorageKey, createResult.OSSDirect.ObjectKey)
	}
	if completeResult.Version.FileHash == nil || *completeResult.Version.FileHash != fileHash {
		t.Fatalf("version file hash = %+v, want %q", completeResult.Version.FileHash, fileHash)
	}
	if ossServer.completeCalls != 1 {
		t.Fatalf("OSS complete calls = %d, want 1", ossServer.completeCalls)
	}

	assets, appErr := svc.ListAssets(taskAssetMutationTestContext(), 2040)
	if appErr != nil {
		t.Fatalf("ListAssets() unexpected error: %+v", appErr)
	}
	if len(assets) != 1 || assets[0].CurrentVersion == nil {
		t.Fatalf("ListAssets() assets = %+v", assets)
	}
	if assets[0].CurrentVersion.StorageKey != createResult.OSSDirect.ObjectKey {
		t.Fatalf("listed storage key = %q, want %q", assets[0].CurrentVersion.StorageKey, createResult.OSSDirect.ObjectKey)
	}
}

func TestTaskAssetCenterServiceCompleteOSSDirectSinglePartWithoutRemoteSync(t *testing.T) {
	ossServer := newFakeOSSDirectServer(t)
	defer ossServer.Close()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	ossDirect := NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        ossServer.EndpointHost(),
		PublicEndpoint:  ossServer.EndpointHost(),
		Bucket:          "test-bucket",
		AccessKeyID:     "test-key",
		AccessKeySecret: "test-secret",
		PartSize:        10 * 1024 * 1024,
	})
	ossDirect.httpClient = ossServer.Client()
	svc := NewTaskAssetCenterService(
		newStep04TaskRepo(&domain.Task{ID: 2041, TaskNo: "T-2041", TaskStatus: domain.TaskStatusPendingAudit}),
		newStep67DesignAssetRepo(),
		newStep04TaskAssetRepo(),
		newStep37UploadRequestRepo(),
		newStep37AssetStorageRefRepo(),
		&step04TaskEventRepo{},
		step04TxRunner{},
		uploadClient,
		WithOSSDirectService(ossDirect),
		WithTaskAssetCenterPreviewRenderer(testPreviewRenderer{}),
	).(*taskAssetCenterService)
	body := []byte("small direct asset")
	created, appErr := svc.CreateUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:         2041,
		CreatedBy:      641,
		AssetType:      domain.TaskAssetTypeReference,
		Filename:       "reference.png",
		ExpectedSize:   uploadRequestInt64Ptr(int64(len(body))),
		MimeType:       "image/png",
		OwnerModuleKey: string(domain.ModuleKeyBasicInfo),
	})
	if appErr != nil {
		t.Fatalf("CreateUploadSession() error = %+v", appErr)
	}
	if created.OSSDirect == nil || created.OSSDirect.Mode != "single_part" || created.OSSDirect.UploadURL == "" {
		t.Fatalf("single-part plan = %+v", created.OSSDirect)
	}
	req, err := http.NewRequest(http.MethodPut, created.OSSDirect.UploadURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build direct PUT: %v", err)
	}
	req.Header.Set("Content-Type", created.OSSDirect.RequiredContentType)
	resp, err := ossServer.Client().Do(req)
	if err != nil {
		t.Fatalf("direct PUT: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("direct PUT status = %d", resp.StatusCode)
	}
	completed, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:            2041,
		SessionID:         created.Session.ID,
		CompletedBy:       641,
		UploadContentType: created.OSSDirect.RequiredContentType,
		OSSObjectKey:      created.OSSDirect.ObjectKey,
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession() error = %+v", appErr)
	}
	if completed.Version == nil || completed.Version.StorageKey != created.OSSDirect.ObjectKey {
		t.Fatalf("completed version = %+v", completed.Version)
	}
	if uploadClient.getUploadSessionCalls != 0 || uploadClient.completeCalls != 0 {
		t.Fatalf("remote calls get=%d complete=%d", uploadClient.getUploadSessionCalls, uploadClient.completeCalls)
	}
}

func TestTaskAssetCenterServiceCreateOSSDirectMultipartUsesASCIIObjectKeyAndPreservesOriginalFilename(t *testing.T) {
	ossServer := newFakeOSSDirectServer(t)
	defer ossServer.Close()

	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2042, TaskNo: "T-2042", TaskStatus: domain.TaskStatusPendingAudit})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.getUploadSessionErr = fmt.Errorf("upload service get session unavailable")

	ossDirect := NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        ossServer.EndpointHost(),
		PublicEndpoint:  ossServer.EndpointHost(),
		Bucket:          "test-bucket",
		AccessKeyID:     "test-key",
		AccessKeySecret: "test-secret",
		PresignExpiry:   15 * time.Minute,
		PartSize:        10 * 1024 * 1024,
	})
	ossDirect.httpClient = ossServer.Client()

	svc := NewTaskAssetCenterService(
		taskRepo,
		designAssetRepo,
		taskAssetRepo,
		uploadRequestRepo,
		storageRefRepo,
		taskEventRepo,
		step04TxRunner{},
		uploadClient,
		WithOSSDirectService(ossDirect),
		WithTaskAssetCenterPreviewRenderer(testPreviewRenderer{}),
	).(*taskAssetCenterService)

	originalFilename := "手淘_SKU_13_蒙的都对【送12色涂鸦笔+木架】.jpg"
	body := []byte("special filename payload")
	bodyHash := sha256.Sum256(body)
	fileHash := hex.EncodeToString(bodyHash[:])

	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2042,
		CreatedBy:    642,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     originalFilename,
		ExpectedSize: uploadRequestInt64Ptr(int64(len(body))),
		MimeType:     "image/jpeg",
		FileHash:     fileHash,
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}
	if createResult == nil || createResult.OSSDirect == nil {
		t.Fatalf("CreateMultipartUploadSession() oss_direct = %+v", createResult)
	}
	if ossServer.initiateCalls != 1 {
		t.Fatalf("OSS initiate calls = %d, want 1", ossServer.initiateCalls)
	}
	if createResult.Session == nil || createResult.Session.Filename != originalFilename {
		t.Fatalf("session filename = %+v, want %q", createResult.Session, originalFilename)
	}

	objectKey := createResult.OSSDirect.ObjectKey
	if !regexp.MustCompile(`^tasks/T-2042/upload-sessions/[A-Za-z0-9_-]+/[A-Za-z0-9_-]+\.jpg$`).MatchString(objectKey) {
		t.Fatalf("object key = %q, want deterministic upload-session jpg key", objectKey)
	}
	for _, forbidden := range []string{"手淘", "+", "木架", " "} {
		if strings.Contains(objectKey, forbidden) {
			t.Fatalf("object key %q contains original filename fragment %q", objectKey, forbidden)
		}
	}

	uploadedParts := make([]OSSCompletePart, 0, len(createResult.OSSDirect.Parts))
	for _, part := range createResult.OSSDirect.Parts {
		req, err := http.NewRequest(part.Method, part.UploadURL, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Content-Type", createResult.OSSDirect.RequiredContentType)
		resp, err := ossServer.Client().Do(req)
		if err != nil {
			t.Fatalf("direct PUT error = %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("direct PUT status = %d", resp.StatusCode)
		}
		uploadedParts = append(uploadedParts, OSSCompletePart{
			PartNumber: part.PartNumber,
			ETag:       strings.TrimSpace(resp.Header.Get("ETag")),
		})
	}

	completeResult, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:            2042,
		SessionID:         createResult.Session.ID,
		CompletedBy:       642,
		FileHash:          fileHash,
		UploadContentType: createResult.OSSDirect.RequiredContentType,
		OSSParts:          uploadedParts,
		OSSUploadID:       createResult.OSSDirect.UploadID,
		OSSObjectKey:      objectKey,
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession() unexpected error: %+v", appErr)
	}
	if completeResult.Session == nil || completeResult.Session.Filename != originalFilename {
		t.Fatalf("completed session filename = %+v, want %q", completeResult.Session, originalFilename)
	}
	if completeResult.Version == nil || completeResult.Version.OriginalFilename != originalFilename {
		t.Fatalf("completed version original filename = %+v, want %q", completeResult.Version, originalFilename)
	}
	if completeResult.Version.StorageKey != objectKey {
		t.Fatalf("version storage key = %q, want %q", completeResult.Version.StorageKey, objectKey)
	}
}

func TestTaskAssetCenterServiceCompleteUploadSessionRejectsPartialOSSDirectPayload(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2041, TaskStatus: domain.TaskStatusPendingAudit})
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	svc := NewTaskAssetCenterService(
		taskRepo,
		newStep67DesignAssetRepo(),
		newStep04TaskAssetRepo(),
		newStep37UploadRequestRepo(),
		newStep37AssetStorageRefRepo(),
		&step04TaskEventRepo{},
		step04TxRunner{},
		uploadClient,
	).(*taskAssetCenterService)

	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2041,
		CreatedBy:    641,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "partial-direct.zip",
		ExpectedSize: uploadRequestInt64Ptr(512),
		MimeType:     "application/zip",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}

	_, appErr = svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2041,
		SessionID:   createResult.Session.ID,
		CompletedBy: 641,
		OSSUploadID: "oss-upload-only",
	})
	if appErr == nil {
		t.Fatal("CompleteUploadSession() appErr = nil, want invalid request")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("CompleteUploadSession() code = %s", appErr.Code)
	}
	if uploadClient.getUploadSessionCalls != 0 {
		t.Fatalf("GetUploadSession() calls = %d, want 0", uploadClient.getUploadSessionCalls)
	}
	if uploadClient.completeCalls != 0 {
		t.Fatalf("remote complete calls = %d, want 0", uploadClient.completeCalls)
	}
}

func TestTaskAssetCenterServiceCreateMultipartAndCancel(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2002, TaskStatus: domain.TaskStatusPendingAudit})
	svc := NewTaskAssetCenterService(taskRepo, newStep67DesignAssetRepo(), newStep04TaskAssetRepo(), newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient()).(*taskAssetCenterService)

	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2002,
		CreatedBy:    502,
		AssetType:    domain.TaskAssetTypeFinal,
		Filename:     "final.ai",
		ExpectedSize: uploadRequestInt64Ptr(987654321),
		MimeType:     "application/postscript",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}
	if createResult.Session.UploadMode != domain.DesignAssetUploadModeMultipart {
		t.Fatalf("CreateMultipartUploadSession() session = %+v", createResult.Session)
	}
	if createResult.Remote.PartSizeHint == 0 {
		t.Fatalf("CreateMultipartUploadSession() remote = %+v", createResult.Remote)
	}
	if createResult.Remote.Headers != nil {
		t.Fatalf("CreateMultipartUploadSession() headers = %+v, want no internal auth headers for browser", createResult.Remote.Headers)
	}

	session, appErr := svc.CancelUploadSession(taskAssetMutationTestContext(), CancelTaskAssetUploadSessionParams{
		TaskID:      2002,
		SessionID:   createResult.Session.ID,
		CancelledBy: 502,
		Remark:      "user cancelled",
	})
	if appErr != nil {
		t.Fatalf("CancelUploadSession() unexpected error: %+v", appErr)
	}
	if session.SessionStatus != domain.DesignAssetSessionStatusCancelled {
		t.Fatalf("CancelUploadSession() session = %+v", session)
	}
}

func TestResolveRejectedDeliveryRevisionAssetIDReusesMatchingRoot(t *testing.T) {
	const taskID = int64(2101)
	task := &domain.Task{
		ID:                    taskID,
		TaskStatus:            domain.TaskStatusPendingCustomizationProduction,
		BusinessLane:          domain.TaskBusinessLaneCustomization,
		CustomizationRequired: true,
	}
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	assetID, err := designAssetRepo.Create(taskAssetMutationTestContext(), nil, &domain.DesignAsset{
		TaskID:    taskID,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeDelivery,
		CreatedBy: 502,
	})
	if err != nil {
		t.Fatalf("create design asset: %v", err)
	}
	versionNo := 1
	originalFilename := "DZK000149.png"
	versionID, err := taskAssetRepo.Create(taskAssetMutationTestContext(), nil, &domain.TaskAsset{
		TaskID:           taskID,
		AssetID:          &assetID,
		AssetType:        domain.TaskAssetTypeDelivery,
		AssetVersionNo:   &versionNo,
		FileName:         originalFilename,
		OriginalName:     &originalFilename,
		FlowReviewStatus: domain.TaskAssetFlowReviewStatusRejected,
	})
	if err != nil {
		t.Fatalf("create rejected version: %v", err)
	}
	if err := designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), nil, assetID, &versionID); err != nil {
		t.Fatalf("set current version: %v", err)
	}
	svc := &taskAssetCenterService{designAssetRepo: designAssetRepo, taskAssetRepo: taskAssetRepo}

	resolved, appErr := svc.resolveRejectedDeliveryRevisionAssetID(taskAssetMutationTestContext(), task, CreateTaskAssetUploadSessionParams{
		TaskID:    taskID,
		AssetType: domain.TaskAssetTypeDelivery,
		Filename:  originalFilename,
	})
	if appErr != nil {
		t.Fatalf("resolveRejectedDeliveryRevisionAssetID() appErr = %+v", appErr)
	}
	if resolved == nil || *resolved != assetID {
		t.Fatalf("resolved asset id = %+v, want %d", resolved, assetID)
	}
}

func TestTaskAssetCenterServiceAllowsSmallUploadForDeliverySourcePreview(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2004, TaskStatus: domain.TaskStatusPendingAudit})
	svc := NewTaskAssetCenterService(taskRepo, newStep67DesignAssetRepo(), newStep04TaskAssetRepo(), newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient()).(*taskAssetCenterService)

	result, appErr := svc.CreateSmallUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2004,
		CreatedBy:    503,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "delivery.png",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "image/png",
	})
	if appErr != nil {
		t.Fatalf("CreateSmallUploadSession() unexpected error: %+v", appErr)
	}
	if result == nil || result.Session == nil || result.Session.UploadMode != domain.DesignAssetUploadModeSmall {
		t.Fatalf("CreateSmallUploadSession() result = %+v", result)
	}
}

func TestTaskAssetCenterServiceListAssetsAndVersions(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2003, TaskNo: "T-2003", TaskStatus: domain.TaskStatusPendingWarehouseReceive})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()

	assetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2003,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeDelivery,
		CreatedBy: 601,
	})
	versionNo := 1
	versionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{
		TaskID:         2003,
		AssetID:        &assetID,
		AssetType:      domain.TaskAssetTypeDelivery,
		VersionNo:      1,
		AssetVersionNo: &versionNo,
		UploadMode:     strPtr("small"),
		FileName:       "delivery.png",
		OriginalName:   strPtr("delivery.png"),
		StorageKey:     strPtr("objects/design-assets/delivery.png"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     601,
		UploadedAt:     timeValuePtr(time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)),
	})
	_ = designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, assetID, &versionID)

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient())

	assets, appErr := svc.ListAssets(taskAssetMutationTestContext(), 2003)
	if appErr != nil {
		t.Fatalf("ListAssets() unexpected error: %+v", appErr)
	}
	if len(assets) != 1 || assets[0].CurrentVersion == nil {
		t.Fatalf("ListAssets() assets = %+v", assets)
	}
	if assets[0].ApprovedVersion == nil || assets[0].WarehouseReadyVersion == nil {
		t.Fatalf("ListAssets() version pointers = %+v", assets[0])
	}
	if assets[0].WarehouseReadyVersion.CurrentVersionRole != "current_warehouse_ready_version" {
		t.Fatalf("ListAssets() warehouse_ready_version = %+v", assets[0].WarehouseReadyVersion)
	}
	versions, appErr := svc.ListVersions(taskAssetMutationTestContext(), 2003, assetID)
	if appErr != nil {
		t.Fatalf("ListVersions() unexpected error: %+v", appErr)
	}
	if len(versions) != 1 || versions[0].VersionNo != 1 {
		t.Fatalf("ListVersions() versions = %+v", versions)
	}
	if !versions[0].ApprovedForFlow || !versions[0].WarehouseReady || versions[0].TaskNo != "T-2003" || versions[0].AssetNo != "AST-0001" {
		t.Fatalf("ListVersions() version semantics = %+v", versions[0])
	}
}

func TestTaskAssetCenterServiceCreateAndCompleteMultipartUploadSessionWithTargetSKUCode(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2005, TaskStatus: domain.TaskStatusPendingAudit, IsBatchTask: true, BatchItemCount: 2, BatchMode: domain.TaskBatchModeMultiSKU})
	taskRepo.skuItems = map[int64][]*domain.TaskSKUItem{
		2005: {
			{ID: 1, TaskID: 2005, SequenceNo: 1, SKUCode: "BATCH-2005-A"},
			{ID: 2, TaskID: 2005, SequenceNo: 2, SKUCode: "BATCH-2005-B"},
		},
	}
	taskRepo.skuByCode = map[string]*domain.TaskSKUItem{
		"BATCH-2005-A": taskRepo.skuItems[2005][0],
		"BATCH-2005-B": taskRepo.skuItems[2005][1],
	}
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)
	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:        2005,
		CreatedBy:     520,
		AssetType:     domain.TaskAssetTypeDelivery,
		Filename:      "batch-sku.psd",
		ExpectedSize:  uploadRequestInt64Ptr(2048),
		MimeType:      "application/octet-stream",
		TargetSKUCode: "BATCH-2005-B",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}
	if createResult.Session == nil || createResult.Session.TargetSKUCode != "BATCH-2005-B" {
		t.Fatalf("CreateMultipartUploadSession() session = %+v", createResult.Session)
	}

	completeResult, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2005,
		SessionID:   createResult.Session.ID,
		CompletedBy: 520,
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession() unexpected error: %+v", appErr)
	}
	if completeResult.Asset == nil || completeResult.Asset.ScopeSKUCode != "BATCH-2005-B" {
		t.Fatalf("CompleteUploadSession() asset = %+v", completeResult.Asset)
	}
	if completeResult.Version == nil || completeResult.Version.ScopeSKUCode != "BATCH-2005-B" {
		t.Fatalf("CompleteUploadSession() version = %+v", completeResult.Version)
	}
	request, err := uploadRequestRepo.GetByRequestID(taskAssetMutationTestContext(), createResult.Session.ID)
	if err != nil {
		t.Fatalf("GetByRequestID() error = %v", err)
	}
	if request == nil || request.TargetSKUCode != "BATCH-2005-B" {
		t.Fatalf("upload request target_sku_code = %+v", request)
	}
}

func TestTaskAssetCenterServiceAuditBatchUploadsNeverAdvanceWorkflow(t *testing.T) {
	designerID := int64(530)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:             2006,
		TaskStatus:     domain.TaskStatusPendingAudit,
		IsBatchTask:    true,
		BatchItemCount: 2,
		BatchMode:      domain.TaskBatchModeMultiSKU,
		DesignerID:     &designerID,
	})
	taskRepo.skuItems = map[int64][]*domain.TaskSKUItem{
		2006: {
			{ID: 1, TaskID: 2006, SequenceNo: 1, SKUCode: "BATCH-2006-A"},
			{ID: 2, TaskID: 2006, SequenceNo: 2, SKUCode: "BATCH-2006-B"},
		},
	}
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted
	workflow := &step04DesignSubmissionWorkflow{}

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient,
		WithTaskAssetCenterBlueprintRuleEngine(workflow)).(*taskAssetCenterService)

	createA, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:        2006,
		CreatedBy:     530,
		AssetType:     domain.TaskAssetTypeDelivery,
		Filename:      "batch-a.psd",
		ExpectedSize:  uploadRequestInt64Ptr(1024),
		MimeType:      "application/octet-stream",
		TargetSKUCode: "BATCH-2006-A",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession(A) unexpected error: %+v", appErr)
	}
	if _, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2006,
		SessionID:   createA.Session.ID,
		CompletedBy: 530,
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession(A) unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[2006].TaskStatus != domain.TaskStatusPendingAudit {
		t.Fatalf("after first SKU complete task status = %s, want PendingAudit", taskRepo.tasks[2006].TaskStatus)
	}
	if countStep04TaskEvents(taskEventRepo.events, domain.TaskEventDesignSubmitted) != 0 {
		t.Fatalf("after first SKU complete design submitted events = %+v, want none", taskEventRepo.events)
	}

	createB, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:        2006,
		CreatedBy:     530,
		AssetType:     domain.TaskAssetTypeDelivery,
		Filename:      "batch-b.psd",
		ExpectedSize:  uploadRequestInt64Ptr(1024),
		MimeType:      "application/octet-stream",
		TargetSKUCode: "BATCH-2006-B",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession(B) unexpected error: %+v", appErr)
	}
	if _, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2006,
		SessionID:   createB.Session.ID,
		CompletedBy: 530,
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession(B) unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[2006].TaskStatus != domain.TaskStatusPendingAudit {
		t.Fatalf("after all SKU complete task status = %s, want PendingAudit until audit decision", taskRepo.tasks[2006].TaskStatus)
	}
	if countStep04TaskEvents(taskEventRepo.events, domain.TaskEventDesignSubmitted) != 0 {
		t.Fatalf("design submitted events = %+v, want none before submit-design", taskEventRepo.events)
	}
	if len(workflow.calls) != 0 {
		t.Fatalf("workflow calls = %+v, want none before submit-design", workflow.calls)
	}
}

func TestTaskAssetCenterServiceBatchCompleteAllowsLateStagedSession(t *testing.T) {
	designerID := int64(932)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:             2090,
		TaskStatus:     domain.TaskStatusPendingAudit,
		IsBatchTask:    true,
		BatchItemCount: 2,
		BatchMode:      domain.TaskBatchModeMultiSKU,
		DesignerID:     &designerID,
	})
	taskRepo.skuItems = map[int64][]*domain.TaskSKUItem{
		2090: {
			{ID: 1, TaskID: 2090, SequenceNo: 1, SKUCode: "BATCH-2090-A"},
			{ID: 2, TaskID: 2090, SequenceNo: 2, SKUCode: "BATCH-2090-B"},
		},
	}
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)

	deliveryA, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:        2090,
		CreatedBy:     932,
		AssetType:     domain.TaskAssetTypeDelivery,
		Filename:      "batch-2090-a.psd",
		ExpectedSize:  uploadRequestInt64Ptr(1024),
		MimeType:      "application/octet-stream",
		TargetSKUCode: "BATCH-2090-A",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession(deliveryA) unexpected error: %+v", appErr)
	}
	deliveryB, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:        2090,
		CreatedBy:     932,
		AssetType:     domain.TaskAssetTypeDelivery,
		Filename:      "batch-2090-b.psd",
		ExpectedSize:  uploadRequestInt64Ptr(1024),
		MimeType:      "application/octet-stream",
		TargetSKUCode: "BATCH-2090-B",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession(deliveryB) unexpected error: %+v", appErr)
	}
	sourceB, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:        2090,
		CreatedBy:     932,
		AssetType:     domain.TaskAssetTypeSource,
		Filename:      "batch-2090-b-source.psd",
		ExpectedSize:  uploadRequestInt64Ptr(1024),
		MimeType:      "application/octet-stream",
		TargetSKUCode: "BATCH-2090-B",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession(sourceB) unexpected error: %+v", appErr)
	}

	if _, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2090,
		SessionID:   deliveryA.Session.ID,
		CompletedBy: 932,
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession(deliveryA) unexpected error: %+v", appErr)
	}
	if _, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2090,
		SessionID:   deliveryB.Session.ID,
		CompletedBy: 932,
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession(deliveryB) unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[2090].TaskStatus != domain.TaskStatusPendingAudit {
		t.Fatalf("task status after delivery completion = %s, want PendingAudit until audit decision", taskRepo.tasks[2090].TaskStatus)
	}

	lateResult, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2090,
		SessionID:   sourceB.Session.ID,
		CompletedBy: 932,
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession(sourceB late) unexpected error: %+v", appErr)
	}
	if lateResult == nil || lateResult.Version == nil || lateResult.Version.AssetType != domain.TaskAssetTypeSource {
		t.Fatalf("late completion result = %+v", lateResult)
	}
}

func TestTaskAssetCenterServiceCompletePrecreatedSessionAllowedInPendingAudit(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2091, TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted
	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)

	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2091,
		CreatedBy:    933,
		AssetType:    domain.TaskAssetTypeSource,
		Filename:     "window-proof.psd",
		ExpectedSize: uploadRequestInt64Ptr(512),
		MimeType:     "image/vnd.adobe.photoshop",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}
	taskRepo.tasks[2091].TaskStatus = domain.TaskStatusPendingAudit

	if _, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2091,
		SessionID:   createResult.Session.ID,
		CompletedBy: 933,
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession() unexpected error in PendingAudit window: %+v", appErr)
	}
	if len(taskAssetRepo.assets) != 1 {
		t.Fatalf("created task assets = %+v, want one", taskAssetRepo.assets)
	}
	for _, asset := range taskAssetRepo.assets {
		if asset.SourceModuleKey != domain.ModuleKeyAudit {
			t.Fatalf("completed audit upload source_module_key = %q, want %q", asset.SourceModuleKey, domain.ModuleKeyAudit)
		}
	}
}

func TestTaskAssetCenterServiceAuditStageAllowsSourceDeliveryAndBasicInfoReference(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2092, TaskStatus: domain.TaskStatusPendingAudit})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)
	ctx := taskAssetMutationTestContext()

	if _, appErr := svc.CreateMultipartUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:       2092,
		CreatedBy:    934,
		AssetType:    domain.TaskAssetTypeSource,
		Filename:     "audit-source.psd",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "image/vnd.adobe.photoshop",
	}); appErr != nil {
		t.Fatalf("CreateMultipartUploadSession(audit source) unexpected error: %+v", appErr)
	}
	if _, appErr := svc.CreateMultipartUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:       2092,
		CreatedBy:    934,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "audit-delivery.jpg",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "image/jpeg",
	}); appErr != nil {
		t.Fatalf("CreateMultipartUploadSession(audit delivery) unexpected error: %+v", appErr)
	}
	_, appErr := svc.CreateMultipartUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:       2092,
		CreatedBy:    934,
		AssetType:    domain.TaskAssetTypeReference,
		Filename:     "audit-reference.jpg",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "image/jpeg",
	})
	if appErr == nil {
		t.Fatal("CreateMultipartUploadSession(audit reference) appErr = nil, want invalid request")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("CreateMultipartUploadSession(audit reference) code = %s, want INVALID_REQUEST", appErr.Code)
	}

	referenceAssetID, err := designAssetRepo.Create(taskAssetMutationTestContext(), nil, &domain.DesignAsset{
		TaskID:    2092,
		AssetNo:   "AST-REF",
		AssetType: domain.TaskAssetTypeReference,
		CreatedBy: 934,
	})
	if err != nil {
		t.Fatalf("create existing reference asset: %v", err)
	}
	basicInfoReference, appErr := svc.CreateMultipartUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:         2092,
		AssetID:        &referenceAssetID,
		CreatedBy:      934,
		AssetType:      domain.TaskAssetTypeReference,
		Filename:       "audit-reference-replace.jpg",
		ExpectedSize:   uploadRequestInt64Ptr(1024),
		MimeType:       "image/jpeg",
		OwnerModuleKey: string(domain.ModuleKeyBasicInfo),
		UploadPolicy:   "replace",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession(audit basic_info reference) unexpected error: %+v", appErr)
	}
	if basicInfoReference == nil || basicInfoReference.Session == nil || basicInfoReference.Session.AssetID == nil || *basicInfoReference.Session.AssetID != referenceAssetID {
		t.Fatalf("CreateMultipartUploadSession(audit basic_info reference) did not bind existing reference asset: %+v", basicInfoReference)
	}
}

func TestTaskAssetCenterServiceBatchDeliveryRequiresTargetSKUCode(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:             2007,
		TaskStatus:     domain.TaskStatusInProgress,
		IsBatchTask:    true,
		BatchItemCount: 2,
		BatchMode:      domain.TaskBatchModeMultiSKU,
	})
	taskRepo.skuItems = map[int64][]*domain.TaskSKUItem{
		2007: {
			{ID: 1, TaskID: 2007, SequenceNo: 1, SKUCode: "BATCH-2007-A"},
			{ID: 2, TaskID: 2007, SequenceNo: 2, SKUCode: "BATCH-2007-B"},
		},
	}
	svc := NewTaskAssetCenterService(taskRepo, newStep67DesignAssetRepo(), newStep04TaskAssetRepo(), newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient()).(*taskAssetCenterService)

	_, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2007,
		CreatedBy:    531,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "batch.psd",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "application/octet-stream",
	})
	if appErr == nil {
		t.Fatal("CreateMultipartUploadSession() appErr = nil, want invalid request")
	}
	if appErr.Message != "target_sku_code is required for batch non-reference asset uploads" {
		t.Fatalf("CreateMultipartUploadSession() message = %q", appErr.Message)
	}
}

func TestTaskAssetCenterServiceCreateUploadSessionDeniesDepartmentManagerOutsideScope(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:              2093,
		TaskNo:          "T-2093",
		TaskStatus:      domain.TaskStatusInProgress,
		OwnerDepartment: "运营部",
		OwnerOrgTeam:    "淘系一组",
	})
	userRepo := newIdentityUserRepo()
	svc := NewTaskAssetCenterService(
		taskRepo,
		newStep67DesignAssetRepo(),
		newStep04TaskAssetRepo(),
		newStep37UploadRequestRepo(),
		newStep37AssetStorageRefRepo(),
		&step04TaskEventRepo{},
		step04TxRunner{},
		newStubUploadServiceClient(),
		WithTaskAssetCenterDataScopeResolver(NewRoleBasedDataScopeResolver()),
		WithTaskAssetCenterScopeUserRepo(userRepo),
	).(*taskAssetCenterService)

	ctx := domain.WithRequestActor(taskAssetMutationTestContext(), domain.RequestActor{
		ID:         935,
		Username:   "design_admin",
		Roles:      []domain.Role{domain.RoleDeptAdmin},
		Department: "设计研发部",
		Source:     domain.RequestActorSourceSessionToken,
		AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
	})
	_, appErr := svc.CreateMultipartUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:       2093,
		CreatedBy:    935,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "out-of-scope.zip",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "application/zip",
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("CreateMultipartUploadSession() appErr = %+v, want permission denied", appErr)
	}
}

func TestTaskAssetCenterServiceCreateUploadSessionInfersModeByFileSize(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2008, TaskStatus: domain.TaskStatusPendingAudit})
	svc := NewTaskAssetCenterService(taskRepo, newStep67DesignAssetRepo(), newStep04TaskAssetRepo(), newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient()).(*taskAssetCenterService)

	referenceResult, appErr := svc.CreateUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:         2008,
		CreatedBy:      540,
		AssetType:      domain.TaskAssetTypeReference,
		Filename:       "reference.png",
		ExpectedSize:   uploadRequestInt64Ptr(512),
		MimeType:       "image/png",
		OwnerModuleKey: domain.ModuleKeyBasicInfo,
	})
	if appErr != nil {
		t.Fatalf("CreateUploadSession(reference) unexpected error: %+v", appErr)
	}
	if referenceResult.Session == nil || referenceResult.Session.UploadMode != domain.DesignAssetUploadModeSmall {
		t.Fatalf("CreateUploadSession(reference) session = %+v", referenceResult.Session)
	}

	deliveryResult, appErr := svc.CreateUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2008,
		CreatedBy:    540,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "delivery.zip",
		ExpectedSize: uploadRequestInt64Ptr(2048),
		MimeType:     "application/zip",
	})
	if appErr != nil {
		t.Fatalf("CreateUploadSession(delivery) unexpected error: %+v", appErr)
	}
	if deliveryResult.Session == nil || deliveryResult.Session.UploadMode != domain.DesignAssetUploadModeSmall {
		t.Fatalf("CreateUploadSession(delivery) session = %+v", deliveryResult.Session)
	}

	largeResult, appErr := svc.CreateUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2008,
		CreatedBy:    540,
		AssetType:    domain.TaskAssetTypeSource,
		Filename:     "source.psd",
		ExpectedSize: uploadRequestInt64Ptr(taskAssetSinglePartThreshold + 1),
		MimeType:     "image/vnd.adobe.photoshop",
	})
	if appErr != nil {
		t.Fatalf("CreateUploadSession(large) unexpected error: %+v", appErr)
	}
	if largeResult.Session == nil || largeResult.Session.UploadMode != domain.DesignAssetUploadModeMultipart {
		t.Fatalf("CreateUploadSession(large) session = %+v", largeResult.Session)
	}
}

func TestTaskAssetCenterServiceResolveTaskAssetSourceModuleIDBackfillsCompletedLegacyTask(t *testing.T) {
	moduleRepo := newStep04TaskModuleRepo()
	svc := &taskAssetCenterService{taskModuleRepo: moduleRepo}
	task := &domain.Task{ID: 2199, TaskStatus: domain.TaskStatusCompleted}

	moduleID, err := svc.resolveTaskAssetSourceModuleID(taskAssetMutationTestContext(), step04Tx{}, task, domain.ModuleKeyCustomization)
	if err != nil {
		t.Fatalf("resolveTaskAssetSourceModuleID() error = %v", err)
	}
	if moduleID == nil || *moduleID <= 0 {
		t.Fatalf("resolveTaskAssetSourceModuleID() module_id = %+v", moduleID)
	}
	module := moduleRepo.modules[domain.ModuleKeyCustomization]
	if module == nil || module.State != domain.ModuleStateClosed {
		t.Fatalf("placeholder module = %+v", module)
	}
	if !strings.Contains(string(module.Data), `"backfill_placeholder":true`) {
		t.Fatalf("placeholder module data = %s", module.Data)
	}
}

func TestTaskAssetCenterServiceCreateUploadSessionRejectsExpectedSizeAboveLimit(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2031, TaskStatus: domain.TaskStatusPendingAudit})
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	svc := NewTaskAssetCenterService(taskRepo, newStep67DesignAssetRepo(), newStep04TaskAssetRepo(), newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, uploadClient).(*taskAssetCenterService)

	tooLarge := taskAssetUploadMaxFileSizeBytes + 1
	_, appErr := svc.CreateUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2031,
		CreatedBy:    540,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "delivery-too-large.zip",
		ExpectedSize: &tooLarge,
		MimeType:     "application/zip",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("CreateUploadSession() appErr = %+v, want invalid request", appErr)
	}
	if appErr.Message != "expected_size exceeds upload limit" {
		t.Fatalf("CreateUploadSession() message = %q, want expected_size exceeds upload limit", appErr.Message)
	}
	if len(uploadClient.createRequests) != 0 {
		t.Fatalf("remote create calls = %d, want 0", len(uploadClient.createRequests))
	}
}

func TestTaskAssetCenterServiceUploadContentTypeContractDefaultsAndRejectsMismatch(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2032, TaskStatus: domain.TaskStatusPendingAudit})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	storageRefRepo := newStep37AssetStorageRefRepo()
	taskEventRepo := &step04TaskEventRepo{}
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)

	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2032,
		CreatedBy:    541,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "delivery.bin",
		ExpectedSize: uploadRequestInt64Ptr(4096),
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}
	if createResult.Session == nil || createResult.Session.MimeType != "application/octet-stream" {
		t.Fatalf("session mime_type = %+v", createResult.Session)
	}
	if len(uploadClient.createRequests) != 1 || uploadClient.createRequests[0].MimeType != "application/octet-stream" {
		t.Fatalf("remote create mime_type = %+v", uploadClient.createRequests)
	}

	if _, appErr = svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:            2032,
		SessionID:         createResult.Session.ID,
		CompletedBy:       541,
		UploadContentType: "image/png",
	}); appErr == nil {
		t.Fatal("CompleteUploadSession() appErr = nil, want invalid request")
	} else {
		if appErr.Code != domain.ErrCodeInvalidRequest {
			t.Fatalf("CompleteUploadSession() code = %s", appErr.Code)
		}
		if appErr.Message != "upload_content_type must match upload_session required content type" {
			t.Fatalf("CompleteUploadSession() message = %q", appErr.Message)
		}
		details, ok := appErr.Details.(map[string]interface{})
		if !ok {
			t.Fatalf("appErr.Details type = %T", appErr.Details)
		}
		if got := details["required_upload_content_type"]; got != "application/octet-stream" {
			t.Fatalf("required_upload_content_type = %+v", got)
		}
	}
}

func TestTaskAssetCenterServiceCreateUploadSessionFreezesAssetIdentityBeforeUpload(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2030, TaskNo: "T-2030", TaskStatus: domain.TaskStatusPendingAudit})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	storageRefRepo := newStep37AssetStorageRefRepo()
	taskEventRepo := &step04TaskEventRepo{}
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)

	first, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2030,
		CreatedBy:    530,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "delivery-a.zip",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "application/zip",
	})
	if appErr != nil {
		t.Fatalf("first CreateMultipartUploadSession() error = %+v", appErr)
	}
	second, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2030,
		CreatedBy:    531,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "delivery-b.zip",
		ExpectedSize: uploadRequestInt64Ptr(2048),
		MimeType:     "application/zip",
	})
	if appErr != nil {
		t.Fatalf("second CreateMultipartUploadSession() error = %+v", appErr)
	}
	if first.Session == nil || first.Session.AssetID == nil || second.Session == nil || second.Session.AssetID == nil {
		t.Fatalf("session asset identity not frozen: first=%+v second=%+v", first.Session, second.Session)
	}
	if *first.Session.AssetID == *second.Session.AssetID {
		t.Fatalf("session asset ids should be unique: first=%d second=%d", *first.Session.AssetID, *second.Session.AssetID)
	}

	assets, err := designAssetRepo.ListByTaskID(taskAssetMutationTestContext(), 2030)
	if err != nil {
		t.Fatalf("ListByTaskID() error = %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("persisted assets = %d, want 2", len(assets))
	}

	if len(uploadClient.createRequests) != 2 {
		t.Fatalf("remote create requests = %d, want 2", len(uploadClient.createRequests))
	}
	firstReq := uploadClient.createRequests[0]
	secondReq := uploadClient.createRequests[1]
	if firstReq.AssetID == nil || secondReq.AssetID == nil {
		t.Fatalf("remote create request asset ids are missing: first=%+v second=%+v", firstReq, secondReq)
	}
	if *firstReq.AssetID == *secondReq.AssetID {
		t.Fatalf("remote create request asset ids drifted: first=%d second=%d", *firstReq.AssetID, *secondReq.AssetID)
	}
	if strings.TrimSpace(firstReq.AssetNo) == "" || strings.TrimSpace(secondReq.AssetNo) == "" || firstReq.AssetNo == secondReq.AssetNo {
		t.Fatalf("remote create request asset_no invalid: first=%q second=%q", firstReq.AssetNo, secondReq.AssetNo)
	}
}

func TestTaskAssetCenterServiceRepairMissingObjectArchivesStorageRef(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2033, TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	storageRefRepo := newStep37AssetStorageRefRepo()
	taskEventRepo := &step04TaskEventRepo{}
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.probeErr = &UploadServiceHTTPError{Operation: "probe_stored_file", StatusCode: http.StatusNotFound, Message: "not found"}

	assetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2033,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeDelivery,
		CreatedBy: 542,
	})
	versionNo := 1
	storageRefID := "ref-missing-1"
	taskAsset := &domain.TaskAsset{
		TaskID:         2033,
		AssetID:        &assetID,
		AssetType:      domain.TaskAssetTypeDelivery,
		VersionNo:      1,
		AssetVersionNo: &versionNo,
		UploadMode:     strPtr("multipart"),
		FileName:       "delivery.png",
		OriginalName:   strPtr("delivery.png"),
		StorageRefID:   strPtr(storageRefID),
		StorageKey:     strPtr("objects/design-assets/missing-delivery.png"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     542,
		UploadedAt:     timeValuePtr(time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)),
		StorageRef: &domain.AssetStorageRef{
			RefID:          storageRefID,
			OwnerType:      domain.AssetOwnerTypeTaskAsset,
			OwnerID:        1,
			StorageAdapter: domain.AssetStorageAdapterOSSUploadService,
			RefType:        domain.AssetStorageRefTypeTaskAssetObject,
			RefKey:         "objects/design-assets/missing-delivery.png",
			Status:         domain.AssetStorageRefStatusRecorded,
		},
	}
	versionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, taskAsset)
	_ = designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, assetID, &versionID)
	storageRefRepo.refs[storageRefID] = taskAsset.StorageRef

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)

	repaired, appErr := svc.repairMissingObjectStorageRef(taskAssetMutationTestContext(), versionID)
	if appErr != nil {
		t.Fatalf("repairMissingObjectStorageRef() unexpected error: %+v", appErr)
	}
	if !repaired {
		t.Fatal("repairMissingObjectStorageRef() repaired = false, want true")
	}
	if storageRefRepo.refs[storageRefID].Status != domain.AssetStorageRefStatusArchived {
		t.Fatalf("storage ref status = %s", storageRefRepo.refs[storageRefID].Status)
	}

	asset, appErr := svc.GetAsset(taskAssetMutationTestContext(), assetID)
	if appErr != nil {
		t.Fatalf("GetAsset() unexpected error: %+v", appErr)
	}
	if asset.CurrentVersion == nil || asset.CurrentVersion.StorageKey != "" {
		t.Fatalf("current version after repair = %+v", asset.CurrentVersion)
	}

	if _, appErr = svc.GetAssetDownloadInfoByID(taskAssetMutationTestContext(), assetID); appErr == nil {
		t.Fatal("GetAssetDownloadInfoByID() appErr = nil, want ASSET_MISSING")
	} else if appErr.Code != domain.ErrCodeAssetMissing {
		t.Fatalf("GetAssetDownloadInfoByID() code = %s", appErr.Code)
	}
}

func TestResolveCompletedUploadMeta_OSSDirectFinalizeSkipsUploadServiceComplete(t *testing.T) {
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	svc := &taskAssetCenterService{
		uploadClient: uploadClient,
		nowFn: func() time.Time {
			return time.Date(2026, 4, 14, 1, 40, 0, 0, time.UTC)
		},
	}
	request := &domain.UploadRequest{
		UploadMode:    domain.DesignAssetUploadModeMultipart,
		SessionStatus: domain.DesignAssetSessionStatusCreated,
		FileName:      "proof.bin",
		MimeType:      "application/octet-stream",
		ExpectedSize:  uploadRequestInt64Ptr(4096),
	}

	meta, appErr := svc.resolveCompletedUploadMeta(
		taskAssetMutationTestContext(),
		request,
		"abc123",
		"tasks/T1/assets/A1/v1/delivery/proof.bin",
		true,
	)
	if appErr != nil {
		t.Fatalf("resolveCompletedUploadMeta() unexpected error: %+v", appErr)
	}
	if meta == nil {
		t.Fatal("resolveCompletedUploadMeta() meta = nil")
	}
	if meta.StorageKey != "tasks/T1/assets/A1/v1/delivery/proof.bin" {
		t.Fatalf("storage key = %q", meta.StorageKey)
	}
	if meta.FileHash == nil || *meta.FileHash != "abc123" {
		t.Fatalf("file hash = %+v", meta.FileHash)
	}
	if meta.FileSize == nil || *meta.FileSize != 4096 {
		t.Fatalf("file size = %+v", meta.FileSize)
	}
	if meta.MimeType != "application/octet-stream" {
		t.Fatalf("mime type = %q", meta.MimeType)
	}
	if uploadClient.completeCalls != 0 {
		t.Fatalf("upload service complete calls = %d, want 0", uploadClient.completeCalls)
	}
}

func TestTaskAssetCenterServiceDownloadInfoPrefersDirectBrowserURL(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2031, TaskNo: "T-2031", TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()

	assetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2031,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeDelivery,
		CreatedBy: 631,
	})
	versionNo := 1
	versionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{
		TaskID:         2031,
		AssetID:        &assetID,
		AssetType:      domain.TaskAssetTypeDelivery,
		VersionNo:      1,
		AssetVersionNo: &versionNo,
		UploadMode:     strPtr("multipart"),
		FileName:       "delivery.png",
		OriginalName:   strPtr("delivery.png"),
		StorageKey:     strPtr("objects/design-assets/delivery.png"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     631,
		UploadedAt:     timeValuePtr(time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)),
	})
	_ = designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, assetID, &versionID)

	directSvc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient())
	directInfo, appErr := directSvc.GetAssetDownloadInfoByID(taskAssetMutationTestContext(), assetID)
	if appErr != nil {
		t.Fatalf("direct GetAssetDownloadInfoByID() error = %+v", appErr)
	}
	if directInfo.DownloadMode != domain.AssetDownloadModeDirect || directInfo.DownloadURL == nil || !strings.Contains(*directInfo.DownloadURL, "https://oss-browser.test/files/") {
		t.Fatalf("direct download info = %+v", directInfo)
	}

	proxySvc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, nil)
	proxyInfo, appErr := proxySvc.GetAssetDownloadInfoByID(taskAssetMutationTestContext(), assetID)
	if appErr != nil {
		t.Fatalf("proxy GetAssetDownloadInfoByID() error = %+v", appErr)
	}
	if proxyInfo.DownloadMode != domain.AssetDownloadModeProxy || proxyInfo.DownloadURL == nil || !strings.HasPrefix(*proxyInfo.DownloadURL, "/v1/assets/files/") {
		t.Fatalf("proxy download info = %+v", proxyInfo)
	}
}

func TestTaskAssetCenterServiceListAssetResourcesAndPreviewByID(t *testing.T) {
	taskRepo := newStep04TaskRepo(
		&domain.Task{ID: 2009, TaskNo: "T-2009", TaskStatus: domain.TaskStatusInProgress},
		&domain.Task{ID: 2010, TaskNo: "T-2010", TaskStatus: domain.TaskStatusInProgress},
	)
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()

	previewAssetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{
		TaskID:       2009,
		AssetNo:      "AST-0001",
		ScopeSKUCode: "SKU-2009-A",
		AssetType:    domain.TaskAssetTypePreview,
		CreatedBy:    601,
	})
	previewVersionNo := 1
	previewVersionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{
		TaskID:         2009,
		AssetID:        &previewAssetID,
		ScopeSKUCode:   strPtr("SKU-2009-A"),
		AssetType:      domain.TaskAssetTypePreview,
		VersionNo:      1,
		AssetVersionNo: &previewVersionNo,
		UploadMode:     strPtr("multipart"),
		FileName:       "preview.png",
		OriginalName:   strPtr("preview.png"),
		MimeType:       strPtr("image/png"),
		StorageKey:     strPtr("objects/design-assets/preview.png"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     601,
		UploadedAt:     timeValuePtr(time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)),
	})
	_ = designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, previewAssetID, &previewVersionID)

	sourceAssetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2010,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeSource,
		CreatedBy: 602,
	})
	sourceVersionNo := 1
	sourceVersionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{
		TaskID:         2010,
		AssetID:        &sourceAssetID,
		AssetType:      domain.TaskAssetTypeSource,
		VersionNo:      1,
		AssetVersionNo: &sourceVersionNo,
		UploadMode:     strPtr("multipart"),
		FileName:       "source.psd",
		OriginalName:   strPtr("source.psd"),
		MimeType:       strPtr("image/vnd.adobe.photoshop"),
		StorageKey:     strPtr("objects/design-assets/source.psd"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     602,
		UploadedAt:     timeValuePtr(time.Date(2026, 3, 14, 11, 0, 0, 0, time.UTC)),
	})
	_ = designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, sourceAssetID, &sourceVersionID)

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient()).(*taskAssetCenterService)

	list, appErr := svc.ListAssetResources(taskAssetMutationTestContext(), ListAssetResourcesParams{
		TaskID:       uploadRequestInt64Ptr(2009),
		AssetType:    domain.TaskAssetTypePreview,
		ScopeSKUCode: "SKU-2009-A",
		UploadStatus: domain.DesignAssetUploadStatusUploaded,
	})
	if appErr != nil {
		t.Fatalf("ListAssetResources() unexpected error: %+v", appErr)
	}
	if len(list) != 1 || list[0].ID != previewAssetID {
		t.Fatalf("ListAssetResources() = %+v, want preview asset only", list)
	}
	if list[0].ArchiveStatus != domain.AssetArchiveStatusActive || list[0].UploadStatus != domain.DesignAssetUploadStatusUploaded {
		t.Fatalf("ListAssetResources() summaries = %+v", list[0])
	}

	info, appErr := svc.GetAssetPreviewInfoByID(taskAssetMutationTestContext(), previewAssetID)
	if appErr != nil {
		t.Fatalf("GetAssetPreviewInfoByID() unexpected error: %+v", appErr)
	}
	if info == nil || !info.PreviewAvailable || info.DownloadURL == nil {
		t.Fatalf("GetAssetPreviewInfoByID() = %+v", info)
	}

	_, appErr = svc.GetAssetPreviewInfoByID(taskAssetMutationTestContext(), sourceAssetID)
	if appErr == nil {
		t.Fatal("GetAssetPreviewInfoByID(source) appErr = nil, want invalid state")
	}
}

func TestTaskAssetCenterServiceStagedPreviewRequiresUploaderOrScopedAuditor(t *testing.T) {
	const (
		taskID     = int64(2049)
		uploaderID = int64(701)
	)
	taskRepo := newStep04TaskRepo(&domain.Task{ID: taskID, TaskNo: "T-2049", TaskStatus: domain.TaskStatusInProgress, CreatorID: uploaderID})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	assetID, _ := designAssetRepo.Create(context.Background(), step04Tx{}, &domain.DesignAsset{
		TaskID: taskID, AssetNo: "AST-0001", AssetType: domain.TaskAssetTypePreview, CreatedBy: uploaderID,
	})
	versionNo := 1
	versionID, _ := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
		TaskID: taskID, AssetID: &assetID, AssetType: domain.TaskAssetTypePreview,
		VersionNo: 1, AssetVersionNo: &versionNo, UploadMode: strPtr("multipart"),
		FileName: "staged-preview.png", OriginalName: strPtr("staged-preview.png"), MimeType: strPtr("image/png"),
		StorageKey: strPtr("objects/design-assets/staged-preview.png"), UploadStatus: strPtr("uploaded"),
		PreviewStatus: strPtr("not_applicable"), UploadedBy: uploaderID,
	})
	taskAssetRepo.stagedPreviewByRoot[assetID] = &domain.StagedTaskAssetPreviewAccess{TaskAssetID: versionID, TaskID: taskID, StagedBy: uploaderID}
	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient())

	if _, appErr := svc.GetAssetPreviewInfoByID(taskAssetPreviewTestContext(uploaderID, domain.PermissionAssetView, domain.AccessScopeSelf), assetID); appErr != nil {
		t.Fatalf("uploader preview error = %+v", appErr)
	}
	if _, appErr := svc.GetAssetPreviewInfoByID(taskAssetPreviewTestContext(702, domain.PermissionAssetView, domain.AccessScopeGlobal), assetID); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("non-uploader asset viewer error = %+v, want permission denied", appErr)
	}
	if _, appErr := svc.GetAssetPreviewInfoByID(taskAssetPreviewTestContext(703, domain.PermissionTaskAuditDecision, domain.AccessScopeGlobal), assetID); appErr != nil {
		t.Fatalf("scoped auditor preview error = %+v", appErr)
	}
	if _, appErr := svc.GetAssetPreviewInfoByID(taskAssetPreviewTestContext(uploaderID, domain.PermissionTaskView, domain.AccessScopeGlobal), assetID); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("uploader without asset.view error = %+v, want permission denied", appErr)
	}
}

func TestTaskAssetCenterServiceSourceDirectPreviewUsesOSSIMGProcess(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2050, TaskNo: "T-2050", TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()

	sourceAssetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2050,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeSource,
		CreatedBy: 650,
	})
	sourceVersionNo := 1
	sourceVersionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{
		TaskID:         2050,
		AssetID:        &sourceAssetID,
		AssetType:      domain.TaskAssetTypeSource,
		VersionNo:      1,
		AssetVersionNo: &sourceVersionNo,
		UploadMode:     strPtr("multipart"),
		FileName:       "hero.tiff",
		OriginalName:   strPtr("hero.tiff"),
		MimeType:       strPtr("image/tiff"),
		StorageKey:     strPtr("objects/design-assets/hero.tiff"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     650,
		UploadedAt:     timeValuePtr(time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC)),
	})
	_ = designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, sourceAssetID, &sourceVersionID)

	svc := NewTaskAssetCenterService(
		taskRepo,
		designAssetRepo,
		taskAssetRepo,
		newStep37UploadRequestRepo(),
		newStep37AssetStorageRefRepo(),
		&step04TaskEventRepo{},
		step04TxRunner{},
		newStubUploadServiceClient(),
		WithOSSDirectService(newTestOSSDirectService()),
	).(*taskAssetCenterService)

	previewInfo, appErr := svc.GetAssetPreviewInfoByID(taskAssetMutationTestContext(), sourceAssetID)
	if appErr != nil {
		t.Fatalf("GetAssetPreviewInfoByID() unexpected error: %+v", appErr)
	}
	if previewInfo == nil || previewInfo.DownloadURL == nil {
		t.Fatalf("GetAssetPreviewInfoByID() = %+v", previewInfo)
	}
	if !strings.Contains(*previewInfo.DownloadURL, "x-oss-process=") {
		t.Fatalf("preview url = %q, want x-oss-process", *previewInfo.DownloadURL)
	}
	if !strings.Contains(*previewInfo.DownloadURL, "resize%2Cw_1600%2Cm_lfit") {
		t.Fatalf("preview url = %q, want resize transform", *previewInfo.DownloadURL)
	}
	if !strings.Contains(*previewInfo.DownloadURL, "format%2Cjpg") {
		t.Fatalf("preview url = %q, want jpg conversion", *previewInfo.DownloadURL)
	}
	if previewInfo.MimeType != "image/jpeg" {
		t.Fatalf("preview mime_type = %q, want image/jpeg", previewInfo.MimeType)
	}

	downloadInfo, appErr := svc.GetAssetDownloadInfoByID(taskAssetMutationTestContext(), sourceAssetID)
	if appErr != nil {
		t.Fatalf("GetAssetDownloadInfoByID() unexpected error: %+v", appErr)
	}
	if downloadInfo == nil || downloadInfo.DownloadURL == nil {
		t.Fatalf("GetAssetDownloadInfoByID() = %+v", downloadInfo)
	}
	if strings.Contains(*downloadInfo.DownloadURL, "x-oss-process=") {
		t.Fatalf("download url = %q, should not include x-oss-process", *downloadInfo.DownloadURL)
	}
}

func TestTaskAssetCenterServiceDeliveryTiffPreviewUsesOSSIMGProcess(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2052, TaskNo: "T-2052", TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()

	deliveryAssetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2052,
		AssetNo:   "AST-0002",
		AssetType: domain.TaskAssetTypeDelivery,
		CreatedBy: 652,
	})
	deliveryVersionNo := 1
	deliveryVersionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{
		TaskID:         2052,
		AssetID:        &deliveryAssetID,
		AssetType:      domain.TaskAssetTypeDelivery,
		VersionNo:      1,
		AssetVersionNo: &deliveryVersionNo,
		UploadMode:     strPtr("multipart"),
		FileName:       "final.tif",
		OriginalName:   strPtr("final.tif"),
		MimeType:       strPtr("image/tiff"),
		StorageKey:     strPtr("objects/design-assets/final.tif"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     652,
		UploadedAt:     timeValuePtr(time.Date(2026, 4, 15, 9, 20, 0, 0, time.UTC)),
	})
	_ = designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, deliveryAssetID, &deliveryVersionID)

	svc := NewTaskAssetCenterService(
		taskRepo,
		designAssetRepo,
		taskAssetRepo,
		newStep37UploadRequestRepo(),
		newStep37AssetStorageRefRepo(),
		&step04TaskEventRepo{},
		step04TxRunner{},
		newStubUploadServiceClient(),
		WithOSSDirectService(newTestOSSDirectService()),
	).(*taskAssetCenterService)

	previewInfo, appErr := svc.GetAssetPreviewInfoByID(taskAssetMutationTestContext(), deliveryAssetID)
	if appErr != nil {
		t.Fatalf("GetAssetPreviewInfoByID() unexpected error: %+v", appErr)
	}
	if previewInfo == nil || previewInfo.DownloadURL == nil {
		t.Fatalf("GetAssetPreviewInfoByID() = %+v", previewInfo)
	}
	if !strings.Contains(*previewInfo.DownloadURL, "x-oss-process=") {
		t.Fatalf("preview url = %q, want x-oss-process", *previewInfo.DownloadURL)
	}
	if !strings.Contains(*previewInfo.DownloadURL, "format%2Cjpg") {
		t.Fatalf("preview url = %q, want jpg conversion", *previewInfo.DownloadURL)
	}
	if previewInfo.MimeType != "image/jpeg" {
		t.Fatalf("preview mime_type = %q, want image/jpeg", previewInfo.MimeType)
	}

	downloadInfo, appErr := svc.GetAssetDownloadInfoByID(taskAssetMutationTestContext(), deliveryAssetID)
	if appErr != nil {
		t.Fatalf("GetAssetDownloadInfoByID() unexpected error: %+v", appErr)
	}
	if downloadInfo == nil || downloadInfo.DownloadURL == nil {
		t.Fatalf("GetAssetDownloadInfoByID() = %+v", downloadInfo)
	}
	if strings.Contains(*downloadInfo.DownloadURL, "x-oss-process=") {
		t.Fatalf("download url = %q, should not include x-oss-process", *downloadInfo.DownloadURL)
	}
}

func TestTaskAssetCenterServiceSourcePreviewFallsBackToDerivedPreviewAsset(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2051, TaskNo: "T-2051", TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()

	sourceAssetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2051,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeSource,
		CreatedBy: 651,
	})
	sourceVersionNo := 1
	sourceVersionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{
		TaskID:         2051,
		AssetID:        &sourceAssetID,
		AssetType:      domain.TaskAssetTypeSource,
		VersionNo:      1,
		AssetVersionNo: &sourceVersionNo,
		UploadMode:     strPtr("multipart"),
		FileName:       "source.psd",
		OriginalName:   strPtr("source.psd"),
		MimeType:       strPtr("image/vnd.adobe.photoshop"),
		StorageKey:     strPtr("objects/design-assets/source.psd"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     651,
		UploadedAt:     timeValuePtr(time.Date(2026, 4, 15, 9, 10, 0, 0, time.UTC)),
	})
	_ = designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, sourceAssetID, &sourceVersionID)

	derivedPreviewAssetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{
		TaskID:        2051,
		AssetNo:       "AST-0002",
		SourceAssetID: &sourceAssetID,
		AssetType:     domain.TaskAssetTypePreview,
		CreatedBy:     651,
	})
	derivedVersionNo := 1
	derivedVersionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{
		TaskID:               2051,
		AssetID:              &derivedPreviewAssetID,
		AssetType:            domain.TaskAssetTypePreview,
		VersionNo:            2,
		AssetVersionNo:       &derivedVersionNo,
		UploadMode:           strPtr("small"),
		FileName:             "preview.png",
		OriginalName:         strPtr("preview.png"),
		MimeType:             strPtr("image/png"),
		StorageKey:           strPtr("objects/design-assets/source-preview.png"),
		UploadStatus:         strPtr("uploaded"),
		PreviewStatus:        strPtr("not_applicable"),
		UploadedBy:           651,
		UploadedAt:           timeValuePtr(time.Date(2026, 4, 15, 9, 12, 0, 0, time.UTC)),
		SourceAssetVersionID: &sourceVersionID,
	})
	_ = designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, derivedPreviewAssetID, &derivedVersionID)

	svc := NewTaskAssetCenterService(
		taskRepo,
		designAssetRepo,
		taskAssetRepo,
		newStep37UploadRequestRepo(),
		newStep37AssetStorageRefRepo(),
		&step04TaskEventRepo{},
		step04TxRunner{},
		newStubUploadServiceClient(),
	).(*taskAssetCenterService)

	info, appErr := svc.GetAssetPreviewInfoByID(taskAssetMutationTestContext(), sourceAssetID)
	if appErr != nil {
		t.Fatalf("GetAssetPreviewInfoByID(source) unexpected error: %+v", appErr)
	}
	if info == nil || info.DownloadURL == nil {
		t.Fatalf("GetAssetPreviewInfoByID(source) = %+v", info)
	}
	if !strings.Contains(*info.DownloadURL, "objects/design-assets/source-preview.png") {
		t.Fatalf("preview url = %q, want derived preview storage key", *info.DownloadURL)
	}
}

func TestTaskAssetCenterServiceCompleteSourceUploadGeneratesDerivedPreviewAssets(t *testing.T) {
	ossServer := newFakeOSSDirectServer(t)
	defer ossServer.Close()

	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2052, TaskNo: "T-2052", TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)

	ossDirect := NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        ossServer.EndpointHost(),
		PublicEndpoint:  ossServer.EndpointHost(),
		Bucket:          "test-bucket",
		AccessKeyID:     "test-key",
		AccessKeySecret: "test-secret",
		PresignExpiry:   15 * time.Minute,
		PartSize:        10 * 1024 * 1024,
	})
	ossDirect.httpClient = ossServer.Client()

	svc := NewTaskAssetCenterService(
		taskRepo,
		designAssetRepo,
		taskAssetRepo,
		uploadRequestRepo,
		storageRefRepo,
		taskEventRepo,
		step04TxRunner{},
		uploadClient,
		WithOSSDirectService(ossDirect),
		WithTaskAssetCenterPreviewRenderer(testPreviewRenderer{}),
	).(*taskAssetCenterService)
	svc.runAsyncFn = func(fn func()) { fn() }
	svc.derivedPreviewGracePeriod = 0

	body := bytes.Repeat([]byte("source-psd-binary"), 80)
	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2052,
		CreatedBy:    652,
		AssetType:    domain.TaskAssetTypeSource,
		Filename:     "source.psd",
		ExpectedSize: uploadRequestInt64Ptr(int64(len(body))),
		MimeType:     "image/vnd.adobe.photoshop",
		FileHash:     "source-psd-hash",
		Remark:       "source upload",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}
	if createResult == nil || createResult.OSSDirect == nil || len(createResult.OSSDirect.Parts) == 0 {
		t.Fatalf("CreateMultipartUploadSession() oss_direct = %+v", createResult)
	}

	uploadedParts := make([]OSSCompletePart, 0, len(createResult.OSSDirect.Parts))
	for _, part := range createResult.OSSDirect.Parts {
		req, err := http.NewRequest(part.Method, part.UploadURL, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Content-Type", createResult.OSSDirect.RequiredContentType)
		resp, err := ossServer.Client().Do(req)
		if err != nil {
			t.Fatalf("direct PUT error = %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("direct PUT status = %d", resp.StatusCode)
		}
		uploadedParts = append(uploadedParts, OSSCompletePart{
			PartNumber: part.PartNumber,
			ETag:       strings.TrimSpace(resp.Header.Get("ETag")),
		})
	}

	completeResult, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:            2052,
		SessionID:         createResult.Session.ID,
		CompletedBy:       652,
		FileHash:          "source-psd-hash",
		UploadContentType: createResult.OSSDirect.RequiredContentType,
		OSSParts:          uploadedParts,
		OSSUploadID:       createResult.OSSDirect.UploadID,
		OSSObjectKey:      createResult.OSSDirect.ObjectKey,
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession() unexpected error: %+v", appErr)
	}
	if completeResult == nil || completeResult.Asset == nil || completeResult.Version == nil {
		t.Fatalf("CompleteUploadSession() result = %+v", completeResult)
	}

	sourceAssetID := completeResult.Asset.ID
	previewAssets, appErr := svc.ListAssetResources(taskAssetMutationTestContext(), ListAssetResourcesParams{
		TaskID:        uploadRequestInt64Ptr(2052),
		SourceAssetID: &sourceAssetID,
		AssetType:     domain.TaskAssetTypePreview,
	})
	if appErr != nil {
		t.Fatalf("ListAssetResources(preview) unexpected error: %+v", appErr)
	}
	if len(previewAssets) == 0 || previewAssets[0].CurrentVersion == nil {
		t.Fatalf("preview derived assets = %+v", previewAssets)
	}
	thumbAssets, appErr := svc.ListAssetResources(taskAssetMutationTestContext(), ListAssetResourcesParams{
		TaskID:        uploadRequestInt64Ptr(2052),
		SourceAssetID: &sourceAssetID,
		AssetType:     domain.TaskAssetTypeDesignThumb,
	})
	if appErr != nil {
		t.Fatalf("ListAssetResources(design_thumb) unexpected error: %+v", appErr)
	}
	if len(thumbAssets) == 0 || thumbAssets[0].CurrentVersion == nil {
		t.Fatalf("design_thumb derived assets = %+v", thumbAssets)
	}

	previewInfo, appErr := svc.GetAssetPreviewInfoByID(taskAssetMutationTestContext(), sourceAssetID)
	if appErr != nil {
		t.Fatalf("GetAssetPreviewInfoByID(source) unexpected error: %+v", appErr)
	}
	if previewInfo == nil || previewInfo.DownloadURL == nil {
		t.Fatalf("GetAssetPreviewInfoByID(source) = %+v", previewInfo)
	}
	if !strings.Contains(*previewInfo.DownloadURL, previewAssets[0].CurrentVersion.StorageKey) {
		t.Fatalf("preview url = %q, want derived preview key %q", *previewInfo.DownloadURL, previewAssets[0].CurrentVersion.StorageKey)
	}

	downloadInfo, appErr := svc.GetAssetDownloadInfoByID(taskAssetMutationTestContext(), sourceAssetID)
	if appErr != nil {
		t.Fatalf("GetAssetDownloadInfoByID(source) unexpected error: %+v", appErr)
	}
	if downloadInfo == nil || downloadInfo.DownloadURL == nil {
		t.Fatalf("GetAssetDownloadInfoByID(source) = %+v", downloadInfo)
	}
	if !strings.Contains(*downloadInfo.DownloadURL, completeResult.Version.StorageKey) {
		t.Fatalf("download url = %q, want source key %q", *downloadInfo.DownloadURL, completeResult.Version.StorageKey)
	}

	oldPreviewVersionID := previewAssets[0].CurrentVersion.ID
	oldPreviewStorageKey := previewAssets[0].CurrentVersion.StorageKey
	replacementBody := bytes.Repeat([]byte("replacement-source-psd-binary"), 80)
	replacementStorageKey := "tasks/T-2052/assets/AST-0001/v2/source-replacement.psd"
	if err := ossDirect.UploadObject(taskAssetMutationTestContext(), replacementStorageKey, "image/vnd.adobe.photoshop", replacementBody); err != nil {
		t.Fatalf("UploadObject(replacement source) error = %v", err)
	}
	replacementVersionNo := 2
	replacementTimelineVersion, _ := taskAssetRepo.NextVersionNo(taskAssetMutationTestContext(), step04Tx{}, 2052)
	replacementSourceVersionID, err := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{
		TaskID:           2052,
		AssetID:          &sourceAssetID,
		AssetType:        domain.TaskAssetTypeSource,
		VersionNo:        replacementTimelineVersion,
		AssetVersionNo:   &replacementVersionNo,
		UploadMode:       strPtr("multipart"),
		FileName:         "source-replacement.psd",
		OriginalName:     strPtr("source-replacement.psd"),
		MimeType:         strPtr("image/vnd.adobe.photoshop"),
		FileSize:         uploadRequestInt64Ptr(int64(len(replacementBody))),
		StorageKey:       &replacementStorageKey,
		UploadStatus:     strPtr("uploaded"),
		PreviewStatus:    strPtr("not_applicable"),
		UploadedBy:       652,
		UploadedAt:       timeValuePtr(time.Date(2026, 4, 15, 9, 30, 0, 0, time.UTC)),
		FlowReviewStatus: domain.TaskAssetFlowReviewStatusNotApplicable,
	})
	if err != nil {
		t.Fatalf("create replacement source version: %v", err)
	}
	if err := designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, sourceAssetID, &replacementSourceVersionID); err != nil {
		t.Fatalf("set replacement source current version: %v", err)
	}
	if err := svc.ensureDerivedPreviewAssets(taskAssetMutationTestContext(), 2052, sourceAssetID, 652); err != nil {
		t.Fatalf("ensureDerivedPreviewAssets(replacement) error = %v", err)
	}

	previewAssets, appErr = svc.ListAssetResources(taskAssetMutationTestContext(), ListAssetResourcesParams{
		TaskID:        uploadRequestInt64Ptr(2052),
		SourceAssetID: &sourceAssetID,
		AssetType:     domain.TaskAssetTypePreview,
	})
	if appErr != nil || len(previewAssets) == 0 || previewAssets[0].CurrentVersion == nil {
		t.Fatalf("replacement preview derived assets = %+v, appErr = %+v", previewAssets, appErr)
	}
	newPreview := previewAssets[0].CurrentVersion
	if newPreview.ID == oldPreviewVersionID || newPreview.StorageKey == oldPreviewStorageKey {
		t.Fatalf("replacement preview reused stale version: old=%d/%q new=%d/%q", oldPreviewVersionID, oldPreviewStorageKey, newPreview.ID, newPreview.StorageKey)
	}
	if newPreview.SourceAssetVersionID == nil || *newPreview.SourceAssetVersionID != replacementSourceVersionID {
		t.Fatalf("replacement preview source version = %v, want %d", newPreview.SourceAssetVersionID, replacementSourceVersionID)
	}
	if got := taskAssetRepo.assets[oldPreviewVersionID].FlowReviewStatus; got != domain.TaskAssetFlowReviewStatusSuperseded {
		t.Fatalf("old preview version status = %q, want superseded", got)
	}
}

func TestTaskAssetCenterServiceCompleteSourceThenDeliverySeriallyWithout500(t *testing.T) {
	ossServer := newFakeOSSDirectServer(t)
	defer ossServer.Close()

	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2053, TaskNo: "T-2053", TaskStatus: domain.TaskStatusPendingAudit})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted

	ossDirect := NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        ossServer.EndpointHost(),
		PublicEndpoint:  ossServer.EndpointHost(),
		Bucket:          "test-bucket",
		AccessKeyID:     "test-key",
		AccessKeySecret: "test-secret",
		PresignExpiry:   15 * time.Minute,
		PartSize:        10 * 1024 * 1024,
	})
	ossDirect.httpClient = ossServer.Client()

	svc := NewTaskAssetCenterService(
		taskRepo,
		designAssetRepo,
		taskAssetRepo,
		uploadRequestRepo,
		storageRefRepo,
		taskEventRepo,
		step04TxRunner{},
		uploadClient,
		WithOSSDirectService(ossDirect),
		WithTaskAssetCenterPreviewRenderer(testPreviewRenderer{}),
	).(*taskAssetCenterService)

	var derivedJobs []func()
	svc.runAsyncFn = func(fn func()) {
		derivedJobs = append(derivedJobs, fn)
	}

	sourceBody := bytes.Repeat([]byte("serial-source-psd"), 80)
	sourceSession, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2053,
		CreatedBy:    653,
		AssetType:    domain.TaskAssetTypeSource,
		Filename:     "source.psd",
		ExpectedSize: uploadRequestInt64Ptr(int64(len(sourceBody))),
		MimeType:     "image/vnd.adobe.photoshop",
		FileHash:     "serial-source-hash",
		Remark:       "serial source upload",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession(source) unexpected error: %+v", appErr)
	}
	uploadedParts := make([]OSSCompletePart, 0, len(sourceSession.OSSDirect.Parts))
	for _, part := range sourceSession.OSSDirect.Parts {
		req, err := http.NewRequest(part.Method, part.UploadURL, bytes.NewReader(sourceBody))
		if err != nil {
			t.Fatalf("http.NewRequest(source part) error = %v", err)
		}
		req.Header.Set("Content-Type", sourceSession.OSSDirect.RequiredContentType)
		resp, err := ossServer.Client().Do(req)
		if err != nil {
			t.Fatalf("direct PUT(source) error = %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("direct PUT(source) status = %d", resp.StatusCode)
		}
		uploadedParts = append(uploadedParts, OSSCompletePart{
			PartNumber: part.PartNumber,
			ETag:       strings.TrimSpace(resp.Header.Get("ETag")),
		})
	}

	sourceResult, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:            2053,
		SessionID:         sourceSession.Session.ID,
		CompletedBy:       653,
		FileHash:          "serial-source-hash",
		UploadContentType: sourceSession.OSSDirect.RequiredContentType,
		OSSParts:          uploadedParts,
		OSSUploadID:       sourceSession.OSSDirect.UploadID,
		OSSObjectKey:      sourceSession.OSSDirect.ObjectKey,
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession(source) unexpected error: %+v", appErr)
	}
	if sourceResult == nil || sourceResult.Version == nil {
		t.Fatalf("CompleteUploadSession(source) result = %+v", sourceResult)
	}

	deliverySession, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2053,
		CreatedBy:    653,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "delivery.jpg",
		ExpectedSize: uploadRequestInt64Ptr(2048),
		MimeType:     "image/jpeg",
		FileHash:     "serial-delivery-hash",
		Remark:       "serial delivery upload",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession(delivery) unexpected error: %+v", appErr)
	}
	deliveryBody := bytes.Repeat([]byte("d"), 2048)
	deliveryParts := make([]OSSCompletePart, 0, len(deliverySession.OSSDirect.Parts))
	for _, part := range deliverySession.OSSDirect.Parts {
		req, err := http.NewRequest(part.Method, part.UploadURL, bytes.NewReader(deliveryBody))
		if err != nil {
			t.Fatalf("http.NewRequest(delivery part) error = %v", err)
		}
		req.Header.Set("Content-Type", deliverySession.OSSDirect.RequiredContentType)
		resp, err := ossServer.Client().Do(req)
		if err != nil {
			t.Fatalf("direct PUT(delivery) error = %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("direct PUT(delivery) status = %d", resp.StatusCode)
		}
		deliveryParts = append(deliveryParts, OSSCompletePart{
			PartNumber: part.PartNumber,
			ETag:       strings.TrimSpace(resp.Header.Get("ETag")),
		})
	}
	deliveryResult, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:            2053,
		SessionID:         deliverySession.Session.ID,
		CompletedBy:       653,
		FileHash:          "serial-delivery-hash",
		UploadContentType: deliverySession.OSSDirect.RequiredContentType,
		OSSParts:          deliveryParts,
		OSSUploadID:       deliverySession.OSSDirect.UploadID,
		OSSObjectKey:      deliverySession.OSSDirect.ObjectKey,
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession(delivery) unexpected error: %+v", appErr)
	}
	if deliveryResult == nil || deliveryResult.Version == nil {
		t.Fatalf("CompleteUploadSession(delivery) result = %+v", deliveryResult)
	}
	if countStep04TaskEvents(taskEventRepo.events, domain.TaskEventAssetVersionCreated) != 2 {
		t.Fatalf("asset version created events = %+v, want 2 user-driven versions before derived jobs", taskEventRepo.events)
	}

	for _, job := range derivedJobs {
		job()
	}
}

func TestTaskAssetCenterServiceRetouchDeliveryBatchCompletesAfterLastPreparedSession(t *testing.T) {
	designerID := int64(701)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:               2060,
		TaskNo:           "T-2060",
		TaskType:         domain.TaskTypeRetouchTask,
		TaskStatus:       domain.TaskStatusInProgress,
		DesignerID:       &designerID,
		CurrentHandlerID: &designerID,
	})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 6, 9, 13, 33, 5, 0, time.UTC)
	}
	ctx := taskAssetMutationTestContext()

	firstSession, appErr := svc.CreateMultipartUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:       2060,
		CreatedBy:    designerID,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "main-01.jpg",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "image/jpeg",
		FileHash:     "main-01-hash",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession(first) unexpected error: %+v", appErr)
	}
	secondSession, appErr := svc.CreateMultipartUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:       2060,
		CreatedBy:    designerID,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "main-02.jpg",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "image/jpeg",
		FileHash:     "main-02-hash",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession(second) unexpected error: %+v", appErr)
	}

	if _, appErr = svc.CompleteUploadSession(ctx, CompleteTaskAssetUploadSessionParams{
		TaskID:      2060,
		SessionID:   firstSession.Session.ID,
		CompletedBy: designerID,
		FileHash:    "main-01-hash",
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession(first) unexpected error: %+v", appErr)
	}
	if got := taskRepo.tasks[2060].TaskStatus; got != domain.TaskStatusInProgress {
		t.Fatalf("task status after first delivery = %s, want InProgress", got)
	}

	if _, appErr = svc.CompleteUploadSession(ctx, CompleteTaskAssetUploadSessionParams{
		TaskID:      2060,
		SessionID:   secondSession.Session.ID,
		CompletedBy: designerID,
		FileHash:    "main-02-hash",
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession(second) unexpected error: %+v", appErr)
	}
	if got := taskRepo.tasks[2060].TaskStatus; got != domain.TaskStatusInProgress {
		t.Fatalf("task status after last delivery = %s, want InProgress until resource-group submission", got)
	}
	if countStep04TaskEvents(taskEventRepo.events, domain.TaskEventAssetVersionCreated) != 2 {
		t.Fatalf("asset version created events = %+v, want 2", taskEventRepo.events)
	}
	if countStep04TaskEvents(taskEventRepo.events, domain.TaskEventDesignSubmitted) != 0 {
		t.Fatalf("design submitted events = %+v, want 0 before resource-group submission", taskEventRepo.events)
	}
}

func TestTaskAssetCenterServiceCompletesLegacyRetouchSessionAfterPrematureCompleted(t *testing.T) {
	designerID := int64(702)
	now := time.Date(2026, 6, 9, 13, 33, 5, 0, time.UTC)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:         2061,
		TaskNo:     "T-2061",
		TaskType:   domain.TaskTypeRetouchTask,
		TaskStatus: domain.TaskStatusCompleted,
		DesignerID: &designerID,
		UpdatedAt:  now.Add(time.Minute),
	})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted

	assetType := domain.TaskAssetTypeDelivery
	precreatedAssetID, err := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2061,
		AssetNo:   "AST-PRECREATED",
		AssetType: assetType,
		CreatedBy: designerID,
	})
	if err != nil {
		t.Fatalf("create precreated legacy retouch root: %v", err)
	}
	uploadRequestRepo.requests["legacy-retouch-session"] = &domain.UploadRequest{
		RequestID:       "legacy-retouch-session",
		OwnerType:       domain.AssetOwnerTypeTask,
		OwnerID:         2061,
		TaskID:          2061,
		AssetID:         &precreatedAssetID,
		TaskAssetType:   &assetType,
		StorageAdapter:  domain.AssetStorageAdapterOSSUploadService,
		UploadMode:      domain.DesignAssetUploadModeMultipart,
		RefType:         domain.AssetStorageRefTypeTaskAssetObject,
		FileName:        "main-02.jpg",
		MimeType:        "image/jpeg",
		ExpectedSize:    uploadRequestInt64Ptr(2048),
		Status:          domain.UploadRequestStatusRequested,
		SessionStatus:   domain.DesignAssetSessionStatusCreated,
		RemoteUploadID:  "remote-legacy-retouch-session",
		StorageProvider: domain.DesignAssetStorageProviderOSS,
		CreatedBy:       designerID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	domain.HydrateUploadRequestDerived(uploadRequestRepo.requests["legacy-retouch-session"])

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)
	_, appErr := svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2061,
		SessionID:   "legacy-retouch-session",
		CompletedBy: designerID,
		FileHash:    "main-02-hash",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition || appErrorDenyCode(appErr) != "completed_resource_requires_reopen" {
		t.Fatalf("CompleteUploadSession(completed retouch) error = %+v, want reopen rejection", appErr)
	}
	if len(taskAssetRepo.assets) != 0 || len(taskEventRepo.events) != 0 {
		t.Fatalf("completed retouch mutation wrote assets/events: assets=%+v events=%+v", taskAssetRepo.assets, taskEventRepo.events)
	}
}

func TestTaskAssetCenterServiceDoesNotTreatPostCloseRetouchSessionAsLegacy(t *testing.T) {
	designerID := int64(703)
	completedAt := time.Date(2026, 6, 9, 13, 33, 5, 0, time.UTC)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID: 2062, TaskNo: "T-2062", TaskType: domain.TaskTypeRetouchTask, TaskStatus: domain.TaskStatusCompleted,
		DesignerID: &designerID, UpdatedAt: completedAt,
	})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetType := domain.TaskAssetTypeDelivery
	assetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{
		TaskID: 2062, AssetNo: "AST-POST-CLOSE", AssetType: assetType, CreatedBy: designerID,
	})
	uploadRequestRepo.requests["post-close-retouch-session"] = &domain.UploadRequest{
		RequestID: "post-close-retouch-session", OwnerType: domain.AssetOwnerTypeTask, OwnerID: 2062, TaskID: 2062,
		AssetID: &assetID, TaskAssetType: &assetType, Status: domain.UploadRequestStatusRequested,
		SessionStatus: domain.DesignAssetSessionStatusCreated, RemoteUploadID: "remote-post-close-retouch",
		CreatedBy: designerID, CreatedAt: completedAt.Add(time.Minute), UpdatedAt: completedAt.Add(time.Minute),
	}
	domain.HydrateUploadRequestDerived(uploadRequestRepo.requests["post-close-retouch-session"])
	svc := NewTaskAssetCenterService(
		taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient(),
	).(*taskAssetCenterService)
	ctx := domain.WithRequestActor(taskAssetMutationTestContext(), domain.RequestActor{
		ID: designerID, Roles: []domain.Role{domain.RoleDesigner}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	_, appErr := svc.CompleteUploadSession(ctx, CompleteTaskAssetUploadSessionParams{
		TaskID: 2062, SessionID: "post-close-retouch-session", CompletedBy: designerID,
	})
	if appErr == nil {
		t.Fatal("CompleteUploadSession(post-close retouch) error = nil")
	}
	details, _ := appErr.Details.(map[string]interface{})
	if got := details["deny_code"]; got != "completed_resource_requires_reopen" {
		t.Fatalf("deny_code = %+v, want completed_resource_requires_reopen", got)
	}
}

func TestTaskAssetCenterServiceCompletedReplacementRejectsCurrentChangedBeforeLockedWrite(t *testing.T) {
	const taskID = int64(2063)
	designerID := int64(704)
	taskRepo := newStep04TaskRepo(&domain.Task{ID: taskID, TaskStatus: domain.TaskStatusCompleted, DesignerID: &designerID})
	baseDesignRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetID, _ := baseDesignRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{TaskID: taskID, AssetNo: "AST-CAS", AssetType: domain.TaskAssetTypeDelivery, CreatedBy: designerID})
	versionNo := 1
	oldVersionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{
		TaskID: taskID, AssetID: &assetID, AssetType: domain.TaskAssetTypeDelivery, VersionNo: 1, AssetVersionNo: &versionNo,
		FileName: "old.psd", FlowReviewStatus: domain.TaskAssetFlowReviewStatusApproved,
	})
	_ = baseDesignRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, assetID, &oldVersionID)
	concurrentVersionID := int64(9999)
	designAssetRepo := &currentChangedOnLockDesignAssetRepo{step67DesignAssetRepo: baseDesignRepo, lockedCurrentVersionID: &concurrentVersionID}
	assetType := domain.TaskAssetTypeDelivery
	uploadRequestRepo.requests["replacement-race"] = &domain.UploadRequest{
		RequestID: "replacement-race", OwnerType: domain.AssetOwnerTypeTask, OwnerID: taskID, TaskID: taskID,
		AssetID: &assetID, TaskAssetType: &assetType, Status: domain.UploadRequestStatusRequested,
		SessionStatus: domain.DesignAssetSessionStatusCreated, RemoteUploadID: "remote-replacement-race",
		UploadMode: domain.DesignAssetUploadModeMultipart, StorageProvider: domain.DesignAssetStorageProviderOSS,
		ExpectedSize: uploadRequestInt64Ptr(1024), FileSize: uploadRequestInt64Ptr(1024),
	}
	domain.HydrateUploadRequestDerived(uploadRequestRepo.requests["replacement-race"])
	svc := NewTaskAssetCenterService(
		taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient(),
	).(*taskAssetCenterService)
	ctx := domain.WithRequestActor(taskAssetMutationTestContext(), domain.RequestActor{
		ID: designerID, Roles: []domain.Role{domain.RoleDesigner}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	_, appErr := svc.CompleteUploadSession(ctx, CompleteTaskAssetUploadSessionParams{TaskID: taskID, SessionID: "replacement-race", CompletedBy: designerID})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("CompleteUploadSession(current changed) error = %+v, want reopen rejection", appErr)
	}
	details, _ := appErr.Details.(map[string]interface{})
	if got := details["deny_code"]; got != "completed_resource_requires_reopen" {
		t.Fatalf("deny_code = %+v", got)
	}
	if got := len(taskAssetRepo.assets); got != 1 {
		t.Fatalf("task asset versions = %d, want original only", got)
	}
}

func TestTaskAssetCenterServiceCompletedReplacementRejectsTaskArchivedBeforeLockedWrite(t *testing.T) {
	const taskID = int64(2065)
	designerID := int64(706)
	taskRepo := newStep04TaskRepo(&domain.Task{ID: taskID, TaskStatus: domain.TaskStatusCompleted, DesignerID: &designerID})
	taskRepo.forUpdateTask = &domain.Task{ID: taskID, TaskStatus: domain.TaskStatusArchived, DesignerID: &designerID}
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{TaskID: taskID, AssetNo: "AST-ARCHIVE-RACE", AssetType: domain.TaskAssetTypeDelivery, CreatedBy: designerID})
	versionNo := 1
	versionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{
		TaskID: taskID, AssetID: &assetID, AssetType: domain.TaskAssetTypeDelivery, VersionNo: 1, AssetVersionNo: &versionNo,
		FileName: "current.psd", FlowReviewStatus: domain.TaskAssetFlowReviewStatusApproved,
	})
	_ = designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, assetID, &versionID)
	assetType := domain.TaskAssetTypeDelivery
	uploadRequestRepo.requests["replacement-archive-race"] = &domain.UploadRequest{
		RequestID: "replacement-archive-race", OwnerType: domain.AssetOwnerTypeTask, OwnerID: taskID, TaskID: taskID,
		AssetID: &assetID, TaskAssetType: &assetType, Status: domain.UploadRequestStatusRequested,
		SessionStatus: domain.DesignAssetSessionStatusCreated, RemoteUploadID: "remote-replacement-archive-race",
		UploadMode: domain.DesignAssetUploadModeMultipart, StorageProvider: domain.DesignAssetStorageProviderOSS,
		ExpectedSize: uploadRequestInt64Ptr(1024), FileSize: uploadRequestInt64Ptr(1024),
	}
	domain.HydrateUploadRequestDerived(uploadRequestRepo.requests["replacement-archive-race"])
	svc := NewTaskAssetCenterService(
		taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient(),
	).(*taskAssetCenterService)
	ctx := domain.WithRequestActor(taskAssetMutationTestContext(), domain.RequestActor{
		ID: designerID, Roles: []domain.Role{domain.RoleDesigner}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	_, appErr := svc.CompleteUploadSession(ctx, CompleteTaskAssetUploadSessionParams{TaskID: taskID, SessionID: "replacement-archive-race", CompletedBy: designerID})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("CompleteUploadSession(task archived) error = %+v, want reopen rejection", appErr)
	}
	if got := len(taskAssetRepo.assets); got != 1 {
		t.Fatalf("task asset versions = %d, want original only", got)
	}
}

func TestTaskAssetCenterServiceConcurrentCompleteObservesWinnerWithoutSecondVersion(t *testing.T) {
	const taskID = int64(2064)
	designerID := int64(705)
	taskRepo := newStep04TaskRepo(&domain.Task{ID: taskID, TaskStatus: domain.TaskStatusCompleted, DesignerID: &designerID})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	baseUploadRepo := newStep37UploadRequestRepo()
	assetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{TaskID: taskID, AssetNo: "AST-IDEMPOTENT-RACE", AssetType: domain.TaskAssetTypeDelivery, CreatedBy: designerID})
	versionNo := 1
	versionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{
		TaskID: taskID, AssetID: &assetID, AssetType: domain.TaskAssetTypeDelivery, VersionNo: 1, AssetVersionNo: &versionNo,
		FileName: "winner.psd", FlowReviewStatus: domain.TaskAssetFlowReviewStatusApproved,
	})
	_ = designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, assetID, &versionID)
	assetType := domain.TaskAssetTypeDelivery
	initial := &domain.UploadRequest{
		RequestID: "same-session-race", OwnerType: domain.AssetOwnerTypeTask, OwnerID: taskID, TaskID: taskID,
		AssetID: &assetID, TaskAssetType: &assetType, Status: domain.UploadRequestStatusRequested,
		SessionStatus: domain.DesignAssetSessionStatusCreated, RemoteUploadID: "remote-same-session-race",
		UploadMode: domain.DesignAssetUploadModeMultipart, StorageProvider: domain.DesignAssetStorageProviderOSS,
		ExpectedSize: uploadRequestInt64Ptr(1024), FileSize: uploadRequestInt64Ptr(1024),
	}
	bound := *initial
	bound.Status = domain.UploadRequestStatusBound
	bound.SessionStatus = domain.DesignAssetSessionStatusCompleted
	bound.BoundAssetID = &versionID
	baseUploadRepo.requests[initial.RequestID] = initial
	domain.HydrateUploadRequestDerived(initial)
	domain.HydrateUploadRequestDerived(&bound)
	uploadRequestRepo := &completedOnLockUploadRequestRepo{step37UploadRequestRepo: baseUploadRepo, lockedRequest: &bound}
	taskEventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssetCenterService(
		taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, newStep37AssetStorageRefRepo(), taskEventRepo, step04TxRunner{}, newStubUploadServiceClient(),
	).(*taskAssetCenterService)
	ctx := domain.WithRequestActor(taskAssetMutationTestContext(), domain.RequestActor{
		ID: designerID, Roles: []domain.Role{domain.RoleDesigner}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	_, appErr := svc.CompleteUploadSession(ctx, CompleteTaskAssetUploadSessionParams{TaskID: taskID, SessionID: initial.RequestID, CompletedBy: designerID})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("CompleteUploadSession(concurrent winner) error = %+v, want reopen rejection", appErr)
	}
	if got := len(taskAssetRepo.assets); got != 1 {
		t.Fatalf("task asset versions = %d, want no second version", got)
	}
	if len(taskEventRepo.events) != 0 {
		t.Fatalf("events = %+v, want no duplicate completion event", taskEventRepo.events)
	}
}

func TestTaskAssetCenterServiceConcurrentCompletePreventsCancelOverwrite(t *testing.T) {
	const taskID = int64(2065)
	designerID := int64(706)
	taskRepo := newStep04TaskRepo(&domain.Task{ID: taskID, TaskStatus: domain.TaskStatusCompleted, DesignerID: &designerID})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	baseUploadRepo := newStep37UploadRequestRepo()
	assetID, _ := designAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.DesignAsset{TaskID: taskID, AssetNo: "AST-CANCEL-RACE", AssetType: domain.TaskAssetTypeDelivery, CreatedBy: designerID})
	versionNo := 1
	versionID, _ := taskAssetRepo.Create(taskAssetMutationTestContext(), step04Tx{}, &domain.TaskAsset{TaskID: taskID, AssetID: &assetID, AssetType: domain.TaskAssetTypeDelivery, VersionNo: 1, AssetVersionNo: &versionNo, FlowReviewStatus: domain.TaskAssetFlowReviewStatusApproved})
	_ = designAssetRepo.UpdateCurrentVersionID(taskAssetMutationTestContext(), step04Tx{}, assetID, &versionID)
	assetType := domain.TaskAssetTypeDelivery
	initial := &domain.UploadRequest{RequestID: "cancel-race", OwnerType: domain.AssetOwnerTypeTask, OwnerID: taskID, TaskID: taskID, AssetID: &assetID, TaskAssetType: &assetType, Status: domain.UploadRequestStatusRequested, SessionStatus: domain.DesignAssetSessionStatusCreated, RemoteUploadID: "remote-cancel-race"}
	bound := *initial
	bound.Status = domain.UploadRequestStatusBound
	bound.SessionStatus = domain.DesignAssetSessionStatusCompleted
	bound.BoundAssetID = &versionID
	baseUploadRepo.requests[initial.RequestID] = initial
	domain.HydrateUploadRequestDerived(initial)
	domain.HydrateUploadRequestDerived(&bound)
	uploadRequestRepo := &completedOnLockUploadRequestRepo{step37UploadRequestRepo: baseUploadRepo, lockedRequest: &bound}
	taskEventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, newStep37AssetStorageRefRepo(), taskEventRepo, step04TxRunner{}, newStubUploadServiceClient()).(*taskAssetCenterService)
	ctx := domain.WithRequestActor(taskAssetMutationTestContext(), domain.RequestActor{ID: designerID, Roles: []domain.Role{domain.RoleDesigner}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced})
	_, appErr := svc.CancelUploadSession(ctx, CancelTaskAssetUploadSessionParams{TaskID: taskID, SessionID: initial.RequestID, CancelledBy: designerID})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("CancelUploadSession(concurrent complete) error = %+v, want reopen rejection", appErr)
	}
	if initial.Status != domain.UploadRequestStatusRequested || initial.SessionStatus != domain.DesignAssetSessionStatusCreated {
		t.Fatalf("stale request was overwritten: %+v", initial)
	}
	if len(taskEventRepo.events) != 0 {
		t.Fatalf("events = %+v, want no cancellation event", taskEventRepo.events)
	}
}

func TestTaskAssetCenterServiceCompleteUploadSessionMapsTaskAssetVersionDuplicateToConflict(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2054, TaskStatus: domain.TaskStatusPendingAudit})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := &duplicateVersionTaskAssetRepo{
		step04TaskAssetRepo: newStep04TaskAssetRepo(),
		failForRequestID:    "dup-session",
	}
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)

	createResult, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
		TaskID:       2054,
		CreatedBy:    654,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "dup.jpg",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "image/jpeg",
		FileHash:     "dup-hash",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}
	taskAssetRepo.failForRequestID = createResult.Session.ID

	_, appErr = svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2054,
		SessionID:   createResult.Session.ID,
		CompletedBy: 654,
		FileHash:    "dup-hash",
	})
	if appErr == nil {
		t.Fatal("CompleteUploadSession() appErr = nil, want conflict")
	}
	if appErr.Code != domain.ErrCodeConflict {
		t.Fatalf("CompleteUploadSession() code = %s, want %s", appErr.Code, domain.ErrCodeConflict)
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("CompleteUploadSession() details type = %T", appErr.Details)
	}
	if got := details["deny_code"]; got != assetVersionRaceRetryDenyCode {
		t.Fatalf("CompleteUploadSession() deny_code = %+v, want %q", got, assetVersionRaceRetryDenyCode)
	}
}

func countStep04TaskEvents(events []*domain.TaskEvent, eventType string) int {
	total := 0
	for _, event := range events {
		if event != nil && event.EventType == eventType {
			total++
		}
	}
	return total
}

func timeValuePtr(v time.Time) *time.Time {
	return &v
}

type currentChangedOnLockDesignAssetRepo struct {
	*step67DesignAssetRepo
	lockedCurrentVersionID *int64
}

func (r *currentChangedOnLockDesignAssetRepo) GetByIDForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.DesignAsset, error) {
	asset, err := r.step67DesignAssetRepo.GetByID(ctx, id)
	if err != nil || asset == nil {
		return asset, err
	}
	locked := *asset
	locked.CurrentVersionID = domain.CloneInt64Ptr(r.lockedCurrentVersionID)
	return &locked, nil
}

type completedOnLockUploadRequestRepo struct {
	*step37UploadRequestRepo
	lockedRequest *domain.UploadRequest
}

func (r *completedOnLockUploadRequestRepo) GetByRequestIDForUpdate(_ context.Context, _ repo.Tx, requestID string) (*domain.UploadRequest, error) {
	if r.lockedRequest == nil || r.lockedRequest.RequestID != requestID {
		return nil, nil
	}
	copyRequest := *r.lockedRequest
	return &copyRequest, nil
}

type stubUploadServiceClient struct {
	mu                    sync.Mutex
	nextUploadIndex       int
	uploadAssetNoByID     map[string]string
	createRequests        []RemoteCreateUploadSessionRequest
	lastCompletedSize     int64
	lastCompletedHash     string
	probeBytesOverride    *int64
	probeHashOverride     string
	probeErr              error
	probeErrSequence      []error
	probeResponseSequence []*RemoteStoredFileProbe
	failComplete          bool
	remoteSessionStatus   domain.DesignAssetSessionStatus
	remoteSessionFileID   string
	getUploadSessionErr   error
	completeCalls         int
	getUploadSessionCalls int
	getFileMetaCalls      int
}

type duplicateVersionTaskAssetRepo struct {
	*step04TaskAssetRepo
	failForRequestID string
}

func (r *duplicateVersionTaskAssetRepo) Create(ctx context.Context, tx repo.Tx, asset *domain.TaskAsset) (int64, error) {
	if asset != nil && asset.UploadRequestID != nil && *asset.UploadRequestID == r.failForRequestID {
		return 0, fmt.Errorf("insert task_asset: %w", &mysql.MySQLError{
			Number:  1062,
			Message: "Duplicate entry '2054-1' for key '" + taskAssetVersionUniqueKey + "'",
		})
	}
	return r.step04TaskAssetRepo.Create(ctx, tx, asset)
}

func newStubUploadServiceClient() UploadServiceClient {
	return &stubUploadServiceClient{
		uploadAssetNoByID: map[string]string{},
	}
}

func (c *stubUploadServiceClient) UploadServiceBaseURL() string {
	return "http://upload-service.test"
}

func (c *stubUploadServiceClient) CreateUploadSession(_ context.Context, req RemoteCreateUploadSessionRequest) (*RemoteUploadSessionPlan, error) {
	c.mu.Lock()
	c.nextUploadIndex++
	uploadID := fmt.Sprintf("remote-upload-%d", c.nextUploadIndex)
	c.uploadAssetNoByID[uploadID] = strings.TrimSpace(req.AssetNo)
	c.createRequests = append(c.createRequests, req)
	c.mu.Unlock()
	expiresAt := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	lastSyncedAt := time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)
	partSizeHint := int64(0)
	if req.UploadMode == domain.DesignAssetUploadModeMultipart {
		partSizeHint = 8 * 1024 * 1024
	}
	return &RemoteUploadSessionPlan{
		UploadID:      uploadID,
		BaseURL:       "http://upload-service.test",
		UploadURL:     "http://upload-service.test/v1/upload-sessions/" + uploadID,
		CompleteURL:   "http://upload-service.test/v1/upload-sessions/" + uploadID + "/complete",
		AbortURL:      "http://upload-service.test/v1/upload-sessions/" + uploadID + "/abort",
		Method:        "PUT",
		PartSizeHint:  partSizeHint,
		UploadMode:    req.UploadMode,
		SessionStatus: domain.DesignAssetSessionStatusCreated,
		ExpiresAt:     &expiresAt,
		LastSyncedAt:  &lastSyncedAt,
	}, nil
}

func (c *stubUploadServiceClient) GetUploadSession(_ context.Context, req RemoteGetUploadSessionRequest) (*RemoteUploadSessionPlan, error) {
	c.getUploadSessionCalls++
	if c.getUploadSessionErr != nil {
		return nil, c.getUploadSessionErr
	}
	lastSyncedAt := time.Date(2026, 3, 14, 9, 5, 0, 0, time.UTC)
	status := c.remoteSessionStatus
	if !status.Valid() {
		status = domain.DesignAssetSessionStatusCreated
	}
	plan := &RemoteUploadSessionPlan{
		UploadID:      req.RemoteUploadID,
		BaseURL:       "http://upload-service.test",
		SessionStatus: status,
		LastSyncedAt:  &lastSyncedAt,
	}
	if strings.TrimSpace(c.remoteSessionFileID) != "" {
		plan.FileID = optionalStringPtr(c.remoteSessionFileID)
	}
	return plan, nil
}

func (c *stubUploadServiceClient) UploadFileToSession(_ context.Context, req RemoteSessionFileUploadRequest) (*RemoteFileMeta, error) {
	if req.File == nil {
		return nil, fmt.Errorf("missing file")
	}
	body, err := io.ReadAll(req.File)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])
	size := int64(len(body))
	c.lastCompletedSize = size
	c.lastCompletedHash = sha
	fileID := "uploaded-file-123"
	return &RemoteFileMeta{
		FileID:     &fileID,
		StorageKey: "objects/design-assets/uploaded-file-123",
		FileSize:   &size,
		FileHash:   &sha,
		MimeType:   req.MimeType,
		UploadedAt: time.Date(2026, 3, 14, 9, 25, 0, 0, time.UTC),
	}, nil
}

func (c *stubUploadServiceClient) UploadSmallFile(_ context.Context, _ RemoteSmallFileUploadRequest) (*RemoteFileMeta, error) {
	return &RemoteFileMeta{}, nil
}

func (c *stubUploadServiceClient) PreparePartUpload(_ context.Context, req RemotePreparePartUploadRequest) (*RemotePartUploadPlan, error) {
	expiresAt := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	return &RemotePartUploadPlan{
		UploadID:     req.RemoteUploadID,
		PartNumber:   req.PartNumber,
		Method:       "PUT",
		UploadURL:    "http://upload-service.test/v1/upload-sessions/remote-upload-1/parts/1",
		ExpiresAt:    &expiresAt,
		PartSizeHint: 8 * 1024 * 1024,
	}, nil
}

func (c *stubUploadServiceClient) CompleteUploadSession(_ context.Context, req RemoteCompleteUploadRequest) (*RemoteFileMeta, error) {
	c.completeCalls++
	if c.failComplete {
		return nil, fmt.Errorf("complete should not be called")
	}
	fileID := "file-123"
	c.mu.Lock()
	if assetNo := strings.TrimSpace(c.uploadAssetNoByID[strings.TrimSpace(req.RemoteUploadID)]); assetNo != "" {
		fileID = "file-" + strings.ToLower(strings.ReplaceAll(assetNo, " ", "-"))
	}
	c.mu.Unlock()
	c.lastCompletedSize = int64ValueFromPtr(req.ExpectedSize)
	c.lastCompletedHash = strings.TrimSpace(req.ChecksumHint)
	return &RemoteFileMeta{
		FileID:     &fileID,
		StorageKey: "objects/design-assets/" + fileID,
		FileSize:   req.ExpectedSize,
		FileHash:   optionalStringPtr(req.ChecksumHint),
		MimeType:   req.MimeType,
		UploadedAt: time.Date(2026, 3, 14, 9, 30, 0, 0, time.UTC),
	}, nil
}

func (c *stubUploadServiceClient) AbortUploadSession(_ context.Context, _ RemoteAbortUploadRequest) error {
	return nil
}

func (c *stubUploadServiceClient) GetFileMeta(_ context.Context, req RemoteGetFileMetaRequest) (*RemoteFileMeta, error) {
	c.getFileMetaCalls++
	fileID := "file-123"
	if strings.TrimSpace(c.remoteSessionFileID) != "" {
		fileID = strings.TrimSpace(c.remoteSessionFileID)
	} else {
		c.mu.Lock()
		if assetNo := strings.TrimSpace(c.uploadAssetNoByID[strings.TrimSpace(req.RemoteUploadID)]); assetNo != "" {
			fileID = "file-" + strings.ToLower(strings.ReplaceAll(assetNo, " ", "-"))
		}
		c.mu.Unlock()
	}
	return &RemoteFileMeta{
		FileID:     &fileID,
		StorageKey: "objects/design-assets/" + fileID,
		FileSize:   req.ExpectedSize,
		FileHash:   optionalStringPtr(req.ChecksumHint),
		MimeType:   req.MimeType,
		UploadedAt: time.Date(2026, 3, 14, 9, 30, 0, 0, time.UTC),
	}, nil
}

func (c *stubUploadServiceClient) ProbeStoredFile(_ context.Context, _ RemoteProbeStoredFileRequest) (*RemoteStoredFileProbe, error) {
	if len(c.probeErrSequence) > 0 {
		err := c.probeErrSequence[0]
		c.probeErrSequence = c.probeErrSequence[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(c.probeResponseSequence) > 0 {
		probe := c.probeResponseSequence[0]
		c.probeResponseSequence = c.probeResponseSequence[1:]
		return probe, c.probeErr
	}
	if c.probeErr != nil {
		return nil, c.probeErr
	}
	bytesRead := c.lastCompletedSize
	if c.probeBytesOverride != nil {
		bytesRead = *c.probeBytesOverride
	}
	sha := c.lastCompletedHash
	if strings.TrimSpace(c.probeHashOverride) != "" {
		sha = strings.TrimSpace(c.probeHashOverride)
	}
	return &RemoteStoredFileProbe{
		StatusCode:          200,
		ContentType:         "application/octet-stream",
		ContentLengthHeader: bytesRead,
		BytesRead:           bytesRead,
		SHA256:              sha,
	}, nil
}

func (c *stubUploadServiceClient) OpenStoredFile(_ context.Context, req RemoteProbeStoredFileRequest) (io.ReadCloser, error) {
	storageKey := strings.TrimSpace(req.StorageKey)
	if storageKey == "" {
		return nil, fmt.Errorf("missing storage key")
	}
	return io.NopCloser(strings.NewReader("stub-stream:" + storageKey)), nil
}

func (c *stubUploadServiceClient) BuildBrowserFileURL(storageKey string) *string {
	storageKey = strings.TrimSpace(storageKey)
	if storageKey == "" {
		return nil
	}
	url := "https://oss-browser.test/files/" + storageKey
	return &url
}

type fakeOSSDirectServer struct {
	t             *testing.T
	server        *httptest.Server
	client        *http.Client
	mu            sync.Mutex
	nextUploadID  int
	initiateCalls int
	copyCalls     int
	lastCopyDate  string
	lastCopyAuth  string
	lastCopySrc   string
	completeCalls int
	uploadKeys    map[string]string
	parts         map[string]map[string][]byte
	objects       map[string][]byte
}

func newFakeOSSDirectServer(t *testing.T) *fakeOSSDirectServer {
	t.Helper()
	fake := &fakeOSSDirectServer{
		t:          t,
		uploadKeys: map[string]string{},
		parts:      map[string]map[string][]byte{},
		objects:    map[string][]byte{},
	}
	fake.server = httptest.NewTLSServer(http.HandlerFunc(fake.handle))
	baseURL, err := url.Parse(fake.server.URL)
	if err != nil {
		t.Fatalf("url.Parse(server.URL) error = %v", err)
	}
	rewriteClient := fake.server.Client()
	rewriteClient.Transport = &rewriteHostTransport{
		base:  baseURL,
		inner: rewriteClient.Transport,
	}
	fake.client = rewriteClient
	return fake
}

func (f *fakeOSSDirectServer) Close() {
	if f.server != nil {
		f.server.Close()
	}
}

func (f *fakeOSSDirectServer) Client() *http.Client {
	return f.client
}

func (f *fakeOSSDirectServer) EndpointHost() string {
	return strings.TrimPrefix(f.server.URL, "https://")
}

func (f *fakeOSSDirectServer) handle(w http.ResponseWriter, r *http.Request) {
	objectKey := strings.TrimPrefix(r.URL.Path, "/")
	objectKey = strings.TrimPrefix(objectKey, "test-bucket/")
	if strings.TrimSpace(objectKey) == "" {
		http.NotFound(w, r)
		return
	}
	query := r.URL.Query()
	switch {
	case r.Method == http.MethodPost && rawQueryHasFlag(r.URL.RawQuery, "uploads"):
		f.mu.Lock()
		f.nextUploadID++
		f.initiateCalls++
		uploadID := fmt.Sprintf("oss-upload-%d", f.nextUploadID)
		f.uploadKeys[uploadID] = objectKey
		f.parts[uploadID] = map[string][]byte{}
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, "<InitiateMultipartUploadResult><Bucket>test-bucket</Bucket><Key>%s</Key><UploadId>%s</UploadId></InitiateMultipartUploadResult>", objectKey, uploadID)
	case r.Method == http.MethodPut && strings.TrimSpace(r.Header.Get("x-oss-copy-source")) != "":
		f.mu.Lock()
		f.copyCalls++
		f.lastCopyDate = r.Header.Get("Date")
		f.lastCopyAuth = r.Header.Get("Authorization")
		f.lastCopySrc = r.Header.Get("x-oss-copy-source")
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, "<CopyObjectResult></CopyObjectResult>")
	case r.Method == http.MethodPut && query.Get("uploadId") != "" && query.Get("partNumber") != "":
		partNumber := strings.TrimSpace(query.Get("partNumber"))
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		if f.parts[query.Get("uploadId")] == nil {
			f.parts[query.Get("uploadId")] = map[string][]byte{}
		}
		f.parts[query.Get("uploadId")][partNumber] = append([]byte(nil), body...)
		f.mu.Unlock()
		w.Header().Set("ETag", fmt.Sprintf("\"etag-%s-%s\"", query.Get("uploadId"), partNumber))
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPut && query.Get("uploadId") == "" && query.Get("partNumber") == "":
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.objects[objectKey] = append([]byte(nil), body...)
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPost && query.Get("uploadId") != "":
		f.mu.Lock()
		f.completeCalls++
		uploadID := query.Get("uploadId")
		if key := f.uploadKeys[uploadID]; key != "" {
			var assembled []byte
			for i := 1; ; i++ {
				part := f.parts[uploadID][fmt.Sprintf("%d", i)]
				if len(part) == 0 {
					break
				}
				assembled = append(assembled, part...)
			}
			f.objects[key] = assembled
		}
		f.mu.Unlock()
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, "<CompleteMultipartUploadResult></CompleteMultipartUploadResult>")
	case r.Method == http.MethodGet:
		f.mu.Lock()
		body, ok := f.objects[objectKey]
		f.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	case r.Method == http.MethodHead:
		f.mu.Lock()
		body, ok := f.objects[objectKey]
		f.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

type rewriteHostTransport struct {
	base  *url.URL
	inner http.RoundTripper
}

func (t *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL = cloneURL(req.URL)
	cloned.URL.Scheme = t.base.Scheme
	cloned.URL.Host = t.base.Host
	return t.inner.RoundTrip(cloned)
}

func cloneURL(src *url.URL) *url.URL {
	if src == nil {
		return &url.URL{}
	}
	cloned := *src
	return &cloned
}

func rawQueryHasFlag(rawQuery, key string) bool {
	if rawQuery == key {
		return true
	}
	return strings.HasPrefix(rawQuery, key+"&") || strings.Contains(rawQuery, "&"+key+"&") || strings.HasSuffix(rawQuery, "&"+key) || strings.Contains(rawQuery, "&"+key+"=")
}

func TestValidateTaskStageUploadAssetTypeNormalAuditStillAllowsDeliveryAndBasicInfoReference(t *testing.T) {
	task := &domain.Task{TaskStatus: domain.TaskStatusPendingAudit}
	if appErr := validateTaskStageUploadAssetType(task, domain.TaskAssetTypeDelivery, string(domain.ModuleKeyAudit), nil); appErr != nil {
		t.Fatalf("validateTaskStageUploadAssetType(delivery) appErr = %+v", appErr)
	}
	if appErr := validateTaskStageUploadAssetType(task, domain.TaskAssetTypeReference, string(domain.ModuleKeyBasicInfo), nil); appErr != nil {
		t.Fatalf("validateTaskStageUploadAssetType(basic_info reference) appErr = %+v", appErr)
	}
	frozenAssetID := int64(91)
	if appErr := validateTaskStageUploadAssetType(task, domain.TaskAssetTypeReference, "", &frozenAssetID); appErr != nil {
		t.Fatalf("validateTaskStageUploadAssetType(frozen reference completion) appErr = %+v", appErr)
	}
	if appErr := validateTaskStageUploadAssetType(task, domain.TaskAssetTypeReference, "", nil); appErr == nil {
		t.Fatal("validateTaskStageUploadAssetType(unscoped new reference) appErr = nil, want rejection")
	}
}

func TestTaskStageUploadPolicySeparatesDesignSourceFromAuditFinals(t *testing.T) {
	designTask := &domain.Task{TaskType: domain.TaskTypeNewProductDevelopment, TaskStatus: domain.TaskStatusInProgress}
	if appErr := validateTaskStageUploadAssetType(designTask, domain.TaskAssetTypeSource, "design", nil); appErr != nil {
		t.Fatalf("design source rejected: %+v", appErr)
	}
	appErr := validateTaskStageUploadAssetType(designTask, domain.TaskAssetTypeDelivery, "design", nil)
	if appErr == nil {
		t.Fatal("design final output was accepted")
	}
	details, _ := appErr.Details.(map[string]interface{})
	if details["deny_code"] != "design_stage_final_output_not_allowed" {
		t.Fatalf("design final output error = %+v", appErr)
	}

	auditTask := &domain.Task{TaskType: domain.TaskTypeNewProductDevelopment, TaskStatus: domain.TaskStatusPendingAudit}
	if appErr := validateTaskStageUploadAssetType(auditTask, domain.TaskAssetTypeDelivery, "audit", nil); appErr != nil {
		t.Fatalf("audit final output rejected: %+v", appErr)
	}

	retouchTask := &domain.Task{TaskType: domain.TaskTypeRetouchTask, TaskStatus: domain.TaskStatusInProgress}
	if appErr := validateTaskStageUploadAssetType(retouchTask, domain.TaskAssetTypeDelivery, "retouch", nil); appErr != nil {
		t.Fatalf("retouch final output rejected: %+v", appErr)
	}
}

type testPreviewRenderer struct{}

func (testPreviewRenderer) Render(ctx context.Context, sourcePath string, source AssetPreviewSourceMeta, spec AssetPreviewRenderSpec) ([]byte, error) {
	return []byte(fmt.Sprintf("webp-preview:%s:%dx%d", source.Filename, spec.MaxWidth, spec.MaxHeight)), nil
}
