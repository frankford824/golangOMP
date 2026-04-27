package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestTaskServiceCreateAcceptsValidatedReferenceFileRefsFromCompletedUpload(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	referenceUploadSvc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, newStubUploadServiceClient()).(*taskCreateReferenceUploadService)
	referenceUploadSvc.nowFn = func() time.Time {
		return time.Date(2026, 3, 19, 1, 0, 0, 0, time.UTC)
	}

	createResult, appErr := referenceUploadSvc.CreateUploadSession(context.Background(), CreateTaskReferenceUploadSessionParams{
		CreatedBy:    9,
		Filename:     "reference-hero.png",
		ExpectedSize: uploadRequestInt64Ptr(2048),
		MimeType:     "image/png",
		FileHash:     "hash-reference-hero",
	})
	if appErr != nil {
		t.Fatalf("CreateUploadSession() unexpected error: %+v", appErr)
	}

	referenceUploadSvc.nowFn = func() time.Time {
		return time.Date(2026, 3, 19, 1, 5, 0, 0, time.UTC)
	}
	completeResult, appErr := referenceUploadSvc.CompleteUploadSession(context.Background(), CompleteTaskReferenceUploadSessionParams{
		SessionID:   createResult.Session.ID,
		CompletedBy: 9,
		FileHash:    "hash-reference-hero",
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession() unexpected error: %+v", appErr)
	}
	if strings.TrimSpace(completeResult.ReferenceFileRef) == "" {
		t.Fatalf("CompleteUploadSession() reference_file_ref = %q", completeResult.ReferenceFileRef)
	}

	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskReferenceFileRefValidation(uploadRequestRepo, assetStorageRefRepo),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:          domain.TaskTypeOriginalProductDevelopment,
		SourceMode:        domain.TaskSourceModeExistingProduct,
		CreatorID:         9,
		OwnerTeam:         domain.AllValidTeams()[0],
		DeadlineAt:        referenceImageTestTimePtr(),
		ChangeRequest:     "update design",
		ProductID:         int64Ptr(88),
		SKUCode:           "SKU-088",
		ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: completeResult.ReferenceFileRef}},
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task == nil {
		t.Fatal("Create() task = nil")
	}

	detail := taskRepo.details[task.ID]
	if detail == nil {
		t.Fatal("Create() detail = nil")
	}
	if detail.ReferenceImagesJSON != "[]" {
		t.Fatalf("reference_images_json = %q, want []", detail.ReferenceImagesJSON)
	}
	var refs []domain.ReferenceFileRef
	if err := json.Unmarshal([]byte(detail.ReferenceFileRefsJSON), &refs); err != nil {
		t.Fatalf("json.Unmarshal(reference_file_refs_json) error = %v", err)
	}
	if len(refs) != 1 || refs[0].AssetID != completeResult.ReferenceFileRef {
		t.Fatalf("reference_file_refs_json = %+v", refs)
	}

	readModel, appErr := svc.GetByID(context.Background(), task.ID)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if len(readModel.ReferenceFileRefs) != 1 {
		t.Fatalf("GetByID().reference_file_refs len = %d, want 1", len(readModel.ReferenceFileRefs))
	}
	if readModel.ReferenceFileRefs[0].AssetID != completeResult.ReferenceFileRef {
		t.Fatalf("GetByID().reference_file_refs[0].asset_id = %q, want %q", readModel.ReferenceFileRefs[0].AssetID, completeResult.ReferenceFileRef)
	}
}

func TestTaskReadModelPrefersReferenceFileRefsJSONOverLegacyReferenceImagesJSON(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			501: {
				ID:         501,
				TaskNo:     "T-501",
				TaskType:   domain.TaskTypeOriginalProductDevelopment,
				TaskStatus: domain.TaskStatusPendingAssign,
			},
		},
		details: map[int64]*domain.TaskDetail{
			501: {
				TaskID:                501,
				ReferenceFileRefsJSON: `[{"asset_id":"formal-ref"}]`,
				ReferenceImagesJSON:   `["legacy-ref"]`,
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	readModel, appErr := svc.GetByID(context.Background(), 501)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if len(readModel.ReferenceFileRefs) != 1 {
		t.Fatalf("reference_file_refs len = %d, want 1", len(readModel.ReferenceFileRefs))
	}
	if readModel.ReferenceFileRefs[0].AssetID != "formal-ref" {
		t.Fatalf("reference_file_refs[0].asset_id = %q, want formal-ref", readModel.ReferenceFileRefs[0].AssetID)
	}
}

func TestTaskReadModelFallsBackToLegacyReferenceImagesJSONWhenFormalFieldEmpty(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			502: {
				ID:         502,
				TaskNo:     "T-502",
				TaskType:   domain.TaskTypeOriginalProductDevelopment,
				TaskStatus: domain.TaskStatusPendingAssign,
			},
		},
		details: map[int64]*domain.TaskDetail{
			502: {
				TaskID:                502,
				ReferenceFileRefsJSON: `[]`,
				ReferenceImagesJSON:   `["legacy-ref"]`,
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	readModel, appErr := svc.GetByID(context.Background(), 502)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if len(readModel.ReferenceFileRefs) != 1 {
		t.Fatalf("reference_file_refs len = %d, want 1", len(readModel.ReferenceFileRefs))
	}
	if readModel.ReferenceFileRefs[0].AssetID != "legacy-ref" {
		t.Fatalf("reference_file_refs[0].asset_id = %q, want legacy-ref", readModel.ReferenceFileRefs[0].AssetID)
	}
}

func TestTaskReadModelPresignsReferenceFileRefsAtReadTime(t *testing.T) {
	now := time.Date(2026, 4, 20, 11, 0, 0, 0, time.UTC)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			503: {
				ID:             503,
				TaskNo:         "T-503",
				TaskType:       domain.TaskTypeNewProductDevelopment,
				TaskStatus:     domain.TaskStatusPendingAssign,
				IsBatchTask:    true,
				BatchItemCount: 2,
				BatchMode:      domain.TaskBatchModeMultiSKU,
				PrimarySKUCode: "SKU-503-A",
				SKUCode:        "SKU-503-A",
			},
		},
		details: map[int64]*domain.TaskDetail{
			503: {
				TaskID:                503,
				ReferenceFileRefsJSON: `[{"asset_id":"ref-oss","storage_key":"tasks/pre-create/ref-oss.png","download_url":"/v1/assets/files/tasks/pre-create/ref-oss.png"},{"asset_id":"ref-legacy","download_url":"/v1/assets/files/tasks/%E4%B8%AD%E6%96%87/ref-legacy.png"}]`,
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			503: {
				{
					ID:                1,
					TaskID:            503,
					SequenceNo:        1,
					SKUCode:           "SKU-503-A",
					ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: "sku-ref-a", StorageKey: "tasks/pre-create/sku-ref-a.png"}},
				},
				{
					ID:         2,
					TaskID:     503,
					SequenceNo: 2,
					SKUCode:    "SKU-503-B",
					ReferenceFileRefs: []domain.ReferenceFileRef{{
						AssetID:     "sku-ref-b",
						DownloadURL: stringPtr("/v1/assets/files/tasks/pre-create/sku-ref-b.png"),
					}},
				},
			},
		},
	}
	oss := newReferenceFileRefsTestOSS(now)
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskReferenceFileRefsOSSDirectService(oss),
	)

	readModel, appErr := svc.GetByID(context.Background(), 503)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if len(readModel.ReferenceFileRefs) != 2 {
		t.Fatalf("reference_file_refs len = %d, want 2", len(readModel.ReferenceFileRefs))
	}
	for _, ref := range readModel.ReferenceFileRefs {
		if ref.DownloadURL == nil || !strings.HasPrefix(*ref.DownloadURL, "https://yongbooss.oss-cn-hangzhou.aliyuncs.com/") {
			t.Fatalf("download_url = %v, want OSS presigned URL", ref.DownloadURL)
		}
		if !strings.Contains(*ref.DownloadURL, "Expires=") {
			t.Fatalf("download_url = %q, want Expires query", *ref.DownloadURL)
		}
		if ref.DownloadURLExpiresAt == nil || !ref.DownloadURLExpiresAt.Equal(now.Add(15*time.Minute)) {
			t.Fatalf("download_url_expires_at = %v, want %v", ref.DownloadURLExpiresAt, now.Add(15*time.Minute))
		}
	}
	if len(readModel.SKUItems) != 2 {
		t.Fatalf("sku_items len = %d, want 2", len(readModel.SKUItems))
	}
	for _, item := range readModel.SKUItems {
		if len(item.ReferenceFileRefs) != 1 {
			t.Fatalf("sku item refs = %+v", item.ReferenceFileRefs)
		}
		ref := item.ReferenceFileRefs[0]
		if ref.DownloadURL == nil || strings.HasPrefix(*ref.DownloadURL, "/v1/assets/files/") {
			t.Fatalf("sku ref download_url = %v, want presigned URL", ref.DownloadURL)
		}
		if ref.DownloadURLExpiresAt == nil || !ref.DownloadURLExpiresAt.Equal(now.Add(15*time.Minute)) {
			t.Fatalf("sku ref expires_at = %v, want %v", ref.DownloadURLExpiresAt, now.Add(15*time.Minute))
		}
	}
}

func TestTaskServiceCreateRejectsReferenceImagesBeforeTx(t *testing.T) {
	txRunner := &countingTxRunner{}
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		txRunner,
	)

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:                domain.TaskTypePurchaseTask,
		SourceMode:              domain.TaskSourceModeNewProduct,
		CreatorID:               9,
		OwnerTeam:               domain.AllValidTeams()[0],
		DeadlineAt:              referenceImageTestTimePtr(),
		PurchaseSKU:             "PUR-001",
		ReferenceImagesProvided: true,
		ReferenceImages:         []string{"data:image/png;base64,AAAA"},
	})
	if appErr == nil {
		t.Fatal("Create() expected error for reference_images")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("Create() error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
	if appErr.Message != "reference_images is no longer accepted in task creation; upload files first and use reference_file_refs" {
		t.Fatalf("Create() error message = %q", appErr.Message)
	}
	if txRunner.calls != 0 {
		t.Fatalf("RunInTx calls = %d, want 0", txRunner.calls)
	}

	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("Create() details type = %T, want map[string]interface{}", appErr.Details)
	}
	if details["field"] != "reference_images" {
		t.Fatalf("Create() field = %v", details["field"])
	}
	if details["suggestion"] != referenceFileRefsSuggestion {
		t.Fatalf("Create() suggestion = %v", details["suggestion"])
	}
}

func TestTaskServiceCreateRejectsInvalidReferenceFileRefs(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskReferenceFileRefValidation(newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo()),
	)

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:          domain.TaskTypeOriginalProductDevelopment,
		SourceMode:        domain.TaskSourceModeExistingProduct,
		CreatorID:         9,
		OwnerTeam:         domain.AllValidTeams()[0],
		DeadlineAt:        referenceImageTestTimePtr(),
		ChangeRequest:     "update design",
		ProductID:         int64Ptr(88),
		SKUCode:           "SKU-088",
		ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: "forged-ref"}},
	})
	if appErr == nil {
		t.Fatal("Create() expected error for invalid reference_file_refs")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("Create() error code = %s", appErr.Code)
	}
	if appErr.Message != "reference_file_refs contain invalid or unauthorized refs" {
		t.Fatalf("Create() error message = %q", appErr.Message)
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("Create() details type = %T", appErr.Details)
	}
	invalidRefs, ok := details["invalid_reference_file_refs"].([]map[string]interface{})
	if ok {
		if len(invalidRefs) != 1 || invalidRefs[0]["ref"] != "forged-ref" {
			t.Fatalf("invalid_reference_file_refs = %+v", invalidRefs)
		}
	} else {
		raw, marshalErr := json.Marshal(details["invalid_reference_file_refs"])
		if marshalErr != nil || !strings.Contains(string(raw), "forged-ref") {
			t.Fatalf("invalid_reference_file_refs = %#v", details["invalid_reference_file_refs"])
		}
	}
}

func TestTaskServiceCreateRejectsUncompletedReferenceFileRefs(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	referenceType := domain.TaskAssetTypeReference
	uploadRequestRepo.requests["upload-pending-ref"] = &domain.UploadRequest{
		RequestID:      "upload-pending-ref",
		OwnerType:      domain.AssetOwnerTypeTaskCreateReference,
		OwnerID:        9,
		TaskAssetType:  &referenceType,
		StorageAdapter: domain.AssetStorageAdapterOSSUploadService,
		UploadMode:     domain.DesignAssetUploadModeSmall,
		RefType:        domain.AssetStorageRefTypeGenericObject,
		FileName:       "reference.png",
		Status:         domain.UploadRequestStatusRequested,
		SessionStatus:  domain.DesignAssetSessionStatusCreated,
		RemoteUploadID: "remote-upload-pending",
		BoundRefID:     "pending-ref",
	}
	domain.HydrateUploadRequestDerived(uploadRequestRepo.requests["upload-pending-ref"])
	assetStorageRefRepo.refs["pending-ref"] = &domain.AssetStorageRef{
		RefID:           "pending-ref",
		OwnerType:       domain.AssetOwnerTypeTaskCreateReference,
		OwnerID:         9,
		UploadRequestID: "upload-pending-ref",
		StorageAdapter:  domain.AssetStorageAdapterOSSUploadService,
		RefType:         domain.AssetStorageRefTypeGenericObject,
		RefKey:          "objects/reference/pending",
		Status:          domain.AssetStorageRefStatusRecorded,
	}
	domain.HydrateAssetStorageRefDerived(assetStorageRefRepo.refs["pending-ref"])

	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskReferenceFileRefValidation(uploadRequestRepo, assetStorageRefRepo),
	)

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:          domain.TaskTypeOriginalProductDevelopment,
		SourceMode:        domain.TaskSourceModeExistingProduct,
		CreatorID:         9,
		OwnerTeam:         domain.AllValidTeams()[0],
		DeadlineAt:        referenceImageTestTimePtr(),
		ChangeRequest:     "update design",
		ProductID:         int64Ptr(88),
		SKUCode:           "SKU-088",
		ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: "pending-ref"}},
	})
	if appErr == nil {
		t.Fatal("Create() expected error for uncompleted reference_file_ref")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("Create() error code = %s", appErr.Code)
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("Create() details type = %T", appErr.Details)
	}
	raw, err := json.Marshal(details["invalid_reference_file_refs"])
	if err != nil {
		t.Fatalf("json.Marshal(invalid_reference_file_refs) error = %v", err)
	}
	if !strings.Contains(string(raw), "reference_file_ref_upload_not_bound") && !strings.Contains(string(raw), "reference_file_ref_upload_not_completed") {
		t.Fatalf("invalid_reference_file_refs = %s", string(raw))
	}
}

type countingTxRunner struct {
	calls int
}

func (r *countingTxRunner) RunInTx(_ context.Context, fn func(tx repo.Tx) error) error {
	r.calls++
	return fn(step04Tx{})
}

func referenceImageTestTimePtr() *time.Time {
	t := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
	return &t
}
