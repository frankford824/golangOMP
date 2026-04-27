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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"workflow/domain"
	"workflow/repo"
)

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

	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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
	completeResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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

func TestTaskAssetCenterServiceCompleteMultipartSkipsSecondRemoteCompleteAfterBrowserRemoteComplete(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2011, TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted
	uploadClient.failComplete = true

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)
	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	completeResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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

	reloadedSession, appErr := svc.GetUploadSession(context.Background(), 2011, createResult.Session.ID)
	if appErr != nil {
		t.Fatalf("GetUploadSession() unexpected error: %+v", appErr)
	}
	if reloadedSession.SessionStatus != domain.DesignAssetSessionStatusCompleted || reloadedSession.CurrentVersionID == nil {
		t.Fatalf("GetUploadSession() session = %+v", reloadedSession)
	}
}

func TestTaskAssetCenterServiceCompleteMultipartFallsBackToBackendRemoteComplete(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2013, TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCreated

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)
	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	completeResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2012, TaskStatus: domain.TaskStatusInProgress})
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted

	svc := NewTaskAssetCenterService(taskRepo, newStep67DesignAssetRepo(), newStep04TaskAssetRepo(), newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, uploadClient).(*taskAssetCenterService)
	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
		TaskID:       2012,
		CreatedBy:    512,
		AssetType:    domain.TaskAssetTypePreview,
		Filename:     "preview.png",
		ExpectedSize: uploadRequestInt64Ptr(4096),
		MimeType:     "image/png",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}

	firstResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2012,
		SessionID:   createResult.Session.ID,
		CompletedBy: 512,
		Remark:      "first finalize",
	})
	if appErr != nil {
		t.Fatalf("first CompleteUploadSession() unexpected error: %+v", appErr)
	}
	secondResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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

	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2040, TaskNo: "T-2040", TaskStatus: domain.TaskStatusInProgress})
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
	).(*taskAssetCenterService)

	body := []byte("canonical oss direct payload")
	bodyHash := sha256.Sum256(body)
	fileHash := hex.EncodeToString(bodyHash[:])

	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	completeResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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

	assets, appErr := svc.ListAssets(context.Background(), 2040)
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

func TestTaskAssetCenterServiceCreateOSSDirectMultipartUsesASCIIObjectKeyAndPreservesOriginalFilename(t *testing.T) {
	ossServer := newFakeOSSDirectServer(t)
	defer ossServer.Close()

	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2042, TaskNo: "T-2042", TaskStatus: domain.TaskStatusInProgress})
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
	).(*taskAssetCenterService)

	originalFilename := "手淘_SKU_13_蒙的都对【送12色涂鸦笔+木架】.jpg"
	body := []byte("special filename payload")
	bodyHash := sha256.Sum256(body)
	fileHash := hex.EncodeToString(bodyHash[:])

	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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
	if !regexp.MustCompile(`^tasks/T-2042/assets/AST-0001/v1/delivery/[A-Za-z0-9_]+\.jpg$`).MatchString(objectKey) {
		t.Fatalf("object key = %q, want ASCII delivery jpg key", objectKey)
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

	completeResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2041, TaskStatus: domain.TaskStatusInProgress})
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

	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	_, appErr = svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2002, TaskStatus: domain.TaskStatusInProgress})
	svc := NewTaskAssetCenterService(taskRepo, newStep67DesignAssetRepo(), newStep04TaskAssetRepo(), newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient()).(*taskAssetCenterService)

	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	session, appErr := svc.CancelUploadSession(context.Background(), CancelTaskAssetUploadSessionParams{
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

func TestTaskAssetCenterServiceRejectsSmallUploadForDeliverySourcePreview(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2004, TaskStatus: domain.TaskStatusInProgress})
	svc := NewTaskAssetCenterService(taskRepo, newStep67DesignAssetRepo(), newStep04TaskAssetRepo(), newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient()).(*taskAssetCenterService)

	_, appErr := svc.CreateSmallUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
		TaskID:       2004,
		CreatedBy:    503,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "delivery.png",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "image/png",
	})
	if appErr == nil {
		t.Fatal("CreateSmallUploadSession() appErr = nil, want invalid request")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("CreateSmallUploadSession() code = %s, want INVALID_REQUEST", appErr.Code)
	}
	if appErr.Message != "delivery/source/preview assets must use multipart upload mode" {
		t.Fatalf("CreateSmallUploadSession() message = %q", appErr.Message)
	}
}

func TestTaskAssetCenterServiceListAssetsAndVersions(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2003, TaskNo: "T-2003", TaskStatus: domain.TaskStatusPendingWarehouseReceive})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()

	assetID, _ := designAssetRepo.Create(context.Background(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2003,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeDelivery,
		CreatedBy: 601,
	})
	versionNo := 1
	versionID, _ := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
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
	_ = designAssetRepo.UpdateCurrentVersionID(context.Background(), step04Tx{}, assetID, &versionID)

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient())

	assets, appErr := svc.ListAssets(context.Background(), 2003)
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
	versions, appErr := svc.ListVersions(context.Background(), 2003, assetID)
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
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2005, TaskStatus: domain.TaskStatusInProgress, IsBatchTask: true, BatchItemCount: 2, BatchMode: domain.TaskBatchModeMultiSKU})
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
	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	completeResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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
	request, err := uploadRequestRepo.GetByRequestID(context.Background(), createResult.Session.ID)
	if err != nil {
		t.Fatalf("GetByRequestID() error = %v", err)
	}
	if request == nil || request.TargetSKUCode != "BATCH-2005-B" {
		t.Fatalf("upload request target_sku_code = %+v", request)
	}
}

func TestTaskAssetCenterServiceBatchDeliveryAdvancesOnlyAfterAllSKUCompleted(t *testing.T) {
	designerID := int64(530)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:             2006,
		TaskStatus:     domain.TaskStatusInProgress,
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

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)

	createA, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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
	if _, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2006,
		SessionID:   createA.Session.ID,
		CompletedBy: 530,
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession(A) unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[2006].TaskStatus != domain.TaskStatusInProgress {
		t.Fatalf("after first SKU complete task status = %s, want InProgress", taskRepo.tasks[2006].TaskStatus)
	}
	if countStep04TaskEvents(taskEventRepo.events, domain.TaskEventDesignSubmitted) != 0 {
		t.Fatalf("after first SKU complete design submitted events = %+v, want none", taskEventRepo.events)
	}

	createB, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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
	if _, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2006,
		SessionID:   createB.Session.ID,
		CompletedBy: 530,
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession(B) unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[2006].TaskStatus != domain.TaskStatusPendingAuditA {
		t.Fatalf("after all SKU complete task status = %s, want PendingAuditA", taskRepo.tasks[2006].TaskStatus)
	}
	if countStep04TaskEvents(taskEventRepo.events, domain.TaskEventDesignSubmitted) != 1 {
		t.Fatalf("design submitted events = %+v, want exactly one", taskEventRepo.events)
	}
}

func TestTaskAssetCenterServiceBatchCompleteAllowsPrecreatedLateSessionAfterPendingAuditA(t *testing.T) {
	designerID := int64(932)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:             2090,
		TaskStatus:     domain.TaskStatusInProgress,
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

	deliveryA, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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
	deliveryB, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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
	sourceB, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	if _, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2090,
		SessionID:   deliveryA.Session.ID,
		CompletedBy: 932,
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession(deliveryA) unexpected error: %+v", appErr)
	}
	if _, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2090,
		SessionID:   deliveryB.Session.ID,
		CompletedBy: 932,
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession(deliveryB) unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[2090].TaskStatus != domain.TaskStatusPendingAuditA {
		t.Fatalf("task status after delivery completion = %s, want PendingAuditA", taskRepo.tasks[2090].TaskStatus)
	}

	lateResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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

func TestTaskAssetCenterServiceCompletePrecreatedSessionAllowedInPendingAuditAWindow(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2091, TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	uploadClient.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted
	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)

	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
		TaskID:       2091,
		CreatedBy:    933,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "window-proof.zip",
		ExpectedSize: uploadRequestInt64Ptr(512),
		MimeType:     "application/zip",
	})
	if appErr != nil {
		t.Fatalf("CreateMultipartUploadSession() unexpected error: %+v", appErr)
	}
	taskRepo.tasks[2091].TaskStatus = domain.TaskStatusPendingAuditA

	if _, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2091,
		SessionID:   createResult.Session.ID,
		CompletedBy: 933,
	}); appErr != nil {
		t.Fatalf("CompleteUploadSession() unexpected error in PendingAuditA window: %+v", appErr)
	}
}

func TestTaskAssetCenterServiceCreateLateSessionStillRejectedInPendingAuditA(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2092, TaskStatus: domain.TaskStatusPendingAuditA})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	taskEventRepo := &step04TaskEventRepo{}
	storageRefRepo := newStep37AssetStorageRefRepo()
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    934,
		Roles: []domain.Role{domain.RoleAdmin},
	})

	_, appErr := svc.CreateMultipartUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:       2092,
		CreatedBy:    934,
		AssetType:    domain.TaskAssetTypeDelivery,
		Filename:     "late-session.zip",
		ExpectedSize: uploadRequestInt64Ptr(1024),
		MimeType:     "application/zip",
	})
	if appErr == nil {
		t.Fatal("CreateMultipartUploadSession() appErr = nil, want permission denied")
	}
	if appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("CreateMultipartUploadSession() code = %s, want PERMISSION_DENIED", appErr.Code)
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

	_, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
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

func TestTaskAssetCenterServiceCreateUploadSessionInfersModeByAssetType(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2008, TaskStatus: domain.TaskStatusInProgress})
	svc := NewTaskAssetCenterService(taskRepo, newStep67DesignAssetRepo(), newStep04TaskAssetRepo(), newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient()).(*taskAssetCenterService)

	referenceResult, appErr := svc.CreateUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
		TaskID:       2008,
		CreatedBy:    540,
		AssetType:    domain.TaskAssetTypeReference,
		Filename:     "reference.png",
		ExpectedSize: uploadRequestInt64Ptr(512),
		MimeType:     "image/png",
	})
	if appErr != nil {
		t.Fatalf("CreateUploadSession(reference) unexpected error: %+v", appErr)
	}
	if referenceResult.Session == nil || referenceResult.Session.UploadMode != domain.DesignAssetUploadModeMultipart {
		t.Fatalf("CreateUploadSession(reference) session = %+v", referenceResult.Session)
	}

	deliveryResult, appErr := svc.CreateUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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
	if deliveryResult.Session == nil || deliveryResult.Session.UploadMode != domain.DesignAssetUploadModeMultipart {
		t.Fatalf("CreateUploadSession(delivery) session = %+v", deliveryResult.Session)
	}
}

func TestTaskAssetCenterServiceUploadContentTypeContractDefaultsAndRejectsMismatch(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2032, TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	storageRefRepo := newStep37AssetStorageRefRepo()
	taskEventRepo := &step04TaskEventRepo{}
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)
	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)

	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	if _, appErr = svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2030, TaskNo: "T-2030", TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()
	uploadRequestRepo := newStep37UploadRequestRepo()
	storageRefRepo := newStep37AssetStorageRefRepo()
	taskEventRepo := &step04TaskEventRepo{}
	uploadClient := newStubUploadServiceClient().(*stubUploadServiceClient)

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)

	first, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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
	second, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	assets, err := designAssetRepo.ListByTaskID(context.Background(), 2030)
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

	assetID, _ := designAssetRepo.Create(context.Background(), step04Tx{}, &domain.DesignAsset{
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
	versionID, _ := taskAssetRepo.Create(context.Background(), step04Tx{}, taskAsset)
	_ = designAssetRepo.UpdateCurrentVersionID(context.Background(), step04Tx{}, assetID, &versionID)
	storageRefRepo.refs[storageRefID] = taskAsset.StorageRef

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, storageRefRepo, taskEventRepo, step04TxRunner{}, uploadClient).(*taskAssetCenterService)

	repaired, appErr := svc.repairMissingObjectStorageRef(context.Background(), versionID)
	if appErr != nil {
		t.Fatalf("repairMissingObjectStorageRef() unexpected error: %+v", appErr)
	}
	if !repaired {
		t.Fatal("repairMissingObjectStorageRef() repaired = false, want true")
	}
	if storageRefRepo.refs[storageRefID].Status != domain.AssetStorageRefStatusArchived {
		t.Fatalf("storage ref status = %s", storageRefRepo.refs[storageRefID].Status)
	}

	asset, appErr := svc.GetAsset(context.Background(), assetID)
	if appErr != nil {
		t.Fatalf("GetAsset() unexpected error: %+v", appErr)
	}
	if asset.CurrentVersion == nil || asset.CurrentVersion.StorageKey != "" {
		t.Fatalf("current version after repair = %+v", asset.CurrentVersion)
	}

	if _, appErr = svc.GetAssetDownloadInfoByID(context.Background(), assetID); appErr == nil {
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
		context.Background(),
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

	assetID, _ := designAssetRepo.Create(context.Background(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2031,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeDelivery,
		CreatedBy: 631,
	})
	versionNo := 1
	versionID, _ := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
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
	_ = designAssetRepo.UpdateCurrentVersionID(context.Background(), step04Tx{}, assetID, &versionID)

	directSvc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient())
	directInfo, appErr := directSvc.GetAssetDownloadInfoByID(context.Background(), assetID)
	if appErr != nil {
		t.Fatalf("direct GetAssetDownloadInfoByID() error = %+v", appErr)
	}
	if directInfo.DownloadMode != domain.AssetDownloadModeDirect || directInfo.DownloadURL == nil || !strings.Contains(*directInfo.DownloadURL, "https://oss-browser.test/files/") {
		t.Fatalf("direct download info = %+v", directInfo)
	}

	proxySvc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, nil)
	proxyInfo, appErr := proxySvc.GetAssetDownloadInfoByID(context.Background(), assetID)
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

	previewAssetID, _ := designAssetRepo.Create(context.Background(), step04Tx{}, &domain.DesignAsset{
		TaskID:       2009,
		AssetNo:      "AST-0001",
		ScopeSKUCode: "SKU-2009-A",
		AssetType:    domain.TaskAssetTypePreview,
		CreatedBy:    601,
	})
	previewVersionNo := 1
	previewVersionID, _ := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
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
	_ = designAssetRepo.UpdateCurrentVersionID(context.Background(), step04Tx{}, previewAssetID, &previewVersionID)

	sourceAssetID, _ := designAssetRepo.Create(context.Background(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2010,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeSource,
		CreatedBy: 602,
	})
	sourceVersionNo := 1
	sourceVersionID, _ := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
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
	_ = designAssetRepo.UpdateCurrentVersionID(context.Background(), step04Tx{}, sourceAssetID, &sourceVersionID)

	svc := NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, step04TxRunner{}, newStubUploadServiceClient()).(*taskAssetCenterService)

	list, appErr := svc.ListAssetResources(context.Background(), ListAssetResourcesParams{
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

	info, appErr := svc.GetAssetPreviewInfoByID(context.Background(), previewAssetID)
	if appErr != nil {
		t.Fatalf("GetAssetPreviewInfoByID() unexpected error: %+v", appErr)
	}
	if info == nil || !info.PreviewAvailable || info.DownloadURL == nil {
		t.Fatalf("GetAssetPreviewInfoByID() = %+v", info)
	}

	_, appErr = svc.GetAssetPreviewInfoByID(context.Background(), sourceAssetID)
	if appErr == nil {
		t.Fatal("GetAssetPreviewInfoByID(source) appErr = nil, want invalid state")
	}
}

func TestTaskAssetCenterServiceSourceDirectPreviewUsesOSSIMGProcess(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2050, TaskNo: "T-2050", TaskStatus: domain.TaskStatusInProgress})
	designAssetRepo := newStep67DesignAssetRepo()
	taskAssetRepo := newStep04TaskAssetRepo()

	sourceAssetID, _ := designAssetRepo.Create(context.Background(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2050,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeSource,
		CreatedBy: 650,
	})
	sourceVersionNo := 1
	sourceVersionID, _ := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
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
	_ = designAssetRepo.UpdateCurrentVersionID(context.Background(), step04Tx{}, sourceAssetID, &sourceVersionID)

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

	previewInfo, appErr := svc.GetAssetPreviewInfoByID(context.Background(), sourceAssetID)
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

	downloadInfo, appErr := svc.GetAssetDownloadInfoByID(context.Background(), sourceAssetID)
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

	sourceAssetID, _ := designAssetRepo.Create(context.Background(), step04Tx{}, &domain.DesignAsset{
		TaskID:    2051,
		AssetNo:   "AST-0001",
		AssetType: domain.TaskAssetTypeSource,
		CreatedBy: 651,
	})
	sourceVersionNo := 1
	sourceVersionID, _ := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
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
	_ = designAssetRepo.UpdateCurrentVersionID(context.Background(), step04Tx{}, sourceAssetID, &sourceVersionID)

	derivedPreviewAssetID, _ := designAssetRepo.Create(context.Background(), step04Tx{}, &domain.DesignAsset{
		TaskID:        2051,
		AssetNo:       "AST-0002",
		SourceAssetID: &sourceAssetID,
		AssetType:     domain.TaskAssetTypePreview,
		CreatedBy:     651,
	})
	derivedVersionNo := 1
	derivedVersionID, _ := taskAssetRepo.Create(context.Background(), step04Tx{}, &domain.TaskAsset{
		TaskID:         2051,
		AssetID:        &derivedPreviewAssetID,
		AssetType:      domain.TaskAssetTypePreview,
		VersionNo:      2,
		AssetVersionNo: &derivedVersionNo,
		UploadMode:     strPtr("small"),
		FileName:       "preview.png",
		OriginalName:   strPtr("preview.png"),
		MimeType:       strPtr("image/png"),
		StorageKey:     strPtr("objects/design-assets/source-preview.png"),
		UploadStatus:   strPtr("uploaded"),
		PreviewStatus:  strPtr("not_applicable"),
		UploadedBy:     651,
		UploadedAt:     timeValuePtr(time.Date(2026, 4, 15, 9, 12, 0, 0, time.UTC)),
	})
	_ = designAssetRepo.UpdateCurrentVersionID(context.Background(), step04Tx{}, derivedPreviewAssetID, &derivedVersionID)

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

	info, appErr := svc.GetAssetPreviewInfoByID(context.Background(), sourceAssetID)
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
	).(*taskAssetCenterService)
	svc.runAsyncFn = func(fn func()) { fn() }
	svc.derivedPreviewGracePeriod = 0

	body := []byte("source-psd-binary")
	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	completeResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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
	previewAssets, appErr := svc.ListAssetResources(context.Background(), ListAssetResourcesParams{
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
	thumbAssets, appErr := svc.ListAssetResources(context.Background(), ListAssetResourcesParams{
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

	previewInfo, appErr := svc.GetAssetPreviewInfoByID(context.Background(), sourceAssetID)
	if appErr != nil {
		t.Fatalf("GetAssetPreviewInfoByID(source) unexpected error: %+v", appErr)
	}
	if previewInfo == nil || previewInfo.DownloadURL == nil {
		t.Fatalf("GetAssetPreviewInfoByID(source) = %+v", previewInfo)
	}
	if !strings.Contains(*previewInfo.DownloadURL, previewAssets[0].CurrentVersion.StorageKey) {
		t.Fatalf("preview url = %q, want derived preview key %q", *previewInfo.DownloadURL, previewAssets[0].CurrentVersion.StorageKey)
	}

	downloadInfo, appErr := svc.GetAssetDownloadInfoByID(context.Background(), sourceAssetID)
	if appErr != nil {
		t.Fatalf("GetAssetDownloadInfoByID(source) unexpected error: %+v", appErr)
	}
	if downloadInfo == nil || downloadInfo.DownloadURL == nil {
		t.Fatalf("GetAssetDownloadInfoByID(source) = %+v", downloadInfo)
	}
	if !strings.Contains(*downloadInfo.DownloadURL, completeResult.Version.StorageKey) {
		t.Fatalf("download url = %q, want source key %q", *downloadInfo.DownloadURL, completeResult.Version.StorageKey)
	}
}

func TestTaskAssetCenterServiceCompleteSourceThenDeliverySeriallyWithout500(t *testing.T) {
	ossServer := newFakeOSSDirectServer(t)
	defer ossServer.Close()

	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2053, TaskNo: "T-2053", TaskStatus: domain.TaskStatusInProgress})
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
	).(*taskAssetCenterService)

	var derivedJobs []func()
	svc.runAsyncFn = func(fn func()) {
		derivedJobs = append(derivedJobs, fn)
	}

	sourceBody := []byte("serial-source-psd")
	sourceSession, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	sourceResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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

	deliverySession, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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
	deliveryResult, appErr := svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
		TaskID:      2053,
		SessionID:   deliverySession.Session.ID,
		CompletedBy: 653,
		FileHash:    "serial-delivery-hash",
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

func TestTaskAssetCenterServiceCompleteUploadSessionMapsTaskAssetVersionDuplicateToConflict(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{ID: 2054, TaskStatus: domain.TaskStatusInProgress})
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

	createResult, appErr := svc.CreateMultipartUploadSession(context.Background(), CreateTaskAssetUploadSessionParams{
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

	_, appErr = svc.CompleteUploadSession(context.Background(), CompleteTaskAssetUploadSessionParams{
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
}

func newFakeOSSDirectServer(t *testing.T) *fakeOSSDirectServer {
	t.Helper()
	fake := &fakeOSSDirectServer{t: t}
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
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("ETag", fmt.Sprintf("\"etag-%s-%s\"", query.Get("uploadId"), partNumber))
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPut && query.Get("uploadId") == "" && query.Get("partNumber") == "":
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPost && query.Get("uploadId") != "":
		f.mu.Lock()
		f.completeCalls++
		f.mu.Unlock()
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, "<CompleteMultipartUploadResult></CompleteMultipartUploadResult>")
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
