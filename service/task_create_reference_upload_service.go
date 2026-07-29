package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

type CreateTaskReferenceUploadSessionParams struct {
	CreatedBy      int64
	OwnerType      domain.AssetOwnerType
	ClientCreateID string
	ClientItemID   string
	Filename       string
	ExpectedSize   *int64
	MimeType       string
	FileHash       string
	Remark         string
}

type CompleteTaskReferenceUploadSessionParams struct {
	SessionID         string
	CompletedBy       int64
	OwnerType         domain.AssetOwnerType
	Remark            string
	FileHash          string
	ExpectedSHA256    string
	UploadContentType string
	OSSParts          []OSSCompletePart
	OSSUploadID       string
	OSSObjectKey      string
}

type CancelTaskReferenceUploadSessionParams struct {
	SessionID    string
	CancelledBy  int64
	OwnerType    domain.AssetOwnerType
	Remark       string
	OSSUploadID  string
	OSSObjectKey string
}

type CreateTaskReferenceUploadSessionResult struct {
	Session          *domain.UploadSession    `json:"session"`
	Remote           *RemoteUploadSessionPlan `json:"remote,omitempty"`
	OSSDirect        *OSSDirectUploadPlan     `json:"oss_direct,omitempty"`
	CompleteEndpoint string                   `json:"complete_endpoint,omitempty"`
	CancelEndpoint   string                   `json:"cancel_endpoint,omitempty"`
}

type UploadTaskReferenceFileParams struct {
	CreatedBy    int64
	Filename     string
	ExpectedSize *int64
	MimeType     string
	FileHash     string
	Remark       string
	File         io.Reader
}

const (
	TaskCreateReferenceUploadMaxFileSizeBytes = int64(300 * 1024 * 1024)
	TaskCreateReferenceUploadMaxFileSizeLabel = "300MB"
)

func isPreCreateUploadOwner(ownerType domain.AssetOwnerType) bool {
	return ownerType == domain.AssetOwnerTypeTaskCreateReference || ownerType == domain.AssetOwnerTypePlanningSKUCreate
}

func preCreateUploadNamespace(ownerType domain.AssetOwnerType) string {
	if ownerType == domain.AssetOwnerTypePlanningSKUCreate {
		return "planning-sku-create"
	}
	return "task-create-reference"
}

type CompleteTaskReferenceUploadSessionResult struct {
	Session          *domain.UploadSession    `json:"session"`
	ReferenceFileRef string                   `json:"reference_file_ref"`
	StorageRef       *domain.AssetStorageRef  `json:"storage_ref,omitempty"`
	RefObject        *domain.ReferenceFileRef `json:"ref_object,omitempty"`
}

type TaskCreateReferenceUploadService interface {
	CreateUploadSession(ctx context.Context, params CreateTaskReferenceUploadSessionParams) (*CreateTaskReferenceUploadSessionResult, *domain.AppError)
	GetUploadSession(ctx context.Context, sessionID string, requesterID int64) (*domain.UploadSession, *domain.AppError)
	UploadFile(ctx context.Context, params UploadTaskReferenceFileParams) (*domain.ReferenceFileRef, *domain.AppError)
	CompleteUploadSession(ctx context.Context, params CompleteTaskReferenceUploadSessionParams) (*CompleteTaskReferenceUploadSessionResult, *domain.AppError)
	CancelUploadSession(ctx context.Context, params CancelTaskReferenceUploadSessionParams) (*domain.UploadSession, *domain.AppError)
}

type taskCreateReferenceUploadService struct {
	uploadRequestRepo   repo.UploadRequestRepo
	assetStorageRefRepo repo.AssetStorageRefRepo
	txRunner            repo.TxRunner
	uploadClient        UploadServiceClient
	ossDirectService    *OSSDirectService
	ossDirectUploadFn   func(ctx context.Context, objectKey, contentType string, body []byte) error
	nowFn               func() time.Time
	sleepFn             func(time.Duration)
	probeRetryMax       int
	probeRetryBackoff   time.Duration
}

type uploadServiceBaseURLProvider interface {
	UploadServiceBaseURL() string
}

type uploadRequestForUpdateReader interface {
	GetByRequestIDForUpdate(ctx context.Context, tx repo.Tx, requestID string) (*domain.UploadRequest, error)
}

func (s *taskCreateReferenceUploadService) getUploadRequestForUpdate(ctx context.Context, tx repo.Tx, requestID string) (*domain.UploadRequest, error) {
	if locker, ok := s.uploadRequestRepo.(uploadRequestForUpdateReader); ok {
		return locker.GetByRequestIDForUpdate(ctx, tx, requestID)
	}
	return s.uploadRequestRepo.GetByRequestID(ctx, requestID)
}

func NewTaskCreateReferenceUploadService(
	uploadRequestRepo repo.UploadRequestRepo,
	assetStorageRefRepo repo.AssetStorageRefRepo,
	txRunner repo.TxRunner,
	uploadClient UploadServiceClient,
	options ...func(*taskCreateReferenceUploadService),
) TaskCreateReferenceUploadService {
	svc := &taskCreateReferenceUploadService{
		uploadRequestRepo:   uploadRequestRepo,
		assetStorageRefRepo: assetStorageRefRepo,
		txRunner:            txRunner,
		uploadClient:        uploadClient,
		nowFn:               time.Now,
		sleepFn:             time.Sleep,
		probeRetryMax:       3,
		probeRetryBackoff:   200 * time.Millisecond,
	}
	for _, option := range options {
		option(svc)
	}
	return svc
}

func WithTaskCreateReferenceOSSDirectService(ossDirect *OSSDirectService) func(*taskCreateReferenceUploadService) {
	return func(s *taskCreateReferenceUploadService) {
		s.ossDirectService = ossDirect
	}
}

func (s *taskCreateReferenceUploadService) CreateUploadSession(ctx context.Context, params CreateTaskReferenceUploadSessionParams) (*CreateTaskReferenceUploadSessionResult, *domain.AppError) {
	if params.CreatedBy <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "created_by must be greater than zero", nil)
	}
	if strings.TrimSpace(params.Filename) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "filename is required", nil)
	}
	if params.ExpectedSize == nil || *params.ExpectedSize <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "expected_size must be greater than zero", nil)
	}
	if *params.ExpectedSize > TaskCreateReferenceUploadMaxFileSizeBytes {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "expected_size exceeds upload limit", map[string]interface{}{
			"max_bytes": TaskCreateReferenceUploadMaxFileSizeBytes,
			"max_label": TaskCreateReferenceUploadMaxFileSizeLabel,
		})
	}
	ownerType := params.OwnerType
	if ownerType == "" {
		ownerType = domain.AssetOwnerTypeTaskCreateReference
	}
	if !isPreCreateUploadOwner(ownerType) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "unsupported pre-create upload owner", nil)
	}
	if ownerType == domain.AssetOwnerTypePlanningSKUCreate && (strings.TrimSpace(params.ClientCreateID) == "" || strings.TrimSpace(params.ClientItemID) == "") {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "client_create_id and client_item_id are required for planning SKU images", nil)
	}
	namespace := preCreateUploadNamespace(ownerType)

	referenceType := domain.TaskAssetTypeReference
	now := s.nowFn().UTC()
	requestID := uuid.NewString()
	contentType := normalizeRequiredUploadContentType(params.MimeType)
	var remote *RemoteUploadSessionPlan
	var ossPlan *OSSDirectUploadPlan
	uploadMode := domain.DesignAssetUploadModeSmall
	var expiresAt *time.Time
	if s.ossDirectService != nil && s.ossDirectService.Enabled() {
		fileSize := *params.ExpectedSize
		objectKey := s.ossDirectService.BuildUploadSessionObjectKey(namespace, requestID, params.Filename)
		plan, err := s.ossDirectService.CreateUploadPlan(ctx, objectKey, fileSize, contentType)
		if err != nil {
			log.Printf("task_reference_oss_direct_plan_fallback request_id=%s error=%v", requestID, err)
		} else {
			ossPlan = plan
			expiresAt = &plan.ExpiresAt
			if plan.Mode == "multipart" {
				uploadMode = domain.DesignAssetUploadModeMultipart
			}
		}
	}
	if ossPlan == nil {
		var err error
		remote, err = s.uploadClient.CreateUploadSession(ctx, RemoteCreateUploadSessionRequest{
			TaskRef:      namespace,
			AssetNo:      map[bool]string{true: "PRECREATE-PLANNING-SKU", false: "PRECREATE-REFERENCE"}[ownerType == domain.AssetOwnerTypePlanningSKUCreate],
			AssetType:    domain.TaskAssetTypeReference,
			VersionNo:    1,
			UploadMode:   uploadMode,
			Filename:     strings.TrimSpace(params.Filename),
			ExpectedSize: params.ExpectedSize,
			MimeType:     contentType,
			CreatedBy:    params.CreatedBy,
		})
		if err != nil {
			return nil, infraError("create task-create reference upload session via upload service client", err)
		}
		expiresAt = remote.ExpiresAt
	}
	remoteUploadID := ""
	remoteFileID := ""
	isPlaceholder := false
	var lastSyncedAt *time.Time = &now
	if remote != nil {
		remoteUploadID = remote.UploadID
		remoteFileID = valueOrEmpty(remote.FileID)
		isPlaceholder = remote.IsStub
		lastSyncedAt = firstNonNilTime(remote.LastSyncedAt, &now)
	}
	request := &domain.UploadRequest{
		RequestID:       requestID,
		OwnerType:       ownerType,
		OwnerID:         params.CreatedBy,
		ClientCreateID:  strings.TrimSpace(params.ClientCreateID),
		ClientItemID:    strings.TrimSpace(params.ClientItemID),
		TaskAssetType:   &referenceType,
		StorageAdapter:  domain.AssetStorageAdapterOSSUploadService,
		UploadMode:      uploadMode,
		RefType:         domain.AssetStorageRefTypeGenericObject,
		FileName:        strings.TrimSpace(params.Filename),
		MimeType:        contentType,
		FileSize:        params.ExpectedSize,
		ExpectedSize:    params.ExpectedSize,
		ChecksumHint:    strings.TrimSpace(params.FileHash),
		Status:          domain.UploadRequestStatusRequested,
		StorageProvider: domain.DesignAssetStorageProviderOSS,
		SessionStatus:   domain.DesignAssetSessionStatusCreated,
		RemoteUploadID:  remoteUploadID,
		RemoteFileID:    remoteFileID,
		IsPlaceholder:   isPlaceholder,
		CreatedBy:       params.CreatedBy,
		ExpiresAt:       expiresAt,
		LastSyncedAt:    lastSyncedAt,
		Remark:          strings.TrimSpace(params.Remark),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		created, createErr := s.uploadRequestRepo.Create(ctx, tx, request)
		if createErr != nil {
			return createErr
		}
		request = created
		return nil
	}); err != nil {
		return nil, infraError("create task-create reference upload session", err)
	}

	baseEndpoint := "/v1/tasks/reference-upload-sessions/"
	if ownerType == domain.AssetOwnerTypePlanningSKUCreate {
		baseEndpoint = "/v1/tasks/sku-planning/image-upload-sessions/"
	}
	return &CreateTaskReferenceUploadSessionResult{
		Session:          domain.BuildUploadSession(request),
		Remote:           remote,
		OSSDirect:        ossPlan,
		CompleteEndpoint: baseEndpoint + request.RequestID + "/complete",
		CancelEndpoint:   baseEndpoint + request.RequestID + "/abort",
	}, nil
}

func (s *taskCreateReferenceUploadService) GetUploadSession(ctx context.Context, sessionID string, requesterID int64) (*domain.UploadSession, *domain.AppError) {
	request, appErr := s.requireUploadRequest(ctx, sessionID, requesterID)
	if appErr != nil {
		return nil, appErr
	}
	if request.RemoteUploadID != "" && request.SessionStatus == domain.DesignAssetSessionStatusCreated {
		var err error
		if request, err = s.syncUploadRequestFromRemote(ctx, request); err != nil {
			return nil, infraError("sync task-create reference upload session from upload service", err)
		}
	}
	return domain.BuildUploadSession(request), nil
}

func (s *taskCreateReferenceUploadService) UploadFile(ctx context.Context, params UploadTaskReferenceFileParams) (*domain.ReferenceFileRef, *domain.AppError) {
	if params.File == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file is required", nil)
	}
	if params.ExpectedSize != nil && *params.ExpectedSize > TaskCreateReferenceUploadMaxFileSizeBytes {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "expected_size exceeds upload limit", map[string]interface{}{
			"max_bytes": TaskCreateReferenceUploadMaxFileSizeBytes,
			"max_label": TaskCreateReferenceUploadMaxFileSizeLabel,
		})
	}
	fileBytes, err := io.ReadAll(io.LimitReader(params.File, TaskCreateReferenceUploadMaxFileSizeBytes+1))
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "read uploaded file bytes", nil)
	}
	if len(fileBytes) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "uploaded file bytes are empty", nil)
	}
	actualSize := int64(len(fileBytes))
	if actualSize > TaskCreateReferenceUploadMaxFileSizeBytes {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "uploaded file exceeds upload limit", map[string]interface{}{
			"max_bytes": TaskCreateReferenceUploadMaxFileSizeBytes,
			"max_label": TaskCreateReferenceUploadMaxFileSizeLabel,
		})
	}
	if params.ExpectedSize != nil && *params.ExpectedSize != actualSize {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "uploaded file size does not match multipart header size", map[string]interface{}{
			"expected_size": *params.ExpectedSize,
			"actual_size":   actualSize,
		})
	}
	sum := sha256.Sum256(fileBytes)
	localSHA256 := hex.EncodeToString(sum[:])
	if providedSHA := strings.TrimSpace(params.FileHash); providedSHA != "" && !strings.EqualFold(providedSHA, localSHA256) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "uploaded file hash does not match provided file_hash", map[string]interface{}{
			"provided_file_hash": providedSHA,
			"actual_file_hash":   localSHA256,
		})
	}
	params.ExpectedSize = &actualSize
	params.FileHash = localSHA256
	logUploadProbe("task_reference_upload_input", map[string]interface{}{
		"trace_id":     domain.TraceIDFromContext(ctx),
		"filename":     strings.TrimSpace(params.Filename),
		"mime_type":    strings.TrimSpace(params.MimeType),
		"file_bytes":   actualSize,
		"file_sha256":  localSHA256,
		"file_read_ok": true,
		"created_by":   params.CreatedBy,
	})
	if s.ossDirectService != nil && s.ossDirectService.Enabled() {
		return s.uploadFileViaOSSDirect(ctx, params, fileBytes)
	}
	createResult, appErr := s.CreateUploadSession(ctx, CreateTaskReferenceUploadSessionParams{
		CreatedBy:    params.CreatedBy,
		Filename:     params.Filename,
		ExpectedSize: params.ExpectedSize,
		MimeType:     params.MimeType,
		FileHash:     params.FileHash,
		Remark:       params.Remark,
	})
	if appErr != nil {
		return nil, appErr
	}
	if createResult == nil || createResult.Session == nil || createResult.Remote == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "reference upload session creation returned incomplete result", nil)
	}
	uploadMeta, err := s.uploadClient.UploadFileToSession(ctx, RemoteSessionFileUploadRequest{
		UploadURL:      strings.TrimSpace(createResult.Remote.UploadURL),
		Method:         strings.TrimSpace(createResult.Remote.Method),
		Headers:        createResult.Remote.Headers,
		RemoteUploadID: strings.TrimSpace(createResult.Session.RemoteUploadID),
		TaskRef:        strings.TrimSpace(createResult.Remote.TaskRef),
		AssetNo:        strings.TrimSpace(createResult.Remote.AssetNo),
		AssetType:      domain.TaskAssetTypeReference,
		VersionNo:      createResult.Remote.VersionNo,
		Filename:       strings.TrimSpace(createResult.Session.Filename),
		MimeType:       strings.TrimSpace(createResult.Session.MimeType),
		ExpectedSize:   createResult.Session.ExpectedSize,
		CreatedBy:      params.CreatedBy,
		File:           bytes.NewReader(fileBytes),
		FileFieldName:  "file",
	})
	if err != nil {
		return nil, infraError("upload task reference file via upload service client", err)
	}
	if createResult.Session.UploadMode == domain.DesignAssetUploadModeSmall {
		request, appErr := s.requireUploadRequest(ctx, createResult.Session.ID, params.CreatedBy)
		if appErr != nil {
			return nil, appErr
		}
		if uploadMeta == nil || (strings.TrimSpace(metaStorageKey(uploadMeta)) == "" && strings.TrimSpace(valueOrEmpty(metaFileID(uploadMeta))) == "") {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "reference upload small session returned incomplete file metadata", map[string]interface{}{
				"upload_request_id": request.RequestID,
				"remote_upload_id":  request.RemoteUploadID,
			})
		}
		logUploadProbe("task_reference_upload_small_upload_result", map[string]interface{}{
			"trace_id":          domain.TraceIDFromContext(ctx),
			"upload_request_id": request.RequestID,
			"remote_upload_id":  request.RemoteUploadID,
			"remote_file_id":    valueOrEmpty(metaFileID(uploadMeta)),
			"storage_key":       metaStorageKey(uploadMeta),
			"remote_file_size":  int64ValueFromPtr(metaFileSize(uploadMeta)),
		})
		completeResult, appErr := s.finalizeUploadedReference(ctx, request, params.CreatedBy, params.Remark, params.FileHash, localSHA256, uploadMeta)
		if appErr != nil {
			return nil, appErr
		}
		if completeResult.RefObject == nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "reference upload completed without ref object", nil)
		}
		refObject := *completeResult.RefObject
		refObject.Source = domain.ReferenceFileRefSourceTaskReferenceUpload
		refObject.Normalize()
		logUploadProbe("task_reference_upload_ref_object", map[string]interface{}{
			"trace_id":          domain.TraceIDFromContext(ctx),
			"asset_id":          refObject.AssetID,
			"upload_request_id": refObject.UploadRequestID,
			"filename":          refObject.Filename,
			"file_size":         refObject.FileSize,
			"download_url":      valueOrEmpty(refObject.DownloadURL),
		})
		return &refObject, nil
	}
	completeResult, appErr := s.CompleteUploadSession(ctx, CompleteTaskReferenceUploadSessionParams{
		SessionID:      createResult.Session.ID,
		CompletedBy:    params.CreatedBy,
		Remark:         params.Remark,
		FileHash:       params.FileHash,
		ExpectedSHA256: localSHA256,
	})
	if appErr != nil {
		return nil, appErr
	}
	if completeResult.RefObject == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "reference upload completed without ref object", nil)
	}
	refObject := *completeResult.RefObject
	refObject.Source = domain.ReferenceFileRefSourceTaskReferenceUpload
	refObject.Normalize()
	logUploadProbe("task_reference_upload_ref_object", map[string]interface{}{
		"trace_id":          domain.TraceIDFromContext(ctx),
		"asset_id":          refObject.AssetID,
		"upload_request_id": refObject.UploadRequestID,
		"filename":          refObject.Filename,
		"file_size":         refObject.FileSize,
		"download_url":      valueOrEmpty(refObject.DownloadURL),
	})
	return &refObject, nil
}

func (s *taskCreateReferenceUploadService) uploadFileViaOSSDirect(ctx context.Context, params UploadTaskReferenceFileParams, fileBytes []byte) (*domain.ReferenceFileRef, *domain.AppError) {
	objectKey := s.ossDirectService.BuildObjectKey(
		"task-create-reference",
		"PRECREATE-REFERENCE",
		1,
		domain.TaskAssetTypeReference,
		strings.TrimSpace(params.Filename),
	)
	contentType := normalizeRequiredUploadContentType(params.MimeType)
	uploadFn := s.ossDirectUploadFn
	if uploadFn == nil {
		uploadFn = s.ossDirectService.UploadObject
	}
	if err := uploadFn(ctx, objectKey, contentType, fileBytes); err != nil {
		return nil, infraError("upload task reference file via oss direct", err)
	}
	refObject, appErr := s.persistDirectReferenceUpload(ctx, params, objectKey, contentType)
	if appErr != nil {
		return nil, appErr
	}
	logUploadProbe("task_reference_upload_ref_object", map[string]interface{}{
		"trace_id":          domain.TraceIDFromContext(ctx),
		"asset_id":          refObject.AssetID,
		"upload_request_id": refObject.UploadRequestID,
		"filename":          refObject.Filename,
		"file_size":         refObject.FileSize,
		"download_url":      valueOrEmpty(refObject.DownloadURL),
		"upload_mode":       "oss_direct_backend_proxy",
	})
	return refObject, nil
}

func (s *taskCreateReferenceUploadService) persistDirectReferenceUpload(ctx context.Context, params UploadTaskReferenceFileParams, objectKey, contentType string) (*domain.ReferenceFileRef, *domain.AppError) {
	now := s.nowFn().UTC()
	referenceType := domain.TaskAssetTypeReference
	request := &domain.UploadRequest{
		OwnerType:       domain.AssetOwnerTypeTaskCreateReference,
		OwnerID:         params.CreatedBy,
		TaskAssetType:   &referenceType,
		StorageAdapter:  domain.AssetStorageAdapterOSSUploadService,
		UploadMode:      domain.DesignAssetUploadModeSmall,
		RefType:         domain.AssetStorageRefTypeGenericObject,
		FileName:        strings.TrimSpace(params.Filename),
		MimeType:        contentType,
		FileSize:        params.ExpectedSize,
		ExpectedSize:    params.ExpectedSize,
		ChecksumHint:    strings.TrimSpace(params.FileHash),
		Status:          domain.UploadRequestStatusRequested,
		StorageProvider: domain.DesignAssetStorageProviderOSS,
		SessionStatus:   domain.DesignAssetSessionStatusCreated,
		CreatedBy:       params.CreatedBy,
		LastSyncedAt:    &now,
		Remark:          strings.TrimSpace(params.Remark),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	var createdRef *domain.AssetStorageRef
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		createdRequest, createErr := s.uploadRequestRepo.Create(ctx, tx, request)
		if createErr != nil {
			return createErr
		}
		request = createdRequest
		createdRef, createErr = s.assetStorageRefRepo.Create(ctx, tx, &domain.AssetStorageRef{
			RefID:           uuid.NewString(),
			OwnerType:       domain.AssetOwnerTypeTaskCreateReference,
			OwnerID:         params.CreatedBy,
			UploadRequestID: request.RequestID,
			StorageAdapter:  domain.AssetStorageAdapterOSSUploadService,
			RefType:         domain.AssetStorageRefTypeGenericObject,
			RefKey:          strings.TrimSpace(objectKey),
			FileName:        strings.TrimSpace(params.Filename),
			MimeType:        contentType,
			FileSize:        params.ExpectedSize,
			ChecksumHint:    strings.TrimSpace(params.FileHash),
			Status:          domain.AssetStorageRefStatusRecorded,
			CreatedAt:       now,
		})
		if createErr != nil {
			return createErr
		}
		if err := s.uploadRequestRepo.UpdateBinding(ctx, tx, request.RequestID, nil, createdRef.RefID, domain.UploadRequestStatusBound, strings.TrimSpace(params.Remark)); err != nil {
			return err
		}
		return s.uploadRequestRepo.UpdateSession(ctx, tx, repo.UploadRequestSessionUpdate{
			RequestID:      request.RequestID,
			SessionStatus:  domain.DesignAssetSessionStatusCompleted,
			RemoteUploadID: request.RemoteUploadID,
			RemoteFileID:   optionalStringPtr(request.RemoteFileID),
			LastSyncedAt:   &now,
			Remark:         strings.TrimSpace(params.Remark),
		})
	}); err != nil {
		return nil, infraError("record task-create reference upload via oss direct", err)
	}
	refObject := s.buildReferenceFileRef(createdRef, domain.ReferenceFileRefSourceTaskReferenceUpload)
	if refObject == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "reference upload completed without ref object", nil)
	}
	return refObject, nil
}

func (s *taskCreateReferenceUploadService) CompleteUploadSession(ctx context.Context, params CompleteTaskReferenceUploadSessionParams) (*CompleteTaskReferenceUploadSessionResult, *domain.AppError) {
	request, appErr := s.requireUploadRequest(ctx, params.SessionID, params.CompletedBy)
	if appErr != nil {
		return nil, appErr
	}
	if params.OwnerType != "" && request.OwnerType != params.OwnerType {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session belongs to another upload workflow", nil)
	}
	if request.Status == domain.UploadRequestStatusBound ||
		(request.SessionStatus == domain.DesignAssetSessionStatusCompleted && strings.TrimSpace(request.BoundRefID) != "") {
		if strings.TrimSpace(request.BoundRefID) == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload_session already completed without bound reference_file_ref", nil)
		}
		session, getErr := s.GetUploadSession(ctx, params.SessionID, params.CompletedBy)
		if getErr != nil {
			return nil, getErr
		}
		storageRef, err := s.assetStorageRefRepo.GetByRefID(ctx, request.BoundRefID)
		if err != nil {
			return nil, infraError("get completed task-create reference_file_ref", err)
		}
		return &CompleteTaskReferenceUploadSessionResult{
			Session:          session,
			ReferenceFileRef: request.BoundRefID,
			StorageRef:       storageRef,
			RefObject:        s.buildReferenceFileRef(storageRef, domain.ReferenceFileRefSourceTaskCreateAssetCenter),
		}, nil
	}
	if request.SessionStatus == domain.DesignAssetSessionStatusCancelled || request.SessionStatus == domain.DesignAssetSessionStatusExpired {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload_session is already terminal", nil)
	}
	hasOSSCompletion := strings.TrimSpace(params.OSSObjectKey) != "" || strings.TrimSpace(params.OSSUploadID) != "" || len(params.OSSParts) > 0
	isDirectSession := strings.TrimSpace(request.RemoteUploadID) == "" && s.ossDirectService != nil && s.ossDirectService.Enabled()
	if isDirectSession && strings.TrimSpace(params.OSSObjectKey) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "oss_object_key is required for OSS direct upload_session completion", nil)
	}
	if isDirectSession || hasOSSCompletion {
		return s.completeOSSDirectReferenceUpload(ctx, request, params)
	}

	checksumHint := firstNonEmpty(strings.TrimSpace(params.FileHash), strings.TrimSpace(request.ChecksumHint))
	if request.RemoteUploadID != "" && request.SessionStatus == domain.DesignAssetSessionStatusCreated {
		var err error
		if request, err = s.syncUploadRequestFromRemote(ctx, request); err != nil {
			return nil, infraError("sync task-create reference upload session before completion", err)
		}
	}

	var meta *RemoteFileMeta
	var err error
	if request.SessionStatus == domain.DesignAssetSessionStatusCompleted && strings.TrimSpace(request.RemoteFileID) != "" {
		meta, err = s.uploadClient.GetFileMeta(ctx, RemoteGetFileMetaRequest{
			RemoteUploadID: request.RemoteUploadID,
			RemoteFileID:   request.RemoteFileID,
			Filename:       request.FileName,
			ExpectedSize:   request.ExpectedSize,
			MimeType:       request.MimeType,
			ChecksumHint:   checksumHint,
		})
	} else {
		meta, err = s.uploadClient.CompleteUploadSession(ctx, RemoteCompleteUploadRequest{
			RemoteUploadID: request.RemoteUploadID,
			Filename:       request.FileName,
			ExpectedSize:   request.ExpectedSize,
			MimeType:       request.MimeType,
			ChecksumHint:   checksumHint,
		})
	}
	if err != nil {
		return nil, infraError("complete task-create reference upload session via upload service client", err)
	}
	if request.ExpectedSize != nil && meta.FileSize != nil && *meta.FileSize != *request.ExpectedSize {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "reference upload complete returned mismatched file_size", map[string]interface{}{
			"upload_request_id": request.RequestID,
			"remote_upload_id":  request.RemoteUploadID,
			"remote_file_id":    valueOrEmpty(meta.FileID),
			"expected_size":     *request.ExpectedSize,
			"remote_file_size":  *meta.FileSize,
		})
	}

	return s.finalizeUploadedReference(ctx, request, params.CompletedBy, params.Remark, checksumHint, params.ExpectedSHA256, meta)
}

func (s *taskCreateReferenceUploadService) completeOSSDirectReferenceUpload(
	ctx context.Context,
	request *domain.UploadRequest,
	params CompleteTaskReferenceUploadSessionParams,
) (*CompleteTaskReferenceUploadSessionResult, *domain.AppError) {
	if s.ossDirectService == nil || !s.ossDirectService.Enabled() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "OSS direct upload is not enabled", nil)
	}
	var createdRef *domain.AssetStorageRef
	var completedRefID string
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		locked, err := s.getUploadRequestForUpdate(ctx, tx, request.RequestID)
		if err != nil {
			return err
		}
		if locked == nil {
			return domain.ErrNotFound
		}
		request = locked
		if !isPreCreateUploadOwner(request.OwnerType) || request.OwnerID != params.CompletedBy {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session does not belong to current actor", nil)
		}
		if request.Status == domain.UploadRequestStatusBound || request.SessionStatus == domain.DesignAssetSessionStatusCompleted {
			completedRefID = strings.TrimSpace(request.BoundRefID)
			if completedRefID == "" {
				return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload_session already completed without bound reference_file_ref", nil)
			}
			return nil
		}
		if request.Status == domain.UploadRequestStatusCancelled || request.Status == domain.UploadRequestStatusExpired ||
			request.SessionStatus == domain.DesignAssetSessionStatusCancelled || request.SessionStatus == domain.DesignAssetSessionStatusExpired {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload_session is already terminal", nil)
		}
		if request.ExpectedSize == nil || *request.ExpectedSize <= 0 || *request.ExpectedSize > TaskCreateReferenceUploadMaxFileSizeBytes {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session expected_size is outside the allowed range", map[string]interface{}{
				"max_bytes": TaskCreateReferenceUploadMaxFileSizeBytes,
			})
		}
		expectedObjectKey := s.ossDirectService.BuildUploadSessionObjectKey(preCreateUploadNamespace(request.OwnerType), request.RequestID, request.FileName)
		objectKey := strings.TrimSpace(params.OSSObjectKey)
		if objectKey != expectedObjectKey {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "oss_object_key does not belong to upload_session", map[string]interface{}{
				"upload_session_id": request.RequestID,
			})
		}
		if contentType := strings.TrimSpace(params.UploadContentType); contentType != "" && contentType != normalizeRequiredUploadContentType(request.MimeType) {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_content_type must match upload_session required content type", nil)
		}
		hasUploadID := strings.TrimSpace(params.OSSUploadID) != ""
		hasParts := len(params.OSSParts) > 0
		if hasUploadID != hasParts {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "multipart OSS completion requires upload_id and parts together", nil)
		}
		if request.UploadMode == domain.DesignAssetUploadModeMultipart && !hasUploadID {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "multipart upload_session requires upload_id and parts", nil)
		}
		if request.UploadMode != domain.DesignAssetUploadModeMultipart && hasUploadID {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "single-part upload_session cannot complete multipart parts", nil)
		}
		var multipartCompleteErr error
		if hasUploadID {
			multipartCompleteErr = s.ossDirectService.CompleteMultipartUpload(ctx, objectKey, strings.TrimSpace(params.OSSUploadID), params.OSSParts)
		}
		info, exists, statErr := s.ossDirectService.StatObject(ctx, objectKey)
		if statErr != nil {
			return fmt.Errorf("verify task reference OSS object: %w", statErr)
		}
		if !exists || info == nil {
			if multipartCompleteErr != nil {
				return fmt.Errorf("complete task reference OSS multipart upload: %w", multipartCompleteErr)
			}
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "uploaded OSS object is missing", nil)
		}
		if info.ContentLength < 0 || info.ContentLength != *request.ExpectedSize {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "uploaded OSS object size does not match upload_session", map[string]interface{}{
				"expected_size": *request.ExpectedSize,
				"actual_size":   info.ContentLength,
			})
		}
		if multipartCompleteErr != nil {
			log.Printf("task_reference_oss_complete_recovered request_id=%s object_key=%s error=%v", request.RequestID, objectKey, multipartCompleteErr)
		}
		now := s.nowFn().UTC()
		fileSize := info.ContentLength
		contentType := normalizeRequiredUploadContentType(request.MimeType)
		if strings.TrimSpace(info.ContentType) != "" {
			contentType = strings.TrimSpace(info.ContentType)
		}
		checksumHint := firstNonEmpty(strings.TrimSpace(params.FileHash), strings.TrimSpace(request.ChecksumHint))
		ref, err := s.assetStorageRefRepo.Create(ctx, tx, &domain.AssetStorageRef{
			RefID:           uuid.NewString(),
			OwnerType:       request.OwnerType,
			OwnerID:         request.OwnerID,
			UploadRequestID: request.RequestID,
			StorageAdapter:  domain.AssetStorageAdapterOSSUploadService,
			RefType:         domain.AssetStorageRefTypeGenericObject,
			RefKey:          objectKey,
			FileName:        request.FileName,
			MimeType:        contentType,
			FileSize:        &fileSize,
			ChecksumHint:    checksumHint,
			Status:          domain.AssetStorageRefStatusRecorded,
			CreatedAt:       now,
		})
		if err != nil {
			return err
		}
		createdRef = ref
		completedRefID = ref.RefID
		remark := firstNonEmpty(strings.TrimSpace(params.Remark), strings.TrimSpace(request.Remark))
		if err := s.uploadRequestRepo.UpdateBinding(ctx, tx, request.RequestID, nil, ref.RefID, domain.UploadRequestStatusBound, remark); err != nil {
			return err
		}
		return s.uploadRequestRepo.UpdateSession(ctx, tx, repo.UploadRequestSessionUpdate{
			RequestID:      request.RequestID,
			SessionStatus:  domain.DesignAssetSessionStatusCompleted,
			RemoteUploadID: request.RemoteUploadID,
			RemoteFileID:   optionalStringPtr(request.RemoteFileID),
			LastSyncedAt:   &now,
			Remark:         remark,
		})
	}); err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			return nil, appErr
		}
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, infraError("record completed task reference OSS direct upload", err)
	}
	session, appErr := s.GetUploadSession(ctx, request.RequestID, params.CompletedBy)
	if appErr != nil {
		return nil, appErr
	}
	if createdRef == nil {
		var err error
		createdRef, err = s.assetStorageRefRepo.GetByRefID(ctx, completedRefID)
		if err != nil {
			return nil, infraError("get completed task reference_file_ref", err)
		}
		if createdRef == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "completed reference_file_ref is missing", nil)
		}
	}
	return &CompleteTaskReferenceUploadSessionResult{
		Session:          session,
		ReferenceFileRef: completedRefID,
		StorageRef:       createdRef,
		RefObject:        s.buildReferenceFileRef(createdRef, domain.ReferenceFileRefSourceTaskReferenceUpload),
	}, nil
}

func (s *taskCreateReferenceUploadService) CancelUploadSession(ctx context.Context, params CancelTaskReferenceUploadSessionParams) (*domain.UploadSession, *domain.AppError) {
	request, appErr := s.requireUploadRequest(ctx, params.SessionID, params.CancelledBy)
	if appErr != nil {
		return nil, appErr
	}
	if params.OwnerType != "" && request.OwnerType != params.OwnerType {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session belongs to another upload workflow", nil)
	}
	alreadyCancelled := false
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		locked, err := s.getUploadRequestForUpdate(ctx, tx, request.RequestID)
		if err != nil {
			return err
		}
		if locked == nil {
			return domain.ErrNotFound
		}
		request = locked
		if !isPreCreateUploadOwner(request.OwnerType) || request.OwnerID != params.CancelledBy {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session does not belong to current actor", nil)
		}
		if request.SessionStatus == domain.DesignAssetSessionStatusCompleted || request.Status == domain.UploadRequestStatusBound {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "completed upload_session cannot be aborted", nil)
		}
		if request.SessionStatus == domain.DesignAssetSessionStatusCancelled || request.Status == domain.UploadRequestStatusCancelled {
			alreadyCancelled = true
			return nil
		}
		if request.SessionStatus == domain.DesignAssetSessionStatusExpired || request.Status == domain.UploadRequestStatusExpired {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "expired upload_session cannot be aborted", nil)
		}
		if strings.TrimSpace(request.RemoteUploadID) != "" {
			if err := s.uploadClient.AbortUploadSession(ctx, RemoteAbortUploadRequest{RemoteUploadID: request.RemoteUploadID}); err != nil {
				return fmt.Errorf("abort task-create reference upload session via upload service client: %w", err)
			}
		} else if s.ossDirectService != nil && s.ossDirectService.Enabled() {
			expectedObjectKey := s.ossDirectService.BuildUploadSessionObjectKey(preCreateUploadNamespace(request.OwnerType), request.RequestID, request.FileName)
			objectKey := strings.TrimSpace(params.OSSObjectKey)
			if objectKey == "" {
				objectKey = expectedObjectKey
			}
			if objectKey != expectedObjectKey {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "oss_object_key does not belong to upload_session", nil)
			}
			uploadID := strings.TrimSpace(params.OSSUploadID)
			if request.UploadMode == domain.DesignAssetUploadModeMultipart && uploadID != "" {
				if err := s.ossDirectService.AbortMultipartUpload(ctx, objectKey, uploadID); err != nil {
					return fmt.Errorf("abort task reference OSS multipart upload: %w", err)
				}
			} else if request.UploadMode != domain.DesignAssetUploadModeMultipart {
				if err := s.ossDirectService.DeleteObject(ctx, objectKey); err != nil {
					return fmt.Errorf("delete task reference OSS single upload: %w", err)
				}
			} else {
				log.Printf("task_reference_oss_direct_cancel_missing_upload_id request_id=%s", request.RequestID)
			}
		}

		lastSyncedAt := s.nowFn().UTC()
		if err := s.uploadRequestRepo.UpdateLifecycle(ctx, tx, repo.UploadRequestLifecycleUpdate{
			RequestID: request.RequestID,
			Status:    domain.UploadRequestStatusCancelled,
			Remark:    strings.TrimSpace(params.Remark),
		}); err != nil {
			return err
		}
		return s.uploadRequestRepo.UpdateSession(ctx, tx, repo.UploadRequestSessionUpdate{
			RequestID:      request.RequestID,
			SessionStatus:  domain.DesignAssetSessionStatusCancelled,
			RemoteUploadID: request.RemoteUploadID,
			RemoteFileID:   optionalStringPtr(request.RemoteFileID),
			LastSyncedAt:   &lastSyncedAt,
			Remark:         strings.TrimSpace(params.Remark),
		})
	}); err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			return nil, appErr
		}
		return nil, infraError("cancel task-create reference upload session", err)
	}
	if alreadyCancelled {
		return domain.BuildUploadSession(request), nil
	}

	return s.GetUploadSession(ctx, request.RequestID, params.CancelledBy)
}

func (s *taskCreateReferenceUploadService) requireUploadRequest(ctx context.Context, sessionID string, requesterID int64) (*domain.UploadRequest, *domain.AppError) {
	if requesterID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "requester_id must be greater than zero", nil)
	}
	request, err := s.uploadRequestRepo.GetByRequestID(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, infraError("get task-create reference upload request", err)
	}
	if request == nil {
		return nil, domain.ErrNotFound
	}
	if !isPreCreateUploadOwner(request.OwnerType) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session does not belong to task-create reference uploads", nil)
	}
	if request.OwnerID != requesterID {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session does not belong to current actor", nil)
	}
	if request.TaskAssetType == nil || request.TaskAssetType.Canonical() != domain.TaskAssetTypeReference {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session is not a reference upload", nil)
	}
	return request, nil
}

func (s *taskCreateReferenceUploadService) syncUploadRequestFromRemote(ctx context.Context, request *domain.UploadRequest) (*domain.UploadRequest, error) {
	remote, err := s.uploadClient.GetUploadSession(ctx, RemoteGetUploadSessionRequest{RemoteUploadID: request.RemoteUploadID})
	if err != nil {
		return nil, err
	}
	currentSyncedAt := s.nowFn().UTC()
	lastSyncedAt := firstNonNilTime(remote.LastSyncedAt, &currentSyncedAt)
	sessionStatus := request.SessionStatus
	if remote.SessionStatus.Valid() {
		sessionStatus = remote.SessionStatus
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.uploadRequestRepo.UpdateSession(ctx, tx, repo.UploadRequestSessionUpdate{
			RequestID:      request.RequestID,
			SessionStatus:  sessionStatus,
			RemoteUploadID: firstNonEmpty(remote.UploadID, request.RemoteUploadID),
			RemoteFileID:   remote.FileID,
			ExpiresAt:      remote.ExpiresAt,
			LastSyncedAt:   lastSyncedAt,
			Remark:         request.Remark,
		})
	}); err != nil {
		return nil, err
	}
	return s.uploadRequestRepo.GetByRequestID(ctx, request.RequestID)
}

func (s *taskCreateReferenceUploadService) buildReferenceFileRef(storageRef *domain.AssetStorageRef, source string) *domain.ReferenceFileRef {
	if storageRef == nil {
		return nil
	}
	ref := &domain.ReferenceFileRef{
		AssetID:         strings.TrimSpace(storageRef.RefID),
		RefID:           strings.TrimSpace(storageRef.RefID),
		UploadRequestID: strings.TrimSpace(storageRef.UploadRequestID),
		Filename:        strings.TrimSpace(storageRef.FileName),
		MimeType:        strings.TrimSpace(storageRef.MimeType),
		FileSize:        storageRef.FileSize,
		Source:          strings.TrimSpace(source),
		Status:          domain.ReferenceFileRefStatusUploaded,
	}
	if storageKey := strings.TrimSpace(storageRef.RefKey); storageKey != "" {
		ref.StorageKey = storageKey
		downloadURL := AppendProxyDownloadFilenameQuery(
			domain.BuildRelativeEscapedURLPath("/v1/assets/files", storageKey),
			storageRef.FileName,
		)
		ref.DownloadURL = &downloadURL
		ref.URL = &downloadURL
	}
	ref.Normalize()
	return ref
}

func (s *taskCreateReferenceUploadService) finalizeUploadedReference(ctx context.Context, request *domain.UploadRequest, completedBy int64, remark, checksumHint, expectedSHA256 string, meta *RemoteFileMeta) (*CompleteTaskReferenceUploadSessionResult, *domain.AppError) {
	now := s.nowFn().UTC()
	lastSyncedAt := now
	resolvedStorageKey := buildRemoteStorageKey(meta, request)
	probeBaseURL := s.uploadServiceBaseURL()
	selectedProbeHost := probeHostFromBaseURL(probeBaseURL)
	expectedSize := int64ValueFromPtr(request.ExpectedSize)
	expectedSHA := strings.TrimSpace(expectedSHA256)
	if strings.TrimSpace(resolvedStorageKey) == "" {
		logUploadProbe("task_reference_upload_probe_prepare", map[string]interface{}{
			"trace_id":                domain.TraceIDFromContext(ctx),
			"upload_mode":             "reference_small",
			"upload_service_base_url": probeBaseURL,
			"selected_probe_host":     selectedProbeHost,
			"filename":                strings.TrimSpace(request.FileName),
			"storage_key":             truncateLogString(resolvedStorageKey, 192),
			"expected_size":           expectedSize,
			"expected_sha256":         expectedSHA,
			"probe_status":            "skipped_empty_storage_key",
		})
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "reference upload complete returned empty storage_key", map[string]interface{}{
			"upload_request_id": request.RequestID,
			"remote_upload_id":  request.RemoteUploadID,
			"remote_file_id":    valueOrEmpty(metaFileID(meta)),
		})
	}
	logUploadProbe("task_reference_upload_probe_prepare", map[string]interface{}{
		"trace_id":                domain.TraceIDFromContext(ctx),
		"upload_mode":             "reference_small",
		"upload_service_base_url": probeBaseURL,
		"selected_probe_host":     selectedProbeHost,
		"filename":                strings.TrimSpace(request.FileName),
		"storage_key":             truncateLogString(resolvedStorageKey, 192),
		"expected_size":           expectedSize,
		"expected_sha256":         expectedSHA,
	})
	probe, err := s.probeStoredReferenceWithRetry(ctx, request, resolvedStorageKey, expectedSHA)
	if err != nil {
		return nil, infraError("probe task-create reference stored file", err)
	}
	if request.ExpectedSize != nil && probe.BytesRead != *request.ExpectedSize {
		logUploadProbe("task_reference_upload_probe_mismatch", map[string]interface{}{
			"trace_id":                domain.TraceIDFromContext(ctx),
			"upload_mode":             "reference_small",
			"upload_service_base_url": probeBaseURL,
			"selected_probe_host":     selectedProbeHost,
			"filename":                strings.TrimSpace(request.FileName),
			"storage_key":             truncateLogString(resolvedStorageKey, 192),
			"expected_size":           expectedSize,
			"expected_sha256":         expectedSHA,
			"probe_status":            probe.StatusCode,
			"stored_bytes_read":       probe.BytesRead,
			"stored_content_length":   probe.ContentLengthHeader,
			"stored_sha256":           probe.SHA256,
			"mismatch_reason":         "stored_size_mismatch",
		})
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "reference upload stored file verification failed", map[string]interface{}{
			"upload_request_id":     request.RequestID,
			"remote_upload_id":      request.RemoteUploadID,
			"remote_file_id":        valueOrEmpty(metaFileID(meta)),
			"storage_key":           resolvedStorageKey,
			"expected_size":         *request.ExpectedSize,
			"stored_bytes_read":     probe.BytesRead,
			"stored_content_length": probe.ContentLengthHeader,
			"stored_sha256":         probe.SHA256,
		})
	}
	if expectedSHA != "" && !strings.EqualFold(probe.SHA256, expectedSHA) {
		logUploadProbe("task_reference_upload_probe_mismatch", map[string]interface{}{
			"trace_id":                domain.TraceIDFromContext(ctx),
			"upload_mode":             "reference_small",
			"upload_service_base_url": probeBaseURL,
			"selected_probe_host":     selectedProbeHost,
			"filename":                strings.TrimSpace(request.FileName),
			"storage_key":             truncateLogString(resolvedStorageKey, 192),
			"expected_size":           expectedSize,
			"expected_sha256":         expectedSHA,
			"probe_status":            probe.StatusCode,
			"stored_bytes_read":       probe.BytesRead,
			"stored_content_length":   probe.ContentLengthHeader,
			"stored_sha256":           probe.SHA256,
			"mismatch_reason":         "stored_hash_mismatch",
		})
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "reference upload stored file hash verification failed", map[string]interface{}{
			"upload_request_id": request.RequestID,
			"remote_upload_id":  request.RemoteUploadID,
			"remote_file_id":    valueOrEmpty(metaFileID(meta)),
			"storage_key":       resolvedStorageKey,
			"expected_sha256":   expectedSHA,
			"stored_sha256":     probe.SHA256,
			"stored_bytes_read": probe.BytesRead,
		})
	}
	logUploadProbe("task_reference_upload_complete_verified", map[string]interface{}{
		"trace_id":                domain.TraceIDFromContext(ctx),
		"upload_mode":             "reference_small",
		"upload_service_base_url": probeBaseURL,
		"selected_probe_host":     selectedProbeHost,
		"upload_request_id":       request.RequestID,
		"remote_upload_id":        request.RemoteUploadID,
		"remote_file_id":          valueOrEmpty(metaFileID(meta)),
		"filename":                strings.TrimSpace(request.FileName),
		"storage_key":             truncateLogString(resolvedStorageKey, 192),
		"expected_size":           expectedSize,
		"expected_sha256":         expectedSHA,
		"remote_file_size":        int64ValueFromPtr(metaFileSize(meta)),
		"stored_content_length":   probe.ContentLengthHeader,
		"stored_bytes_read":       probe.BytesRead,
		"stored_sha256":           probe.SHA256,
		"probe_status":            probe.StatusCode,
	})
	var createdRef *domain.AssetStorageRef
	var completedRefID string
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		locked, lockErr := s.getUploadRequestForUpdate(ctx, tx, request.RequestID)
		if lockErr != nil {
			return lockErr
		}
		if locked == nil {
			return domain.ErrNotFound
		}
		request = locked
		if !isPreCreateUploadOwner(request.OwnerType) || request.OwnerID != completedBy {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session does not belong to current actor", nil)
		}
		if request.Status == domain.UploadRequestStatusBound ||
			(request.SessionStatus == domain.DesignAssetSessionStatusCompleted && strings.TrimSpace(request.BoundRefID) != "") {
			completedRefID = strings.TrimSpace(request.BoundRefID)
			if completedRefID == "" {
				return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload_session already completed without bound reference_file_ref", nil)
			}
			return nil
		}
		if request.Status == domain.UploadRequestStatusCancelled || request.Status == domain.UploadRequestStatusExpired ||
			request.SessionStatus == domain.DesignAssetSessionStatusCancelled || request.SessionStatus == domain.DesignAssetSessionStatusExpired {
			return domain.NewAppError(domain.ErrCodeConflict, "upload_session changed concurrently and is already terminal", nil)
		}
		ref, createErr := s.assetStorageRefRepo.Create(ctx, tx, &domain.AssetStorageRef{
			RefID:           uuid.NewString(),
			OwnerType:       request.OwnerType,
			OwnerID:         request.OwnerID,
			UploadRequestID: request.RequestID,
			StorageAdapter:  domain.AssetStorageAdapterOSSUploadService,
			RefType:         domain.AssetStorageRefTypeGenericObject,
			RefKey:          resolvedStorageKey,
			FileName:        request.FileName,
			MimeType:        firstNonEmpty(metaMimeType(meta), request.MimeType),
			FileSize:        firstNonNilInt64(metaFileSize(meta), request.ExpectedSize, request.FileSize),
			IsPlaceholder:   metaIsStub(meta),
			ChecksumHint:    firstNonEmpty(strings.TrimSpace(checksumHint), request.ChecksumHint),
			Status:          domain.AssetStorageRefStatusRecorded,
			CreatedAt:       now,
		})
		if createErr != nil {
			return createErr
		}
		createdRef = ref
		completedRefID = ref.RefID
		if err := s.uploadRequestRepo.UpdateBinding(ctx, tx, request.RequestID, nil, createdRef.RefID, domain.UploadRequestStatusBound, firstNonEmpty(strings.TrimSpace(remark), strings.TrimSpace(request.Remark))); err != nil {
			return err
		}
		return s.uploadRequestRepo.UpdateSession(ctx, tx, repo.UploadRequestSessionUpdate{
			RequestID:      request.RequestID,
			SessionStatus:  domain.DesignAssetSessionStatusCompleted,
			RemoteUploadID: request.RemoteUploadID,
			RemoteFileID:   metaFileID(meta),
			LastSyncedAt:   &lastSyncedAt,
			Remark:         firstNonEmpty(strings.TrimSpace(remark), strings.TrimSpace(request.Remark)),
		})
	}); err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			return nil, appErr
		}
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, infraError("complete task-create reference upload session", err)
	}

	session, appErr := s.GetUploadSession(ctx, request.RequestID, completedBy)
	if appErr != nil {
		return nil, appErr
	}
	if createdRef == nil {
		var err error
		createdRef, err = s.assetStorageRefRepo.GetByRefID(ctx, completedRefID)
		if err != nil {
			return nil, infraError("get completed task-create reference_file_ref", err)
		}
		if createdRef == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "completed reference_file_ref is missing", nil)
		}
	}
	return &CompleteTaskReferenceUploadSessionResult{
		Session:          session,
		ReferenceFileRef: completedRefID,
		StorageRef:       createdRef,
		RefObject:        s.buildReferenceFileRef(createdRef, domain.ReferenceFileRefSourceTaskCreateAssetCenter),
	}, nil
}

func (s *taskCreateReferenceUploadService) probeStoredReferenceWithRetry(ctx context.Context, request *domain.UploadRequest, storageKey, expectedSHA256 string) (*RemoteStoredFileProbe, error) {
	maxAttempts := s.probeRetryMax
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	probeBaseURL := s.uploadServiceBaseURL()
	selectedProbeHost := probeHostFromBaseURL(probeBaseURL)
	expectedSize := int64ValueFromPtr(request.ExpectedSize)
	expectedSHA := strings.TrimSpace(expectedSHA256)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		probe, err := s.uploadClient.ProbeStoredFile(ctx, RemoteProbeStoredFileRequest{StorageKey: storageKey})
		if err != nil {
			retryable := attempt < maxAttempts && isRetryableStoredFileProbeError(err)
			logUploadProbe("task_reference_upload_probe_attempt", map[string]interface{}{
				"trace_id":                domain.TraceIDFromContext(ctx),
				"upload_mode":             "reference_small",
				"upload_service_base_url": probeBaseURL,
				"selected_probe_host":     selectedProbeHost,
				"attempt":                 attempt,
				"max_attempts":            maxAttempts,
				"filename":                strings.TrimSpace(request.FileName),
				"storage_key":             truncateLogString(storageKey, 192),
				"expected_size":           expectedSize,
				"expected_sha256":         expectedSHA,
				"probe_error":             err.Error(),
				"retryable":               retryable,
			})
			if !retryable {
				return nil, err
			}
			lastErr = err
			s.sleepProbeRetry(attempt)
			continue
		}
		if probe == nil {
			err = fmt.Errorf("upload service probe_stored_file returned empty response")
			logUploadProbe("task_reference_upload_probe_attempt", map[string]interface{}{
				"trace_id":                domain.TraceIDFromContext(ctx),
				"upload_mode":             "reference_small",
				"upload_service_base_url": probeBaseURL,
				"selected_probe_host":     selectedProbeHost,
				"attempt":                 attempt,
				"max_attempts":            maxAttempts,
				"filename":                strings.TrimSpace(request.FileName),
				"storage_key":             truncateLogString(storageKey, 192),
				"expected_size":           expectedSize,
				"expected_sha256":         expectedSHA,
				"probe_error":             err.Error(),
				"retryable":               false,
			})
			return nil, err
		}
		retryable, retryReason := shouldRetryStoredFileProbeResult(probe, request.ExpectedSize)
		logUploadProbe("task_reference_upload_probe_attempt", map[string]interface{}{
			"trace_id":                domain.TraceIDFromContext(ctx),
			"upload_mode":             "reference_small",
			"upload_service_base_url": probeBaseURL,
			"selected_probe_host":     selectedProbeHost,
			"attempt":                 attempt,
			"max_attempts":            maxAttempts,
			"filename":                strings.TrimSpace(request.FileName),
			"storage_key":             truncateLogString(storageKey, 192),
			"expected_size":           expectedSize,
			"expected_sha256":         expectedSHA,
			"probe_status":            probe.StatusCode,
			"stored_bytes_read":       probe.BytesRead,
			"stored_content_length":   probe.ContentLengthHeader,
			"stored_sha256":           probe.SHA256,
			"retryable":               retryable && attempt < maxAttempts,
			"retry_reason":            retryReason,
		})
		if retryable && attempt < maxAttempts {
			lastErr = fmt.Errorf("transient stored file probe result: %s", retryReason)
			s.sleepProbeRetry(attempt)
			continue
		}
		if expectedSHA != "" && strings.TrimSpace(probe.SHA256) == "" {
			err = fmt.Errorf("upload service probe_stored_file returned incomplete metadata")
			logUploadProbe("task_reference_upload_probe_attempt", map[string]interface{}{
				"trace_id":                domain.TraceIDFromContext(ctx),
				"upload_mode":             "reference_small",
				"upload_service_base_url": probeBaseURL,
				"selected_probe_host":     selectedProbeHost,
				"attempt":                 attempt,
				"max_attempts":            maxAttempts,
				"filename":                strings.TrimSpace(request.FileName),
				"storage_key":             truncateLogString(storageKey, 192),
				"expected_size":           expectedSize,
				"expected_sha256":         expectedSHA,
				"probe_status":            probe.StatusCode,
				"stored_bytes_read":       probe.BytesRead,
				"stored_content_length":   probe.ContentLengthHeader,
				"stored_sha256":           probe.SHA256,
				"probe_error":             err.Error(),
				"retryable":               false,
			})
			return nil, err
		}
		return probe, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("stored file probe exhausted without result")
	}
	return nil, lastErr
}

func (s *taskCreateReferenceUploadService) sleepProbeRetry(attempt int) {
	if s.sleepFn == nil || s.probeRetryBackoff <= 0 {
		return
	}
	s.sleepFn(time.Duration(attempt) * s.probeRetryBackoff)
}

func (s *taskCreateReferenceUploadService) uploadServiceBaseURL() string {
	provider, ok := s.uploadClient.(uploadServiceBaseURLProvider)
	if !ok {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(provider.UploadServiceBaseURL()), "/")
}

func probeHostFromBaseURL(rawBaseURL string) string {
	if strings.TrimSpace(rawBaseURL) == "" {
		return ""
	}
	parsed, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Host)
}

func isRetryableStoredFileProbeError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *UploadServiceHTTPError
	if errors.As(err, &httpErr) {
		switch {
		case httpErr.StatusCode == http.StatusNotFound,
			httpErr.StatusCode == http.StatusConflict,
			httpErr.StatusCode == http.StatusTooEarly,
			httpErr.StatusCode == http.StatusTooManyRequests,
			httpErr.StatusCode >= http.StatusInternalServerError:
			return true
		default:
			return false
		}
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

func shouldRetryStoredFileProbeResult(probe *RemoteStoredFileProbe, expectedSize *int64) (bool, string) {
	if probe == nil {
		return false, ""
	}
	if expectedSize != nil && *expectedSize > 0 && probe.BytesRead == 0 && probe.ContentLengthHeader == 0 {
		return true, "stored_file_empty_during_probe"
	}
	return false, ""
}
