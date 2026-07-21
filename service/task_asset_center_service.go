package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"workflow/domain"
	"workflow/repo"
)

type CreateTaskAssetUploadSessionParams struct {
	TaskID               int64
	AssetID              *int64
	SourceAssetID        *int64
	CreatedBy            int64
	AssetType            domain.TaskAssetType
	Filename             string
	ExpectedSize         *int64
	MimeType             string
	FileHash             string
	Remark               string
	TargetSKUCode        string
	OwnerModuleKey       string
	UploadPolicy         string
	RetouchRequirementID *int64
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
	TaskID       int64
	SessionID    string
	CancelledBy  int64
	Remark       string
	OSSUploadID  string
	OSSObjectKey string
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
	BuildTaskReferenceBatchDownloadManifest(ctx context.Context, taskID int64, actorID int64) (*TaskReferenceBatchDownloadManifest, *domain.AppError)
	EnsureDerivedPreviewAssets(ctx context.Context, taskID, sourceAssetID, actorID int64) *domain.AppError
}

type taskAssetCenterService struct {
	taskRepo                  repo.TaskRepo
	designAssetRepo           repo.DesignAssetRepo
	taskAssetRepo             repo.TaskAssetRepo
	uploadRequestRepo         repo.UploadRequestRepo
	assetStorageRefRepo       repo.AssetStorageRefRepo
	taskEventRepo             repo.TaskEventRepo
	auditV7Repo               repo.AuditV7Repo
	taskModuleRepo            repo.TaskModuleRepo
	customizationJobRepo      repo.CustomizationJobRepo
	txRunner                  repo.TxRunner
	uploadClient              UploadServiceClient
	ossDirectService          *OSSDirectService
	nowFn                     func() time.Time
	runAsyncFn                func(func())
	derivedPreviewGracePeriod time.Duration
	previewRenderer           AssetPreviewRenderer
	derivedPreviewSlots       chan struct{}
	derivedPreviewMu          sync.Mutex
	derivedPreviewInflight    map[string]struct{}
	userDisplayNameResolver   UserDisplayNameResolver
	retouchRequirementRepo    repo.TaskRetouchRequirementRepo
	referenceFileRefFlatRepo  repo.ReferenceFileRefFlatRepo
}

const (
	taskAssetVersionUniqueKey        = "uq_task_assets_task_version"
	assetVersionRaceRetryDenyCode    = "asset_version_race_retry"
	assetVersionReplacementRetention = 15 * 24 * time.Hour
	taskAssetStagingRetention        = 7 * 24 * time.Hour
	taskAssetUploadMaxFileSizeBytes  = int64(1024 * 1024 * 1024)
	taskAssetUploadMaxFileSizeLabel  = "1GB"
	taskAssetSinglePartThreshold     = int64(10 * 1024 * 1024)
)

type taskAssetVersionSupersedeRepo interface {
	MarkAssetVersionSuperseded(ctx context.Context, tx repo.Tx, versionID, supersededByVersionID int64, supersededAt, cleanupAfterAt time.Time) error
}

type taskAssetBindingStagingRepo interface {
	MarkBindingStaged(ctx context.Context, tx repo.Tx, taskAssetID, taskID, actorID int64, scopeSKUCode string, retouchRequirementID *int64, role, uploadSessionID string, expiresAt time.Time) error
}

type stagedTaskAssetPreviewAccessRepo interface {
	GetStagedPreviewAccessByDesignAssetID(ctx context.Context, assetID int64) (*domain.StagedTaskAssetPreviewAccess, error)
}

type uploadRequestForUpdateRepo interface {
	GetByRequestIDForUpdate(ctx context.Context, tx repo.Tx, requestID string) (*domain.UploadRequest, error)
}

type designAssetForUpdateRepo interface {
	GetByIDForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.DesignAsset, error)
}

type taskForUpdateRepo interface {
	GetByIDForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.Task, error)
}

type taskModuleForUpdateRepo interface {
	GetByTaskAndKeyForUpdate(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string) (*domain.TaskModule, error)
}

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
		previewRenderer:           NewExternalAssetPreviewRenderer(),
		derivedPreviewSlots:       make(chan struct{}, 2),
		derivedPreviewInflight:    make(map[string]struct{}),
	}
	for _, opt := range options {
		opt(svc)
	}
	return svc
}

func WithTaskAssetCenterPreviewRenderer(renderer AssetPreviewRenderer) TaskAssetCenterServiceOption {
	return func(s *taskAssetCenterService) {
		s.previewRenderer = renderer
	}
}

func WithOSSDirectService(ossDirect *OSSDirectService) func(*taskAssetCenterService) {
	return func(s *taskAssetCenterService) {
		s.ossDirectService = ossDirect
	}
}

func WithTaskAssetCenterModuleRepo(moduleRepo repo.TaskModuleRepo) TaskAssetCenterServiceOption {
	return func(s *taskAssetCenterService) {
		s.taskModuleRepo = moduleRepo
	}
}

func WithTaskAssetCenterCustomizationJobRepo(customizationJobRepo repo.CustomizationJobRepo) TaskAssetCenterServiceOption {
	return func(s *taskAssetCenterService) {
		s.customizationJobRepo = customizationJobRepo
	}
}

func WithTaskAssetCenterUserDisplayNameResolver(resolver UserDisplayNameResolver) TaskAssetCenterServiceOption {
	return func(s *taskAssetCenterService) {
		s.userDisplayNameResolver = resolver
	}
}

func WithTaskAssetCenterRetouchRequirementRepo(retouchRequirementRepo repo.TaskRetouchRequirementRepo) TaskAssetCenterServiceOption {
	return func(s *taskAssetCenterService) {
		s.retouchRequirementRepo = retouchRequirementRepo
	}
}

func WithTaskAssetCenterReferenceFileRefFlatRepo(referenceFileRefFlatRepo repo.ReferenceFileRefFlatRepo) TaskAssetCenterServiceOption {
	return func(s *taskAssetCenterService) {
		s.referenceFileRefFlatRepo = referenceFileRefFlatRepo
	}
}

func WithTaskAssetCenterAuditRepo(auditV7Repo repo.AuditV7Repo) TaskAssetCenterServiceOption {
	return func(s *taskAssetCenterService) {
		s.auditV7Repo = auditV7Repo
	}
}

func (s *taskAssetCenterService) taskActionAuthorizer() *taskActionAuthorizer {
	return newTaskActionAuthorizer()
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
	asset, appErr := s.requireDesignAssetByID(ctx, assetID)
	if appErr != nil {
		return nil, appErr
	}
	task, appErr := s.requireTask(ctx, asset.TaskID)
	if appErr != nil {
		return nil, appErr
	}

	var stagedAccess *domain.StagedTaskAssetPreviewAccess
	if accessRepo, ok := s.taskAssetRepo.(stagedTaskAssetPreviewAccessRepo); ok {
		var err error
		stagedAccess, err = accessRepo.GetStagedPreviewAccessByDesignAssetID(ctx, assetID)
		if err != nil {
			return nil, infraError("load staged asset preview access", err)
		}
	}
	if appErr := authorizeV8TaskAssetPreview(ctx, task, stagedAccess); appErr != nil {
		return nil, appErr
	}
	if stagedAccess != nil {
		record, err := s.taskAssetRepo.GetByID(ctx, stagedAccess.TaskAssetID)
		if err != nil {
			return nil, infraError("load staged asset preview version", err)
		}
		if record == nil || record.TaskID != task.ID || record.AssetID == nil || *record.AssetID != asset.ID {
			return nil, domain.ErrNotFound
		}
		asset.CurrentVersionID = &record.ID
		asset.CurrentVersion = domain.BuildDesignAssetVersion(record)
		if asset.CurrentVersion != nil {
			s.applyDesignAssetVersionDerivedFields(task, asset, asset.CurrentVersion)
		}
		s.applyDesignAssetResourceSummary(asset)
	} else {
		asset, appErr = s.loadAssetResource(ctx, asset)
		if appErr != nil {
			return nil, appErr
		}
	}
	if asset.CurrentVersion == nil {
		return nil, domain.ErrNotFound
	}
	if !asset.AssetType.IsPreview() && !asset.AssetType.IsDesignThumb() {
		info, resolveErr := s.resolveDerivedPreviewInfo(ctx, asset)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if info != nil {
			return info, nil
		}
		if isDerivedPreviewGenerationCandidate(asset.CurrentVersion) {
			actorID := asset.CurrentVersion.UploadedBy
			if actorID <= 0 {
				actorID = asset.CreatedBy
			}
			s.scheduleDerivedPreviewGeneration(asset.TaskID, asset.ID, actorID, asset.CurrentVersion)
		}
	}
	if !asset.CurrentVersion.PreviewAvailable {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "asset preview is not available", map[string]interface{}{
			"asset_id": asset.ID,
		})
	}
	if appErr := validateAssetVersionObjectAvailable(asset.CurrentVersion); appErr != nil {
		return nil, appErr
	}
	return buildAssetPreviewInfoWithOSS(asset.CurrentVersion, s.uploadClient, s.ossDirectService), nil
}

func authorizeV8TaskAssetPreview(ctx context.Context, task *domain.Task, staged *domain.StagedTaskAssetPreviewAccess) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || actor.EffectiveAccess == nil {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "asset preview requires explicit access", nil)
	}
	if task == nil || (staged != nil && staged.TaskID != task.ID) {
		return domain.ErrNotFound
	}
	if staged != nil {
		if (staged.StagedBy == actor.ID && actor.EffectiveAccess.Has(domain.PermissionAssetView)) ||
			domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAuditDecision, task.AccessSubject()) {
			return nil
		}
		return domain.NewAppError(domain.ErrCodePermissionDenied, "staged asset preview is limited to its uploader or an authorized auditor", map[string]interface{}{
			"deny_code": "staged_preview_scope_denied",
		})
	}
	if domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetView, task.AccessSubject()) {
		return nil
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "asset preview is outside the actor's explicit data scope", map[string]interface{}{
		"deny_code": "asset_preview_scope_denied",
	})
}

func (s *taskAssetCenterService) GetUploadSessionByID(ctx context.Context, sessionID string) (*domain.UploadSession, *domain.AppError) {
	request, appErr := s.requireUploadRequestByID(ctx, sessionID)
	if appErr != nil {
		return nil, appErr
	}
	return s.GetUploadSession(ctx, request.TaskID, request.RequestID)
}

func (s *taskAssetCenterService) CreateUploadSession(ctx context.Context, params CreateTaskAssetUploadSessionParams) (*CreateTaskAssetUploadSessionResult, *domain.AppError) {
	mode, appErr := s.inferTaskAssetUploadMode(params.AssetType, params.ExpectedSize)
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
	task, appErr := s.requireTask(ctx, taskID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := rejectCompletedTaskAssetMutation(task); appErr != nil {
		return nil, appErr
	}
	if appErr := authorizeV8TaskAssetSessionRead(ctx, task); appErr != nil {
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
			if appErr, ok := err.(*domain.AppError); ok {
				return nil, appErr
			}
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
	if appErr := rejectCompletedTaskAssetMutation(task); appErr != nil {
		return nil, appErr
	}
	if request.TaskAssetType == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session asset_type is required", nil)
	}
	if appErr := authorizeV8TaskAssetMutation(ctx, task, *request.TaskAssetType); appErr != nil {
		return nil, appErr
	}
	if request.Status == domain.UploadRequestStatusBound || (request.SessionStatus == domain.DesignAssetSessionStatusCompleted && request.BoundAssetID != nil) {
		return s.buildCompletedUploadSessionResult(ctx, params.TaskID, request)
	}
	if request.SessionStatus == domain.DesignAssetSessionStatusCancelled || request.SessionStatus == domain.DesignAssetSessionStatusExpired {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload_session is already terminal", nil)
	}
	if appErr := validateTaskStageUploadAssetType(task, *request.TaskAssetType, "", request.AssetID); appErr != nil {
		return nil, appErr
	}
	if request.ExpectedSize == nil || *request.ExpectedSize <= 0 || *request.ExpectedSize > taskAssetUploadMaxFileSizeBytes {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session expected_size is outside the allowed range", map[string]interface{}{
			"max_bytes": taskAssetUploadMaxFileSizeBytes,
		})
	}
	if appErr := validateUploadContentTypeContract(request, params.UploadContentType); appErr != nil {
		return nil, appErr
	}
	if appErr := validateOSSDirectCompleteContract(params); appErr != nil {
		return nil, appErr
	}
	requestAssetType := domain.NormalizeTaskAssetType(*request.TaskAssetType)
	scopeSKUCode := strings.TrimSpace(request.TargetSKUCode)
	retouchRequirementID := domain.CloneInt64Ptr(request.RetouchRequirementID)
	referenceSKUItemID, appErr := s.resolveReferenceSKUItemID(ctx, params.TaskID, requestAssetType, scopeSKUCode)
	if appErr != nil {
		return nil, appErr
	}

	checksumHint := firstNonEmpty(strings.TrimSpace(params.FileHash), strings.TrimSpace(request.ChecksumHint))
	var err error
	ossDirectReady := s.canFinalizeOSSDirectUpload(params)
	isOSSDirectSession := strings.TrimSpace(request.RemoteUploadID) == "" && s.ossDirectService != nil && s.ossDirectService.Enabled()
	if isOSSDirectSession && !ossDirectReady {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "oss_object_key is required for OSS direct upload_session completion", nil)
	}
	if request.RemoteUploadID != "" && request.SessionStatus == domain.DesignAssetSessionStatusCreated && !ossDirectReady {
		if request, err = s.syncUploadRequestFromRemote(ctx, request); err != nil {
			if appErr, ok := err.(*domain.AppError); ok {
				return nil, appErr
			}
			return nil, infraError("sync upload session before completion", err)
		}
	}

	ossDirectFinalized := false
	ossDirectObjectKey := ""
	if ossDirectReady {
		ossObjectKey := strings.TrimSpace(params.OSSObjectKey)
		expectedObjectKey := s.ossDirectService.BuildUploadSessionObjectKey(task.TaskNo, request.RequestID, request.FileName)
		legacyTaskPrefix := "tasks/" + strings.Trim(strings.TrimSpace(task.TaskNo), "/") + "/assets/"
		isExpectedObjectKey := ossObjectKey == expectedObjectKey
		if request.UploadMode == domain.DesignAssetUploadModeMultipart && strings.HasPrefix(ossObjectKey, legacyTaskPrefix) {
			isExpectedObjectKey = true
		}
		if !isExpectedObjectKey {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "oss_object_key does not belong to upload_session", map[string]interface{}{
				"upload_session_id": request.RequestID,
			})
		}
		hasMultipartParts := len(params.OSSParts) > 0 && strings.TrimSpace(params.OSSUploadID) != ""
		if request.UploadMode == domain.DesignAssetUploadModeMultipart && !hasMultipartParts {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "multipart upload_session requires oss_parts and oss_upload_id", nil)
		}
		if request.UploadMode == domain.DesignAssetUploadModeSmall && hasMultipartParts {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "single-part upload_session cannot complete multipart parts", nil)
		}
		var multipartCompleteErr error
		if hasMultipartParts {
			multipartCompleteErr = s.ossDirectService.CompleteMultipartUpload(ctx, ossObjectKey, strings.TrimSpace(params.OSSUploadID), params.OSSParts)
		}
		info, exists, statErr := s.ossDirectService.StatObject(ctx, ossObjectKey)
		if statErr != nil {
			return nil, infraError("verify completed oss direct object", statErr)
		}
		if !exists || info == nil {
			if multipartCompleteErr != nil {
				return nil, infraError("complete oss direct multipart upload", multipartCompleteErr)
			}
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "completed OSS direct object is missing", nil)
		}
		if info.ContentLength < 0 || info.ContentLength != *request.ExpectedSize {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "completed OSS direct object size does not match upload_session", map[string]interface{}{
				"expected_size": *request.ExpectedSize,
				"actual_size":   info.ContentLength,
			})
		}
		if multipartCompleteErr != nil {
			log.Printf("oss_direct_complete_recovered_from_existing_object session=%s object_key=%s error=%v", request.RequestID, ossObjectKey, multipartCompleteErr)
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
	var previousCurrentVersionID *int64
	alreadyCompleted := false

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		lockedRequest, err := s.getUploadRequestForUpdate(ctx, tx, request.RequestID)
		if err != nil {
			return fmt.Errorf("lock upload request before completion: %w", err)
		}
		if lockedRequest == nil || lockedRequest.TaskID != params.TaskID {
			return domain.ErrNotFound
		}
		request = lockedRequest
		lockedTask, err := s.getTaskForUpdate(ctx, tx, params.TaskID)
		if err != nil {
			return fmt.Errorf("lock task before upload completion: %w", err)
		}
		if lockedTask == nil {
			return domain.ErrNotFound
		}
		task = lockedTask
		if appErr := rejectCompletedTaskAssetMutation(task); appErr != nil {
			return appErr
		}
		if appErr := authorizeV8TaskAssetMutation(ctx, task, *request.TaskAssetType); appErr != nil {
			return appErr
		}
		if request.TaskAssetType == nil {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session asset_type is required", nil)
		}
		if appErr := validateTaskStageUploadAssetType(task, *request.TaskAssetType, "", request.AssetID); appErr != nil {
			return appErr
		}
		if request.Status == domain.UploadRequestStatusBound || (request.SessionStatus == domain.DesignAssetSessionStatusCompleted && request.BoundAssetID != nil) {
			alreadyCompleted = true
			return nil
		}
		if request.Status == domain.UploadRequestStatusCancelled || request.Status == domain.UploadRequestStatusExpired ||
			request.SessionStatus == domain.DesignAssetSessionStatusCancelled || request.SessionStatus == domain.DesignAssetSessionStatusExpired {
			return domain.NewAppError(domain.ErrCodeConflict, "upload_session changed concurrently and is already terminal", map[string]interface{}{
				"upload_session_id": request.RequestID,
				"session_status":    request.SessionStatus,
			})
		}
		if request.AssetID != nil {
			existingAsset, err := s.getDesignAssetForUpdate(ctx, tx, *request.AssetID)
			if err != nil {
				return fmt.Errorf("lock existing design asset: %w", err)
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
			if !retouchRequirementIDsEqual(existingAsset.RetouchRequirementID, retouchRequirementID) {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "retouch_requirement_id does not match existing asset scope", map[string]interface{}{
					"retouch_requirement_id":       retouchRequirementID,
					"asset_retouch_requirement_id": existingAsset.RetouchRequirementID,
					"asset_id":                     existingAsset.ID,
					"upload_session_id":            request.RequestID,
				})
			}
			asset = existingAsset
			assetID = existingAsset.ID
			previousCurrentVersionID = domain.CloneInt64Ptr(existingAsset.CurrentVersionID)
		} else {
			assetNo, err := s.designAssetRepo.NextAssetNo(ctx, tx, params.TaskID)
			if err != nil {
				return fmt.Errorf("next design asset no: %w", err)
			}
			asset = &domain.DesignAsset{
				TaskID:               params.TaskID,
				AssetNo:              assetNo,
				SourceAssetID:        request.SourceAssetID,
				ScopeSKUCode:         scopeSKUCode,
				RetouchRequirementID: retouchRequirementID,
				AssetType:            requestAssetType,
				CreatedBy:            params.CompletedBy,
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
		flowReviewStatus := domain.TaskAssetFlowReviewStatusNotApplicable
		var approvedAt *time.Time
		var approvedBy *int64
		if requestAssetType.IsDelivery() {
			flowReviewStatus = domain.TaskAssetFlowReviewStatusPendingReview
		}
		sourceModuleKey := designAssetSourceModuleKeyForTask(task, requestAssetType)
		sourceTaskModuleID, err := s.resolveTaskAssetSourceModuleID(ctx, tx, task, sourceModuleKey)
		if err != nil {
			return fmt.Errorf("resolve task asset source module: %w", err)
		}
		taskAsset := &domain.TaskAsset{
			TaskID:               params.TaskID,
			AssetID:              &assetID,
			ScopeSKUCode:         optionalStringPtr(scopeSKUCode),
			RetouchRequirementID: retouchRequirementID,
			AssetType:            requestAssetType,
			VersionNo:            timelineVersionNo,
			AssetVersionNo:       &assetVersionNo,
			UploadMode:           optionalStringPtr(string(request.UploadMode)),
			UploadRequestID:      &request.RequestID,
			StorageRefID:         &storageRefID,
			FileName:             request.FileName,
			OriginalName:         optionalStringPtr(request.FileName),
			RemoteFileID:         meta.FileID,
			MimeType:             optionalStringPtr(firstNonEmpty(meta.MimeType, request.MimeType)),
			FileSize:             firstNonNilInt64(meta.FileSize, request.ExpectedSize, request.FileSize),
			StorageKey:           optionalStringPtr(resolvedStorageKey),
			WholeHash:            meta.FileHash,
			UploadStatus:         &uploadStatus,
			PreviewStatus:        &previewStatus,
			UploadedBy:           params.CompletedBy,
			UploadedAt:           &now,
			Remark:               firstNonEmpty(strings.TrimSpace(params.Remark), strings.TrimSpace(request.Remark)),
			SourceModuleKey:      sourceModuleKey,
			SourceTaskModuleID:   sourceTaskModuleID,
			FlowReviewStatus:     flowReviewStatus,
			ApprovedAt:           approvedAt,
			ApprovedBy:           approvedBy,
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
		if requestAssetType.IsReference() {
			if err := s.insertReferenceFileRefFlat(ctx, tx, params.TaskID, referenceSKUItemID, retouchRequirementID, storageRefID); err != nil {
				return err
			}
		}
		bindableResource := requestAssetType.IsSource() || requestAssetType.IsDelivery()
		if bindableResource {
			stagingRepo, ok := s.taskAssetRepo.(taskAssetBindingStagingRepo)
			if !ok {
				return fmt.Errorf("task asset repository does not support staged resource binding")
			}
			bindingRole := "final"
			if requestAssetType.IsSource() {
				bindingRole = "source"
			}
			if err := stagingRepo.MarkBindingStaged(ctx, tx, versionID, params.TaskID, params.CompletedBy, scopeSKUCode, retouchRequirementID, bindingRole, request.RequestID, now.Add(taskAssetStagingRetention)); err != nil {
				return err
			}
		} else if err := s.designAssetRepo.UpdateCurrentVersionID(ctx, tx, assetID, &versionID); err != nil {
			return fmt.Errorf("update design asset current version: %w", err)
		}
		if !bindableResource && previousCurrentVersionID != nil && *previousCurrentVersionID > 0 && *previousCurrentVersionID != versionID {
			if supersedeRepo, ok := s.taskAssetRepo.(taskAssetVersionSupersedeRepo); ok {
				cleanupAfter := now.Add(assetVersionReplacementRetention)
				if err := supersedeRepo.MarkAssetVersionSuperseded(ctx, tx, *previousCurrentVersionID, versionID, now, cleanupAfter); err != nil {
					return err
				}
			}
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
		// Completing an upload only produces a staged file. Workflow state moves
		// exclusively in submit-design/audit decision transactions.
		shouldAppendDesignSubmitted := false
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
		if shouldAppendDesignSubmitted {
			_, err = s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventDesignSubmitted, &params.CompletedBy, map[string]interface{}{
				"asset_type": string(requestAssetType), "asset_id": assetID, "designer_id": task.DesignerID,
				"last_customization_operator_id": submittedCustomizationOperatorID(task, params.CompletedBy),
				"upload_session_id":              request.RequestID, "uploaded_by": params.CompletedBy, "target_sku_code": scopeSKUCode,
			})
			if err != nil {
				return fmt.Errorf("append design submitted event: %w", err)
			}
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
	if alreadyCompleted {
		return s.buildCompletedUploadSessionResult(ctx, params.TaskID, request)
	}

	request, appErr = s.requireUploadRequest(ctx, params.TaskID, request.RequestID)
	if appErr != nil {
		return nil, appErr
	}
	result, appErr := s.buildCompletedUploadSessionResult(ctx, params.TaskID, request)
	if appErr != nil {
		return nil, appErr
	}
	if result != nil {
		s.scheduleDerivedPreviewGeneration(params.TaskID, assetID, params.CompletedBy, result.Version)
	}
	return result, nil
}

func (s *taskAssetCenterService) resolveTaskAssetSourceModuleID(ctx context.Context, tx repo.Tx, task *domain.Task, moduleKey string) (*int64, error) {
	moduleKey = strings.TrimSpace(moduleKey)
	if s.taskModuleRepo == nil || task == nil || moduleKey == "" {
		return nil, nil
	}

	var (
		module *domain.TaskModule
		err    error
	)
	if atomicRepo, ok := s.taskModuleRepo.(taskModuleForUpdateRepo); ok {
		module, err = atomicRepo.GetByTaskAndKeyForUpdate(ctx, tx, task.ID, moduleKey)
	} else {
		module, err = s.taskModuleRepo.GetByTaskAndKey(ctx, task.ID, moduleKey)
	}
	if err != nil {
		return nil, err
	}
	if module == nil && task.TaskStatus == domain.TaskStatusCompleted {
		module, err = s.taskModuleRepo.InsertPlaceholder(ctx, tx, task.ID, moduleKey)
		if err != nil {
			return nil, err
		}
	}
	if module == nil || module.ID <= 0 {
		return nil, nil
	}
	moduleID := module.ID
	return &moduleID, nil
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
	request, appErr := s.requireUploadRequest(ctx, params.TaskID, params.SessionID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := rejectCompletedTaskAssetMutation(task); appErr != nil {
		return nil, appErr
	}
	assetType := domain.TaskAssetType("")
	if request.TaskAssetType != nil {
		assetType = *request.TaskAssetType
	}
	if appErr := authorizeV8TaskAssetMutation(ctx, task, assetType); appErr != nil {
		return nil, appErr
	}
	if request.SessionStatus == domain.DesignAssetSessionStatusCompleted {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "completed upload_session cannot be cancelled", nil)
	}
	if request.SessionStatus == domain.DesignAssetSessionStatusCancelled {
		return domain.BuildUploadSession(request), nil
	}
	lastSyncedAt := s.nowFn().UTC()
	alreadyCancelled := false
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		lockedRequest, err := s.getUploadRequestForUpdate(ctx, tx, request.RequestID)
		if err != nil {
			return fmt.Errorf("lock upload request before cancellation: %w", err)
		}
		if lockedRequest == nil || lockedRequest.TaskID != params.TaskID {
			return domain.ErrNotFound
		}
		request = lockedRequest
		lockedTask, err := s.getTaskForUpdate(ctx, tx, params.TaskID)
		if err != nil {
			return fmt.Errorf("lock task before upload cancellation: %w", err)
		}
		if lockedTask == nil {
			return domain.ErrNotFound
		}
		task = lockedTask
		if appErr := rejectCompletedTaskAssetMutation(task); appErr != nil {
			return appErr
		}
		if appErr := authorizeV8TaskAssetMutation(ctx, task, assetType); appErr != nil {
			return appErr
		}
		if request.Status == domain.UploadRequestStatusBound || request.SessionStatus == domain.DesignAssetSessionStatusCompleted {
			return domain.NewAppError(domain.ErrCodeConflict, "upload_session completed concurrently and cannot be cancelled", map[string]interface{}{
				"upload_session_id": request.RequestID,
				"session_status":    request.SessionStatus,
			})
		}
		if request.Status == domain.UploadRequestStatusCancelled || request.SessionStatus == domain.DesignAssetSessionStatusCancelled {
			alreadyCancelled = true
			return nil
		}
		if request.Status == domain.UploadRequestStatusExpired || request.SessionStatus == domain.DesignAssetSessionStatusExpired {
			return domain.NewAppError(domain.ErrCodeConflict, "upload_session expired concurrently and cannot be cancelled", map[string]interface{}{
				"upload_session_id": request.RequestID,
				"session_status":    request.SessionStatus,
			})
		}
		remoteUploadID := strings.TrimSpace(request.RemoteUploadID)
		ossObjectKey := strings.TrimSpace(params.OSSObjectKey)
		ossUploadID := strings.TrimSpace(params.OSSUploadID)
		hasOSSCleanupIdentifiers := ossObjectKey != "" || ossUploadID != ""
		if s.ossDirectService != nil && s.ossDirectService.Enabled() && (hasOSSCleanupIdentifiers || remoteUploadID == "") {
			expectedObjectKey := s.ossDirectService.BuildUploadSessionObjectKey(task.TaskNo, request.RequestID, request.FileName)
			if ossObjectKey == "" {
				ossObjectKey = expectedObjectKey
			}
			legacyTaskPrefix := "tasks/" + strings.Trim(strings.TrimSpace(task.TaskNo), "/") + "/assets/"
			isExpectedObjectKey := ossObjectKey == expectedObjectKey
			if request.UploadMode == domain.DesignAssetUploadModeMultipart && strings.HasPrefix(ossObjectKey, legacyTaskPrefix) {
				isExpectedObjectKey = true
			}
			if !isExpectedObjectKey {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "oss_object_key does not belong to upload_session", nil)
			}
			if request.UploadMode == domain.DesignAssetUploadModeMultipart && ossUploadID != "" {
				if err := s.ossDirectService.AbortMultipartUpload(ctx, ossObjectKey, ossUploadID); err != nil {
					return fmt.Errorf("abort OSS direct multipart upload: %w", err)
				}
			} else if request.UploadMode != domain.DesignAssetUploadModeMultipart {
				if err := s.ossDirectService.DeleteObject(ctx, ossObjectKey); err != nil {
					return fmt.Errorf("delete OSS direct single upload: %w", err)
				}
			} else {
				log.Printf("oss_direct_cancel_missing_upload_id session=%s", request.RequestID)
			}
		}
		if remoteUploadID != "" {
			if err := s.uploadClient.AbortUploadSession(ctx, RemoteAbortUploadRequest{RemoteUploadID: remoteUploadID}); err != nil {
				return fmt.Errorf("abort upload session via upload service client: %w", err)
			}
		}
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
		_, err = s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventAssetUploadSessionCancelled, &params.CancelledBy, map[string]interface{}{
			"upload_session_id": request.RequestID,
			"upload_mode":       string(request.UploadMode),
			"remote_upload_id":  request.RemoteUploadID,
			"remark":            strings.TrimSpace(params.Remark),
		})
		return err
	})
	if txErr != nil {
		if appErr, ok := txErr.(*domain.AppError); ok {
			return nil, appErr
		}
		return nil, infraError("cancel upload session", txErr)
	}
	if alreadyCancelled {
		return domain.BuildUploadSession(request), nil
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
	if params.ExpectedSize == nil || *params.ExpectedSize <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "expected_size must be greater than zero", nil)
	}
	if *params.ExpectedSize > taskAssetUploadMaxFileSizeBytes {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "expected_size exceeds upload limit", map[string]interface{}{
			"max_bytes": taskAssetUploadMaxFileSizeBytes,
			"max_label": taskAssetUploadMaxFileSizeLabel,
		})
	}
	inferredMode, modeErr := s.inferTaskAssetUploadMode(params.AssetType, params.ExpectedSize)
	if modeErr != nil {
		return nil, modeErr
	}
	// Keep the explicit multipart compatibility routes deterministic for small
	// files, while never allowing the explicit small-file route to downgrade a
	// payload that the backend requires to use multipart upload.
	if inferredMode == domain.DesignAssetUploadModeMultipart {
		mode = domain.DesignAssetUploadModeMultipart
	} else if mode != domain.DesignAssetUploadModeMultipart {
		mode = domain.DesignAssetUploadModeSmall
	}
	task, appErr := s.requireTask(ctx, params.TaskID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := rejectConflictingRetouchAssetScopes(params.TargetSKUCode, params.RetouchRequirementID); appErr != nil {
		return nil, appErr
	}
	if appErr := validateRetouchRequirementAssetScope(ctx, task, params.RetouchRequirementID, s.retouchRequirementRepo); appErr != nil {
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
	if appErr := rejectCompletedTaskAssetMutation(task); appErr != nil {
		return nil, appErr
	}
	if appErr := authorizeV8TaskAssetMutation(ctx, task, params.AssetType); appErr != nil {
		return nil, appErr
	}
	if appErr := validateTaskStageUploadAssetType(task, params.AssetType, params.OwnerModuleKey, params.AssetID); appErr != nil {
		return nil, appErr
	}
	taskRef := strings.TrimSpace(task.TaskNo)
	identity, appErr := s.freezeUploadAssetIdentity(ctx, params.TaskID, params.AssetID, params.SourceAssetID, params.TargetSKUCode, params.RetouchRequirementID, params.AssetType, params.CreatedBy)
	if appErr != nil {
		return nil, appErr
	}
	params.AssetID = &identity.AssetID
	versionNo, appErr := s.nextPendingAssetVersionNo(ctx, identity.AssetID)
	if appErr != nil {
		return nil, appErr
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

	requiredContentType := normalizeRequiredUploadContentType(params.MimeType)
	requestID := uuid.NewString()
	fileSize := *params.ExpectedSize
	var remote *RemoteUploadSessionPlan
	var ossPlan *OSSDirectUploadPlan
	if s.ossDirectService != nil && s.ossDirectService.Enabled() {
		objectKey := s.ossDirectService.BuildUploadSessionObjectKey(taskRef, requestID, strings.TrimSpace(params.Filename))
		var plan *OSSDirectUploadPlan
		var ossErr error
		if mode == domain.DesignAssetUploadModeMultipart {
			plan, ossErr = s.ossDirectService.CreateMultipartUploadPlan(ctx, objectKey, fileSize, requiredContentType)
		} else {
			plan, ossErr = s.ossDirectService.CreateUploadPlan(ctx, objectKey, fileSize, requiredContentType)
		}
		if ossErr != nil {
			log.Printf("oss_direct_upload_plan_fallback error=%v session=%s", ossErr, requestID)
		} else {
			ossPlan = plan
			if plan.Mode == "multipart" {
				mode = domain.DesignAssetUploadModeMultipart
			} else {
				mode = domain.DesignAssetUploadModeSmall
			}
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
		MimeType:     requiredContentType,
		CreatedBy:    params.CreatedBy,
	}
	if ossPlan == nil {
		var err error
		remote, err = s.uploadClient.CreateUploadSession(ctx, createReq)
		if err != nil {
			return nil, infraError("create upload session via upload service client", err)
		}
	}

	now := s.nowFn().UTC()
	remoteUploadID := ""
	remoteFileID := ""
	isPlaceholder := false
	var expiresAt *time.Time
	lastSyncedAt := &now
	if ossPlan != nil {
		expiresAt = &ossPlan.ExpiresAt
	} else if remote != nil {
		remoteUploadID = remote.UploadID
		remoteFileID = valueOrEmpty(remote.FileID)
		isPlaceholder = remote.IsStub
		expiresAt = remote.ExpiresAt
		lastSyncedAt = firstNonNilTime(remote.LastSyncedAt, &now)
	}
	request := &domain.UploadRequest{
		RequestID:            requestID,
		OwnerType:            domain.AssetOwnerTypeTask,
		OwnerID:              params.TaskID,
		TaskID:               params.TaskID,
		AssetID:              params.AssetID,
		SourceAssetID:        params.SourceAssetID,
		TargetSKUCode:        params.TargetSKUCode,
		RetouchRequirementID: domain.CloneInt64Ptr(params.RetouchRequirementID),
		TaskAssetType:        &params.AssetType,
		StorageAdapter:       domain.AssetStorageAdapterOSSUploadService,
		UploadMode:           mode,
		RefType:              domain.AssetStorageRefTypeTaskAssetObject,
		FileName:             strings.TrimSpace(params.Filename),
		MimeType:             requiredContentType,
		FileSize:             params.ExpectedSize,
		ExpectedSize:         params.ExpectedSize,
		ChecksumHint:         strings.TrimSpace(params.FileHash),
		Status:               domain.UploadRequestStatusRequested,
		StorageProvider:      domain.DesignAssetStorageProviderOSS,
		SessionStatus:        domain.DesignAssetSessionStatusCreated,
		RemoteUploadID:       remoteUploadID,
		RemoteFileID:         remoteFileID,
		IsPlaceholder:        isPlaceholder,
		CreatedBy:            params.CreatedBy,
		ExpiresAt:            expiresAt,
		LastSyncedAt:         lastSyncedAt,
		Remark:               strings.TrimSpace(params.Remark),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		lockedTask, err := s.getTaskForUpdate(ctx, tx, params.TaskID)
		if err != nil {
			return fmt.Errorf("lock task before upload session creation: %w", err)
		}
		if lockedTask == nil {
			return domain.ErrNotFound
		}
		task = lockedTask
		if appErr := rejectCompletedTaskAssetMutation(task); appErr != nil {
			return appErr
		}
		if appErr := authorizeV8TaskAssetMutation(ctx, task, params.AssetType); appErr != nil {
			return appErr
		}
		if appErr := validateTaskStageUploadAssetType(task, params.AssetType, params.OwnerModuleKey, params.AssetID); appErr != nil {
			return appErr
		}
		created, err := s.uploadRequestRepo.Create(ctx, tx, request)
		if err != nil {
			return err
		}
		request = created
		_, err = s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventAssetUploadSessionCreated, &params.CreatedBy, map[string]interface{}{
			"upload_session_id":      request.RequestID,
			"asset_id":               params.AssetID,
			"asset_type":             string(params.AssetType),
			"target_sku_code":        params.TargetSKUCode,
			"owner_module_key":       strings.TrimSpace(params.OwnerModuleKey),
			"upload_policy":          strings.TrimSpace(params.UploadPolicy),
			"retouch_requirement_id": params.RetouchRequirementID,
			"filename":               request.FileName,
			"expected_size":          request.ExpectedSize,
			"mime_type":              request.MimeType,
			"upload_mode":            string(mode),
			"storage_provider":       string(request.StorageProvider),
			"remote_upload_id":       request.RemoteUploadID,
			"expires_at":             request.ExpiresAt,
		})
		return err
	})
	if txErr != nil {
		if appErr, ok := txErr.(*domain.AppError); ok {
			return nil, appErr
		}
		return nil, infraError("create upload session", txErr)
	}
	result := &CreateTaskAssetUploadSessionResult{
		Session:   domain.BuildUploadSession(request),
		Remote:    remote,
		OSSDirect: ossPlan,
	}
	return result, nil
}

func (s *taskAssetCenterService) syncUploadRequestFromRemote(ctx context.Context, request *domain.UploadRequest) (*domain.UploadRequest, error) {
	if request == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session is required", nil)
	}
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
		lockedRequest, err := s.getUploadRequestForUpdate(ctx, tx, request.RequestID)
		if err != nil {
			return fmt.Errorf("lock upload request before remote sync: %w", err)
		}
		if lockedRequest == nil || lockedRequest.TaskID != request.TaskID {
			return domain.ErrNotFound
		}
		request = lockedRequest
		lockedTask, err := s.getTaskForUpdate(ctx, tx, request.TaskID)
		if err != nil {
			return fmt.Errorf("lock task before upload session remote sync: %w", err)
		}
		if lockedTask == nil {
			return domain.ErrNotFound
		}
		if appErr := rejectCompletedTaskAssetMutation(lockedTask); appErr != nil {
			return appErr
		}
		if request.Status == domain.UploadRequestStatusBound || request.Status == domain.UploadRequestStatusCancelled || request.Status == domain.UploadRequestStatusExpired ||
			request.SessionStatus == domain.DesignAssetSessionStatusCompleted || request.SessionStatus == domain.DesignAssetSessionStatusCancelled || request.SessionStatus == domain.DesignAssetSessionStatusExpired {
			return nil
		}
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
	filename := resolveDesignAssetDownloadFilename(version)
	if ossDirect != nil && ossDirect.Enabled() && strings.TrimSpace(version.StorageKey) != "" {
		key := strings.TrimSpace(version.StorageKey)
		var info *OSSDirectDownloadInfo
		mimeType := version.MimeType
		if preview {
			if process, ok := buildOSSIMGPreviewProcessForVersion(version); ok {
				info = ossDirect.PresignPreviewURLWithProcess(key, process)
				if strings.Contains(process, "format,jpg") {
					mimeType = "image/jpeg"
				}
			} else {
				info = ossDirect.PresignPreviewURL(key)
			}
		} else {
			info = ossDirect.PresignDownloadURLWithFilename(key, filename)
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
				Filename:         filename,
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
	filename := resolveDesignAssetDownloadFilename(version)
	downloadMode := domain.AssetDownloadModeProxy
	downloadURL := version.DownloadURL
	if uploadClient != nil {
		if directURL := uploadClient.BuildBrowserFileURL(version.StorageKey); directURL != nil && strings.TrimSpace(*directURL) != "" {
			urlValue := strings.TrimSpace(*directURL)
			downloadURL = directURL
			if isDirectBrowserURL(urlValue) {
				downloadMode = domain.AssetDownloadModeDirect
			}
		}
	}
	if downloadMode != domain.AssetDownloadModeDirect && downloadURL != nil {
		urlValue := strings.TrimSpace(*downloadURL)
		if strings.HasPrefix(urlValue, "http://") || strings.HasPrefix(urlValue, "https://") {
			downloadMode = domain.AssetDownloadModeDirect
		}
	}
	if downloadMode == domain.AssetDownloadModeProxy && downloadURL != nil {
		urlValue := AppendProxyDownloadFilenameQuery(*downloadURL, filename)
		downloadURL = &urlValue
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
		Filename:         filename,
		FileSize:         fileSize,
		MimeType:         version.MimeType,
	}
}

func resolveDesignAssetDownloadFilename(version *domain.DesignAssetVersion) string {
	if version == nil {
		return "asset"
	}
	originalName := ""
	if version.OriginalNameExplicit {
		originalName = strings.TrimSpace(version.OriginalFilename)
	}
	fileName := firstNonEmpty(strings.TrimSpace(version.FileName), strings.TrimSpace(version.OriginalFilename))
	return ResolveAssetDownloadFilenameForSingle(originalName, fileName, version.AssetID, version.ScopeSKUCode)
}

func isDirectBrowserURL(urlValue string) bool {
	urlValue = strings.TrimSpace(urlValue)
	if urlValue == "" {
		return false
	}
	if strings.HasPrefix(urlValue, "/v1/assets/files/") || strings.HasPrefix(urlValue, "/files/") {
		return false
	}
	if parsed, err := url.Parse(urlValue); err == nil && parsed.Path != "" {
		if strings.HasPrefix(parsed.Path, "/v1/assets/files/") || strings.HasPrefix(parsed.Path, "/files/") {
			return false
		}
	}
	return strings.HasPrefix(urlValue, "http://") || strings.HasPrefix(urlValue, "https://") || strings.HasPrefix(urlValue, "/")
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
	if !hasParts && !hasUploadID && hasObjectKey {
		return nil
	}
	if hasParts && hasUploadID && hasObjectKey {
		return nil
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "oss direct complete requires object_key alone for single-part, or parts, upload_id, and object_key together for multipart", map[string]interface{}{
		"has_oss_parts":      hasParts,
		"has_oss_upload_id":  hasUploadID,
		"has_oss_object_key": hasObjectKey,
	})
}

func (s *taskAssetCenterService) canFinalizeOSSDirectUpload(params CompleteTaskAssetUploadSessionParams) bool {
	if s.ossDirectService == nil || !s.ossDirectService.Enabled() || strings.TrimSpace(params.OSSObjectKey) == "" {
		return false
	}
	hasParts := len(params.OSSParts) > 0
	hasUploadID := strings.TrimSpace(params.OSSUploadID) != ""
	return (!hasParts && !hasUploadID) || (hasParts && hasUploadID)
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

func (s *taskAssetCenterService) inferTaskAssetUploadMode(assetType domain.TaskAssetType, expectedSize *int64) (domain.DesignAssetUploadMode, *domain.AppError) {
	normalized := domain.NormalizeTaskAssetType(assetType)
	if normalized == "" {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_type is required", nil)
	}
	if expectedSize != nil && *expectedSize >= 0 {
		if s.ossDirectService != nil && s.ossDirectService.Enabled() {
			if !s.ossDirectService.UsesMultipartUpload(*expectedSize) {
				return domain.DesignAssetUploadModeSmall, nil
			}
		} else if *expectedSize <= taskAssetSinglePartThreshold {
			return domain.DesignAssetUploadModeSmall, nil
		}
	}
	return domain.DesignAssetUploadModeMultipart, nil
}

func validateTaskStageUploadAssetType(task *domain.Task, assetType domain.TaskAssetType, ownerModuleKey string, frozenAssetID *int64) *domain.AppError {
	if task == nil {
		return nil
	}
	normalized := domain.NormalizeTaskAssetType(assetType)
	if task.TaskType == domain.TaskTypeSKUPlanning || task.TaskType == domain.TaskTypePurchaseTask {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "planning SKU images do not use task resource upload sessions", map[string]interface{}{
			"deny_code": "planning_sku_resource_upload_not_allowed",
		})
	}
	if normalized == domain.TaskAssetTypeReference {
		if strings.TrimSpace(ownerModuleKey) == string(domain.ModuleKeyBasicInfo) {
			return nil
		}
		// Completion and cancellation reload the frozen upload request rather than
		// the original command, so owner_module_key is no longer present there.
		// A frozen asset id proves that the session passed the create-time
		// basic_info reference gate; it does not widen creation of new references.
		if strings.TrimSpace(ownerModuleKey) == "" && frozenAssetID != nil && *frozenAssetID > 0 {
			return nil
		}
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "reference uploads must belong to the task basic information", map[string]interface{}{
			"deny_code":                          "reference_owner_module_not_allowed",
			"owner_module_key":                   strings.TrimSpace(ownerModuleKey),
			"allowed_reference_owner_module_key": string(domain.ModuleKeyBasicInfo),
		})
	}
	if task.TaskStatus == domain.TaskStatusInProgress {
		if task.TaskType == domain.TaskTypeRetouchTask {
			if normalized == domain.TaskAssetTypeSource || normalized == domain.TaskAssetTypeDelivery {
				return nil
			}
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "retouch uploads only support source or final output files", map[string]interface{}{
				"deny_code": "retouch_asset_type_not_allowed",
			})
		}
		if normalized == domain.TaskAssetTypeSource {
			return nil
		}
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "design stage only accepts source files; final output is uploaded during audit", map[string]interface{}{
			"deny_code":           "design_stage_final_output_not_allowed",
			"task_status":         string(task.TaskStatus),
			"asset_type":          string(assetType),
			"allowed_asset_types": []string{string(domain.TaskAssetTypeSource)},
		})
	}
	if task.TaskStatus != domain.TaskStatusPendingAudit {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task assets cannot be uploaded in the current task state", map[string]interface{}{
			"deny_code":   "task_asset_stage_not_uploadable",
			"task_status": string(task.TaskStatus),
		})
	}
	if normalized == domain.TaskAssetTypeSource || normalized == domain.TaskAssetTypeDelivery {
		return nil
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "audit-stage uploads only support source, delivery, or basic_info reference assets", map[string]interface{}{
		"deny_code":        "audit_stage_asset_type_not_allowed",
		"task_status":      string(task.TaskStatus),
		"asset_type":       string(assetType),
		"owner_module_key": strings.TrimSpace(ownerModuleKey),
		"allowed_asset_types": []string{
			string(domain.TaskAssetTypeSource),
			string(domain.TaskAssetTypeDelivery),
		},
		"allowed_reference_owner_module_key": string(domain.ModuleKeyBasicInfo),
	})
}

func rejectCompletedTaskAssetMutation(task *domain.Task) *domain.AppError {
	if task == nil || (task.TaskStatus != domain.TaskStatusCompleted && task.TaskStatus != domain.TaskStatusArchived) {
		return nil
	}
	return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "finalized task resources can only change after the task is reopened", map[string]interface{}{
		"deny_code":   "completed_resource_requires_reopen",
		"task_id":     task.ID,
		"task_status": task.TaskStatus,
	})
}

func authorizeV8TaskAssetMutation(ctx context.Context, task *domain.Task, assetType domain.TaskAssetType) *domain.AppError {
	if task == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "task is required", nil)
	}
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || actor.EffectiveAccess == nil {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "task asset mutation requires explicit access", map[string]interface{}{
			"deny_code": "task_asset_explicit_access_required",
			"task_id":   task.ID,
		})
	}

	permissions := []domain.PermissionCode{domain.PermissionAssetManage}
	if assetType.IsReference() {
		// Operational reference attachments are a task-maintenance action. They are
		// intentionally not implied by design/audit capabilities.
		permissions = append([]domain.PermissionCode{domain.PermissionTaskCreate}, permissions...)
		switch task.TaskStatus {
		case domain.TaskStatusDraft, domain.TaskStatusPendingAssign, domain.TaskStatusAssigned, domain.TaskStatusInProgress, domain.TaskStatusPendingAudit:
		default:
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task references cannot be changed in the current task state", map[string]interface{}{
				"deny_code":   "task_reference_status_not_editable",
				"task_id":     task.ID,
				"task_status": task.TaskStatus,
			})
		}
	} else {
		switch task.TaskStatus {
		case domain.TaskStatusPendingAssign, domain.TaskStatusAssigned, domain.TaskStatusInProgress:
			permissions = append([]domain.PermissionCode{domain.PermissionTaskDesignSubmit}, permissions...)
		case domain.TaskStatusPendingAudit:
			permissions = append([]domain.PermissionCode{domain.PermissionTaskAuditDecision}, permissions...)
		default:
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task assets cannot be changed in the current task state", map[string]interface{}{
				"deny_code":   "task_asset_status_not_editable",
				"task_id":     task.ID,
				"task_status": task.TaskStatus,
			})
		}
	}

	subject := task.AccessSubject()
	for _, permission := range permissions {
		if domain.EffectiveAccessAllowsTask(actor, permission, subject) {
			return nil
		}
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "task asset mutation is outside the actor's explicit capability or data scope", map[string]interface{}{
		"deny_code":            "task_asset_permission_or_scope_denied",
		"task_id":              task.ID,
		"task_status":          task.TaskStatus,
		"required_permissions": permissions,
	})
}

func authorizeV8TaskAssetSessionRead(ctx context.Context, task *domain.Task) *domain.AppError {
	if task == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "task is required", nil)
	}
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || actor.EffectiveAccess == nil {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "task asset session read requires explicit access", map[string]interface{}{
			"deny_code": "task_asset_explicit_access_required",
			"task_id":   task.ID,
		})
	}
	permissions := []domain.PermissionCode{
		domain.PermissionTaskView,
		domain.PermissionAssetView,
		domain.PermissionTaskCreate,
		domain.PermissionTaskDesignSubmit,
		domain.PermissionTaskAuditDecision,
		domain.PermissionAssetManage,
	}
	for _, permission := range permissions {
		if domain.EffectiveAccessAllowsTask(actor, permission, task.AccessSubject()) {
			return nil
		}
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "task asset session is outside the actor's explicit capability or data scope", map[string]interface{}{
		"deny_code":            "task_asset_permission_or_scope_denied",
		"task_id":              task.ID,
		"required_permissions": permissions,
	})
}

func (s *taskAssetCenterService) getUploadRequestForUpdate(ctx context.Context, tx repo.Tx, requestID string) (*domain.UploadRequest, error) {
	if lockingRepo, ok := s.uploadRequestRepo.(uploadRequestForUpdateRepo); ok {
		return lockingRepo.GetByRequestIDForUpdate(ctx, tx, requestID)
	}
	return s.uploadRequestRepo.GetByRequestID(ctx, requestID)
}

func (s *taskAssetCenterService) getTaskForUpdate(ctx context.Context, tx repo.Tx, taskID int64) (*domain.Task, error) {
	if lockingRepo, ok := s.taskRepo.(taskForUpdateRepo); ok {
		return lockingRepo.GetByIDForUpdate(ctx, tx, taskID)
	}
	return s.taskRepo.GetByID(ctx, taskID)
}

func (s *taskAssetCenterService) getDesignAssetForUpdate(ctx context.Context, tx repo.Tx, assetID int64) (*domain.DesignAsset, error) {
	if lockingRepo, ok := s.designAssetRepo.(designAssetForUpdateRepo); ok {
		return lockingRepo.GetByIDForUpdate(ctx, tx, assetID)
	}
	return s.designAssetRepo.GetByID(ctx, assetID)
}

func optionalInt64Equal(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func isAuditStageTaskStatus(status domain.TaskStatus) bool {
	return status == domain.TaskStatusPendingAudit
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

func (s *taskAssetCenterService) resolveReferenceSKUItemID(
	ctx context.Context,
	taskID int64,
	assetType domain.TaskAssetType,
	targetSKUCode string,
) (*int64, *domain.AppError) {
	if !assetType.IsReference() || strings.TrimSpace(targetSKUCode) == "" {
		return nil, nil
	}
	items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list task sku items for reference scope", err)
	}
	for _, item := range items {
		if item != nil && strings.TrimSpace(item.SKUCode) == strings.TrimSpace(targetSKUCode) {
			id := item.ID
			return &id, nil
		}
	}
	return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "reference target_sku_code must belong to the task", map[string]interface{}{
		"task_id":         taskID,
		"target_sku_code": strings.TrimSpace(targetSKUCode),
	})
}

func (s *taskAssetCenterService) insertReferenceFileRefFlat(
	ctx context.Context,
	tx repo.Tx,
	taskID int64,
	skuItemID *int64,
	retouchRequirementID *int64,
	refID string,
) error {
	if s.referenceFileRefFlatRepo == nil {
		return nil
	}
	refID = strings.TrimSpace(refID)
	if refID == "" {
		return nil
	}
	contextValue := "task_reference"
	if retouchRequirementID != nil && *retouchRequirementID > 0 {
		contextValue = "retouch_requirement_reference"
	} else if skuItemID != nil && *skuItemID > 0 {
		contextValue = "sku_reference"
	}
	if _, err := s.referenceFileRefFlatRepo.InsertFlat(ctx, tx, &domain.ReferenceFileRefFlat{
		TaskID:               taskID,
		SKUItemID:            domain.CloneInt64Ptr(skuItemID),
		RetouchRequirementID: retouchRequirementID,
		RefID:                refID,
		OwnerModuleKey:       string(domain.ModuleKeyBasicInfo),
		Context:              stringPtr(contextValue),
	}); err != nil {
		return fmt.Errorf("insert reference_file_ref flat row: %w", err)
	}
	return nil
}

func (s *taskAssetCenterService) freezeUploadAssetIdentity(
	ctx context.Context,
	taskID int64,
	requestedAssetID *int64,
	sourceAssetID *int64,
	targetSKUCode string,
	retouchRequirementID *int64,
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
		if !retouchRequirementIDsEqual(asset.RetouchRequirementID, retouchRequirementID) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "retouch_requirement_id does not match existing asset scope", map[string]interface{}{
				"retouch_requirement_id":       retouchRequirementID,
				"asset_retouch_requirement_id": asset.RetouchRequirementID,
				"asset_id":                     asset.ID,
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
			TaskID:               taskID,
			AssetNo:              assetNo,
			SourceAssetID:        sourceAssetID,
			ScopeSKUCode:         strings.TrimSpace(targetSKUCode),
			RetouchRequirementID: domain.CloneInt64Ptr(retouchRequirementID),
			AssetType:            assetType,
			CreatedBy:            createdBy,
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
