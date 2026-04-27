package service

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestAssetUploadServiceCreateAndAdvanceLifecycle(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 1001})
	uploadRequestRepo := newStep37UploadRequestRepo()
	svc := NewAssetUploadService(taskRepo, uploadRequestRepo, noopTxRunner{}).(*assetUploadService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 11, 1, 0, 0, 0, time.UTC)
	}

	request, appErr := svc.CreateUploadRequest(context.Background(), CreateUploadRequestParams{
		OwnerType: domain.AssetOwnerTypeTask,
		OwnerID:   1001,
		FileName:  "draft.psd",
		MimeType:  "image/vnd.adobe.photoshop",
		Remark:    "prepared for upload",
	})
	if appErr != nil {
		t.Fatalf("CreateUploadRequest() unexpected error: %+v", appErr)
	}
	if request.Status != domain.UploadRequestStatusRequested || !request.CanBind || !request.CanCancel || !request.CanExpire {
		t.Fatalf("created request = %+v", request)
	}
	if request.PolicyMode != domain.PolicyModeRouteRoleVisibilityScaffolding || request.PolicyScopeSummary == nil {
		t.Fatalf("created request policy scaffolding = mode:%s summary:%+v", request.PolicyMode, request.PolicyScopeSummary)
	}
	if len(request.VisibleToRoles) == 0 || len(request.ActionRoles) == 0 {
		t.Fatalf("created request policy roles/actions = visible:%+v actions:%+v", request.VisibleToRoles, request.ActionRoles)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 11, 1, 5, 0, 0, time.UTC)
	}
	request, appErr = svc.AdvanceUploadRequest(context.Background(), request.RequestID, AdvanceUploadRequestParams{
		Action: domain.UploadRequestAdvanceActionCancel,
		Remark: "designer cancelled placeholder request",
	})
	if appErr != nil {
		t.Fatalf("AdvanceUploadRequest(cancel) unexpected error: %+v", appErr)
	}
	if request.Status != domain.UploadRequestStatusCancelled {
		t.Fatalf("cancelled request status = %s, want cancelled", request.Status)
	}
	if request.CanBind || request.CanCancel || request.CanExpire {
		t.Fatalf("cancelled request flags = %+v", request)
	}
	if request.HandoffRefSummary == nil || request.HandoffRefSummary.Status != string(domain.UploadRequestStatusCancelled) {
		t.Fatalf("cancelled handoff summary = %+v", request.HandoffRefSummary)
	}
}

func TestAssetUploadServiceAdvanceRejectsTerminalRequest(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 1002})
	uploadRequestRepo := newStep37UploadRequestRepo()
	uploadRequestRepo.requests["upload-bound"] = &domain.UploadRequest{
		RequestID:      "upload-bound",
		OwnerType:      domain.AssetOwnerTypeTask,
		OwnerID:        1002,
		StorageAdapter: domain.AssetStorageAdapterPlaceholderStorage,
		RefType:        domain.AssetStorageRefTypeTaskAssetObject,
		FileName:       "final.ai",
		Status:         domain.UploadRequestStatusBound,
		IsPlaceholder:  true,
		BoundAssetID:   uploadRequestInt64Ptr(77),
		BoundRefID:     "ref-77",
		CreatedAt:      time.Date(2026, 3, 11, 2, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 3, 11, 2, 10, 0, 0, time.UTC),
	}
	domain.HydrateUploadRequestDerived(uploadRequestRepo.requests["upload-bound"])

	svc := NewAssetUploadService(taskRepo, uploadRequestRepo, noopTxRunner{}).(*assetUploadService)
	_, appErr := svc.AdvanceUploadRequest(context.Background(), "upload-bound", AdvanceUploadRequestParams{
		Action: domain.UploadRequestAdvanceActionExpire,
	})
	if appErr == nil {
		t.Fatal("AdvanceUploadRequest(bound) expected error")
	}
	if appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("AdvanceUploadRequest(bound) error code = %s", appErr.Code)
	}
}

func TestAssetUploadServiceAdvanceExpireMarksPlaceholderExpired(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 1003})
	uploadRequestRepo := newStep37UploadRequestRepo()
	uploadRequestRepo.requests["upload-requested"] = &domain.UploadRequest{
		RequestID:      "upload-requested",
		OwnerType:      domain.AssetOwnerTypeTask,
		OwnerID:        1003,
		StorageAdapter: domain.AssetStorageAdapterPlaceholderStorage,
		RefType:        domain.AssetStorageRefTypeGenericObject,
		FileName:       "reference.pdf",
		Status:         domain.UploadRequestStatusRequested,
		IsPlaceholder:  true,
		Remark:         "waiting for placeholder bind",
		CreatedAt:      time.Date(2026, 3, 11, 3, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 3, 11, 3, 0, 0, 0, time.UTC),
	}
	domain.HydrateUploadRequestDerived(uploadRequestRepo.requests["upload-requested"])

	svc := NewAssetUploadService(taskRepo, uploadRequestRepo, noopTxRunner{}).(*assetUploadService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 11, 3, 30, 0, 0, time.UTC)
	}
	request, appErr := svc.AdvanceUploadRequest(context.Background(), "upload-requested", AdvanceUploadRequestParams{
		Action: domain.UploadRequestAdvanceActionExpire,
	})
	if appErr != nil {
		t.Fatalf("AdvanceUploadRequest(expire) unexpected error: %+v", appErr)
	}
	if request.Status != domain.UploadRequestStatusExpired {
		t.Fatalf("expired request status = %s, want expired", request.Status)
	}
	if request.CanBind || request.CanCancel || request.CanExpire {
		t.Fatalf("expired request flags = %+v", request)
	}
}

func TestAssetUploadServiceListUploadRequestsFiltersAndPagination(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 1004})
	uploadRequestRepo := newStep37UploadRequestRepo()
	designAssetType := domain.TaskAssetTypeFinal
	previewAssetType := domain.TaskAssetTypeReference
	uploadRequestRepo.requests["upload-queued"] = &domain.UploadRequest{
		RequestID:      "upload-queued",
		OwnerType:      domain.AssetOwnerTypeTask,
		OwnerID:        1004,
		TaskAssetType:  &designAssetType,
		StorageAdapter: domain.AssetStorageAdapterPlaceholderStorage,
		RefType:        domain.AssetStorageRefTypeTaskAssetObject,
		FileName:       "design.ai",
		Status:         domain.UploadRequestStatusRequested,
		IsPlaceholder:  true,
		CreatedAt:      time.Date(2026, 3, 11, 4, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 3, 11, 4, 0, 0, 0, time.UTC),
	}
	uploadRequestRepo.requests["upload-expired"] = &domain.UploadRequest{
		RequestID:      "upload-expired",
		OwnerType:      domain.AssetOwnerTypeTask,
		OwnerID:        1004,
		TaskAssetType:  &previewAssetType,
		StorageAdapter: domain.AssetStorageAdapterPlaceholderStorage,
		RefType:        domain.AssetStorageRefTypeGenericObject,
		FileName:       "preview.png",
		Status:         domain.UploadRequestStatusExpired,
		IsPlaceholder:  true,
		CreatedAt:      time.Date(2026, 3, 11, 4, 5, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 3, 11, 4, 10, 0, 0, time.UTC),
	}
	domain.HydrateUploadRequestDerived(uploadRequestRepo.requests["upload-queued"])
	domain.HydrateUploadRequestDerived(uploadRequestRepo.requests["upload-expired"])

	svc := NewAssetUploadService(taskRepo, uploadRequestRepo, noopTxRunner{}).(*assetUploadService)
	ownerID := int64(1004)
	status := domain.UploadRequestStatusRequested
	requests, pagination, appErr := svc.ListUploadRequests(context.Background(), UploadRequestFilter{
		OwnerID:  &ownerID,
		Status:   &status,
		Page:     1,
		PageSize: 1,
	})
	if appErr != nil {
		t.Fatalf("ListUploadRequests() unexpected error: %+v", appErr)
	}
	if len(requests) != 1 || requests[0].RequestID != "upload-queued" {
		t.Fatalf("filtered requests = %+v", requests)
	}
	if pagination.Total != 1 || pagination.Page != 1 || pagination.PageSize != 1 {
		t.Fatalf("pagination = %+v", pagination)
	}
	if requests[0].CanBind != true || requests[0].CanCancel != true || requests[0].CanExpire != true {
		t.Fatalf("requested lifecycle flags = %+v", requests[0])
	}
	if requests[0].PolicyMode != domain.PolicyModeRouteRoleVisibilityScaffolding || requests[0].PolicyScopeSummary == nil {
		t.Fatalf("policy scaffolding = %+v", requests[0])
	}
}

func uploadRequestInt64Ptr(v int64) *int64 {
	return &v
}

var _ repo.UploadRequestRepo = (*step37UploadRequestRepo)(nil)
