package asset_center

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"

	"workflow/domain"
	"workflow/repo"
)

type finalizedPackageGroup struct {
	ID          int64
	RevisionID  int64
	Mode        domain.TaskAssetGroupMode
	FinalizedAt int64
	Items       []repo.ProductionPackageAsset
}

func (s *Service) matchFinalizedPackageRows(ctx context.Context, rawRows []ExcelPackageRow, formatFilter string) (map[string][]scoredExcelAsset, *domain.AppError) {
	out := map[string][]scoredExcelAsset{}
	if s == nil || s.productionRepo == nil {
		return out, nil
	}
	codes := make([]string, 0, len(rawRows))
	names := make([]string, 0, len(rawRows))
	rowsByKey := map[string]ExcelPackageRow{}
	for index, raw := range rawRows {
		row := normalizeExcelPackageRow(raw, index+2)
		key := excelPackageMatchCacheKey(row) + "\x00" + formatFilter
		if _, exists := rowsByKey[key]; exists {
			continue
		}
		rowsByKey[key] = row
		if row.SKUCode != "" {
			codes = append(codes, row.SKUCode)
		} else if row.SKUName != "" {
			names = append(names, row.SKUName)
		}
	}
	assets, err := s.productionRepo.ListFinalizedAssets(ctx, repo.ProductionPackageQuery{SKUCodes: codes, SKUNames: names})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	groupsByID := map[int64]*finalizedPackageGroup{}
	for _, asset := range assets {
		group := groupsByID[asset.GroupID]
		if group == nil {
			group = &finalizedPackageGroup{
				ID: asset.GroupID, RevisionID: asset.RevisionID, Mode: asset.RevisionMode,
				FinalizedAt: asset.RevisionFinalizedAt.UnixNano(),
			}
			groupsByID[asset.GroupID] = group
		}
		group.Items = append(group.Items, asset)
	}
	groups := make([]*finalizedPackageGroup, 0, len(groupsByID))
	for _, group := range groupsByID {
		sort.SliceStable(group.Items, func(i, j int) bool {
			if group.Items[i].SortOrder != group.Items[j].SortOrder {
				return group.Items[i].SortOrder < group.Items[j].SortOrder
			}
			return group.Items[i].RevisionItemID < group.Items[j].RevisionItemID
		})
		groups = append(groups, group)
	}

	for key, row := range rowsByKey {
		var selected *finalizedPackageGroup
		selectedScore := 0
		for _, group := range groups {
			score := scoreFinalizedPackageGroup(group, row)
			if score <= 0 {
				continue
			}
			if selected == nil || score > selectedScore ||
				(score == selectedScore && (group.FinalizedAt > selected.FinalizedAt ||
					(group.FinalizedAt == selected.FinalizedAt && group.ID > selected.ID))) {
				selected = group
				selectedScore = score
			}
		}
		if selected == nil {
			continue
		}
		items := finalizedPackageRendition(selected.Items, formatFilter)
		if len(items) == 0 {
			continue
		}
		folder := ""
		if selected.Mode == domain.TaskAssetGroupModeSet || len(items) > 1 || productionPackageSetIntent(row.SKUName, items[0].SKUName) {
			folder = sanitizeProductionPackageBusinessName(firstNonEmptyExcelPackage(row.SKUCode, items[0].SKUCode), firstNonEmptyExcelPackage(items[0].SKUName, row.SKUName))
		}
		matches := make([]scoredExcelAsset, 0, len(items))
		for index := range items {
			matches = append(matches, scoredExcelAsset{
				finalized: &items[index], score: selectedScore, ready: true,
				updated: items[index].RevisionFinalizedAt, packageFolder: folder,
			})
		}
		out[key] = matches
	}
	return out, nil
}

func scoreFinalizedPackageGroup(group *finalizedPackageGroup, row ExcelPackageRow) int {
	if group == nil || len(group.Items) == 0 {
		return 0
	}
	wantedCode := strings.ToUpper(strings.TrimSpace(row.SKUCode))
	wantedName := strings.ToLower(strings.TrimSpace(row.SKUName))
	keyword := strings.ToLower(strings.TrimSpace(row.Keyword))
	score := 100
	matchedCode := false
	matchedName := false
	matchedKeyword := keyword == ""
	for _, item := range group.Items {
		if wantedCode != "" && (strings.EqualFold(strings.TrimSpace(item.SKUCode), wantedCode) || finalizedPackageFilenameContainsSKU(item, wantedCode)) {
			matchedCode = true
		}
		name := strings.ToLower(strings.TrimSpace(item.SKUName))
		if wantedName != "" && (name == wantedName || strings.Contains(name, wantedName) || strings.Contains(wantedName, name)) {
			matchedName = true
		}
		if keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{item.SKUName, item.FileName, item.OriginalFilename, item.ItemName}, " "))
			if strings.Contains(haystack, keyword) {
				matchedKeyword = true
			}
		}
	}
	if wantedCode != "" && !matchedCode {
		return 0
	}
	if wantedCode == "" && wantedName != "" && !matchedName {
		return 0
	}
	if !matchedKeyword {
		return 0
	}
	if matchedCode {
		score += 1000
	}
	if matchedName {
		score += 100
	}
	if keyword != "" {
		score += 50
	}
	return score
}

func finalizedPackageFilenameContainsSKU(item repo.ProductionPackageAsset, skuCode string) bool {
	skuCode = strings.ToUpper(strings.TrimSpace(skuCode))
	if skuCode == "" {
		return false
	}
	value := strings.ToUpper(strings.Join([]string{item.FileName, item.OriginalFilename, item.ItemName}, " "))
	return regexp.MustCompile(`(^|[^A-Z0-9])` + regexp.QuoteMeta(skuCode) + `([^A-Z0-9]|$)`).MatchString(value)
}

func finalizedPackageRendition(items []repo.ProductionPackageAsset, formatFilter string) []repo.ProductionPackageAsset {
	eligible := make([]repo.ProductionPackageAsset, 0, len(items))
	for _, item := range items {
		ext := normalizeExcelPackageExtension(firstNonEmptyExcelPackage(item.OriginalFilename, item.FileName, item.ItemName), item.MimeType)
		if !excelPackageExtensionMatches(ext, formatFilter) || hasProductionPackageExcludedMarker(item.FileName, item.OriginalFilename, item.ItemName) {
			continue
		}
		eligible = append(eligible, item)
	}
	if len(eligible) < 2 {
		return eligible
	}
	normalized := normalizeExcelPackageFormat(formatFilter)
	if normalized == "tif" || normalized == "jpg" || normalized == "png" {
		return eligible
	}
	selectedExt := normalizeExcelPackageExtension(firstNonEmptyExcelPackage(eligible[0].OriginalFilename, eligible[0].FileName, eligible[0].ItemName), eligible[0].MimeType)
	out := make([]repo.ProductionPackageAsset, 0, len(eligible))
	for _, item := range eligible {
		if normalizeExcelPackageExtension(firstNonEmptyExcelPackage(item.OriginalFilename, item.FileName, item.ItemName), item.MimeType) == selectedExt {
			out = append(out, item)
		}
	}
	return out
}

func excelPackageExtensionMatches(ext, formatFilter string) bool {
	switch normalizeExcelPackageFormat(formatFilter) {
	case "tif":
		return ext == "tif"
	case "jpg":
		return ext == "jpg"
	case "png":
		return ext == "png"
	case "jpg_png":
		return ext == "jpg" || ext == "png"
	default:
		return ext == "jpg" || ext == "png" || ext == "tif"
	}
}

func hasProductionPackageExcludedMarker(values ...string) bool {
	value := strings.ToLower(strings.Join(values, " "))
	for _, marker := range []string{"参考图", "效果图", "预览图", "设计源文件", "源文件", "reference", "preview", "mockup"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func (s *Service) matchExternalPackageRows(ctx context.Context, rawRows []ExcelPackageRow, formatFilter string, finalized map[string][]scoredExcelAsset) (map[string][]scoredExcelAsset, *domain.AppError) {
	out := map[string][]scoredExcelAsset{}
	if s == nil || s.productionRepo == nil {
		return out, nil
	}
	rowsByKey := map[string]ExcelPackageRow{}
	for index, raw := range rawRows {
		row := normalizeExcelPackageRow(raw, index+2)
		key := excelPackageMatchCacheKey(row) + "\x00" + formatFilter
		if len(finalized[key]) > 0 {
			continue
		}
		if _, exists := rowsByKey[key]; !exists {
			rowsByKey[key] = row
		}
	}
	if len(rowsByKey) == 0 {
		return out, nil
	}

	var mu sync.Mutex
	var firstErr *domain.AppError
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for key, row := range rowsByKey {
		key, row := key, row
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			matches, appErr := s.matchExcelPackageExternalAssets(ctx, row, formatFilter)
			mu.Lock()
			defer mu.Unlock()
			if appErr != nil && firstErr == nil {
				firstErr = appErr
				return
			}
			out[key] = matches
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return out, nil
}

func (s *Service) matchExcelPackageExternalAssets(ctx context.Context, row ExcelPackageRow, formatFilter string) ([]scoredExcelAsset, *domain.AppError) {
	keyword := firstExcelPackageKeyword(row.SKUCode, row.SKUName)
	externalRows, externalErr := s.batchSearchExternalRowsWithArchive(ctx, keyword, formatFilter, true)
	if externalErr != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, externalErr.Error(), nil)
	}
	externalCandidates := make([]scoredExcelAsset, 0, len(externalRows))
	for _, candidate := range externalRows {
		score := scoreExcelPackageExternalAsset(candidate, row, formatFilter)
		if score <= 0 {
			continue
		}
		ready := strings.EqualFold(strings.TrimSpace(candidate.OSSSyncStatus), string(domain.ExternalAssetOSSStatusReady))
		externalCandidates = append(externalCandidates, scoredExcelAsset{external: candidate, score: score, ready: ready, updated: candidate.UpdatedAt})
	}
	if len(externalCandidates) == 0 {
		return nil, nil
	}
	sortExcelPackageCandidates(externalCandidates)
	externalCandidates = dedupeExcelPackageExternalFileCandidates(externalCandidates)
	externalCandidates = chooseExcelPackageRendition(externalCandidates, formatFilter)
	if setCandidates := excelPackageExternalSetCandidates(externalCandidates, row); len(setCandidates) >= 2 {
		return setCandidates, nil
	}
	return []scoredExcelAsset{externalCandidates[0]}, nil
}
