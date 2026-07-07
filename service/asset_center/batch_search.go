package asset_center

import (
	"context"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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
	Term       string        `json:"term"`
	Status     string        `json:"status"`
	Message    string        `json:"message"`
	Candidates int           `json:"candidates"`
	Asset      *AssetDetail  `json:"asset,omitempty"`
	Assets     []AssetDetail `json:"assets,omitempty"`
}

type scoredBatchSearchAsset struct {
	row   *repo.TaskAssetSearchRow
	score int
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
		candidates = append(candidates, scoredBatchSearchAsset{row: row, score: score})
	}
	if len(candidates) == 0 {
		return BatchSearchResult{
			Term:       term,
			Status:     BatchSearchStatusNotFound,
			Message:    batchSearchNoMatchMessage(len(rows), formatFilter, assetKind),
			Candidates: 0,
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		ai := candidates[i].row.Asset
		aj := candidates[j].row.Asset
		if ai == nil || aj == nil {
			return ai != nil
		}
		return ai.CreatedAt.After(aj.CreatedAt)
	})
	assets := make([]AssetDetail, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.row == nil {
			continue
		}
		detail := buildAssetDetail(candidate.row, nil)
		if detail == nil {
			continue
		}
		assets = append(assets, *detail)
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
		Term:       term,
		Status:     BatchSearchStatusMatched,
		Message:    "已匹配",
		Candidates: len(assets),
		Asset:      &assets[0],
		Assets:     assets,
	}
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

func normalizeBatchSearchFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "jpg_png", "jpg", "png", "webp", "image", "design", "pdf", "archive", "all":
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
	case "jpg_png", "jpg", "png", "webp", "image":
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

func batchSearchNoMatchMessage(totalRows int, formatFilter, assetKind string) string {
	if totalRows == 0 {
		return "未找到匹配资产"
	}
	return "找到了资产，但没有符合当前格式或资源类型筛选的可下载资源"
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
