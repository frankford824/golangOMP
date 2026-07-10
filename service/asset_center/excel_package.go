package asset_center

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const (
	MaxExcelPackageRows       = 500
	MaxExcelPackageTotalFiles = 1000
	MaxExcelPackageTotalBytes = int64(512 * 1024 * 1024)
)

type ExcelPackageRow struct {
	RowNumber int    `json:"row_number,omitempty"`
	OrderNo   string `json:"order_no"`
	SKUCode   string `json:"sku_code"`
	SKUName   string `json:"sku_name,omitempty"`
	Quantity  int    `json:"quantity"`
	Address   string `json:"address,omitempty"`
	Keyword   string `json:"keyword,omitempty"`
}

type ExcelPackageManifest struct {
	Items        []ExcelPackageItem    `json:"items"`
	Failures     []ExcelPackageFailure `json:"failures,omitempty"`
	SuccessCount int                   `json:"success_count"`
	FailureCount int                   `json:"failure_count"`
	TotalFiles   int                   `json:"total_files"`
	TotalSize    int64                 `json:"total_size"`
	ExpiresAt    *time.Time            `json:"expires_at,omitempty"`
}

type ExcelPackageItem struct {
	RowNumber   int        `json:"row_number,omitempty"`
	OrderNo     string     `json:"order_no"`
	SKUCode     string     `json:"sku_code"`
	SKUName     string     `json:"sku_name,omitempty"`
	Quantity    int        `json:"quantity"`
	AssetID     int64      `json:"asset_id"`
	ResourceID  string     `json:"resource_id,omitempty"`
	SourceType  string     `json:"source_type,omitempty"`
	TaskID      int64      `json:"task_id"`
	TaskNo      string     `json:"task_no,omitempty"`
	Filename    string     `json:"filename"`
	FileSize    int64      `json:"file_size"`
	MimeType    string     `json:"mime_type,omitempty"`
	DownloadURL string     `json:"download_url"`
	Address     string     `json:"address,omitempty"`
	OriginPath  string     `json:"origin_path,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type ExcelPackageFailure struct {
	RowNumber int    `json:"row_number,omitempty"`
	OrderNo   string `json:"order_no,omitempty"`
	SKUCode   string `json:"sku_code,omitempty"`
	SKUName   string `json:"sku_name,omitempty"`
	Quantity  int    `json:"quantity,omitempty"`
	Address   string `json:"address,omitempty"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
}

type scoredExcelAsset struct {
	system   *repo.TaskAssetSearchRow
	external *AssetDetail
	score    int
	ready    bool
	updated  time.Time
}

type preparedExcelPackageAsset struct {
	item           ExcelPackageItem
	failureReason  string
	failureMessage string
}

func (s *Service) BuildExcelPackageManifest(ctx context.Context, rows []ExcelPackageRow) (*ExcelPackageManifest, *domain.AppError) {
	if len(rows) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rows must not be empty", nil)
	}
	if len(rows) > MaxExcelPackageRows {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rows exceed excel package limit", map[string]interface{}{
			"limit": MaxExcelPackageRows,
		})
	}
	if s.presigner == nil || !s.presigner.Enabled() {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "oss direct download presigner is not configured", nil)
	}

	manifest := &ExcelPackageManifest{
		Items:    make([]ExcelPackageItem, 0, len(rows)),
		Failures: make([]ExcelPackageFailure, 0),
	}
	var totalBytes int64
	var totalFiles int
	matchCache := map[string]scoredExcelAsset{}
	preparedCache := map[string]preparedExcelPackageAsset{}

	for idx, raw := range rows {
		row := normalizeExcelPackageRow(raw, idx+2)
		if row.OrderNo == "" {
			manifest.Failures = append(manifest.Failures, row.failure("missing_order_no", "订单号不能为空"))
			continue
		}
		if row.SKUCode == "" && row.SKUName == "" {
			manifest.Failures = append(manifest.Failures, row.failure("missing_sku", "SKU 编码或 SKU 名称不能为空"))
			continue
		}
		if row.Quantity <= 0 {
			manifest.Failures = append(manifest.Failures, row.failure("invalid_quantity", "数量必须大于 0"))
			continue
		}
		if totalFiles+row.Quantity > MaxExcelPackageTotalFiles {
			manifest.Failures = append(manifest.Failures, row.failure("total_file_limit_exceeded", fmt.Sprintf("总文件数超过 %d 个", MaxExcelPackageTotalFiles)))
			continue
		}

		cacheKey := excelPackageMatchCacheKey(row)
		match, ok := matchCache[cacheKey]
		if !ok {
			var appErr *domain.AppError
			match, appErr = s.matchExcelPackageAsset(ctx, row)
			if appErr != nil {
				return nil, appErr
			}
			matchCache[cacheKey] = match
		}
		if match.system == nil && match.external == nil {
			manifest.Failures = append(manifest.Failures, row.failure("asset_not_found", "未找到匹配的 JPG/PNG 资产"))
			continue
		}

		prepared, ok := preparedCache[cacheKey]
		if !ok {
			item, failure := s.buildExcelPackageItem(ctx, match, row)
			prepared.item = item
			if failure != nil {
				prepared.failureReason = failure.Reason
				prepared.failureMessage = failure.Message
			}
			preparedCache[cacheKey] = prepared
		}
		if prepared.failureReason != "" {
			manifest.Failures = append(manifest.Failures, row.failure(prepared.failureReason, prepared.failureMessage))
			continue
		}
		item := applyExcelPackageRow(prepared.item, row)
		nextBytes := totalBytes + item.FileSize*int64(row.Quantity)
		if nextBytes > MaxExcelPackageTotalBytes {
			manifest.Failures = append(manifest.Failures, row.failure("total_size_limit_exceeded", fmt.Sprintf("总大小超过 %d MB", MaxExcelPackageTotalBytes/1024/1024)))
			continue
		}
		totalBytes = nextBytes
		totalFiles += row.Quantity
		if manifest.ExpiresAt == nil || (item.ExpiresAt != nil && item.ExpiresAt.Before(*manifest.ExpiresAt)) {
			manifest.ExpiresAt = item.ExpiresAt
		}
		manifest.Items = append(manifest.Items, item)
	}

	manifest.SuccessCount = len(manifest.Items)
	manifest.FailureCount = len(manifest.Failures)
	manifest.TotalFiles = totalFiles
	manifest.TotalSize = totalBytes
	return manifest, nil
}

func applyExcelPackageRow(item ExcelPackageItem, row ExcelPackageRow) ExcelPackageItem {
	item.RowNumber = row.RowNumber
	item.OrderNo = row.OrderNo
	item.SKUCode = row.SKUCode
	item.SKUName = row.SKUName
	item.Quantity = row.Quantity
	item.Address = row.Address
	return item
}

func normalizeExcelPackageRow(row ExcelPackageRow, fallbackRowNumber int) ExcelPackageRow {
	row.OrderNo = strings.TrimSpace(row.OrderNo)
	row.SKUCode = strings.ToUpper(strings.TrimSpace(row.SKUCode))
	row.SKUName = strings.TrimSpace(row.SKUName)
	row.Address = strings.TrimSpace(row.Address)
	row.Keyword = strings.TrimSpace(row.Keyword)
	if row.RowNumber <= 0 {
		row.RowNumber = fallbackRowNumber
	}
	return row
}

func excelPackageMatchCacheKey(row ExcelPackageRow) string {
	return strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(row.SKUCode)),
		strings.ToLower(strings.TrimSpace(row.SKUName)),
		strings.ToLower(strings.TrimSpace(row.Keyword)),
	}, "\x00")
}

func (r ExcelPackageRow) failure(reason, message string) ExcelPackageFailure {
	return ExcelPackageFailure{
		RowNumber: r.RowNumber,
		OrderNo:   r.OrderNo,
		SKUCode:   r.SKUCode,
		SKUName:   r.SKUName,
		Quantity:  r.Quantity,
		Address:   r.Address,
		Reason:    reason,
		Message:   message,
	}
}

func (s *Service) matchExcelPackageAsset(ctx context.Context, row ExcelPackageRow) (scoredExcelAsset, *domain.AppError) {
	keyword := firstExcelPackageKeyword(row.SKUCode, row.SKUName)
	resultRows, _, err := s.searchRepo.Search(ctx, domain.AssetSearchQuery{
		Keyword:    keyword,
		Page:       1,
		Size:       100,
		IsArchived: domain.AssetArchiveFilterFalse,
		TaskStatus: domain.AssetTaskStatusFilterAll,
	})
	if err != nil {
		return scoredExcelAsset{}, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}

	candidates := make([]scoredExcelAsset, 0, len(resultRows))
	for _, candidate := range resultRows {
		score := scoreExcelPackageAsset(candidate, row)
		if score <= 0 {
			continue
		}
		updated := time.Time{}
		if candidate != nil && candidate.Asset != nil {
			updated = candidate.Asset.CreatedAt
		}
		ready := candidate != nil && candidate.Asset != nil && strings.TrimSpace(stringPtrValue(candidate.Asset.StorageKey)) != ""
		candidates = append(candidates, scoredExcelAsset{system: candidate, score: score, ready: ready, updated: updated})
	}
	externalRows, externalErr := s.batchSearchExternalRows(ctx, keyword, "jpg_png")
	if externalErr != nil {
		return scoredExcelAsset{}, domain.NewAppError(domain.ErrCodeInternalError, externalErr.Error(), nil)
	}
	for _, candidate := range externalRows {
		score := scoreExcelPackageExternalAsset(candidate, row)
		if score <= 0 {
			continue
		}
		ready := strings.EqualFold(strings.TrimSpace(candidate.OSSSyncStatus), string(domain.ExternalAssetOSSStatusReady))
		candidates = append(candidates, scoredExcelAsset{external: candidate, score: score, ready: ready, updated: candidate.UpdatedAt})
	}
	if len(candidates) == 0 {
		return scoredExcelAsset{}, nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].ready != candidates[j].ready {
			return candidates[i].ready
		}
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].updated.After(candidates[j].updated)
	})
	candidates = dedupeExcelPackageExternalFileCandidates(candidates)
	return candidates[0], nil
}

func firstExcelPackageKeyword(skuCode, skuName string) string {
	if strings.TrimSpace(skuCode) != "" {
		return strings.TrimSpace(skuCode)
	}
	return strings.TrimSpace(skuName)
}

func scoreExcelPackageAsset(row *repo.TaskAssetSearchRow, req ExcelPackageRow) int {
	if row == nil || row.Asset == nil || row.Task == nil {
		return 0
	}
	asset := row.Asset
	if asset.DeletedAt != nil || asset.CleanedAt != nil {
		return 0
	}
	if asset.UploadStatus == nil || domain.DesignAssetUploadStatus(strings.TrimSpace(*asset.UploadStatus)) != domain.DesignAssetUploadStatusUploaded {
		return 0
	}
	if !isExcelPackageImage(asset) {
		return 0
	}

	score := 1
	sku := strings.ToUpper(strings.TrimSpace(req.SKUCode))
	skuName := strings.TrimSpace(req.SKUName)
	keyword := strings.TrimSpace(req.Keyword)
	fileName := strings.ToUpper(strings.TrimSpace(asset.FileName + " " + stringPtrValue(asset.OriginalName)))
	scopeSKU := strings.ToUpper(stringPtrValue(asset.ScopeSKUCode))
	taskSKU := strings.ToUpper(strings.TrimSpace(row.Task.SKUCode))
	taskPrimarySKU := strings.ToUpper(strings.TrimSpace(row.Task.PrimarySKUCode))
	productName := strings.TrimSpace(row.Task.ProductNameSnapshot)

	if sku != "" {
		switch {
		case scopeSKU == sku:
			score += 120
		case taskSKU == sku || taskPrimarySKU == sku:
			score += 90
		case strings.Contains(fileName, sku):
			score += 70
		default:
			return 0
		}
	}
	if skuName != "" {
		if strings.Contains(strings.ToLower(fileName), strings.ToLower(skuName)) ||
			strings.Contains(strings.ToLower(productName), strings.ToLower(skuName)) {
			score += 20
		}
	}
	if keyword != "" {
		if strings.Contains(strings.ToLower(fileName), strings.ToLower(keyword)) ||
			strings.Contains(strings.ToLower(productName), strings.ToLower(keyword)) {
			score += 35
		} else {
			return 0
		}
	}

	switch asset.AssetType.Canonical() {
	case domain.TaskAssetTypeDelivery:
		score += 40
	case domain.TaskAssetTypePreview:
		score += 25
	case domain.TaskAssetTypeDesignThumb:
		score += 20
	case domain.TaskAssetTypeReference:
		score += 10
	}
	return score
}

func scoreExcelPackageExternalAsset(asset *AssetDetail, req ExcelPackageRow) int {
	if asset == nil || asset.SourceType != string(domain.AssetResourceSourceExternal) {
		return 0
	}
	if !isExcelPackageExternalImage(asset) {
		return 0
	}
	sku := strings.ToUpper(strings.TrimSpace(req.SKUCode))
	skuName := strings.TrimSpace(req.SKUName)
	keyword := strings.TrimSpace(req.Keyword)
	fileName := strings.ToUpper(strings.TrimSpace(asset.FileName + " " + asset.OriginalFilename))
	originPath := strings.ToUpper(strings.TrimSpace(asset.OriginPath + " " + asset.ProductName))

	score := 35
	if sku != "" {
		switch {
		case strings.HasPrefix(fileName, sku):
			score += 120
		case strings.Contains(fileName, sku):
			score += 100
		case strings.Contains(originPath, sku):
			score += 70
		default:
			return 0
		}
	}
	if skuName != "" {
		if strings.Contains(strings.ToLower(fileName), strings.ToLower(skuName)) ||
			strings.Contains(strings.ToLower(originPath), strings.ToLower(skuName)) {
			score += 20
		}
	}
	if keyword != "" {
		if strings.Contains(strings.ToLower(fileName), strings.ToLower(keyword)) ||
			strings.Contains(strings.ToLower(originPath), strings.ToLower(keyword)) {
			score += 35
		} else {
			return 0
		}
	}
	return score
}

func dedupeExcelPackageExternalFileCandidates(candidates []scoredExcelAsset) []scoredExcelAsset {
	if len(candidates) < 2 {
		return candidates
	}
	seen := map[string]struct{}{}
	out := make([]scoredExcelAsset, 0, len(candidates))
	for _, candidate := range candidates {
		key := ""
		if candidate.external != nil {
			key = batchSearchExternalFileFingerprint(candidate.external)
		}
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		out = append(out, candidate)
	}
	return out
}

func (s *Service) buildExcelPackageItem(ctx context.Context, match scoredExcelAsset, req ExcelPackageRow) (ExcelPackageItem, *ExcelPackageFailure) {
	if match.external != nil {
		return s.buildExternalExcelPackageItem(ctx, match.external, req)
	}
	return s.buildSystemExcelPackageItem(match.system, req)
}

func (s *Service) buildSystemExcelPackageItem(row *repo.TaskAssetSearchRow, req ExcelPackageRow) (ExcelPackageItem, *ExcelPackageFailure) {
	if row == nil || row.Asset == nil || row.Task == nil {
		f := req.failure("asset_not_found", "未找到匹配的 JPG/PNG 资产")
		return ExcelPackageItem{}, &f
	}
	asset := row.Asset
	assetID := valueInt64(asset.AssetID, asset.ID)
	filename := resolveBatchFilename(asset, assetID)
	storageKey := stringPtrValue(asset.StorageKey)
	if strings.TrimSpace(storageKey) == "" {
		f := req.failure("missing_storage_key", "资产缺少存储路径")
		return ExcelPackageItem{}, &f
	}
	fileSize := int64(0)
	if asset.FileSize != nil {
		fileSize = *asset.FileSize
	}
	signed := s.presigner.PresignDownloadURL(storageKey)
	if filenamePresigner, ok := s.presigner.(DownloadFilenamePresigner); ok {
		signed = filenamePresigner.PresignDownloadURLWithFilename(storageKey, filename)
	}
	if signed == nil || strings.TrimSpace(signed.DownloadURL) == "" {
		f := req.failure("download_url_unavailable", "下载地址生成失败")
		return ExcelPackageItem{}, &f
	}
	expiresAt := signed.ExpiresAt
	mimeType := stringPtrValue(asset.MimeType)
	return ExcelPackageItem{
		RowNumber:   req.RowNumber,
		OrderNo:     req.OrderNo,
		SKUCode:     req.SKUCode,
		SKUName:     req.SKUName,
		Quantity:    req.Quantity,
		AssetID:     assetID,
		ResourceID:  strconv.FormatInt(assetID, 10),
		SourceType:  string(domain.AssetResourceSourceSystem),
		TaskID:      asset.TaskID,
		TaskNo:      row.Task.TaskNo,
		Filename:    sanitizeBatchFilename(filename),
		FileSize:    fileSize,
		MimeType:    mimeType,
		DownloadURL: strings.TrimSpace(signed.DownloadURL),
		Address:     req.Address,
		ExpiresAt:   &expiresAt,
	}, nil
}

func (s *Service) buildExternalExcelPackageItem(ctx context.Context, asset *AssetDetail, req ExcelPackageRow) (ExcelPackageItem, *ExcelPackageFailure) {
	if asset == nil {
		f := req.failure("asset_not_found", "未找到匹配的 JPG/PNG 资产")
		return ExcelPackageItem{}, &f
	}
	info, appErr := s.externalSvc.BatchDownloadInfo(ctx, asset.ID)
	if appErr != nil {
		f := req.failure(normalizeBatchFailureReason(appErr.Code), appErr.Message)
		return ExcelPackageItem{}, &f
	}
	if info == nil || info.DownloadURL == nil || strings.TrimSpace(*info.DownloadURL) == "" {
		reason := "download_url_unavailable"
		if info != nil && strings.TrimSpace(info.AccessHint) != "" {
			reason = strings.TrimSpace(info.AccessHint)
		}
		f := req.failure(reason, "外部素材正在准备下载，请稍后重试")
		return ExcelPackageItem{}, &f
	}
	fileSize := info.FileSize
	expiresAt := info.ExpiresAt
	return ExcelPackageItem{
		RowNumber:   req.RowNumber,
		OrderNo:     req.OrderNo,
		SKUCode:     req.SKUCode,
		SKUName:     req.SKUName,
		Quantity:    req.Quantity,
		AssetID:     asset.ID,
		ResourceID:  asset.ResourceID,
		SourceType:  string(domain.AssetResourceSourceExternal),
		TaskID:      0,
		Filename:    sanitizeBatchFilename(info.Filename),
		FileSize:    fileSize,
		MimeType:    strings.TrimSpace(info.MimeType),
		DownloadURL: strings.TrimSpace(*info.DownloadURL),
		Address:     req.Address,
		OriginPath:  asset.OriginPath,
		ExpiresAt:   expiresAt,
	}, nil
}

func isExcelPackageImage(asset *domain.TaskAsset) bool {
	if asset == nil {
		return false
	}
	mimeType := strings.ToLower(stringPtrValue(asset.MimeType))
	if mimeType == "image/jpeg" || mimeType == "image/png" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(firstNonEmptyExcelPackage(asset.FileName, stringPtrValue(asset.OriginalName))))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png"
}

func isExcelPackageExternalImage(asset *AssetDetail) bool {
	if asset == nil {
		return false
	}
	mimeType := strings.ToLower(strings.TrimSpace(asset.MimeType))
	if mimeType == "image/jpeg" || mimeType == "image/png" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(firstNonEmptyExcelPackage(asset.FileName, asset.OriginalFilename)))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png"
}

func firstNonEmptyExcelPackage(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringPtrValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return strings.TrimSpace(*ptr)
}

func ExcelPackageLimits() map[string]string {
	return map[string]string{
		"max_rows":        strconv.Itoa(MaxExcelPackageRows),
		"max_total_files": strconv.Itoa(MaxExcelPackageTotalFiles),
		"max_total_mb":    strconv.FormatInt(MaxExcelPackageTotalBytes/1024/1024, 10),
	}
}
