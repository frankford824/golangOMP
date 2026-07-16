package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	"workflow/domain"
	"workflow/repo"
)

const (
	defaultTaskProductCodeShortLen  = 1
	defaultTaskProductCodeSeqLength = 6
	maxPrepareProductCodeCount      = 500
)

var defaultTaskProductCodeExplicitShortCodeMap = map[string]string{
	"KT_STANDARD": "K",
}

type PrepareTaskProductCodesParams struct {
	TaskType     domain.TaskType
	BusinessLane domain.TaskBusinessLane
	CategoryCode string
	SKUCodeType  domain.TaskSKUCodeType
	Count        int
	BatchItems   []PrepareTaskProductCodeBatchItemParams
}

type PrepareTaskProductCodeBatchItemParams struct {
	CategoryCode string
	SKUCodeType  domain.TaskSKUCodeType
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

type volatileProductCodeSequenceRepo struct {
	mu   sync.Mutex
	next map[string]int64
}

// newVolatileProductCodeSequenceRepo is a non-persistent fallback for unit tests
// and partial service wiring. Production cmd wiring must provide the MySQL-backed
// product_code_sequences repo so allocated product codes survive restarts.
func newVolatileProductCodeSequenceRepo() repo.ProductCodeSequenceRepo {
	return &volatileProductCodeSequenceRepo{next: make(map[string]int64)}
}

func (r *volatileProductCodeSequenceRepo) AllocateRange(_ context.Context, _ repo.Tx, prefix, categoryShortCode string, count int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := strings.ToUpper(strings.TrimSpace(prefix)) + "|" + strings.ToUpper(strings.TrimSpace(categoryShortCode))
	start := r.next[key]
	r.next[key] += int64(count)
	return start, nil
}

func (s *taskService) PrepareProductCodes(ctx context.Context, p PrepareTaskProductCodesParams) (*PrepareTaskProductCodesResult, *domain.AppError) {
	if !supportsDefaultTaskProductCode(p.TaskType) {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			"task_type must be new_product_development",
			map[string]interface{}{"task_type": p.TaskType},
		)
	}

	p.BusinessLane = normalizeTaskBusinessLaneForTaskCreate(p.BusinessLane, false)
	if p.SKUCodeType.Valid() && !taskSKUCodeTypeMatchesBusinessLane(p.SKUCodeType, p.BusinessLane) {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			"sku_code_type conflicts with business_lane",
			map[string]interface{}{
				"sku_code_type": p.SKUCodeType,
				"business_lane": p.BusinessLane,
			},
		)
	}

	if len(p.BatchItems) > 0 {
		result := make([]PreparedTaskProductCode, len(p.BatchItems))
		type batchGroupKey struct {
			categoryCode string
			skuCodeType  domain.TaskSKUCodeType
		}
		categoryIndexes := make(map[batchGroupKey][]int, len(p.BatchItems))
		for idx, item := range p.BatchItems {
			categoryCode, appErr := normalizeDefaultTaskProductCategoryCode(item.CategoryCode)
			if appErr != nil {
				appErr.Details = map[string]interface{}{
					"field": fmt.Sprintf("batch_items[%d].category_code", idx),
				}
				return nil, appErr
			}
			if item.SKUCodeType.Valid() && !taskSKUCodeTypeMatchesBusinessLane(item.SKUCodeType, p.BusinessLane) {
				return nil, domain.NewAppError(
					domain.ErrCodeInvalidRequest,
					fmt.Sprintf("batch_items[%d].sku_code_type conflicts with business_lane", idx),
					map[string]interface{}{
						"field":         fmt.Sprintf("batch_items[%d].sku_code_type", idx),
						"sku_code_type": item.SKUCodeType,
						"business_lane": p.BusinessLane,
					},
				)
			}
			skuCodeType := normalizeTaskSKUCodeTypeByBusinessLane(item.SKUCodeType, p.BusinessLane)
			key := batchGroupKey{categoryCode: categoryCode, skuCodeType: skuCodeType}
			categoryIndexes[key] = append(categoryIndexes[key], idx)
		}

		for key, indexes := range categoryIndexes {
			codes, appErr := s.generateDefaultTaskProductCodes(ctx, p.TaskType, key.categoryCode, key.skuCodeType, len(indexes))
			if appErr != nil {
				return nil, appErr
			}
			for i, idx := range indexes {
				result[idx] = PreparedTaskProductCode{
					Index:        idx,
					CategoryCode: key.categoryCode,
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
	codes, appErr := s.generateDefaultTaskProductCodes(ctx, p.TaskType, categoryCode, normalizeTaskSKUCodeTypeByBusinessLane(p.SKUCodeType, p.BusinessLane), count)
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
	return taskType == domain.TaskTypeNewProductDevelopment
}

func normalizeTaskSKUCodeType(value domain.TaskSKUCodeType, customizationRequired bool) domain.TaskSKUCodeType {
	return normalizeTaskSKUCodeTypeByBusinessLane(value, normalizeTaskBusinessLaneForTaskCreate("", customizationRequired))
}

func normalizeTaskBusinessLaneForTaskCreate(lane domain.TaskBusinessLane, customizationRequired bool) domain.TaskBusinessLane {
	return domain.NormalizeTaskBusinessLane(lane, customizationRequired)
}

func defaultTaskSKUCodeTypeForBusinessLane(lane domain.TaskBusinessLane) domain.TaskSKUCodeType {
	switch normalizeTaskBusinessLaneForTaskCreate(lane, false) {
	case domain.TaskBusinessLaneCustomization:
		return domain.TaskSKUCodeTypeCustomization
	default:
		return domain.TaskSKUCodeTypeRegular
	}
}

func normalizeTaskSKUCodeTypeByBusinessLane(value domain.TaskSKUCodeType, lane domain.TaskBusinessLane) domain.TaskSKUCodeType {
	if value.Valid() {
		return value
	}
	return defaultTaskSKUCodeTypeForBusinessLane(lane)
}

func taskSKUCodeTypeMatchesBusinessLane(value domain.TaskSKUCodeType, lane domain.TaskBusinessLane) bool {
	if !value.Valid() {
		return true
	}
	return value == defaultTaskSKUCodeTypeForBusinessLane(lane)
}

func normalizeDefaultTaskProductCategoryCode(categoryCode string) (string, *domain.AppError) {
	normalized := strings.ToUpper(strings.TrimSpace(categoryCode))
	if normalized == "" {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "category_code is required for default product-code generation", nil)
	}
	return normalized, nil
}

func formatDefaultTaskProductCode(categoryShortCode string, seq int64) string {
	return formatTaskProductCode(domain.TaskSKUCodeTypeRegular, categoryShortCode, seq)
}

func formatTaskProductCode(skuCodeType domain.TaskSKUCodeType, categoryShortCode string, seq int64) string {
	return prefixForTaskSKUCodeType(skuCodeType) + categoryShortCode + fmt.Sprintf("%0*d", defaultTaskProductCodeSeqLength, seq)
}

func prefixForTaskSKUCodeType(skuCodeType domain.TaskSKUCodeType) string {
	switch normalizeTaskSKUCodeType(skuCodeType, false) {
	case domain.TaskSKUCodeTypeCustomization:
		return "DZ"
	default:
		return "CG"
	}
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

func (s *taskService) generateDefaultTaskProductCode(ctx context.Context, taskType domain.TaskType, categoryCode string, skuCodeType domain.TaskSKUCodeType) (string, *domain.AppError) {
	codes, appErr := s.generateDefaultTaskProductCodes(ctx, taskType, categoryCode, skuCodeType, 1)
	if appErr != nil {
		return "", appErr
	}
	return codes[0], nil
}

func (s *taskService) generateDefaultTaskProductCodes(ctx context.Context, taskType domain.TaskType, categoryCode string, skuCodeType domain.TaskSKUCodeType, count int) ([]string, *domain.AppError) {
	if !supportsDefaultTaskProductCode(taskType) {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			"default product-code generation is only enabled for new_product_development",
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
	skuCodeType = normalizeTaskSKUCodeType(skuCodeType, false)
	prefix := prefixForTaskSKUCodeType(skuCodeType)

	if s.productCodeSeqRepo == nil {
		s.productCodeSeqRepo = newVolatileProductCodeSequenceRepo()
	}

	var start int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		// Allocation dimension is (prefix, category_short_code), so category_codes that
		// collapse to the same short code share one sequence and cannot collide.
		start, err = s.productCodeSeqRepo.AllocateRange(ctx, tx, prefix, categoryShortCode, count)
		return err
	})
	if txErr != nil {
		return nil, infraError("allocate default task product code", txErr)
	}

	codes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		codes = append(codes, formatTaskProductCode(skuCodeType, categoryShortCode, start+int64(i)))
	}
	return codes, nil
}
