package service

import (
	"context"
	"strings"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestProductSearchFallsBackToSearchEntryMappingsForChildCategory(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:      1,
		CategoryCode:    "HBJ",
		CategoryName:    "HBJ",
		DisplayName:     "HBJ",
		Level:           1,
		SearchEntryCode: "HBJ",
		IsSearchEntry:   true,
		IsActive:        true,
	})
	parentID := int64(1)
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:      2,
		CategoryCode:    "HBJ_CHILD",
		CategoryName:    "HBJ Child",
		DisplayName:     "HBJ Child",
		ParentID:        &parentID,
		Level:           2,
		SearchEntryCode: "HBJ",
		IsSearchEntry:   false,
		IsActive:        true,
	})

	productRepo := &productSearchProductRepoStub{
		products: []*domain.Product{
			{ID: 1, ERPProductID: "ERP-1", SKUCode: "HBJ-001", ProductName: "HBJ Banner", Category: "HBJ"},
			{ID: 2, ERPProductID: "ERP-2", SKUCode: "OTHER-001", ProductName: "Other", Category: "OTHER"},
		},
	}
	mappingRepo := &productSearchMappingRepoStub{
		mappings: []*domain.CategoryERPMapping{
			{
				MappingID:       11,
				CategoryCode:    "HBJ",
				SearchEntryCode: "HBJ",
				ERPMatchType:    domain.CategoryERPMatchTypeCategoryCode,
				ERPMatchValue:   "HBJ",
				IsPrimary:       true,
				IsActive:        true,
				Priority:        100,
			},
		},
	}

	svc := NewProductService(productRepo, categoryRepo, mappingRepo)
	items, pagination, appErr := svc.Search(context.Background(), ProductFilter{
		CategoryCode: "HBJ_CHILD",
	})
	if appErr != nil {
		t.Fatalf("Search() unexpected error: %+v", appErr)
	}
	if pagination.Total != 1 || len(items) != 1 {
		t.Fatalf("Search() results = %d total=%d, want 1", len(items), pagination.Total)
	}
	if items[0].MatchedSearchEntryCode != "HBJ" || items[0].MatchedCategoryCode != "HBJ" {
		t.Fatalf("matched codes = %+v", items[0])
	}
	if items[0].MatchedMappingRule == nil || items[0].MatchedMappingRule.MappingID != 11 {
		t.Fatalf("matched mapping rule = %+v, want mapping_id=11", items[0].MatchedMappingRule)
	}
	if len(productRepo.lastFilter.MappingRules) != 1 || productRepo.lastFilter.MappingRules[0].MappingID != 11 {
		t.Fatalf("repo mapping rules = %+v, want top-level fallback mapping", productRepo.lastFilter.MappingRules)
	}
}

func TestProductSearchPrefersExactCategoryMappingsWhenMappingMatchAll(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:      10,
		CategoryCode:    "A4_PRINT",
		CategoryName:    "A4 Print",
		DisplayName:     "A4 Print",
		Level:           1,
		SearchEntryCode: "A4_PRINT",
		IsSearchEntry:   true,
		IsActive:        true,
	})
	parentID := int64(10)
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:      11,
		CategoryCode:    "A4_PRINT_DOUBLE",
		CategoryName:    "A4 Double",
		DisplayName:     "A4 Double",
		ParentID:        &parentID,
		Level:           2,
		SearchEntryCode: "A4_PRINT",
		IsSearchEntry:   false,
		IsActive:        true,
	})

	productRepo := &productSearchProductRepoStub{
		products: []*domain.Product{
			{ID: 3, ERPProductID: "ERP-3", SKUCode: "A4-DBL-001", ProductName: "A4 Double Poster", Category: "print"},
			{ID: 4, ERPProductID: "ERP-4", SKUCode: "A4-SGL-001", ProductName: "A4 Single Poster", Category: "print"},
		},
	}
	mappingRepo := &productSearchMappingRepoStub{
		mappings: []*domain.CategoryERPMapping{
			{
				MappingID:       21,
				CategoryCode:    "A4_PRINT",
				SearchEntryCode: "A4_PRINT",
				ERPMatchType:    domain.CategoryERPMatchTypeKeyword,
				ERPMatchValue:   "A4",
				IsPrimary:       true,
				IsActive:        true,
				Priority:        100,
			},
			{
				MappingID:               22,
				CategoryCode:            "A4_PRINT_DOUBLE",
				SearchEntryCode:         "A4_PRINT",
				ERPMatchType:            domain.CategoryERPMatchTypeKeyword,
				ERPMatchValue:           "Double",
				SecondaryConditionKey:   "print_side",
				SecondaryConditionValue: "double",
				IsPrimary:               false,
				IsActive:                true,
				Priority:                110,
			},
		},
	}

	svc := NewProductService(productRepo, categoryRepo, mappingRepo)
	items, pagination, appErr := svc.Search(context.Background(), ProductFilter{
		CategoryCode:   "A4_PRINT_DOUBLE",
		MappingMatch:   domain.ProductMappingMatchAll,
		SecondaryKey:   "print_side",
		SecondaryValue: "double",
	})
	if appErr != nil {
		t.Fatalf("Search() unexpected error: %+v", appErr)
	}
	if pagination.Total != 1 || len(items) != 1 {
		t.Fatalf("Search() results = %d total=%d, want 1", len(items), pagination.Total)
	}
	if items[0].MatchedMappingRule == nil || items[0].MatchedMappingRule.MappingID != 22 {
		t.Fatalf("matched mapping rule = %+v, want mapping_id=22", items[0].MatchedMappingRule)
	}
	if len(productRepo.lastFilter.MappingRules) != 1 || productRepo.lastFilter.MappingRules[0].MappingID != 22 {
		t.Fatalf("repo mapping rules = %+v, want exact category mapping only", productRepo.lastFilter.MappingRules)
	}
}

func TestProductSearchRejectsMismatchedSearchEntryCode(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:      20,
		CategoryCode:    "HBJ_CHILD",
		CategoryName:    "HBJ Child",
		DisplayName:     "HBJ Child",
		Level:           2,
		SearchEntryCode: "HBJ",
		IsSearchEntry:   false,
		IsActive:        true,
	})

	svc := NewProductService(&productSearchProductRepoStub{}, categoryRepo, &productSearchMappingRepoStub{})
	_, _, appErr := svc.Search(context.Background(), ProductFilter{
		CategoryCode:    "HBJ_CHILD",
		SearchEntryCode: "WRONG",
	})
	if appErr == nil || !strings.Contains(appErr.Message, "search_entry_code") {
		t.Fatalf("Search() error = %+v, want search_entry_code validation", appErr)
	}
}

type productSearchProductRepoStub struct {
	products    []*domain.Product
	lastFilter  repo.ProductSearchFilter
	lastKeyword string
}

func (r *productSearchProductRepoStub) GetByID(_ context.Context, id int64) (*domain.Product, error) {
	for _, product := range r.products {
		if product.ID == id {
			copyProduct := *product
			return &copyProduct, nil
		}
	}
	return nil, nil
}

func (r *productSearchProductRepoStub) GetByERPProductID(_ context.Context, erpProductID string) (*domain.Product, error) {
	for _, product := range r.products {
		if product.ERPProductID == erpProductID {
			copyProduct := *product
			return &copyProduct, nil
		}
	}
	return nil, nil
}

func (r *productSearchProductRepoStub) Search(_ context.Context, filter repo.ProductSearchFilter) ([]*domain.Product, int64, error) {
	r.lastFilter = filter
	items := make([]*domain.Product, 0, len(r.products))
	for _, product := range r.products {
		if filter.Keyword != "" && !containsFold(product.ProductName, filter.Keyword) && !containsFold(product.SKUCode, filter.Keyword) {
			continue
		}
		if filter.Category != "" && !containsFold(product.Category, filter.Category) {
			continue
		}
		if len(filter.MappingRules) > 0 && matchProductAgainstMappings(product, filter.MappingRules) == nil {
			continue
		}
		copyProduct := *product
		items = append(items, &copyProduct)
	}
	return items, int64(len(items)), nil
}

func (r *productSearchProductRepoStub) UpsertBatch(_ context.Context, _ repo.Tx, products []*domain.Product) (int64, error) {
	r.products = append(r.products, products...)
	return int64(len(products)), nil
}

type productSearchMappingRepoStub struct {
	mappings []*domain.CategoryERPMapping
}

func (r *productSearchMappingRepoStub) GetByID(_ context.Context, id int64) (*domain.CategoryERPMapping, error) {
	for _, item := range r.mappings {
		if item.MappingID == id {
			copyItem := *item
			return &copyItem, nil
		}
	}
	return nil, nil
}

func (r *productSearchMappingRepoStub) List(_ context.Context, _ repo.CategoryERPMappingListFilter) ([]*domain.CategoryERPMapping, int64, error) {
	items := make([]*domain.CategoryERPMapping, 0, len(r.mappings))
	for _, item := range r.mappings {
		copyItem := *item
		items = append(items, &copyItem)
	}
	return items, int64(len(items)), nil
}

func (r *productSearchMappingRepoStub) Search(_ context.Context, _ repo.CategoryERPMappingSearchFilter) ([]*domain.CategoryERPMapping, error) {
	items := make([]*domain.CategoryERPMapping, 0, len(r.mappings))
	for _, item := range r.mappings {
		copyItem := *item
		items = append(items, &copyItem)
	}
	return items, nil
}

func (r *productSearchMappingRepoStub) ListActiveByCategory(_ context.Context, categoryID *int64, categoryCode string) ([]*domain.CategoryERPMapping, error) {
	var items []*domain.CategoryERPMapping
	for _, item := range r.mappings {
		if !item.IsActive {
			continue
		}
		if categoryID != nil && item.CategoryID != nil && *item.CategoryID == *categoryID {
			copyItem := *item
			items = append(items, &copyItem)
			continue
		}
		if item.CategoryCode == categoryCode {
			copyItem := *item
			items = append(items, &copyItem)
		}
	}
	return items, nil
}

func (r *productSearchMappingRepoStub) ListActiveBySearchEntry(_ context.Context, searchEntryCode string) ([]*domain.CategoryERPMapping, error) {
	var items []*domain.CategoryERPMapping
	for _, item := range r.mappings {
		if !item.IsActive || item.SearchEntryCode != searchEntryCode {
			continue
		}
		copyItem := *item
		items = append(items, &copyItem)
	}
	return items, nil
}

func (r *productSearchMappingRepoStub) Create(_ context.Context, _ repo.Tx, mapping *domain.CategoryERPMapping) (int64, error) {
	copyItem := *mapping
	r.mappings = append(r.mappings, &copyItem)
	return copyItem.MappingID, nil
}

func (r *productSearchMappingRepoStub) Update(_ context.Context, _ repo.Tx, mapping *domain.CategoryERPMapping) error {
	for i, item := range r.mappings {
		if item.MappingID == mapping.MappingID {
			copyItem := *mapping
			r.mappings[i] = &copyItem
			return nil
		}
	}
	copyItem := *mapping
	r.mappings = append(r.mappings, &copyItem)
	return nil
}
