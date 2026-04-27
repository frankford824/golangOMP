package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

func TestTaskHandlerCreateAcceptsReferenceFileRefObjects(t *testing.T) {
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
		"owner_team":     "鎬荤粡鍔炵粍",
		"due_at":         "2026-03-20T00:00:00Z",
		"product_id":     88,
		"sku_code":       "SKU-088",
		"change_request": "update design",
		"reference_file_refs": []map[string]interface{}{
			{
				"asset_id":     "ref-object-1",
				"download_url": "/v1/assets/files/objects/ref-object-1",
				"source":       "task_reference_upload",
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
	if len(taskSvc.createParams.ReferenceFileRefs) != 1 {
		t.Fatalf("captured reference_file_refs = %+v", taskSvc.createParams.ReferenceFileRefs)
	}
	ref := taskSvc.createParams.ReferenceFileRefs[0]
	if ref.AssetID != "ref-object-1" {
		t.Fatalf("captured ref asset_id = %q", ref.AssetID)
	}
	if ref.Source != "task_reference_upload" {
		t.Fatalf("captured ref source = %q", ref.Source)
	}
}

func TestTaskCreateReferenceUploadHandlerUploadFileReturnsRefObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewTaskCreateReferenceUploadHandler(&taskReferenceUploadServiceStub{
		uploadResult: &domain.ReferenceFileRef{
			AssetID:     "ref-upload-1",
			RefID:       "ref-upload-1",
			Source:      domain.ReferenceFileRefSourceTaskReferenceUpload,
			Status:      domain.ReferenceFileRefStatusUploaded,
			DownloadURL: stringPtr("/v1/assets/files/objects/ref-upload-1"),
			URL:         stringPtr("/v1/assets/files/objects/ref-upload-1"),
		},
	})
	router.POST("/v1/tasks/reference-upload", handler.UploadFile)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("created_by", "9"); err != nil {
		t.Fatalf("writer.WriteField(created_by) error = %v", err)
	}
	fileWriter, err := writer.CreateFormFile("file", "reference.png")
	if err != nil {
		t.Fatalf("writer.CreateFormFile() error = %v", err)
	}
	if _, err := fileWriter.Write([]byte("png-binary")); err != nil {
		t.Fatalf("fileWriter.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/reference-upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks/reference-upload code = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data domain.ReferenceFileRef `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v body=%s", err, rec.Body.String())
	}
	if resp.Data.AssetID != "ref-upload-1" {
		t.Fatalf("response asset_id = %q", resp.Data.AssetID)
	}
	if resp.Data.Source != domain.ReferenceFileRefSourceTaskReferenceUpload {
		t.Fatalf("response source = %q", resp.Data.Source)
	}
}

func TestTaskCreateReferenceUploadHandlerUploadFileReturnsInternalErrorWithTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("trace_id", "trace-upload-failure")
		c.Next()
	})
	handler := NewTaskCreateReferenceUploadHandler(&taskReferenceUploadServiceStub{
		appErr: domain.NewAppError(domain.ErrCodeInternalError, "internal error during probe task-create reference stored file", nil),
	})
	router.POST("/v1/tasks/reference-upload", handler.UploadFile)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("created_by", "9"); err != nil {
		t.Fatalf("writer.WriteField(created_by) error = %v", err)
	}
	fileWriter, err := writer.CreateFormFile("file", "reference.png")
	if err != nil {
		t.Fatalf("writer.CreateFormFile() error = %v", err)
	}
	if _, err := fileWriter.Write([]byte("png-binary")); err != nil {
		t.Fatalf("fileWriter.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/reference-upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("POST /v1/tasks/reference-upload code = %d, want 500 body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Error domain.AppError `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v body=%s", err, rec.Body.String())
	}
	if resp.Error.Code != domain.ErrCodeInternalError {
		t.Fatalf("response error code = %q", resp.Error.Code)
	}
	if resp.Error.TraceID != "trace-upload-failure" {
		t.Fatalf("response trace_id = %q", resp.Error.TraceID)
	}
}

func TestTaskReferenceUploadAndTaskDetailExposePresignedReferenceURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	presignedURL := "https://yongbooss.oss-cn-hangzhou.aliyuncs.com/tasks/pre-create/ref-upload-1.png?OSSAccessKeyId=ak-test&Expires=1776683700&Signature=sig&response-content-disposition=inline"
	taskSvc := &taskServiceCaptureStub{
		createResult: &domain.Task{ID: 401},
		readResult: &domain.TaskReadModel{
			Task: domain.Task{ID: 401},
			ReferenceFileRefs: []domain.ReferenceFileRef{{
				AssetID:              "ref-upload-1",
				StorageKey:           "tasks/pre-create/ref-upload-1.png",
				DownloadURL:          stringPtr(presignedURL),
				URL:                  stringPtr(presignedURL),
				DownloadURLExpiresAt: timePtr("2026-04-20T11:15:00Z"),
				Source:               domain.ReferenceFileRefSourceTaskReferenceUpload,
				Status:               domain.ReferenceFileRefStatusUploaded,
			}},
		},
	}
	uploadHandler := NewTaskCreateReferenceUploadHandler(&taskReferenceUploadServiceStub{
		uploadResult: &domain.ReferenceFileRef{
			AssetID:     "ref-upload-1",
			RefID:       "ref-upload-1",
			StorageKey:  "tasks/pre-create/ref-upload-1.png",
			Source:      domain.ReferenceFileRefSourceTaskReferenceUpload,
			Status:      domain.ReferenceFileRefStatusUploaded,
			DownloadURL: stringPtr("/v1/assets/files/tasks/pre-create/ref-upload-1.png"),
			URL:         stringPtr("/v1/assets/files/tasks/pre-create/ref-upload-1.png"),
		},
	})
	taskHandler := NewTaskHandler(taskSvc, nil, nil)

	router.POST("/v1/tasks/reference-upload", uploadHandler.UploadFile)
	router.POST("/v1/tasks", taskHandler.Create)
	router.GET("/v1/tasks/:id", taskHandler.GetByID)

	var uploadBody bytes.Buffer
	writer := multipart.NewWriter(&uploadBody)
	if err := writer.WriteField("created_by", "9"); err != nil {
		t.Fatalf("writer.WriteField(created_by) error = %v", err)
	}
	fileWriter, err := writer.CreateFormFile("file", "reference.jpg")
	if err != nil {
		t.Fatalf("writer.CreateFormFile() error = %v", err)
	}
	if _, err := fileWriter.Write([]byte("jpg-binary")); err != nil {
		t.Fatalf("fileWriter.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/v1/tasks/reference-upload", &uploadBody)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	router.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks/reference-upload code = %d, want 201 body=%s", uploadRec.Code, uploadRec.Body.String())
	}

	var uploadResp struct {
		Data domain.ReferenceFileRef `json:"data"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("json.Unmarshal(upload response) error = %v body=%s", err, uploadRec.Body.String())
	}

	createBody := map[string]interface{}{
		"task_type":      "original_product_development",
		"source_mode":    "existing_product",
		"creator_id":     9,
		"owner_team":     "运营部/淘系一组",
		"due_at":         "2026-04-20T12:00:00Z",
		"product_id":     88,
		"sku_code":       "SKU-401",
		"change_request": "update detail page",
		"reference_file_refs": []domain.ReferenceFileRef{
			uploadResp.Data,
		},
	}
	rawCreate, err := json.Marshal(createBody)
	if err != nil {
		t.Fatalf("json.Marshal(createBody) error = %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(rawCreate))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks code = %d, want 201 body=%s", createRec.Code, createRec.Body.String())
	}
	if len(taskSvc.createParams.ReferenceFileRefs) != 1 || taskSvc.createParams.ReferenceFileRefs[0].StorageKey != "tasks/pre-create/ref-upload-1.png" {
		t.Fatalf("captured create reference_file_refs = %+v", taskSvc.createParams.ReferenceFileRefs)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/tasks/401", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/tasks/401 code = %d, want 200 body=%s", getRec.Code, getRec.Body.String())
	}

	var getResp struct {
		Data domain.TaskReadModel `json:"data"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("json.Unmarshal(get response) error = %v body=%s", err, getRec.Body.String())
	}
	if len(getResp.Data.ReferenceFileRefs) != 1 {
		t.Fatalf("GET /v1/tasks/401 reference_file_refs = %+v", getResp.Data.ReferenceFileRefs)
	}
	downloadURL := getResp.Data.ReferenceFileRefs[0].DownloadURL
	if downloadURL == nil || strings.HasPrefix(*downloadURL, "/v1/assets/files/") {
		t.Fatalf("download_url = %v, want presigned OSS URL", downloadURL)
	}
	if !strings.HasPrefix(*downloadURL, "https://yongbooss.oss-cn-hangzhou.aliyuncs.com/") || !strings.Contains(*downloadURL, "Expires=") {
		t.Fatalf("download_url = %q, want OSS presign contract", *downloadURL)
	}
}

type taskReferenceUploadServiceStub struct {
	uploadResult *domain.ReferenceFileRef
	appErr       *domain.AppError
}

func (s *taskReferenceUploadServiceStub) CreateUploadSession(context.Context, service.CreateTaskReferenceUploadSessionParams) (*service.CreateTaskReferenceUploadSessionResult, *domain.AppError) {
	return nil, nil
}

func (s *taskReferenceUploadServiceStub) GetUploadSession(context.Context, string, int64) (*domain.UploadSession, *domain.AppError) {
	return nil, nil
}

func (s *taskReferenceUploadServiceStub) UploadFile(_ context.Context, _ service.UploadTaskReferenceFileParams) (*domain.ReferenceFileRef, *domain.AppError) {
	return s.uploadResult, s.appErr
}

func (s *taskReferenceUploadServiceStub) CompleteUploadSession(context.Context, service.CompleteTaskReferenceUploadSessionParams) (*service.CompleteTaskReferenceUploadSessionResult, *domain.AppError) {
	return nil, nil
}

func (s *taskReferenceUploadServiceStub) CancelUploadSession(context.Context, service.CancelTaskReferenceUploadSessionParams) (*domain.UploadSession, *domain.AppError) {
	return nil, nil
}

var _ service.TaskCreateReferenceUploadService = (*taskReferenceUploadServiceStub)(nil)

func stringPtr(value string) *string {
	return &value
}

func timePtr(value string) *time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return &parsed
}
