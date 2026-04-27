package service

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
)

func TestRuleTemplateServiceProductCodeDeprecated(t *testing.T) {
	now := time.Now().UTC()
	svc := NewRuleTemplateService(ruleTemplateRepoStub{
		list: []*domain.RuleTemplate{
			{ID: 1, TemplateType: domain.RuleTemplateTypeCostPricing, ConfigJSON: "{}"},
			{ID: 2, TemplateType: domain.RuleTemplateTypeProductCode, ConfigJSON: "{}"},
			{ID: 3, TemplateType: domain.RuleTemplateTypeShortName, ConfigJSON: "{}"},
		},
		getByType: map[domain.RuleTemplateType]*domain.RuleTemplate{
			domain.RuleTemplateTypeCostPricing: {ID: 1, TemplateType: domain.RuleTemplateTypeCostPricing, ConfigJSON: "{}", CreatedAt: now, UpdatedAt: now},
		},
	})

	list, appErr := svc.List(context.Background())
	if appErr != nil {
		t.Fatalf("List() unexpected error: %+v", appErr)
	}
	if len(list) != 2 {
		t.Fatalf("List() len=%d, want 2", len(list))
	}
	for _, item := range list {
		if item.TemplateType == domain.RuleTemplateTypeProductCode {
			t.Fatalf("List() should not return deprecated product-code template: %+v", item)
		}
	}

	if _, appErr := svc.GetByType(context.Background(), domain.RuleTemplateTypeProductCode); appErr == nil {
		t.Fatal("GetByType(product-code) expected error")
	}
	if _, appErr := svc.Put(context.Background(), domain.RuleTemplateTypeProductCode, `{"enabled":true}`); appErr == nil {
		t.Fatal("Put(product-code) expected error")
	}
}

type ruleTemplateRepoStub struct {
	list      []*domain.RuleTemplate
	getByType map[domain.RuleTemplateType]*domain.RuleTemplate
}

func (s ruleTemplateRepoStub) GetByType(_ context.Context, templateType domain.RuleTemplateType) (*domain.RuleTemplate, error) {
	if s.getByType == nil {
		return nil, nil
	}
	return s.getByType[templateType], nil
}

func (s ruleTemplateRepoStub) ListAll(_ context.Context) ([]*domain.RuleTemplate, error) {
	return s.list, nil
}

func (s ruleTemplateRepoStub) Upsert(_ context.Context, templateType domain.RuleTemplateType, configJSON string) (*domain.RuleTemplate, error) {
	return &domain.RuleTemplate{ID: 99, TemplateType: templateType, ConfigJSON: configJSON}, nil
}
