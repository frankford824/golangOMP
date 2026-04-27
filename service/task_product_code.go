package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

const (
	defaultTaskProductCodePrefix    = "NS"
	defaultTaskProductCodeShortLen  = 2
	defaultTaskProductCodeSeqLength = 6
	maxPrepareProductCodeCount      = 500
)

var defaultTaskProductCodeExplicitShortCodeMap = map[string]string{
	"KT_STANDARD": "KT",
}

type PrepareTaskProductCodesParams struct {
	TaskType     domain.TaskType
	CategoryCode string
	Count        int
	BatchItems   []PrepareTaskProductCodeBatchItemParams
}

type PrepareTaskProductCodeBatchItemParams struct {
	CategoryCode string
}

type PreparedTaskProductCode struct {
	Index        int    `json:"index"`
	CategoryCode string `json:"category_code"`
	SKUCode      string `json:"sku_code"`
}

type PrepareTaskProductCodesResult struct {
	Codes []PreparedTaskProductCode `json:"codes"`
}

type TaskProductCodePrepareService interface {
	PrepareProductCodes(ctx context.Context, p PrepareTaskProductCodesParams) (*PrepareTaskProductCodesResult, *domain.AppError)
}

func (s *taskService) PrepareProductCodes(ctx context.Context, p PrepareTaskProductCodesParams) (*PrepareTaskProductCodesResult, *domain.AppError) {
	if !supportsDefaultTaskProductCode(p.TaskType) {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			"task_type must be new_product_development or purchase_task",
			map[string]interface{}{"task_type": p.TaskType},
		)
	}

	if len(p.BatchItems) > 0 {
		result := make([]PreparedTaskProductCode, len(p.BatchItems))
		categoryIndexes := make(map[string][]int, len(p.BatchItems))
		for idx, item := range p.BatchItems {
			categoryCode, appErr := normalizeDefaultTaskProductCategoryCode(item.CategoryCode)
			if appErr != nil {
				appErr.Details = map[string]interface{}{
					"field": fmt.Sprintf("batch_items[%d].category_code", idx),
				}
				return nil, appErr
			}
			categoryIndexes[categoryCode] = append(categoryIndexes[categoryCode], idx)
		}

		for categoryCode, indexes := range categoryIndexes {
			codes, appErr := s.generateDefaultTaskProductCodes(ctx, p.TaskType, categoryCode, len(indexes))
			if appErr != nil {
				return nil, appErr
			}
			for i, idx := range indexes {
				result[idx] = PreparedTaskProductCode{
					Index:        idx,
					CategoryCode: categoryCode,
					SKUCode:      codes[i],
				}
			}
		}
		return &PrepareTaskProductCodesResult{Codes: result}, nil
	}

	count := p.Count
	if count == 0 {
		count = 1
	}
	if count < 1 || count > maxPrepareProductCodeCount {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			fmt.Sprintf("count must be between 1 and %d", maxPrepareProductCodeCount),
			map[string]interface{}{"count": count},
		)
	}

	categoryCode, appErr := normalizeDefaultTaskProductCategoryCode(p.CategoryCode)
	if appErr != nil {
		return nil, appErr
	}
	codes, appErr := s.generateDefaultTaskProductCodes(ctx, p.TaskType, categoryCode, count)
	if appErr != nil {
		return nil, appErr
	}

	items := make([]PreparedTaskProductCode, 0, len(codes))
	for i, code := range codes {
		items = append(items, PreparedTaskProductCode{
			Index:        i,
			CategoryCode: categoryCode,
			SKUCode:      code,
		})
	}
	return &PrepareTaskProductCodesResult{Codes: items}, nil
}

func supportsDefaultTaskProductCode(taskType domain.TaskType) bool {
	return taskType == domain.TaskTypeNewProductDevelopment || taskType == domain.TaskTypePurchaseTask
}

func normalizeDefaultTaskProductCategoryCode(categoryCode string) (string, *domain.AppError) {
	normalized := strings.ToUpper(strings.TrimSpace(categoryCode))
	if normalized == "" {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "category_code is required for default product-code generation", nil)
	}
	return normalized, nil
}

func formatDefaultTaskProductCode(categoryShortCode string, seq int64) string {
	return defaultTaskProductCodePrefix + categoryShortCode + fmt.Sprintf("%0*d", defaultTaskProductCodeSeqLength, seq)
}

func deriveDefaultTaskProductCategoryShortCode(categoryCode string) (string, *domain.AppError) {
	normalizedCategoryCode, appErr := normalizeDefaultTaskProductCategoryCode(categoryCode)
	if appErr != nil {
		return "", appErr
	}

	if explicit, ok := defaultTaskProductCodeExplicitShortCodeMap[normalizedCategoryCode]; ok {
		return explicit, nil
	}

	letters := collectUppercaseASCIILetters(normalizedCategoryCode)
	switch {
	case len(letters) >= defaultTaskProductCodeShortLen:
		return letters[:defaultTaskProductCodeShortLen], nil
	case len(letters) == 1:
		return letters + deterministicFallbackLetters(normalizedCategoryCode, 1), nil
	default:
		return deterministicFallbackLetters(normalizedCategoryCode, defaultTaskProductCodeShortLen), nil
	}
}

func collectUppercaseASCIILetters(input string) string {
	var b strings.Builder
	for _, r := range input {
		if r >= 'a' && r <= 'z' {
			r = r - 'a' + 'A'
		}
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func deterministicFallbackLetters(seed string, n int) string {
	if n <= 0 {
		return ""
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	sum := h.Sum32()

	out := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		shift := uint((i % 6) * 5)
		idx := int((sum >> shift) & 31)
		out = append(out, byte('A'+(idx%26)))
	}
	return string(out)
}

func (s *taskService) generateDefaultTaskProductCode(ctx context.Context, taskType domain.TaskType, categoryCode string) (string, *domain.AppError) {
	codes, appErr := s.generateDefaultTaskProductCodes(ctx, taskType, categoryCode, 1)
	if appErr != nil {
		return "", appErr
	}
	return codes[0], nil
}

func (s *taskService) generateDefaultTaskProductCodes(ctx context.Context, taskType domain.TaskType, categoryCode string, count int) ([]string, *domain.AppError) {
	if !supportsDefaultTaskProductCode(taskType) {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			"default product-code generation is only enabled for new_product_development and purchase_task",
			map[string]interface{}{"task_type": taskType},
		)
	}
	if count < 1 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "count must be greater than 0", nil)
	}
	normalizedCategoryCode, appErr := normalizeDefaultTaskProductCategoryCode(categoryCode)
	if appErr != nil {
		return nil, appErr
	}
	categoryShortCode, appErr := deriveDefaultTaskProductCategoryShortCode(normalizedCategoryCode)
	if appErr != nil {
		return nil, appErr
	}

	if s.productCodeSeqRepo == nil {
		// Backward-compatible fallback used only in tests or partial wiring environments.
		codes := make([]string, 0, count)
		seen := make(map[string]struct{}, count)
		for i := 0; i < count; i++ {
			code, appErr := s.codeRuleSvc.GenerateCode(ctx, domain.CodeRuleTypeNewSKU)
			if appErr != nil {
				return nil, appErr
			}
			code = strings.TrimSpace(code)
			if code == "" {
				return nil, domain.NewAppError(domain.ErrCodeInternalError, "generated sku_code is empty", nil)
			}
			if _, exists := seen[code]; exists {
				return nil, domain.NewAppError(domain.ErrCodeInternalError, "duplicate generated sku_code in one allocation", map[string]interface{}{"sku_code": code})
			}
			seen[code] = struct{}{}
			codes = append(codes, code)
		}
		return codes, nil
	}

	var start int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		// Allocation dimension is (prefix, category_short_code), so category_codes that
		// collapse to the same short code share one sequence and cannot collide.
		start, err = s.productCodeSeqRepo.AllocateRange(ctx, tx, defaultTaskProductCodePrefix, categoryShortCode, count)
		return err
	})
	if txErr != nil {
		return nil, infraError("allocate default task product code", txErr)
	}

	codes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		codes = append(codes, formatDefaultTaskProductCode(categoryShortCode, start+int64(i)))
	}
	return codes, nil
}
