package service

import (
	"context"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type CreateUploadRequestParams struct {
	OwnerType      domain.AssetOwnerType
	OwnerID        int64
	TaskAssetType  *domain.TaskAssetType
	StorageAdapter domain.AssetStorageAdapter
	RefType        domain.AssetStorageRefType
	FileName       string
	MimeType       string
	FileSize       *int64
	ChecksumHint   string
	Remark         string
}

type AdvanceUploadRequestParams struct {
	Action domain.UploadRequestAdvanceAction
	Remark string
}

type UploadRequestFilter struct {
	OwnerType     *domain.AssetOwnerType
	OwnerID       *int64
	TaskAssetType *domain.TaskAssetType
	Status        *domain.UploadRequestStatus
	Page          int
	PageSize      int
}

type AssetUploadService interface {
	CreateUploadRequest(ctx context.Context, params CreateUploadRequestParams) (*domain.UploadRequest, *domain.AppError)
	ListUploadRequests(ctx context.Context, filter UploadRequestFilter) ([]*domain.UploadRequest, domain.PaginationMeta, *domain.AppError)
	GetUploadRequest(ctx context.Context, requestID string) (*domain.UploadRequest, *domain.AppError)
	AdvanceUploadRequest(ctx context.Context, requestID string, params AdvanceUploadRequestParams) (*domain.UploadRequest, *domain.AppError)
}

type assetUploadService struct {
	taskRepo          repo.TaskRepo
	uploadRequestRepo repo.UploadRequestRepo
	txRunner          repo.TxRunner
	nowFn             func() time.Time
}

func NewAssetUploadService(taskRepo repo.TaskRepo, uploadRequestRepo repo.UploadRequestRepo, txRunner repo.TxRunner) AssetUploadService {
	return &assetUploadService{
		taskRepo:          taskRepo,
		uploadRequestRepo: uploadRequestRepo,
		txRunner:          txRunner,
		nowFn:             time.Now,
	}
}

func (s *assetUploadService) CreateUploadRequest(ctx context.Context, params CreateUploadRequestParams) (*domain.UploadRequest, *domain.AppError) {
	if params.StorageAdapter == "" {
		params.StorageAdapter = domain.AssetStorageAdapterPlaceholderStorage
	}
	if params.RefType == "" {
		params.RefType = domain.AssetStorageRefTypeGenericObject
	}
	if err := validateUploadRequestParams(params); err != nil {
		return nil, err
	}
	if err := s.validateUploadRequestOwner(ctx, params.OwnerType, params.OwnerID); err != nil {
		return nil, err
	}
	now := s.nowFn().UTC()
	request := &domain.UploadRequest{
		OwnerType:       params.OwnerType,
		OwnerID:         params.OwnerID,
		TaskID:          params.OwnerID,
		TaskAssetType:   normalizeOptionalTaskAssetType(params.TaskAssetType),
		StorageAdapter:  params.StorageAdapter,
		UploadMode:      domain.DesignAssetUploadModeSmall,
		RefType:         params.RefType,
		FileName:        strings.TrimSpace(params.FileName),
		MimeType:        strings.TrimSpace(params.MimeType),
		FileSize:        params.FileSize,
		ExpectedSize:    params.FileSize,
		ChecksumHint:    strings.TrimSpace(params.ChecksumHint),
		Status:          domain.UploadRequestStatusRequested,
		StorageProvider: domain.DesignAssetStorageProviderOSS,
		SessionStatus:   domain.DesignAssetSessionStatusCreated,
		IsPlaceholder:   true,
		Remark:          strings.TrimSpace(params.Remark),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		created, err := s.uploadRequestRepo.Create(ctx, tx, request)
		if err != nil {
			return err
		}
		request = created
		return nil
	}); err != nil {
		return nil, infraError("create upload request", err)
	}
	domain.HydrateUploadRequestDerived(request)
	return request, nil
}

func (s *assetUploadService) ListUploadRequests(ctx context.Context, filter UploadRequestFilter) ([]*domain.UploadRequest, domain.PaginationMeta, *domain.AppError) {
	if filter.OwnerType != nil && !filterValueUploadOwnerTypeValid(*filter.OwnerType) {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "owner_type is not supported", nil)
	}
	if filter.OwnerID != nil && *filter.OwnerID <= 0 {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "owner_id must be greater than zero", nil)
	}
	if filter.TaskAssetType != nil {
		if err := validateTaskAssetType(*filter.TaskAssetType); err != nil {
			return nil, domain.PaginationMeta{}, err
		}
	}
	if filter.Status != nil && !filter.Status.Valid() {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "status must be requested/bound/expired/cancelled", nil)
	}

	requests, total, err := s.uploadRequestRepo.List(ctx, repo.UploadRequestListFilter{
		OwnerType:     filter.OwnerType,
		OwnerID:       filter.OwnerID,
		TaskAssetType: filter.TaskAssetType,
		Status:        filter.Status,
		Page:          filter.Page,
		PageSize:      filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list upload requests", err)
	}
	if requests == nil {
		requests = []*domain.UploadRequest{}
	}
	for _, request := range requests {
		domain.HydrateUploadRequestDerived(request)
	}
	return requests, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *assetUploadService) GetUploadRequest(ctx context.Context, requestID string) (*domain.UploadRequest, *domain.AppError) {
	request, err := s.uploadRequestRepo.GetByRequestID(ctx, strings.TrimSpace(requestID))
	if err != nil {
		return nil, infraError("get upload request", err)
	}
	if request == nil {
		return nil, domain.ErrNotFound
	}
	domain.HydrateUploadRequestDerived(request)
	return request, nil
}

func (s *assetUploadService) AdvanceUploadRequest(ctx context.Context, requestID string, params AdvanceUploadRequestParams) (*domain.UploadRequest, *domain.AppError) {
	trimmedID := strings.TrimSpace(requestID)
	if trimmedID == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_request_id is required", nil)
	}
	if !params.Action.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "action must be cancel/expire", nil)
	}

	request, appErr := s.GetUploadRequest(ctx, trimmedID)
	if appErr != nil {
		return nil, appErr
	}
	if request.Status != domain.UploadRequestStatusRequested {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			"upload_request can only advance from requested",
			map[string]interface{}{
				"request_id": trimmedID,
				"status":     request.Status,
			},
		)
	}

	nextStatus := nextUploadRequestStatus(params.Action)
	nextRemark := strings.TrimSpace(params.Remark)
	if nextRemark == "" {
		nextRemark = strings.TrimSpace(request.Remark)
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.uploadRequestRepo.UpdateLifecycle(ctx, tx, repo.UploadRequestLifecycleUpdate{
			RequestID: trimmedID,
			Status:    nextStatus,
			Remark:    nextRemark,
		})
	}); err != nil {
		return nil, infraError("advance upload request", err)
	}
	return s.GetUploadRequest(ctx, trimmedID)
}

func (s *assetUploadService) validateUploadRequestOwner(ctx context.Context, ownerType domain.AssetOwnerType, ownerID int64) *domain.AppError {
	switch ownerType {
	case domain.AssetOwnerTypeTask:
		task, err := s.taskRepo.GetByID(ctx, ownerID)
		if err != nil {
			return infraError("get task for upload request", err)
		}
		if task == nil {
			return domain.ErrNotFound
		}
		return nil
	default:
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "owner_type is modeled generically, but only task is enabled in the current placeholder phase", nil)
	}
}

func validateUploadRequestParams(params CreateUploadRequestParams) *domain.AppError {
	if !params.OwnerType.Valid() {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "owner_type is not supported", nil)
	}
	if params.OwnerID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "owner_id must be greater than zero", nil)
	}
	if params.TaskAssetType != nil {
		normalized := domain.NormalizeTaskAssetType(*params.TaskAssetType)
		params.TaskAssetType = &normalized
		if err := validateTaskAssetType(*params.TaskAssetType); err != nil {
			return err
		}
	}
	if !params.StorageAdapter.Valid() {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "storage_adapter is not supported", nil)
	}
	if !params.RefType.Valid() {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "ref_type is not supported", nil)
	}
	if strings.TrimSpace(params.FileName) == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "file_name is required", nil)
	}
	if params.FileSize != nil && *params.FileSize < 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "file_size must be greater than or equal to zero", nil)
	}
	return nil
}

func filterValueUploadOwnerTypeValid(ownerType domain.AssetOwnerType) bool {
	return ownerType == "" || ownerType.Valid()
}

func nextUploadRequestStatus(action domain.UploadRequestAdvanceAction) domain.UploadRequestStatus {
	switch action {
	case domain.UploadRequestAdvanceActionCancel:
		return domain.UploadRequestStatusCancelled
	case domain.UploadRequestAdvanceActionExpire:
		return domain.UploadRequestStatusExpired
	default:
		return domain.UploadRequestStatusRequested
	}
}

func normalizeOptionalTaskAssetType(assetType *domain.TaskAssetType) *domain.TaskAssetType {
	if assetType == nil {
		return nil
	}
	normalized := domain.NormalizeTaskAssetType(*assetType)
	if normalized == "" {
		return nil
	}
	return &normalized
}
