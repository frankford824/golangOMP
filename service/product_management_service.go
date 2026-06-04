package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type ProductManagementService interface {
	List(ctx context.Context, filter repo.ProductManagementListFilter) ([]*domain.ProductManagementRecord, domain.PaginationMeta, *domain.AppError)
	GetByTaskID(ctx context.Context, taskID int64) ([]*domain.ProductManagementRecord, *domain.AppError)
	ListImageCandidates(ctx context.Context, actor domain.RequestActor, recordID int64) ([]*domain.ProductManagementImageCandidate, *domain.AppError)
	ReparseImage(ctx context.Context, actor domain.RequestActor, recordID int64) (*domain.ProductManagementRecord, *domain.AppError)
	SetManualImage(ctx context.Context, actor domain.RequestActor, recordID int64, assetID int64) (*domain.ProductManagementRecord, *domain.AppError)
	RequestSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError)
	ProcessQueuedERPSync(ctx context.Context, limit int) (int, *domain.AppError)
}

type ProductManagementServiceOption func(*productManagementService)

type productManagementService struct {
	records      repo.ProductManagementRepo
	taskAssets   repo.TaskAssetRepo
	assetSearch  repo.TaskAssetSearchRepo
	txRunner     repo.TxRunner
	erpBridge    ERPBridgeService
	ossDirect    *OSSDirectService
	uploadClient UploadServiceClient
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
	patch := imagePatchFromCandidate(candidateFromAsset(row.Asset, taskNo, domain.ProductManagementImageSourceManual))
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
		if appErr := s.syncRecordToERP(ctx, record); appErr != nil {
			s.markProductManagementSyncFailed(ctx, record, appErr.Message)
		} else {
			s.markProductManagementSyncSucceeded(ctx, record)
		}
		processed++
	}
	return processed, nil
}

func (s *productManagementService) RequestSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError) {
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
		return s.records.UpdateSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            status,
			LastERPCheckedAt:  &now,
			SyncCooldownUntil: &cooldown,
			LastSyncError:     errMsg,
		})
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

func (s *productManagementService) syncRecordToERP(ctx context.Context, record *domain.ProductManagementRecord) *domain.AppError {
	if record == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "product management record is required", nil)
	}
	if strings.TrimSpace(record.SKUCode) == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "SKU is required for ERP image sync", nil)
	}
	imageURL, appErr := s.resolveERPImageURL(ctx, record)
	if appErr != nil {
		return appErr
	}
	productName := firstNonEmptyString(strings.TrimSpace(record.ProductName), strings.TrimSpace(record.SKUCode))
	payload := domain.ERPProductUpsertPayload{
		ProductID:        strings.TrimSpace(record.SKUCode),
		SKUID:            strings.TrimSpace(record.SKUCode),
		SKUCode:          strings.TrimSpace(record.SKUCode),
		IID:              strings.TrimSpace(record.ProductIID),
		Name:             productName,
		ProductName:      productName,
		ShortName:        productName,
		ProductShortName: productName,
		Pic:              imageURL,
		PicBig:           imageURL,
		SKUPic:           imageURL,
		Operation:        "product_management_image_sync",
		Source:           "product_management",
		TaskContext: &domain.ERPTaskFilingContext{
			TaskNo: strings.TrimSpace(record.TaskNo),
			Remark: fmt.Sprintf("产品管理同步 ERP 图片，图片来源：%s", domain.ProductManagementImageSourceLabel(record.ImageSource)),
		},
	}
	if record.CostPrice != nil && *record.CostPrice > 0 {
		cost := *record.CostPrice
		payload.CostPrice = &cost
	}
	_, appErr = s.erpBridge.UpsertProduct(ctx, payload)
	return appErr
}

func (s *productManagementService) resolveERPImageURL(ctx context.Context, record *domain.ProductManagementRecord) (string, *domain.AppError) {
	if record == nil || record.ImageAssetID == nil || *record.ImageAssetID <= 0 {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片待补充，无法同步 ERP", nil)
	}
	row, err := s.assetSearch.GetCurrentByAssetID(ctx, *record.ImageAssetID)
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
	filename := strings.TrimSpace(asset.FileName)
	if asset.OriginalName != nil && strings.TrimSpace(*asset.OriginalName) != "" {
		filename = strings.TrimSpace(*asset.OriginalName)
	}
	if s.ossDirect != nil && s.ossDirect.Enabled() {
		if info := s.ossDirect.PresignDownloadURLWithFilename(storageKey, filename); info != nil && isAbsoluteHTTPURL(info.DownloadURL) {
			return strings.TrimSpace(info.DownloadURL), nil
		}
	}
	if s.uploadClient != nil {
		if directURL := s.uploadClient.BuildBrowserFileURL(storageKey); directURL != nil && isAbsoluteHTTPURL(*directURL) {
			return strings.TrimSpace(*directURL), nil
		}
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
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusSynced,
			LastERPCheckedAt:  &now,
			LastERPSyncedAt:   &now,
			SyncCooldownUntil: cooldown,
			LastSyncError:     "",
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
			LastERPCheckedAt:  &now,
			SyncCooldownUntil: cooldown,
			LastSyncError:     truncateProductManagementSyncError(message),
		})
	})
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
		s.persistAutoImagePatch(ctx, record, patch)
		record.ImageSource = patch.ImageSource
		record.ImageSelectionMode = patch.ImageSelectionMode
		record.ImageAssetID = patch.ImageAssetID
		record.ImageAssetVersionID = patch.ImageAssetVersionID
		record.ImageFilename = patch.ImageFilename
		record.ImageMimeType = patch.ImageMimeType
		record.ImageMissingReason = patch.ImageMissingReason
		record.ImageSourceLabel = domain.ProductManagementImageSourceLabel(record.ImageSource)
		if record.ImageAssetID != nil {
			record.ImagePreviewURL = fmt.Sprintf("/v1/assets/%d/preview", *record.ImageAssetID)
		} else {
			record.ImagePreviewURL = ""
		}
	}
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
	candidates, err := s.imageCandidatesForRecordWithCache(ctx, record, assetsByTaskID)
	if err != nil || len(candidates) == 0 {
		return repo.ProductManagementImagePatch{
			ImageSource:        domain.ProductManagementImageSourceMissing,
			ImageSelectionMode: domain.ProductManagementImageSelectionAuto,
			ImageMissingReason: "ERP 图片待补充",
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
	for _, source := range []domain.ProductManagementImageSource{
		domain.ProductManagementImageSourceDelivery,
		domain.ProductManagementImageSourceDerivedPreview,
		domain.ProductManagementImageSourceTaskReference,
	} {
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
