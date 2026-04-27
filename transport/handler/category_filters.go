package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

func parseCategoryFilterQuery(c *gin.Context) (service.CategoryFilter, *domain.AppError) {
	filter := service.CategoryFilter{
		Keyword: c.Query("keyword"),
		Source:  c.Query("source"),
	}
	if raw := strings.TrimSpace(c.Query("category_type")); raw != "" {
		value := domain.CategoryType(raw)
		filter.CategoryType = &value
	}
	if raw := c.Query("parent_id"); raw != "" {
		value, err := parseInt64(raw)
		if err != nil {
			return service.CategoryFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "parent_id must be an integer", nil)
		}
		filter.ParentID = &value
	}
	if raw := c.Query("level"); raw != "" {
		value, err := parseInt(raw)
		if err != nil {
			return service.CategoryFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "level must be an integer", nil)
		}
		filter.Level = &value
	}
	if raw := c.Query("is_active"); raw != "" {
		value, err := parseBool(raw)
		if err != nil {
			return service.CategoryFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "is_active must be true/false/1/0", nil)
		}
		filter.IsActive = &value
	}
	if raw := c.Query("page"); raw != "" {
		value, err := parseInt(raw)
		if err != nil {
			return service.CategoryFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be an integer", nil)
		}
		filter.Page = value
	}
	if raw := c.Query("page_size"); raw != "" {
		value, err := parseInt(raw)
		if err != nil {
			return service.CategoryFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil)
		}
		filter.PageSize = value
	}
	return filter, nil
}

func parseCategorySearchQuery(c *gin.Context) (service.CategorySearchFilter, *domain.AppError) {
	filter := service.CategorySearchFilter{
		Keyword: c.Query("keyword"),
	}
	if raw := strings.TrimSpace(c.Query("category_type")); raw != "" {
		value := domain.CategoryType(raw)
		filter.CategoryType = &value
	}
	if raw := c.Query("is_active"); raw != "" {
		value, err := parseBool(raw)
		if err != nil {
			return service.CategorySearchFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "is_active must be true/false/1/0", nil)
		}
		filter.IsActive = &value
	}
	if raw := c.Query("limit"); raw != "" {
		value, err := parseInt(raw)
		if err != nil {
			return service.CategorySearchFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "limit must be an integer", nil)
		}
		filter.Limit = value
	}
	return filter, nil
}

func parseCostRuleFilterQuery(c *gin.Context) (service.CostRuleFilter, *domain.AppError) {
	filter := service.CostRuleFilter{
		CategoryCode:  c.Query("category_code"),
		ProductFamily: c.Query("product_family"),
	}
	if raw := c.Query("category_id"); raw != "" {
		value, err := parseInt64(raw)
		if err != nil {
			return service.CostRuleFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_id must be an integer", nil)
		}
		filter.CategoryID = &value
	}
	if raw := strings.TrimSpace(c.Query("rule_type")); raw != "" {
		value := domain.CostRuleType(raw)
		filter.RuleType = &value
	}
	if raw := c.Query("is_active"); raw != "" {
		value, err := parseBool(raw)
		if err != nil {
			return service.CostRuleFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "is_active must be true/false/1/0", nil)
		}
		filter.IsActive = &value
	}
	if raw := c.Query("page"); raw != "" {
		value, err := parseInt(raw)
		if err != nil {
			return service.CostRuleFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be an integer", nil)
		}
		filter.Page = value
	}
	if raw := c.Query("page_size"); raw != "" {
		value, err := parseInt(raw)
		if err != nil {
			return service.CostRuleFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil)
		}
		filter.PageSize = value
	}
	return filter, nil
}

func parseCategoryERPMappingFilterQuery(c *gin.Context) (service.CategoryERPMappingFilter, *domain.AppError) {
	filter := service.CategoryERPMappingFilter{
		Keyword:         c.Query("keyword"),
		CategoryCode:    c.Query("category_code"),
		SearchEntryCode: c.Query("search_entry_code"),
		Source:          c.Query("source"),
	}
	if raw := c.Query("category_id"); raw != "" {
		value, err := parseInt64(raw)
		if err != nil {
			return service.CategoryERPMappingFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_id must be an integer", nil)
		}
		filter.CategoryID = &value
	}
	if raw := strings.TrimSpace(c.Query("erp_match_type")); raw != "" {
		value := domain.CategoryERPMatchType(raw)
		filter.ERPMatchType = &value
	}
	if raw := c.Query("is_active"); raw != "" {
		value, err := parseBool(raw)
		if err != nil {
			return service.CategoryERPMappingFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "is_active must be true/false/1/0", nil)
		}
		filter.IsActive = &value
	}
	if raw := c.Query("is_primary"); raw != "" {
		value, err := parseBool(raw)
		if err != nil {
			return service.CategoryERPMappingFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "is_primary must be true/false/1/0", nil)
		}
		filter.IsPrimary = &value
	}
	if raw := c.Query("page"); raw != "" {
		value, err := parseInt(raw)
		if err != nil {
			return service.CategoryERPMappingFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be an integer", nil)
		}
		filter.Page = value
	}
	if raw := c.Query("page_size"); raw != "" {
		value, err := parseInt(raw)
		if err != nil {
			return service.CategoryERPMappingFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil)
		}
		filter.PageSize = value
	}
	return filter, nil
}

func parseCategoryERPMappingSearchQuery(c *gin.Context) (service.CategoryERPMappingSearchFilter, *domain.AppError) {
	filter := service.CategoryERPMappingSearchFilter{
		Keyword:         c.Query("keyword"),
		CategoryCode:    c.Query("category_code"),
		SearchEntryCode: c.Query("search_entry_code"),
	}
	if raw := strings.TrimSpace(c.Query("erp_match_type")); raw != "" {
		value := domain.CategoryERPMatchType(raw)
		filter.ERPMatchType = &value
	}
	if raw := c.Query("is_active"); raw != "" {
		value, err := parseBool(raw)
		if err != nil {
			return service.CategoryERPMappingSearchFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "is_active must be true/false/1/0", nil)
		}
		filter.IsActive = &value
	}
	if raw := c.Query("limit"); raw != "" {
		value, err := parseInt(raw)
		if err != nil {
			return service.CategoryERPMappingSearchFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "limit must be an integer", nil)
		}
		filter.Limit = value
	}
	return filter, nil
}
