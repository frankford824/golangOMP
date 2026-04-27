package service

import (
	"context"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type CategoryERPMappingFilter struct {
	Keyword         string
	CategoryID      *int64
	CategoryCode    string
	SearchEntryCode string
	ERPMatchType    *domain.CategoryERPMatchType
	IsActive        *bool
	IsPrimary       *bool
	Source          string
	Page            int
	PageSize        int
}

type CategoryERPMappingSearchFilter struct {
	Keyword         string
	CategoryCode    string
	SearchEntryCode string
	ERPMatchType    *domain.CategoryERPMatchType
	IsActive        *bool
	Limit           int
}

type CreateCategoryERPMappingParams struct {
	CategoryID              *int64
	CategoryCode            string
	SearchEntryCode         string
	ERPMatchType            domain.CategoryERPMatchType
	ERPMatchValue           string
	SecondaryConditionKey   string
	SecondaryConditionValue string
	TertiaryConditionKey    string
	TertiaryConditionValue  string
	IsPrimary               *bool
	IsActive                *bool
	Priority                int
	Source                  string
	Remark                  string
}

type PatchCategoryERPMappingParams struct {
	MappingID               int64
	CategoryID              *int64
	CategoryCode            *string
	SearchEntryCode         *string
	ERPMatchType            *domain.CategoryERPMatchType
	ERPMatchValue           *string
	SecondaryConditionKey   *string
	SecondaryConditionValue *string
	TertiaryConditionKey    *string
	TertiaryConditionValue  *string
	IsPrimary               *bool
	IsActive                *bool
	Priority                *int
	Source                  *string
	Remark                  *string
}

type CategoryERPMappingService interface {
	List(ctx context.Context, filter CategoryERPMappingFilter) ([]*domain.CategoryERPMapping, domain.PaginationMeta, *domain.AppError)
	Search(ctx context.Context, filter CategoryERPMappingSearchFilter) ([]*domain.CategoryERPMapping, *domain.AppError)
	GetByID(ctx context.Context, id int64) (*domain.CategoryERPMapping, *domain.AppError)
	Create(ctx context.Context, p CreateCategoryERPMappingParams) (*domain.CategoryERPMapping, *domain.AppError)
	Patch(ctx context.Context, p PatchCategoryERPMappingParams) (*domain.CategoryERPMapping, *domain.AppError)
}

type categoryERPMappingService struct {
	mappingRepo  repo.CategoryERPMappingRepo
	categoryRepo repo.CategoryRepo
	txRunner     repo.TxRunner
}

func NewCategoryERPMappingService(mappingRepo repo.CategoryERPMappingRepo, categoryRepo repo.CategoryRepo, txRunner repo.TxRunner) CategoryERPMappingService {
	return &categoryERPMappingService{
		mappingRepo:  mappingRepo,
		categoryRepo: categoryRepo,
		txRunner:     txRunner,
	}
}

func (s *categoryERPMappingService) List(ctx context.Context, filter CategoryERPMappingFilter) ([]*domain.CategoryERPMapping, domain.PaginationMeta, *domain.AppError) {
	items, total, err := s.mappingRepo.List(ctx, repo.CategoryERPMappingListFilter{
		Keyword:         strings.TrimSpace(filter.Keyword),
		CategoryID:      filter.CategoryID,
		CategoryCode:    strings.ToUpper(strings.TrimSpace(filter.CategoryCode)),
		SearchEntryCode: strings.ToUpper(strings.TrimSpace(filter.SearchEntryCode)),
		ERPMatchType:    filter.ERPMatchType,
		IsActive:        filter.IsActive,
		IsPrimary:       filter.IsPrimary,
		Source:          strings.TrimSpace(filter.Source),
		Page:            filter.Page,
		PageSize:        filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list category ERP mappings", err)
	}
	return items, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *categoryERPMappingService) Search(ctx context.Context, filter CategoryERPMappingSearchFilter) ([]*domain.CategoryERPMapping, *domain.AppError) {
	items, err := s.mappingRepo.Search(ctx, repo.CategoryERPMappingSearchFilter{
		Keyword:         strings.TrimSpace(filter.Keyword),
		CategoryCode:    strings.ToUpper(strings.TrimSpace(filter.CategoryCode)),
		SearchEntryCode: strings.ToUpper(strings.TrimSpace(filter.SearchEntryCode)),
		ERPMatchType:    filter.ERPMatchType,
		IsActive:        filter.IsActive,
		Limit:           filter.Limit,
	})
	if err != nil {
		return nil, infraError("search category ERP mappings", err)
	}
	return items, nil
}

func (s *categoryERPMappingService) GetByID(ctx context.Context, id int64) (*domain.CategoryERPMapping, *domain.AppError) {
	item, err := s.mappingRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get category ERP mapping", err)
	}
	if item == nil {
		return nil, domain.ErrNotFound
	}
	return item, nil
}

func (s *categoryERPMappingService) Create(ctx context.Context, p CreateCategoryERPMappingParams) (*domain.CategoryERPMapping, *domain.AppError) {
	mapping, appErr := s.buildMappingDraft(ctx, 0, p.CategoryID, p.CategoryCode, p.SearchEntryCode, p.ERPMatchType, p.ERPMatchValue, p.SecondaryConditionKey, p.SecondaryConditionValue, p.TertiaryConditionKey, p.TertiaryConditionValue, p.IsPrimary, p.IsActive, p.Priority, p.Source, p.Remark)
	if appErr != nil {
		return nil, appErr
	}

	var id int64
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		createdID, err := s.mappingRepo.Create(ctx, tx, mapping)
		if err != nil {
			return err
		}
		id = createdID
		return nil
	}); err != nil {
		return nil, infraError("create category ERP mapping tx", err)
	}
	return s.GetByID(ctx, id)
}

func (s *categoryERPMappingService) Patch(ctx context.Context, p PatchCategoryERPMappingParams) (*domain.CategoryERPMapping, *domain.AppError) {
	current, err := s.mappingRepo.GetByID(ctx, p.MappingID)
	if err != nil {
		return nil, infraError("get category ERP mapping for patch", err)
	}
	if current == nil {
		return nil, domain.ErrNotFound
	}

	categoryID := current.CategoryID
	if p.CategoryID != nil {
		categoryID = p.CategoryID
	}
	categoryCode := current.CategoryCode
	if p.CategoryCode != nil {
		categoryCode = *p.CategoryCode
	}
	searchEntryCode := current.SearchEntryCode
	if p.SearchEntryCode != nil {
		searchEntryCode = *p.SearchEntryCode
	}
	matchType := current.ERPMatchType
	if p.ERPMatchType != nil {
		matchType = *p.ERPMatchType
	}
	matchValue := current.ERPMatchValue
	if p.ERPMatchValue != nil {
		matchValue = *p.ERPMatchValue
	}
	secondaryKey := current.SecondaryConditionKey
	if p.SecondaryConditionKey != nil {
		secondaryKey = *p.SecondaryConditionKey
	}
	secondaryValue := current.SecondaryConditionValue
	if p.SecondaryConditionValue != nil {
		secondaryValue = *p.SecondaryConditionValue
	}
	tertiaryKey := current.TertiaryConditionKey
	if p.TertiaryConditionKey != nil {
		tertiaryKey = *p.TertiaryConditionKey
	}
	tertiaryValue := current.TertiaryConditionValue
	if p.TertiaryConditionValue != nil {
		tertiaryValue = *p.TertiaryConditionValue
	}
	isPrimary := current.IsPrimary
	if p.IsPrimary != nil {
		isPrimary = *p.IsPrimary
	}
	isActive := current.IsActive
	if p.IsActive != nil {
		isActive = *p.IsActive
	}
	priority := current.Priority
	if p.Priority != nil {
		priority = *p.Priority
	}
	source := current.Source
	if p.Source != nil {
		source = *p.Source
	}
	remark := current.Remark
	if p.Remark != nil {
		remark = *p.Remark
	}

	mapping, appErr := s.buildMappingDraft(ctx, p.MappingID, categoryID, categoryCode, searchEntryCode, matchType, matchValue, secondaryKey, secondaryValue, tertiaryKey, tertiaryValue, &isPrimary, &isActive, priority, source, remark)
	if appErr != nil {
		return nil, appErr
	}
	mapping.MappingID = p.MappingID

	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.mappingRepo.Update(ctx, tx, mapping)
	}); err != nil {
		return nil, infraError("patch category ERP mapping tx", err)
	}
	return s.GetByID(ctx, p.MappingID)
}

func (s *categoryERPMappingService) buildMappingDraft(ctx context.Context, mappingID int64, categoryID *int64, categoryCode, searchEntryCode string, matchType domain.CategoryERPMatchType, matchValue, secondaryKey, secondaryValue, tertiaryKey, tertiaryValue string, isPrimary, isActive *bool, priority int, source, remark string) (*domain.CategoryERPMapping, *domain.AppError) {
	category, appErr := s.resolveMappingCategory(ctx, categoryID, categoryCode)
	if appErr != nil {
		return nil, appErr
	}
	if category == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_id or category_code is required", nil)
	}
	if !matchType.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "erp_match_type is required and must be supported", nil)
	}

	resolvedMatchValue := normalizeERPMatchValue(matchType, matchValue)
	if resolvedMatchValue == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "erp_match_value is required", nil)
	}

	resolvedSearchEntryCode := strings.ToUpper(strings.TrimSpace(searchEntryCode))
	if resolvedSearchEntryCode == "" {
		resolvedSearchEntryCode = category.SearchEntryCode
	}
	if resolvedSearchEntryCode == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category does not expose search_entry_code", nil)
	}
	if resolvedSearchEntryCode != category.SearchEntryCode {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "search_entry_code must match the category search entry", nil)
	}

	secondaryKey, secondaryValue, appErr = normalizeReservedCondition("secondary", secondaryKey, secondaryValue)
	if appErr != nil {
		return nil, appErr
	}
	tertiaryKey, tertiaryValue, appErr = normalizeReservedCondition("tertiary", tertiaryKey, tertiaryValue)
	if appErr != nil {
		return nil, appErr
	}

	active := true
	if isActive != nil {
		active = *isActive
	}
	primary := false
	if isPrimary != nil {
		primary = *isPrimary
	}
	if priority == 0 {
		priority = 100
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "admin_manual"
	}

	return &domain.CategoryERPMapping{
		MappingID:               mappingID,
		CategoryID:              &category.CategoryID,
		CategoryCode:            category.CategoryCode,
		SearchEntryCode:         resolvedSearchEntryCode,
		ERPMatchType:            matchType,
		ERPMatchValue:           resolvedMatchValue,
		SecondaryConditionKey:   secondaryKey,
		SecondaryConditionValue: secondaryValue,
		TertiaryConditionKey:    tertiaryKey,
		TertiaryConditionValue:  tertiaryValue,
		IsPrimary:               primary,
		IsActive:                active,
		Priority:                priority,
		Source:                  source,
		Remark:                  strings.TrimSpace(remark),
	}, nil
}

func (s *categoryERPMappingService) resolveMappingCategory(ctx context.Context, categoryID *int64, categoryCode string) (*domain.Category, *domain.AppError) {
	categoryCode = strings.ToUpper(strings.TrimSpace(categoryCode))
	if categoryID != nil {
		category, err := s.categoryRepo.GetByID(ctx, *categoryID)
		if err != nil {
			return nil, infraError("get category for ERP mapping", err)
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
		return nil, infraError("get category by code for ERP mapping", err)
	}
	if category == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_code does not exist", nil)
	}
	return category, nil
}

func normalizeERPMatchValue(matchType domain.CategoryERPMatchType, value string) string {
	value = strings.TrimSpace(value)
	switch matchType {
	case domain.CategoryERPMatchTypeCategoryCode, domain.CategoryERPMatchTypeSKUPrefix, domain.CategoryERPMatchTypeExternalID:
		return strings.ToUpper(value)
	default:
		return value
	}
}

func normalizeReservedCondition(level, key, value string) (string, string, *domain.AppError) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" && value == "" {
		return "", "", nil
	}
	if key == "" || value == "" {
		return "", "", domain.NewAppError(domain.ErrCodeInvalidRequest, level+"_condition_key and "+level+"_condition_value must be provided together", nil)
	}
	return key, value, nil
}
