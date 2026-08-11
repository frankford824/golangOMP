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
	MaxBatchSearchTerms       = 200
	batchSearchPageSize       = 100
	BatchSearchStatusMatched  = "matched"
	BatchSearchStatusNotFound = "not_found"
	BatchSearchStatusError    = "error"
)

type BatchSearchRequest struct {
	Terms        []string `json:"terms"`
	FormatFilter string   `json:"format_filter,omitempty"`
	AssetKind    string   `json:"asset_kind,omitempty"`
}

type BatchSearchResponse struct {
	Results      []BatchSearchResult `json:"results"`
	MatchedCount int                 `json:"matched_count"`
	FailedCount  int                 `json:"failed_count"`
}

type BatchSearchResult struct {
	Term          string        `json:"term"`
	Status        string        `json:"status"`
	Message       string        `json:"message"`
	Candidates    int           `json:"candidates"`
	PackageFolder string        `json:"package_folder,omitempty"`
	Asset         *AssetDetail  `json:"asset,omitempty"`
	Assets        []AssetDetail `json:"assets,omitempty"`
}

type scoredBatchSearchAsset struct {
	detail    *AssetDetail
	score     int
	createdAt time.Time
}

func (s *Service) BatchSearch(ctx context.Context, req BatchSearchRequest) (*BatchSearchResponse, *domain.AppError) {
	terms := normalizeBatchSearchTerms(req.Terms)
	if len(terms) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "terms must not be empty", nil)
	}
	if len(terms) > MaxBatchSearchTerms {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "terms exceed batch search limit", map[string]interface{}{
			"limit": MaxBatchSearchTerms,
		})
	}

	formatFilter := normalizeBatchSearchFormat(req.FormatFilter)
	assetKind := normalizeBatchSearchAssetKind(req.AssetKind)
	response := &BatchSearchResponse{Results: make([]BatchSearchResult, 0, len(terms))}
	for _, term := range terms {
		result := s.batchSearchOne(ctx, term, formatFilter, assetKind)
		response.Results = append(response.Results, result)
		if result.Status == BatchSearchStatusMatched {
			response.MatchedCount++
		} else {
			response.FailedCount++
		}
	}
	return response, nil
}

func (s *Service) batchSearchOne(ctx context.Context, term, formatFilter, assetKind string) BatchSearchResult {
	rows, err := s.batchSearchRows(ctx, term, formatFilter)
	if err != nil {
		return BatchSearchResult{
			Term:       term,
			Status:     BatchSearchStatusError,
			Message:    "搜索失败",
			Candidates: 0,
		}
	}

	candidates := make([]scoredBatchSearchAsset, 0, len(rows))
	for _, row := range rows {
		if !matchesBatchSearchFormat(row, formatFilter) || !matchesBatchSearchAssetKind(row, assetKind) {
			continue
		}
		score := scoreBatchSearchAsset(row, term)
		if score <= 0 {
			continue
		}
		detail := s.buildAssetDetail(row, nil)
		if detail == nil {
			continue
		}
		candidates = append(candidates, scoredBatchSearchAsset{detail: detail, score: score, createdAt: detail.CreatedAt})
	}
	externalRows, externalErr := s.batchSearchExternalRows(ctx, term, formatFilter)
	if externalErr != nil {
		return BatchSearchResult{
			Term:       term,
			Status:     BatchSearchStatusError,
			Message:    "搜索失败",
			Candidates: 0,
		}
	}
	if matchesBatchSearchExternalAssetKind(assetKind) {
		for _, detail := range externalRows {
			if !matchesBatchSearchDetailFormat(detail, formatFilter) {
				continue
			}
			score := scoreBatchSearchDetail(detail, term)
			if score <= 0 {
				continue
			}
			candidates = append(candidates, scoredBatchSearchAsset{detail: detail, score: score, createdAt: detail.UpdatedAt})
		}
	}
	if len(candidates) == 0 {
		return BatchSearchResult{
			Term:       term,
			Status:     BatchSearchStatusNotFound,
			Message:    batchSearchNoMatchMessage(len(rows)+len(externalRows), formatFilter, assetKind),
			Candidates: 0,
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].createdAt.After(candidates[j].createdAt)
	})
	candidates = dedupeBatchSearchExternalFileCandidates(candidates)
	assets := make([]AssetDetail, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.detail == nil {
			continue
		}
		assets = append(assets, *candidate.detail)
	}
	if len(assets) == 0 {
		return BatchSearchResult{
			Term:       term,
			Status:     BatchSearchStatusNotFound,
			Message:    batchSearchNoMatchMessage(len(rows), formatFilter, assetKind),
			Candidates: 0,
		}
	}
	return BatchSearchResult{
		Term:          term,
		Status:        BatchSearchStatusMatched,
		Message:       "已匹配",
		Candidates:    len(assets),
		PackageFolder: batchSearchPackageFolder(candidates, term),
		Asset:         &assets[0],
		Assets:        assets,
	}
}

func batchSearchPackageFolder(candidates []scoredBatchSearchAsset, term string) string {
	folder := sanitizeBatchFilename(strings.TrimSpace(term))
	if folder == "" {
		return ""
	}
	systemByScope := map[string]int{}
	for _, candidate := range candidates {
		if candidate.detail == nil {
			continue
		}
		detail := candidate.detail
		if detail.SourceType == string(domain.AssetResourceSourceExternal) {
			continue
		}
		scope := firstNonEmptyExcelPackage(detail.ScopeSKUCode, detail.SKUCode, detail.PrimarySKUCode)
		key := strconv.FormatInt(detail.TaskID, 10) + "|" + strings.ToUpper(strings.TrimSpace(scope))
		systemByScope[key]++
		if productionPackageSetIntent(detail.FileName, detail.OriginalFilename, detail.ProductName) {
			return folder
		}
	}
	for _, count := range systemByScope {
		if count >= 2 {
			return folder
		}
	}
	return ""
}

func (s *Service) batchSearchRows(ctx context.Context, term, formatFilter string) ([]*repo.TaskAssetSearchRow, error) {
	var allRows []*repo.TaskAssetSearchRow
	for page := 1; ; page++ {
		rows, total, err := s.searchRepo.Search(ctx, domain.AssetSearchQuery{
			Keyword:        term,
			Source:         domain.AssetResourceSourceSystem,
			Page:           page,
			Size:           batchSearchPageSize,
			IsArchived:     domain.AssetArchiveFilterFalse,
			TaskStatus:     domain.AssetTaskStatusFilterAll,
			FormatCategory: batchSearchFormatCategory(formatFilter),
		})
		if err != nil {
			return nil, err
		}
		allRows = append(allRows, rows...)
		if len(rows) == 0 || int64(len(allRows)) >= total || len(rows) < batchSearchPageSize {
			return allRows, nil
		}
	}
}

func (s *Service) batchSearchExternalRows(ctx context.Context, term, formatFilter string) ([]*AssetDetail, error) {
	return s.batchSearchExternalRowsWithArchive(ctx, term, formatFilter, false)
}

func (s *Service) batchSearchExternalRowsWithArchive(ctx context.Context, term, formatFilter string, includeOSSArchive bool) ([]*AssetDetail, error) {
	if s == nil || s.externalSvc == nil || !s.externalSvc.Enabled() {
		return []*AssetDetail{}, nil
	}
	var allRows []*AssetDetail
	for page := 1; ; page++ {
		rows, total, appErr := s.searchExternalRows(ctx, domain.AssetSearchQuery{
			Keyword:                   term,
			Source:                    domain.AssetResourceSourceExternal,
			Page:                      page,
			Size:                      batchSearchPageSize,
			FormatCategory:            batchSearchFormatCategory(formatFilter),
			IncludeExternalOSSArchive: includeOSSArchive,
		})
		if appErr != nil {
			return nil, fmt.Errorf("%s: %s", appErr.Code, appErr.Message)
		}
		allRows = append(allRows, rows...)
		if len(rows) == 0 || int64(len(allRows)) >= total || len(rows) < batchSearchPageSize {
			return allRows, nil
		}
	}
}

func normalizeBatchSearchTerms(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	terms := make([]string, 0, len(raw))
	for _, item := range raw {
		term := strings.ToUpper(strings.TrimSpace(item))
		if term == "" {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	return terms
}

func dedupeBatchSearchExternalFileCandidates(candidates []scoredBatchSearchAsset) []scoredBatchSearchAsset {
	if len(candidates) < 2 {
		return candidates
	}
	seen := map[string]struct{}{}
	out := make([]scoredBatchSearchAsset, 0, len(candidates))
	for _, candidate := range candidates {
		key := batchSearchExternalFileFingerprint(candidate.detail)
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

func batchSearchExternalFileFingerprint(asset *AssetDetail) string {
	if asset == nil || asset.SourceType != string(domain.AssetResourceSourceExternal) {
		return ""
	}
	size := int64(0)
	if asset.FileSize != nil {
		size = *asset.FileSize
	}
	name := strings.ToLower(strings.TrimSpace(firstNonEmptyExcelPackage(asset.FileName, asset.OriginalFilename)))
	if name == "" || size <= 0 {
		return ""
	}
	return name + "|" + strconv.FormatInt(size, 10)
}

func normalizeBatchSearchFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "jpg_png", "jpg", "png", "tif", "webp", "image", "design", "pdf", "archive", "all":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "jpg_png"
	}
}

func normalizeBatchSearchAssetKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto", "all", "delivery", "reference", "source", "preview", "other":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "delivery"
	}
}

func batchSearchFormatCategory(formatFilter string) domain.AssetFormatCategoryFilter {
	switch formatFilter {
	case "jpg_png", "jpg", "png", "tif", "webp", "image":
		return domain.AssetFormatCategoryImage
	case "design":
		return domain.AssetFormatCategoryDesign
	case "pdf":
		return domain.AssetFormatCategoryPDF
	case "archive":
		return domain.AssetFormatCategoryArchive
	default:
		return domain.AssetFormatCategoryAll
	}
}

func matchesBatchSearchFormat(row *repo.TaskAssetSearchRow, formatFilter string) bool {
	if row == nil || row.Asset == nil {
		return false
	}
	asset := row.Asset
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(firstNonEmptyExcelPackage(asset.FileName, stringPtrValue(asset.OriginalName)))), ".")
	if ext == "jpeg" {
		ext = "jpg"
	}
	mimeType := strings.ToLower(stringPtrValue(asset.MimeType))
	switch formatFilter {
	case "all":
		return true
	case "jpg_png":
		return ext == "jpg" || ext == "png" || mimeType == "image/jpeg" || mimeType == "image/png"
	case "jpg":
		return ext == "jpg" || mimeType == "image/jpeg"
	case "png":
		return ext == "png" || mimeType == "image/png"
	case "tif":
		return ext == "tif" || ext == "tiff" || mimeType == "image/tif" || mimeType == "image/tiff"
	case "webp":
		return ext == "webp" || mimeType == "image/webp"
	case "image":
		return batchSearchImageExt(ext) || strings.HasPrefix(mimeType, "image/")
	case "design":
		return batchSearchDesignExt(ext)
	case "pdf":
		return ext == "pdf" || mimeType == "application/pdf"
	case "archive":
		return batchSearchArchiveExt(ext) || strings.Contains(mimeType, "zip") || strings.Contains(mimeType, "rar") || strings.Contains(mimeType, "7z")
	default:
		return true
	}
}

func matchesBatchSearchAssetKind(row *repo.TaskAssetSearchRow, assetKind string) bool {
	if row == nil || row.Asset == nil {
		return false
	}
	kind := row.Asset.AssetType.Canonical()
	switch assetKind {
	case "auto", "all":
		return true
	case "preview":
		return kind == domain.TaskAssetTypePreview || kind == domain.TaskAssetTypeDesignThumb
	case "other":
		return kind != domain.TaskAssetTypeDelivery &&
			kind != domain.TaskAssetTypeReference &&
			kind != domain.TaskAssetTypeSource &&
			kind != domain.TaskAssetTypePreview &&
			kind != domain.TaskAssetTypeDesignThumb
	default:
		return string(kind) == assetKind
	}
}

func matchesBatchSearchExternalAssetKind(assetKind string) bool {
	switch assetKind {
	case "reference", "preview", "other":
		return false
	default:
		return true
	}
}

func scoreBatchSearchAsset(row *repo.TaskAssetSearchRow, term string) int {
	if row == nil || row.Asset == nil || row.Task == nil {
		return 0
	}
	asset := row.Asset
	if asset.DeletedAt != nil || asset.CleanedAt != nil {
		return 0
	}
	normalizedTerm := strings.ToUpper(strings.TrimSpace(term))
	fileName := strings.ToUpper(strings.TrimSpace(asset.FileName + " " + stringPtrValue(asset.OriginalName)))
	scopeSKU := strings.ToUpper(stringPtrValue(asset.ScopeSKUCode))
	taskSKU := strings.ToUpper(strings.TrimSpace(row.Task.SKUCode))
	taskPrimarySKU := strings.ToUpper(strings.TrimSpace(row.Task.PrimarySKUCode))
	taskNo := strings.ToUpper(strings.TrimSpace(row.Task.TaskNo))
	taskID := strconv.FormatInt(row.Task.ID, 10)
	productName := strings.ToUpper(strings.TrimSpace(row.Task.ProductNameSnapshot))

	score := 0
	if scopeSKU == normalizedTerm {
		score += 140
	}
	if taskSKU == normalizedTerm || taskPrimarySKU == normalizedTerm {
		score += 120
	}
	if taskNo == normalizedTerm {
		score += 130
	}
	if taskID == normalizedTerm {
		score += 110
	}
	if fileName != "" && strings.Contains(fileName, normalizedTerm) {
		score += 80
	}
	if productName != "" && strings.Contains(productName, normalizedTerm) {
		score += 45
	}

	switch asset.AssetType.Canonical() {
	case domain.TaskAssetTypeDelivery:
		score += 60
	case domain.TaskAssetTypePreview:
		score += 42
	case domain.TaskAssetTypeDesignThumb:
		score += 34
	case domain.TaskAssetTypeReference:
		score += 18
	}
	if score == 0 && fileName != "" {
		score = 1
	}
	return score
}

func scoreBatchSearchDetail(asset *AssetDetail, term string) int {
	if asset == nil {
		return 0
	}
	normalizedTerm := strings.ToUpper(strings.TrimSpace(term))
	if normalizedTerm == "" {
		return 0
	}
	resourceID := strings.ToUpper(strings.TrimSpace(asset.ResourceID))
	fileName := strings.ToUpper(strings.TrimSpace(asset.FileName + " " + asset.OriginalFilename))
	originPath := strings.ToUpper(strings.TrimSpace(asset.OriginPath + " " + asset.ProductName))

	score := 0
	if resourceID == normalizedTerm {
		score += 110
	}
	if fileName != "" && strings.Contains(fileName, normalizedTerm) {
		score += 100
		if strings.HasPrefix(fileName, normalizedTerm) {
			score += 20
		}
	}
	if originPath != "" && strings.Contains(originPath, normalizedTerm) {
		score += 45
	}
	if asset.SourceType == string(domain.AssetResourceSourceExternal) {
		score += 30
	}
	if score == 0 && fileName != "" {
		score = 1
	}
	return score
}

func batchSearchNoMatchMessage(totalRows int, formatFilter, assetKind string) string {
	if totalRows == 0 {
		return "未找到匹配资产"
	}
	return "找到了资产，但没有符合当前格式或资源类型筛选的可下载资源"
}

func matchesBatchSearchDetailFormat(asset *AssetDetail, formatFilter string) bool {
	if asset == nil {
		return false
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(firstNonEmptyExcelPackage(asset.FileName, asset.OriginalFilename))), ".")
	if ext == "jpeg" {
		ext = "jpg"
	}
	mimeType := strings.ToLower(strings.TrimSpace(asset.MimeType))
	switch formatFilter {
	case "all":
		return true
	case "jpg_png":
		return ext == "jpg" || ext == "png" || mimeType == "image/jpeg" || mimeType == "image/png"
	case "jpg":
		return ext == "jpg" || mimeType == "image/jpeg"
	case "png":
		return ext == "png" || mimeType == "image/png"
	case "webp":
		return ext == "webp" || mimeType == "image/webp"
	case "image":
		return batchSearchImageExt(ext) || strings.HasPrefix(mimeType, "image/")
	case "design":
		return batchSearchDesignExt(ext)
	case "pdf":
		return ext == "pdf" || mimeType == "application/pdf"
	case "archive":
		return batchSearchArchiveExt(ext) || strings.Contains(mimeType, "zip") || strings.Contains(mimeType, "rar") || strings.Contains(mimeType, "7z")
	default:
		return true
	}
}

func batchSearchImageExt(ext string) bool {
	switch ext {
	case "jpg", "jpeg", "png", "webp", "gif", "bmp", "svg", "tif", "tiff":
		return true
	default:
		return false
	}
}

func batchSearchDesignExt(ext string) bool {
	switch ext {
	case "psd", "psb", "ai", "cdr", "eps", "sketch", "fig", "xd":
		return true
	default:
		return false
	}
}

func batchSearchArchiveExt(ext string) bool {
	switch ext {
	case "zip", "rar", "7z", "tar", "gz":
		return true
	default:
		return false
	}
}
