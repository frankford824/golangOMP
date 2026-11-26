package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

// sku_codes is a comma-separated OpenAPI form array on the two SKU lists.
func skuCodesQuery(c *gin.Context) ([]string, *domain.AppError) {
	raw, present := c.GetQueryArray("sku_codes")
	if !present {
		return nil, nil
	}
	if len(raw) != 1 || strings.Count(raw[0], ",") >= 200 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "请用逗号分隔编码，每次最多查询 200 个", nil)
	}
	codes := strings.Split(raw[0], ",")
	for i, code := range codes {
		codes[i] = strings.TrimSpace(code)
		if codes[i] == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "查询编码不能为空", nil)
		}
	}
	return codes, nil
}
