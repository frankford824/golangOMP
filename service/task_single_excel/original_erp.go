package task_single_excel

import (
	"context"
	"strings"

	"workflow/domain"
)

func erpProductsMatchingSKU(items []*domain.ERPProduct, skuCode string) []*domain.ERPProduct {
	skuCode = strings.TrimSpace(skuCode)
	if skuCode == "" {
		return nil
	}
	var matches []*domain.ERPProduct
	for _, item := range items {
		if item == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.SKUCode), skuCode) ||
			strings.EqualFold(strings.TrimSpace(item.SKUID), skuCode) {
			matches = append(matches, item)
		}
	}
	return matches
}

func erpProductDisplayName(product *domain.ERPProduct) string {
	if product == nil {
		return ""
	}
	if name := strings.TrimSpace(product.ProductName); name != "" {
		return name
	}
	return strings.TrimSpace(product.Name)
}

func applyERPProductToOriginalDraft(draft *SingleTaskDraft, product *domain.ERPProduct) {
	if draft == nil || product == nil {
		return
	}
	draft.ProductID = strings.TrimSpace(product.ProductID)
	draft.SKUID = strings.TrimSpace(product.SKUID)
	draft.SKUCode = strings.TrimSpace(product.SKUCode)
	if draft.SKUCode == "" {
		draft.SKUCode = strings.TrimSpace(product.SKUID)
	}
	name := erpProductDisplayName(product)
	draft.ProductName = name
	draft.ProductNameSnapshot = name
	draft.CategoryCode = strings.TrimSpace(product.CategoryCode)
	draft.CategoryName = strings.TrimSpace(product.CategoryName)
	draft.ImageURL = strings.TrimSpace(product.ImageURL)
	draft.ERPProduct = &ERPProductDraftSnapshot{
		ProductID:    draft.ProductID,
		SKUCode:      draft.SKUCode,
		SKUID:        draft.SKUID,
		Name:         name,
		ProductName:  name,
		CategoryCode: draft.CategoryCode,
		CategoryName: draft.CategoryName,
		ImageURL:     draft.ImageURL,
	}
}

func (s *parseService) enrichOriginalDraftFromERP(
	ctx context.Context,
	draft *SingleTaskDraft,
	rowNumber int,
	fields []FieldSpec,
	lookup ERPProductLookup,
) ([]ParseViolation, *domain.AppError) {
	skuField := fieldByKey(fields)["sku_code"]
	skuCode := strings.TrimSpace(draft.SKUCode)
	if skuCode == "" {
		return nil, nil
	}
	if lookup == nil {
		return []ParseViolation{{
			Row:     rowNumber,
			Column:  skuField.Column,
			Code:    "erp_lookup_failed",
			Message: "ERP product lookup is not configured",
		}}, nil
	}

	resp, appErr := lookup.SearchProducts(ctx, domain.ERPProductSearchFilter{
		SKUCode:  skuCode,
		Page:     1,
		PageSize: 50,
	})
	if appErr != nil {
		return []ParseViolation{{
			Row:     rowNumber,
			Column:  skuField.Column,
			Code:    "erp_lookup_failed",
			Message: appErr.Message,
		}}, nil
	}

	items := []*domain.ERPProduct{}
	if resp != nil {
		items = resp.Items
	}
	matches := erpProductsMatchingSKU(items, skuCode)
	if len(matches) == 0 {
		return []ParseViolation{{
			Row:     rowNumber,
			Column:  skuField.Column,
			Code:    "product_not_found",
			Message: "no ERP product found for the given SKU code",
		}}, nil
	}
	if len(matches) > 1 {
		return []ParseViolation{{
			Row:     rowNumber,
			Column:  skuField.Column,
			Code:    "ambiguous_product",
			Message: "multiple ERP products match the given SKU code",
		}}, nil
	}

	applyERPProductToOriginalDraft(draft, matches[0])
	return nil, nil
}
