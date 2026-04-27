package service

import (
	"encoding/json"
	"strings"

	"workflow/domain"
)

const (
	taskProductSelectionMatchMapped = "mapped_product_search"
	taskProductSelectionMatchManual = "manual_existing_product_binding"
	taskProductSelectionMatchLegacy = "legacy_existing_product_binding"
)

// normalizeTaskProductSelection normalizes and validates product_selection for task creation/update.
// When sourceMode != existing_product: product_selection is not supported. The handler MUST pass nil
// for new_product_development/purchase_task when product_selection was not explicitly in the request body,
// to avoid false rejection from selection synthesized via product_id/sku_code in bindCreateTaskERPProductID.
// This layer rejects only when a non-empty selection is passed for non-existing_product (defense in depth).
func normalizeTaskProductSelection(
	sourceMode domain.TaskSourceMode,
	allowRebind bool,
	productID **int64,
	skuCode *string,
	productName *string,
	selection *domain.TaskProductSelectionContext,
) (*domain.TaskProductSelectionContext, *domain.AppError) {
	hasEffectiveSelection := !isTaskProductSelectionEmpty(selection)
	if sourceMode != domain.TaskSourceModeExistingProduct {
		// Only reject when an effective selection is provided.
		// Empty/null payloads are treated as "not provided".
		if hasEffectiveSelection {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "product_selection is only supported when source_mode is existing_product", nil)
		}
		return nil, nil
	}

	if selection == nil {
		return buildLegacyTaskProductSelection(*productID, strings.TrimSpace(*productName), strings.TrimSpace(*skuCode)), nil
	}

	normalized := cloneTaskProductSelection(selection)
	normalized.SelectedProductName = strings.TrimSpace(normalized.SelectedProductName)
	normalized.SelectedProductSKUCode = strings.TrimSpace(normalized.SelectedProductSKUCode)
	normalized.MatchedCategoryCode = strings.ToUpper(strings.TrimSpace(normalized.MatchedCategoryCode))
	normalized.MatchedSearchEntryCode = strings.ToUpper(strings.TrimSpace(normalized.MatchedSearchEntryCode))
	normalized.SourceProductName = strings.TrimSpace(normalized.SourceProductName)
	normalized.SourceMatchType = strings.TrimSpace(normalized.SourceMatchType)
	normalized.SourceMatchRule = strings.TrimSpace(normalized.SourceMatchRule)
	normalized.SourceSearchEntryCode = strings.ToUpper(strings.TrimSpace(normalized.SourceSearchEntryCode))
	normalized.ERPProduct = normalizeERPProductSelectionSnapshot(normalized.ERPProduct)

	if normalized.MatchedMappingRule != nil {
		normalized.MatchedMappingRule.CategoryCode = strings.ToUpper(strings.TrimSpace(normalized.MatchedMappingRule.CategoryCode))
		normalized.MatchedMappingRule.SearchEntryCode = strings.ToUpper(strings.TrimSpace(normalized.MatchedMappingRule.SearchEntryCode))
		normalized.MatchedMappingRule.ERPMatchValue = strings.TrimSpace(normalized.MatchedMappingRule.ERPMatchValue)
		normalized.MatchedMappingRule.SecondaryConditionKey = strings.TrimSpace(normalized.MatchedMappingRule.SecondaryConditionKey)
		normalized.MatchedMappingRule.SecondaryConditionValue = strings.TrimSpace(normalized.MatchedMappingRule.SecondaryConditionValue)
		normalized.MatchedMappingRule.TertiaryConditionKey = strings.TrimSpace(normalized.MatchedMappingRule.TertiaryConditionKey)
		normalized.MatchedMappingRule.TertiaryConditionValue = strings.TrimSpace(normalized.MatchedMappingRule.TertiaryConditionValue)
	}

	switch {
	case normalized.SelectedProductID != nil && *productID == nil:
		*productID = cloneInt64Ptr(normalized.SelectedProductID)
	case normalized.SelectedProductID != nil && *productID != nil && **productID != *normalized.SelectedProductID && allowRebind:
		*productID = cloneInt64Ptr(normalized.SelectedProductID)
	case normalized.SelectedProductID != nil && *productID != nil && **productID != *normalized.SelectedProductID:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "product_selection.selected_product_id must match product_id", nil)
	}
	if normalized.SelectedProductID == nil {
		normalized.SelectedProductID = cloneInt64Ptr(*productID)
	}

	switch {
	case normalized.SelectedProductSKUCode != "" && strings.TrimSpace(*skuCode) == "":
		*skuCode = normalized.SelectedProductSKUCode
	case normalized.SelectedProductSKUCode != "" && strings.TrimSpace(*skuCode) != normalized.SelectedProductSKUCode && allowRebind:
		*skuCode = normalized.SelectedProductSKUCode
	case normalized.SelectedProductSKUCode != "" && strings.TrimSpace(*skuCode) != normalized.SelectedProductSKUCode:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "product_selection.selected_product_sku_code must match sku_code", nil)
	}
	if normalized.SelectedProductSKUCode == "" {
		normalized.SelectedProductSKUCode = strings.TrimSpace(*skuCode)
	}

	switch {
	case normalized.SelectedProductName != "" && strings.TrimSpace(*productName) == "":
		*productName = normalized.SelectedProductName
	case normalized.SelectedProductName != "" && strings.TrimSpace(*productName) != normalized.SelectedProductName && allowRebind:
		*productName = normalized.SelectedProductName
	case normalized.SelectedProductName != "" && strings.TrimSpace(*productName) != normalized.SelectedProductName:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "product_selection.selected_product_name must match product_name_snapshot", nil)
	}
	if normalized.SelectedProductName == "" {
		normalized.SelectedProductName = strings.TrimSpace(*productName)
	}

	if normalized.SourceProductID == nil {
		normalized.SourceProductID = cloneInt64Ptr(normalized.SelectedProductID)
	}
	if normalized.SourceProductName == "" {
		normalized.SourceProductName = normalized.SelectedProductName
	}

	if normalized.MatchedMappingRule != nil {
		if normalized.MatchedCategoryCode == "" {
			normalized.MatchedCategoryCode = normalized.MatchedMappingRule.CategoryCode
		} else if normalized.MatchedMappingRule.CategoryCode != "" && normalized.MatchedCategoryCode != normalized.MatchedMappingRule.CategoryCode {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "product_selection.matched_category_code must match matched_mapping_rule.category_code", nil)
		}

		if normalized.MatchedSearchEntryCode == "" {
			normalized.MatchedSearchEntryCode = normalized.MatchedMappingRule.SearchEntryCode
		} else if normalized.MatchedMappingRule.SearchEntryCode != "" && normalized.MatchedSearchEntryCode != normalized.MatchedMappingRule.SearchEntryCode {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "product_selection.matched_search_entry_code must match matched_mapping_rule.search_entry_code", nil)
		}

		if normalized.SourceSearchEntryCode == "" {
			normalized.SourceSearchEntryCode = normalized.MatchedMappingRule.SearchEntryCode
		}
		if normalized.SourceMatchType == "" && normalized.MatchedMappingRule.ERPMatchType != "" {
			normalized.SourceMatchType = string(normalized.MatchedMappingRule.ERPMatchType)
		}
		if normalized.SourceMatchRule == "" {
			normalized.SourceMatchRule = normalized.MatchedMappingRule.ERPMatchValue
		}
	}

	if normalized.SourceSearchEntryCode == "" {
		normalized.SourceSearchEntryCode = normalized.MatchedSearchEntryCode
	}
	if normalized.SourceMatchType == "" {
		switch {
		case normalized.MatchedMappingRule != nil || normalized.MatchedSearchEntryCode != "" || normalized.MatchedCategoryCode != "":
			normalized.SourceMatchType = taskProductSelectionMatchMapped
		default:
			normalized.SourceMatchType = taskProductSelectionMatchManual
		}
	}
	normalized.ERPProduct = hydrateERPProductSelectionSnapshot(normalized.ERPProduct, nil, &domain.Task{
		ProductID:           cloneInt64Ptr(normalized.SelectedProductID),
		SKUCode:             strings.TrimSpace(normalized.SelectedProductSKUCode),
		ProductNameSnapshot: strings.TrimSpace(normalized.SelectedProductName),
	})

	if normalized.SelectedProductID == nil {
		if !(normalized.DeferLocalProductBinding && erpSnapshotSufficientForDeferredBinding(normalized.ERPProduct)) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "selected_product_id is required when source_mode is existing_product (unless defer_local_product_binding with complete erp_product)", nil)
		}
	}

	return normalized, nil
}

func erpSnapshotSufficientForDeferredBinding(erp *domain.ERPProductSelectionSnapshot) bool {
	if erp == nil {
		return false
	}
	hasKey := strings.TrimSpace(erp.ProductID) != "" || strings.TrimSpace(erp.SKUID) != "" || strings.TrimSpace(erp.SKUCode) != ""
	if !hasKey {
		return false
	}
	name := strings.TrimSpace(erp.ProductName)
	if name == "" {
		name = strings.TrimSpace(erp.Name)
	}
	if name == "" {
		name = strings.TrimSpace(erp.SKUCode)
	}
	if name == "" {
		name = strings.TrimSpace(erp.SKUID)
	}
	if name == "" {
		name = strings.TrimSpace(erp.ProductID)
	}
	return name != ""
}

func buildLegacyTaskProductSelection(productID *int64, productName, skuCode string) *domain.TaskProductSelectionContext {
	if productID == nil && strings.TrimSpace(productName) == "" && strings.TrimSpace(skuCode) == "" {
		return nil
	}
	return &domain.TaskProductSelectionContext{
		SelectedProductID:      cloneInt64Ptr(productID),
		SelectedProductName:    strings.TrimSpace(productName),
		SelectedProductSKUCode: strings.TrimSpace(skuCode),
		SourceProductID:        cloneInt64Ptr(productID),
		SourceProductName:      strings.TrimSpace(productName),
		SourceMatchType:        taskProductSelectionMatchLegacy,
	}
}

func applyTaskProductSelection(detail *domain.TaskDetail, selection *domain.TaskProductSelectionContext, task *domain.Task) {
	if detail == nil {
		return
	}
	if selection == nil {
		detail.SourceProductID = nil
		detail.SourceProductName = ""
		detail.SourceSearchEntryCode = ""
		detail.SourceMatchType = ""
		detail.SourceMatchRule = ""
		detail.MatchedCategoryCode = ""
		detail.MatchedSearchEntryCode = ""
		detail.MatchedMappingRuleJSON = ""
		detail.ProductSelectionSnapshotJSON = ""
		detail.ProductSelection = nil
		return
	}

	detail.SourceProductID = cloneInt64Ptr(selection.SourceProductID)
	detail.SourceProductName = strings.TrimSpace(selection.SourceProductName)
	detail.SourceSearchEntryCode = strings.ToUpper(strings.TrimSpace(selection.SourceSearchEntryCode))
	detail.SourceMatchType = strings.TrimSpace(selection.SourceMatchType)
	detail.SourceMatchRule = strings.TrimSpace(selection.SourceMatchRule)
	detail.MatchedCategoryCode = strings.ToUpper(strings.TrimSpace(selection.MatchedCategoryCode))
	detail.MatchedSearchEntryCode = strings.ToUpper(strings.TrimSpace(selection.MatchedSearchEntryCode))
	detail.MatchedMappingRuleJSON = marshalTaskProductSelectionMapping(selection.MatchedMappingRule)
	detail.ProductSelectionSnapshotJSON = marshalTaskProductSelectionSnapshot(selection)
	detail.ProductSelection = buildTaskProductSelectionContext(task, detail)
}

func buildTaskProductSelectionContext(task *domain.Task, detail *domain.TaskDetail) *domain.TaskProductSelectionContext {
	if task == nil && detail == nil {
		return nil
	}
	if task != nil && task.SourceMode != domain.TaskSourceModeExistingProduct {
		return nil
	}

	var mapping *domain.ProductSearchMatchedMapping
	if detail != nil && strings.TrimSpace(detail.MatchedMappingRuleJSON) != "" {
		var parsed domain.ProductSearchMatchedMapping
		if err := json.Unmarshal([]byte(detail.MatchedMappingRuleJSON), &parsed); err == nil {
			mapping = &parsed
		}
	}
	var snapshot *domain.TaskProductSelectionContext
	if detail != nil && !isTaskProductSelectionEmpty(detail.ProductSelection) {
		snapshot = cloneTaskProductSelection(detail.ProductSelection)
	}
	if detail != nil && strings.TrimSpace(detail.ProductSelectionSnapshotJSON) != "" {
		var parsed domain.TaskProductSelectionContext
		if err := json.Unmarshal([]byte(detail.ProductSelectionSnapshotJSON), &parsed); err == nil {
			snapshot = &parsed
		}
	}

	if detail == nil {
		return buildLegacyTaskProductSelection(task.ProductID, task.ProductNameSnapshot, task.SKUCode)
	}

	if detail.SourceProductID == nil &&
		strings.TrimSpace(detail.SourceProductName) == "" &&
		strings.TrimSpace(detail.SourceSearchEntryCode) == "" &&
		strings.TrimSpace(detail.SourceMatchType) == "" &&
		strings.TrimSpace(detail.SourceMatchRule) == "" &&
		strings.TrimSpace(detail.MatchedCategoryCode) == "" &&
		strings.TrimSpace(detail.MatchedSearchEntryCode) == "" &&
		mapping == nil &&
		snapshot == nil {
		return buildLegacyTaskProductSelection(productIDFromTask(task), productNameFromTask(task), skuCodeFromTask(task))
	}

	selectedProductID := detail.SourceProductID
	selectedProductName := detail.SourceProductName
	selectedProductSKUCode := skuCodeFromTask(task)
	erpProduct := (*domain.ERPProductSelectionSnapshot)(nil)
	sourceProductID := cloneInt64Ptr(detail.SourceProductID)
	sourceProductName := strings.TrimSpace(detail.SourceProductName)
	sourceMatchType := strings.TrimSpace(detail.SourceMatchType)
	sourceMatchRule := strings.TrimSpace(detail.SourceMatchRule)
	sourceSearchEntryCode := strings.TrimSpace(detail.SourceSearchEntryCode)
	matchedCategoryCode := strings.TrimSpace(detail.MatchedCategoryCode)
	matchedSearchEntryCode := strings.TrimSpace(detail.MatchedSearchEntryCode)
	if task != nil {
		if task.ProductID != nil {
			selectedProductID = cloneInt64Ptr(task.ProductID)
		}
		if strings.TrimSpace(task.ProductNameSnapshot) != "" {
			selectedProductName = strings.TrimSpace(task.ProductNameSnapshot)
		}
	}
	if snapshot != nil {
		selectedProductID = firstNonNilSelectionInt64(cloneInt64Ptr(productIDFromTask(task)), cloneInt64Ptr(snapshot.SelectedProductID), cloneInt64Ptr(detail.SourceProductID))
		if strings.TrimSpace(snapshot.SelectedProductName) != "" {
			selectedProductName = strings.TrimSpace(snapshot.SelectedProductName)
		}
		if strings.TrimSpace(snapshot.SelectedProductSKUCode) != "" {
			selectedProductSKUCode = strings.TrimSpace(snapshot.SelectedProductSKUCode)
		}
		if sourceProductID == nil {
			sourceProductID = cloneInt64Ptr(snapshot.SourceProductID)
		}
		if sourceProductName == "" {
			sourceProductName = strings.TrimSpace(snapshot.SourceProductName)
		}
		if sourceMatchType == "" {
			sourceMatchType = strings.TrimSpace(snapshot.SourceMatchType)
		}
		if sourceMatchRule == "" {
			sourceMatchRule = strings.TrimSpace(snapshot.SourceMatchRule)
		}
		if sourceSearchEntryCode == "" {
			sourceSearchEntryCode = strings.TrimSpace(snapshot.SourceSearchEntryCode)
		}
		if matchedCategoryCode == "" {
			matchedCategoryCode = strings.TrimSpace(snapshot.MatchedCategoryCode)
		}
		if matchedSearchEntryCode == "" {
			matchedSearchEntryCode = strings.TrimSpace(snapshot.MatchedSearchEntryCode)
		}
		if mapping == nil && snapshot.MatchedMappingRule != nil {
			mappingCopy := *snapshot.MatchedMappingRule
			mapping = &mappingCopy
		}
		if snapshot.ERPProduct != nil {
			erpCopy := *snapshot.ERPProduct
			erpProduct = &erpCopy
		}
	}
	erpProduct = hydrateERPProductSelectionSnapshot(erpProduct, nil, task)

	return &domain.TaskProductSelectionContext{
		SelectedProductID:      cloneInt64Ptr(selectedProductID),
		SelectedProductName:    strings.TrimSpace(selectedProductName),
		SelectedProductSKUCode: selectedProductSKUCode,
		MatchedCategoryCode:    matchedCategoryCode,
		MatchedSearchEntryCode: matchedSearchEntryCode,
		MatchedMappingRule:     mapping,
		SourceProductID:        sourceProductID,
		SourceProductName:      sourceProductName,
		SourceMatchType:        sourceMatchType,
		SourceMatchRule:        sourceMatchRule,
		SourceSearchEntryCode:  sourceSearchEntryCode,
		ERPProduct:             erpProduct,
	}
}

func attachTaskProductSelection(detail *domain.TaskDetail, task *domain.Task) {
	if detail == nil {
		return
	}
	detail.ProductSelection = buildTaskProductSelectionContext(task, detail)
}

func buildTaskProductSelectionSummary(selection *domain.TaskProductSelectionContext) *domain.TaskProductSelectionSummary {
	if selection == nil {
		return nil
	}
	return &domain.TaskProductSelectionSummary{
		SelectedProductID:      cloneInt64Ptr(selection.SelectedProductID),
		SelectedProductName:    strings.TrimSpace(selection.SelectedProductName),
		SelectedProductSKUCode: strings.TrimSpace(selection.SelectedProductSKUCode),
		MatchedCategoryCode:    strings.TrimSpace(selection.MatchedCategoryCode),
		MatchedSearchEntryCode: strings.TrimSpace(selection.MatchedSearchEntryCode),
		SourceProductID:        cloneInt64Ptr(selection.SourceProductID),
		SourceProductName:      strings.TrimSpace(selection.SourceProductName),
		SourceMatchType:        strings.TrimSpace(selection.SourceMatchType),
		SourceMatchRule:        strings.TrimSpace(selection.SourceMatchRule),
		SourceSearchEntryCode:  strings.TrimSpace(selection.SourceSearchEntryCode),
		ERPProduct:             cloneERPProductSelectionSnapshot(selection.ERPProduct),
	}
}

func buildTaskProductSelectionSummaryFromTask(task *domain.Task, detail *domain.TaskDetail) *domain.TaskProductSelectionSummary {
	return buildTaskProductSelectionSummary(buildTaskProductSelectionContext(task, detail))
}

func buildTaskProductSelectionSummaryFromListItem(item *domain.TaskListItem) *domain.TaskProductSelectionSummary {
	if item == nil {
		return nil
	}
	if item.SourceMode != domain.TaskSourceModeExistingProduct {
		return nil
	}
	if item.SourceProductID == nil &&
		strings.TrimSpace(item.SourceProductName) == "" &&
		strings.TrimSpace(item.SourceSearchEntryCode) == "" &&
		strings.TrimSpace(item.SourceMatchType) == "" &&
		strings.TrimSpace(item.SourceMatchRule) == "" &&
		strings.TrimSpace(item.MatchedCategoryCode) == "" &&
		strings.TrimSpace(item.MatchedSearchEntryCode) == "" &&
		item.ProductID == nil &&
		strings.TrimSpace(item.ProductNameSnapshot) == "" &&
		strings.TrimSpace(item.SKUCode) == "" {
		return item.ProductSelection
	}

	return &domain.TaskProductSelectionSummary{
		SelectedProductID:      cloneInt64Ptr(item.ProductID),
		SelectedProductName:    strings.TrimSpace(item.ProductNameSnapshot),
		SelectedProductSKUCode: strings.TrimSpace(item.SKUCode),
		MatchedCategoryCode:    strings.TrimSpace(item.MatchedCategoryCode),
		MatchedSearchEntryCode: strings.TrimSpace(item.MatchedSearchEntryCode),
		SourceProductID:        cloneInt64Ptr(item.SourceProductID),
		SourceProductName:      strings.TrimSpace(item.SourceProductName),
		SourceMatchType:        strings.TrimSpace(item.SourceMatchType),
		SourceMatchRule:        strings.TrimSpace(item.SourceMatchRule),
		SourceSearchEntryCode:  strings.TrimSpace(item.SourceSearchEntryCode),
		ERPProduct:             taskProductSelectionERPProductFromListItem(item),
	}
}

func cloneTaskProductSelection(selection *domain.TaskProductSelectionContext) *domain.TaskProductSelectionContext {
	if selection == nil {
		return nil
	}
	cloned := *selection
	cloned.SelectedProductID = cloneInt64Ptr(selection.SelectedProductID)
	cloned.SourceProductID = cloneInt64Ptr(selection.SourceProductID)
	if selection.MatchedMappingRule != nil {
		mapping := *selection.MatchedMappingRule
		cloned.MatchedMappingRule = &mapping
	}
	cloned.ERPProduct = cloneERPProductSelectionSnapshot(selection.ERPProduct)
	return &cloned
}

func isTaskProductSelectionEmpty(selection *domain.TaskProductSelectionContext) bool {
	if selection == nil {
		return true
	}
	if selection.DeferLocalProductBinding && erpSnapshotSufficientForDeferredBinding(selection.ERPProduct) {
		return false
	}
	return selection.SelectedProductID == nil &&
		strings.TrimSpace(selection.SelectedProductName) == "" &&
		strings.TrimSpace(selection.SelectedProductSKUCode) == "" &&
		strings.TrimSpace(selection.MatchedCategoryCode) == "" &&
		strings.TrimSpace(selection.MatchedSearchEntryCode) == "" &&
		selection.MatchedMappingRule == nil &&
		selection.SourceProductID == nil &&
		strings.TrimSpace(selection.SourceProductName) == "" &&
		strings.TrimSpace(selection.SourceMatchType) == "" &&
		strings.TrimSpace(selection.SourceMatchRule) == "" &&
		strings.TrimSpace(selection.SourceSearchEntryCode) == "" &&
		selection.ERPProduct == nil
}

func marshalTaskProductSelectionMapping(mapping *domain.ProductSearchMatchedMapping) string {
	if mapping == nil {
		return ""
	}
	raw, err := json.Marshal(mapping)
	if err != nil {
		return ""
	}
	return string(raw)
}

func marshalTaskProductSelectionSnapshot(selection *domain.TaskProductSelectionContext) string {
	if selection == nil {
		return ""
	}
	raw, err := json.Marshal(selection)
	if err != nil {
		return ""
	}
	return string(raw)
}

func normalizeERPProductSelectionSnapshot(snapshot *domain.ERPProductSelectionSnapshot) *domain.ERPProductSelectionSnapshot {
	if snapshot == nil {
		return nil
	}
	normalized := *snapshot
	normalized.ProductID = strings.TrimSpace(normalized.ProductID)
	normalized.SKUID = strings.TrimSpace(normalized.SKUID)
	normalized.IID = strings.TrimSpace(normalized.IID)
	normalized.SKUCode = strings.TrimSpace(normalized.SKUCode)
	normalized.Name = strings.TrimSpace(normalized.Name)
	normalized.ProductName = strings.TrimSpace(normalized.ProductName)
	normalized.ShortName = strings.TrimSpace(normalized.ShortName)
	normalized.CategoryID = strings.TrimSpace(normalized.CategoryID)
	normalized.CategoryCode = strings.TrimSpace(normalized.CategoryCode)
	normalized.CategoryName = strings.TrimSpace(normalized.CategoryName)
	normalized.ProductShortName = strings.TrimSpace(normalized.ProductShortName)
	normalized.ImageURL = strings.TrimSpace(normalized.ImageURL)
	normalized.WMSCoID = strings.TrimSpace(normalized.WMSCoID)
	normalized.Currency = strings.TrimSpace(normalized.Currency)
	if normalized.ProductID == "" &&
		normalized.SKUID == "" &&
		normalized.IID == "" &&
		normalized.SKUCode == "" &&
		normalized.Name == "" &&
		normalized.ProductName == "" &&
		normalized.ShortName == "" &&
		normalized.CategoryID == "" &&
		normalized.CategoryCode == "" &&
		normalized.CategoryName == "" &&
		normalized.ProductShortName == "" &&
		normalized.ImageURL == "" &&
		normalized.Price == nil &&
		normalized.SPrice == nil &&
		normalized.WMSCoID == "" &&
		normalized.Currency == "" {
		return nil
	}
	if normalized.Name == "" {
		normalized.Name = normalized.ProductName
	}
	if normalized.ProductName == "" {
		normalized.ProductName = normalized.Name
	}
	if normalized.ShortName == "" {
		normalized.ShortName = normalized.ProductShortName
	}
	if normalized.ProductShortName == "" {
		normalized.ProductShortName = normalized.ShortName
	}
	if normalized.SPrice == nil {
		normalized.SPrice = normalized.Price
	}
	if normalized.Price == nil {
		normalized.Price = normalized.SPrice
	}
	return &normalized
}

func cloneERPProductSelectionSnapshot(snapshot *domain.ERPProductSelectionSnapshot) *domain.ERPProductSelectionSnapshot {
	if snapshot == nil {
		return nil
	}
	cloned := *snapshot
	if snapshot.Price != nil {
		price := *snapshot.Price
		cloned.Price = &price
	}
	if snapshot.SPrice != nil {
		sPrice := *snapshot.SPrice
		cloned.SPrice = &sPrice
	}
	return &cloned
}

func firstNonNilSelectionInt64(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func taskProductSelectionERPProductFromListItem(item *domain.TaskListItem) *domain.ERPProductSelectionSnapshot {
	if item == nil || strings.TrimSpace(item.ProductSelectionSnapshotJSON) == "" {
		return nil
	}
	var selection domain.TaskProductSelectionContext
	if err := json.Unmarshal([]byte(item.ProductSelectionSnapshotJSON), &selection); err != nil {
		return nil
	}
	return hydrateERPProductSelectionSnapshot(cloneERPProductSelectionSnapshot(selection.ERPProduct), nil, &domain.Task{
		ProductID:           cloneInt64Ptr(item.ProductID),
		SKUCode:             strings.TrimSpace(item.SKUCode),
		ProductNameSnapshot: strings.TrimSpace(item.ProductNameSnapshot),
	})
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func productIDFromTask(task *domain.Task) *int64 {
	if task == nil {
		return nil
	}
	return task.ProductID
}

func productNameFromTask(task *domain.Task) string {
	if task == nil {
		return ""
	}
	return task.ProductNameSnapshot
}

func skuCodeFromTask(task *domain.Task) string {
	if task == nil {
		return ""
	}
	return strings.TrimSpace(task.SKUCode)
}
