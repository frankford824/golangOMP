package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

func TestTaskHandlerCreateBindsERPProductSnapshotFromJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"source_mode": "existing_product",
		"task_type":   "original_product_development",
		"creator_id":  9,
		"product_selection": map[string]interface{}{
			"source_match_type": "erp_bridge_keyword_search",
			"source_match_rule": "定制车缝",
			"erp_product": map[string]interface{}{
				"product_id":    "ERP-9001",
				"sku_id":        "SKU-9001",
				"sku_code":      "CF-9001",
				"product_name":  "定制车缝测试商品",
				"category_name": "旗帜",
				"image_url":     "https://img.example.com/9001.png",
				"price":         18.9,
			},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks code = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.createParams.ProductSelection == nil || taskSvc.createParams.ProductSelection.ERPProduct == nil {
		t.Fatalf("captured product_selection = %+v", taskSvc.createParams.ProductSelection)
	}
	if taskSvc.createParams.ProductSelection.ERPProduct.ProductID != "ERP-9001" {
		t.Fatalf("captured erp_product = %+v", taskSvc.createParams.ProductSelection.ERPProduct)
	}
	if taskSvc.createParams.ProductSelection.SourceMatchType != "erp_bridge_keyword_search" {
		t.Fatalf("captured source_match_type = %s", taskSvc.createParams.ProductSelection.SourceMatchType)
	}
}

func TestTaskHandlerCreateAcceptsStringProductIDAsERPFacadeKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"source_mode":           "existing_product",
		"task_type":             "original_product_development",
		"creator_id":            9,
		"product_id":            "ERP-STRING-9002",
		"sku_code":              "CF-9002",
		"product_name_snapshot": "ERP String Product",
		"change_request":        "string product_id binding",
		"owner_team":            "总经办组",
		"due_at":                "2026-03-20T00:00:00Z",
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks code = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.createParams.ProductID != nil {
		t.Fatalf("captured local product_id = %+v, want nil for string ERP product_id", taskSvc.createParams.ProductID)
	}
	if taskSvc.createParams.ProductSelection == nil || taskSvc.createParams.ProductSelection.ERPProduct == nil {
		t.Fatalf("captured product_selection = %+v", taskSvc.createParams.ProductSelection)
	}
	if taskSvc.createParams.ProductSelection.ERPProduct.ProductID != "ERP-STRING-9002" {
		t.Fatalf("captured erp_product = %+v", taskSvc.createParams.ProductSelection.ERPProduct)
	}
	if taskSvc.createParams.ProductSelection.ERPProduct.SKUCode != "CF-9002" {
		t.Fatalf("captured erp sku_code = %+v", taskSvc.createParams.ProductSelection.ERPProduct)
	}
	if taskSvc.createParams.ProductSelection.ERPProduct.ProductName != "ERP String Product" {
		t.Fatalf("captured erp product_name = %+v", taskSvc.createParams.ProductSelection.ERPProduct)
	}
}

func TestTaskHandlerCreateRejectsReferenceImages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":          "new_product_development",
		"source_mode":        "new_product",
		"creator_id":         9,
		"owner_team":         "鎬荤粡鍔炵粍",
		"due_at":             "2026-03-20T00:00:00Z",
		"category_code":      "LIGHTBOX",
		"material_mode":      "preset",
		"material":           "aluminum",
		"product_name":       "New Lightbox",
		"product_short_name": "Lightbox",
		"design_requirement": "need design",
		"reference_images": []string{
			testHandlerReferenceImageDataURI(64 * 1024),
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/tasks code = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error struct {
			Code    string                 `json:"code"`
			Message string                 `json:"message"`
			Details map[string]interface{} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("json.Unmarshal(error) error = %v body=%s", err, rec.Body.String())
	}
	if errResp.Error.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("error.code = %s, want %s", errResp.Error.Code, domain.ErrCodeInvalidRequest)
	}
	if errResp.Error.Message != "reference_images is no longer accepted in task creation; upload files first and use reference_file_refs" {
		t.Fatalf("error.message = %q", errResp.Error.Message)
	}
	if errResp.Error.Details["field"] != "reference_images" {
		t.Fatalf("details.field = %v", errResp.Error.Details["field"])
	}
	if errResp.Error.Details["suggestion"] != "use /v1/tasks/reference-upload and pass returned reference_file_refs objects" {
		t.Fatalf("details.suggestion = %v", errResp.Error.Details["suggestion"])
	}
}

func TestTaskHandlerCreateRejectsReferenceImagesEvenWhenEmptyArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":        "original_product_development",
		"source_mode":      "existing_product",
		"creator_id":       9,
		"owner_team":       "鎬荤粡鍔炵粍",
		"due_at":           "2026-03-20T00:00:00Z",
		"product_id":       88,
		"sku_code":         "SKU-088",
		"change_request":   "update design",
		"reference_images": []string{},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/tasks code = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error struct {
			Code    string                 `json:"code"`
			Message string                 `json:"message"`
			Details map[string]interface{} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("json.Unmarshal(error) error = %v body=%s", err, rec.Body.String())
	}
	if errResp.Error.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("error.code = %s, want %s", errResp.Error.Code, domain.ErrCodeInvalidRequest)
	}
	if errResp.Error.Message != "reference_images is no longer accepted in task creation; upload files first and use reference_file_refs" {
		t.Fatalf("error.message = %q", errResp.Error.Message)
	}
	if errResp.Error.Details["field"] != "reference_images" {
		t.Fatalf("details.field = %v", errResp.Error.Details["field"])
	}
	if errResp.Error.Details["suggestion"] != "use /v1/tasks/reference-upload and pass returned reference_file_refs objects" {
		t.Fatalf("details.suggestion = %v", errResp.Error.Details["suggestion"])
	}
}

// Case1: new_product_development without product_selection in body (sku_code from new_sku) — must not trigger
// "product_selection is only supported when source_mode is existing_product".
func TestTaskHandlerCreateNewProductDevelopmentWithoutProductSelectionSucceeds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":          "new_product_development",
		"source_mode":        "new_product",
		"creator_id":         9,
		"owner_team":         "总经办组",
		"due_at":             "2026-03-20T00:00:00Z",
		"category_code":      "LIGHTBOX",
		"material_mode":      "preset",
		"material":           "铝型材",
		"product_name":       "New Lightbox",
		"product_short_name": "Lightbox",
		"design_requirement": "need design",
		"new_sku":            "NEW-SKU-001",
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks code = %d, want 201 (Case1: new_product without product_selection must succeed) body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.createParams.ProductSelection != nil {
		t.Fatalf("Case1: ProductSelection should be nil when not in body, got %+v", taskSvc.createParams.ProductSelection)
	}
	if taskSvc.createParams.SourceMode != domain.TaskSourceModeNewProduct {
		t.Fatalf("Case1: source_mode = %s, want new_product", taskSvc.createParams.SourceMode)
	}
}

// Case2: new_product_development with explicit product_selection in body — must reject.
func TestTaskHandlerCreateNewProductDevelopmentWithProductSelectionRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":          "new_product_development",
		"source_mode":        "new_product",
		"creator_id":         9,
		"owner_team":         "总经办组",
		"due_at":             "2026-03-20T00:00:00Z",
		"category_code":      "LIGHTBOX",
		"material_mode":      "preset",
		"material":           "铝型材",
		"product_name":       "New Lightbox",
		"product_short_name": "Lightbox",
		"design_requirement": "need design",
		"product_selection":  map[string]interface{}{"selected_product_id": 88},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/tasks code = %d, want 400 (Case2: explicit product_selection with new_product must reject) body=%s", rec.Code, rec.Body.String())
	}
	var errResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err == nil {
		if errResp.Error.Message != "product_selection is only supported when source_mode is existing_product" {
			t.Fatalf("Case2: error message = %q, want product_selection is only supported when source_mode is existing_product", errResp.Error.Message)
		}
	}
}

// Case3: purchase_task without product_selection in body (purchase_sku) — must not trigger the error.
func TestTaskHandlerCreatePurchaseTaskWithoutProductSelectionSucceeds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":       "purchase_task",
		"creator_id":      9,
		"owner_team":      "总经办组",
		"due_at":          "2026-03-20T00:00:00Z",
		"purchase_sku":    "PUR-001",
		"product_name":    "Accessory Pack",
		"cost_price_mode": "template",
		"quantity":        100,
		"base_sale_price": 12.5,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks code = %d, want 201 (Case3: purchase_task without product_selection must succeed) body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.createParams.ProductSelection != nil {
		t.Fatalf("Case3: ProductSelection should be nil when not in body, got %+v", taskSvc.createParams.ProductSelection)
	}
}

func TestTaskHandlerCreateRejectsMismatchedStringProductIDAndERPSelection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"source_mode": "existing_product",
		"task_type":   "original_product_development",
		"creator_id":  9,
		"product_id":  "ERP-STRING-9003",
		"product_selection": map[string]interface{}{
			"erp_product": map[string]interface{}{
				"product_id": "ERP-OTHER-9003",
			},
		},
		"change_request": "string product_id mismatch",
		"owner_team":     "总经办组",
		"due_at":         "2026-03-20T00:00:00Z",
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/tasks code = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
}

func TestTaskHandlerCreateInfersSourceModeForOriginalProductSelection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":      "original_product_development",
		"creator_id":     9,
		"owner_team":     "总经办组",
		"due_at":         "2026-03-20T00:00:00Z",
		"change_request": "bind erp selection without source_mode",
		"product_selection": map[string]interface{}{
			"erp_product": map[string]interface{}{
				"product_id":   "ERP-9004",
				"sku_code":     "CF-9004",
				"product_name": "ERP inferred product",
			},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks code = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.createParams.SourceMode != domain.TaskSourceModeExistingProduct {
		t.Fatalf("captured source_mode = %s, want %s", taskSvc.createParams.SourceMode, domain.TaskSourceModeExistingProduct)
	}
	if taskSvc.createParams.ProductSelection == nil || taskSvc.createParams.ProductSelection.ERPProduct == nil {
		t.Fatalf("captured product_selection = %+v", taskSvc.createParams.ProductSelection)
	}
}

func TestTaskHandlerCreateCase1NewProductWithoutProductSelectionDoesNotHitExistingProductError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":          "new_product_development",
		"source_mode":        "new_product",
		"creator_id":         9,
		"product_id":         nil,
		"sku_code":           "NEW-CASE1-001",
		"product_name":       "Case1 New Product",
		"category_code":      "LIGHTBOX",
		"material_mode":      "preset",
		"material":           "铝型材",
		"product_short_name": "Case1",
		"design_requirement": "case1 design request",
		"owner_team":         "总经办组",
		"due_at":             "2026-03-20T00:00:00Z",
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks code = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.createParams.SourceMode != domain.TaskSourceModeNewProduct {
		t.Fatalf("captured source_mode = %s, want %s", taskSvc.createParams.SourceMode, domain.TaskSourceModeNewProduct)
	}
	if taskSvc.createParams.ProductSelection != nil {
		t.Fatalf("captured product_selection = %+v, want nil", taskSvc.createParams.ProductSelection)
	}
}

func TestTaskHandlerCreateCase2PurchaseTaskWithoutProductSelectionDoesNotHitExistingProductError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":       "purchase_task",
		"source_mode":     "new_product",
		"creator_id":      9,
		"purchase_sku":    "PUR-CASE2-001",
		"product_name":    "Case2 Purchase Product",
		"cost_price_mode": "template",
		"quantity":        50,
		"base_sale_price": 19.9,
		"owner_team":      "总经办组",
		"due_at":          "2026-03-20T00:00:00Z",
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks code = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.createParams.SourceMode != domain.TaskSourceModeNewProduct {
		t.Fatalf("captured source_mode = %s, want %s", taskSvc.createParams.SourceMode, domain.TaskSourceModeNewProduct)
	}
	if taskSvc.createParams.ProductSelection != nil {
		t.Fatalf("captured product_selection = %+v, want nil", taskSvc.createParams.ProductSelection)
	}
}

func TestTaskHandlerCreateCase3OriginalProductWithEffectiveProductSelectionStillWorks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":      "original_product_development",
		"source_mode":    "existing_product",
		"creator_id":     9,
		"owner_team":     "总经办组",
		"due_at":         "2026-03-20T00:00:00Z",
		"change_request": "case3 effective selection",
		"product_selection": map[string]interface{}{
			"selected_product_id":       88,
			"selected_product_name":     "Case3 Existing Product",
			"selected_product_sku_code": "CASE3-SKU-088",
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks code = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.createParams.SourceMode != domain.TaskSourceModeExistingProduct {
		t.Fatalf("captured source_mode = %s, want %s", taskSvc.createParams.SourceMode, domain.TaskSourceModeExistingProduct)
	}
	if taskSvc.createParams.ProductSelection == nil || taskSvc.createParams.ProductSelection.SelectedProductID == nil {
		t.Fatalf("captured product_selection = %+v", taskSvc.createParams.ProductSelection)
	}
	if *taskSvc.createParams.ProductSelection.SelectedProductID != 88 {
		t.Fatalf("captured selected_product_id = %v, want 88", *taskSvc.createParams.ProductSelection.SelectedProductID)
	}
}

func TestTaskCreateOriginalProductDevelopmentResponseEchoesChangeRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 1},
		readResult: &domain.TaskReadModel{
			Task:              domain.Task{ID: 1, TaskType: domain.TaskTypeOriginalProductDevelopment},
			ChangeRequest:     "ABC",
			DesignRequirement: "ABC",
			SKUItems: []*domain.TaskSKUItem{
				{TaskID: 1, SequenceNo: 1, SKUCode: "SKU-ABC", SKUStatus: domain.TaskSKUStatusGenerated, ChangeRequest: "ABC", DesignRequirement: "ABC"},
			},
			ReferenceFileRefs: []domain.ReferenceFileRef{},
		},
	}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":      "original_product_development",
		"source_mode":    "existing_product",
		"creator_id":     9,
		"owner_team":     "总经办组",
		"due_at":         "2026-03-20T00:00:00Z",
		"product_id":     88,
		"sku_code":       "SKU-ABC",
		"product_name":   "Original Product",
		"change_request": "ABC",
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks code = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			ChangeRequest     string `json:"change_request"`
			DesignRequirement string `json:"design_requirement"`
			SKUItems          []struct {
				ChangeRequest     string `json:"change_request"`
				DesignRequirement string `json:"design_requirement"`
			} `json:"sku_items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v body=%s", err, rec.Body.String())
	}
	if resp.Data.ChangeRequest != "ABC" || resp.Data.DesignRequirement != "ABC" {
		t.Fatalf("response demand fields = change_request:%q design_requirement:%q, want ABC/ABC", resp.Data.ChangeRequest, resp.Data.DesignRequirement)
	}
	if len(resp.Data.SKUItems) != 1 {
		t.Fatalf("response sku_items len = %d, want 1", len(resp.Data.SKUItems))
	}
	if resp.Data.SKUItems[0].ChangeRequest != "ABC" || resp.Data.SKUItems[0].DesignRequirement != "ABC" {
		t.Fatalf("response sku demand fields = change_request:%q design_requirement:%q, want ABC/ABC", resp.Data.SKUItems[0].ChangeRequest, resp.Data.SKUItems[0].DesignRequirement)
	}
}

type taskServiceCaptureStub struct {
	createParams service.CreateTaskParams
	createResult *domain.Task
	readResult   *domain.TaskReadModel
	appErr       *domain.AppError
}

func (s *taskServiceCaptureStub) Create(_ context.Context, p service.CreateTaskParams) (*domain.Task, *domain.AppError) {
	s.createParams = p
	return s.createResult, s.appErr
}

func (s *taskServiceCaptureStub) List(context.Context, service.TaskFilter) ([]*domain.TaskListItem, domain.PaginationMeta, *domain.AppError) {
	return nil, domain.PaginationMeta{}, nil
}

func (s *taskServiceCaptureStub) ListBoardCandidates(context.Context, service.TaskFilter, []domain.TaskQueryFilterDefinition) ([]*domain.TaskListItem, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) GetByID(context.Context, int64) (*domain.TaskReadModel, *domain.AppError) {
	return s.readResult, nil
}

func (s *taskServiceCaptureStub) GetFilingStatus(context.Context, int64) (*domain.TaskFilingStatusView, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) RetryFiling(context.Context, service.RetryTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) TriggerFiling(context.Context, service.TriggerTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) UpdateBusinessInfo(context.Context, service.UpdateTaskBusinessInfoParams) (*domain.TaskDetail, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) UpdateProcurement(context.Context, service.UpdateTaskProcurementParams) (*domain.ProcurementRecord, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) AdvanceProcurement(context.Context, service.AdvanceTaskProcurementParams) (*domain.ProcurementRecord, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) PrepareWarehouse(context.Context, service.PrepareTaskForWarehouseParams) (*domain.Task, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) Close(context.Context, service.CloseTaskParams) (*domain.TaskReadModel, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) SubmitCustomizationReview(context.Context, service.SubmitCustomizationReviewParams) (*domain.CustomizationJob, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) SubmitCustomizationEffectPreview(context.Context, service.SubmitCustomizationEffectPreviewParams) (*domain.CustomizationJob, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) ReviewCustomizationEffect(context.Context, service.ReviewCustomizationEffectParams) (*domain.CustomizationJob, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) TransferCustomizationProduction(context.Context, service.TransferCustomizationProductionParams) (*domain.CustomizationJob, *domain.AppError) {
	return nil, nil
}

func (s *taskServiceCaptureStub) ListCustomizationJobs(context.Context, service.CustomizationJobFilter) ([]*domain.CustomizationJob, domain.PaginationMeta, *domain.AppError) {
	return nil, domain.PaginationMeta{}, nil
}

func (s *taskServiceCaptureStub) GetCustomizationJob(context.Context, int64) (*domain.CustomizationJob, *domain.AppError) {
	return nil, nil
}

var _ service.TaskService = (*taskServiceCaptureStub)(nil)

func testHandlerReferenceImageDataURI(sizeBytes int) string {
	if sizeBytes <= 0 {
		return "data:image/png;base64,"
	}
	raw := strings.Repeat("a", sizeBytes)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte(raw))
}
