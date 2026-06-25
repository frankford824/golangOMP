package service

import (
	"context"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestCodeRuleServiceArchivesLegacyNewSKU(t *testing.T) {
	svc := NewCodeRuleService(&codeRuleArchiveRepo{}, productCodeTestTxRunner{})

	if code, appErr := svc.GenerateCode(context.Background(), domain.CodeRuleTypeNewSKU); appErr == nil {
		t.Fatalf("GenerateCode(new_sku) code=%q, want archived error", code)
	}
	if code, appErr := svc.GenerateSKU(context.Background(), 2); appErr == nil {
		t.Fatalf("GenerateSKU() code=%q, want archived error", code)
	}
	if preview, appErr := svc.Preview(context.Background(), 2); appErr == nil {
		t.Fatalf("Preview(new_sku) preview=%+v, want archived error", preview)
	}

	rules, appErr := svc.List(context.Background())
	if appErr != nil {
		t.Fatalf("List() unexpected error: %+v", appErr)
	}
	for _, rule := range rules {
		if rule.RuleType == domain.CodeRuleTypeNewSKU {
			t.Fatalf("List() leaked archived new_sku rule: %+v", rule)
		}
	}
}

type codeRuleArchiveRepo struct{}

func (codeRuleArchiveRepo) GetByID(_ context.Context, id int64) (*domain.CodeRule, error) {
	return &domain.CodeRule{ID: id, RuleType: domain.CodeRuleTypeNewSKU, RuleName: "Default New SKU Rule", Prefix: "SKU", SeqLength: 6}, nil
}

func (codeRuleArchiveRepo) GetEnabledByType(_ context.Context, ruleType domain.CodeRuleType) (*domain.CodeRule, error) {
	return &domain.CodeRule{ID: 1, RuleType: ruleType, RuleName: "Rule", Prefix: "RW", SeqLength: 6}, nil
}

func (codeRuleArchiveRepo) ListAll(context.Context) ([]*domain.CodeRule, error) {
	return []*domain.CodeRule{
		{ID: 1, RuleType: domain.CodeRuleTypeTaskNo, RuleName: "Default Task No Rule", Prefix: "RW", SeqLength: 6},
		{ID: 2, RuleType: domain.CodeRuleTypeNewSKU, RuleName: "Default New SKU Rule", Prefix: "SKU", SeqLength: 6},
	}, nil
}

func (codeRuleArchiveRepo) NextSeq(context.Context, repo.Tx, int64) (int64, error) {
	return 1, nil
}
