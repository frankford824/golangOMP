package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"

	"workflow/domain"
	"workflow/repo"
)

type ProductManagementService interface {
	List(ctx context.Context, filter repo.ProductManagementListFilter) ([]*domain.ProductManagementRecord, domain.PaginationMeta, *domain.AppError)
	ListComboTree(ctx context.Context, filter repo.ProductManagementListFilter) (*domain.ProductManagementComboTreeResponse, *domain.AppError)
	GetByTaskID(ctx context.Context, taskID int64) ([]*domain.ProductManagementRecord, *domain.AppError)
	ListImageCandidates(ctx context.Context, actor domain.RequestActor, recordID int64) ([]*domain.ProductManagementImageCandidate, *domain.AppError)
	ReparseImage(ctx context.Context, actor domain.RequestActor, recordID int64) (*domain.ProductManagementRecord, *domain.AppError)
	SetManualImage(ctx context.Context, actor domain.RequestActor, recordID int64, assetID int64) (*domain.ProductManagementRecord, *domain.AppError)
	RequestSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError)
	RequestBaseSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError)
	RequestImageSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError)
	AutoSyncImagesAfterTaskClosed(ctx context.Context, taskID int64, actorID int64) *domain.AppError
	RefreshReadModelNow(ctx context.Context) *domain.AppError
	ProcessQueuedERPSync(ctx context.Context, limit int) (int, *domain.AppError)
}

type ProductManagementServiceOption func(*productManagementService)

type productManagementService struct {
	records      repo.ProductManagementRepo
	taskAssets   repo.TaskAssetRepo
	assetSearch  repo.TaskAssetSearchRepo
	taskEvents   repo.TaskEventRepo
	skuCombos    repo.SKUComboRepo
	txRunner     repo.TxRunner
	erpBridge    ERPBridgeService
	ossDirect    *OSSDirectService
	uploadClient UploadServiceClient
	imageProxy   *ERPImageProxySigner
	now          func() time.Time
	refreshEvery time.Duration
	refreshMu    sync.Mutex
	lastRefresh  time.Time
}

func NewProductManagementService(records repo.ProductManagementRepo, taskAssets repo.TaskAssetRepo, assetSearch repo.TaskAssetSearchRepo, txRunner repo.TxRunner, opts ...ProductManagementServiceOption) ProductManagementService {
	s := &productManagementService{
		records:      records,
		taskAssets:   taskAssets,
		assetSearch:  assetSearch,
		txRunner:     txRunner,
		now:          time.Now,
		refreshEvery: 30 * time.Second,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

func WithProductManagementERPBridge(erpBridge ERPBridgeService) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.erpBridge = erpBridge
	}
}

func WithProductManagementAssetURLServices(ossDirect *OSSDirectService, uploadClient UploadServiceClient) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.ossDirect = ossDirect
		s.uploadClient = uploadClient
	}
}

func WithProductManagementERPImageProxy(imageProxy *ERPImageProxySigner) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.imageProxy = imageProxy
	}
}

func WithProductManagementTaskEventRepo(taskEvents repo.TaskEventRepo) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.taskEvents = taskEvents
	}
}

func WithProductManagementSKUComboRepo(skuCombos repo.SKUComboRepo) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.skuCombos = skuCombos
	}
}

func (s *productManagementService) List(ctx context.Context, filter repo.ProductManagementListFilter) ([]*domain.ProductManagementRecord, domain.PaginationMeta, *domain.AppError) {
	if appErr := s.refreshReadModel(ctx); appErr != nil {
		return nil, domain.PaginationMeta{}, appErr
	}
	items, total, err := s.records.List(ctx, filter)
	if err != nil {
		return nil, domain.PaginationMeta{}, infraAppError("list product management records", err)
	}
	actor, _ := domain.RequestActorFromContext(ctx)
	s.decorateRecords(ctx, actor, items)
	page, pageSize := normalizeProductManagementPage(filter.Page, filter.PageSize)
	return items, domain.PaginationMeta{Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *productManagementService) ListComboTree(ctx context.Context, filter repo.ProductManagementListFilter) (*domain.ProductManagementComboTreeResponse, *domain.AppError) {
	items, meta, appErr := s.List(ctx, filter)
	if appErr != nil {
		return nil, appErr
	}
	groups := s.productManagementComboGroups(ctx, items)
	var summary *domain.OMPSKUComboSyncState
	if s.skuCombos != nil {
		state, err := s.skuCombos.GetLatestSyncState(ctx)
		if err != nil {
			if isMySQLTableMissing(err) {
				return &domain.ProductManagementComboTreeResponse{
					Groups:     productManagementSingleGroups(items),
					Data:       items,
					Pagination: meta,
				}, nil
			}
			return nil, infraAppError("get product management combo sync state", err)
		}
		summary = state
	}
	return &domain.ProductManagementComboTreeResponse{
		Groups:      groups,
		Data:        items,
		Pagination:  meta,
		SyncSummary: summary,
	}, nil
}

func (s *productManagementService) GetByTaskID(ctx context.Context, taskID int64) ([]*domain.ProductManagementRecord, *domain.AppError) {
	if taskID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil)
	}
	if appErr := s.refreshReadModel(ctx); appErr != nil {
		return nil, appErr
	}
	items, err := s.records.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraAppError("list task product management records", err)
	}
	actor, _ := domain.RequestActorFromContext(ctx)
	s.decorateRecords(ctx, actor, items)
	return items, nil
}

func (s *productManagementService) ListImageCandidates(ctx context.Context, actor domain.RequestActor, recordID int64) ([]*domain.ProductManagementImageCandidate, *domain.AppError) {
	record, appErr := s.getRecord(ctx, recordID)
	if appErr != nil {
		return nil, appErr
	}
	if !canMaintainProductManagementImage(actor, record) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "only task creator or ERP/Admin can maintain ERP product image", nil)
	}
	candidates, err := s.imageCandidatesForRecord(ctx, record)
	if err != nil {
		return nil, infraAppError("list product image candidates", err)
	}
	return candidates, nil
}

func (s *productManagementService) ReparseImage(ctx context.Context, actor domain.RequestActor, recordID int64) (*domain.ProductManagementRecord, *domain.AppError) {
	record, appErr := s.getRecord(ctx, recordID)
	if appErr != nil {
		return nil, appErr
	}
	if !canMaintainProductManagementImage(actor, record) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "only task creator or ERP/Admin can reparse ERP product image", nil)
	}
	patch := s.autoImagePatch(ctx, record)
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateImage(ctx, tx, record.ID, patch)
	}); err != nil {
		return nil, infraAppError("reparse product management image", err)
	}
	return s.getAndDecorateRecord(ctx, actor, record.ID)
}

func (s *productManagementService) SetManualImage(ctx context.Context, actor domain.RequestActor, recordID int64, assetID int64) (*domain.ProductManagementRecord, *domain.AppError) {
	if assetID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_id is required", nil)
	}
	record, appErr := s.getRecord(ctx, recordID)
	if appErr != nil {
		return nil, appErr
	}
	if !canMaintainProductManagementImage(actor, record) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "only task creator or ERP/Admin can maintain ERP product image", nil)
	}
	row, err := s.assetSearch.GetCurrentByAssetID(ctx, assetID)
	if err != nil {
		return nil, infraAppError("get selected product image asset", err)
	}
	if row == nil || row.Asset == nil {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "asset not found", nil)
	}
	if !isProductManagementERPImageAsset(row.Asset) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "ERP product image must be jpg, png, webp, or gif", nil)
	}
	if row.Asset.TaskID != record.TaskID && !isProductManagementAdmin(actor) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "cross-task image selection requires ERP/Admin", nil)
	}
	taskNo := ""
	if row.Task != nil {
		taskNo = row.Task.TaskNo
	}
	source := domain.ProductManagementImageSourceManual
	if domain.NormalizeTaskAssetType(row.Asset.AssetType).IsERPProductImage() {
		source = domain.ProductManagementImageSourceERPProduct
	}
	patch := imagePatchFromCandidate(candidateFromAsset(row.Asset, taskNo, source))
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateImage(ctx, tx, record.ID, patch)
	}); err != nil {
		return nil, infraAppError("set product management image", err)
	}
	return s.getAndDecorateRecord(ctx, actor, record.ID)
}

func (s *productManagementService) ProcessQueuedERPSync(ctx context.Context, limit int) (int, *domain.AppError) {
	if s.erpBridge == nil {
		return 0, domain.NewAppError(domain.ErrCodeInternalError, "ERP bridge is not configured for product management sync", nil)
	}
	now := s.now()
	claimToken := fmt.Sprintf("product-management-%d", now.UnixNano())
	records, err := s.records.ClaimQueuedSyncRecords(ctx, limit, claimToken, now)
	if err != nil {
		return 0, infraAppError("claim product management erp sync records", err)
	}
	processed := 0
	for _, record := range records {
		if record == nil {
			continue
		}
		if ctx.Err() != nil {
			return processed, domain.NewAppError(domain.ErrCodeInternalError, "product management sync worker stopped", map[string]string{"cause": ctx.Err().Error()})
		}
		didWork := false
		if productManagementStatusInFlight(record.BaseSyncStatus) {
			didWork = true
			if appErr := s.syncBaseRecordToERP(ctx, record); appErr != nil {
				s.markProductManagementBaseSyncFailed(ctx, record, appErr.Message)
			} else {
				s.markProductManagementBaseSyncSucceeded(ctx, record)
			}
		}
		if productManagementStatusInFlight(record.ImageSyncStatus) {
			didWork = true
			if appErr := s.syncImageRecordToERP(ctx, record); appErr != nil {
				if isProductManagementERPImageSyncRetryable(appErr) {
					s.markProductManagementImageSyncRetrying(ctx, record, appErr.Message)
				} else {
					s.markProductManagementImageSyncFailed(ctx, record, appErr.Message)
				}
			} else {
				s.markProductManagementImageSyncSucceeded(ctx, record)
			}
		}
		if !didWork {
			if appErr := s.syncRecordToERP(ctx, record); appErr != nil {
				s.markProductManagementSyncFailed(ctx, record, appErr.Message)
			} else {
				s.markProductManagementSyncSucceeded(ctx, record)
			}
		}
		processed++
	}
	return processed, nil
}

func (s *productManagementService) RequestSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError) {
	return s.queueProductManagementSync(ctx, actor, recordID, force, "all")
}

func (s *productManagementService) RequestBaseSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError) {
	return s.queueProductManagementSync(ctx, actor, recordID, force, "base")
}

func (s *productManagementService) RequestImageSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError) {
	return s.queueProductManagementSync(ctx, actor, recordID, force, "image")
}

func (s *productManagementService) AutoSyncImagesAfterTaskClosed(ctx context.Context, taskID int64, actorID int64) *domain.AppError {
	if taskID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil)
	}
	if err := s.records.RefreshReadModel(ctx); err != nil {
		return infraAppError("refresh product management read model for closed task", err)
	}
	records, err := s.records.GetByTaskID(ctx, taskID)
	if err != nil {
		return infraAppError("list closed task product management records", err)
	}
	if len(records) == 0 {
		return nil
	}
	assetsByTaskID := make(map[int64][]*domain.TaskAsset)
	for _, record := range records {
		if record == nil || strings.TrimSpace(record.SKUCode) == "" {
			continue
		}
		patch := s.autoImagePatchWithCache(ctx, record, assetsByTaskID)
		if patch.ImageSource == domain.ProductManagementImageSourceDelivery {
			patch.ImageSource = domain.ProductManagementImageSourceAutoOnClose
			patch.ImageSyncSource = domain.ProductManagementImageSourceAutoOnClose
		}
		if patch.ImageSource == domain.ProductManagementImageSourceMissing {
			patch.ImageSyncStatus = domain.ProductManagementERPSyncStatusWaitingImage
			if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
				if err := s.records.UpdateImage(ctx, tx, record.ID, patch); err != nil {
					return err
				}
				return s.appendProductManagementTaskEvent(ctx, tx, record.TaskID, domain.TaskEventERPImageAwaitingUpload, actorID,
					fmt.Sprintf("未找到最终成品图，SKU %s 待人工上传 ERP 商品图", strings.TrimSpace(record.SKUCode)),
					record, patch.ImageMissingReason)
			}); err != nil {
				return infraAppError("mark product management image awaiting upload", err)
			}
			continue
		}
		patch.ImageSyncStatus = domain.ProductManagementERPSyncStatusSyncing
		if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			return s.records.UpdateImage(ctx, tx, record.ID, patch)
		}); err != nil {
			return infraAppError("prepare closed task erp image sync", err)
		}
		record.ImageSource = patch.ImageSource
		record.ImageSelectionMode = patch.ImageSelectionMode
		record.ImageAssetID = patch.ImageAssetID
		record.ImageAssetVersionID = patch.ImageAssetVersionID
		record.ImageFilename = patch.ImageFilename
		record.ImageMimeType = patch.ImageMimeType
		record.ImageMissingReason = patch.ImageMissingReason
		record.ImageSyncSource = patch.ImageSyncSource
		record.ImageSyncStatus = patch.ImageSyncStatus
		if s.erpBridge == nil {
			msg := "ERP 图片同步服务未配置，可稍后手动重试"
			s.markProductManagementImageSyncFailed(ctx, record, msg)
			_ = s.appendProductManagementTaskEventInTx(ctx, record, domain.TaskEventERPImageAutoSyncFailed, actorID, fmt.Sprintf("ERP 图片自动同步失败：SKU %s，%s", strings.TrimSpace(record.SKUCode), msg), msg)
			continue
		}
		if appErr := s.syncImageRecordToERP(ctx, record); appErr != nil {
			msg := truncateProductManagementSyncError(appErr.Message)
			if isProductManagementERPImageSyncRetryable(appErr) {
				s.markProductManagementImageSyncRetrying(ctx, record, msg)
				continue
			}
			s.markProductManagementImageSyncFailed(ctx, record, msg)
			_ = s.appendProductManagementTaskEventInTx(ctx, record, domain.TaskEventERPImageAutoSyncFailed, actorID, fmt.Sprintf("ERP 图片自动同步失败：SKU %s，%s", strings.TrimSpace(record.SKUCode), msg), msg)
			continue
		}
		s.markProductManagementImageSyncSucceeded(ctx, record)
		_ = s.appendProductManagementTaskEventInTx(ctx, record, domain.TaskEventERPImageAutoSynced, actorID, fmt.Sprintf("ERP 图片已自动同步：SKU %s", strings.TrimSpace(record.SKUCode)), domain.ProductManagementImageSourceLabel(record.ImageSource))
	}
	return nil
}

func (s *productManagementService) queueProductManagementSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool, scope string) (*domain.ProductManagementRecord, *domain.AppError) {
	record, appErr := s.getRecord(ctx, recordID)
	if appErr != nil {
		return nil, appErr
	}
	if !canSyncProductManagementERP(actor, record) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "ERP sync requires ERP/Admin or task creator", nil)
	}
	now := s.now()
	status := domain.ProductManagementERPSyncStatusQueued
	errMsg := ""
	cooldown := now.Add(5 * time.Minute)
	if record.SyncCooldownUntil != nil && record.SyncCooldownUntil.After(now) && !force {
		status = domain.ProductManagementERPSyncStatusCoolingDown
		errMsg = fmt.Sprintf("单 SKU 刷新冷却中，%s 后可再次同步", record.SyncCooldownUntil.Format("2006-01-02 15:04:05"))
		cooldown = *record.SyncCooldownUntil
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		patch := repo.ProductManagementSyncPatch{
			Status:            status,
			LastERPCheckedAt:  &now,
			SyncCooldownUntil: &cooldown,
			LastSyncError:     errMsg,
		}
		switch scope {
		case "base":
			patch.BaseStatus = status
			patch.BaseSyncError = errMsg
			return s.records.UpdateBaseSyncStatus(ctx, tx, record.ID, patch)
		case "image":
			patch.ImageStatus = status
			patch.ImageSyncError = errMsg
			return s.records.UpdateImageSyncStatus(ctx, tx, record.ID, patch)
		default:
			patch.BaseStatus = status
			patch.ImageStatus = status
			patch.BaseSyncError = errMsg
			patch.ImageSyncError = errMsg
			return s.records.UpdateSyncStatus(ctx, tx, record.ID, patch)
		}
	}); err != nil {
		return nil, infraAppError("request product management erp sync", err)
	}
	return s.getAndDecorateRecord(ctx, actor, record.ID)
}

func (s *productManagementService) refreshReadModel(ctx context.Context) *domain.AppError {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	now := s.now()
	if !s.lastRefresh.IsZero() && now.Sub(s.lastRefresh) < s.refreshEvery {
		return nil
	}
	if err := s.records.RefreshReadModel(ctx); err != nil {
		return infraAppError("refresh product management read model", err)
	}
	s.lastRefresh = now
	return nil
}

func (s *productManagementService) RefreshReadModelNow(ctx context.Context) *domain.AppError {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if err := s.records.RefreshReadModel(ctx); err != nil {
		return infraAppError("refresh product management read model", err)
	}
	s.lastRefresh = s.now()
	return nil
}

func (s *productManagementService) syncRecordToERP(ctx context.Context, record *domain.ProductManagementRecord) *domain.AppError {
	if appErr := s.syncBaseRecordToERP(ctx, record); appErr != nil {
		return appErr
	}
	if record != nil && record.ImageAssetID != nil && *record.ImageAssetID > 0 {
		return s.syncImageRecordToERP(ctx, record)
	}
	return nil
}

func (s *productManagementService) syncBaseRecordToERP(ctx context.Context, record *domain.ProductManagementRecord) *domain.AppError {
	if record == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "product management record is required", nil)
	}
	if strings.TrimSpace(record.SKUCode) == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "SKU is required for ERP product sync", nil)
	}
	productName := firstNonEmptyString(strings.TrimSpace(record.ProductName), strings.TrimSpace(record.SKUCode))
	productIID := firstNonEmptyString(strings.TrimSpace(record.ERPIID), strings.TrimSpace(record.ProductIID))
	shortName := productManagementERPShortName(productName, productIID, record.SKUCode)
	payload := domain.ERPProductUpsertPayload{
		ProductID:        strings.TrimSpace(record.SKUCode),
		SKUID:            strings.TrimSpace(record.SKUCode),
		SKUCode:          strings.TrimSpace(record.SKUCode),
		IID:              productIID,
		Name:             productName,
		ProductName:      productName,
		ShortName:        shortName,
		ProductShortName: shortName,
		Operation:        "product_management_base_sync",
		Source:           "product_management",
		TaskContext: &domain.ERPTaskFilingContext{
			TaskNo: strings.TrimSpace(record.TaskNo),
			Remark: "产品管理同步 ERP 基础资料",
		},
	}
	if record.CostPrice != nil && *record.CostPrice > 0 {
		cost := *record.CostPrice
		payload.CostPrice = &cost
	}
	_, appErr := s.erpBridge.UpsertProduct(ctx, payload)
	return appErr
}

func (s *productManagementService) syncImageRecordToERP(ctx context.Context, record *domain.ProductManagementRecord) *domain.AppError {
	if record == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "product management record is required", nil)
	}
	skuCode := strings.TrimSpace(record.SKUCode)
	if skuCode == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "SKU is required for ERP image sync", nil)
	}
	imageURL, appErr := s.resolveERPImageURL(ctx, record)
	if appErr != nil {
		return appErr
	}
	productIID, lookupErr := s.resolveProductManagementStyleIID(ctx, record)
	if lookupErr != nil {
		return lookupErr
	}
	payload := domain.ERPItemStyleUpdatePayload{
		SKUID:     skuCode,
		IID:       productIID,
		Pic:       imageURL,
		PicBig:    imageURL,
		SKUPic:    imageURL,
		Operation: "product_management_image_sync",
		Source:    "product_management",
		TaskContext: &domain.ERPTaskFilingContext{
			TaskNo: strings.TrimSpace(record.TaskNo),
			Remark: fmt.Sprintf("产品管理同步 ERP 图片，图片来源：%s", domain.ProductManagementImageSourceLabel(record.ImageSource)),
		},
	}
	_, appErr = s.erpBridge.UpdateItemStyle(ctx, payload)
	if appErr != nil {
		return appErr
	}
	return s.verifyERPImageReadback(ctx, record)
}

func (s *productManagementService) resolveProductManagementStyleIID(ctx context.Context, record *domain.ProductManagementRecord) (string, *domain.AppError) {
	if s == nil || s.erpBridge == nil {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片同步服务未配置，无法确认 SKU 是否已建档", nil)
	}
	skuCode := strings.TrimSpace(record.SKUCode)
	product, appErr := s.erpBridge.GetProductByID(ctx, skuCode)
	if appErr != nil {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片同步前未找到该 SKU："+appErr.Message, nil)
	}
	productIID := firstNonEmptyString(strings.TrimSpace(record.ERPIID), strings.TrimSpace(record.ProductIID))
	if product != nil {
		productIID = firstNonEmptyString(productIID, strings.TrimSpace(product.IID))
	}
	if productIID == "" {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片同步缺少款式编码，请先确认该 SKU 已在聚水潭建档", nil)
	}
	return productIID, nil
}

var productManagementERPImageReadbackRetryDelays = []time.Duration{
	300 * time.Millisecond,
	800 * time.Millisecond,
	1500 * time.Millisecond,
}

var productManagementERPImageReadbackSleep = time.Sleep

func (s *productManagementService) verifyERPImageReadback(ctx context.Context, record *domain.ProductManagementRecord) *domain.AppError {
	if s == nil || s.erpBridge == nil || record == nil {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片回读校验失败：同步服务未配置", nil)
	}
	sku := strings.TrimSpace(record.SKUCode)
	if sku == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "SKU is required for ERP image readback", nil)
	}
	maxAttempts := 1 + len(productManagementERPImageReadbackRetryDelays)
	var lastMessage string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		product, appErr := s.erpBridge.GetProductByID(ctx, sku)
		if appErr != nil {
			lastMessage = strings.TrimSpace(appErr.Message)
		} else if product == nil {
			lastMessage = "ERP 未返回该 SKU 商品资料"
		} else {
			imageURL := strings.TrimSpace(product.ImageURL)
			if isAbsoluteHTTPURL(imageURL) {
				return nil
			}
			if imageURL == "" {
				lastMessage = "ERP 尚未返回商品图"
			} else {
				lastMessage = "ERP 返回的图片地址不是公网地址"
			}
		}
		if attempt < maxAttempts {
			productManagementERPImageReadbackSleep(productManagementERPImageReadbackRetryDelays[attempt-1])
		}
	}
	if lastMessage == "" {
		lastMessage = "ERP 图片状态未知"
	}
	return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片回读校验未通过："+lastMessage, nil)
}

func isProductManagementERPImageReadbackPending(appErr *domain.AppError) bool {
	if appErr == nil {
		return false
	}
	msg := strings.TrimSpace(appErr.Message)
	if !strings.HasPrefix(msg, "ERP 图片回读校验未通过：") {
		return false
	}
	return strings.Contains(msg, "ERP 尚未返回商品图") || strings.Contains(msg, "ERP 返回的图片地址不是公网地址")
}

func isProductManagementERPImageSyncRetryable(appErr *domain.AppError) bool {
	if appErr == nil {
		return false
	}
	if isProductManagementERPImageReadbackPending(appErr) {
		return true
	}
	return strings.Contains(strings.TrimSpace(appErr.Message), "upstream business rejected request")
}

func productManagementERPShortName(productName, productIID, skuCode string) string {
	return strings.TrimSpace(firstNonEmptyString(productName, skuCode, productIID))
}

func productManagementStatusInFlight(status domain.ProductManagementERPSyncStatus) bool {
	switch status {
	case domain.ProductManagementERPSyncStatusQueued,
		domain.ProductManagementERPSyncStatusSyncing,
		domain.ProductManagementERPSyncStatusCoolingDown:
		return true
	default:
		return false
	}
}

func (s *productManagementService) resolveERPImageURL(ctx context.Context, record *domain.ProductManagementRecord) (string, *domain.AppError) {
	if record == nil || record.ImageAssetID == nil || *record.ImageAssetID <= 0 {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片待补充，无法同步 ERP", nil)
	}
	var row *repo.TaskAssetSearchRow
	var err error
	if record.ImageAssetVersionID != nil && *record.ImageAssetVersionID > 0 {
		row, err = s.assetSearch.GetVersion(ctx, *record.ImageAssetID, *record.ImageAssetVersionID)
	} else {
		row, err = s.assetSearch.GetCurrentByAssetID(ctx, *record.ImageAssetID)
	}
	if err != nil {
		return "", infraAppError("get product management erp image asset", err)
	}
	if row == nil || row.Asset == nil {
		return "", domain.NewAppError(domain.ErrCodeNotFound, "ERP 图片资产不存在", nil)
	}
	asset := row.Asset
	if !isProductManagementERPImageAsset(asset) {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片只支持 jpg、png、webp、gif，请先选择可预览图片资产", nil)
	}
	storageKey := ""
	if asset.StorageKey != nil {
		storageKey = strings.TrimSpace(*asset.StorageKey)
	}
	if storageKey == "" {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片资产缺少 OSS 存储地址", nil)
	}
	if isAbsoluteHTTPURL(storageKey) {
		return storageKey, nil
	}
	if s.imageProxy != nil {
		if proxyURL := s.imageProxy.BuildImageURL(asset); proxyURL != nil && isAbsoluteHTTPURL(*proxyURL) {
			return strings.TrimSpace(*proxyURL), nil
		}
	}
	filename := strings.TrimSpace(asset.FileName)
	if asset.OriginalName != nil && strings.TrimSpace(*asset.OriginalName) != "" {
		filename = strings.TrimSpace(*asset.OriginalName)
	}
	ossSignedURLTooLong := false
	if s.ossDirect != nil && s.ossDirect.Enabled() {
		if info := s.ossDirect.PresignDownloadURLWithFilename(storageKey, filename); info != nil && isAbsoluteHTTPURL(info.DownloadURL) {
			signedURL := strings.TrimSpace(info.DownloadURL)
			if len(signedURL) <= 300 {
				return signedURL, nil
			}
			ossSignedURLTooLong = true
		}
	}
	if s.uploadClient != nil {
		if directURL := s.uploadClient.BuildBrowserFileURL(storageKey); directURL != nil && isAbsoluteHTTPURL(*directURL) {
			return strings.TrimSpace(*directURL), nil
		}
	}
	if ossSignedURLTooLong {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片地址超过聚水潭 300 字符限制，请启用 ERP 图片短链代理", nil)
	}
	return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "缺少可供 ERP 拉取的公网图片地址配置", nil)
}

func (s *productManagementService) markProductManagementSyncSucceeded(ctx context.Context, record *domain.ProductManagementRecord) {
	if record == nil {
		return
	}
	now := s.now()
	cooldown := record.SyncCooldownUntil
	if cooldown == nil || cooldown.Before(now) {
		next := now.Add(5 * time.Minute)
		cooldown = &next
	}
	imageStatus := domain.ProductManagementERPSyncStatusSynced
	imageErr := ""
	imageSyncedAt := &now
	if record.ImageAssetID == nil || *record.ImageAssetID <= 0 {
		imageStatus = domain.ProductManagementERPSyncStatusWaitingImage
		imageErr = "ERP 商品图待上传"
		imageSyncedAt = nil
	}
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusSynced,
			BaseStatus:        domain.ProductManagementERPSyncStatusSynced,
			ImageStatus:       imageStatus,
			LastERPCheckedAt:  &now,
			LastERPSyncedAt:   &now,
			LastBaseSyncedAt:  &now,
			LastImageSyncedAt: imageSyncedAt,
			SyncCooldownUntil: cooldown,
			LastSyncError:     "",
			BaseSyncError:     "",
			ImageSyncError:    imageErr,
		})
	})
}

func (s *productManagementService) markProductManagementSyncFailed(ctx context.Context, record *domain.ProductManagementRecord, message string) {
	if record == nil {
		return
	}
	now := s.now()
	cooldown := record.SyncCooldownUntil
	if cooldown == nil || cooldown.Before(now) {
		next := now.Add(5 * time.Minute)
		cooldown = &next
	}
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusFailed,
			BaseStatus:        domain.ProductManagementERPSyncStatusFailed,
			ImageStatus:       domain.ProductManagementERPSyncStatusFailed,
			LastERPCheckedAt:  &now,
			SyncCooldownUntil: cooldown,
			LastSyncError:     truncateProductManagementSyncError(message),
			BaseSyncError:     truncateProductManagementSyncError(message),
			ImageSyncError:    truncateProductManagementSyncError(message),
		})
	})
}

func (s *productManagementService) markProductManagementBaseSyncSucceeded(ctx context.Context, record *domain.ProductManagementRecord) {
	if record == nil {
		return
	}
	now := s.now()
	cooldown := record.SyncCooldownUntil
	if cooldown == nil || cooldown.Before(now) {
		next := now.Add(5 * time.Minute)
		cooldown = &next
	}
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateBaseSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusSynced,
			BaseStatus:        domain.ProductManagementERPSyncStatusSynced,
			LastERPCheckedAt:  &now,
			LastERPSyncedAt:   &now,
			LastBaseSyncedAt:  &now,
			SyncCooldownUntil: cooldown,
			LastSyncError:     "",
			BaseSyncError:     "",
		})
	})
}

func (s *productManagementService) markProductManagementBaseSyncFailed(ctx context.Context, record *domain.ProductManagementRecord, message string) {
	if record == nil {
		return
	}
	now := s.now()
	cooldown := record.SyncCooldownUntil
	if cooldown == nil || cooldown.Before(now) {
		next := now.Add(5 * time.Minute)
		cooldown = &next
	}
	msg := truncateProductManagementSyncError(message)
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateBaseSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusFailed,
			BaseStatus:        domain.ProductManagementERPSyncStatusFailed,
			LastERPCheckedAt:  &now,
			SyncCooldownUntil: cooldown,
			LastSyncError:     msg,
			BaseSyncError:     msg,
		})
	})
}

func (s *productManagementService) markProductManagementImageSyncSucceeded(ctx context.Context, record *domain.ProductManagementRecord) {
	if record == nil {
		return
	}
	now := s.now()
	cooldown := record.SyncCooldownUntil
	if cooldown == nil || cooldown.Before(now) {
		next := now.Add(5 * time.Minute)
		cooldown = &next
	}
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateImageSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusSynced,
			ImageStatus:       domain.ProductManagementERPSyncStatusSynced,
			LastERPCheckedAt:  &now,
			LastERPSyncedAt:   &now,
			LastImageSyncedAt: &now,
			SyncCooldownUntil: cooldown,
			LastSyncError:     "",
			ImageSyncError:    "",
		})
	})
}

func (s *productManagementService) markProductManagementImageSyncFailed(ctx context.Context, record *domain.ProductManagementRecord, message string) {
	if record == nil {
		return
	}
	now := s.now()
	cooldown := record.SyncCooldownUntil
	if cooldown == nil || cooldown.Before(now) {
		next := now.Add(5 * time.Minute)
		cooldown = &next
	}
	status := domain.ProductManagementERPSyncStatusFailed
	msg := truncateProductManagementSyncError(message)
	if strings.Contains(msg, "待补充") || strings.Contains(msg, "待人工上传") {
		status = domain.ProductManagementERPSyncStatusWaitingImage
	}
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateImageSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            status,
			ImageStatus:       status,
			LastERPCheckedAt:  &now,
			SyncCooldownUntil: cooldown,
			LastSyncError:     msg,
			ImageSyncError:    msg,
		})
	})
}

func (s *productManagementService) markProductManagementImageSyncRetrying(ctx context.Context, record *domain.ProductManagementRecord, message string) {
	if record == nil {
		return
	}
	now := s.now()
	retryAt := now.Add(5 * time.Minute)
	msg := truncateProductManagementSyncError(message)
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateImageSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusCoolingDown,
			ImageStatus:       domain.ProductManagementERPSyncStatusCoolingDown,
			LastERPCheckedAt:  &now,
			SyncCooldownUntil: &retryAt,
			LastSyncError:     msg,
			ImageSyncError:    msg,
		})
	})
}

func (s *productManagementService) appendProductManagementTaskEventInTx(ctx context.Context, record *domain.ProductManagementRecord, eventType string, actorID int64, summary string, reason string) error {
	if record == nil {
		return nil
	}
	return s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.appendProductManagementTaskEvent(ctx, tx, record.TaskID, eventType, actorID, summary, record, reason)
	})
}

func (s *productManagementService) appendProductManagementTaskEvent(ctx context.Context, tx repo.Tx, taskID int64, eventType string, actorID int64, summary string, record *domain.ProductManagementRecord, reason string) error {
	if s.taskEvents == nil || taskID <= 0 {
		return nil
	}
	var actorPtr *int64
	if actorID > 0 {
		actorPtr = &actorID
	}
	payload := map[string]interface{}{
		"summary": strings.TrimSpace(summary),
	}
	if record != nil {
		payload["sku_code"] = strings.TrimSpace(record.SKUCode)
		payload["task_no"] = strings.TrimSpace(record.TaskNo)
		payload["image_source"] = string(record.ImageSource)
		payload["image_source_label"] = domain.ProductManagementImageSourceLabel(record.ImageSource)
	}
	if strings.TrimSpace(reason) != "" {
		payload["reason"] = strings.TrimSpace(reason)
	}
	_, err := s.taskEvents.Append(ctx, tx, taskID, eventType, actorPtr, payload)
	return err
}

func (s *productManagementService) getRecord(ctx context.Context, id int64) (*domain.ProductManagementRecord, *domain.AppError) {
	if id <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid product management record id", nil)
	}
	record, err := s.records.GetByID(ctx, id)
	if err != nil {
		return nil, infraAppError("get product management record", err)
	}
	if record == nil {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "product management record not found", nil)
	}
	return record, nil
}

func (s *productManagementService) getAndDecorateRecord(ctx context.Context, actor domain.RequestActor, id int64) (*domain.ProductManagementRecord, *domain.AppError) {
	record, appErr := s.getRecord(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	s.decorateRecords(ctx, actor, []*domain.ProductManagementRecord{record})
	return record, nil
}

func (s *productManagementService) decorateRecords(ctx context.Context, actor domain.RequestActor, records []*domain.ProductManagementRecord) {
	assetsByTaskID := make(map[int64][]*domain.TaskAsset)
	for _, record := range records {
		if record == nil {
			continue
		}
		record.ImageSourceLabel = domain.ProductManagementImageSourceLabel(record.ImageSource)
		record.CanMaintainImage = canMaintainProductManagementImage(actor, record)
		record.CanCrossTaskSelect = isProductManagementAdmin(actor)
		record.CanForceOverride = isProductManagementAdmin(actor)
		record.CanSyncERP = canSyncProductManagementERP(actor, record)
		if record.ImageAssetID != nil {
			record.ImagePreviewURL = fmt.Sprintf("/v1/assets/%d/preview", *record.ImageAssetID)
		}
		if record.ImageSelectionMode == domain.ProductManagementImageSelectionManual {
			continue
		}
		patch := s.autoImagePatchWithCache(ctx, record, assetsByTaskID)
		patchChanged := productManagementImagePatchChanged(record, patch)
		if patchChanged {
			s.persistAutoImagePatch(ctx, record, patch)
		}
		record.ImageSource = patch.ImageSource
		record.ImageSelectionMode = patch.ImageSelectionMode
		record.ImageAssetID = patch.ImageAssetID
		record.ImageAssetVersionID = patch.ImageAssetVersionID
		record.ImageFilename = patch.ImageFilename
		record.ImageMimeType = patch.ImageMimeType
		record.ImageMissingReason = patch.ImageMissingReason
		record.ImageSyncSource = patch.ImageSyncSource
		if record.ImageSyncSource == "" {
			record.ImageSyncSource = patch.ImageSource
		}
		record.ImageSyncStatus = productManagementDecoratedImageSyncStatus(record, patch, patchChanged)
		record.ImageSourceLabel = domain.ProductManagementImageSourceLabel(record.ImageSource)
		if record.ImageAssetID != nil {
			record.ImagePreviewURL = fmt.Sprintf("/v1/assets/%d/preview", *record.ImageAssetID)
		} else {
			record.ImagePreviewURL = ""
		}
	}
}

func (s *productManagementService) productManagementComboGroups(ctx context.Context, records []*domain.ProductManagementRecord) []domain.ProductManagementComboGroup {
	if len(records) == 0 {
		return []domain.ProductManagementComboGroup{}
	}
	if s.skuCombos == nil {
		return productManagementSingleGroups(records)
	}
	skus := make([]string, 0, len(records))
	seenSKU := make(map[string]struct{})
	for _, record := range records {
		if record == nil {
			continue
		}
		sku := strings.TrimSpace(record.SKUCode)
		if sku == "" {
			continue
		}
		key := strings.ToUpper(sku)
		if _, ok := seenSKU[key]; !ok {
			seenSKU[key] = struct{}{}
			skus = append(skus, sku)
		}
	}
	if len(skus) == 0 {
		return productManagementSingleGroups(records)
	}
	relations, err := s.skuCombos.ListRelationsByChildSKUs(ctx, skus)
	if err != nil {
		return productManagementSingleGroups(records)
	}
	relationsByChild := make(map[string][]*domain.OMPSKUComboRelationWithRecord)
	for _, rel := range relations {
		if rel == nil {
			continue
		}
		childKey := strings.ToUpper(strings.TrimSpace(rel.Relation.ChildSKUCode))
		if childKey == "" {
			continue
		}
		relationsByChild[childKey] = append(relationsByChild[childKey], rel)
	}
	groupMap := make(map[string]*domain.ProductManagementComboGroup)
	groups := make([]domain.ProductManagementComboGroup, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		childKey := strings.ToUpper(strings.TrimSpace(record.SKUCode))
		rels := relationsByChild[childKey]
		if len(rels) == 0 {
			groups = append(groups, productManagementSingleGroup(record))
			continue
		}
		addedToCombo := false
		for _, rel := range rels {
			if rel == nil {
				continue
			}
			comboCode := strings.TrimSpace(rel.Relation.ComboSKUCode)
			if comboCode == "" {
				continue
			}
			groupKey := "combo:" + strings.ToUpper(comboCode)
			group := groupMap[groupKey]
			if group == nil {
				group = &domain.ProductManagementComboGroup{
					GroupKey:     groupKey,
					GroupType:    "combo",
					ComboSKUCode: comboCode,
					Children:     []domain.ProductManagementComboChild{},
				}
				if rel.Record != nil {
					group.ComboName = strings.TrimSpace(rel.Record.Name)
					group.ComboShortName = strings.TrimSpace(rel.Record.ShortName)
					group.ERPIID = strings.TrimSpace(rel.Record.ERPIID)
					group.Enabled = rel.Record.Enabled
					group.CostPrice = rel.Record.CostPrice
					group.SalePrice = rel.Record.SalePrice
					group.ModifiedAt = rel.Record.ModifiedAt
					if !rel.Record.LastSyncedAt.IsZero() {
						group.LastSyncedAt = &rel.Record.LastSyncedAt
					}
				}
				groupMap[groupKey] = group
				groups = append(groups, *group)
			}
			qty := rel.Relation.Quantity
			if qty <= 0 {
				qty = 1
			}
			group.Children = append(group.Children, domain.ProductManagementComboChild{Record: record, Quantity: qty})
			addedToCombo = true
		}
		if !addedToCombo {
			groups = append(groups, productManagementSingleGroup(record))
		}
	}
	for idx := range groups {
		if groups[idx].GroupType != "combo" {
			continue
		}
		if group := groupMap[groups[idx].GroupKey]; group != nil {
			groups[idx] = *group
		}
	}
	return groups
}

func productManagementSingleGroups(records []*domain.ProductManagementRecord) []domain.ProductManagementComboGroup {
	groups := make([]domain.ProductManagementComboGroup, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		groups = append(groups, productManagementSingleGroup(record))
	}
	return groups
}

func productManagementSingleGroup(record *domain.ProductManagementRecord) domain.ProductManagementComboGroup {
	return domain.ProductManagementComboGroup{
		GroupKey:  fmt.Sprintf("single:%d", record.ID),
		GroupType: "single",
		Children:  []domain.ProductManagementComboChild{{Record: record, Quantity: 1}},
	}
}

func isMySQLTableMissing(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1146
}

func productManagementDecoratedImageSyncStatus(record *domain.ProductManagementRecord, patch repo.ProductManagementImagePatch, patchChanged bool) domain.ProductManagementERPSyncStatus {
	patchStatus := patch.ImageSyncStatus
	if patchStatus == "" {
		patchStatus = domain.ProductManagementERPSyncStatusWaitingImage
		if patch.ImageAssetID != nil && *patch.ImageAssetID > 0 {
			patchStatus = domain.ProductManagementERPSyncStatusPendingSync
		}
	}
	if record == nil {
		return patchStatus
	}
	if productManagementStatusInFlight(record.ERPSyncStatus) {
		return record.ERPSyncStatus
	}
	if productManagementStatusInFlight(record.ImageSyncStatus) {
		return record.ImageSyncStatus
	}
	if !patchChanged && record.ImageSyncStatus != "" {
		if record.ImageSyncStatus == domain.ProductManagementERPSyncStatusSynced && record.LastImageSyncedAt == nil {
			return domain.ProductManagementERPSyncStatusPendingSync
		}
		return record.ImageSyncStatus
	}
	return patchStatus
}

func (s *productManagementService) persistAutoImagePatch(ctx context.Context, record *domain.ProductManagementRecord, patch repo.ProductManagementImagePatch) {
	if record == nil || !productManagementImagePatchChanged(record, patch) {
		return
	}
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateImage(ctx, tx, record.ID, patch)
	})
}

func productManagementImagePatchChanged(record *domain.ProductManagementRecord, patch repo.ProductManagementImagePatch) bool {
	if record.ImageSource != patch.ImageSource {
		return true
	}
	if record.ImageSelectionMode != patch.ImageSelectionMode {
		return true
	}
	if !sameProductManagementInt64Ptr(record.ImageAssetID, patch.ImageAssetID) {
		return true
	}
	if !sameProductManagementInt64Ptr(record.ImageAssetVersionID, patch.ImageAssetVersionID) {
		return true
	}
	if strings.TrimSpace(record.ImageFilename) != strings.TrimSpace(patch.ImageFilename) {
		return true
	}
	if strings.TrimSpace(record.ImageMimeType) != strings.TrimSpace(patch.ImageMimeType) {
		return true
	}
	return strings.TrimSpace(record.ImageMissingReason) != strings.TrimSpace(patch.ImageMissingReason)
}

func sameProductManagementInt64Ptr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (s *productManagementService) autoImagePatch(ctx context.Context, record *domain.ProductManagementRecord) repo.ProductManagementImagePatch {
	return s.autoImagePatchWithCache(ctx, record, nil)
}

func (s *productManagementService) autoImagePatchWithCache(ctx context.Context, record *domain.ProductManagementRecord, assetsByTaskID map[int64][]*domain.TaskAsset) repo.ProductManagementImagePatch {
	if record != nil && record.ImageSource == domain.ProductManagementImageSourceAutoOnClose && record.ImageAssetID != nil && *record.ImageAssetID > 0 {
		return repo.ProductManagementImagePatch{
			ImageSource:         record.ImageSource,
			ImageSelectionMode:  record.ImageSelectionMode,
			ImageAssetID:        record.ImageAssetID,
			ImageAssetVersionID: record.ImageAssetVersionID,
			ImageFilename:       record.ImageFilename,
			ImageMimeType:       record.ImageMimeType,
			ImageMissingReason:  record.ImageMissingReason,
			ImageSyncSource:     record.ImageSyncSource,
			ImageSyncStatus:     record.ImageSyncStatus,
		}
	}
	candidates, err := s.imageCandidatesForRecordWithCache(ctx, record, assetsByTaskID)
	if err != nil || len(candidates) == 0 {
		reason := "ERP 图片待补充"
		if record != nil && strings.EqualFold(strings.TrimSpace(record.TaskType), string(domain.TaskTypePurchaseTask)) {
			reason = "采购任务不会自动产生设计成品图，请上传 ERP 商品图"
		}
		return repo.ProductManagementImagePatch{
			ImageSource:        domain.ProductManagementImageSourceMissing,
			ImageSelectionMode: domain.ProductManagementImageSelectionAuto,
			ImageMissingReason: reason,
			ImageSyncSource:    domain.ProductManagementImageSourceMissing,
			ImageSyncStatus:    domain.ProductManagementERPSyncStatusWaitingImage,
		}
	}
	return imagePatchFromCandidate(candidates[0])
}

func (s *productManagementService) imageCandidatesForRecord(ctx context.Context, record *domain.ProductManagementRecord) ([]*domain.ProductManagementImageCandidate, error) {
	return s.imageCandidatesForRecordWithCache(ctx, record, nil)
}

func (s *productManagementService) imageCandidatesForRecordWithCache(ctx context.Context, record *domain.ProductManagementRecord, assetsByTaskID map[int64][]*domain.TaskAsset) ([]*domain.ProductManagementImageCandidate, error) {
	if record == nil || record.TaskID <= 0 {
		return []*domain.ProductManagementImageCandidate{}, nil
	}
	assets, ok := assetsByTaskID[record.TaskID]
	if assetsByTaskID == nil {
		ok = false
	}
	if !ok {
		var err error
		assets, err = s.taskAssets.ListByTaskID(ctx, record.TaskID)
		if err != nil {
			return nil, err
		}
		if assetsByTaskID != nil {
			assetsByTaskID[record.TaskID] = assets
		}
	}
	sku := strings.TrimSpace(record.SKUCode)
	candidates := make([]*domain.ProductManagementImageCandidate, 0, len(assets))
	sources := []domain.ProductManagementImageSource{
		domain.ProductManagementImageSourceERPProduct,
	}
	if !strings.EqualFold(strings.TrimSpace(record.TaskType), string(domain.TaskTypePurchaseTask)) {
		sources = append(sources,
			domain.ProductManagementImageSourceDelivery,
			domain.ProductManagementImageSourceDerivedPreview,
			domain.ProductManagementImageSourceTaskReference,
		)
	}
	for _, source := range sources {
		for _, asset := range bestAssetsBySource(assets, sku, source) {
			candidates = append(candidates, candidateFromAsset(asset, record.TaskNo, source))
		}
	}
	return candidates, nil
}

func bestAssetsBySource(assets []*domain.TaskAsset, sku string, source domain.ProductManagementImageSource) []*domain.TaskAsset {
	out := make([]*domain.TaskAsset, 0, len(assets))
	for _, asset := range assets {
		if asset == nil || asset.AssetID == nil || asset.DeletedAt != nil || asset.IsArchived {
			continue
		}
		if !isProductManagementERPImageAsset(asset) {
			continue
		}
		if strings.TrimSpace(sku) != "" && asset.ScopeSKUCode != nil && strings.TrimSpace(*asset.ScopeSKUCode) != "" && !strings.EqualFold(strings.TrimSpace(*asset.ScopeSKUCode), sku) {
			continue
		}
		assetType := domain.NormalizeTaskAssetType(asset.AssetType)
		switch source {
		case domain.ProductManagementImageSourceERPProduct:
			if !assetType.IsERPProductImage() {
				continue
			}
		case domain.ProductManagementImageSourceDelivery:
			if !assetType.IsDelivery() {
				continue
			}
		case domain.ProductManagementImageSourceDerivedPreview:
			if !assetType.IsPreview() && !assetType.IsDesignThumb() {
				continue
			}
		case domain.ProductManagementImageSourceTaskReference:
			if !assetType.IsReference() {
				continue
			}
		default:
			continue
		}
		out = appendNewestTaskAsset(out, asset)
	}
	return out
}

func appendNewestTaskAsset(items []*domain.TaskAsset, asset *domain.TaskAsset) []*domain.TaskAsset {
	for idx, existing := range items {
		if existing != nil && existing.AssetID != nil && asset.AssetID != nil && *existing.AssetID == *asset.AssetID {
			if taskAssetNewer(asset, existing) {
				items[idx] = asset
			}
			return items
		}
	}
	return append(items, asset)
}

func taskAssetNewer(a, b *domain.TaskAsset) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	if a.AssetVersionNo != nil && b.AssetVersionNo != nil && *a.AssetVersionNo != *b.AssetVersionNo {
		return *a.AssetVersionNo > *b.AssetVersionNo
	}
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.After(b.CreatedAt)
	}
	return a.ID > b.ID
}

func candidateFromAsset(asset *domain.TaskAsset, taskNo string, source domain.ProductManagementImageSource) *domain.ProductManagementImageCandidate {
	if asset == nil || asset.AssetID == nil {
		return nil
	}
	sku := ""
	if asset.ScopeSKUCode != nil {
		sku = strings.TrimSpace(*asset.ScopeSKUCode)
	}
	mimeType := ""
	if asset.MimeType != nil {
		mimeType = strings.TrimSpace(*asset.MimeType)
	}
	return &domain.ProductManagementImageCandidate{
		AssetID:        *asset.AssetID,
		AssetVersionID: asset.ID,
		TaskID:         asset.TaskID,
		TaskNo:         strings.TrimSpace(taskNo),
		SKUCode:        sku,
		Source:         source,
		SourceLabel:    domain.ProductManagementImageSourceLabel(source),
		PreviewURL:     fmt.Sprintf("/v1/assets/%d/preview", *asset.AssetID),
		FileName:       asset.FileName,
		MimeType:       mimeType,
		CreatedAt:      asset.CreatedAt,
	}
}

func imagePatchFromCandidate(candidate *domain.ProductManagementImageCandidate) repo.ProductManagementImagePatch {
	if candidate == nil || candidate.AssetID <= 0 {
		return repo.ProductManagementImagePatch{
			ImageSource:        domain.ProductManagementImageSourceMissing,
			ImageSelectionMode: domain.ProductManagementImageSelectionAuto,
			ImageMissingReason: "ERP 图片待补充",
			ImageSyncSource:    domain.ProductManagementImageSourceMissing,
			ImageSyncStatus:    domain.ProductManagementERPSyncStatusWaitingImage,
		}
	}
	assetID := candidate.AssetID
	versionID := candidate.AssetVersionID
	mode := domain.ProductManagementImageSelectionAuto
	if candidate.Source == domain.ProductManagementImageSourceManual {
		mode = domain.ProductManagementImageSelectionManual
	}
	return repo.ProductManagementImagePatch{
		ImageSource:         candidate.Source,
		ImageSelectionMode:  mode,
		ImageAssetID:        &assetID,
		ImageAssetVersionID: &versionID,
		ImageFilename:       candidate.FileName,
		ImageMimeType:       candidate.MimeType,
		ImageSyncSource:     candidate.Source,
		ImageSyncStatus:     domain.ProductManagementERPSyncStatusPendingSync,
	}
}

func isProductManagementAdmin(actor domain.RequestActor) bool {
	return actorHasAnyRole(actor, domain.RoleERP, domain.RoleAdmin, domain.RoleSuperAdmin)
}

func canMaintainProductManagementImage(actor domain.RequestActor, record *domain.ProductManagementRecord) bool {
	if record == nil {
		return false
	}
	if isProductManagementAdmin(actor) {
		return true
	}
	return actor.ID > 0 && actor.ID == record.CreatorID
}

func canSyncProductManagementERP(actor domain.RequestActor, record *domain.ProductManagementRecord) bool {
	if record == nil {
		return false
	}
	if isProductManagementAdmin(actor) {
		return true
	}
	return actor.ID > 0 && actor.ID == record.CreatorID
}

func actorHasAnyRole(actor domain.RequestActor, roles ...domain.Role) bool {
	for _, role := range actor.Roles {
		for _, candidate := range roles {
			if role == candidate {
				return true
			}
		}
	}
	return false
}

func normalizeProductManagementPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func isProductManagementERPImageAsset(asset *domain.TaskAsset) bool {
	if asset == nil {
		return false
	}
	mimeType := ""
	if asset.MimeType != nil {
		mimeType = strings.ToLower(strings.TrimSpace(*asset.MimeType))
	}
	switch mimeType {
	case "image/jpeg", "image/jpg", "image/png", "image/webp", "image/gif":
		return true
	case "image/vnd.adobe.photoshop", "image/tiff", "image/heic", "image/heif", "image/avif":
		return false
	}
	filename := strings.ToLower(strings.TrimSpace(asset.FileName))
	for _, suffix := range []string{".jpg", ".jpeg", ".png", ".webp", ".gif"} {
		if strings.HasSuffix(filename, suffix) {
			return true
		}
	}
	return false
}

func isAbsoluteHTTPURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func truncateProductManagementSyncError(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "ERP 图片同步失败"
	}
	const max = 500
	runes := []rune(message)
	if len(runes) <= max {
		return message
	}
	return string(runes[:max])
}

func infraAppError(message string, err error) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInternalError, message, map[string]string{"cause": err.Error()})
}
