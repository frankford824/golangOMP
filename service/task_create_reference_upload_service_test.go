package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"workflow/domain"
)

func TestTaskCreateReferenceUploadServiceCreateAndComplete(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, newStubUploadServiceClient()).(*taskCreateReferenceUploadService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 19, 2, 0, 0, 0, time.UTC)
	}

	createResult, appErr := svc.CreateUploadSession(context.Background(), CreateTaskReferenceUploadSessionParams{
		CreatedBy:    9,
		Filename:     "reference-a.png",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "image/png",
		FileHash:     "hash-a",
	})
	if appErr != nil {
		t.Fatalf("CreateUploadSession() unexpected error: %+v", appErr)
	}
	if createResult.Session == nil || createResult.Session.SessionStatus != domain.DesignAssetSessionStatusCreated {
		t.Fatalf("CreateUploadSession() session = %+v", createResult.Session)
	}

	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 19, 2, 5, 0, 0, time.UTC)
	}
	completeResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskReferenceUploadSessionParams{
		SessionID:   createResult.Session.ID,
		CompletedBy: 9,
		FileHash:    "hash-a",
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession() unexpected error: %+v", appErr)
	}
	if completeResult.Session == nil || completeResult.Session.SessionStatus != domain.DesignAssetSessionStatusCompleted {
		t.Fatalf("CompleteUploadSession() session = %+v", completeResult.Session)
	}
	if completeResult.ReferenceFileRef == "" {
		t.Fatalf("CompleteUploadSession() reference_file_ref = %q", completeResult.ReferenceFileRef)
	}
	if completeResult.StorageRef == nil || completeResult.StorageRef.RefID != completeResult.ReferenceFileRef {
		t.Fatalf("CompleteUploadSession() storage_ref = %+v", completeResult.StorageRef)
	}
	if completeResult.RefObject == nil || completeResult.RefObject.AssetID != completeResult.ReferenceFileRef {
		t.Fatalf("CompleteUploadSession() ref_object = %+v", completeResult.RefObject)
	}
	if completeResult.RefObject.Source != domain.ReferenceFileRefSourceTaskCreateAssetCenter {
		t.Fatalf("CompleteUploadSession() ref_object.source = %q", completeResult.RefObject.Source)
	}

	request := uploadRequestRepo.requests[createResult.Session.ID]
	if request.Status != domain.UploadRequestStatusBound {
		t.Fatalf("upload request status = %s, want bound", request.Status)
	}
	if request.SessionStatus != domain.DesignAssetSessionStatusCompleted {
		t.Fatalf("upload request session_status = %s, want completed", request.SessionStatus)
	}
	if request.BoundRefID != completeResult.ReferenceFileRef {
		t.Fatalf("upload request bound_ref_id = %s, want %s", request.BoundRefID, completeResult.ReferenceFileRef)
	}
}

func TestTaskCreateReferenceUploadServiceUploadFile(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	stub := newStubUploadServiceClient().(*stubUploadServiceClient)
	stub.failComplete = true
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, stub).(*taskCreateReferenceUploadService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 19, 3, 0, 0, 0, time.UTC)
	}

	refObject, appErr := svc.UploadFile(context.Background(), UploadTaskReferenceFileParams{
		CreatedBy:    9,
		Filename:     "reference-upload.png",
		ExpectedSize: uploadRequestInt64Ptr(12),
		MimeType:     "image/png",
		File:         bytes.NewBufferString("hello world!"),
	})
	if appErr != nil {
		t.Fatalf("UploadFile() unexpected error: %+v", appErr)
	}
	if refObject == nil {
		t.Fatal("UploadFile() ref_object = nil")
	}
	if refObject.AssetID == "" || refObject.RefID == "" {
		t.Fatalf("UploadFile() ref_object ids = %+v", refObject)
	}
	if refObject.Source != domain.ReferenceFileRefSourceTaskReferenceUpload {
		t.Fatalf("UploadFile() source = %q", refObject.Source)
	}
	if refObject.DownloadURL == nil || *refObject.DownloadURL == "" {
		t.Fatalf("UploadFile() download_url = %+v", refObject.DownloadURL)
	}
	if stub.completeCalls != 0 {
		t.Fatalf("UploadFile() completeCalls = %d, want 0 for small reference", stub.completeCalls)
	}
}

func TestTaskCreateReferenceUploadServiceUploadFileUsesOSSDirectWhenEnabled(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	stub := newStubUploadServiceClient().(*stubUploadServiceClient)
	ossDirect := NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        "oss-cn-hangzhou.aliyuncs.com",
		Bucket:          "workflow-test",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		PublicEndpoint:  "oss-cn-hangzhou.aliyuncs.com",
	})
	svc := NewTaskCreateReferenceUploadService(
		uploadRequestRepo,
		assetStorageRefRepo,
		step04TxRunner{},
		stub,
		WithTaskCreateReferenceOSSDirectService(ossDirect),
	).(*taskCreateReferenceUploadService)
	svc.ossDirectUploadFn = func(_ context.Context, objectKey, contentType string, body []byte) error {
		if objectKey == "" {
			t.Fatal("oss direct object key is empty")
		}
		if contentType != "image/png" {
			t.Fatalf("oss direct content_type = %q", contentType)
		}
		if string(body) != "hello world!" {
			t.Fatalf("oss direct upload body mismatch: %q", string(body))
		}
		return nil
	}

	refObject, appErr := svc.UploadFile(context.Background(), UploadTaskReferenceFileParams{
		CreatedBy:    9,
		Filename:     "reference-upload.png",
		ExpectedSize: uploadRequestInt64Ptr(12),
		MimeType:     "image/png",
		File:         bytes.NewBufferString("hello world!"),
	})
	if appErr != nil {
		t.Fatalf("UploadFile() unexpected error: %+v", appErr)
	}
	if refObject == nil || refObject.AssetID == "" {
		t.Fatalf("UploadFile() ref_object = %+v", refObject)
	}
	if len(stub.createRequests) != 0 {
		t.Fatalf("UploadFile() should not call upload-service create_session in oss direct mode")
	}
	if stub.completeCalls != 0 {
		t.Fatalf("UploadFile() completeCalls = %d, want 0 in oss direct mode", stub.completeCalls)
	}
}

func TestTaskCreateReferenceUploadServiceBuildReferenceFileRefEscapesURLs(t *testing.T) {
	svc := NewTaskCreateReferenceUploadService(
		newStep37UploadRequestRepo(),
		newStep37AssetStorageRefRepo(),
		step04TxRunner{},
		newStubUploadServiceClient(),
	).(*taskCreateReferenceUploadService)

	ref := svc.buildReferenceFileRef(&domain.AssetStorageRef{
		RefID:           "ref-1",
		UploadRequestID: "req-1",
		FileName:        "💚97% 能量充满啦.jpg",
		MimeType:        "image/jpeg",
		FileSize:        uploadRequestInt64Ptr(12),
		RefKey:          "tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/💚97% 能量充满啦.jpg",
	}, domain.ReferenceFileRefSourceTaskReferenceUpload)
	if ref == nil {
		t.Fatal("buildReferenceFileRef() = nil")
	}
	const wantDownloadURL = "/v1/assets/files/tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/%F0%9F%92%9A97%25%20%E8%83%BD%E9%87%8F%E5%85%85%E6%BB%A1%E5%95%A6.jpg"
	if ref.DownloadURL == nil || *ref.DownloadURL != wantDownloadURL {
		t.Fatalf("buildReferenceFileRef() download_url = %+v, want %q", ref.DownloadURL, wantDownloadURL)
	}
	if ref.URL == nil || *ref.URL != wantDownloadURL {
		t.Fatalf("buildReferenceFileRef() url = %+v, want %q", ref.URL, wantDownloadURL)
	}
}

func TestTaskCreateReferenceUploadServiceUploadFileRejectsProvidedHashMismatch(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, newStubUploadServiceClient()).(*taskCreateReferenceUploadService)

	_, appErr := svc.UploadFile(context.Background(), UploadTaskReferenceFileParams{
		CreatedBy:    9,
		Filename:     "reference-upload.png",
		ExpectedSize: uploadRequestInt64Ptr(12),
		MimeType:     "image/png",
		FileHash:     "wrong-hash",
		File:         bytes.NewBufferString("hello world!"),
	})
	if appErr == nil {
		t.Fatal("UploadFile() appErr = nil, want invalid request")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("UploadFile() code = %s", appErr.Code)
	}
}

func TestTaskCreateReferenceUploadServiceCompleteRejectsStoredSizeMismatch(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	stub := newStubUploadServiceClient().(*stubUploadServiceClient)
	probeBytes := int64(0)
	stub.probeBytesOverride = &probeBytes
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, stub).(*taskCreateReferenceUploadService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 3, 19, 4, 0, 0, 0, time.UTC)
	}

	createResult, appErr := svc.CreateUploadSession(context.Background(), CreateTaskReferenceUploadSessionParams{
		CreatedBy:    9,
		Filename:     "reference-a.png",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "image/png",
		FileHash:     "hash-a",
	})
	if appErr != nil {
		t.Fatalf("CreateUploadSession() unexpected error: %+v", appErr)
	}

	_, appErr = svc.CompleteUploadSession(context.Background(), CompleteTaskReferenceUploadSessionParams{
		SessionID:   createResult.Session.ID,
		CompletedBy: 9,
		FileHash:    "hash-a",
	})
	if appErr == nil {
		t.Fatal("CompleteUploadSession() appErr = nil, want verification failure")
	}
	if appErr.Code != domain.ErrCodeInternalError {
		t.Fatalf("CompleteUploadSession() code = %s", appErr.Code)
	}
	if appErr.Message != "reference upload stored file verification failed" {
		t.Fatalf("CompleteUploadSession() message = %q", appErr.Message)
	}
}

func TestTaskCreateReferenceUploadServiceUploadFileReturnsInternalErrorWhenProbeUnavailable(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	stub := newStubUploadServiceClient().(*stubUploadServiceClient)
	stub.probeErr = &UploadServiceHTTPError{Operation: "probe_stored_file", StatusCode: http.StatusBadGateway, Body: "upstream unavailable"}
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, stub).(*taskCreateReferenceUploadService)
	svc.sleepFn = func(time.Duration) {}

	_, appErr := svc.UploadFile(context.Background(), UploadTaskReferenceFileParams{
		CreatedBy:    9,
		Filename:     "reference-upload.png",
		ExpectedSize: uploadRequestInt64Ptr(12),
		MimeType:     "image/png",
		File:         bytes.NewBufferString("hello world!"),
	})
	if appErr == nil {
		t.Fatal("UploadFile() appErr = nil, want internal error")
	}
	if appErr.Code != domain.ErrCodeInternalError {
		t.Fatalf("UploadFile() code = %s", appErr.Code)
	}
	if appErr.Message != "internal error during probe task-create reference stored file" {
		t.Fatalf("UploadFile() message = %q", appErr.Message)
	}
}

func TestTaskCreateReferenceUploadServiceUploadFileRejectsProbeIncompleteMetadata(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	stub := newStubUploadServiceClient().(*stubUploadServiceClient)
	stub.probeResponseSequence = []*RemoteStoredFileProbe{{
		StatusCode:          200,
		ContentType:         "image/png",
		ContentLengthHeader: 12,
		BytesRead:           12,
		SHA256:              "",
	}}
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, stub).(*taskCreateReferenceUploadService)
	svc.sleepFn = func(time.Duration) {}

	_, appErr := svc.UploadFile(context.Background(), UploadTaskReferenceFileParams{
		CreatedBy:    9,
		Filename:     "reference-upload.png",
		ExpectedSize: uploadRequestInt64Ptr(12),
		MimeType:     "image/png",
		File:         bytes.NewBufferString("hello world!"),
	})
	if appErr == nil {
		t.Fatal("UploadFile() appErr = nil, want internal error")
	}
	if appErr.Code != domain.ErrCodeInternalError {
		t.Fatalf("UploadFile() code = %s", appErr.Code)
	}
	if appErr.Message != "internal error during probe task-create reference stored file" {
		t.Fatalf("UploadFile() message = %q", appErr.Message)
	}
}

func TestTaskCreateReferenceUploadServiceUploadFileRejectsStoredHashMismatch(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	stub := newStubUploadServiceClient().(*stubUploadServiceClient)
	stub.probeHashOverride = "wrong-sha"
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, stub).(*taskCreateReferenceUploadService)
	svc.sleepFn = func(time.Duration) {}

	_, appErr := svc.UploadFile(context.Background(), UploadTaskReferenceFileParams{
		CreatedBy:    9,
		Filename:     "reference-upload.png",
		ExpectedSize: uploadRequestInt64Ptr(12),
		MimeType:     "image/png",
		File:         bytes.NewBufferString("hello world!"),
	})
	if appErr == nil {
		t.Fatal("UploadFile() appErr = nil, want hash verification failure")
	}
	if appErr.Code != domain.ErrCodeInternalError {
		t.Fatalf("UploadFile() code = %s", appErr.Code)
	}
	if appErr.Message != "reference upload stored file hash verification failed" {
		t.Fatalf("UploadFile() message = %q", appErr.Message)
	}
}

func TestTaskCreateReferenceUploadServiceUploadFileRetriesTransientProbeFailure(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	stub := newStubUploadServiceClient().(*stubUploadServiceClient)
	stub.probeErrSequence = []error{
		&UploadServiceHTTPError{Operation: "probe_stored_file", StatusCode: http.StatusNotFound, Body: "not ready"},
		nil,
	}
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, stub).(*taskCreateReferenceUploadService)
	svc.sleepFn = func(time.Duration) {}

	refObject, appErr := svc.UploadFile(context.Background(), UploadTaskReferenceFileParams{
		CreatedBy:    9,
		Filename:     "reference-upload.png",
		ExpectedSize: uploadRequestInt64Ptr(12),
		MimeType:     "image/png",
		File:         bytes.NewBufferString("hello world!"),
	})
	if appErr != nil {
		t.Fatalf("UploadFile() unexpected error: %+v", appErr)
	}
	if refObject == nil || refObject.AssetID == "" {
		t.Fatalf("UploadFile() ref_object = %+v", refObject)
	}
	if len(stub.probeErrSequence) != 0 {
		t.Fatalf("UploadFile() probeErrSequence remaining = %d, want 0", len(stub.probeErrSequence))
	}
}

func TestTaskCreateReferenceUploadServiceUploadFileRetriesEmptyProbeThenSucceeds(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	stub := newStubUploadServiceClient().(*stubUploadServiceClient)
	stub.probeResponseSequence = []*RemoteStoredFileProbe{
		{
			StatusCode:          200,
			ContentType:         "application/octet-stream",
			ContentLengthHeader: 0,
			BytesRead:           0,
			SHA256:              "",
		},
	}
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, stub).(*taskCreateReferenceUploadService)
	svc.sleepFn = func(time.Duration) {}

	refObject, appErr := svc.UploadFile(context.Background(), UploadTaskReferenceFileParams{
		CreatedBy:    9,
		Filename:     "reference-upload.png",
		ExpectedSize: uploadRequestInt64Ptr(12),
		MimeType:     "image/png",
		File:         bytes.NewBufferString("hello world!"),
	})
	if appErr != nil {
		t.Fatalf("UploadFile() unexpected error: %+v", appErr)
	}
	if refObject == nil || refObject.AssetID == "" {
		t.Fatalf("UploadFile() ref_object = %+v", refObject)
	}
}

func TestTaskCreateReferenceUploadServiceUploadFileFailsAfterProbeRetryExhausted(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	stub := newStubUploadServiceClient().(*stubUploadServiceClient)
	stub.probeErr = &UploadServiceHTTPError{Operation: "probe_stored_file", StatusCode: http.StatusNotFound, Body: "not ready"}
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, stub).(*taskCreateReferenceUploadService)
	svc.sleepFn = func(time.Duration) {}
	svc.probeRetryMax = 2

	_, appErr := svc.UploadFile(context.Background(), UploadTaskReferenceFileParams{
		CreatedBy:    9,
		Filename:     "reference-upload.png",
		ExpectedSize: uploadRequestInt64Ptr(12),
		MimeType:     "image/png",
		File:         bytes.NewBufferString("hello world!"),
	})
	if appErr == nil {
		t.Fatal("UploadFile() appErr = nil, want internal error")
	}
	if appErr.Code != domain.ErrCodeInternalError {
		t.Fatalf("UploadFile() code = %s", appErr.Code)
	}
	if appErr.Message != "internal error during probe task-create reference stored file" {
		t.Fatalf("UploadFile() message = %q", appErr.Message)
	}
}

func TestTaskCreateReferenceUploadServiceRetryableProbeErrorClassification(t *testing.T) {
	if !isRetryableStoredFileProbeError(&UploadServiceHTTPError{StatusCode: http.StatusNotFound}) {
		t.Fatal("404 should be retryable")
	}
	if isRetryableStoredFileProbeError(&UploadServiceHTTPError{StatusCode: http.StatusForbidden}) {
		t.Fatal("403 should not be retryable")
	}
	if isRetryableStoredFileProbeError(errors.New("plain failure")) {
		t.Fatal("plain failure should not be retryable")
	}
}
