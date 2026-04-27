package service

import (
	"context"
	"strings"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestCategoryServiceChildCategoryInheritsParentSearchEntry(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:      1,
		CategoryCode:    "HBJ",
		CategoryName:    "HBJ",
		DisplayName:     "HBJ",
		Level:           1,
		SearchEntryCode: "HBJ",
		IsSearchEntry:   true,
		CategoryType:    domain.CategoryTypeCodedStyle,
		IsActive:        true,
	})

	svc := NewCategoryService(categoryRepo, noopTxRunner{}).(*categoryService)
	parentID := int64(1)
	category, appErr := svc.Create(context.Background(), CreateCategoryParams{
		CategoryCode: "HBJ_CHILD",
		CategoryName: "HBJ child",
		ParentID:     &parentID,
		CategoryType: domain.CategoryTypeCustom,
		Source:       "test",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if category.Level != 2 {
		t.Fatalf("created child level = %d, want 2", category.Level)
	}
	if category.SearchEntryCode != "HBJ" || category.IsSearchEntry {
		t.Fatalf("created child search-entry fields = %+v", category)
	}
}

func TestCategoryERPMappingServiceCreateBuildsERPMappingSkeleton(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	mappingRepo := newCategoryERPMappingRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:      10,
		CategoryCode:    "KT_STANDARD",
		CategoryName:    "常规KT板",
		DisplayName:     "常规KT板",
		Level:           1,
		SearchEntryCode: "KT_STANDARD",
		IsSearchEntry:   true,
		CategoryType:    domain.CategoryTypeBoard,
		IsActive:        true,
	})

	svc := NewCategoryERPMappingService(mappingRepo, categoryRepo, noopTxRunner{}).(*categoryERPMappingService)
	isPrimary := true
	item, appErr := svc.Create(context.Background(), CreateCategoryERPMappingParams{
		CategoryCode:            "KT_STANDARD",
		ERPMatchType:            domain.CategoryERPMatchTypeSKUPrefix,
		ERPMatchValue:           "kt-",
		SecondaryConditionKey:   "material",
		SecondaryConditionValue: "PVC",
		IsPrimary:               &isPrimary,
		Source:                  "test",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if item.CategoryCode != "KT_STANDARD" || item.SearchEntryCode != "KT_STANDARD" {
		t.Fatalf("created mapping category/search entry = %+v", item)
	}
	if item.ERPMatchType != domain.CategoryERPMatchTypeSKUPrefix || item.ERPMatchValue != "KT-" {
		t.Fatalf("created mapping ERP match = %+v", item)
	}
	if item.SecondaryConditionKey != "material" || item.SecondaryConditionValue != "PVC" {
		t.Fatalf("created mapping reserved conditions = %+v", item)
	}
	if !item.IsPrimary || !item.IsActive || item.Priority != 100 {
		t.Fatalf("created mapping flags = %+v", item)
	}
}

func TestCategoryERPMappingServiceRejectsMismatchedSearchEntry(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	mappingRepo := newCategoryERPMappingRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:      11,
		CategoryCode:    "A4_PRINT",
		CategoryName:    "A4打印",
		DisplayName:     "A4打印",
		Level:           1,
		SearchEntryCode: "A4_PRINT",
		IsSearchEntry:   true,
		CategoryType:    domain.CategoryTypePaper,
		IsActive:        true,
	})

	svc := NewCategoryERPMappingService(mappingRepo, categoryRepo, noopTxRunner{}).(*categoryERPMappingService)
	_, appErr := svc.Create(context.Background(), CreateCategoryERPMappingParams{
		CategoryCode:    "A4_PRINT",
		SearchEntryCode: "WRONG_ENTRY",
		ERPMatchType:    domain.CategoryERPMatchTypeKeyword,
		ERPMatchValue:   "打印",
	})
	if appErr == nil || !strings.Contains(appErr.Message, "search_entry_code") {
		t.Fatalf("Create() error = %+v, want search_entry_code validation", appErr)
	}
}

type categoryERPMappingRepoStub struct {
	byID   map[int64]*domain.CategoryERPMapping
	nextID int64
}

func newCategoryERPMappingRepoStub() *categoryERPMappingRepoStub {
	return &categoryERPMappingRepoStub{
		byID:   map[int64]*domain.CategoryERPMapping{},
		nextID: 1,
	}
}

func (r *categoryERPMappingRepoStub) GetByID(_ context.Context, id int64) (*domain.CategoryERPMapping, error) {
	item, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	copyItem := *item
	return &copyItem, nil
}

func (r *categoryERPMappingRepoStub) List(_ context.Context, _ repo.CategoryERPMappingListFilter) ([]*domain.CategoryERPMapping, int64, error) {
	items := make([]*domain.CategoryERPMapping, 0, len(r.byID))
	for _, item := range r.byID {
		copyItem := *item
		items = append(items, &copyItem)
	}
	return items, int64(len(items)), nil
}

func (r *categoryERPMappingRepoStub) Search(_ context.Context, _ repo.CategoryERPMappingSearchFilter) ([]*domain.CategoryERPMapping, error) {
	items := make([]*domain.CategoryERPMapping, 0, len(r.byID))
	for _, item := range r.byID {
		copyItem := *item
		items = append(items, &copyItem)
	}
	return items, nil
}

func (r *categoryERPMappingRepoStub) ListActiveByCategory(_ context.Context, categoryID *int64, categoryCode string) ([]*domain.CategoryERPMapping, error) {
	var items []*domain.CategoryERPMapping
	for _, item := range r.byID {
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

func (r *categoryERPMappingRepoStub) ListActiveBySearchEntry(_ context.Context, searchEntryCode string) ([]*domain.CategoryERPMapping, error) {
	var items []*domain.CategoryERPMapping
	for _, item := range r.byID {
		if !item.IsActive || item.SearchEntryCode != searchEntryCode {
			continue
		}
		copyItem := *item
		items = append(items, &copyItem)
	}
	return items, nil
}

func (r *categoryERPMappingRepoStub) Create(_ context.Context, _ repo.Tx, mapping *domain.CategoryERPMapping) (int64, error) {
	copyItem := *mapping
	copyItem.MappingID = r.nextID
	r.nextID++
	r.byID[copyItem.MappingID] = &copyItem
	return copyItem.MappingID, nil
}

func (r *categoryERPMappingRepoStub) Update(_ context.Context, _ repo.Tx, mapping *domain.CategoryERPMapping) error {
	copyItem := *mapping
	r.byID[copyItem.MappingID] = &copyItem
	return nil
}
