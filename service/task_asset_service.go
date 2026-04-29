package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

type MockUploadTaskAssetParams struct {
	TaskID          int64
	UploadedBy      int64
	AssetType       domain.TaskAssetType
	UploadRequestID string
	FileName        string
	MimeType        string
	FileSize        *int64
	FilePath        *string
	WholeHash       *string
	Remark          string
}

type SubmitDesignParams struct {
	TaskID          int64
	UploadedBy      int64
	AssetType       domain.TaskAssetType
	UploadRequestID string
	FileName        string
	MimeType        string
	FileSize        *int64
	FilePath        *string
	WholeHash       *string
	Remark          string
}

type TaskAssetService interface {
	ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskAsset, *domain.AppError)
	MockUpload(ctx context.Context, p MockUploadTaskAssetParams) (*domain.TaskAsset, *domain.AppError)
	SubmitDesign(ctx context.Context, p SubmitDesignParams) (*domain.TaskAsset, *domain.AppError)
}

type taskAssetService struct {
	taskRepo                repo.TaskRepo
	taskAssetRepo           repo.TaskAssetRepo
	taskEventRepo           repo.TaskEventRepo
	taskModuleRepo          repo.TaskModuleRepo
	uploadRequestRepo       repo.UploadRequestRepo
	assetStorageRefRepo     repo.AssetStorageRefRepo
	txRunner                repo.TxRunner
	dataScopeResolver       DataScopeResolver
	scopeUserRepo           repo.UserRepo
	userDisplayNameResolver UserDisplayNameResolver
}

type TaskAssetServiceOption func(*taskAssetService)

func WithTaskAssetDataScopeResolver(resolver DataScopeResolver) TaskAssetServiceOption {
	return func(s *taskAssetService) {
		s.dataScopeResolver = resolver
	}
}

func WithTaskAssetScopeUserRepo(userRepo repo.UserRepo) TaskAssetServiceOption {
	return func(s *taskAssetService) {
		s.scopeUserRepo = userRepo
	}
}

func WithTaskAssetUserDisplayNameResolver(resolver UserDisplayNameResolver) TaskAssetServiceOption {
	return func(s *taskAssetService) {
		s.userDisplayNameResolver = resolver
	}
}

func WithTaskAssetModuleRepo(moduleRepo repo.TaskModuleRepo) TaskAssetServiceOption {
	return func(s *taskAssetService) {
		s.taskModuleRepo = moduleRepo
	}
}

func NewTaskAssetService(
	taskRepo repo.TaskRepo,
	taskAssetRepo repo.TaskAssetRepo,
	taskEventRepo repo.TaskEventRepo,
	uploadRequestRepo repo.UploadRequestRepo,
	assetStorageRefRepo repo.AssetStorageRefRepo,
	txRunner repo.TxRunner,
	opts ...TaskAssetServiceOption,
) TaskAssetService {
	svc := &taskAssetService{
		taskRepo:            taskRepo,
		taskAssetRepo:       taskAssetRepo,
		taskEventRepo:       taskEventRepo,
		uploadRequestRepo:   uploadRequestRepo,
		assetStorageRefRepo: assetStorageRefRepo,
		txRunner:            txRunner,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *taskAssetService) taskActionAuthorizer() *taskActionAuthorizer {
	return newTaskActionAuthorizer(s.dataScopeResolver, s.scopeUserRepo)
}

func (s *taskAssetService) ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskAsset, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task for asset list", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}

	assets, err := s.taskAssetRepo.ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list task assets", err)
	}
	if assets == nil {
		assets = []*domain.TaskAsset{}
	}
	for _, asset := range assets {
		if asset != nil && asset.StorageRef != nil {
			domain.HydrateAssetStorageRefDerived(asset.StorageRef)
		}
	}
	enrichTaskAssetUploaderNames(ctx, s.userDisplayNameResolver, assets)
	return assets, nil
}

func (s *taskAssetService) MockUpload(ctx context.Context, p MockUploadTaskAssetParams) (*domain.TaskAsset, *domain.AppError) {
	p.AssetType = domain.NormalizeTaskAssetType(p.AssetType)
	if err := validateTaskAssetType(p.AssetType); err != nil {
		return nil, err
	}
	task, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task for mock upload", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if task.TaskType == domain.TaskTypePurchaseTask {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "purchase_task does not support design asset upload", nil)
	}

	return s.createAsset(ctx, task, p.UploadedBy, p.AssetType, p.UploadRequestID, p.FileName, p.MimeType, p.FileSize, p.FilePath, p.WholeHash, p.Remark, false, task.TaskStatus)
}

func (s *taskAssetService) SubmitDesign(ctx context.Context, p SubmitDesignParams) (*domain.TaskAsset, *domain.AppError) {
	p.AssetType = domain.NormalizeTaskAssetType(p.AssetType)
	if err := validateSubmitDesignType(p.AssetType); err != nil {
		return nil, err
	}
	task, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task for submit design", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionSubmitDesign, task, "", "")
	authz.logDecision(TaskActionSubmitDesign, decision)
	if !decision.Allowed {
		return nil, taskActionDecisionAppError(TaskActionSubmitDesign, decision)
	}
	if task.TaskType == domain.TaskTypePurchaseTask {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "purchase_task does not support submit-design", nil)
	}
	if task.DesignerID == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task must have designer_id before submit-design", nil)
	}
	if !taskActionDecisionHasElevatedScope(decision) &&
		(task.CurrentHandlerID == nil || *task.CurrentHandlerID != p.UploadedBy) &&
		(task.DesignerID == nil || *task.DesignerID != p.UploadedBy) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "submit-design requires the assigned designer or current handler", map[string]interface{}{
			"task_id":            task.ID,
			"deny_code":          "task_not_assigned_to_actor",
			"deny_reason":        "task_not_assigned_to_actor",
			"current_handler_id": cloneInt64Ptr(task.CurrentHandlerID),
			"designer_id":        cloneInt64Ptr(task.DesignerID),
			"uploaded_by":        p.UploadedBy,
			"action":             string(TaskActionSubmitDesign),
			"owner_department":   task.OwnerDepartment,
			"owner_org_team":     task.OwnerOrgTeam,
			"scope_source":       decision.ScopeSource,
			"matched_rule":       decision.MatchedRule,
		})
	}
	if task.TaskStatus != domain.TaskStatusInProgress &&
		task.TaskStatus != domain.TaskStatusRejectedByAuditA &&
		task.TaskStatus != domain.TaskStatusRejectedByAuditB {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d is in status %q, must be InProgress, RejectedByAuditA, or RejectedByAuditB", p.TaskID, task.TaskStatus),
			nil,
		)
	}

	advanceStatus := p.AssetType.IsDelivery()
	return s.createAsset(ctx, task, p.UploadedBy, p.AssetType, p.UploadRequestID, p.FileName, p.MimeType, p.FileSize, p.FilePath, p.WholeHash, p.Remark, advanceStatus, task.TaskStatus)
}

func (s *taskAssetService) createAsset(
	ctx context.Context,
	task *domain.Task,
	uploadedBy int64,
	assetType domain.TaskAssetType,
	uploadRequestID string,
	fileName string,
	mimeType string,
	fileSize *int64,
	filePath *string,
	wholeHash *string,
	remark string,
	advanceStatus bool,
	fromStatus domain.TaskStatus,
) (*domain.TaskAsset, *domain.AppError) {
	taskID := task.ID
	if strings.TrimSpace(fileName) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_name is required", nil)
	}
	if fileSize != nil && *fileSize < 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_size must be greater than or equal to zero", nil)
	}

	uploadRequest, appErr := s.resolveUploadRequest(ctx, strings.TrimSpace(uploadRequestID), taskID, assetType)
	if appErr != nil {
		return nil, appErr
	}
	resolvedFileName := firstNonEmpty(strings.TrimSpace(fileName), uploadRequestFileName(uploadRequest))
	resolvedMimeType := firstNonEmpty(strings.TrimSpace(mimeType), uploadRequestMimeType(uploadRequest))
	resolvedFileSize := firstNonNilInt64(fileSize, uploadRequestFileSize(uploadRequest))
	resolvedChecksumHint := firstNonEmpty(optString(wholeHash), uploadRequestChecksumHint(uploadRequest))
	resolvedUploadRequestID := uploadRequestIDPtr(uploadRequest)
	resolvedUploadMode := resolveTaskAssetUploadMode(assetType, uploadRequest)
	storageAdapter := defaultTaskAssetStorageAdapter(advanceStatus, uploadRequest)
	refType := domain.AssetStorageRefTypeTaskAssetObject
	storageRefID := uuid.NewString()
	refKey := buildTaskAssetRefKey(taskID, storageRefID, resolvedFileName)
	uploadedAt := time.Now().UTC()
	uploadStatus := string(domain.DesignAssetUploadStatusUploaded)
	previewStatus := string(domain.DesignAssetPreviewStatusPending)

	var newID int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		versionNo, err := s.taskAssetRepo.NextVersionNo(ctx, tx, taskID)
		if err != nil {
			return err
		}

		asset := &domain.TaskAsset{
			TaskID:          taskID,
			AssetType:       domain.NormalizeTaskAssetType(assetType),
			VersionNo:       versionNo,
			UploadMode:      optionalStringPtr(string(resolvedUploadMode)),
			UploadRequestID: resolvedUploadRequestID,
			StorageRefID:    &storageRefID,
			FileName:        resolvedFileName,
			OriginalName:    optionalStringPtr(resolvedFileName),
			MimeType:        optionalStringPtr(resolvedMimeType),
			FileSize:        resolvedFileSize,
			FilePath:        filePath,
			StorageKey:      optionalStringPtr(refKey),
			WholeHash:       optionalStringPtr(resolvedChecksumHint),
			UploadStatus:    &uploadStatus,
			PreviewStatus:   &previewStatus,
			UploadedBy:      uploadedBy,
			UploadedAt:      &uploadedAt,
			Remark:          remark,
		}
		id, err := s.taskAssetRepo.Create(ctx, tx, asset)
		if err != nil {
			return err
		}
		newID = id
		storageRef := &domain.AssetStorageRef{
			RefID:           storageRefID,
			AssetID:         &newID,
			OwnerType:       domain.AssetOwnerTypeTaskAsset,
			OwnerID:         newID,
			StorageAdapter:  storageAdapter,
			RefType:         refType,
			RefKey:          refKey,
			FileName:        resolvedFileName,
			MimeType:        resolvedMimeType,
			FileSize:        resolvedFileSize,
			IsPlaceholder:   true,
			ChecksumHint:    resolvedChecksumHint,
			Status:          domain.AssetStorageRefStatusRecorded,
			UploadRequestID: strings.TrimSpace(uploadRequestID),
		}
		if _, err := s.assetStorageRefRepo.Create(ctx, tx, storageRef); err != nil {
			return err
		}
		if uploadRequest != nil {
			if err := s.uploadRequestRepo.UpdateBinding(ctx, tx, uploadRequest.RequestID, &newID, storageRefID, domain.UploadRequestStatusBound, uploadRequest.Remark); err != nil {
				return err
			}
		}

		eventType := domain.TaskEventAssetMockUploaded
		payload := map[string]interface{}{
			"asset_type":        string(assetType),
			"version_no":        versionNo,
			"upload_request_id": strings.TrimSpace(uploadRequestID),
			"storage_ref_id":    storageRefID,
			"storage_adapter":   string(storageAdapter),
			"ref_type":          string(refType),
			"ref_key":           refKey,
			"upload_mode":       string(resolvedUploadMode),
			"file_name":         resolvedFileName,
			"mime_type":         resolvedMimeType,
			"file_size":         resolvedFileSize,
			"file_path":         filePath,
			"whole_hash":        optionalStringPtr(resolvedChecksumHint),
			"remark":            remark,
		}

		if advanceStatus {
			if err := s.taskRepo.UpdateStatus(ctx, tx, taskID, domain.TaskStatusPendingAuditA); err != nil {
				return err
			}
			if err := s.taskRepo.UpdateHandler(ctx, tx, taskID, nil); err != nil {
				return err
			}
			if err := s.markDesignModuleSubmitted(ctx, tx, taskID); err != nil {
				return err
			}
			eventType = domain.TaskEventDesignSubmitted
			payload = taskTransitionEventPayload(task, fromStatus, domain.TaskStatusPendingAuditA, task.CurrentHandlerID, nil, payload)
			payload["designer_id"] = cloneInt64Ptr(task.DesignerID)
			payload["uploaded_by"] = uploadedBy
		}

		_, err = s.taskEventRepo.Append(ctx, tx, taskID, eventType, &uploadedBy, payload)
		return err
	})
	if txErr != nil {
		return nil, infraError("create task asset tx", txErr)
	}

	asset, err := s.taskAssetRepo.GetByID(ctx, newID)
	if err != nil || asset == nil {
		return nil, infraError("re-read task asset", err)
	}
	if asset.StorageRef == nil && asset.StorageRefID != nil && strings.TrimSpace(*asset.StorageRefID) != "" {
		ref, refErr := s.assetStorageRefRepo.GetByRefID(ctx, *asset.StorageRefID)
		if refErr != nil {
			return nil, infraError("re-read task asset storage ref", refErr)
		}
		asset.StorageRef = ref
	}
	if asset.StorageRef != nil {
		domain.HydrateAssetStorageRefDerived(asset.StorageRef)
	}
	enrichTaskAssetUploaderNames(ctx, s.userDisplayNameResolver, []*domain.TaskAsset{asset})
	return asset, nil
}

func (s *taskAssetService) markDesignModuleSubmitted(ctx context.Context, tx repo.Tx, taskID int64) error {
	if s.taskModuleRepo == nil {
		return nil
	}
	return s.taskModuleRepo.UpdateState(ctx, tx, taskID, domain.ModuleKeyDesign, domain.ModuleStateSubmitted, false, nil)
}

func (s *taskAssetService) resolveUploadRequest(ctx context.Context, requestID string, taskID int64, assetType domain.TaskAssetType) (*domain.UploadRequest, *domain.AppError) {
	if requestID == "" {
		return nil, nil
	}
	request, err := s.uploadRequestRepo.GetByRequestID(ctx, requestID)
	if err != nil {
		return nil, infraError("get upload request for task asset", err)
	}
	if request == nil {
		return nil, domain.ErrNotFound
	}
	if request.OwnerType != domain.AssetOwnerTypeTask || request.OwnerID != taskID {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_request owner does not match current task", nil)
	}
	if request.Status != domain.UploadRequestStatusRequested {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, fmt.Sprintf("upload_request %s is in status %s", request.RequestID, request.Status), nil)
	}
	if request.TaskAssetType != nil && domain.NormalizeTaskAssetType(*request.TaskAssetType) != domain.NormalizeTaskAssetType(assetType) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_request task_asset_type does not match asset_type", nil)
	}
	return request, nil
}

func uploadRequestIDPtr(request *domain.UploadRequest) *string {
	if request == nil || strings.TrimSpace(request.RequestID) == "" {
		return nil
	}
	id := strings.TrimSpace(request.RequestID)
	return &id
}

func uploadRequestFileName(request *domain.UploadRequest) string {
	if request == nil {
		return ""
	}
	return strings.TrimSpace(request.FileName)
}

func uploadRequestMimeType(request *domain.UploadRequest) string {
	if request == nil {
		return ""
	}
	return strings.TrimSpace(request.MimeType)
}

func uploadRequestFileSize(request *domain.UploadRequest) *int64 {
	if request == nil {
		return nil
	}
	return request.FileSize
}

func uploadRequestChecksumHint(request *domain.UploadRequest) string {
	if request == nil {
		return ""
	}
	return strings.TrimSpace(request.ChecksumHint)
}

func defaultTaskAssetStorageAdapter(advanceStatus bool, request *domain.UploadRequest) domain.AssetStorageAdapter {
	if request != nil && request.StorageAdapter.Valid() {
		return request.StorageAdapter
	}
	if advanceStatus {
		return domain.AssetStorageAdapterPlaceholderStorage
	}
	return domain.AssetStorageAdapterMockUpload
}

func buildTaskAssetRefKey(taskID int64, storageRefID string, fileName string) string {
	cleanFileName := strings.ReplaceAll(strings.TrimSpace(fileName), " ", "_")
	if cleanFileName == "" {
		cleanFileName = "unnamed"
	}
	return fmt.Sprintf("task-assets/%d/%s/%s", taskID, storageRefID, cleanFileName)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonNilInt64(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			v := *value
			return &v
		}
	}
	return nil
}

func optionalStringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}

func optString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func validateTaskAssetType(assetType domain.TaskAssetType) *domain.AppError {
	switch domain.NormalizeTaskAssetType(assetType) {
	case domain.TaskAssetTypeReference,
		domain.TaskAssetTypeSource,
		domain.TaskAssetTypeDelivery,
		domain.TaskAssetTypePreview:
		return nil
	default:
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset_type", nil)
	}
}

func validateSubmitDesignType(assetType domain.TaskAssetType) *domain.AppError {
	switch domain.NormalizeTaskAssetType(assetType) {
	case domain.TaskAssetTypeSource,
		domain.TaskAssetTypeDelivery:
		return nil
	default:
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "submit-design only allows source or delivery", nil)
	}
}

func resolveTaskAssetUploadMode(assetType domain.TaskAssetType, request *domain.UploadRequest) domain.DesignAssetUploadMode {
	if request != nil && request.UploadMode.Valid() {
		return request.UploadMode
	}
	if domain.NormalizeTaskAssetType(assetType).IsReference() {
		return domain.DesignAssetUploadModeSmall
	}
	return domain.DesignAssetUploadModeMultipart
}
