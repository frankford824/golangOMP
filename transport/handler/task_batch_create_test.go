package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

func TestTaskHandlerCreateParsesBatchItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
		readResult:   &domain.TaskReadModel{Task: domain.Task{ID: 1}, ReferenceFileRefs: []domain.ReferenceFileRef{}},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":      "new_product_development",
		"creator_id":     9,
		"owner_team":     "总经办组",
		"due_at":         "2026-04-01T00:00:00Z",
		"batch_sku_mode": "multiple",
		"batch_items": []map[string]interface{}{
			{
				"product_name":       "Batch A",
				"product_short_name": "A",
				"category_code":      "LIGHTBOX",
				"material_mode":      "preset",
				"design_requirement": "need design A",
			},
			{
				"product_name":       "Batch B",
				"product_short_name": "B",
				"category_code":      "LIGHTBOX",
				"material_mode":      "other",
				"design_requirement": "need design B",
				"variant_json": map[string]interface{}{
					"color": "red",
				},
			},
		},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks code = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.createParams.BatchSKUMode != "multiple" {
		t.Fatalf("batch_sku_mode = %q, want multiple", taskSvc.createParams.BatchSKUMode)
	}
	if len(taskSvc.createParams.BatchItems) != 2 {
		t.Fatalf("batch_items len = %d, want 2", len(taskSvc.createParams.BatchItems))
	}
	if taskSvc.createParams.BatchItems[1].ProductName != "Batch B" {
		t.Fatalf("batch_items[1] = %+v", taskSvc.createParams.BatchItems[1])
	}
}

func TestTaskHandlerCreateBatchErrorIncludesViolations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		appErr: domain.NewAppError(domain.ErrCodeInvalidRequest, "batch task validation failed", map[string]interface{}{
			"violations": []map[string]interface{}{
				{
					"field":   "batch_items",
					"code":    "insufficient_batch_items",
					"message": "batch_items must contain at least 2 items when batch_sku_mode=multiple",
				},
			},
		}),
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":      "purchase_task",
		"creator_id":     9,
		"owner_team":     "采购仓储组",
		"due_at":         "2026-04-01T00:00:00Z",
		"batch_sku_mode": "multiple",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/tasks code = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Error struct {
			Details map[string]interface{} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v body=%s", err, rec.Body.String())
	}
	if _, ok := resp.Error.Details["violations"]; !ok {
		t.Fatalf("details.violations missing: %+v", resp.Error.Details)
	}
}

func TestTaskHandlerCreateBatchResponseIncludesSKUItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
		readResult: &domain.TaskReadModel{
			Task: domain.Task{
				ID:                  1,
				SKUCode:             "SKU-001",
				PrimarySKUCode:      "SKU-001",
				IsBatchTask:         true,
				BatchItemCount:      2,
				BatchMode:           domain.TaskBatchModeMultiSKU,
				SKUGenerationStatus: domain.TaskSKUGenerationStatusCompleted,
			},
			ReferenceFileRefs: []domain.ReferenceFileRef{},
			SKUItems: []*domain.TaskSKUItem{
				{ID: 1, SequenceNo: 1, SKUCode: "SKU-001", SKUStatus: domain.TaskSKUStatusGenerated, ProductNameSnapshot: "Batch A"},
				{ID: 2, SequenceNo: 2, SKUCode: "SKU-002", SKUStatus: domain.TaskSKUStatusGenerated, ProductNameSnapshot: "Batch B"},
			},
		},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":      "new_product_development",
		"creator_id":     9,
		"owner_team":     "总经办组",
		"due_at":         "2026-04-01T00:00:00Z",
		"batch_sku_mode": "multiple",
		"batch_items": []map[string]interface{}{
			{"product_name": "Batch A", "product_short_name": "A", "category_code": "LIGHTBOX", "material_mode": "preset", "design_requirement": "need design A"},
			{"product_name": "Batch B", "product_short_name": "B", "category_code": "LIGHTBOX", "material_mode": "preset", "design_requirement": "need design B"},
		},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks code = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			IsBatchTask    bool `json:"is_batch_task"`
			BatchItemCount int  `json:"batch_item_count"`
			SKUItems       []struct {
				SKUCode string `json:"sku_code"`
			} `json:"sku_items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v body=%s", err, rec.Body.String())
	}
	if !resp.Data.IsBatchTask || resp.Data.BatchItemCount != 2 {
		t.Fatalf("response batch meta = %+v", resp.Data)
	}
	if len(resp.Data.SKUItems) != 2 {
		t.Fatalf("response sku_items len = %d, want 2", len(resp.Data.SKUItems))
	}
}

var _ service.TaskService = (*taskServiceCaptureStub)(nil)
