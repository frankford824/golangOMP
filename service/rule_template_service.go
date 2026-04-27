package service

import (
	"context"
	"encoding/json"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type RuleTemplateService interface {
	List(ctx context.Context) ([]*domain.RuleTemplate, *domain.AppError)
	GetByType(ctx context.Context, templateType domain.RuleTemplateType) (*domain.RuleTemplate, *domain.AppError)
	Put(ctx context.Context, templateType domain.RuleTemplateType, configJSON string) (*domain.RuleTemplate, *domain.AppError)
}

type ruleTemplateService struct {
	repo repo.RuleTemplateRepo
}

func NewRuleTemplateService(repo repo.RuleTemplateRepo) RuleTemplateService {
	return &ruleTemplateService{repo: repo}
}

func (s *ruleTemplateService) List(ctx context.Context) ([]*domain.RuleTemplate, *domain.AppError) {
	list, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, infraError("list rule templates", err)
	}
	filtered := make([]*domain.RuleTemplate, 0, len(list))
	for _, item := range list {
		if item == nil {
			continue
		}
		if item.TemplateType == domain.RuleTemplateTypeProductCode {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered, nil
}

func (s *ruleTemplateService) GetByType(ctx context.Context, templateType domain.RuleTemplateType) (*domain.RuleTemplate, *domain.AppError) {
	if templateType == domain.RuleTemplateTypeProductCode {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rule_templates/product-code is deprecated; use default backend task product-code generation", nil)
	}
	if !validRuleTemplateType(templateType) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid template type: "+string(templateType), nil)
	}
	rt, err := s.repo.GetByType(ctx, templateType)
	if err != nil {
		return nil, infraError("get rule template", err)
	}
	if rt == nil {
		return nil, domain.ErrNotFound
	}
	return rt, nil
}

func (s *ruleTemplateService) Put(ctx context.Context, templateType domain.RuleTemplateType, configJSON string) (*domain.RuleTemplate, *domain.AppError) {
	if templateType == domain.RuleTemplateTypeProductCode {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rule_templates/product-code is deprecated and no longer configurable", nil)
	}
	if !validRuleTemplateType(templateType) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid template type: "+string(templateType), nil)
	}
	configJSON = strings.TrimSpace(configJSON)
	if configJSON == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "config_json is required", nil)
	}
	if !json.Valid([]byte(configJSON)) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "config_json must be valid JSON", nil)
	}
	rt, err := s.repo.Upsert(ctx, templateType, configJSON)
	if err != nil {
		return nil, infraError("put rule template", err)
	}
	return rt, nil
}

func validRuleTemplateType(t domain.RuleTemplateType) bool {
	switch t {
	case domain.RuleTemplateTypeCostPricing, domain.RuleTemplateTypeShortName:
		return true
	}
	return false
}
