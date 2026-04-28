package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"workflow/domain"
	"workflow/repo"
)

type CreateTaskAssetUploadSessionParams struct {
	TaskID        int64
	AssetID       *int64
	SourceAssetID *int64
	CreatedBy     int64
	AssetType     domain.TaskAssetType
	Filename      string
	ExpectedSize  *int64
	MimeType      string
	FileHash      string
	Remark        string
	TargetSKUCode string
}

type CompleteTaskAssetUploadSessionParams struct {
	TaskID            int64
	SessionID         string
	CompletedBy       int64
	Remark            string
	FileHash          string
	UploadContentType string
	OSSParts          []OSSCompletePart
	OSSUploadID       string
	OSSObjectKey      string
}

type CancelTaskAssetUploadSessionParams struct {
	TaskID      int64
	SessionID   string
	CancelledBy int64
	Remark      string
}

type ListAssetResourcesParams struct {
	TaskID        *int64
	SourceAssetID *int64
	AssetType     domain.TaskAssetType
	ScopeSKUCode  string
	ArchiveStatus domain.AssetArchiveStatus
	UploadStatus  domain.DesignAssetUploadStatus
}

type CreateTaskAssetUploadSessionResult struct {
	Session   *domain.UploadSession    `json:"session"`
	Remote    *RemoteUploadSessionPlan `json:"remote"`
	OSSDirect *OSSDirectUploadPlan     `json:"oss_direct,omitempty"`
}

type CompleteTaskAssetUploadSessionResult struct {
	Session *domain.UploadSession      `json:"session"`
	Asset   *domain.DesignAsset        `json:"asset"`
	Version *domain.DesignAssetVersion `json:"version"`
}

type TaskAssetCenterService interface {
	ListAssetResources(ctx context.Context, params ListAssetResourcesParams) ([]*domain.DesignAsset, *domain.AppError)
	GetAsset(ctx context.Context, assetID int64) (*domain.DesignAsset, *domain.AppError)
	ListAssets(ctx context.Context, taskID int64) ([]*domain.DesignAsset, *domain.AppError)
	ListVersions(ctx context.Context, taskID, assetID int64) ([]*domain.DesignAssetVersion, *domain.AppError)
	GetAssetDownloadInfoByID(ctx context.Context, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError)
	GetAssetPreviewInfoByID(ctx context.Context, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError)
	GetAssetDownloadInfo(ctx context.Context, taskID, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError)
	GetVersionDownloadInfo(ctx context.Context, taskID, assetID, versionID int64) (*domain.AssetDownloadInfo, *domain.AppError)
	GetUploadSessionByID(ctx context.Context, sessionID string) (*domain.UploadSession, *domain.AppError)
	CreateUploadSession(ctx context.Context, params CreateTaskAssetUploadSessionParams) (*CreateTaskAssetUploadSessionResult, *domain.AppError)
	GetUploadSession(ctx context.Context, taskID int64, sessionID string) (*domain.UploadSession, *domain.AppError)
	CreateSmallUploadSession(ctx context.Context, params CreateTaskAssetUploadSessionParams) (*CreateTaskAssetUploadSessionResult, *domain.AppError)
	CreateMultipartUploadSession(ctx context.Context, params CreateTaskAssetUploadSessionParams) (*CreateTaskAssetUploadSessionResult, *domain.AppError)
	CompleteUploadSessionByID(ctx context.Context, params CompleteTaskAssetUploadSessionParams) (*CompleteTaskAssetUploadSessionResult, *domain.AppError)
	CompleteUploadSession(ctx context.Context, params CompleteTaskAssetUploadSessionParams) (*CompleteTaskAssetUploadSessionResult, *domain.AppError)
	CancelUploadSessionByID(ctx context.Context, params CancelTaskAssetUploadSessionParams) (*domain.UploadSession, *domain.AppError)
	CancelUploadSession(ctx context.Context, params CancelTaskAssetUploadSessionParams) (*domain.UploadSession, *domain.AppError)
}

type taskAssetCenterService struct {
	taskRepo                  repo.TaskRepo
	designAssetRepo           repo.DesignAssetRepo
	taskAssetRepo             repo.TaskAssetRepo
	uploadRequestRepo         repo.UploadRequestRepo
	assetStorageRefRepo       repo.AssetStorageRefRepo
	taskEventRepo             repo.TaskEventRepo
	txRunner                  repo.TxRunner
	uploadClient              UploadServiceClient
	ossDirectService          *OSSDirectService
	nowFn                     func() time.Time
	runAsyncFn                func(func())
	derivedPreviewGracePeriod time.Duration
	dataScopeResolver         DataScopeResolver
	scopeUserRepo             repo.UserRepo
	userDisplayNameResolver   UserDisplayNameResolver
}

const (
	taskAssetVersionUniqueKey     = "uq_task_assets_task_version"
	assetVersionRaceRetryDenyCode = "asset_version_race_retry"
)

type TaskAssetCenterServiceOption func(*taskAssetCenterService)

func NewTaskAssetCenterService(
	taskRepo repo.TaskRepo,
	designAssetRepo repo.DesignAssetRepo,
	taskAssetRepo repo.TaskAssetRepo,
	uploadRequestRepo repo.UploadRequestRepo,
	assetStorageRefRepo repo.AssetStorageRefRepo,
	taskEventRepo repo.TaskEventRepo,
	txRunner repo.TxRunner,
	uploadClient UploadServiceClient,
	options ...TaskAssetCenterServiceOption,
) TaskAssetCenterService {
	svc := &taskAssetCenterService{
		taskRepo:            taskRepo,
		designAssetRepo:     designAssetRepo,
		taskAssetRepo:       taskAssetRepo,
		uploadRequestRepo:   uploadRequestRepo,
		assetStorageRefRepo: assetStorageRefRepo,
		taskEventRepo:       taskEventRepo,
		txRunner:            txRunner,
		uploadClient:        uploadClient,
		nowFn:               time.Now,
		runAsyncFn: func(fn func()) {
			go fn()
		},
		derivedPreviewGracePeriod: 3 * time.Second,
	}
	for _, opt := range options {
		opt(svc)
	}
	return svc
}

func WithOSSDirectService(ossDirect *OSSDirectService) func(*taskAssetCenterService) {
	return func(s *taskAssetCenterService) {
		s.ossDirectService = ossDirect
	}
}

func WithTaskAssetCenterDataScopeResolver(resolver DataScopeResolver) TaskAssetCenterServiceOption {
	return func(s *taskAssetCenterService) {
		s.dataScopeResolver = resolver
	}
}

func WithTaskAssetCenterScopeUserRepo(userRepo repo.UserRepo) TaskAssetCenterServiceOption {
	return func(s *taskAssetCenterService) {
		s.scopeUserRepo = userRepo
	}
}

func WithTaskAssetCenterUserDisplayNameResolver(resolver UserDisplayNameResolver) TaskAssetCenterServiceOption {
	return func(s *taskAssetCenterService) {
		s.userDisplayNameResolver = resolver
	}
}

func (s *taskAssetCenterService) taskActionAuthorizer() *taskActionAuthorizer {
	return newTaskActionAuthorizer(s.dataScopeResolver, s.scopeUserRepo)
}

func isTaskAssetVersionConflict(err error) bool {
	if err == nil {
		return false
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return strings.Contains(mysqlErr.Message, taskAssetVersionUniqueKey)
	}
	return strings.Contains(err.Error(), taskAssetVersionUniqueKey)
}

func assetVersionRaceConflictAppError(taskID int64, requestID string, attemptedVersionNo int) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeConflict, "asset version race detected; retry with a fresh upload session", map[string]interface{}{
		"deny_code":            assetVersionRaceRetryDenyCode,
		"task_id":              taskID,
		"request_id":           strings.TrimSpace(requestID),
		"attempted_version_no": attemptedVersionNo,
	})
}

func (s *taskAssetCenterService) logAssetVersionConflict(ctx context.Context, taskID int64, request *domain.UploadRequest, attemptedVersionNo int, err error, ossRequestID string) {
	requestID := ""
	remoteUploadID := ""
	if request != nil {
		requestID = strings.TrimSpace(request.RequestID)
		remoteUploadID = strings.TrimSpace(request.RemoteUploadID)
	}
	logUploadProbe("task_asset_version_conflict", map[string]interface{}{
		"trace_id":             domain.TraceIDFromContext(ctx),
		"request_id":           requestID,
		"upload_request_id":    requestID,
		"task_id":              taskID,
		"attempted_version_no": attemptedVersionNo,
		"remote_upload_id":     remoteUploadID,
		"oss_request_id":       strings.TrimSpace(ossRequestID),
		"error":                err.Error(),
	})
}

func (s *taskAssetCenterService) ListAssets(ctx context.Context, taskID int64) ([]*domain.DesignAsset, *domain.AppError) {
	params := ListAssetResourcesParams{}
	params.TaskID = &taskID
	return s.ListAssetResources(ctx, params)
}

func (s *taskAssetCenterService) ListAssetResources(ctx context.Context, params ListAssetResourcesParams) ([]*domain.DesignAsset, *domain.AppError) {
	filter := repo.DesignAssetListFilter{
		TaskID:        params.TaskID,
		SourceAssetID: params.SourceAssetID,
		ScopeSKUCode:  strings.TrimSpace(params.ScopeSKUCode),
	}
	if normalized := domain.NormalizeTaskAssetType(params.AssetType); normalized != "" {
		filter.AssetType = &normalized
	}
	if params.TaskID != nil {
		if _, appErr := s.requireTask(ctx, *params.TaskID); appErr != nil {
			return nil, appErr
		}
	}
	if params.SourceAssetID != nil {
		sourceAsset, appErr := s.requireDesignAssetByID(ctx, *params.SourceAssetID)
		if appErr != nil {
			return nil, appErr
		}
		if !sourceAsset.AssetType.IsSource() {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "source_asset_id must point to source asset", map[string]interface{}{
				"source_asset_id": *params.SourceAssetID,
				"asset_type":      sourceAsset.AssetType,
			})
		}
		if params.TaskID != nil && sourceAsset.TaskID != *params.TaskID {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "source_asset_id does not belong to task_id", map[string]interface{}{
				"task_id":         *params.TaskID,
				"source_asset_id": *params.SourceAssetID,
			})
		}
	}
	assets, err := s.designAssetRepo.List(ctx, filter)
	if err != nil {
		return nil, infraError("list design asset resources", err)
	}
	if assets == nil {
		return []*domain.DesignAsset{}, nil
	}
	filtered := make([]*domain.DesignAsset, 0, len(assets))
	for _, asset := range assets {
		if asset == nil || asset.CurrentVersionID == nil || *asset.CurrentVersionID == 0 {
			continue
		}
		hydrated, appErr := s.loadAssetResource(ctx, asset)
		if appErr != nil {
			return nil, appErr
		}
		if !matchesAssetResourceFilters(hydrated, params) {
			continue
		}
		filtered = append(filtered, hydrated)
	}
	return filtered, nil
}

func (s *taskAssetCenterService) GetAsset(ctx context.Context, assetID int64) (*domain.DesignAsset, *domain.AppError) {
	asset, appErr := s.requireDesignAssetByID(ctx, assetID)
	if appErr != nil {
		return nil, appErr
	}
	return s.loadAssetResource(ctx, asset)
}

func (s *taskAssetCenterService) GetAssetDownloadInfoByID(ctx context.Context, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	asset, appErr := s.GetAsset(ctx, assetID)
	if appErr != nil {
		return nil, appErr
	}
	if asset.CurrentVersion == nil {
		return nil, domain.ErrNotFound
	}
	if appErr := validateAssetVersionObjectAvailable(asset.CurrentVersion); appErr != nil {
		return nil, appErr
	}
	return buildAssetDownloadInfoWithOSS(asset.CurrentVersion, s.uploadClient, s.ossDirectService), nil
}

func (s *taskAssetCenterService) GetAssetPreviewInfoByID(ctx context.Context, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	asset, appErr := s.GetAsset(ctx, assetID)
	if appErr != nil {
		return nil, appErr
	}
	if asset.CurrentVersion == nil {
		return nil, domain.ErrNotFound
	}
	if !asset.CurrentVersion.PreviewAvailable {
		if asset.AssetType.IsSource() {
			info, resolveErr := s.resolveSourceDerivedPreviewInfo(ctx, asset)
			if resolveErr != nil {
				return nil, resolveErr
			}
			if info != nil {
				return info, nil
			}
		}
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "asset preview is not available", map[string]interface{}{
			"asset_id": asset.ID,
		})
	}
	if appErr := validateAssetVersionObjectAvailable(asset.CurrentVersion); appErr != nil {
		return nil, appErr
	}
	return buildAssetPreviewInfoWithOSS(asset.CurrentVersion, s.uploadClient, s.ossDirectService), nil
}

func (s *taskAssetCenterService) GetUploadSessionByID(ctx context.Context, sessionID string) (*domain.UploadSession, *domain.AppError) {
	request, appErr := s.requireUploadRequestByID(ctx, sessionID)
	if appErr != nil {
		return nil, appErr
	}
	return s.GetUploadSession(ctx, request.TaskID, request.RequestID)
}

func (s *taskAssetCenterService) CreateUploadSession(ctx context.Context, params CreateTaskAssetUploadSessionParams) (*CreateTaskAssetUploadSessionResult, *domain.AppError) {
	mode, appErr := inferTaskAssetUploadMode(params.AssetType)
	if appErr != nil {
		return nil, appErr
	}
	if mode == domain.DesignAssetUploadModeMultipart {
		return s.CreateMultipartUploadSession(ctx, params)
	}
	return s.CreateSmallUploadSession(ctx, params)
}

func (s *taskAssetCenterService) CompleteUploadSessionByID(ctx context.Context, params CompleteTaskAssetUploadSessionParams) (*CompleteTaskAssetUploadSessionResult, *domain.AppError) {
	request, appErr := s.requireUploadRequestByID(ctx, params.SessionID)
	if appErr != nil {
		return nil, appErr
	}
	params.TaskID = request.TaskID
	return s.CompleteUploadSession(ctx, params)
}

func (s *taskAssetCenterService) CancelUploadSessionByID(ctx context.Context, params CancelTaskAssetUploadSessionParams) (*domain.UploadSession, *domain.AppError) {
	request, appErr := s.requireUploadRequestByID(ctx, params.SessionID)
	if appErr != nil {
		return nil, appErr
	}
	params.TaskID = request.TaskID
	return s.CancelUploadSession(ctx, params)
}

func (s *taskAssetCenterService) ListVersions(ctx context.Context, taskID, assetID int64) ([]*domain.DesignAssetVersion, *domain.AppError) {
	task, appErr := s.requireTask(ctx, taskID)
	if appErr != nil {
		return nil, appErr
	}
	asset, appErr := s.requireDesignAsset(ctx, taskID, assetID)
	if appErr != nil {
		return nil, appErr
	}
	records, err := s.taskAssetRepo.ListByAssetID(ctx, asset.ID)
	if err != nil {
		return nil, infraError("list design asset versions", err)
	}
	versions := make([]*domain.DesignAssetVersion, 0, len(records))
	for _, record := range records {
		if version := domain.BuildDesignAssetVersion(record); version != nil {
			s.applyDesignAssetVersionDerivedFields(task, asset, version)
			versions = append(versions, version)
		}
	}
	enrichDesignAssetVersionUploaderNames(ctx, s.userDisplayNameResolver, versions)
	s.applyDesignAssetVersionRoles(task, asset, versions)
	return versions, nil
}

func (s *taskAssetCenterService) GetAssetDownloadInfo(ctx context.Context, taskID, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	versions, appErr := s.ListVersions(ctx, taskID, assetID)
	if appErr != nil {
		return nil, appErr
	}
	if len(versions) == 0 {
		return nil, domain.ErrNotFound
	}
	version := versions[len(versions)-1]
	if appErr := validateAssetVersionObjectAvailable(version); appErr != nil {
		return nil, appErr
	}
	return buildAssetDownloadInfoWithOSS(version, s.uploadClient, s.ossDirectService), nil
}

func (s *taskAssetCenterService) GetVersionDownloadInfo(ctx context.Context, taskID, assetID, versionID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	versions, appErr := s.ListVersions(ctx, taskID, assetID)
	if appErr != nil {
		return nil, appErr
	}
	for _, version := range versions {
		if version != nil && version.ID == versionID {
			if appErr := validateAssetVersionObjectAvailable(version); appErr != nil {
				return nil, appErr
			}
			return buildAssetDownloadInfoWithOSS(version, s.uploadClient, s.ossDirectService), nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *taskAssetCenterService) GetUploadSession(ctx context.Context, taskID int64, sessionID string) (*domain.UploadSession, *domain.AppError) {
	if _, appErr := s.requireTask(ctx, taskID); appErr != nil {
		return nil, appErr
	}
	request, err := s.uploadRequestRepo.GetByRequestID(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, infraError("get upload session", err)
	}
	if request == nil {
		return nil, domain.ErrNotFound
	}
	if request.TaskID != taskID && !(request.OwnerType == domain.AssetOwnerTypeTask && request.OwnerID == taskID) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session does not belong to current task", nil)
	}
	if request.RemoteUploadID != "" && request.SessionStatus == domain.DesignAssetSessionStatusCreated {
		if request, err = s.syncUploadRequestFromRemote(ctx, request); err != nil {
			return nil, infraError("sync upload session from upload service", err)
		}
	}
	return domain.BuildUploadSession(request), nil
}

func (s *taskAssetCenterService) CreateSmallUploadSession(ctx context.Context, params CreateTaskAssetUploadSessionParams) (*CreateTaskAssetUploadSessionResult, *domain.AppError) {
	return s.createUploadSession(ctx, params, domain.DesignAssetUploadModeSmall)
}

func (s *taskAssetCenterService) CreateMultipartUploadSession(ctx context.Context, params CreateTaskAssetUploadSessionParams) (*CreateTaskAssetUploadSessionResult, *domain.AppError) {
	return s.createUploadSession(ctx, params, domain.DesignAssetUploadModeMultipart)
}

func (s *taskAssetCenterService) CompleteUploadSession(ctx context.Context, params CompleteTaskAssetUploadSessionParams) (*CompleteTaskAssetUploadSessionResult, *domain.AppError) {
	task, appErr := s.requireTask(ctx, params.TaskID)
	if appErr != nil {
		return nil, appErr
	}
	request, appErr := s.requireUploadRequest(ctx, params.TaskID, params.SessionID)
	if appErr != nil {
		return nil, appErr
	}
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionAssetUploadSessionComplete, task, "", "")
	if !decision.Allowed {
		if !allowPostTransitionUploadSessionComplete(ctx, authz, decision, task, request) {
			authz.logDecision(TaskActionAssetUploadSessionComplete, decision)
			return nil, taskActionDecisionAppError(TaskActionAssetUploadSessionComplete, decision)
		}
	}
	authz.logDecision(TaskActionAssetUploadSessionComplete, decision)
	if request.Status == domain.UploadRequestStatusBound || (request.SessionStatus == domain.DesignAssetSessionStatusCompleted && request.BoundAssetID != nil) {
		return s.buildCompletedUploadSessionResult(ctx, params.TaskID, request)
	}
	if request.SessionStatus == domain.DesignAssetSessionStatusCancelled || request.SessionStatus == domain.DesignAssetSessionStatusExpired {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload_session is already terminal", nil)
	}
	if request.TaskAssetType == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session asset_type is required", nil)
	}
	if appErr := validateUploadContentTypeContract(request, params.UploadContentType); appErr != nil {
		return nil, appErr
	}
	if appErr := validateOSSDirectCompleteContract(params); appErr != nil {
		return nil, appErr
	}
	requestAssetType := domain.NormalizeTaskAssetType(*request.TaskAssetType)
	scopeSKUCode := strings.TrimSpace(request.TargetSKUCode)

	checksumHint := firstNonEmpty(strings.TrimSpace(params.FileHash), strings.TrimSpace(request.ChecksumHint))
	var err error
	ossDirectReady := s.canFinalizeOSSDirectUpload(params)
	if request.RemoteUploadID != "" && request.SessionStatus == domain.DesignAssetSessionStatusCreated && !ossDirectReady {
		if request, err = s.syncUploadRequestFromRemote(ctx, request); err != nil {
			return nil, infraError("sync upload session before completion", err)
		}
	}

	ossDirectFinalized := false
	ossDirectObjectKey := ""
	if ossDirectReady {
		ossObjectKey := strings.TrimSpace(params.OSSObjectKey)
		ossUploadID := strings.TrimSpace(params.OSSUploadID)
		if err := s.ossDirectService.CompleteMultipartUpload(ctx, ossObjectKey, ossUploadID, params.OSSParts); err != nil {
			return nil, infraError("complete oss direct multipart upload", err)
		}
		ossDirectFinalized = true
		ossDirectObjectKey = ossObjectKey
	}

	meta, appErr := s.resolveCompletedUploadMeta(ctx, request, checksumHint, ossDirectObjectKey, ossDirectFinalized)
	if appErr != nil {
		return nil, appErr
	}

	now := s.nowFn().UTC()
	lastSyncedAt := now
	resolvedStorageKey := buildRemoteStorageKey(meta, request)
	var assetID int64
	var versionID int64
	storageRefID := uuid.NewString()
	var asset *domain.DesignAsset
	attemptedTimelineVersionNo := 0

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if request.AssetID != nil {
			existingAsset, err := s.designAssetRepo.GetByID(ctx, *request.AssetID)
			if err != nil {
				return fmt.Errorf("get existing design asset: %w", err)
			}
			if existingAsset == nil || existingAsset.TaskID != params.TaskID {
				return domain.ErrNotFound
			}
			if existingAsset.AssetType != requestAssetType {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_type does not match existing asset", nil)
			}
			if strings.TrimSpace(existingAsset.ScopeSKUCode) != scopeSKUCode {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "target_sku_code does not match existing asset scope", map[string]interface{}{
					"target_sku_code":   scopeSKUCode,
					"asset_scope_sku":   existingAsset.ScopeSKUCode,
					"asset_id":          existingAsset.ID,
					"upload_session_id": request.RequestID,
				})
			}
			asset = existingAsset
			assetID = existingAsset.ID
		} else {
			assetNo, err := s.designAssetRepo.NextAssetNo(ctx, tx, params.TaskID)
			if err != nil {
				return fmt.Errorf("next design asset no: %w", err)
			}
			asset = &domain.DesignAsset{
				TaskID:        params.TaskID,
				AssetNo:       assetNo,
				SourceAssetID: request.SourceAssetID,
				ScopeSKUCode:  scopeSKUCode,
				AssetType:     requestAssetType,
				CreatedBy:     params.CompletedBy,
			}
			id, err := s.designAssetRepo.Create(ctx, tx, asset)
			if err != nil {
				return fmt.Errorf("create design asset: %w", err)
			}
			asset.ID = id
			assetID = id
		}

		timelineVersionNo, err := s.taskAssetRepo.NextVersionNo(ctx, tx, params.TaskID)
		if err != nil {
			return fmt.Errorf("next task asset timeline version: %w", err)
		}
		attemptedTimelineVersionNo = timelineVersionNo
		assetVersionNo, err := s.taskAssetRepo.NextAssetVersionNo(ctx, tx, assetID)
		if err != nil {
			return fmt.Errorf("next design asset version: %w", err)
		}

		uploadStatus := string(domain.DesignAssetUploadStatusUploaded)
		previewStatus := string(domain.DesignAssetPreviewStatusNotApplicable)
		taskAsset := &domain.TaskAsset{
			TaskID:          params.TaskID,
			AssetID:         &assetID,
			ScopeSKUCode:    optionalStringPtr(scopeSKUCode),
			AssetType:       requestAssetType,
			VersionNo:       timelineVersionNo,
			AssetVersionNo:  &assetVersionNo,
			UploadMode:      optionalStringPtr(string(request.UploadMode)),
			UploadRequestID: &request.RequestID,
			StorageRefID:    &storageRefID,
			FileName:        request.FileName,
			OriginalName:    optionalStringPtr(request.FileName),
			RemoteFileID:    meta.FileID,
			MimeType:        optionalStringPtr(firstNonEmpty(meta.MimeType, request.MimeType)),
			FileSize:        firstNonNilInt64(meta.FileSize, request.ExpectedSize, request.FileSize),
			StorageKey:      optionalStringPtr(resolvedStorageKey),
			WholeHash:       meta.FileHash,
			UploadStatus:    &uploadStatus,
			PreviewStatus:   &previewStatus,
			UploadedBy:      params.CompletedBy,
			UploadedAt:      &now,
			Remark:          firstNonEmpty(strings.TrimSpace(params.Remark), strings.TrimSpace(request.Remark)),
		}
		id, err := s.taskAssetRepo.Create(ctx, tx, taskAsset)
		if err != nil {
			return fmt.Errorf("create task asset version: %w", err)
		}
		versionID = id

		ref := &domain.AssetStorageRef{
			RefID:           storageRefID,
			AssetID:         &versionID,
			OwnerType:       domain.AssetOwnerTypeTaskAsset,
			OwnerID:         versionID,
			UploadRequestID: request.RequestID,
			StorageAdapter:  domain.AssetStorageAdapterOSSUploadService,
			RefType:         domain.AssetStorageRefTypeTaskAssetObject,
			RefKey:          resolvedStorageKey,
			FileName:        request.FileName,
			MimeType:        firstNonEmpty(meta.MimeType, request.MimeType),
			FileSize:        firstNonNilInt64(meta.FileSize, request.ExpectedSize, request.FileSize),
			IsPlaceholder:   meta.IsStub,
			ChecksumHint:    firstNonEmpty(checksumHint, request.ChecksumHint),
			Status:          domain.AssetStorageRefStatusRecorded,
		}
		if _, err := s.assetStorageRefRepo.Create(ctx, tx, ref); err != nil {
			return fmt.Errorf("create asset storage ref: %w", err)
		}
		if err := s.designAssetRepo.UpdateCurrentVersionID(ctx, tx, assetID, &versionID); err != nil {
			return fmt.Errorf("update design asset current version: %w", err)
		}
		if err := s.uploadRequestRepo.UpdateBinding(ctx, tx, request.RequestID, &versionID, storageRefID, domain.UploadRequestStatusBound, taskAsset.Remark); err != nil {
			return fmt.Errorf("update upload request binding: %w", err)
		}
		if err := s.uploadRequestRepo.UpdateSession(ctx, tx, repo.UploadRequestSessionUpdate{
			RequestID:      request.RequestID,
			AssetID:        &assetID,
			SessionStatus:  domain.DesignAssetSessionStatusCompleted,
			RemoteUploadID: request.RemoteUploadID,
			RemoteFileID:   meta.FileID,
			LastSyncedAt:   &lastSyncedAt,
			Remark:         taskAsset.Remark,
		}); err != nil {
			return fmt.Errorf("update upload request session: %w", err)
		}
		if requestAssetType.IsDelivery() {
			switch task.TaskStatus {
			case domain.TaskStatusPendingAssign, domain.TaskStatusAssigned, domain.TaskStatusInProgress, domain.TaskStatusRejectedByAuditA, domain.TaskStatusRejectedByAuditB:
				advance, gateErr := s.shouldAdvanceTaskToPendingAuditA(ctx, task, scopeSKUCode)
				if gateErr != nil {
					return fmt.Errorf("check design submit gate: %w", gateErr)
				}
				if advance {
					if err := s.taskRepo.UpdateStatus(ctx, tx, params.TaskID, domain.TaskStatusPendingAuditA); err != nil {
						return fmt.Errorf("advance task status after delivery upload: %w", err)
					}
					if err := s.taskRepo.UpdateHandler(ctx, tx, params.TaskID, nil); err != nil {
						return fmt.Errorf("clear current handler after delivery upload: %w", err)
					}
					_, err = s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventDesignSubmitted, &params.CompletedBy, map[string]interface{}{
						"asset_type": string(requestAssetType), "asset_id": assetID, "designer_id": task.DesignerID,
						"upload_session_id": request.RequestID, "uploaded_by": params.CompletedBy, "target_sku_code": scopeSKUCode,
					})
					if err != nil {
						return fmt.Errorf("append design submitted event: %w", err)
					}
				}
			}
		}
		_, err = s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventAssetVersionCreated, &params.CompletedBy, map[string]interface{}{
			"asset_id":          assetID,
			"asset_type":        string(requestAssetType),
			"target_sku_code":   scopeSKUCode,
			"asset_version_id":  versionID,
			"asset_version_no":  assetVersionNo,
			"timeline_version":  timelineVersionNo,
			"upload_session_id": request.RequestID,
			"remote_file_id":    meta.FileID,
			"storage_key":       resolvedStorageKey,
			"remark":            taskAsset.Remark,
		})
		if err != nil {
			return fmt.Errorf("append asset version created event: %w", err)
		}
		_, err = s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventAssetUploadSessionCompleted, &params.CompletedBy, map[string]interface{}{
			"asset_id":          assetID,
			"asset_type":        string(requestAssetType),
			"target_sku_code":   scopeSKUCode,
			"asset_version_id":  versionID,
			"asset_version_no":  assetVersionNo,
			"timeline_version":  timelineVersionNo,
			"upload_session_id": request.RequestID,
			"upload_mode":       string(request.UploadMode),
			"storage_provider":  string(request.StorageProvider),
			"remote_upload_id":  request.RemoteUploadID,
			"remote_file_id":    meta.FileID,
			"storage_key":       resolvedStorageKey,
			"file_hash":         meta.FileHash,
			"remark":            taskAsset.Remark,
		})
		if err != nil {
			return fmt.Errorf("append upload session completed event: %w", err)
		}
		return nil
	})
	if txErr != nil {
		log.Printf("complete_upload_session_tx_failed trace_id=%s task_id=%d session_id=%s asset_id=%v err=%v",
			domain.TraceIDFromContext(ctx), params.TaskID, request.RequestID, request.AssetID, txErr)
		if appErr, ok := txErr.(*domain.AppError); ok {
			return nil, appErr
		}
		if isTaskAssetVersionConflict(txErr) {
			s.logAssetVersionConflict(ctx, params.TaskID, request, attemptedTimelineVersionNo, txErr, "")
			return nil, assetVersionRaceConflictAppError(params.TaskID, request.RequestID, attemptedTimelineVersionNo)
		}
		return nil, infraError("complete upload session", txErr)
	}

	request, appErr = s.requireUploadRequest(ctx, params.TaskID, request.RequestID)
	if appErr != nil {
		return nil, appErr
	}
	result, appErr := s.buildCompletedUploadSessionResult(ctx, params.TaskID, request)
	if appErr != nil {
		return nil, appErr
	}
	if requestAssetType.IsSource() && result != nil {
		s.scheduleDerivedPreviewGeneration(params.TaskID, assetID, params.CompletedBy, result.Version)
	}
	return result, nil
}

func (s *taskAssetCenterService) resolveCompletedUploadMeta(
	ctx context.Context,
	request *domain.UploadRequest,
	checksumHint string,
	ossObjectKey string,
	ossDirectFinalized bool,
) (*RemoteFileMeta, *domain.AppError) {
	if request == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session is required", nil)
	}
	if ossDirectFinalized {
		return buildOSSDirectCompletedMeta(request, checksumHint, ossObjectKey, s.nowFn().UTC()), nil
	}
	if s.shouldFinalizeWithoutRemoteComplete(request) {
		if request.SessionStatus != domain.DesignAssetSessionStatusCompleted {
			// Fallback to backend-driven remote complete so MAIN complete
			// does not depend on browser-side remote-complete reachability.
			meta, err := s.uploadClient.CompleteUploadSession(ctx, RemoteCompleteUploadRequest{
				RemoteUploadID: request.RemoteUploadID,
				Filename:       request.FileName,
				ExpectedSize:   request.ExpectedSize,
				MimeType:       request.MimeType,
				ChecksumHint:   checksumHint,
			})
			if err != nil {
				return nil, infraError("complete multipart upload session via upload service client", err)
			}
			return meta, nil
		}
		meta, err := s.uploadClient.GetFileMeta(ctx, RemoteGetFileMetaRequest{
			RemoteUploadID: request.RemoteUploadID,
			RemoteFileID:   request.RemoteFileID,
			Filename:       request.FileName,
			ExpectedSize:   request.ExpectedSize,
			MimeType:       request.MimeType,
			ChecksumHint:   checksumHint,
		})
		if err != nil {
			return nil, infraError("get completed multipart file meta via upload service client", err)
		}
		return meta, nil
	}
	meta, err := s.uploadClient.CompleteUploadSession(ctx, RemoteCompleteUploadRequest{
		RemoteUploadID: request.RemoteUploadID,
		Filename:       request.FileName,
		ExpectedSize:   request.ExpectedSize,
		MimeType:       request.MimeType,
		ChecksumHint:   checksumHint,
	})
	if err != nil {
		return nil, infraError("complete upload session via upload service client", err)
	}
	return meta, nil
}

func buildOSSDirectCompletedMeta(request *domain.UploadRequest, checksumHint, objectKey string, uploadedAt time.Time) *RemoteFileMeta {
	meta := &RemoteFileMeta{
		StorageKey: strings.TrimSpace(objectKey),
		UploadedAt: uploadedAt,
	}
	if request != nil {
		meta.MimeType = strings.TrimSpace(request.MimeType)
		meta.FileSize = request.ExpectedSize
	}
	if hash := strings.TrimSpace(checksumHint); hash != "" {
		meta.FileHash = optionalStringPtr(hash)
	}
	return meta
}

func (s *taskAssetCenterService) shouldFinalizeWithoutRemoteComplete(request *domain.UploadRequest) bool {
	return request != nil && request.UploadMode == domain.DesignAssetUploadModeMultipart
}

func (s *taskAssetCenterService) buildCompletedUploadSessionResult(ctx context.Context, taskID int64, request *domain.UploadRequest) (*CompleteTaskAssetUploadSessionResult, *domain.AppError) {
	if request == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session is required", nil)
	}
	if request.BoundAssetID == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload_session already completed without bound asset version", nil)
	}
	versionRecord, err := s.taskAssetRepo.GetByID(ctx, *request.BoundAssetID)
	if err != nil {
		return nil, infraError("get completed asset version", err)
	}
	if versionRecord == nil || versionRecord.TaskID != taskID || versionRecord.AssetID == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload_session bound asset version is missing", map[string]interface{}{
			"upload_session_id": request.RequestID,
			"bound_asset_id":    request.BoundAssetID,
		})
	}
	session := domain.BuildUploadSession(request)
	asset, err := s.designAssetRepo.GetByID(ctx, *versionRecord.AssetID)
	if err != nil {
		return nil, infraError("get completed design asset", err)
	}
	if asset == nil || asset.TaskID != taskID {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload_session completed design asset is missing", map[string]interface{}{
			"upload_session_id": request.RequestID,
			"asset_id":          versionRecord.AssetID,
		})
	}
	task, appErr := s.requireTask(ctx, taskID)
	if appErr != nil {
		return nil, appErr
	}
	if err := s.hydrateDesignAssetReadModel(ctx, task, asset); err != nil {
		return nil, infraError("hydrate completed design asset", err)
	}
	version := domain.BuildDesignAssetVersion(versionRecord)
	if version != nil {
		s.applyDesignAssetVersionDerivedFields(task, asset, version)
		enrichDesignAssetVersionUploaderNames(ctx, s.userDisplayNameResolver, []*domain.DesignAssetVersion{version})
		s.applyDesignAssetVersionRoles(task, asset, []*domain.DesignAssetVersion{version})
	}
	return &CompleteTaskAssetUploadSessionResult{
		Session: session,
		Asset:   asset,
		Version: version,
	}, nil
}

func (s *taskAssetCenterService) CancelUploadSession(ctx context.Context, params CancelTaskAssetUploadSessionParams) (*domain.UploadSession, *domain.AppError) {
	task, appErr := s.requireTask(ctx, params.TaskID)
	if appErr != nil {
		return nil, appErr
	}
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionAssetUploadSessionCancel, task, "", "")
	authz.logDecision(TaskActionAssetUploadSessionCancel, decision)
	if !decision.Allowed {
		return nil, taskActionDecisionAppError(TaskActionAssetUploadSessionCancel, decision)
	}
	request, appErr := s.requireUploadRequest(ctx, params.TaskID, params.SessionID)
	if appErr != nil {
		return nil, appErr
	}
	if request.SessionStatus == domain.DesignAssetSessionStatusCompleted {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "completed upload_session cannot be cancelled", nil)
	}
	if request.SessionStatus == domain.DesignAssetSessionStatusCancelled {
		return domain.BuildUploadSession(request), nil
	}
	if err := s.uploadClient.AbortUploadSession(ctx, RemoteAbortUploadRequest{RemoteUploadID: request.RemoteUploadID}); err != nil {
		return nil, infraError("abort upload session via upload service client", err)
	}
	lastSyncedAt := s.nowFn().UTC()
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.uploadRequestRepo.UpdateLifecycle(ctx, tx, repo.UploadRequestLifecycleUpdate{
			RequestID: request.RequestID,
			Status:    domain.UploadRequestStatusCancelled,
			Remark:    strings.TrimSpace(params.Remark),
		}); err != nil {
			return err
		}
		if err := s.uploadRequestRepo.UpdateSession(ctx, tx, repo.UploadRequestSessionUpdate{
			RequestID:      request.RequestID,
			AssetID:        request.AssetID,
			SessionStatus:  domain.DesignAssetSessionStatusCancelled,
			RemoteUploadID: request.RemoteUploadID,
			RemoteFileID:   optionalStringPtr(request.RemoteFileID),
			LastSyncedAt:   &lastSyncedAt,
			Remark:         strings.TrimSpace(params.Remark),
		}); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventAssetUploadSessionCancelled, &params.CancelledBy, map[string]interface{}{
			"upload_session_id": request.RequestID,
			"upload_mode":       string(request.UploadMode),
			"remote_upload_id":  request.RemoteUploadID,
			"remark":            strings.TrimSpace(params.Remark),
		})
		return err
	})
	if txErr != nil {
		return nil, infraError("cancel upload session", txErr)
	}
	return s.GetUploadSession(ctx, params.TaskID, request.RequestID)
}

func (s *taskAssetCenterService) createUploadSession(ctx context.Context, params CreateTaskAssetUploadSessionParams, mode domain.DesignAssetUploadMode) (*CreateTaskAssetUploadSessionResult, *domain.AppError) {
	normalizedAssetType, appErr := normalizeRequestedUploadAssetType(params.AssetType, mode)
	if appErr != nil {
		return nil, appErr
	}
	params.AssetType = normalizedAssetType
	if err := validateTaskAssetType(params.AssetType); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Filename) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "filename is required", nil)
	}
	if params.ExpectedSize != nil && *params.ExpectedSize < 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "expected_size must be greater than or equal to zero", nil)
	}
	task, appErr := s.requireTask(ctx, params.TaskID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.requireScopedBatchAsset(ctx, params.TaskID, normalizedAssetType, params.TargetSKUCode); appErr != nil {
		return nil, appErr
	}
	targetSKUCode, appErr := s.resolveTargetSKUCode(ctx, params.TaskID, params.TargetSKUCode)
	if appErr != nil {
		return nil, appErr
	}
	params.TargetSKUCode = targetSKUCode
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionAssetUploadSessionCreate, task, "", "")
	authz.logDecision(TaskActionAssetUploadSessionCreate, decision)
	if !decision.Allowed {
		return nil, taskActionDecisionAppError(TaskActionAssetUploadSessionCreate, decision)
	}
	taskRef := strings.TrimSpace(task.TaskNo)
	identity, appErr := s.freezeUploadAssetIdentity(ctx, params.TaskID, params.AssetID, params.SourceAssetID, params.TargetSKUCode, params.AssetType, params.CreatedBy)
	if appErr != nil {
		return nil, appErr
	}
	params.AssetID = &identity.AssetID
	versionNo, appErr := s.nextPendingAssetVersionNo(ctx, identity.AssetID)
	if appErr != nil {
		return nil, appErr
	}
	if mode != domain.DesignAssetUploadModeMultipart {
		if params.AssetType.IsDelivery() || params.AssetType.IsSource() || params.AssetType.IsPreview() || params.AssetType.IsDesignThumb() {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "delivery/source/preview assets must use multipart upload mode", nil)
		}
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "assets must use multipart upload mode", nil)
	}
	if params.SourceAssetID != nil {
		if !(params.AssetType.IsPreview() || params.AssetType.IsDesignThumb()) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "source_asset_id is only allowed for preview or design_thumb assets", nil)
		}
		sourceAsset, appErr := s.requireDesignAsset(ctx, params.TaskID, *params.SourceAssetID)
		if appErr != nil {
			return nil, appErr
		}
		if !sourceAsset.AssetType.IsSource() {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "source_asset_id must point to source asset", map[string]interface{}{
				"source_asset_id": *params.SourceAssetID,
				"asset_type":      sourceAsset.AssetType,
			})
		}
	}

	createReq := RemoteCreateUploadSessionRequest{
		TaskID:       params.TaskID,
		TaskRef:      taskRef,
		AssetID:      params.AssetID,
		AssetNo:      identity.AssetNo,
		AssetType:    params.AssetType,
		VersionNo:    versionNo,
		UploadMode:   mode,
		Filename:     strings.TrimSpace(params.Filename),
		ExpectedSize: params.ExpectedSize,
		MimeType:     normalizeRequiredUploadContentType(params.MimeType),
		CreatedBy:    params.CreatedBy,
	}

	remote, err := s.uploadClient.CreateUploadSession(ctx, createReq)
	if err != nil {
		return nil, infraError("create upload session via upload service client", err)
	}

	now := s.nowFn().UTC()
	requiredContentType := normalizeRequiredUploadContentType(params.MimeType)
	request := &domain.UploadRequest{
		OwnerType:       domain.AssetOwnerTypeTask,
		OwnerID:         params.TaskID,
		TaskID:          params.TaskID,
		AssetID:         params.AssetID,
		SourceAssetID:   params.SourceAssetID,
		TargetSKUCode:   params.TargetSKUCode,
		TaskAssetType:   &params.AssetType,
		StorageAdapter:  domain.AssetStorageAdapterOSSUploadService,
		UploadMode:      mode,
		RefType:         domain.AssetStorageRefTypeTaskAssetObject,
		FileName:        strings.TrimSpace(params.Filename),
		MimeType:        requiredContentType,
		FileSize:        params.ExpectedSize,
		ExpectedSize:    params.ExpectedSize,
		ChecksumHint:    strings.TrimSpace(params.FileHash),
		Status:          domain.UploadRequestStatusRequested,
		StorageProvider: domain.DesignAssetStorageProviderOSS,
		SessionStatus:   domain.DesignAssetSessionStatusCreated,
		RemoteUploadID:  remote.UploadID,
		RemoteFileID:    valueOrEmpty(remote.FileID),
		IsPlaceholder:   remote.IsStub,
		CreatedBy:       params.CreatedBy,
		ExpiresAt:       remote.ExpiresAt,
		LastSyncedAt:    firstNonNilTime(remote.LastSyncedAt, &now),
		Remark:          strings.TrimSpace(params.Remark),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		created, err := s.uploadRequestRepo.Create(ctx, tx, request)
		if err != nil {
			return err
		}
		request = created
		_, err = s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventAssetUploadSessionCreated, &params.CreatedBy, map[string]interface{}{
			"upload_session_id": request.RequestID,
			"asset_id":          params.AssetID,
			"asset_type":        string(params.AssetType),
			"target_sku_code":   params.TargetSKUCode,
			"filename":          request.FileName,
			"expected_size":     request.ExpectedSize,
			"mime_type":         request.MimeType,
			"upload_mode":       string(mode),
			"storage_provider":  string(request.StorageProvider),
			"remote_upload_id":  request.RemoteUploadID,
			"expires_at":        request.ExpiresAt,
		})
		return err
	})
	if txErr != nil {
		return nil, infraError("create upload session", txErr)
	}
	result := &CreateTaskAssetUploadSessionResult{
		Session: domain.BuildUploadSession(request),
		Remote:  remote,
	}

	if s.ossDirectService != nil && s.ossDirectService.Enabled() {
		objectKey := s.ossDirectService.BuildObjectKey(taskRef, identity.AssetNo, versionNo, params.AssetType, strings.TrimSpace(params.Filename))
		fileSize := int64(0)
		if params.ExpectedSize != nil {
			fileSize = *params.ExpectedSize
		}
		contentType := requiredContentType
		if mode == domain.DesignAssetUploadModeMultipart {
			ossPlan, ossErr := s.ossDirectService.CreateMultipartUploadPlan(ctx, objectKey, fileSize, contentType)
			if ossErr != nil {
				log.Printf("oss_direct_upload_plan_fallback error=%v session=%s", ossErr, request.RequestID)
			} else {
				result.OSSDirect = ossPlan
			}
		} else {
			ossPlan, ossErr := s.ossDirectService.CreateSingleUploadPlan(objectKey, contentType)
			if ossErr != nil {
				log.Printf("oss_direct_upload_plan_fallback error=%v session=%s", ossErr, request.RequestID)
			} else {
				result.OSSDirect = ossPlan
			}
		}
	}

	return result, nil
}

func (s *taskAssetCenterService) syncUploadRequestFromRemote(ctx context.Context, request *domain.UploadRequest) (*domain.UploadRequest, error) {
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
			AssetID:        request.AssetID,
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

func buildRemoteStorageKey(meta *RemoteFileMeta, request *domain.UploadRequest) string {
	if meta != nil && strings.TrimSpace(meta.StorageKey) != "" {
		return strings.TrimSpace(meta.StorageKey)
	}
	if meta != nil && meta.FileID != nil && strings.TrimSpace(*meta.FileID) != "" {
		return strings.TrimSpace(*meta.FileID)
	}
	if request != nil && strings.TrimSpace(request.RemoteFileID) != "" {
		return strings.TrimSpace(request.RemoteFileID)
	}
	if request != nil && strings.TrimSpace(request.RemoteUploadID) != "" {
		return strings.TrimSpace(request.RemoteUploadID)
	}
	return ""
}

func firstNonNilTime(values ...*time.Time) *time.Time {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func buildAssetDownloadInfoWithOSS(version *domain.DesignAssetVersion, uploadClient UploadServiceClient, ossDirect *OSSDirectService) *domain.AssetDownloadInfo {
	return buildOSSOrFallback(version, uploadClient, ossDirect, false)
}

func buildAssetPreviewInfoWithOSS(version *domain.DesignAssetVersion, uploadClient UploadServiceClient, ossDirect *OSSDirectService) *domain.AssetDownloadInfo {
	return buildOSSOrFallback(version, uploadClient, ossDirect, true)
}

func buildOSSOrFallback(version *domain.DesignAssetVersion, uploadClient UploadServiceClient, ossDirect *OSSDirectService, preview bool) *domain.AssetDownloadInfo {
	if version == nil {
		return nil
	}
	if ossDirect != nil && ossDirect.Enabled() && strings.TrimSpace(version.StorageKey) != "" {
		key := strings.TrimSpace(version.StorageKey)
		var info *OSSDirectDownloadInfo
		mimeType := version.MimeType
		if preview {
			if process, ok := buildOSSIMGPreviewProcessForSource(version); ok {
				info = ossDirect.PresignPreviewURLWithProcess(key, process)
				if strings.Contains(process, "format,jpg") {
					mimeType = "image/jpeg"
				}
			} else {
				info = ossDirect.PresignPreviewURL(key)
			}
		} else {
			info = ossDirect.PresignDownloadURL(key)
		}
		if info != nil && strings.TrimSpace(info.DownloadURL) != "" {
			downloadURL := info.DownloadURL
			fileSize := int64(0)
			if version.FileSize != nil {
				fileSize = *version.FileSize
			}
			return &domain.AssetDownloadInfo{
				DownloadMode:     domain.AssetDownloadModeDirect,
				DownloadURL:      &downloadURL,
				AccessHint:       "oss_presigned",
				PreviewAvailable: version.PreviewAvailable,
				Filename:         version.OriginalFilename,
				FileSize:         fileSize,
				MimeType:         mimeType,
				ExpiresAt:        &info.ExpiresAt,
			}
		}
	}
	return buildAssetDownloadInfo(version, uploadClient)
}

func buildAssetDownloadInfo(version *domain.DesignAssetVersion, uploadClient UploadServiceClient) *domain.AssetDownloadInfo {
	if version == nil {
		return nil
	}
	downloadMode := domain.AssetDownloadModeProxy
	downloadURL := version.DownloadURL
	if uploadClient != nil {
		if directURL := uploadClient.BuildBrowserFileURL(version.StorageKey); directURL != nil && strings.TrimSpace(*directURL) != "" {
			urlValue := strings.TrimSpace(*directURL)
			if isDirectBrowserURL(urlValue) {
				downloadMode = domain.AssetDownloadModeDirect
				downloadURL = directURL
			}
		}
	}
	if downloadMode != domain.AssetDownloadModeDirect && downloadURL != nil {
		urlValue := strings.TrimSpace(*downloadURL)
		if strings.HasPrefix(urlValue, "http://") || strings.HasPrefix(urlValue, "https://") {
			downloadMode = domain.AssetDownloadModeDirect
		}
	}
	fileSize := int64(0)
	if version.FileSize != nil {
		fileSize = *version.FileSize
	}
	return &domain.AssetDownloadInfo{
		DownloadMode:     downloadMode,
		DownloadURL:      downloadURL,
		AccessHint:       version.AccessHint,
		PreviewAvailable: version.PreviewAvailable,
		Filename:         version.OriginalFilename,
		FileSize:         fileSize,
		MimeType:         version.MimeType,
	}
}

func isDirectBrowserURL(urlValue string) bool {
	urlValue = strings.TrimSpace(urlValue)
	if urlValue == "" {
		return false
	}
	if strings.HasPrefix(urlValue, "/v1/assets/files/") || strings.HasPrefix(urlValue, "/files/") {
		return false
	}
	return strings.HasPrefix(urlValue, "http://") || strings.HasPrefix(urlValue, "https://") || strings.HasPrefix(urlValue, "/")
}

func allowPostTransitionUploadSessionComplete(
	ctx context.Context,
	authz *taskActionAuthorizer,
	decision TaskActionDecision,
	task *domain.Task,
	request *domain.UploadRequest,
) bool {
	if strings.TrimSpace(decision.DenyCode) != "task_status_not_actionable" {
		return false
	}
	if task == nil || task.TaskStatus != domain.TaskStatusPendingAuditA {
		return false
	}
	if !isPrecreatedCompletableUploadSession(request) {
		return false
	}
	shadowTask := *task
	shadowTask.TaskStatus = domain.TaskStatusInProgress
	shadowDecision := authz.EvaluateTaskActionPolicy(ctx, TaskActionAssetUploadSessionComplete, &shadowTask, "", "")
	return shadowDecision.Allowed
}

func isPrecreatedCompletableUploadSession(request *domain.UploadRequest) bool {
	if request == nil {
		return false
	}
	if request.TaskAssetType == nil {
		return false
	}
	assetType := domain.NormalizeTaskAssetType(*request.TaskAssetType)
	if !assetType.IsSource() && !assetType.IsDelivery() {
		return false
	}
	if request.SessionStatus == domain.DesignAssetSessionStatusCancelled || request.SessionStatus == domain.DesignAssetSessionStatusExpired {
		return false
	}
	switch request.Status {
	case domain.UploadRequestStatusRequested, domain.UploadRequestStatusBound:
		return true
	default:
		return false
	}
}

func validateUploadContentTypeContract(request *domain.UploadRequest, actualContentType string) *domain.AppError {
	if request == nil {
		return nil
	}
	actualContentType = strings.TrimSpace(actualContentType)
	if actualContentType == "" {
		return nil
	}
	expectedContentType := normalizeRequiredUploadContentType(request.MimeType)
	if actualContentType == expectedContentType {
		return nil
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_content_type must match upload_session required content type", map[string]interface{}{
		"upload_session_id":            request.RequestID,
		"required_upload_content_type": expectedContentType,
		"upload_content_type":          actualContentType,
	})
}

func validateOSSDirectCompleteContract(params CompleteTaskAssetUploadSessionParams) *domain.AppError {
	hasParts := len(params.OSSParts) > 0
	hasUploadID := strings.TrimSpace(params.OSSUploadID) != ""
	hasObjectKey := strings.TrimSpace(params.OSSObjectKey) != ""
	if !hasParts && !hasUploadID && !hasObjectKey {
		return nil
	}
	if hasParts && hasUploadID && hasObjectKey {
		return nil
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "oss direct complete requires oss_parts, oss_upload_id, and oss_object_key together", map[string]interface{}{
		"has_oss_parts":      hasParts,
		"has_oss_upload_id":  hasUploadID,
		"has_oss_object_key": hasObjectKey,
	})
}

func (s *taskAssetCenterService) canFinalizeOSSDirectUpload(params CompleteTaskAssetUploadSessionParams) bool {
	return s.ossDirectService != nil &&
		s.ossDirectService.Enabled() &&
		len(params.OSSParts) > 0 &&
		strings.TrimSpace(params.OSSUploadID) != "" &&
		strings.TrimSpace(params.OSSObjectKey) != ""
}

func validateAssetVersionObjectAvailable(version *domain.DesignAssetVersion) *domain.AppError {
	if version == nil {
		return nil
	}
	if version.StorageRefStatus == domain.AssetStorageRefStatusArchived {
		return domain.ErrAssetMissing
	}
	return nil
}

func (s *taskAssetCenterService) repairMissingObjectStorageRef(ctx context.Context, versionID int64) (bool, *domain.AppError) {
	versionRecord, err := s.taskAssetRepo.GetByID(ctx, versionID)
	if err != nil {
		return false, infraError("get asset version for storage repair", err)
	}
	if versionRecord == nil {
		return false, domain.ErrNotFound
	}
	storageRefID := ""
	if versionRecord.StorageRefID != nil {
		storageRefID = strings.TrimSpace(*versionRecord.StorageRefID)
	}
	if storageRefID == "" {
		return false, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "asset version storage_ref_id is required for repair", map[string]interface{}{
			"asset_version_id": versionID,
		})
	}
	storageKey := ""
	if versionRecord.StorageKey != nil {
		storageKey = strings.TrimSpace(*versionRecord.StorageKey)
	}
	if storageKey == "" && versionRecord.StorageRef != nil {
		storageKey = strings.TrimSpace(versionRecord.StorageRef.RefKey)
	}
	if storageKey == "" {
		return false, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "asset version storage_key is required for repair", map[string]interface{}{
			"asset_version_id": versionID,
			"storage_ref_id":   storageRefID,
		})
	}
	ref := versionRecord.StorageRef
	if ref == nil {
		ref, err = s.assetStorageRefRepo.GetByRefID(ctx, storageRefID)
		if err != nil {
			return false, infraError("get asset storage ref for repair", err)
		}
	}
	if ref == nil {
		return false, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "asset storage ref is missing", map[string]interface{}{
			"asset_version_id": versionID,
			"storage_ref_id":   storageRefID,
		})
	}
	if ref.Status == domain.AssetStorageRefStatusArchived {
		return false, nil
	}
	if s.uploadClient == nil {
		return false, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload service probe client is not configured", nil)
	}
	probe, err := s.uploadClient.ProbeStoredFile(ctx, RemoteProbeStoredFileRequest{StorageKey: storageKey})
	if !storedFileProbeMissing(probe, err) {
		if err != nil {
			return false, infraError("probe stored asset object", err)
		}
		return false, nil
	}
	if txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.assetStorageRefRepo.UpdateStatus(ctx, tx, storageRefID, domain.AssetStorageRefStatusArchived)
	}); txErr != nil {
		if appErr, ok := txErr.(*domain.AppError); ok {
			return false, appErr
		}
		return false, infraError("archive missing asset storage ref", txErr)
	}
	ref.Status = domain.AssetStorageRefStatusArchived
	if versionRecord.StorageRef != nil {
		versionRecord.StorageRef.Status = domain.AssetStorageRefStatusArchived
	}
	return true, nil
}

func storedFileProbeMissing(probe *RemoteStoredFileProbe, err error) bool {
	if probe != nil && probe.StatusCode == http.StatusNotFound {
		return true
	}
	if err == nil {
		return false
	}
	var httpErr *UploadServiceHTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound
}

func inferTaskAssetUploadMode(assetType domain.TaskAssetType) (domain.DesignAssetUploadMode, *domain.AppError) {
	normalized := domain.NormalizeTaskAssetType(assetType)
	if normalized == "" {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_type is required", nil)
	}
	return domain.DesignAssetUploadModeMultipart, nil
}

func matchesAssetResourceFilters(asset *domain.DesignAsset, params ListAssetResourcesParams) bool {
	if asset == nil {
		return false
	}
	if params.ArchiveStatus.Valid() && asset.ArchiveStatus != params.ArchiveStatus {
		return false
	}
	if params.UploadStatus.Valid() && asset.UploadStatus != params.UploadStatus {
		return false
	}
	return true
}

func (s *taskAssetCenterService) loadAssetResource(ctx context.Context, asset *domain.DesignAsset) (*domain.DesignAsset, *domain.AppError) {
	if asset == nil {
		return nil, domain.ErrNotFound
	}
	task, appErr := s.requireTask(ctx, asset.TaskID)
	if appErr != nil {
		return nil, appErr
	}
	if err := s.hydrateDesignAssetReadModel(ctx, task, asset); err != nil {
		return nil, infraError("hydrate design asset resource", err)
	}
	s.applyDesignAssetResourceSummary(asset)
	return asset, nil
}

func (s *taskAssetCenterService) applyDesignAssetResourceSummary(asset *domain.DesignAsset) {
	if asset == nil {
		return
	}
	asset.ArchiveStatus = domain.AssetArchiveStatusActive
	asset.UploadStatus = domain.DesignAssetUploadStatusPending
	if asset.CurrentVersion != nil && asset.CurrentVersion.UploadStatus.Valid() {
		asset.UploadStatus = asset.CurrentVersion.UploadStatus
	}
}

func (s *taskAssetCenterService) requireTask(ctx context.Context, taskID int64) (*domain.Task, *domain.AppError) {
	if taskID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_id must be greater than zero", nil)
	}
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	return task, nil
}

func (s *taskAssetCenterService) requireDesignAssetByID(ctx context.Context, assetID int64) (*domain.DesignAsset, *domain.AppError) {
	if assetID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_id must be greater than zero", nil)
	}
	asset, err := s.designAssetRepo.GetByID(ctx, assetID)
	if err != nil {
		return nil, infraError("get design asset", err)
	}
	if asset == nil {
		return nil, domain.ErrNotFound
	}
	return asset, nil
}

func (s *taskAssetCenterService) requireDesignAsset(ctx context.Context, taskID, assetID int64) (*domain.DesignAsset, *domain.AppError) {
	if assetID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_id must be greater than zero", nil)
	}
	asset, err := s.designAssetRepo.GetByID(ctx, assetID)
	if err != nil {
		return nil, infraError("get design asset", err)
	}
	if asset == nil || asset.TaskID != taskID {
		return nil, domain.ErrNotFound
	}
	return asset, nil
}

func (s *taskAssetCenterService) requireUploadRequestByID(ctx context.Context, sessionID string) (*domain.UploadRequest, *domain.AppError) {
	request, err := s.uploadRequestRepo.GetByRequestID(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, infraError("get upload request", err)
	}
	if request == nil {
		return nil, domain.ErrNotFound
	}
	if request.TaskID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session does not have a bound task context", map[string]interface{}{
			"upload_session_id": strings.TrimSpace(sessionID),
		})
	}
	return request, nil
}

func (s *taskAssetCenterService) requireUploadRequest(ctx context.Context, taskID int64, sessionID string) (*domain.UploadRequest, *domain.AppError) {
	request, err := s.uploadRequestRepo.GetByRequestID(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, infraError("get upload request", err)
	}
	if request == nil {
		return nil, domain.ErrNotFound
	}
	if request.TaskID != taskID && !(request.OwnerType == domain.AssetOwnerTypeTask && request.OwnerID == taskID) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session does not belong to current task", nil)
	}
	return request, nil
}

func (s *taskAssetCenterService) resolveTargetSKUCode(ctx context.Context, taskID int64, targetSKUCode string) (string, *domain.AppError) {
	targetSKUCode = strings.TrimSpace(targetSKUCode)
	if targetSKUCode == "" {
		return "", nil
	}
	items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, taskID)
	if err != nil {
		return "", infraError("list task sku items for upload scope", err)
	}
	for _, item := range items {
		if item != nil && strings.TrimSpace(item.SKUCode) == targetSKUCode {
			return targetSKUCode, nil
		}
	}
	return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "target_sku_code must belong to the task", map[string]interface{}{
		"target_sku_code": targetSKUCode,
		"task_id":         taskID,
	})
}

type frozenUploadAssetIdentity struct {
	AssetID int64
	AssetNo string
}

func (s *taskAssetCenterService) freezeUploadAssetIdentity(
	ctx context.Context,
	taskID int64,
	requestedAssetID *int64,
	sourceAssetID *int64,
	targetSKUCode string,
	assetType domain.TaskAssetType,
	createdBy int64,
) (*frozenUploadAssetIdentity, *domain.AppError) {
	if requestedAssetID != nil {
		asset, appErr := s.requireDesignAsset(ctx, taskID, *requestedAssetID)
		if appErr != nil {
			return nil, appErr
		}
		if asset.AssetType != assetType {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_type does not match existing asset", nil)
		}
		if sourceAssetID != nil {
			if asset.SourceAssetID == nil || *asset.SourceAssetID != *sourceAssetID {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "source_asset_id does not match existing asset linkage", map[string]interface{}{
					"asset_id":               asset.ID,
					"source_asset_id":        sourceAssetID,
					"existing_source":        asset.SourceAssetID,
					"upload_session_task_id": taskID,
				})
			}
		}
		if strings.TrimSpace(asset.ScopeSKUCode) != strings.TrimSpace(targetSKUCode) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "target_sku_code does not match existing asset scope", map[string]interface{}{
				"target_sku_code": targetSKUCode,
				"asset_scope_sku": asset.ScopeSKUCode,
				"asset_id":        asset.ID,
			})
		}
		return &frozenUploadAssetIdentity{
			AssetID: asset.ID,
			AssetNo: strings.TrimSpace(asset.AssetNo),
		}, nil
	}

	var identity frozenUploadAssetIdentity
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		assetNo, err := s.designAssetRepo.NextAssetNo(ctx, tx, taskID)
		if err != nil {
			return err
		}
		asset := &domain.DesignAsset{
			TaskID:        taskID,
			AssetNo:       assetNo,
			SourceAssetID: sourceAssetID,
			ScopeSKUCode:  strings.TrimSpace(targetSKUCode),
			AssetType:     assetType,
			CreatedBy:     createdBy,
		}
		assetID, err := s.designAssetRepo.Create(ctx, tx, asset)
		if err != nil {
			return err
		}
		identity = frozenUploadAssetIdentity{
			AssetID: assetID,
			AssetNo: strings.TrimSpace(assetNo),
		}
		return nil
	})
	if txErr != nil {
		if appErr, ok := txErr.(*domain.AppError); ok {
			return nil, appErr
		}
		return nil, infraError("freeze upload asset identity", txErr)
	}
	return &identity, nil
}

func (s *taskAssetCenterService) nextPendingAssetVersionNo(ctx context.Context, assetID int64) (int, *domain.AppError) {
	records, err := s.taskAssetRepo.ListByAssetID(ctx, assetID)
	if err != nil {
		return 0, infraError("list design asset versions for upload identity", err)
	}
	return len(records) + 1, nil
}
