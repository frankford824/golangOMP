package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"workflow/domain"
)

const ERPProductNameMaxRunes = 50

func erpProductNameLimitMessage() string {
	return fmt.Sprintf("产品名称不能超过 %d 个字符，请精简后再提交，避免同步聚水潭失败", ERPProductNameMaxRunes)
}

func erpProductNameLength(value string) int {
	return utf8.RuneCountInString(strings.TrimSpace(value))
}

func erpProductNameTooLong(value string) bool {
	return erpProductNameLength(value) > ERPProductNameMaxRunes
}

func erpProductNameLengthViolation(field string, value string) map[string]interface{} {
	return taskCreateViolation(
		field,
		"erp_product_name_too_long",
		erpProductNameLimitMessage(),
	)
}

func validateERPProductUpsertNameLength(payload domain.ERPProductUpsertPayload) *domain.AppError {
	name := firstNonEmptyString(payload.Name, payload.ProductName, payload.ProductShortName, payload.SKUCode)
	if !erpProductNameTooLong(name) {
		return nil
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, erpProductNameLimitMessage(), map[string]interface{}{
		"field":      "name",
		"code":       "erp_product_name_too_long",
		"max_length": ERPProductNameMaxRunes,
		"length":     erpProductNameLength(name),
		"message":    erpProductNameLimitMessage(),
	})
}

func validateCreateTaskERPProductNameLength(p CreateTaskParams) *domain.AppError {
	var violations []map[string]interface{}
	if isMultipleBatchTaskRequest(p) {
		for idx, item := range p.BatchItems {
			if erpProductNameTooLong(item.ProductName) {
				violations = append(violations, erpProductNameLengthViolation(fmt.Sprintf("batch_items[%d].product_name", idx), item.ProductName))
			}
		}
	} else if erpProductNameTooLong(p.ProductNameSnapshot) {
		violations = append(violations, erpProductNameLengthViolation("product_name", p.ProductNameSnapshot))
	}
	if len(violations) == 0 {
		return nil
	}
	return taskCreateValidationError(erpProductNameLimitMessage(), p, violations...)
}
