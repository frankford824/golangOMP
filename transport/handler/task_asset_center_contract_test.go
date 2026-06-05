package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

func TestTaskAssetCenterHandlerCreateUploadSessionInfersStrategy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &taskAssetCenterServiceStub{
		createResult: &service.CreateTaskAssetUploadSessionResult{
			Session: &domain.UploadSession{
				ID:         "sess-task-1",
				TaskID:     123,
				UploadMode: domain.DesignAssetUploadModeMultipart,
				MimeType:   "application/octet-stream",
			},
		},
	}
	handler := NewTaskAssetCenterHandler(svc)
	router.POST("/v1/tasks/:id/asset-center/upload-sessions", handler.CreateUploadSession)

	body := bytes.NewBufferString(`{"created_by":9,"asset_type":"delivery","filename":"delivery.zip","mime_type":"application/zip"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/123/asset-center/upload-sessions", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/tasks/:id/asset-center/upload-sessions code = %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.createCalls != 1 || svc.createSmallCalls != 0 || svc.createMultipartCalls != 0 {
		t.Fatalf("create call routing = create:%d small:%d multipart:%d", svc.createCalls, svc.createSmallCalls, svc.createMultipartCalls)
	}

	var resp struct {
		Data struct {
			UploadStrategy            string `json:"upload_strategy"`
			RequiredUploadContentType string `json:"required_upload_content_type"`
			CompleteEndpoint          string `json:"complete_endpoint"`
			CancelEndpoint            string `json:"cancel_endpoint"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v body=%s", err, rec.Body.String())
	}
	if resp.Data.UploadStrategy != "multipart" {
		t.Fatalf("upload_strategy = %q", resp.Data.UploadStrategy)
	}
	if resp.Data.RequiredUploadContentType != "application/octet-stream" {
		t.Fatalf("required_upload_content_type = %q", resp.Data.RequiredUploadContentType)
	}
	if resp.Data.CompleteEndpoint != "/v1/tasks/123/asset-center/upload-sessions/sess-task-1/complete" {
		t.Fatalf("complete_endpoint = %q", resp.Data.CompleteEndpoint)
	}
	if resp.Data.CancelEndpoint != "/v1/tasks/123/asset-center/upload-sessions/sess-task-1/cancel" {
		t.Fatalf("cancel_endpoint = %q", resp.Data.CancelEndpoint)
	}
}

func TestTaskAssetCenterHandlerCreateAssetUploadSessionReturnsCanonicalEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &taskAssetCenterServiceStub{
		createResult: &service.CreateTaskAssetUploadSessionResult{
			Session: &domain.UploadSession{
				ID:         "sess-asset-1",
				TaskID:     456,
				UploadMode: domain.DesignAssetUploadModeSmall,
				MimeType:   "image/png",
			},
		},
	}
	handler := NewTaskAssetCenterHandler(svc)
	router.POST("/v1/assets/upload-sessions", handler.CreateAssetUploadSession)

	body := bytes.NewBufferString(`{"task_id":456,"created_by":9,"asset_kind":"reference","file_name":"reference.png","mime_type":"image/png"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/assets/upload-sessions", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/assets/upload-sessions code = %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.createCalls != 1 {
		t.Fatalf("CreateUploadSession() calls = %d", svc.createCalls)
	}

	var resp struct {
		Data struct {
			UploadStrategy            string `json:"upload_strategy"`
			RequiredUploadContentType string `json:"required_upload_content_type"`
			CompleteEndpoint          string `json:"complete_endpoint"`
			CancelEndpoint            string `json:"cancel_endpoint"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v body=%s", err, rec.Body.String())
	}
	if resp.Data.UploadStrategy != "single_part" {
		t.Fatalf("upload_strategy = %q", resp.Data.UploadStrategy)
	}
	if resp.Data.RequiredUploadContentType != "image/png" {
		t.Fatalf("required_upload_content_type = %q", resp.Data.RequiredUploadContentType)
	}
	if resp.Data.CompleteEndpoint != "/v1/assets/upload-sessions/sess-asset-1/complete" {
		t.Fatalf("complete_endpoint = %q", resp.Data.CompleteEndpoint)
	}
	if resp.Data.CancelEndpoint != "/v1/assets/upload-sessions/sess-asset-1/cancel" {
		t.Fatalf("cancel_endpoint = %q", resp.Data.CancelEndpoint)
	}
}

func TestTaskAssetCenterHandlerCreateAssetUploadSessionAcceptsNumericStringIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &taskAssetCenterServiceStub{
		createResult: &service.CreateTaskAssetUploadSessionResult{
			Session: &domain.UploadSession{
				ID:         "sess-asset-string-ids",
				TaskID:     456,
				UploadMode: domain.DesignAssetUploadModeMultipart,
				MimeType:   "image/png",
			},
		},
	}
	handler := NewTaskAssetCenterHandler(svc)
	router.POST("/v1/assets/upload-sessions", handler.CreateAssetUploadSession)

	body := bytes.NewBufferString(`{"task_id":"456","created_by":"9","asset_kind":"reference","asset_id":"4477","source_asset_id":"4480","file_name":"reference.png","expected_size":"1024","mime_type":"image/png"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/assets/upload-sessions", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/assets/upload-sessions code = %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.createCalls != 1 {
		t.Fatalf("CreateUploadSession() calls = %d", svc.createCalls)
	}
	if svc.lastCreateParams.TaskID != 456 {
		t.Fatalf("TaskID = %d, want 456", svc.lastCreateParams.TaskID)
	}
	if svc.lastCreateParams.CreatedBy != 9 {
		t.Fatalf("CreatedBy = %d, want 9", svc.lastCreateParams.CreatedBy)
	}
	if svc.lastCreateParams.AssetID == nil || *svc.lastCreateParams.AssetID != 4477 {
		t.Fatalf("AssetID = %v, want 4477", svc.lastCreateParams.AssetID)
	}
	if svc.lastCreateParams.SourceAssetID == nil || *svc.lastCreateParams.SourceAssetID != 4480 {
		t.Fatalf("SourceAssetID = %v, want 4480", svc.lastCreateParams.SourceAssetID)
	}
	if svc.lastCreateParams.ExpectedSize == nil || *svc.lastCreateParams.ExpectedSize != 1024 {
		t.Fatalf("ExpectedSize = %v, want 1024", svc.lastCreateParams.ExpectedSize)
	}
}

func TestTaskAssetCenterHandlerCreateAssetUploadSessionRejectsLegacyUUIDAssetID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &taskAssetCenterServiceStub{}
	handler := NewTaskAssetCenterHandler(svc)
	router.POST("/v1/assets/upload-sessions", handler.CreateAssetUploadSession)

	body := bytes.NewBufferString(`{"task_id":456,"created_by":9,"asset_kind":"reference","asset_id":"0ec54522-98fb-4d66-a7af-74cafb59e088","file_name":"reference.png","mime_type":"image/png"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/assets/upload-sessions", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/assets/upload-sessions code = %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.createCalls != 0 {
		t.Fatalf("CreateUploadSession() calls = %d, want 0", svc.createCalls)
	}
	if !strings.Contains(rec.Body.String(), "integer fields must use a JSON integer or numeric string") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

type taskAssetCenterServiceStub struct {
	createResult         *service.CreateTaskAssetUploadSessionResult
	createCalls          int
	createSmallCalls     int
	createMultipartCalls int
	lastCreateParams     service.CreateTaskAssetUploadSessionParams
}

func (s *taskAssetCenterServiceStub) ListAssetResources(context.Context, service.ListAssetResourcesParams) ([]*domain.DesignAsset, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) GetAsset(context.Context, int64) (*domain.DesignAsset, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) ListAssets(context.Context, int64) ([]*domain.DesignAsset, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) ListVersions(context.Context, int64, int64) ([]*domain.DesignAssetVersion, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) GetAssetDownloadInfoByID(context.Context, int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) GetAssetPreviewInfoByID(context.Context, int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) GetAssetDownloadInfo(context.Context, int64, int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) GetVersionDownloadInfo(context.Context, int64, int64, int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) GetUploadSessionByID(context.Context, string) (*domain.UploadSession, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) CreateUploadSession(_ context.Context, params service.CreateTaskAssetUploadSessionParams) (*service.CreateTaskAssetUploadSessionResult, *domain.AppError) {
	s.createCalls++
	s.lastCreateParams = params
	return s.createResult, nil
}
func (s *taskAssetCenterServiceStub) GetUploadSession(context.Context, int64, string) (*domain.UploadSession, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) CreateSmallUploadSession(_ context.Context, params service.CreateTaskAssetUploadSessionParams) (*service.CreateTaskAssetUploadSessionResult, *domain.AppError) {
	s.createSmallCalls++
	s.lastCreateParams = params
	return s.createResult, nil
}
func (s *taskAssetCenterServiceStub) CreateMultipartUploadSession(_ context.Context, params service.CreateTaskAssetUploadSessionParams) (*service.CreateTaskAssetUploadSessionResult, *domain.AppError) {
	s.createMultipartCalls++
	s.lastCreateParams = params
	return s.createResult, nil
}
func (s *taskAssetCenterServiceStub) CompleteUploadSessionByID(context.Context, service.CompleteTaskAssetUploadSessionParams) (*service.CompleteTaskAssetUploadSessionResult, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) CompleteUploadSession(context.Context, service.CompleteTaskAssetUploadSessionParams) (*service.CompleteTaskAssetUploadSessionResult, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) CancelUploadSessionByID(context.Context, service.CancelTaskAssetUploadSessionParams) (*domain.UploadSession, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) CancelUploadSession(context.Context, service.CancelTaskAssetUploadSessionParams) (*domain.UploadSession, *domain.AppError) {
	return nil, nil
}
func (s *taskAssetCenterServiceStub) BuildTaskReferenceBatchDownloadManifest(context.Context, int64, int64) (*service.TaskReferenceBatchDownloadManifest, *domain.AppError) {
	return &service.TaskReferenceBatchDownloadManifest{}, nil
}
func (s *taskAssetCenterServiceStub) EnsureDerivedPreviewAssets(context.Context, int64, int64, int64) *domain.AppError {
	return nil
}

var _ service.TaskAssetCenterService = (*taskAssetCenterServiceStub)(nil)
