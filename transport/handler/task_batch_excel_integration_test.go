//go:build integration

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
	taskbatchexcel "workflow/service/task_batch_excel"
)

func TestSAEI_DownloadTemplate_NPD(t *testing.T) {
	router := saeiRouter(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/batch-create/template.xlsx?task_type=new_product_development", nil)
	req.Header.Set("Authorization", "Bearer token")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "spreadsheetml") {
		t.Fatalf("content-type=%q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Content-Disposition") == "" {
		t.Fatalf("missing Content-Disposition")
	}
	if _, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes())); err != nil {
		t.Fatalf("open response workbook: %v", err)
	}
}

func TestSAEI_DownloadTemplate_PT(t *testing.T) {
	router := saeiRouter(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/batch-create/template.xlsx?task_type=purchase_task", nil)
	req.Header.Set("Authorization", "Bearer token")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes())); err != nil {
		t.Fatalf("open response workbook: %v", err)
	}
}

func TestSAEI_ParseUpload_HappyPath_NPD(t *testing.T) {
	assertSAEIParseHappyPath(t, domain.TaskTypeNewProductDevelopment)
}

func TestSAEI_ParseUpload_HappyPath_PT(t *testing.T) {
	assertSAEIParseHappyPath(t, domain.TaskTypePurchaseTask)
}

func TestSAEI_ParseUpload_RejectedTaskType(t *testing.T) {
	router := saeiRouter(true)
	body, contentType := saeiMultipart(t, "original_product_development", []byte("not-xlsx"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/batch-create/parse-excel", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "batch_not_supported_for_task_type") {
		t.Fatalf("body=%s, want batch_not_supported_for_task_type", rec.Body.String())
	}
}

func TestSAEI_DownloadTemplate_Auth_401(t *testing.T) {
	router := saeiRouter(false)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/batch-create/template.xlsx?task_type=new_product_development", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func assertSAEIParseHappyPath(t *testing.T, taskType domain.TaskType) {
	t.Helper()
	templateSvc := taskbatchexcel.NewTemplateService()
	content, appErr := templateSvc.Generate(t.Context(), taskType)
	if appErr != nil {
		t.Fatalf("Generate appErr = %v", appErr)
	}
	router := saeiRouter(true)
	body, contentType := saeiMultipart(t, string(taskType), content)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/batch-create/parse-excel", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			Preview    []json.RawMessage `json:"preview"`
			Violations []json.RawMessage `json:"violations"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if len(out.Data.Preview) != 2 || len(out.Data.Violations) != 0 {
		t.Fatalf("data=%+v, want 2 preview rows and no violations", out.Data)
	}
}

func saeiRouter(authRequired bool) *gin.Engine {
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
	templateSvc, parseSvc := taskbatchexcel.New()
	h := NewTaskBatchExcelHandler(templateSvc, parseSvc)
	router.GET("/v1/tasks/batch-create/template.xlsx", h.DownloadTemplate)
	router.POST("/v1/tasks/batch-create/parse-excel", h.ParseUpload)
	return router
}

func saeiMultipart(t *testing.T, taskType string, content []byte) (io.Reader, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("task_type", taskType); err != nil {
		t.Fatalf("write task_type: %v", err)
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
