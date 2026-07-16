package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"workflow/domain"
	tasksingleexcel "workflow/service/task_single_excel"
)

func TestExcelAssist_DownloadTemplate_Success(t *testing.T) {
	router := excelAssistRouter(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/excel-assist/template.xlsx?task_type=new_product_development&mode=single", nil)
	req.Header.Set("Authorization", "Bearer token")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "spreadsheetml") {
		t.Fatalf("content-type=%q", rec.Header().Get("Content-Type"))
	}
	if _, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes())); err != nil {
		t.Fatalf("open workbook: %v", err)
	}
}

func TestExcelAssist_DownloadTemplate_InvalidMode(t *testing.T) {
	router := excelAssistRouter(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/excel-assist/template.xlsx?task_type=new_product_development&mode=multiple", nil)
	req.Header.Set("Authorization", "Bearer token")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_excel_assist_mode") {
		t.Fatalf("body=%s, want invalid_excel_assist_mode", rec.Body.String())
	}
}

func TestExcelAssist_DownloadTemplate_PurchaseRetired(t *testing.T) {
	router := excelAssistRouter(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/excel-assist/template.xlsx?task_type=purchase_task&mode=single", nil)
	req.Header.Set("Authorization", "Bearer token")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "excel_assist_task_type_not_supported") {
		t.Fatalf("body=%s, want retired task type rejection", rec.Body.String())
	}
}

func TestExcelAssist_DownloadTemplate_UnsupportedTaskType(t *testing.T) {
	router := excelAssistRouter(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/excel-assist/template.xlsx?task_type=retouch_task&mode=single", nil)
	req.Header.Set("Authorization", "Bearer token")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "excel_assist_task_type_not_supported") {
		t.Fatalf("body=%s, want excel_assist_task_type_not_supported", rec.Body.String())
	}
}

func TestExcelAssist_ParseUpload_HappyPath(t *testing.T) {
	content, appErr := tasksingleexcel.NewTemplateService().Generate(t.Context(), domain.TaskTypeNewProductDevelopment, tasksingleexcel.AssistModeSingle)
	if appErr != nil {
		t.Fatalf("Generate appErr = %v", appErr)
	}
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer f.Close()
	_ = f.SetCellValue("Items", "A2", "IID-TEST")
	_ = f.SetCellValue("Items", "B2", "产品名")
	_ = f.SetCellValue("Items", "C2", "设计要求")
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	router := excelAssistRouter(true)
	body, contentType := excelAssistMultipart(t, "new_product_development", "single", buf.Bytes())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/excel-assist/parse-excel", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			Draft      json.RawMessage   `json:"draft"`
			Violations []json.RawMessage `json:"violations"`
			Mode       string            `json:"mode"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if out.Data.Mode != "single" {
		t.Fatalf("mode=%q", out.Data.Mode)
	}
	if len(out.Data.Violations) != 0 {
		t.Fatalf("violations=%v", out.Data.Violations)
	}
	if len(out.Data.Draft) == 0 {
		t.Fatal("draft missing")
	}
}

func TestExcelAssist_ParseUpload_InvalidMode(t *testing.T) {
	router := excelAssistRouter(true)
	body, contentType := excelAssistMultipart(t, "new_product_development", "multiple", []byte("x"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/excel-assist/parse-excel", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_excel_assist_mode") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestExcelAssist_ParseUpload_PurchaseRetired(t *testing.T) {
	router := excelAssistRouter(true)
	body, contentType := excelAssistMultipart(t, "purchase_task", "single", []byte("retired"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/excel-assist/parse-excel", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "excel_assist_task_type_not_supported") {
		t.Fatalf("body=%s, want retired task type rejection", rec.Body.String())
	}
}

func TestExcelAssist_DownloadTemplate_OriginalSuccess(t *testing.T) {
	router := excelAssistRouter(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/excel-assist/template.xlsx?task_type=original_product_development&mode=single", nil)
	req.Header.Set("Authorization", "Bearer token")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("Items")
	if err != nil || len(rows) < 1 || rows[0][0] != "SKU编码" {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
}

func TestExcelAssist_ParseUpload_OriginalHappyPath(t *testing.T) {
	content, appErr := tasksingleexcel.NewTemplateService().Generate(t.Context(), domain.TaskTypeOriginalProductDevelopment, tasksingleexcel.AssistModeSingle)
	if appErr != nil {
		t.Fatalf("Generate appErr = %v", appErr)
	}
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer f.Close()
	_ = f.SetCellValue("Items", "A2", "SKU-ORIG-H")
	_ = f.SetCellValue("Items", "B2", "修改要求文案")
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	lookup := &excelAssistMockERPLookup{
		skuProducts: map[string][]*domain.ERPProduct{
			"SKU-ORIG-H": {{
				ProductID:   "ERP-H-1",
				SKUCode:     "SKU-ORIG-H",
				ProductName: "原款测试商品",
			}},
		},
	}
	router := excelAssistRouterWithParse(true, tasksingleexcel.NewParseServiceWithDependencies(lookup))
	body, contentType := excelAssistMultipart(t, "original_product_development", "single", buf.Bytes())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/excel-assist/parse-excel", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"sku_code":"SKU-ORIG-H"`) && !strings.Contains(rec.Body.String(), `"sku_code": "SKU-ORIG-H"`) {
		t.Fatalf("body=%s, want sku_code in draft", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "原款测试商品") {
		t.Fatalf("body=%s, want enriched product name", rec.Body.String())
	}
}

func TestExcelAssist_ParseUpload_UnsupportedTaskType(t *testing.T) {
	router := excelAssistRouter(true)
	body, contentType := excelAssistMultipart(t, "retouch_task", "single", []byte("x"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/excel-assist/parse-excel", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "excel_assist_task_type_not_supported") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

type excelAssistMockERPLookup struct {
	skuProducts map[string][]*domain.ERPProduct
}

func (m *excelAssistMockERPLookup) ListIIDs(context.Context, domain.ERPIIDListFilter) (*domain.ERPIIDListResponse, *domain.AppError) {
	return &domain.ERPIIDListResponse{Items: nil}, nil
}

func (m *excelAssistMockERPLookup) SearchProducts(_ context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, *domain.AppError) {
	sku := strings.TrimSpace(filter.SKUCode)
	if m != nil && m.skuProducts != nil {
		if items, ok := m.skuProducts[sku]; ok {
			return &domain.ERPProductListResponse{Items: items}, nil
		}
	}
	return &domain.ERPProductListResponse{Items: nil}, nil
}

func excelAssistRouter(authRequired bool) *gin.Engine {
	return excelAssistRouterWithParse(authRequired, tasksingleexcel.NewParseService())
}

func excelAssistRouterWithParse(authRequired bool, parseSvc tasksingleexcel.ParseService) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	if authRequired {
		router.Use(func(c *gin.Context) {
			if !strings.HasPrefix(c.GetHeader("Authorization"), "Bearer ") {
				respondError(c, domain.ErrUnauthorized)
				return
			}
			c.Next()
		})
	} else {
		router.Use(func(c *gin.Context) {
			respondError(c, domain.ErrUnauthorized)
		})
	}
	templateSvc := tasksingleexcel.NewTemplateService()
	h := NewTaskSingleExcelHandler(templateSvc, parseSvc)
	router.GET("/v1/tasks/excel-assist/template.xlsx", h.DownloadTemplate)
	router.POST("/v1/tasks/excel-assist/parse-excel", h.ParseUpload)
	return router
}

func excelAssistMultipart(t *testing.T, taskType, mode string, content []byte) (io.Reader, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("task_type", taskType); err != nil {
		t.Fatalf("write task_type: %v", err)
	}
	if err := writer.WriteField("mode", mode); err != nil {
		t.Fatalf("write mode: %v", err)
	}
	part, err := writer.CreateFormFile("file", "template.xlsx")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &body, writer.FormDataContentType()
}
