package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

func TestTaskHandlerPrepareProductCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServicePrepareStub{
		taskServiceCaptureStub: &taskServiceCaptureStub{},
		prepareResult: &service.PrepareTaskProductCodesResult{
			Codes: []service.PreparedTaskProductCode{
				{Index: 0, CategoryCode: "KT", SKUCode: "NSKT000000"},
				{Index: 1, CategoryCode: "KT", SKUCode: "NSKT000001"},
			},
		},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks/prepare-product-codes", handler.PrepareProductCodes)

	body := map[string]interface{}{
		"task_type":     "new_product_development",
		"category_code": "KT",
		"count":         2,
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/prepare-product-codes", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/tasks/prepare-product-codes code=%d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.prepareParams.TaskType != domain.TaskTypeNewProductDevelopment || taskSvc.prepareParams.CategoryCode != "KT" || taskSvc.prepareParams.Count != 2 {
		t.Fatalf("prepare params=%+v", taskSvc.prepareParams)
	}
	var resp struct {
		Data service.PrepareTaskProductCodesResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error=%v body=%s", err, rec.Body.String())
	}
	if len(resp.Data.Codes) != 2 || resp.Data.Codes[0].SKUCode != "NSKT000000" {
		t.Fatalf("response codes=%+v", resp.Data.Codes)
	}
}

type taskServicePrepareStub struct {
	*taskServiceCaptureStub
	prepareParams service.PrepareTaskProductCodesParams
	prepareResult *service.PrepareTaskProductCodesResult
	prepareErr    *domain.AppError
}

func (s *taskServicePrepareStub) PrepareProductCodes(_ context.Context, p service.PrepareTaskProductCodesParams) (*service.PrepareTaskProductCodesResult, *domain.AppError) {
	s.prepareParams = p
	if s.prepareErr != nil {
		return nil, s.prepareErr
	}
	return s.prepareResult, nil
}
