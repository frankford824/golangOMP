package task_batch_excel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
	"workflow/service"
)

const (
	maxEmbeddedReferenceImagesPerRow = 5
	maxEmbeddedReferenceImageBytes   = 20 * 1024 * 1024
	defaultExcelRowHeightPoints      = 15
	defaultExcelRowHeightPixels      = 20
)

type parseService struct {
	referenceUploader ReferenceUploader
	iidLookup         ERPIIDLookup
}

var batchFieldPathRE = regexp.MustCompile(`^batch_items\[(\d+)\](?:\.(.+))?$`)

func (s *parseService) Parse(ctx context.Context, taskType domain.TaskType, file io.Reader, opts ...ParseOption) (*BatchParseResult, *domain.AppError) {
	fields, ok := FieldsForTaskType(taskType)
	if !ok {
		return nil, unsupportedTaskTypeError(taskType)
	}
	options := ParseOptions{
		ReferenceUploader: s.referenceUploader,
		IIDLookup:         s.iidLookup,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid Excel file", nil)
	}
	defer f.Close()

	dataSheet, rows, appErr := resolveDataSheet(f)
	if appErr != nil {
		return nil, appErr
	}
	columnIndex, appErr := parseHeader(rows[0], fields)
	if appErr != nil {
		return nil, appErr
	}
	imagesByRow, appErr := extractEmbeddedReferenceImages(f, dataSheet)
	if appErr != nil {
		return nil, appErr
	}

	maxDataRows := len(rows) - 1
	for row := range imagesByRow {
		if row > maxDataRows+1 {
			maxDataRows = row - 1
		}
	}
	items := make([]service.CreateTaskBatchSKUItemParams, 0, maxDataRows)
	preview := make([]BatchItem, 0, maxDataRows)
	itemRows := make([]int, 0, maxDataRows)
	for rowIdx := 1; rowIdx <= maxDataRows; rowIdx++ {
		var row []string
		if rowIdx < len(rows) {
			row = rows[rowIdx]
		}
		rowNumber := rowIdx + 1
		if rowIsEmpty(row) && len(imagesByRow[rowNumber]) == 0 {
			continue
		}
		item, parseViolations := parseItemRow(row, fields, columnIndex)
		items = append(items, item)
		previewItem := batchItemFromService(item)
		previewItem.SourceRow = rowNumber
		preview = append(preview, previewItem)
		itemRows = append(itemRows, rowNumber)
		if len(parseViolations) > 0 {
			parseViolations = parseViolationsForRow(rowIdx+1, parseViolations)
			return &BatchParseResult{TaskType: taskType, Preview: preview, Violations: parseViolations}, nil
		}
	}

	if iidViolations, appErr := s.validateProductIIDs(ctx, items, itemRows, options.IIDLookup); appErr != nil {
		return nil, appErr
	} else if len(iidViolations) > 0 {
		return &BatchParseResult{TaskType: taskType, Preview: preview, Violations: iidViolations}, nil
	}

	params := service.CreateTaskParams{
		TaskType:     taskType,
		SourceMode:   domain.TaskSourceModeNewProduct,
		BatchSKUMode: "multiple",
		BatchItems:   items,
	}
	if appErr := service.ValidateBatchTaskCreateRequest(params); appErr != nil {
		return &BatchParseResult{
			TaskType:   taskType,
			Preview:    preview,
			Violations: mapValidationViolations(appErr, taskType, fields, items, itemRows, imagesByRow),
		}, nil
	}
	if appErr := s.uploadEmbeddedReferenceImages(ctx, imagesByRow, options, items, preview, itemRows); appErr != nil {
		return nil, appErr
	}
	return &BatchParseResult{TaskType: taskType, Preview: preview, Violations: []ParseViolation{}}, nil
}

func parseHeader(header []string, fields []FieldSpec) (map[string]int, *domain.AppError) {
	index := make(map[string]int, len(fields))
	byColumn := make(map[string]FieldSpec, len(fields))
	for _, field := range fields {
		byColumn[strings.TrimSpace(field.Column)] = field
	}
	legacyProductIIDColumn := "商品编码"
	productIIDColumn := ""
	for _, field := range fields {
		if field.Key == "product_i_id" {
			productIIDColumn = strings.TrimSpace(field.Column)
			break
		}
	}
	for i, raw := range header {
		column := strings.TrimSpace(raw)
		if field, ok := byColumn[column]; ok {
			index[field.Key] = i
			continue
		}
		if column == legacyProductIIDColumn && productIIDColumn != "" {
			if _, exists := index["product_i_id"]; !exists {
				index["product_i_id"] = i
			}
			index["product_i_id__legacy"] = i
		}
	}
	if productIIDColumn != "" {
		if idx, ok := index["product_i_id"]; ok {
			legacyIdx, hasLegacy := index["product_i_id__legacy"]
			if hasLegacy && legacyIdx != idx {
				index["product_i_id__primary"] = idx
			}
		}
	}
	for _, field := range fields {
		if field.Required {
			if _, ok := index[field.Key]; !ok {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, errMsgExcelHeaderMismatch, map[string]interface{}{
					"violations": []ParseViolation{{
						Row:     1,
						Column:  field.Column,
						Code:    "invalid_header",
						Message: errMsgExcelHeaderMismatch,
					}},
				})
			}
		}
	}
	return index, nil
}

func parseItemRow(row []string, fields []FieldSpec, columnIndex map[string]int) (service.CreateTaskBatchSKUItemParams, []ParseViolation) {
	var item service.CreateTaskBatchSKUItemParams
	var violations []ParseViolation
	var productIIDField FieldSpec
	var hasProductIIDField bool
	for _, field := range fields {
		if field.Key == "product_i_id" {
			productIIDField = field
			hasProductIIDField = true
			continue
		}
		idx, ok := columnIndex[field.Key]
		if !ok || idx >= len(row) {
			continue
		}
		value := strings.TrimSpace(row[idx])
		if value == "" {
			continue
		}
		switch field.Key {
		case "product_name":
			item.ProductName = value
		case "product_short_name":
			item.ProductShortName = value
		case "category_code":
			item.CategoryCode = value
		case "reference_image":
			// Images pasted into the workbook are extracted from worksheet drawings
			// by row anchor. Text in this column is only an operator hint.
		case "material_mode":
			item.MaterialMode = value
		case "design_requirement":
			item.DesignRequirement = value
		case "new_sku":
			item.NewSKU = value
		case "purchase_sku":
			item.PurchaseSKU = value
		case "cost_price_mode":
			item.CostPriceMode = value
		case "quantity":
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				violations = append(violations, ParseViolation{Column: field.Column, Code: "missing_required_field", Message: "batch_items[].quantity is required and must be greater than 0"})
				continue
			}
			item.Quantity = &parsed
		case "base_sale_price":
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil {
				violations = append(violations, ParseViolation{Column: field.Column, Code: "missing_required_field", Message: "batch_items[].base_sale_price is required"})
				continue
			}
			item.BaseSalePrice = &parsed
		case "variant_json":
			if !json.Valid([]byte(value)) {
				violations = append(violations, ParseViolation{Column: field.Column, Code: "invalid_variant_json", Message: "batch_items[].variant_json must be valid JSON"})
				continue
			}
			item.VariantJSON = json.RawMessage(value)
		}
	}
	if hasProductIIDField {
		primaryIdx, hasPrimary := columnIndex["product_i_id"]
		legacyIdx, hasLegacy := columnIndex["product_i_id__legacy"]
		dualPrimaryIdx, hasDualPrimary := columnIndex["product_i_id__primary"]
		if hasDualPrimary {
			primaryIdx = dualPrimaryIdx
			hasPrimary = true
		}
		primaryValue := ""
		legacyValue := ""
		if hasPrimary && primaryIdx < len(row) {
			primaryValue = strings.TrimSpace(row[primaryIdx])
		}
		if hasLegacy && legacyIdx < len(row) {
			legacyValue = strings.TrimSpace(row[legacyIdx])
		}
		if hasPrimary && hasLegacy && primaryIdx != legacyIdx && primaryValue != "" && legacyValue != "" && primaryValue != legacyValue {
			violations = append(violations, ParseViolation{
				Column:  productIIDField.Column,
				Code:    "conflicting_product_i_id_columns",
				Message: "产品款式编码与商品编码列值不一致，请保持一致后重试",
			})
		} else if primaryValue != "" {
			item.ProductIID = primaryValue
		} else if legacyValue != "" {
			item.ProductIID = legacyValue
		}
	}
	return item, violations
}

type embeddedReferenceImage struct {
	Cell      string
	Row       int
	Index     int
	Extension string
	File      []byte
	MimeType  string
}

func extractEmbeddedReferenceImages(f *excelize.File, dataSheet string) (map[int][]embeddedReferenceImage, *domain.AppError) {
	cells, err := f.GetPictureCells(dataSheet)
	if err != nil {
		return nil, excelAppError("read embedded reference images", err)
	}
	out := make(map[int][]embeddedReferenceImage)
	for _, cell := range cells {
		_, anchorRow, err := excelize.CellNameToCoordinates(cell)
		if err != nil {
			return nil, excelAppError("read embedded reference image anchor", err)
		}
		if anchorRow <= 1 {
			continue
		}
		pictures, err := f.GetPictures(dataSheet, cell)
		if err != nil {
			return nil, excelAppError("read embedded reference image bytes", err)
		}
		for _, pic := range pictures {
			if len(pic.File) == 0 {
				continue
			}
			if len(pic.File) > maxEmbeddedReferenceImageBytes {
				row := visualPictureRow(f, dataSheet, anchorRow, pic)
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "embedded reference image is too large", map[string]interface{}{
					"violations": []ParseViolation{{
						Row:     row,
						Column:  "参考图",
						Code:    "reference_image_too_large",
						Message: fmt.Sprintf("one embedded reference image can be at most %d bytes", maxEmbeddedReferenceImageBytes),
					}},
				})
			}
			extension := normalizePictureExtension(pic.Extension, pic.File)
			row := resolvedPictureRow(f, dataSheet, cell, anchorRow, pic)
			if row <= 1 {
				continue
			}
			if len(out[row]) >= maxEmbeddedReferenceImagesPerRow {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "too many reference images in one Excel row", map[string]interface{}{
					"violations": []ParseViolation{{
						Row:     row,
						Column:  "参考图",
						Code:    "too_many_reference_images",
						Message: fmt.Sprintf("one row can contain at most %d reference images", maxEmbeddedReferenceImagesPerRow),
					}},
				})
			}
			out[row] = append(out[row], embeddedReferenceImage{
				Cell:      cell,
				Row:       row,
				Index:     len(out[row]) + 1,
				Extension: extension,
				File:      append([]byte(nil), pic.File...),
				MimeType:  mimeTypeForPicture(extension, pic.File),
			})
		}
	}
	return out, nil
}

func visualPictureRow(f *excelize.File, sheet string, anchorRow int, pic excelize.Picture) int {
	if anchorRow <= 0 || len(pic.File) == 0 {
		return anchorRow
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(pic.File))
	if err != nil || cfg.Height <= 0 {
		return anchorRow
	}
	scaleY := 1.0
	offsetY := 0
	if pic.Format != nil {
		if pic.Format.ScaleY > 0 {
			scaleY = pic.Format.ScaleY
		}
		if pic.Format.OffsetY > 0 {
			offsetY = pic.Format.OffsetY
		}
	}
	centerY := float64(offsetY) + float64(cfg.Height)*scaleY/2
	row := anchorRow
	for rowHeight := excelRowHeightPixels(f, sheet, row); rowHeight > 0 && centerY >= rowHeight; rowHeight = excelRowHeightPixels(f, sheet, row) {
		centerY -= rowHeight
		row++
	}
	return row
}

func resolvedPictureRow(f *excelize.File, sheet string, cell string, anchorRow int, pic excelize.Picture) int {
	if pictureCellIsReferenceColumn(f, sheet, cell) {
		return anchorRow
	}
	return visualPictureRow(f, sheet, anchorRow, pic)
}

func pictureCellIsReferenceColumn(f *excelize.File, sheet string, cell string) bool {
	col, _, err := excelize.CellNameToCoordinates(cell)
	if err != nil || col <= 0 {
		return false
	}
	headerCell, err := excelize.CoordinatesToCellName(col, 1)
	if err != nil {
		return false
	}
	header, err := f.GetCellValue(sheet, headerCell)
	if err != nil {
		return false
	}
	normalized := strings.ReplaceAll(strings.TrimSpace(header), " ", "")
	return normalized == "参考图" || strings.HasPrefix(normalized, "参考图")
}

func excelRowHeightPixels(f *excelize.File, sheet string, row int) float64 {
	height, err := f.GetRowHeight(sheet, row)
	if err != nil || height <= 0 {
		return defaultExcelRowHeightPixels
	}
	if math.Abs(height-defaultExcelRowHeightPoints) < 0.01 {
		return defaultExcelRowHeightPixels
	}
	return math.Ceil(4.0 / 3.4 * height)
}

func (s *parseService) validateProductIIDs(ctx context.Context, items []service.CreateTaskBatchSKUItemParams, itemRows []int, lookup ERPIIDLookup) ([]ParseViolation, *domain.AppError) {
	if lookup == nil {
		return nil, nil
	}
	seen := make(map[string]bool)
	var violations []ParseViolation
	for idx, item := range items {
		iid := strings.TrimSpace(item.ProductIID)
		if iid == "" || seen[iid] {
			continue
		}
		seen[iid] = true
		resp, appErr := lookup.ListIIDs(ctx, domain.ERPIIDListFilter{Q: iid, Page: 1, PageSize: 50})
		if appErr != nil {
			return nil, appErr
		}
		found := false
		if resp != nil {
			for _, option := range resp.Items {
				if option != nil && strings.EqualFold(strings.TrimSpace(option.IID), iid) {
					found = true
					break
				}
			}
		}
		if !found {
			row := idx + 2
			if idx < len(itemRows) && itemRows[idx] > 0 {
				row = itemRows[idx]
			}
			violations = append(violations, ParseViolation{
				Row:     row,
				Column:  "产品款式编码",
				Code:    "invalid_i_id",
				Message: "batch_items[].product_i_id must be selected from ERP product i_id options",
			})
		}
	}
	return violations, nil
}

func (s *parseService) uploadEmbeddedReferenceImages(ctx context.Context, imagesByRow map[int][]embeddedReferenceImage, options ParseOptions, items []service.CreateTaskBatchSKUItemParams, preview []BatchItem, itemRows []int) *domain.AppError {
	if len(imagesByRow) == 0 {
		return nil
	}
	if options.ReferenceUploader == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "batch Excel reference image upload is not configured", nil)
	}
	if options.ActorID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "actor_id is required for embedded reference image upload", nil)
	}
	for idx := range items {
		rowNumber := idx + 2
		if idx < len(itemRows) && itemRows[idx] > 0 {
			rowNumber = itemRows[idx]
		}
		images := imagesByRow[rowNumber]
		if len(images) == 0 {
			continue
		}
		refs := make([]domain.ReferenceFileRef, 0, len(images))
		for _, image := range images {
			size := int64(len(image.File))
			filename := fmt.Sprintf("batch-row-%d-reference-%d%s", rowNumber, image.Index, image.Extension)
			ref, appErr := options.ReferenceUploader.UploadFile(ctx, service.UploadTaskReferenceFileParams{
				CreatedBy:    options.ActorID,
				Filename:     filename,
				ExpectedSize: &size,
				MimeType:     image.MimeType,
				Remark:       fmt.Sprintf("batch Excel row %d reference image", rowNumber),
				File:         bytes.NewReader(image.File),
			})
			if appErr != nil {
				return appErr
			}
			if ref != nil {
				refs = append(refs, *ref)
			}
		}
		items[idx].ReferenceFileRefs = refs
		preview[idx].ReferenceFileRefs = refs
	}
	return nil
}

func normalizePictureExtension(extension string, file []byte) string {
	ext := strings.ToLower(strings.TrimSpace(extension))
	if ext != "" {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		return ext
	}
	switch http.DetectContentType(file) {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

func mimeTypeForPicture(extension string, file []byte) string {
	switch strings.ToLower(strings.TrimPrefix(extension, ".")) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "bmp":
		return "image/bmp"
	case "webp":
		return "image/webp"
	case "tif", "tiff":
		return "image/tiff"
	default:
		detected := http.DetectContentType(file)
		if strings.HasPrefix(detected, "image/") {
			return detected
		}
		return "application/octet-stream"
	}
}

func mapValidationViolations(appErr *domain.AppError, taskType domain.TaskType, fields []FieldSpec, items []service.CreateTaskBatchSKUItemParams, itemRows []int, imagesByRow map[int][]embeddedReferenceImage) []ParseViolation {
	rawViolations := extractViolations(appErr.Details)
	out := make([]ParseViolation, 0, len(rawViolations))
	byKey := fieldByKey(fields)
	imageOnlyRequiredRows := map[int]bool{}
	for _, raw := range rawViolations {
		fieldPath, _ := raw["field"].(string)
		code, _ := raw["code"].(string)
		message, _ := raw["message"].(string)
		idx, key := rowIndexAndKeyFromFieldPath(fieldPath)
		row := sourceRowForItemIndex(idx, itemRows)
		if key == "sku_code" {
			key = skuColumnKey(fields)
		}
		column := ""
		if field, ok := byKey[key]; ok {
			column = field.Column
		}
		if code == "missing_required_field" && isImageOnlyBatchItem(idx, items, itemRows, imagesByRow) && (key == "product_name" || key == "design_requirement") {
			if imageOnlyRequiredRows[row] {
				continue
			}
			imageOnlyRequiredRows[row] = true
			out = append(out, ParseViolation{
				Row:     row,
				Column:  "产品信息",
				Code:    "image_only_row_missing_required",
				Message: "该行只检测到参考图，没有读取到产品名称和设计要求；请把图片放到对应产品行，或补齐该行必填文字后重新上传",
			})
			continue
		}
		if code == "duplicate_batch_item" && isImageOnlyBatchItem(idx, items, itemRows, imagesByRow) {
			continue
		}
		if code == "duplicate_batch_item" {
			message = duplicateBatchItemMessage(taskType, idx, items, itemRows, message)
		}
		out = append(out, ParseViolation{
			Row:     row,
			Column:  column,
			Code:    code,
			Message: message,
		})
	}
	return out
}

func skuColumnKey(fields []FieldSpec) string {
	for _, field := range fields {
		if field.Key == "new_sku" || field.Key == "purchase_sku" {
			return field.Key
		}
	}
	return "sku_code"
}

func extractViolations(details interface{}) []map[string]interface{} {
	detailMap, ok := details.(map[string]interface{})
	if !ok {
		return nil
	}
	raw, ok := detailMap["violations"]
	if !ok {
		return nil
	}
	switch violations := raw.(type) {
	case []map[string]interface{}:
		return violations
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(violations))
		for _, item := range violations {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func rowIndexAndKeyFromFieldPath(fieldPath string) (int, string) {
	matches := batchFieldPathRE.FindStringSubmatch(fieldPath)
	if len(matches) == 0 {
		return -1, fieldPath
	}
	idx, _ := strconv.Atoi(matches[1])
	return idx, matches[2]
}

func sourceRowForItemIndex(idx int, itemRows []int) int {
	if idx >= 0 && idx < len(itemRows) && itemRows[idx] > 0 {
		return itemRows[idx]
	}
	if idx >= 0 {
		return idx + 2
	}
	return 0
}

func rowIsEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func isImageOnlyBatchItem(idx int, items []service.CreateTaskBatchSKUItemParams, itemRows []int, imagesByRow map[int][]embeddedReferenceImage) bool {
	if idx < 0 || idx >= len(items) {
		return false
	}
	row := sourceRowForItemIndex(idx, itemRows)
	if row <= 0 || len(imagesByRow[row]) == 0 {
		return false
	}
	item := items[idx]
	return strings.TrimSpace(item.ProductName) == "" &&
		strings.TrimSpace(item.ProductShortName) == "" &&
		strings.TrimSpace(item.CategoryCode) == "" &&
		strings.TrimSpace(item.ProductIID) == "" &&
		strings.TrimSpace(item.MaterialMode) == "" &&
		strings.TrimSpace(item.DesignRequirement) == "" &&
		strings.TrimSpace(item.NewSKU) == "" &&
		strings.TrimSpace(item.PurchaseSKU) == "" &&
		strings.TrimSpace(item.CostPriceMode) == "" &&
		item.Quantity == nil &&
		item.BaseSalePrice == nil &&
		len(bytes.TrimSpace(item.VariantJSON)) == 0
}

func duplicateBatchItemMessage(taskType domain.TaskType, idx int, items []service.CreateTaskBatchSKUItemParams, itemRows []int, fallback string) string {
	if idx <= 0 || idx >= len(items) {
		return fallback
	}
	key, appErr := service.ComputeTaskBatchItemDedupeKeyForExcelDiagnostics(taskType, items[idx])
	if appErr != nil || key == "" {
		return fallback
	}
	for prev := 0; prev < idx; prev++ {
		prevKey, prevErr := service.ComputeTaskBatchItemDedupeKeyForExcelDiagnostics(taskType, items[prev])
		if prevErr == nil && prevKey == key {
			return fmt.Sprintf("第 %d 行与第 %d 行内容重复，请删除重复行或调整产品信息/设计要求", sourceRowForItemIndex(idx, itemRows), sourceRowForItemIndex(prev, itemRows))
		}
	}
	return fallback
}

func batchItemFromService(item service.CreateTaskBatchSKUItemParams) BatchItem {
	return BatchItem{
		ProductName:       item.ProductName,
		ProductShortName:  item.ProductShortName,
		CategoryCode:      item.CategoryCode,
		ProductIID:        item.ProductIID,
		MaterialMode:      item.MaterialMode,
		DesignRequirement: item.DesignRequirement,
		NewSKU:            item.NewSKU,
		PurchaseSKU:       item.PurchaseSKU,
		CostPriceMode:     item.CostPriceMode,
		Quantity:          item.Quantity,
		BaseSalePrice:     item.BaseSalePrice,
		VariantJSON:       item.VariantJSON,
		ReferenceFileRefs: item.ReferenceFileRefs,
	}
}

func parseViolationForRow(row int, violation ParseViolation) ParseViolation {
	violation.Row = row
	return violation
}

func parseViolationsForRow(row int, violations []ParseViolation) []ParseViolation {
	for i := range violations {
		violations[i] = parseViolationForRow(row, violations[i])
	}
	return violations
}

func invalidCellMessage(field FieldSpec, value string) string {
	return fmt.Sprintf("%s has invalid value %q", field.Key, value)
}
