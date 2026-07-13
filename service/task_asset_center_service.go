package service

import (
	"context"
	"encoding/json"
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

type CreateAuditSupplementUploadSessionParams struct {
	TaskID        int64
	CreatedBy     int64
	AssetID       *int64
	Filename      string
	ExpectedSize  *int64
	MimeType      string
	FileHash      string
	Reason        string
	TargetSKUCode string
}

type CompleteAuditSupplementUploadSessionParams struct {
	TaskID            int64
	SessionID         string
	CompletedBy       int64
	Reason            string
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

type AuditSupplementItem struct {
	EventID          string    `json:"event_id"`
	Sequence         int64     `json:"sequence"`
	TaskID           int64     `json:"task_id"`
	AssetID          int64     `json:"asset_id"`
	AssetVersionID   int64     `json:"asset_version_id"`
	AssetVersionNo   int       `json:"asset_version_no"`
	TimelineVersion  int       `json:"timeline_version"`
	UploadSessionID  string    `json:"upload_session_id"`
	Filename         string    `json:"filename"`
	Reason           string    `json:"reason"`
	TargetSKUCode    string    `json:"target_sku_code,omitempty"`
	UploadedBy       int64     `json:"uploaded_by"`
	UploadedByName   string    `json:"uploaded_by_name,omitempty"`
	AuditCountBefore int       `json:"audit_delivery_count_before"`
	AuditCountAfter  int       `json:"audit_delivery_count_after"`
	DesignCount      int       `json:"design_delivery_count"`
	CreatedAt        time.Time `json:"created_at"`
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
	ListAuditSupplements(ctx context.Context, taskID int64) ([]AuditSupplementItem, *domain.AppError)
	CreateAuditSupplementUploadSession(ctx context.Context, params CreateAuditSupplementUploadSessionParams) (*CreateTaskAssetUploadSessionResult, *domain.AppError)
	CompleteUploadSessionByID(ctx context.Context, params CompleteTaskAssetUploadSessionParams) (*CompleteTaskAssetUploadSessionResult, *domain.AppError)
	CompleteUploadSession(ctx context.Context, params CompleteTaskAssetUploadSessionParams) (*CompleteTaskAssetUploadSessionResult, *domain.AppError)
	CompleteAuditSupplementUploadSession(ctx context.Context, params CompleteAuditSupplementUploadSessionParams) (*CompleteTaskAssetUploadSessionResult, *domain.AppError)
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
	dataScopeResolver         DataScopeResolver
	scopeUserRepo             repo.UserRepo
	userDisplayNameResolver   UserDisplayNameResolver
	workflowRules             designSubmissionWorkflowEngine
	retouchRequirementRepo    repo.TaskRetouchRequirementRepo
	referenceFileRefFlatRepo  repo.ReferenceFileRefFlatRepo
}

const (
	taskAssetVersionUniqueKey        = "uq_task_assets_task_version"
	assetVersionRaceRetryDenyCode    = "asset_version_race_retry"
	assetVersionReplacementRetention = 15 * 24 * time.Hour
	taskAssetUploadMaxFileSizeBytes  = int64(1024 * 1024 * 1024)
	taskAssetUploadMaxFileSizeLabel  = "1GB"
	taskAssetSinglePartThreshold     = int64(10 * 1024 * 1024)
	auditSupplementUploadPolicy      = "audit_post_close_supplement"
	auditSupplementRemarkPrefix      = "[audit_supplement]"
)

type taskAssetVersionSupersedeRepo interface {
	MarkAssetVersionSuperseded(ctx context.Context, tx repo.Tx, versionID, supersededByVersionID int64, supersededAt, cleanupAfterAt time.Time) error
}

type uploadRequestForUpdateRepo interface {
	GetByRequestIDForUpdate(ctx context.Context, tx repo.Tx, requestID string) (*domain.UploadRequest, error)
}

type designAssetForUpdateRepo interface {
	GetByIDForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.DesignAsset, error)
}

type designAssetCurrentVersionCASRepo interface {
	UpdateCurrentVersionIDCAS(ctx context.Context, tx repo.Tx, id int64, expectedCurrentVersionID, currentVersionID *int64) (bool, error)
}

type taskAssetForUpdateRepo interface {
	GetByIDForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.TaskAsset, error)
}

type taskForUpdateRepo interface {
	GetByIDForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.Task, error)
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

func WithTaskAssetCenterBlueprintRuleEngine(rules designSubmissionWorkflowEngine) TaskAssetCenterServiceOption {
	return func(s *taskAssetCenterService) {
		s.workflowRules = rules
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

func (s *taskAssetCenterService) ListAuditSupplements(ctx context.Context, taskID int64) ([]AuditSupplementItem, *domain.AppError) {
	task, appErr := s.requireTask(ctx, taskID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.authorizeAuditSupplementRead(ctx, task); appErr != nil {
		return nil, appErr
	}
	events, err := s.taskEventRepo.ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list audit supplement events", err)
	}
	items := make([]AuditSupplementItem, 0)
	for _, event := range events {
		if event == nil || event.EventType != domain.TaskEventAuditSupplementUploaded {
			continue
		}
		item, ok := auditSupplementItemFromEvent(event)
		if !ok {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *taskAssetCenterService) CreateAuditSupplementUploadSession(ctx context.Context, params CreateAuditSupplementUploadSessionParams) (*CreateTaskAssetUploadSessionResult, *domain.AppError) {
	reason := strings.TrimSpace(params.Reason)
	if reason == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "reason is required", map[string]interface{}{
			"deny_code": "audit_supplement_reason_required",
		})
	}
	if params.AssetID != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "audit supplement must append a new delivery asset", map[string]interface{}{
			"deny_code": "audit_supplement_replace_not_allowed",
		})
	}
	return s.createUploadSession(ctx, CreateTaskAssetUploadSessionParams{
		TaskID:         params.TaskID,
		CreatedBy:      params.CreatedBy,
		AssetType:      domain.TaskAssetTypeDelivery,
		Filename:       params.Filename,
		ExpectedSize:   params.ExpectedSize,
		MimeType:       params.MimeType,
		FileHash:       params.FileHash,
		Remark:         buildAuditSupplementRemark(reason),
		TargetSKUCode:  params.TargetSKUCode,
		OwnerModuleKey: domain.ModuleKeyAudit,
		UploadPolicy:   auditSupplementUploadPolicy,
	}, domain.DesignAssetUploadModeMultipart)
}

func (s *taskAssetCenterService) createAuditSupplementUploadSession(ctx context.Context, task *domain.Task, params CreateTaskAssetUploadSessionParams, mode domain.DesignAssetUploadMode) (*CreateTaskAssetUploadSessionResult, *domain.AppError) {
	reason := auditSupplementReasonFromRemark(params.Remark)
	if reason == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "reason is required", map[string]interface{}{
			"deny_code": "audit_supplement_reason_required",
		})
	}
	if mode != domain.DesignAssetUploadModeMultipart {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "audit supplement delivery assets must use multipart upload mode", nil)
	}
	if params.AssetType != domain.TaskAssetTypeDelivery {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "audit supplement only supports delivery assets", map[string]interface{}{
			"deny_code":           "audit_supplement_asset_type_not_allowed",
			"allowed_asset_types": []string{string(domain.TaskAssetTypeDelivery)},
			"asset_type":          string(params.AssetType),
		})
	}
	if params.AssetID != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "audit supplement must append a new delivery asset", map[string]interface{}{
			"deny_code": "audit_supplement_replace_not_allowed",
		})
	}
	if appErr := s.authorizeAuditSupplementWrite(ctx, task); appErr != nil {
		return nil, appErr
	}

	taskRef := strings.TrimSpace(task.TaskNo)
	identity, appErr := s.freezeUploadAssetIdentity(ctx, params.TaskID, nil, nil, params.TargetSKUCode, nil, params.AssetType, params.CreatedBy)
	if appErr != nil {
		return nil, appErr
	}
	params.AssetID = &identity.AssetID
	versionNo, appErr := s.nextPendingAssetVersionNo(ctx, identity.AssetID)
	if appErr != nil {
		return nil, appErr
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
		return nil, infraError("create audit supplement upload session via upload service client", err)
	}

	now := s.nowFn().UTC()
	requiredContentType := normalizeRequiredUploadContentType(params.MimeType)
	request := &domain.UploadRequest{
		OwnerType:       domain.AssetOwnerTypeTask,
		OwnerID:         params.TaskID,
		TaskID:          params.TaskID,
		AssetID:         params.AssetID,
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
		Remark:          buildAuditSupplementRemark(reason),
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
			"owner_module_key":  domain.ModuleKeyAudit,
			"upload_policy":     auditSupplementUploadPolicy,
			"filename":          request.FileName,
			"expected_size":     request.ExpectedSize,
			"mime_type":         request.MimeType,
			"upload_mode":       string(mode),
			"storage_provider":  string(request.StorageProvider),
			"remote_upload_id":  request.RemoteUploadID,
			"expires_at":        request.ExpiresAt,
			"reason":            reason,
		})
		return err
	})
	if txErr != nil {
		return nil, infraError("create audit supplement upload session", txErr)
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
		if ossPlan, ossErr := s.ossDirectService.CreateMultipartUploadPlan(ctx, objectKey, fileSize, requiredContentType); ossErr != nil {
			log.Printf("oss_direct_audit_supplement_upload_plan_fallback error=%v session=%s", ossErr, request.RequestID)
		} else {
			result.OSSDirect = ossPlan
		}
	}
	return result, nil
}

func (s *taskAssetCenterService) CompleteAuditSupplementUploadSession(ctx context.Context, params CompleteAuditSupplementUploadSessionParams) (*CompleteTaskAssetUploadSessionResult, *domain.AppError) {
	task, appErr := s.requireTask(ctx, params.TaskID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.authorizeAuditSupplementWrite(ctx, task); appErr != nil {
		return nil, appErr
	}
	request, appErr := s.requireUploadRequest(ctx, params.TaskID, params.SessionID)
	if appErr != nil {
		return nil, appErr
	}
	if !isAuditSupplementUploadRequest(request) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session is not an audit supplement session", map[string]interface{}{
			"deny_code":         "upload_session_not_audit_supplement",
			"upload_session_id": request.RequestID,
		})
	}
	if request.Status == domain.UploadRequestStatusBound || (request.SessionStatus == domain.DesignAssetSessionStatusCompleted && request.BoundAssetID != nil) {
		return s.buildCompletedUploadSessionResult(ctx, params.TaskID, request)
	}
	if request.SessionStatus == domain.DesignAssetSessionStatusCancelled || request.SessionStatus == domain.DesignAssetSessionStatusExpired {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload_session is already terminal", nil)
	}
	if request.TaskAssetType == nil || domain.NormalizeTaskAssetType(*request.TaskAssetType) != domain.TaskAssetTypeDelivery {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "audit supplement only supports delivery assets", map[string]interface{}{
			"deny_code": "audit_supplement_asset_type_not_allowed",
		})
	}
	if request.ExpectedSize == nil || *request.ExpectedSize <= 0 || *request.ExpectedSize > taskAssetUploadMaxFileSizeBytes {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session expected_size is outside the allowed range", map[string]interface{}{
			"max_bytes": taskAssetUploadMaxFileSizeBytes,
		})
	}
	if appErr := validateUploadContentTypeContract(request, params.UploadContentType); appErr != nil {
		return nil, appErr
	}
	if appErr := validateOSSDirectCompleteContract(CompleteTaskAssetUploadSessionParams{
		OSSParts:     params.OSSParts,
		OSSUploadID:  params.OSSUploadID,
		OSSObjectKey: params.OSSObjectKey,
	}); appErr != nil {
		return nil, appErr
	}

	reason := firstNonEmpty(strings.TrimSpace(params.Reason), auditSupplementReasonFromRemark(params.Remark), auditSupplementReasonFromRemark(request.Remark))
	if reason == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "reason is required", map[string]interface{}{
			"deny_code": "audit_supplement_reason_required",
		})
	}
	scopeSKUCode := strings.TrimSpace(request.TargetSKUCode)
	checksumHint := firstNonEmpty(strings.TrimSpace(params.FileHash), strings.TrimSpace(request.ChecksumHint))
	var err error
	ossDirectReady := s.canFinalizeOSSDirectUpload(CompleteTaskAssetUploadSessionParams{
		OSSParts:     params.OSSParts,
		OSSUploadID:  params.OSSUploadID,
		OSSObjectKey: params.OSSObjectKey,
	})
	if request.RemoteUploadID != "" && request.SessionStatus == domain.DesignAssetSessionStatusCreated && !ossDirectReady {
		if request, err = s.syncUploadRequestFromRemote(ctx, request); err != nil {
			return nil, infraError("sync audit supplement upload session before completion", err)
		}
	}
	ossDirectFinalized := false
	ossDirectObjectKey := ""
	if ossDirectReady {
		ossObjectKey := strings.TrimSpace(params.OSSObjectKey)
		ossUploadID := strings.TrimSpace(params.OSSUploadID)
		legacyTaskPrefix := "tasks/" + strings.Trim(strings.TrimSpace(task.TaskNo), "/") + "/assets/"
		if !strings.HasPrefix(ossObjectKey, legacyTaskPrefix) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "oss_object_key does not belong to upload_session", map[string]interface{}{
				"upload_session_id": request.RequestID,
			})
		}
		multipartCompleteErr := s.ossDirectService.CompleteMultipartUpload(ctx, ossObjectKey, ossUploadID, params.OSSParts)
		info, exists, statErr := s.ossDirectService.StatObject(ctx, ossObjectKey)
		if statErr != nil {
			return nil, infraError("verify completed audit supplement OSS object", statErr)
		}
		if !exists || info == nil {
			if multipartCompleteErr != nil {
				return nil, infraError("complete audit supplement oss direct multipart upload", multipartCompleteErr)
			}
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "completed audit supplement OSS object is missing", nil)
		}
		if info.ContentLength < 0 || info.ContentLength != *request.ExpectedSize {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "completed audit supplement OSS object size does not match upload_session", map[string]interface{}{
				"expected_size": *request.ExpectedSize,
				"actual_size":   info.ContentLength,
			})
		}
		if multipartCompleteErr != nil {
			log.Printf("oss_direct_audit_supplement_complete_recovered session=%s object_key=%s error=%v", request.RequestID, ossObjectKey, multipartCompleteErr)
		}
		ossDirectFinalized = true
		ossDirectObjectKey = ossObjectKey
	}
	meta, appErr := s.resolveCompletedUploadMeta(ctx, request, checksumHint, ossDirectObjectKey, ossDirectFinalized)
	if appErr != nil {
		return nil, appErr
	}

	assetsBefore, err := s.taskAssetRepo.ListByTaskID(ctx, params.TaskID)
	if err != nil {
		return nil, infraError("list task assets before audit supplement", err)
	}
	auditDeliveryCountBefore := countDeliveryAssetsBySourceModule(assetsBefore, domain.ModuleKeyAudit)
	designDeliveryCount := countDeliveryAssetsBySourceModule(assetsBefore, domain.ModuleKeyDesign)
	now := s.nowFn().UTC()
	lastSyncedAt := now
	resolvedStorageKey := buildRemoteStorageKey(meta, request)
	storageRefID := uuid.NewString()
	var assetID int64
	var versionID int64
	var assetVersionNo int
	var timelineVersionNo int
	alreadyCompleted := false

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		lockedRequest, err := s.getUploadRequestForUpdate(ctx, tx, request.RequestID)
		if err != nil {
			return fmt.Errorf("lock audit supplement upload request before completion: %w", err)
		}
		if lockedRequest == nil || lockedRequest.TaskID != params.TaskID {
			return domain.ErrNotFound
		}
		request = lockedRequest
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
		if request.AssetID == nil || *request.AssetID <= 0 {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "audit supplement upload_session is missing frozen asset identity", map[string]interface{}{
				"upload_session_id": request.RequestID,
			})
		}
		existingAsset, err := s.designAssetRepo.GetByID(ctx, *request.AssetID)
		if err != nil {
			return fmt.Errorf("get audit supplement design asset: %w", err)
		}
		if existingAsset == nil || existingAsset.TaskID != params.TaskID {
			return domain.ErrNotFound
		}
		if existingAsset.CurrentVersionID != nil {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "audit supplement asset already has a current version", map[string]interface{}{
				"asset_id":          existingAsset.ID,
				"upload_session_id": request.RequestID,
			})
		}
		assetID = existingAsset.ID
		timelineVersionNo, err = s.taskAssetRepo.NextVersionNo(ctx, tx, params.TaskID)
		if err != nil {
			return fmt.Errorf("next audit supplement task asset timeline version: %w", err)
		}
		assetVersionNo, err = s.taskAssetRepo.NextAssetVersionNo(ctx, tx, assetID)
		if err != nil {
			return fmt.Errorf("next audit supplement asset version: %w", err)
		}
		uploadStatus := string(domain.DesignAssetUploadStatusUploaded)
		previewStatus := string(domain.DesignAssetPreviewStatusNotApplicable)
		taskAsset := &domain.TaskAsset{
			TaskID:           params.TaskID,
			AssetID:          &assetID,
			ScopeSKUCode:     optionalStringPtr(scopeSKUCode),
			AssetType:        domain.TaskAssetTypeDelivery,
			VersionNo:        timelineVersionNo,
			AssetVersionNo:   &assetVersionNo,
			UploadMode:       optionalStringPtr(string(request.UploadMode)),
			UploadRequestID:  &request.RequestID,
			StorageRefID:     &storageRefID,
			FileName:         request.FileName,
			OriginalName:     optionalStringPtr(request.FileName),
			RemoteFileID:     meta.FileID,
			MimeType:         optionalStringPtr(firstNonEmpty(meta.MimeType, request.MimeType)),
			FileSize:         firstNonNilInt64(meta.FileSize, request.ExpectedSize, request.FileSize),
			StorageKey:       optionalStringPtr(resolvedStorageKey),
			WholeHash:        meta.FileHash,
			UploadStatus:     &uploadStatus,
			PreviewStatus:    &previewStatus,
			UploadedBy:       params.CompletedBy,
			UploadedAt:       &now,
			Remark:           reason,
			SourceModuleKey:  domain.ModuleKeyAudit,
			FlowReviewStatus: domain.TaskAssetFlowReviewStatusApproved,
			ApprovedAt:       &now,
			ApprovedBy:       &params.CompletedBy,
		}
		id, err := s.taskAssetRepo.Create(ctx, tx, taskAsset)
		if err != nil {
			return fmt.Errorf("create audit supplement task asset version: %w", err)
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
			return fmt.Errorf("create audit supplement asset storage ref: %w", err)
		}
		if err := s.designAssetRepo.UpdateCurrentVersionID(ctx, tx, assetID, &versionID); err != nil {
			return fmt.Errorf("update audit supplement design asset current version: %w", err)
		}
		if err := s.uploadRequestRepo.UpdateBinding(ctx, tx, request.RequestID, &versionID, storageRefID, domain.UploadRequestStatusBound, buildAuditSupplementRemark(reason)); err != nil {
			return fmt.Errorf("update audit supplement upload request binding: %w", err)
		}
		if err := s.uploadRequestRepo.UpdateSession(ctx, tx, repo.UploadRequestSessionUpdate{
			RequestID:      request.RequestID,
			AssetID:        &assetID,
			SessionStatus:  domain.DesignAssetSessionStatusCompleted,
			RemoteUploadID: request.RemoteUploadID,
			RemoteFileID:   meta.FileID,
			LastSyncedAt:   &lastSyncedAt,
			Remark:         buildAuditSupplementRemark(reason),
		}); err != nil {
			return fmt.Errorf("update audit supplement upload request session: %w", err)
		}
		eventPayload := auditSupplementEventPayload(assetID, versionID, assetVersionNo, timelineVersionNo, request, reason, scopeSKUCode, designDeliveryCount, auditDeliveryCountBefore, auditDeliveryCountBefore+1, meta, resolvedStorageKey)
		if _, err := s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventAssetVersionCreated, &params.CompletedBy, eventPayload); err != nil {
			return fmt.Errorf("append audit supplement asset version event: %w", err)
		}
		if _, err := s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventAssetUploadSessionCompleted, &params.CompletedBy, eventPayload); err != nil {
			return fmt.Errorf("append audit supplement upload completed event: %w", err)
		}
		if _, err := s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventAuditSupplementUploaded, &params.CompletedBy, eventPayload); err != nil {
			return fmt.Errorf("append audit supplement uploaded event: %w", err)
		}
		return nil
	})
	if txErr != nil {
		log.Printf("complete_audit_supplement_upload_session_tx_failed trace_id=%s task_id=%d session_id=%s asset_id=%v err=%v",
			domain.TraceIDFromContext(ctx), params.TaskID, request.RequestID, request.AssetID, txErr)
		if appErr, ok := txErr.(*domain.AppError); ok {
			return nil, appErr
		}
		if isTaskAssetVersionConflict(txErr) {
			s.logAssetVersionConflict(ctx, params.TaskID, request, timelineVersionNo, txErr, "")
			return nil, assetVersionRaceConflictAppError(params.TaskID, request.RequestID, timelineVersionNo)
		}
		return nil, infraError("complete audit supplement upload session", txErr)
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
	if appErr := requireCompletedTaskAssetMutationActor(ctx, task); appErr != nil {
		return nil, appErr
	}
	if appErr := s.requireCustomizationReviewerUploadSessionSource(ctx, task, request); appErr != nil {
		return nil, appErr
	}
	legacyRetouchCompletion, appErr := s.isCompletedLegacyRetouchCompletion(ctx, task, request)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := validateCompletedTaskReplacementRequest(task, request, legacyRetouchCompletion); appErr != nil {
		return nil, appErr
	}
	var expectedCurrentVersionID *int64
	if task.TaskStatus == domain.TaskStatusCompleted && request.AssetID != nil {
		if legacyRetouchCompletion {
			expectedCurrentVersionID = nil
		} else {
			expectedCurrentVersionID, appErr = s.completedTaskReplacementCurrentVersionID(ctx, task, request.AssetID)
			if appErr != nil {
				return nil, appErr
			}
		}
	}
	if request.Status == domain.UploadRequestStatusBound || (request.SessionStatus == domain.DesignAssetSessionStatusCompleted && request.BoundAssetID != nil) {
		return s.buildCompletedUploadSessionResult(ctx, params.TaskID, request)
	}
	if request.SessionStatus == domain.DesignAssetSessionStatusCancelled || request.SessionStatus == domain.DesignAssetSessionStatusExpired {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "upload_session is already terminal", nil)
	}
	if request.TaskAssetType == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session asset_type is required", nil)
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

	checksumHint := firstNonEmpty(strings.TrimSpace(params.FileHash), strings.TrimSpace(request.ChecksumHint))
	var err error
	ossDirectReady := s.canFinalizeOSSDirectUpload(params)
	isOSSDirectSession := strings.TrimSpace(request.RemoteUploadID) == "" && s.ossDirectService != nil && s.ossDirectService.Enabled()
	if isOSSDirectSession && !ossDirectReady {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "oss_object_key is required for OSS direct upload_session completion", nil)
	}
	if request.RemoteUploadID != "" && request.SessionStatus == domain.DesignAssetSessionStatusCreated && !ossDirectReady {
		if request, err = s.syncUploadRequestFromRemote(ctx, request); err != nil {
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
		if task.TaskStatus == domain.TaskStatusCompleted {
			lockedTask, err := s.getTaskForUpdate(ctx, tx, params.TaskID)
			if err != nil {
				return fmt.Errorf("lock completed task before asset replacement: %w", err)
			}
			if lockedTask == nil {
				return domain.ErrNotFound
			}
			if lockedTask.TaskStatus != domain.TaskStatusCompleted {
				return domain.NewAppError(domain.ErrCodeConflict, "task status changed before completed asset replacement", map[string]interface{}{
					"task_id":         task.ID,
					"expected_status": domain.TaskStatusCompleted,
					"actual_status":   lockedTask.TaskStatus,
				})
			}
			task = lockedTask
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
		if appErr := validateCompletedTaskReplacementRequest(task, request, legacyRetouchCompletion); appErr != nil {
			return appErr
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
			if task.TaskStatus == domain.TaskStatusCompleted {
				if !optionalInt64Equal(existingAsset.CurrentVersionID, expectedCurrentVersionID) {
					return completedReplacementConcurrentChangeError(task, assetID, expectedCurrentVersionID, existingAsset.CurrentVersionID)
				}
				if !legacyRetouchCompletion {
					current, err := s.getTaskAssetForUpdate(ctx, tx, *expectedCurrentVersionID)
					if err != nil {
						return fmt.Errorf("lock completed task replacement current version: %w", err)
					}
					if !isUsableCompletedReplacementCurrentVersion(current, task.ID, existingAsset.ID) {
						return completedReplacementCurrentAssetRequiredError(task, existingAsset.ID)
					}
				}
			}
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
			if task.TaskStatus == domain.TaskStatusCompleted {
				flowReviewStatus = domain.TaskAssetFlowReviewStatusApproved
				approvedAt = &now
				approvedBy = &params.CompletedBy
			}
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
			SourceModuleKey:      designAssetSourceModuleKeyForTask(task, requestAssetType),
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
			if err := s.insertRetouchRequirementReferenceFlat(ctx, tx, params.TaskID, retouchRequirementID, storageRefID); err != nil {
				return err
			}
		}
		if task.TaskStatus == domain.TaskStatusCompleted && request.AssetID != nil {
			if casRepo, ok := s.designAssetRepo.(designAssetCurrentVersionCASRepo); ok {
				updated, err := casRepo.UpdateCurrentVersionIDCAS(ctx, tx, assetID, expectedCurrentVersionID, &versionID)
				if err != nil {
					return err
				}
				if !updated {
					return completedReplacementConcurrentChangeError(task, assetID, expectedCurrentVersionID, nil)
				}
			} else if err := s.designAssetRepo.UpdateCurrentVersionID(ctx, tx, assetID, &versionID); err != nil {
				return fmt.Errorf("update design asset current version: %w", err)
			}
		} else if err := s.designAssetRepo.UpdateCurrentVersionID(ctx, tx, assetID, &versionID); err != nil {
			return fmt.Errorf("update design asset current version: %w", err)
		}
		if previousCurrentVersionID != nil && *previousCurrentVersionID > 0 && *previousCurrentVersionID != versionID {
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
		shouldAppendDesignSubmitted := false
		if requestAssetType.IsDelivery() {
			if task.CustomizationRequired && task.TaskStatus == domain.TaskStatusPendingCustomizationProduction &&
				!submitDesignActorCanUseCustomizationLane(ctx) {
				return domain.NewAppError(domain.ErrCodePermissionDenied, "customization submit-design requires a customization operator, operation, or management role", map[string]interface{}{
					"task_id":   task.ID,
					"deny_code": "missing_customization_submit_role",
					"action":    string(TaskActionSubmitDesign),
				})
			}
			switch task.TaskStatus {
			case domain.TaskStatusPendingAssign, domain.TaskStatusAssigned, domain.TaskStatusInProgress, domain.TaskStatusRejectedByAuditA, domain.TaskStatusRejectedByAuditB, domain.TaskStatusPendingCustomizationProduction:
				advance, gateErr := s.shouldAdvanceTaskToPendingAuditA(ctx, task, scopeSKUCode, request)
				if gateErr != nil {
					return fmt.Errorf("check design submit gate: %w", gateErr)
				}
				if advance {
					transition := designSubmissionTransitionForTask(task)
					if err := s.taskRepo.UpdateStatus(ctx, tx, params.TaskID, transition.TaskStatus); err != nil {
						return fmt.Errorf("advance task status after delivery upload: %w", err)
					}
					if err := s.taskRepo.UpdateHandler(ctx, tx, params.TaskID, nil); err != nil {
						return fmt.Errorf("clear current handler after delivery upload: %w", err)
					}
					if err := s.markDesignSubmissionModuleState(ctx, tx, params.TaskID, transition); err != nil {
						return fmt.Errorf("mark design module submitted after delivery upload: %w", err)
					}
					if err := applyDesignSubmissionWorkflow(ctx, tx, s.workflowRules, task, transition, params.CompletedBy); err != nil {
						return fmt.Errorf("apply design submission workflow after delivery upload: %w", err)
					}
					if err := syncCustomizationDesignSubmission(ctx, tx, s.taskRepo, s.customizationJobRepo, task, params.CompletedBy); err != nil {
						return fmt.Errorf("sync customization submission after delivery upload: %w", err)
					}
					shouldAppendDesignSubmitted = true
				}
			}
		}
		_, err = s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventAssetVersionCreated, &params.CompletedBy, map[string]interface{}{
			"asset_id":               assetID,
			"asset_type":             string(requestAssetType),
			"target_sku_code":        scopeSKUCode,
			"asset_version_id":       versionID,
			"asset_version_no":       assetVersionNo,
			"timeline_version":       timelineVersionNo,
			"upload_session_id":      request.RequestID,
			"remote_file_id":         meta.FileID,
			"storage_key":            resolvedStorageKey,
			"remark":                 taskAsset.Remark,
			"post_close_replacement": task.TaskStatus == domain.TaskStatusCompleted,
		})
		if err != nil {
			return fmt.Errorf("append asset version created event: %w", err)
		}
		_, err = s.taskEventRepo.Append(ctx, tx, params.TaskID, domain.TaskEventAssetUploadSessionCompleted, &params.CompletedBy, map[string]interface{}{
			"asset_id":               assetID,
			"asset_type":             string(requestAssetType),
			"target_sku_code":        scopeSKUCode,
			"asset_version_id":       versionID,
			"asset_version_no":       assetVersionNo,
			"timeline_version":       timelineVersionNo,
			"upload_session_id":      request.RequestID,
			"upload_mode":            string(request.UploadMode),
			"storage_provider":       string(request.StorageProvider),
			"remote_upload_id":       request.RemoteUploadID,
			"remote_file_id":         meta.FileID,
			"storage_key":            resolvedStorageKey,
			"file_hash":              meta.FileHash,
			"remark":                 taskAsset.Remark,
			"post_close_replacement": task.TaskStatus == domain.TaskStatusCompleted,
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

func (s *taskAssetCenterService) markDesignModuleSubmitted(ctx context.Context, tx repo.Tx, taskID int64) error {
	return s.markDesignSubmissionModuleState(ctx, tx, taskID, designSubmissionTransitionForTask(nil))
}

func (s *taskAssetCenterService) markDesignSubmissionModuleState(ctx context.Context, tx repo.Tx, taskID int64, transition designSubmissionTransition) error {
	if s.taskModuleRepo == nil {
		return nil
	}
	return s.taskModuleRepo.UpdateState(ctx, tx, taskID, transition.ModuleKey, transition.ModuleState, transition.ModuleTerminal, nil)
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
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionAssetUploadSessionCancel, task, "", "")
	authz.logDecision(TaskActionAssetUploadSessionCancel, decision)
	if !decision.Allowed {
		return nil, taskActionDecisionAppError(TaskActionAssetUploadSessionCancel, decision)
	}
	if appErr := requireCompletedTaskAssetMutationActor(ctx, task); appErr != nil {
		return nil, appErr
	}
	if appErr := s.requireCustomizationReviewerUploadSessionSource(ctx, task, request); appErr != nil {
		return nil, appErr
	}
	if appErr := validateCompletedTaskReplacementRequest(task, request, false); appErr != nil {
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
		if appErr := validateCompletedTaskReplacementRequest(task, request, false); appErr != nil {
			return appErr
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
	if isAuditSupplementUploadPolicy(params.UploadPolicy) {
		return s.createAuditSupplementUploadSession(ctx, task, params, mode)
	}
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionAssetUploadSessionCreate, task, "", "")
	authz.logDecision(TaskActionAssetUploadSessionCreate, decision)
	if !decision.Allowed {
		return nil, taskActionDecisionAppError(TaskActionAssetUploadSessionCreate, decision)
	}
	if appErr := requireCompletedTaskAssetMutationActor(ctx, task); appErr != nil {
		return nil, appErr
	}
	if params.AssetID == nil {
		revisionAssetID, appErr := s.resolveRejectedDeliveryRevisionAssetID(ctx, task, params)
		if appErr != nil {
			return nil, appErr
		}
		params.AssetID = revisionAssetID
	}
	if appErr := validateCompletedTaskReplacementIntent(task, params.AssetID, params.AssetType); appErr != nil {
		return nil, appErr
	}
	if appErr := s.validateCompletedTaskReplacementCurrentAsset(ctx, task, params.AssetID); appErr != nil {
		return nil, appErr
	}
	if appErr := validateAuditStageUploadAssetType(task, params.AssetType, params.OwnerModuleKey, params.AssetID); appErr != nil {
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
			"post_close_replacement": task.TaskStatus == domain.TaskStatusCompleted,
		})
		return err
	})
	if txErr != nil {
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
	if task == nil {
		return false
	}
	switch task.TaskStatus {
	case domain.TaskStatusPendingAuditA:
	case domain.TaskStatusCompleted:
		if task.TaskType != domain.TaskTypeRetouchTask {
			return false
		}
	default:
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

func isAuditSupplementUploadPolicy(policy string) bool {
	return strings.TrimSpace(policy) == auditSupplementUploadPolicy
}

func buildAuditSupplementRemark(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return auditSupplementRemarkPrefix
	}
	return auditSupplementRemarkPrefix + " " + reason
}

func auditSupplementReasonFromRemark(remark string) string {
	remark = strings.TrimSpace(remark)
	if remark == "" {
		return ""
	}
	if strings.HasPrefix(remark, auditSupplementRemarkPrefix) {
		return strings.TrimSpace(strings.TrimPrefix(remark, auditSupplementRemarkPrefix))
	}
	return remark
}

func isAuditSupplementUploadRequest(request *domain.UploadRequest) bool {
	if request == nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(request.Remark), auditSupplementRemarkPrefix)
}

func (s *taskAssetCenterService) authorizeAuditSupplementRead(ctx context.Context, task *domain.Task) *domain.AppError {
	if task == nil {
		return domain.ErrNotFound
	}
	actor, ok := resolveTaskActionActor(ctx)
	if !ok {
		return domain.NewAppError(domain.ErrCodeUnauthorized, "actor context required", nil)
	}
	if !hasAnyRoleValue(actor.Roles,
		domain.RoleAuditA,
		domain.RoleAuditB,
		domain.RoleAdmin,
		domain.RoleSuperAdmin,
		domain.RoleHRAdmin,
		domain.RoleRoleAdmin,
		domain.RoleDeptAdmin,
		domain.RoleTeamLead,
		domain.RoleDesignDirector,
	) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "audit supplement requires an audit or management role", map[string]interface{}{
			"deny_code":   "audit_supplement_missing_role",
			"task_id":     task.ID,
			"actor_id":    actor.ID,
			"actor_roles": actor.Roles,
		})
	}
	return s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionReadDetail, task)
}

func (s *taskAssetCenterService) authorizeAuditSupplementWrite(ctx context.Context, task *domain.Task) *domain.AppError {
	return s.authorizeAuditSupplementAccess(ctx, task, true)
}

func (s *taskAssetCenterService) authorizeAuditSupplementAccess(ctx context.Context, task *domain.Task, requireCompleted bool) *domain.AppError {
	if task == nil {
		return domain.ErrNotFound
	}
	if requireCompleted && task.TaskStatus != domain.TaskStatusCompleted {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "audit supplement is only allowed after task completion", map[string]interface{}{
			"deny_code":   "audit_supplement_task_not_completed",
			"task_id":     task.ID,
			"task_status": string(task.TaskStatus),
		})
	}
	actor, ok := resolveTaskActionActor(ctx)
	if !ok {
		return domain.NewAppError(domain.ErrCodeUnauthorized, "actor context required", nil)
	}
	if !hasAnyRoleValue(actor.Roles,
		domain.RoleAuditA,
		domain.RoleAuditB,
		domain.RoleAdmin,
		domain.RoleSuperAdmin,
		domain.RoleHRAdmin,
		domain.RoleRoleAdmin,
		domain.RoleDeptAdmin,
		domain.RoleTeamLead,
		domain.RoleDesignDirector,
	) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "audit supplement requires an audit or management role", map[string]interface{}{
			"deny_code":   "audit_supplement_missing_role",
			"task_id":     task.ID,
			"actor_id":    actor.ID,
			"actor_roles": actor.Roles,
		})
	}
	if hasAnyRoleValue(actor.Roles, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin) {
		return nil
	}
	scopeEval := evaluateTaskActionScope(actor, task, task.OwnerDepartment, task.OwnerOrgTeam)
	if hasRoleValue(actor.Roles, domain.RoleDeptAdmin) &&
		(scopeEval.Has(TaskActionScopeDepartment) || scopeEval.Has(TaskActionScopeManagedDepartment)) {
		return nil
	}
	if hasRoleValue(actor.Roles, domain.RoleDesignDirector) &&
		(scopeEval.Has(TaskActionScopeDepartment) || scopeEval.Has(TaskActionScopeManagedDepartment)) {
		return nil
	}
	if hasRoleValue(actor.Roles, domain.RoleTeamLead) &&
		(scopeEval.Has(TaskActionScopeTeam) || scopeEval.Has(TaskActionScopeManagedTeam)) {
		return nil
	}
	if hasAnyRoleValue(actor.Roles, domain.RoleAuditA, domain.RoleAuditB) {
		if s.auditV7Repo == nil {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "audit supplement requires audit history verification", map[string]interface{}{
				"deny_code": "audit_supplement_audit_history_unavailable",
				"task_id":   task.ID,
				"actor_id":  actor.ID,
			})
		}
		records, err := s.auditV7Repo.ListRecordsByTaskID(ctx, task.ID)
		if err != nil {
			return infraError("list audit records for supplement authorization", err)
		}
		for _, record := range records {
			if record != nil && record.AuditorID == actor.ID {
				return nil
			}
		}
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "audit supplement is outside the actor audit history or organization scope", map[string]interface{}{
		"deny_code":      "audit_supplement_scope_denied",
		"task_id":        task.ID,
		"actor_id":       actor.ID,
		"owner_org_team": task.OwnerOrgTeam,
	})
}

func countDeliveryAssetsBySourceModule(assets []*domain.TaskAsset, sourceModuleKey string) int {
	sourceModuleKey = strings.TrimSpace(sourceModuleKey)
	count := 0
	for _, asset := range assets {
		if asset == nil || domain.NormalizeTaskAssetType(asset.AssetType) != domain.TaskAssetTypeDelivery {
			continue
		}
		moduleKey := strings.TrimSpace(asset.SourceModuleKey)
		if moduleKey == "" {
			moduleKey = domain.ModuleKeyDesign
		}
		if moduleKey == sourceModuleKey {
			count++
		}
	}
	return count
}

func auditSupplementEventPayload(
	assetID int64,
	versionID int64,
	assetVersionNo int,
	timelineVersionNo int,
	request *domain.UploadRequest,
	reason string,
	scopeSKUCode string,
	designDeliveryCount int,
	auditDeliveryCountBefore int,
	auditDeliveryCountAfter int,
	meta *RemoteFileMeta,
	storageKey string,
) map[string]interface{} {
	payload := map[string]interface{}{
		"asset_id":                    assetID,
		"asset_type":                  string(domain.TaskAssetTypeDelivery),
		"asset_version_id":            versionID,
		"asset_version_no":            assetVersionNo,
		"timeline_version":            timelineVersionNo,
		"upload_policy":               auditSupplementUploadPolicy,
		"source_module_key":           domain.ModuleKeyAudit,
		"supplement_after_completed":  true,
		"reason":                      strings.TrimSpace(reason),
		"target_sku_code":             strings.TrimSpace(scopeSKUCode),
		"design_delivery_count":       designDeliveryCount,
		"audit_delivery_count_before": auditDeliveryCountBefore,
		"audit_delivery_count_after":  auditDeliveryCountAfter,
		"storage_key":                 strings.TrimSpace(storageKey),
	}
	if request != nil {
		payload["upload_session_id"] = request.RequestID
		payload["filename"] = request.FileName
		payload["upload_mode"] = string(request.UploadMode)
		payload["storage_provider"] = string(request.StorageProvider)
		payload["remote_upload_id"] = request.RemoteUploadID
	}
	if meta != nil {
		payload["remote_file_id"] = meta.FileID
		payload["file_hash"] = meta.FileHash
		payload["mime_type"] = strings.TrimSpace(meta.MimeType)
		payload["file_size"] = meta.FileSize
	}
	return payload
}

type auditSupplementEventPayloadJSON struct {
	AssetID                  int64  `json:"asset_id"`
	AssetVersionID           int64  `json:"asset_version_id"`
	AssetVersionNo           int    `json:"asset_version_no"`
	TimelineVersion          int    `json:"timeline_version"`
	UploadSessionID          string `json:"upload_session_id"`
	Filename                 string `json:"filename"`
	Reason                   string `json:"reason"`
	TargetSKUCode            string `json:"target_sku_code"`
	DesignDeliveryCount      int    `json:"design_delivery_count"`
	AuditDeliveryCountBefore int    `json:"audit_delivery_count_before"`
	AuditDeliveryCountAfter  int    `json:"audit_delivery_count_after"`
}

func auditSupplementItemFromEvent(event *domain.TaskEvent) (AuditSupplementItem, bool) {
	if event == nil {
		return AuditSupplementItem{}, false
	}
	var payload auditSupplementEventPayloadJSON
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return AuditSupplementItem{}, false
	}
	return AuditSupplementItem{
		EventID:          event.ID,
		Sequence:         event.Sequence,
		TaskID:           event.TaskID,
		AssetID:          payload.AssetID,
		AssetVersionID:   payload.AssetVersionID,
		AssetVersionNo:   payload.AssetVersionNo,
		TimelineVersion:  payload.TimelineVersion,
		UploadSessionID:  strings.TrimSpace(payload.UploadSessionID),
		Filename:         strings.TrimSpace(payload.Filename),
		Reason:           strings.TrimSpace(payload.Reason),
		TargetSKUCode:    strings.TrimSpace(payload.TargetSKUCode),
		UploadedBy:       auditSupplementOperatorID(event.OperatorID),
		UploadedByName:   strings.TrimSpace(event.OperatorName),
		AuditCountBefore: payload.AuditDeliveryCountBefore,
		AuditCountAfter:  payload.AuditDeliveryCountAfter,
		DesignCount:      payload.DesignDeliveryCount,
		CreatedAt:        event.CreatedAt,
	}, true
}

func auditSupplementOperatorID(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
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

func validateAuditStageUploadAssetType(task *domain.Task, assetType domain.TaskAssetType, ownerModuleKey string, assetID *int64) *domain.AppError {
	if task == nil {
		return nil
	}
	normalized := domain.NormalizeTaskAssetType(assetType)
	if isCustomizationReviewTaskStatus(task.TaskStatus) {
		if normalized == domain.TaskAssetTypeSource {
			return nil
		}
		if normalized == domain.TaskAssetTypeDelivery && assetID != nil && *assetID > 0 {
			return nil
		}
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "customization review uploads only support source assets or replacement of an existing delivery asset", map[string]interface{}{
			"deny_code":           "customization_review_asset_type_not_allowed",
			"task_status":         string(task.TaskStatus),
			"asset_type":          string(assetType),
			"allowed_asset_types": []string{string(domain.TaskAssetTypeSource)},
			"replaceable_asset_types": []string{
				string(domain.TaskAssetTypeDelivery),
			},
		})
	}
	if !isAuditStageTaskStatus(task.TaskStatus) {
		return nil
	}
	if normalized == domain.TaskAssetTypeSource || normalized == domain.TaskAssetTypeDelivery {
		return nil
	}
	if normalized == domain.TaskAssetTypeReference && strings.TrimSpace(ownerModuleKey) == string(domain.ModuleKeyBasicInfo) {
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

func validateCompletedTaskReplacementIntent(task *domain.Task, assetID *int64, assetType domain.TaskAssetType) *domain.AppError {
	if task == nil || task.TaskStatus != domain.TaskStatusCompleted {
		return nil
	}
	if assetID == nil || *assetID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "completed tasks only allow replacing an existing current asset", map[string]interface{}{
			"deny_code":   "post_close_replacement_requires_asset_id",
			"task_id":     task.ID,
			"task_status": task.TaskStatus,
		})
	}
	normalized := domain.NormalizeTaskAssetType(assetType)
	if normalized.IsReference() || normalized.IsSource() || normalized.IsDelivery() {
		return nil
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "completed task replacement only supports reference, source, or delivery assets", map[string]interface{}{
		"deny_code":           "post_close_replacement_asset_type_not_allowed",
		"task_id":             task.ID,
		"task_status":         task.TaskStatus,
		"asset_type":          normalized,
		"allowed_asset_types": []string{string(domain.TaskAssetTypeReference), string(domain.TaskAssetTypeSource), string(domain.TaskAssetTypeDelivery)},
	})
}

func validateCompletedTaskReplacementRequest(task *domain.Task, request *domain.UploadRequest, allowLegacyRetouchCompletion bool) *domain.AppError {
	if task == nil || task.TaskStatus != domain.TaskStatusCompleted {
		return nil
	}
	if allowLegacyRetouchCompletion && task.TaskType == domain.TaskTypeRetouchTask && isPrecreatedCompletableUploadSession(request) {
		return nil
	}
	if request == nil || request.TaskAssetType == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "completed task replacement upload session is missing asset type", map[string]interface{}{
			"deny_code": "post_close_replacement_missing_asset_type",
			"task_id":   task.ID,
		})
	}
	return validateCompletedTaskReplacementIntent(task, request.AssetID, *request.TaskAssetType)
}

func (s *taskAssetCenterService) validateCompletedTaskReplacementCurrentAsset(ctx context.Context, task *domain.Task, assetID *int64) *domain.AppError {
	_, appErr := s.completedTaskReplacementCurrentVersionID(ctx, task, assetID)
	return appErr
}

func (s *taskAssetCenterService) completedTaskReplacementCurrentVersionID(ctx context.Context, task *domain.Task, assetID *int64) (*int64, *domain.AppError) {
	if task == nil || task.TaskStatus != domain.TaskStatusCompleted || assetID == nil || *assetID <= 0 {
		return nil, nil
	}
	asset, appErr := s.requireDesignAsset(ctx, task.ID, *assetID)
	if appErr != nil {
		return nil, appErr
	}
	if asset.CurrentVersionID == nil || *asset.CurrentVersionID <= 0 {
		return nil, completedReplacementCurrentAssetRequiredError(task, *assetID)
	}
	current, err := s.taskAssetRepo.GetByID(ctx, *asset.CurrentVersionID)
	if err != nil {
		return nil, infraError("get completed task replacement current asset version", err)
	}
	if !isUsableCompletedReplacementCurrentVersion(current, task.ID, asset.ID) {
		return nil, completedReplacementCurrentAssetRequiredError(task, *assetID)
	}
	return domain.CloneInt64Ptr(asset.CurrentVersionID), nil
}

func isUsableCompletedReplacementCurrentVersion(current *domain.TaskAsset, taskID, assetID int64) bool {
	if current == nil || current.TaskID != taskID || current.AssetID == nil || *current.AssetID != assetID {
		return false
	}
	if current.IsArchived || current.ArchivedAt != nil || current.CleanedAt != nil || current.DeletedAt != nil {
		return false
	}
	if current.FlowReviewStatus == domain.TaskAssetFlowReviewStatusSuperseded || current.SupersededAt != nil || current.SupersededByVersionID != nil {
		return false
	}
	return true
}

func (s *taskAssetCenterService) isCompletedLegacyRetouchCompletion(ctx context.Context, task *domain.Task, request *domain.UploadRequest) (bool, *domain.AppError) {
	if task == nil || task.TaskStatus != domain.TaskStatusCompleted || task.TaskType != domain.TaskTypeRetouchTask || request == nil {
		return false, nil
	}
	if !isPrecreatedCompletableUploadSession(request) || request.Status != domain.UploadRequestStatusRequested || request.BoundAssetID != nil {
		return false, nil
	}
	// The exception is only for a session that predates the task's transition
	// to Completed. Sessions created after close must follow the normal
	// existing-current replacement contract.
	if request.CreatedAt.IsZero() || task.UpdatedAt.IsZero() || request.CreatedAt.After(task.UpdatedAt) {
		return false, nil
	}
	if request.AssetID == nil || *request.AssetID <= 0 {
		return true, nil
	}
	asset, appErr := s.requireDesignAsset(ctx, task.ID, *request.AssetID)
	if appErr != nil {
		return false, appErr
	}
	return asset.CurrentVersionID == nil || *asset.CurrentVersionID <= 0, nil
}

func requireCompletedTaskAssetMutationActor(ctx context.Context, task *domain.Task) *domain.AppError {
	if task == nil || task.TaskStatus != domain.TaskStatusCompleted {
		return nil
	}
	actor, ok := resolveTaskActionActor(ctx)
	if ok && actor != nil && hasAnyRoleValue(actor.Roles,
		domain.RoleDesigner,
		domain.RoleCustomizationOperator,
		domain.RoleCustomizationReviewer,
		domain.RoleAuditA,
		domain.RoleAuditB,
		domain.RoleOps,
		domain.RoleAssetManager,
		domain.RoleAdmin,
		domain.RoleSuperAdmin,
		domain.RoleHRAdmin,
		domain.RoleRoleAdmin,
		domain.RoleDeptAdmin,
		domain.RoleTeamLead,
		domain.RoleDesignDirector,
	) {
		return nil
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "completed task asset replacement requires a design, audit, customization review, operation, asset management, or management role", map[string]interface{}{
		"deny_code":   "post_close_replacement_role_not_allowed",
		"task_id":     task.ID,
		"task_status": task.TaskStatus,
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

func (s *taskAssetCenterService) getTaskAssetForUpdate(ctx context.Context, tx repo.Tx, versionID int64) (*domain.TaskAsset, error) {
	if lockingRepo, ok := s.taskAssetRepo.(taskAssetForUpdateRepo); ok {
		return lockingRepo.GetByIDForUpdate(ctx, tx, versionID)
	}
	return s.taskAssetRepo.GetByID(ctx, versionID)
}

func optionalInt64Equal(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func completedReplacementConcurrentChangeError(task *domain.Task, assetID int64, expected, actual *int64) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeConflict, "completed task replacement current asset changed concurrently; refresh and retry", map[string]interface{}{
		"deny_code":                   "post_close_replacement_current_changed",
		"task_id":                     task.ID,
		"task_status":                 task.TaskStatus,
		"asset_id":                    assetID,
		"expected_current_version_id": expected,
		"actual_current_version_id":   actual,
	})
}

func completedReplacementCurrentAssetRequiredError(task *domain.Task, assetID int64) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "completed tasks only allow replacing an existing current asset", map[string]interface{}{
		"deny_code":   "post_close_replacement_requires_current_asset",
		"task_id":     task.ID,
		"task_status": task.TaskStatus,
		"asset_id":    assetID,
	})
}

func isAuditStageTaskStatus(status domain.TaskStatus) bool {
	switch status {
	case domain.TaskStatusPendingAuditA, domain.TaskStatusPendingAuditB, domain.TaskStatusPendingOutsourceReview:
		return true
	default:
		return false
	}
}

func isCustomizationReviewTaskStatus(status domain.TaskStatus) bool {
	switch status {
	case domain.TaskStatusPendingCustomizationReview, domain.TaskStatusPendingEffectReview:
		return true
	default:
		return false
	}
}

func (s *taskAssetCenterService) requireCustomizationReviewerUploadSessionSource(ctx context.Context, task *domain.Task, request *domain.UploadRequest) *domain.AppError {
	if task == nil || request == nil || !isCustomizationReviewTaskStatus(task.TaskStatus) {
		return nil
	}
	actor, ok := resolveTaskActionActor(ctx)
	if !ok || !hasRoleValue(actor.Roles, domain.RoleCustomizationReviewer) || actorHasGenericAssetUploadRole(actor) {
		return nil
	}
	if request.TaskAssetType != nil && domain.NormalizeTaskAssetType(*request.TaskAssetType) == domain.TaskAssetTypeSource {
		return nil
	}
	if request.TaskAssetType != nil && domain.NormalizeTaskAssetType(*request.TaskAssetType) == domain.TaskAssetTypeDelivery && request.AssetID != nil && *request.AssetID > 0 {
		asset, err := s.designAssetRepo.GetByID(ctx, *request.AssetID)
		if err != nil {
			return infraError("get customization review replacement asset", err)
		}
		if asset != nil && asset.TaskID == task.ID && asset.AssetType.IsDelivery() && asset.CurrentVersionID != nil && *asset.CurrentVersionID > 0 {
			return nil
		}
	}
	assetType := ""
	if request.TaskAssetType != nil {
		assetType = string(*request.TaskAssetType)
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "customization reviewer upload sessions only support source assets or replacement of an existing delivery asset", map[string]interface{}{
		"deny_code":         "customization_review_upload_session_asset_type_not_allowed",
		"task_status":       string(task.TaskStatus),
		"upload_session_id": request.RequestID,
		"asset_type":        assetType,
		"allowed_asset_types": []string{
			string(domain.TaskAssetTypeSource),
		},
		"replaceable_asset_types": []string{
			string(domain.TaskAssetTypeDelivery),
		},
	})
}

func (s *taskAssetCenterService) resolveRejectedDeliveryRevisionAssetID(ctx context.Context, task *domain.Task, params CreateTaskAssetUploadSessionParams) (*int64, *domain.AppError) {
	if task == nil || !domain.NormalizeTaskAssetType(params.AssetType).IsDelivery() || !taskStatusCanReuseRejectedDelivery(task.TaskStatus) {
		return nil, nil
	}
	taskID := task.ID
	assetType := domain.TaskAssetTypeDelivery
	assets, err := s.designAssetRepo.List(ctx, repo.DesignAssetListFilter{
		TaskID:       &taskID,
		AssetType:    &assetType,
		ScopeSKUCode: strings.TrimSpace(params.TargetSKUCode),
	})
	if err != nil {
		return nil, infraError("list rejected delivery revision assets", err)
	}
	type candidate struct {
		asset   *domain.DesignAsset
		version *domain.TaskAsset
	}
	candidates := make([]candidate, 0, len(assets))
	exactFilename := make([]candidate, 0, 1)
	for _, asset := range assets {
		if asset == nil || strings.TrimSpace(asset.ScopeSKUCode) != strings.TrimSpace(params.TargetSKUCode) || !retouchRequirementIDsEqual(asset.RetouchRequirementID, params.RetouchRequirementID) || asset.CurrentVersionID == nil || *asset.CurrentVersionID <= 0 {
			continue
		}
		version, err := s.taskAssetRepo.GetByID(ctx, *asset.CurrentVersionID)
		if err != nil {
			return nil, infraError("get rejected delivery revision current version", err)
		}
		if version == nil || version.TaskID != task.ID || domain.NormalizeTaskAssetFlowReviewStatus(version.FlowReviewStatus, version.AssetType) != domain.TaskAssetFlowReviewStatusRejected || version.DeletedAt != nil || version.CleanedAt != nil {
			continue
		}
		item := candidate{asset: asset, version: version}
		candidates = append(candidates, item)
		if taskAssetFilenameMatches(version, params.Filename) {
			exactFilename = append(exactFilename, item)
		}
	}
	selected := candidates
	if len(exactFilename) > 0 {
		selected = exactFilename
	} else if len(candidates) != 1 {
		return nil, nil
	}
	best := selected[0]
	for _, item := range selected[1:] {
		if taskAssetRevisionCandidateNewer(item.version, best.version) {
			best = item
		}
	}
	assetID := best.asset.ID
	return &assetID, nil
}

func taskStatusCanReuseRejectedDelivery(status domain.TaskStatus) bool {
	switch status {
	case domain.TaskStatusRejectedByAuditA,
		domain.TaskStatusRejectedByAuditB,
		domain.TaskStatusPendingCustomizationProduction,
		domain.TaskStatusPendingEffectRevision,
		domain.TaskStatusRejectedByWarehouse:
		return true
	default:
		return false
	}
}

func taskAssetFilenameMatches(asset *domain.TaskAsset, filename string) bool {
	if asset == nil {
		return false
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(asset.FileName), filename) ||
		(asset.OriginalName != nil && strings.EqualFold(strings.TrimSpace(*asset.OriginalName), filename))
}

func taskAssetRevisionCandidateNewer(left, right *domain.TaskAsset) bool {
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	leftTime := left.CreatedAt
	if left.UploadedAt != nil {
		leftTime = *left.UploadedAt
	}
	rightTime := right.CreatedAt
	if right.UploadedAt != nil {
		rightTime = *right.UploadedAt
	}
	if !leftTime.Equal(rightTime) {
		return leftTime.After(rightTime)
	}
	return left.ID > right.ID
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

func (s *taskAssetCenterService) insertRetouchRequirementReferenceFlat(
	ctx context.Context,
	tx repo.Tx,
	taskID int64,
	retouchRequirementID *int64,
	refID string,
) error {
	if s.referenceFileRefFlatRepo == nil || retouchRequirementID == nil || *retouchRequirementID <= 0 {
		return nil
	}
	refID = strings.TrimSpace(refID)
	if refID == "" {
		return nil
	}
	if _, err := s.referenceFileRefFlatRepo.InsertFlat(ctx, tx, &domain.ReferenceFileRefFlat{
		TaskID:               taskID,
		RetouchRequirementID: retouchRequirementID,
		RefID:                refID,
		OwnerModuleKey:       string(domain.ModuleKeyBasicInfo),
		Context:              stringPtr("retouch_requirement_reference"),
	}); err != nil {
		return fmt.Errorf("insert retouch requirement reference_file_ref flat row: %w", err)
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
