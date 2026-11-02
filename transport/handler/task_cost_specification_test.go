package handler

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSpecificationCorrectionRejectsPriceAndBusinessFieldInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{"sku_item_id":1,"token":"x","reason":"test","cost_input":{},"cost_price":0}`, `{"sku_item_id":1,"token":"x","reason":"test","cost_input":{},"product_i_id":"other"}`, `{"sku_item_id":1,"token":"x","reason":"test","cost_input":{}}{}`, `{}`, `{"token":"x","reason":"test","cost_input":{}}`} {
		t.Run(body, func(t *testing.T) {
			router := gin.New()
			h := &TaskHandler{}
			router.PATCH("/tasks/:id/cost-specifications", h.CorrectCostSpecification)
			response := httptest.NewRecorder()
			request := httptest.NewRequest("PATCH", "/tasks/1/cost-specifications", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)
			if response.Code != 400 {
				t.Fatalf("injected body accepted: %d %s", response.Code, response.Body.String())
			}
		})
	}
}
