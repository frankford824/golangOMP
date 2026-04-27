package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

// CodeRuleService manages numbering rule lookup and code generation (V7 §5).
type CodeRuleService interface {
	List(ctx context.Context) ([]*domain.CodeRule, *domain.AppError)
	Preview(ctx context.Context, ruleID int64) (*domain.CodePreview, *domain.AppError)
	GenerateCode(ctx context.Context, ruleType domain.CodeRuleType) (string, *domain.AppError)
	GenerateSKU(ctx context.Context, ruleID int64) (string, *domain.AppError)
}

type codeRuleService struct {
	codeRuleRepo repo.CodeRuleRepo
	txRunner     repo.TxRunner
}

func NewCodeRuleService(codeRuleRepo repo.CodeRuleRepo, txRunner repo.TxRunner) CodeRuleService {
	return &codeRuleService{codeRuleRepo: codeRuleRepo, txRunner: txRunner}
}

func (s *codeRuleService) List(ctx context.Context) ([]*domain.CodeRule, *domain.AppError) {
	rules, err := s.codeRuleRepo.ListAll(ctx)
	if err != nil {
		return nil, infraError("list code rules", err)
	}
	return rules, nil
}

func (s *codeRuleService) Preview(ctx context.Context, ruleID int64) (*domain.CodePreview, *domain.AppError) {
	rule, err := s.codeRuleRepo.GetByID(ctx, ruleID)
	if err != nil {
		return nil, infraError("get code rule for preview", err)
	}
	if rule == nil {
		return nil, domain.ErrNotFound
	}
	// seq=0 for preview — does NOT increment the counter (spec V7 §5.3).
	preview := buildCode(rule, 0)
	return &domain.CodePreview{RuleID: ruleID, Preview: preview, IsPreview: true}, nil
}

func (s *codeRuleService) GenerateCode(ctx context.Context, ruleType domain.CodeRuleType) (string, *domain.AppError) {
	rule, err := s.codeRuleRepo.GetEnabledByType(ctx, ruleType)
	if err != nil {
		return "", infraError("get enabled code rule", err)
	}
	if rule == nil {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest,
			fmt.Sprintf("no enabled rule found for type %q", ruleType), nil)
	}
	return s.generate(ctx, rule)
}

func (s *codeRuleService) GenerateSKU(ctx context.Context, ruleID int64) (string, *domain.AppError) {
	rule, err := s.codeRuleRepo.GetByID(ctx, ruleID)
	if err != nil {
		return "", infraError("get rule for sku generation", err)
	}
	if rule == nil {
		return "", domain.ErrNotFound
	}
	if rule.RuleType != domain.CodeRuleTypeNewSKU {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest,
			fmt.Sprintf("rule %d is type %q, not new_sku", ruleID, rule.RuleType), nil)
	}
	return s.generate(ctx, rule)
}

func (s *codeRuleService) generate(ctx context.Context, rule *domain.CodeRule) (string, *domain.AppError) {
	var code string
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		seq, err := s.codeRuleRepo.NextSeq(ctx, tx, rule.ID)
		if err != nil {
			return err
		}
		code = buildCode(rule, seq)
		return nil
	})
	if txErr != nil {
		return "", infraError("generate code tx", txErr)
	}
	return code, nil
}

// buildCode assembles a code string from the rule config and sequence number.
// Formal generation and preview differ only in seq (preview uses 0).
func buildCode(rule *domain.CodeRule, seq int64) string {
	seqLen := rule.SeqLength
	if seqLen < 1 {
		seqLen = 6
	}
	parts := []string{}
	if rule.Prefix != "" {
		parts = append(parts, rule.Prefix)
	}
	if rule.DateFormat != "" {
		parts = append(parts, time.Now().Format(rule.DateFormat))
	}
	if rule.SiteCode != "" {
		parts = append(parts, rule.SiteCode)
	}
	if rule.BizCode != "" {
		parts = append(parts, rule.BizCode)
	}
	parts = append(parts, fmt.Sprintf("%0*d", seqLen, seq))
	return strings.Join(parts, "-")
}
