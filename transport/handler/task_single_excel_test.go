package handler

import (
	"bytes"
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

func TestExcelAssist_DownloadTemplate_UnsupportedTaskType(t *testing.T) {
	router := excelAssistRouter(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/excel-assist/template.xlsx?task_type=purchase_task&mode=single", nil)
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
			Draft      json.RawMessage        `json:"draft"`
			Violations []json.RawMessage      `json:"violations"`
			Mode       string                 `json:"mode"`
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

func TestExcelAssist_ParseUpload_UnsupportedTaskType(t *testing.T) {
	router := excelAssistRouter(true)
	body, contentType := excelAssistMultipart(t, "purchase_task", "single", []byte("x"))
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

func excelAssistRouter(authRequired bool) *gin.Engine {
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
	parseSvc := tasksingleexcel.NewParseService()
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
