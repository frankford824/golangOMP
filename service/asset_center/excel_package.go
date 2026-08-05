package asset_center

import (
	"context"
	"fmt"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
	RowNumber     int        `json:"row_number,omitempty"`
	OrderNo       string     `json:"order_no"`
	SKUCode       string     `json:"sku_code"`
	SKUName       string     `json:"sku_name,omitempty"`
	Quantity      int        `json:"quantity"`
	AssetID       int64      `json:"asset_id"`
	ResourceID    string     `json:"resource_id,omitempty"`
	SourceType    string     `json:"source_type,omitempty"`
	TaskID        int64      `json:"task_id"`
	TaskNo        string     `json:"task_no,omitempty"`
	Filename      string     `json:"filename"`
	FileSize      int64      `json:"file_size"`
	MimeType      string     `json:"mime_type,omitempty"`
	DownloadURL   string     `json:"download_url"`
	Address       string     `json:"address,omitempty"`
	OriginPath    string     `json:"origin_path,omitempty"`
	PackageFolder string     `json:"package_folder,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
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
	system        *repo.TaskAssetSearchRow
	external      *AssetDetail
	score         int
	ready         bool
	updated       time.Time
	packageFolder string
}

type preparedExcelPackageMatch struct {
	items          []ExcelPackageItem
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
	successRows := 0
	matchCache := map[string][]scoredExcelAsset{}
	preparedCache := map[string]preparedExcelPackageMatch{}

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
		cacheKey := excelPackageMatchCacheKey(row)
		matches, ok := matchCache[cacheKey]
		if !ok {
			var appErr *domain.AppError
			matches, appErr = s.matchExcelPackageAssets(ctx, row)
			if appErr != nil {
				return nil, appErr
			}
			matchCache[cacheKey] = matches
		}
		if len(matches) == 0 {
			manifest.Failures = append(manifest.Failures, row.failure("asset_not_found", "未找到匹配的生产图片（JPG/PNG/TIF/TIFF）"))
			continue
		}

		prepared, ok := preparedCache[cacheKey]
		if !ok {
			prepared.items = make([]ExcelPackageItem, 0, len(matches))
			for _, match := range matches {
				item, failure := s.buildExcelPackageItem(ctx, match, row)
				if failure != nil {
					if prepared.failureReason == "" {
						prepared.failureReason = failure.Reason
						prepared.failureMessage = failure.Message
					}
					continue
				}
				prepared.items = append(prepared.items, item)
			}
			if prepared.failureReason != "" {
				prepared.items = nil
			}
			preparedCache[cacheKey] = prepared
		}
		if prepared.failureReason != "" {
			manifest.Failures = append(manifest.Failures, row.failure(prepared.failureReason, prepared.failureMessage))
			continue
		}
		if row.Quantity > MaxExcelPackageTotalFiles || len(prepared.items) > MaxExcelPackageTotalFiles/row.Quantity {
			manifest.Failures = append(manifest.Failures, row.failure("total_file_limit_exceeded", fmt.Sprintf("总文件数超过 %d 个", MaxExcelPackageTotalFiles)))
			continue
		}
		rowFileCount := len(prepared.items) * row.Quantity
		if totalFiles+rowFileCount > MaxExcelPackageTotalFiles {
			manifest.Failures = append(manifest.Failures, row.failure("total_file_limit_exceeded", fmt.Sprintf("总文件数超过 %d 个", MaxExcelPackageTotalFiles)))
			continue
		}
		items := make([]ExcelPackageItem, 0, len(prepared.items))
		rowBytes := int64(0)
		for _, preparedItem := range prepared.items {
			item := applyExcelPackageRow(preparedItem, row)
			if item.FileSize > 0 && item.FileSize > MaxExcelPackageTotalBytes/int64(row.Quantity) {
				rowBytes = MaxExcelPackageTotalBytes + 1
				break
			}
			rowBytes += item.FileSize * int64(row.Quantity)
			items = append(items, item)
		}
		if rowBytes > MaxExcelPackageTotalBytes-totalBytes {
			manifest.Failures = append(manifest.Failures, row.failure("total_size_limit_exceeded", fmt.Sprintf("总大小超过 %d MB", MaxExcelPackageTotalBytes/1024/1024)))
			continue
		}
		totalBytes += rowBytes
		totalFiles += rowFileCount
		successRows++
		for _, item := range items {
			if manifest.ExpiresAt == nil || (item.ExpiresAt != nil && item.ExpiresAt.Before(*manifest.ExpiresAt)) {
				manifest.ExpiresAt = item.ExpiresAt
			}
			manifest.Items = append(manifest.Items, item)
		}
	}

	manifest.SuccessCount = successRows
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

func (s *Service) matchExcelPackageAssets(ctx context.Context, row ExcelPackageRow) ([]scoredExcelAsset, *domain.AppError) {
	keyword := firstExcelPackageKeyword(row.SKUCode, row.SKUName)
	resultRows, _, err := s.searchRepo.Search(ctx, domain.AssetSearchQuery{
		Keyword:    keyword,
		Page:       1,
		Size:       100,
		IsArchived: domain.AssetArchiveFilterFalse,
		TaskStatus: domain.AssetTaskStatusFilterAll,
	})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
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
	externalRows, externalErr := s.batchSearchExternalRows(ctx, keyword, "image")
	if externalErr != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, externalErr.Error(), nil)
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
		return nil, nil
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
	if setCandidates := excelPackageExternalSetCandidates(candidates, row.SKUCode); len(setCandidates) >= 2 {
		return setCandidates, nil
	}
	if setCandidates := excelPackageSystemSetCandidates(candidates, row.SKUCode); len(setCandidates) > 0 {
		return setCandidates, nil
	}
	return []scoredExcelAsset{candidates[0]}, nil
}

func excelPackageSystemSetCandidates(candidates []scoredExcelAsset, skuCode string) []scoredExcelAsset {
	folder := sanitizeBatchFilename(strings.TrimSpace(skuCode))
	if folder == "" {
		return nil
	}
	for index, candidate := range candidates {
		if candidate.system == nil || candidate.system.Asset == nil || candidate.system.Task == nil || !candidate.ready {
			continue
		}
		asset := candidate.system.Asset
		scope := strings.ToUpper(strings.TrimSpace(firstNonEmptyExcelPackage(
			stringPtrValue(asset.ScopeSKUCode),
			candidate.system.Task.SKUCode,
			candidate.system.Task.PrimarySKUCode,
		)))
		group := make([]scoredExcelAsset, 0, 4)
		for _, sibling := range candidates {
			if sibling.system == nil || sibling.system.Asset == nil || sibling.system.Task == nil || !sibling.ready {
				continue
			}
			siblingAsset := sibling.system.Asset
			siblingScope := strings.ToUpper(strings.TrimSpace(firstNonEmptyExcelPackage(
				stringPtrValue(siblingAsset.ScopeSKUCode),
				sibling.system.Task.SKUCode,
				sibling.system.Task.PrimarySKUCode,
			)))
			if sibling.system.Task.ID == candidate.system.Task.ID && siblingScope == scope {
				sibling.packageFolder = folder
				group = append(group, sibling)
			}
		}
		if len(group) >= 2 {
			sort.SliceStable(group, func(i, j int) bool {
				leftOrder := excelPackageComponentOrder(group[i].system.Asset.FileName)
				rightOrder := excelPackageComponentOrder(group[j].system.Asset.FileName)
				if leftOrder != rightOrder {
					return leftOrder < rightOrder
				}
				return strings.ToLower(group[i].system.Asset.FileName) < strings.ToLower(group[j].system.Asset.FileName)
			})
			return group
		}
		if index == 0 && productionPackageSetIntent(
			asset.FileName,
			stringPtrValue(asset.OriginalName),
			candidate.system.Task.ProductNameSnapshot,
		) {
			candidate.packageFolder = folder
			return []scoredExcelAsset{candidate}
		}
	}
	return nil
}

var productionPackageSetPattern = regexp.MustCompile(`(?i)(?:[2-9]|[1-9][0-9]+)\s*(?:个装|件套)|套装`)

func productionPackageSetIntent(values ...string) bool {
	for _, value := range values {
		if productionPackageSetPattern.MatchString(strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

func excelPackageExternalSetCandidates(candidates []scoredExcelAsset, skuCode string) []scoredExcelAsset {
	skuCode = strings.ToUpper(strings.TrimSpace(skuCode))
	if skuCode == "" {
		return nil
	}
	for _, candidate := range candidates {
		if candidate.external == nil {
			continue
		}
		parentPath, ok := excelPackageSetParent(candidate.external.OriginPath, skuCode)
		if !ok {
			continue
		}
		group := make([]scoredExcelAsset, 0, 4)
		for _, sibling := range candidates {
			if sibling.external == nil {
				continue
			}
			if cleanExcelPackageOriginPath(pathpkg.Dir(sibling.external.OriginPath)) == parentPath {
				group = append(group, sibling)
			}
		}
		if len(group) < 2 {
			continue
		}
		packageFolder := strings.TrimSpace(pathpkg.Base(parentPath))
		for index := range group {
			group[index].packageFolder = packageFolder
		}
		sort.SliceStable(group, func(i, j int) bool {
			leftOrder := excelPackageComponentOrder(group[i].external.FileName)
			rightOrder := excelPackageComponentOrder(group[j].external.FileName)
			if leftOrder != rightOrder {
				return leftOrder < rightOrder
			}
			return strings.ToLower(group[i].external.FileName) < strings.ToLower(group[j].external.FileName)
		})
		return group
	}
	return nil
}

func excelPackageSetParent(originPath, skuCode string) (string, bool) {
	parentPath := cleanExcelPackageOriginPath(pathpkg.Dir(originPath))
	if !strings.Contains(parentPath, "/仓库素材区/徐凯/") {
		return "", false
	}
	folderName := strings.ToUpper(strings.TrimSpace(pathpkg.Base(parentPath)))
	if !strings.HasPrefix(folderName, skuCode) {
		return "", false
	}
	remainder := strings.TrimPrefix(folderName, skuCode)
	if remainder == "" {
		return parentPath, true
	}
	first, _ := utf8.DecodeRuneInString(remainder)
	if unicode.IsLetter(first) || unicode.IsDigit(first) {
		return "", false
	}
	return parentPath, true
}

func cleanExcelPackageOriginPath(value string) string {
	return pathpkg.Clean("/" + strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/"))
}

func excelPackageComponentOrder(filename string) int {
	for index, marker := range []string{"第一张", "第二张", "第三张", "第四张", "第五张", "第六张", "第七张", "第八张", "第九张", "第十张"} {
		if strings.Contains(filename, marker) {
			return index + 1
		}
	}
	if match := productionPackageComponentSuffixPattern.FindStringSubmatch(strings.TrimSpace(filename)); len(match) == 2 {
		if value, err := strconv.Atoi(match[1]); err == nil && value > 0 {
			return value
		}
	}
	return 1
}

var productionPackageComponentSuffixPattern = regexp.MustCompile(`(?i)-([1-9][0-9]*)(?:\.[^.]+)?$`)

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
			originPath := cleanExcelPackageOriginPath(candidate.external.OriginPath)
			if originPath != "/" {
				key = "path:" + strings.ToLower(originPath)
			} else {
				key = batchSearchExternalFileFingerprint(candidate.external)
			}
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
	var item ExcelPackageItem
	var failure *ExcelPackageFailure
	if match.external != nil {
		item, failure = s.buildExternalExcelPackageItem(ctx, match.external, req)
	} else {
		item, failure = s.buildSystemExcelPackageItem(match.system, req)
	}
	if failure == nil {
		item.PackageFolder = match.packageFolder
	}
	return item, failure
}

func (s *Service) buildSystemExcelPackageItem(row *repo.TaskAssetSearchRow, req ExcelPackageRow) (ExcelPackageItem, *ExcelPackageFailure) {
	if row == nil || row.Asset == nil || row.Task == nil {
		f := req.failure("asset_not_found", "未找到匹配的生产图片（JPG/PNG/TIF/TIFF）")
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
		f := req.failure("asset_not_found", "未找到匹配的生产图片（JPG/PNG/TIF/TIFF）")
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
	if mimeType == "image/jpeg" || mimeType == "image/png" || mimeType == "image/tiff" || mimeType == "image/tif" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(firstNonEmptyExcelPackage(asset.FileName, stringPtrValue(asset.OriginalName))))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".tif" || ext == ".tiff"
}

func isExcelPackageExternalImage(asset *AssetDetail) bool {
	if asset == nil {
		return false
	}
	mimeType := strings.ToLower(strings.TrimSpace(asset.MimeType))
	if mimeType == "image/jpeg" || mimeType == "image/png" || mimeType == "image/tiff" || mimeType == "image/tif" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(firstNonEmptyExcelPackage(asset.FileName, asset.OriginalFilename)))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".tif" || ext == ".tiff"
}

func firstNonEmptyExcelPackage(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func ExcelPackageLimits() map[string]string {
	return map[string]string{
		"max_rows":        strconv.Itoa(MaxExcelPackageRows),
		"max_total_files": strconv.Itoa(MaxExcelPackageTotalFiles),
		"max_total_mb":    strconv.FormatInt(MaxExcelPackageTotalBytes/1024/1024, 10),
	}
}
