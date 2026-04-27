package service

import (
	"context"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type CategoryFilter struct {
	Keyword      string
	CategoryType *domain.CategoryType
	ParentID     *int64
	Level        *int
	IsActive     *bool
	Source       string
	Page         int
	PageSize     int
}

type CategorySearchFilter struct {
	Keyword      string
	CategoryType *domain.CategoryType
	IsActive     *bool
	Limit        int
}

type CreateCategoryParams struct {
	CategoryCode    string
	CategoryName    string
	DisplayName     string
	ParentID        *int64
	Level           int
	SearchEntryCode string
	IsSearchEntry   *bool
	CategoryType    domain.CategoryType
	IsActive        *bool
	SortOrder       int
	Source          string
	Remark          string
}

type PatchCategoryParams struct {
	CategoryID      int64
	CategoryCode    *string
	CategoryName    *string
	DisplayName     *string
	ParentID        *int64
	Level           *int
	SearchEntryCode *string
	IsSearchEntry   *bool
	CategoryType    *domain.CategoryType
	IsActive        *bool
	SortOrder       *int
	Source          *string
	Remark          *string
}

type CategoryService interface {
	List(ctx context.Context, filter CategoryFilter) ([]*domain.Category, domain.PaginationMeta, *domain.AppError)
	Search(ctx context.Context, filter CategorySearchFilter) ([]*domain.Category, *domain.AppError)
	GetByID(ctx context.Context, id int64) (*domain.Category, *domain.AppError)
	Create(ctx context.Context, p CreateCategoryParams) (*domain.Category, *domain.AppError)
	Patch(ctx context.Context, p PatchCategoryParams) (*domain.Category, *domain.AppError)
}

type categoryService struct {
	categoryRepo repo.CategoryRepo
	txRunner     repo.TxRunner
}

func NewCategoryService(categoryRepo repo.CategoryRepo, txRunner repo.TxRunner) CategoryService {
	return &categoryService{categoryRepo: categoryRepo, txRunner: txRunner}
}

func (s *categoryService) List(ctx context.Context, filter CategoryFilter) ([]*domain.Category, domain.PaginationMeta, *domain.AppError) {
	items, total, err := s.categoryRepo.List(ctx, repo.CategoryListFilter{
		Keyword:      strings.TrimSpace(filter.Keyword),
		CategoryType: filter.CategoryType,
		ParentID:     filter.ParentID,
		Level:        filter.Level,
		IsActive:     filter.IsActive,
		Source:       strings.TrimSpace(filter.Source),
		Page:         filter.Page,
		PageSize:     filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list categories", err)
	}
	return items, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *categoryService) Search(ctx context.Context, filter CategorySearchFilter) ([]*domain.Category, *domain.AppError) {
	items, err := s.categoryRepo.Search(ctx, repo.CategorySearchFilter{
		Keyword:      strings.TrimSpace(filter.Keyword),
		CategoryType: filter.CategoryType,
		IsActive:     filter.IsActive,
		Limit:        filter.Limit,
	})
	if err != nil {
		return nil, infraError("search categories", err)
	}
	return items, nil
}

func (s *categoryService) GetByID(ctx context.Context, id int64) (*domain.Category, *domain.AppError) {
	category, err := s.categoryRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get category", err)
	}
	if category == nil {
		return nil, domain.ErrNotFound
	}
	return category, nil
}

func (s *categoryService) Create(ctx context.Context, p CreateCategoryParams) (*domain.Category, *domain.AppError) {
	category, appErr := s.buildCategoryDraft(ctx, nil, p.CategoryCode, p.CategoryName, p.DisplayName, p.ParentID, p.Level, p.SearchEntryCode, p.IsSearchEntry, p.CategoryType, p.IsActive, p.SortOrder, p.Source, p.Remark)
	if appErr != nil {
		return nil, appErr
	}

	var id int64
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		createdID, err := s.categoryRepo.Create(ctx, tx, category)
		if err != nil {
			return err
		}
		id = createdID
		return nil
	}); err != nil {
		return nil, infraError("create category tx", err)
	}
	return s.GetByID(ctx, id)
}

func (s *categoryService) Patch(ctx context.Context, p PatchCategoryParams) (*domain.Category, *domain.AppError) {
	current, err := s.categoryRepo.GetByID(ctx, p.CategoryID)
	if err != nil {
		return nil, infraError("get category for patch", err)
	}
	if current == nil {
		return nil, domain.ErrNotFound
	}

	code := current.CategoryCode
	if p.CategoryCode != nil {
		code = *p.CategoryCode
	}
	name := current.CategoryName
	if p.CategoryName != nil {
		name = *p.CategoryName
	}
	displayName := current.DisplayName
	if p.DisplayName != nil {
		displayName = *p.DisplayName
	}
	parentID := current.ParentID
	if p.ParentID != nil {
		parentID = p.ParentID
	}
	level := current.Level
	if p.Level != nil {
		level = *p.Level
	}
	searchEntryCode := current.SearchEntryCode
	if p.SearchEntryCode != nil {
		searchEntryCode = *p.SearchEntryCode
	}
	isSearchEntry := current.IsSearchEntry
	if p.IsSearchEntry != nil {
		isSearchEntry = *p.IsSearchEntry
	}
	categoryType := current.CategoryType
	if p.CategoryType != nil {
		categoryType = *p.CategoryType
	}
	isActive := current.IsActive
	if p.IsActive != nil {
		isActive = *p.IsActive
	}
	sortOrder := current.SortOrder
	if p.SortOrder != nil {
		sortOrder = *p.SortOrder
	}
	source := current.Source
	if p.Source != nil {
		source = *p.Source
	}
	remark := current.Remark
	if p.Remark != nil {
		remark = *p.Remark
	}

	category, appErr := s.buildCategoryDraft(ctx, &p.CategoryID, code, name, displayName, parentID, level, searchEntryCode, &isSearchEntry, categoryType, &isActive, sortOrder, source, remark)
	if appErr != nil {
		return nil, appErr
	}
	category.CategoryID = p.CategoryID

	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.categoryRepo.Update(ctx, tx, category)
	}); err != nil {
		return nil, infraError("patch category tx", err)
	}
	return s.GetByID(ctx, p.CategoryID)
}

func (s *categoryService) buildCategoryDraft(ctx context.Context, categoryID *int64, code, name, displayName string, parentID *int64, level int, searchEntryCode string, isSearchEntry *bool, categoryType domain.CategoryType, isActive *bool, sortOrder int, source, remark string) (*domain.Category, *domain.AppError) {
	code = strings.ToUpper(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	displayName = strings.TrimSpace(displayName)
	searchEntryCode = strings.ToUpper(strings.TrimSpace(searchEntryCode))
	source = strings.TrimSpace(source)
	remark = strings.TrimSpace(remark)

	if code == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_code is required", nil)
	}
	if name == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_name is required", nil)
	}
	if displayName == "" {
		displayName = name
	}
	if !categoryType.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_type is required and must be supported", nil)
	}

	existing, err := s.categoryRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, infraError("check category code", err)
	}
	if existing != nil && (categoryID == nil || existing.CategoryID != *categoryID) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_code already exists", nil)
	}

	normalizedLevel := level
	resolvedSearchEntryCode := searchEntryCode
	resolvedIsSearchEntry := false
	if parentID != nil {
		parent, err := s.categoryRepo.GetByID(ctx, *parentID)
		if err != nil {
			return nil, infraError("get parent category", err)
		}
		if parent == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "parent_id does not exist", nil)
		}
		if categoryID != nil && *parentID == *categoryID {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "parent_id cannot equal category_id", nil)
		}
		if normalizedLevel <= 0 {
			normalizedLevel = parent.Level + 1
		}
		if resolvedSearchEntryCode == "" {
			resolvedSearchEntryCode = parent.SearchEntryCode
		}
		if isSearchEntry != nil {
			resolvedIsSearchEntry = *isSearchEntry
		}
		if resolvedSearchEntryCode == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "parent category must expose search_entry_code", nil)
		}
		if resolvedSearchEntryCode != parent.SearchEntryCode {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "child category search_entry_code must match parent search_entry_code", nil)
		}
		if resolvedIsSearchEntry {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "child category cannot be marked as is_search_entry=true", nil)
		}
	} else if normalizedLevel <= 0 {
		normalizedLevel = 1
	}
	if parentID == nil {
		if resolvedSearchEntryCode == "" {
			resolvedSearchEntryCode = code
		}
		if isSearchEntry == nil {
			resolvedIsSearchEntry = true
		} else {
			resolvedIsSearchEntry = *isSearchEntry
		}
		if resolvedSearchEntryCode != code {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "top-level category search_entry_code must equal category_code", nil)
		}
		if !resolvedIsSearchEntry {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "top-level category must be marked as is_search_entry=true", nil)
		}
	}

	active := true
	if isActive != nil {
		active = *isActive
	}
	if source == "" {
		source = "admin_manual"
	}

	return &domain.Category{
		CategoryCode:    code,
		CategoryName:    name,
		DisplayName:     displayName,
		ParentID:        parentID,
		Level:           normalizedLevel,
		SearchEntryCode: resolvedSearchEntryCode,
		IsSearchEntry:   resolvedIsSearchEntry,
		CategoryType:    categoryType,
		IsActive:        active,
		SortOrder:       sortOrder,
		Source:          source,
		Remark:          remark,
	}, nil
}
