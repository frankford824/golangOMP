package handler

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
)

const workbenchIncomeHiddenKey = "asset_workbench.income_hidden"

// JSON responses only: do not buffer streamed originals or mutate service/DB
// objects. UseNumber preserves file IDs beyond JavaScript's safe integer range.
func incomeSafeResponse(c *gin.Context, data interface{}) interface{} {
	if !c.GetBool(workbenchIncomeHiddenKey) {
		return data
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var projected interface{}
	if decoder.Decode(&projected) != nil {
		return nil
	}
	redactIncomeJSON(projected)
	return projected
}

func incomeField(key string) bool {
	return key == "gross_total" || key == "price" || strings.Contains(key, "amount") || key == "unit_price" || strings.HasSuffix(key, "_unit_price") || strings.HasSuffix(key, "unit_price_snapshot")
}

func redactIncomeJSON(value interface{}) {
	switch node := value.(type) {
	case map[string]interface{}:
		for key, value := range node {
			if incomeField(key) {
				switch value.(type) {
				case json.Number:
					node[key] = json.Number("0")
				case string:
					node[key] = ""
				default:
					redactIncomeJSON(value)
				}
			} else {
				redactIncomeJSON(value)
			}
		}
	case []interface{}:
		for _, item := range node {
			redactIncomeJSON(item)
		}
	}
}
