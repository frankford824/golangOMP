package task_batch_excel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
)

type parseService struct {
	referenceUploader ReferenceUploader
	iidLookup         ERPIIDLookup
}

var batchFieldPathRE = regexp.MustCompile(`^batch_items\[(\d+)\](?:\.(.+))?$`)

func (s *parseService) Parse(ctx context.Context, taskType domain.TaskType, file io.Reader, opts ...ParseOption) (*ParseResult, *domain.AppError) {
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

	rows, err := f.GetRows(itemsSheet)
	if err != nil {
		return nil, excelAppError("read Items sheet", err)
	}
	if len(rows) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file is missing header row", nil)
	}
	columnIndex, appErr := parseHeader(rows[0], fields)
	if appErr != nil {
		return nil, appErr
	}
	imagesByRow, appErr := extractEmbeddedReferenceImages(f)
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
		preview = append(preview, batchItemFromService(item))
		itemRows = append(itemRows, rowNumber)
		if len(parseViolations) > 0 {
			parseViolations = parseViolationsForRow(rowIdx+1, parseViolations)
			return &ParseResult{TaskType: taskType, Preview: preview, Violations: parseViolations}, nil
		}
	}

	if iidViolations, appErr := s.validateProductIIDs(ctx, items, itemRows, options.IIDLookup); appErr != nil {
		return nil, appErr
	} else if len(iidViolations) > 0 {
		return &ParseResult{TaskType: taskType, Preview: preview, Violations: iidViolations}, nil
	}

	params := service.CreateTaskParams{
		TaskType:     taskType,
		SourceMode:   domain.TaskSourceModeNewProduct,
		BatchSKUMode: "multiple",
		BatchItems:   items,
	}
	if appErr := service.ValidateBatchTaskCreateRequest(params); appErr != nil {
		return &ParseResult{
			TaskType:   taskType,
			Preview:    preview,
			Violations: mapValidationViolations(appErr, fields),
		}, nil
	}
	if appErr := s.uploadEmbeddedReferenceImages(ctx, imagesByRow, options, items, preview, itemRows); appErr != nil {
		return nil, appErr
	}
	return &ParseResult{TaskType: taskType, Preview: preview, Violations: []ParseViolation{}}, nil
}

func parseHeader(header []string, fields []FieldSpec) (map[string]int, *domain.AppError) {
	index := make(map[string]int, len(fields))
	byColumn := make(map[string]FieldSpec, len(fields))
	for _, field := range fields {
		byColumn[strings.TrimSpace(field.Column)] = field
	}
	for i, raw := range header {
		column := strings.TrimSpace(raw)
		if field, ok := byColumn[column]; ok {
			index[field.Key] = i
		}
	}
	for _, field := range fields {
		if field.Required {
			if _, ok := index[field.Key]; !ok {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel header is missing required column", map[string]interface{}{
					"violations": []ParseViolation{{
						Row:     1,
						Column:  field.Column,
						Code:    "missing_required_field",
						Message: "missing required column " + field.Column,
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
	for _, field := range fields {
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
		case "product_i_id":
			item.ProductIID = value
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

func extractEmbeddedReferenceImages(f *excelize.File) (map[int][]embeddedReferenceImage, *domain.AppError) {
	cells, err := f.GetPictureCells(itemsSheet)
	if err != nil {
		return nil, excelAppError("read embedded reference images", err)
	}
	out := make(map[int][]embeddedReferenceImage)
	for _, cell := range cells {
		_, row, err := excelize.CellNameToCoordinates(cell)
		if err != nil {
			return nil, excelAppError("read embedded reference image anchor", err)
		}
		if row <= 1 {
			continue
		}
		pictures, err := f.GetPictures(itemsSheet, cell)
		if err != nil {
			return nil, excelAppError("read embedded reference image bytes", err)
		}
		if len(out[row])+len(pictures) > maxEmbeddedReferenceImagesPerRow {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "too many reference images in one Excel row", map[string]interface{}{
				"violations": []ParseViolation{{
					Row:     row,
					Column:  "参考图",
					Code:    "too_many_reference_images",
					Message: fmt.Sprintf("one row can contain at most %d reference images", maxEmbeddedReferenceImagesPerRow),
				}},
			})
		}
		for _, pic := range pictures {
			if len(pic.File) == 0 {
				continue
			}
			if len(pic.File) > maxEmbeddedReferenceImageBytes {
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
				Column:  "产品i_id",
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

func mapValidationViolations(appErr *domain.AppError, fields []FieldSpec) []ParseViolation {
	rawViolations := extractViolations(appErr.Details)
	out := make([]ParseViolation, 0, len(rawViolations))
	byKey := fieldByKey(fields)
	for _, raw := range rawViolations {
		fieldPath, _ := raw["field"].(string)
		code, _ := raw["code"].(string)
		message, _ := raw["message"].(string)
		row, key := rowAndKeyFromFieldPath(fieldPath)
		if key == "sku_code" {
			key = skuColumnKey(fields)
		}
		column := ""
		if field, ok := byKey[key]; ok {
			column = field.Column
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

func rowAndKeyFromFieldPath(fieldPath string) (int, string) {
	matches := batchFieldPathRE.FindStringSubmatch(fieldPath)
	if len(matches) == 0 {
		return 0, fieldPath
	}
	idx, _ := strconv.Atoi(matches[1])
	return idx + 2, matches[2]
}

func rowIsEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
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
