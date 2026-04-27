package service

import (
	"context"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

// ProductFilter for product search queries.
type ProductFilter struct {
	Keyword         string
	Category        string
	CategoryID      *int64
	CategoryCode    string
	SearchEntryCode string
	MappingMatch    domain.ProductMappingMatchMode
	SecondaryKey    string
	SecondaryValue  string
	TertiaryKey     string
	TertiaryValue   string
	Page            int
	PageSize        int
}

// ProductService defines product master data operations (V7 §4).
type ProductService interface {
	Search(ctx context.Context, filter ProductFilter) ([]*domain.ProductSearchResult, domain.PaginationMeta, *domain.AppError)
	GetByID(ctx context.Context, id int64) (*domain.Product, *domain.AppError)
}

type productService struct {
	productRepo  repo.ProductRepo
	categoryRepo repo.CategoryRepo
	mappingRepo  repo.CategoryERPMappingRepo
}

func NewProductService(productRepo repo.ProductRepo, categoryRepo repo.CategoryRepo, mappingRepo repo.CategoryERPMappingRepo) ProductService {
	return &productService{
		productRepo:  productRepo,
		categoryRepo: categoryRepo,
		mappingRepo:  mappingRepo,
	}
}

func (s *productService) Search(ctx context.Context, filter ProductFilter) ([]*domain.ProductSearchResult, domain.PaginationMeta, *domain.AppError) {
	filter, category, appErr := s.normalizeProductFilter(ctx, filter)
	if appErr != nil {
		return nil, domain.PaginationMeta{}, appErr
	}

	mappings, appErr := s.resolveSearchMappings(ctx, category, filter)
	if appErr != nil {
		return nil, domain.PaginationMeta{}, appErr
	}
	if usesMappedSearch(filter) && len(mappings) == 0 {
		return []*domain.ProductSearchResult{}, buildPaginationMeta(filter.Page, filter.PageSize, 0), nil
	}

	products, total, err := s.productRepo.Search(ctx, repo.ProductSearchFilter{
		Keyword:      filter.Keyword,
		Category:     filter.Category,
		MappingRules: mappings,
		Page:         filter.Page,
		PageSize:     filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("search products", err)
	}

	return buildProductSearchResults(products, mappings), buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *productService) GetByID(ctx context.Context, id int64) (*domain.Product, *domain.AppError) {
	p, err := s.productRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get product", err)
	}
	if p == nil {
		return nil, domain.ErrNotFound
	}
	return p, nil
}

func (s *productService) normalizeProductFilter(ctx context.Context, filter ProductFilter) (ProductFilter, *domain.Category, *domain.AppError) {
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	filter.Category = strings.TrimSpace(filter.Category)
	filter.CategoryCode = strings.ToUpper(strings.TrimSpace(filter.CategoryCode))
	filter.SearchEntryCode = strings.ToUpper(strings.TrimSpace(filter.SearchEntryCode))
	filter.SecondaryKey = strings.TrimSpace(filter.SecondaryKey)
	filter.SecondaryValue = strings.TrimSpace(filter.SecondaryValue)
	filter.TertiaryKey = strings.TrimSpace(filter.TertiaryKey)
	filter.TertiaryValue = strings.TrimSpace(filter.TertiaryValue)

	if !filter.MappingMatch.Valid() {
		return ProductFilter{}, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "mapping_match must be one of: primary, all", nil)
	}
	if err := validateReservedQueryPair("secondary", filter.SecondaryKey, filter.SecondaryValue); err != nil {
		return ProductFilter{}, nil, err
	}
	if err := validateReservedQueryPair("tertiary", filter.TertiaryKey, filter.TertiaryValue); err != nil {
		return ProductFilter{}, nil, err
	}

	category, appErr := s.resolveSearchCategory(ctx, filter.CategoryID, filter.CategoryCode)
	if appErr != nil {
		return ProductFilter{}, nil, appErr
	}

	if category != nil {
		if filter.SearchEntryCode == "" {
			filter.SearchEntryCode = category.SearchEntryCode
		} else if category.SearchEntryCode != "" && filter.SearchEntryCode != category.SearchEntryCode {
			return ProductFilter{}, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "search_entry_code must match the category search entry", nil)
		}
	}

	if usesMappedSearch(filter) && filter.MappingMatch == "" {
		filter.MappingMatch = domain.ProductMappingMatchPrimary
	}
	if filter.MappingMatch != "" && filter.SearchEntryCode == "" {
		return ProductFilter{}, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "mapping_match requires category_id, category_code, or search_entry_code", nil)
	}

	return filter, category, nil
}

func (s *productService) resolveSearchMappings(ctx context.Context, category *domain.Category, filter ProductFilter) ([]*domain.CategoryERPMapping, *domain.AppError) {
	if filter.SearchEntryCode == "" {
		return nil, nil
	}

	items, err := s.mappingRepo.ListActiveBySearchEntry(ctx, filter.SearchEntryCode)
	if err != nil {
		return nil, infraError("list product search mappings", err)
	}

	items = filterMappingsByConditions(items, filter.SecondaryKey, filter.SecondaryValue, filter.TertiaryKey, filter.TertiaryValue)
	switch filter.MappingMatch {
	case domain.ProductMappingMatchPrimary:
		items = filterPrimaryMappings(items)
	case domain.ProductMappingMatchAll:
	}

	if category != nil {
		exact := filterMappingsByCategory(items, category.CategoryCode)
		if len(exact) > 0 {
			return exact, nil
		}
	}

	return items, nil
}

func (s *productService) resolveSearchCategory(ctx context.Context, categoryID *int64, categoryCode string) (*domain.Category, *domain.AppError) {
	if categoryID != nil {
		category, err := s.categoryRepo.GetByID(ctx, *categoryID)
		if err != nil {
			return nil, infraError("get product search category", err)
		}
		if category == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_id does not exist", nil)
		}
		if categoryCode != "" && category.CategoryCode != categoryCode {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_id and category_code do not refer to the same category", nil)
		}
		return category, nil
	}
	if categoryCode == "" {
		return nil, nil
	}

	category, err := s.categoryRepo.GetByCode(ctx, categoryCode)
	if err != nil {
		return nil, infraError("get product search category by code", err)
	}
	if category == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_code does not exist", nil)
	}
	return category, nil
}

func validateReservedQueryPair(prefix, key, value string) *domain.AppError {
	if key == "" && value == "" {
		return nil
	}
	if key == "" || value == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, prefix+"_key and "+prefix+"_value must be provided together", nil)
	}
	return nil
}

func usesMappedSearch(filter ProductFilter) bool {
	return filter.SearchEntryCode != "" || filter.CategoryID != nil || filter.CategoryCode != "" || filter.SecondaryKey != "" || filter.TertiaryKey != ""
}

func filterMappingsByConditions(items []*domain.CategoryERPMapping, secondaryKey, secondaryValue, tertiaryKey, tertiaryValue string) []*domain.CategoryERPMapping {
	filtered := make([]*domain.CategoryERPMapping, 0, len(items))
	for _, item := range items {
		if !matchesReservedCondition(item.SecondaryConditionKey, item.SecondaryConditionValue, secondaryKey, secondaryValue) {
			continue
		}
		if !matchesReservedCondition(item.TertiaryConditionKey, item.TertiaryConditionValue, tertiaryKey, tertiaryValue) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterPrimaryMappings(items []*domain.CategoryERPMapping) []*domain.CategoryERPMapping {
	filtered := make([]*domain.CategoryERPMapping, 0, len(items))
	for _, item := range items {
		if item != nil && item.IsPrimary {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterMappingsByCategory(items []*domain.CategoryERPMapping, categoryCode string) []*domain.CategoryERPMapping {
	filtered := make([]*domain.CategoryERPMapping, 0, len(items))
	for _, item := range items {
		if item != nil && item.CategoryCode == categoryCode {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func matchesReservedCondition(mappingKey, mappingValue, requestedKey, requestedValue string) bool {
	if requestedKey == "" && requestedValue == "" {
		return true
	}
	if mappingKey != requestedKey {
		return false
	}
	return matchesReservedValue(mappingValue, requestedValue)
}

func matchesReservedValue(mappingValue, requestedValue string) bool {
	if mappingValue == "" {
		return false
	}
	for _, token := range strings.Split(mappingValue, "|") {
		if strings.EqualFold(strings.TrimSpace(token), strings.TrimSpace(requestedValue)) {
			return true
		}
	}
	return false
}

func buildProductSearchResults(products []*domain.Product, mappings []*domain.CategoryERPMapping) []*domain.ProductSearchResult {
	if len(products) == 0 {
		return []*domain.ProductSearchResult{}
	}

	results := make([]*domain.ProductSearchResult, 0, len(products))
	for _, product := range products {
		if product == nil {
			continue
		}
		result := &domain.ProductSearchResult{Product: *product}
		if matched := matchProductAgainstMappings(product, mappings); matched != nil {
			result.MatchedCategoryCode = matched.CategoryCode
			result.MatchedSearchEntryCode = matched.SearchEntryCode
			result.MatchedMappingRule = &domain.ProductSearchMatchedMapping{
				MappingID:               matched.MappingID,
				CategoryCode:            matched.CategoryCode,
				SearchEntryCode:         matched.SearchEntryCode,
				ERPMatchType:            matched.ERPMatchType,
				ERPMatchValue:           matched.ERPMatchValue,
				SecondaryConditionKey:   matched.SecondaryConditionKey,
				SecondaryConditionValue: matched.SecondaryConditionValue,
				TertiaryConditionKey:    matched.TertiaryConditionKey,
				TertiaryConditionValue:  matched.TertiaryConditionValue,
				IsPrimary:               matched.IsPrimary,
				Priority:                matched.Priority,
			}
		}
		results = append(results, result)
	}
	return results
}

func matchProductAgainstMappings(product *domain.Product, mappings []*domain.CategoryERPMapping) *domain.CategoryERPMapping {
	for _, mapping := range mappings {
		if productMatchesMapping(product, mapping) {
			return mapping
		}
	}
	return nil
}

func productMatchesMapping(product *domain.Product, mapping *domain.CategoryERPMapping) bool {
	if product == nil || mapping == nil {
		return false
	}

	switch mapping.ERPMatchType {
	case domain.CategoryERPMatchTypeCategoryCode:
		return strings.EqualFold(strings.TrimSpace(product.Category), strings.TrimSpace(mapping.ERPMatchValue))
	case domain.CategoryERPMatchTypeProductFamily, domain.CategoryERPMatchTypeKeyword:
		return containsFold(product.Category, mapping.ERPMatchValue) ||
			containsFold(product.ProductName, mapping.ERPMatchValue) ||
			containsFold(product.SpecJSON, mapping.ERPMatchValue)
	case domain.CategoryERPMatchTypeSKUPrefix:
		return strings.HasPrefix(strings.ToUpper(product.SKUCode), strings.ToUpper(strings.TrimSpace(mapping.ERPMatchValue)))
	case domain.CategoryERPMatchTypeExternalID:
		return strings.EqualFold(strings.TrimSpace(product.ERPProductID), strings.TrimSpace(mapping.ERPMatchValue))
	default:
		return false
	}
}

func containsFold(text, part string) bool {
	return strings.Contains(strings.ToUpper(text), strings.ToUpper(strings.TrimSpace(part)))
}
