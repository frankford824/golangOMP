package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestTaskCreateReferenceUploadServiceBindsRemotelyCompletedSession(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, uploadClient).(*taskCreateReferenceUploadService)

	created, appErr := svc.CreateUploadSession(context.Background(), CreateTaskReferenceUploadSessionParams{
		CreatedBy:    9,
		Filename:     "reference-completed-remotely.png",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "image/png",
		FileHash:     "hash-completed-remotely",
	})
	if appErr != nil {
		t.Fatalf("CreateUploadSession() unexpected error: %+v", appErr)
	}

	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted
	uploadClient.remoteSessionFileID = "file-completed-remotely"
	uploadClient.lastCompletedSize = 1024
	uploadClient.lastCompletedHash = "hash-completed-remotely"
	uploadClient.failComplete = true

	completed, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskReferenceUploadSessionParams{
		SessionID:   created.Session.ID,
		CompletedBy: 9,
		FileHash:    "hash-completed-remotely",
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession() unexpected error: %+v", appErr)
	}
	if completed.Session == nil || completed.Session.SessionStatus != domain.DesignAssetSessionStatusCompleted {
		t.Fatalf("CompleteUploadSession() session = %+v", completed.Session)
	}
	if completed.ReferenceFileRef == "" || completed.StorageRef == nil || completed.RefObject == nil {
		t.Fatalf("CompleteUploadSession() result = %+v", completed)
	}
	if uploadClient.completeCalls != 0 {
		t.Fatalf("CompleteUploadSession() remote complete calls = %d, want 0", uploadClient.completeCalls)
	}
	if uploadClient.getFileMetaCalls != 1 {
		t.Fatalf("CompleteUploadSession() get file meta calls = %d, want 1", uploadClient.getFileMetaCalls)
	}
	request := uploadRequestRepo.requests[created.Session.ID]
	if request.Status != domain.UploadRequestStatusBound || request.BoundRefID != completed.ReferenceFileRef {
		t.Fatalf("upload request = %+v", request)
	}
}

func TestTaskCreateReferenceUploadServiceOSSDirectSessionCompletesWithoutBackendFileProxy(t *testing.T) {
	const fileSize = int64(1024)
	deleteCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalls++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodHead {
			t.Fatalf("method = %s, want HEAD or DELETE", r.Method)
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse OSS test server URL: %v", err)
	}
	ossDirect := NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        strings.TrimPrefix(server.URL, "https://"),
		PublicEndpoint:  strings.TrimPrefix(server.URL, "https://"),
		Bucket:          "test-bucket",
		AccessKeyID:     "test-id",
		AccessKeySecret: "test-secret",
		PartSize:        10 * 1024 * 1024,
	})
	httpClient := server.Client()
	httpClient.Transport = &rewriteHostTransport{base: baseURL, inner: httpClient.Transport}
	ossDirect.httpClient = httpClient

	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	stub := newStubUploadServiceClient().(*stubUploadServiceClient)
	svc := NewTaskCreateReferenceUploadService(
		uploadRequestRepo,
		assetStorageRefRepo,
		step04TxRunner{},
		stub,
		WithTaskCreateReferenceOSSDirectService(ossDirect),
	).(*taskCreateReferenceUploadService)

	created, appErr := svc.CreateUploadSession(context.Background(), CreateTaskReferenceUploadSessionParams{
		CreatedBy:    9,
		Filename:     "reference.png",
		ExpectedSize: uploadRequestInt64Ptr(fileSize),
		MimeType:     "image/png",
	})
	if appErr != nil {
		t.Fatalf("CreateUploadSession() error = %+v", appErr)
	}
	if created.OSSDirect == nil || created.OSSDirect.Mode != "single_part" || created.OSSDirect.UploadURL == "" {
		t.Fatalf("OSS direct plan = %+v", created.OSSDirect)
	}
	if created.Remote != nil || len(stub.createRequests) != 0 {
		t.Fatalf("remote fallback unexpectedly created: remote=%+v calls=%d", created.Remote, len(stub.createRequests))
	}
	completed, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskReferenceUploadSessionParams{
		SessionID:         created.Session.ID,
		CompletedBy:       9,
		UploadContentType: "image/png",
		OSSObjectKey:      created.OSSDirect.ObjectKey,
	})
	if appErr != nil {
		t.Fatalf("CompleteUploadSession() error = %+v", appErr)
	}
	if completed.RefObject == nil || completed.RefObject.AssetID == "" || completed.RefObject.Source != domain.ReferenceFileRefSourceTaskReferenceUpload {
		t.Fatalf("completed ref_object = %+v", completed.RefObject)
	}
	if stub.completeCalls != 0 {
		t.Fatalf("remote complete calls = %d, want 0", stub.completeCalls)
	}

	cancelCreated, appErr := svc.CreateUploadSession(context.Background(), CreateTaskReferenceUploadSessionParams{
		CreatedBy:    9,
		Filename:     "cancel.png",
		ExpectedSize: uploadRequestInt64Ptr(fileSize),
		MimeType:     "image/png",
	})
	if appErr != nil {
		t.Fatalf("CreateUploadSession(cancel) error = %+v", appErr)
	}
	cancelled, appErr := svc.CancelUploadSession(context.Background(), CancelTaskReferenceUploadSessionParams{
		SessionID:   cancelCreated.Session.ID,
		CancelledBy: 9,
	})
	if appErr != nil {
		t.Fatalf("CancelUploadSession() error = %+v", appErr)
	}
	if cancelled.SessionStatus != domain.DesignAssetSessionStatusCancelled || deleteCalls != 1 {
		t.Fatalf("cancelled session = %+v deleteCalls=%d", cancelled, deleteCalls)
	}
}

func TestTaskCreateReferenceUploadServiceCreateRejectsExpectedSizeAboveLimit(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, uploadClient).(*taskCreateReferenceUploadService)

	tooLarge := TaskCreateReferenceUploadMaxFileSizeBytes + 1
	_, appErr := svc.CreateUploadSession(context.Background(), CreateTaskReferenceUploadSessionParams{
		CreatedBy:    501,
		Filename:     "reference-too-large.png",
		ExpectedSize: &tooLarge,
		MimeType:     "image/png",
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

func TestTaskCreateReferenceUploadServiceUploadFileRejectsExpectedSizeAboveLimitBeforeRead(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	svc := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, uploadClient).(*taskCreateReferenceUploadService)

	tooLarge := TaskCreateReferenceUploadMaxFileSizeBytes + 1
	_, appErr := svc.UploadFile(context.Background(), UploadTaskReferenceFileParams{
		CreatedBy:    501,
		Filename:     "reference-too-large.png",
		ExpectedSize: &tooLarge,
		MimeType:     "image/png",
		File:         bytes.NewReader([]byte("small")),
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("UploadFile() appErr = %+v, want invalid request", appErr)
	}
	if appErr.Message != "expected_size exceeds upload limit" {
		t.Fatalf("UploadFile() message = %q, want expected_size exceeds upload limit", appErr.Message)
	}
	if len(uploadClient.createRequests) != 0 {
		t.Fatalf("remote create calls = %d, want 0", len(uploadClient.createRequests))
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
	const wantDownloadURL = "/v1/assets/files/tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/%F0%9F%92%9A97%25%20%E8%83%BD%E9%87%8F%E5%85%85%E6%BB%A1%E5%95%A6.jpg?download_filename=%F0%9F%92%9A97%25+%E8%83%BD%E9%87%8F%E5%85%85%E6%BB%A1%E5%95%A6.jpg"
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
