package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"workflow/domain"
	"workflow/repo"
	"workflow/service"
)

func TestAssetFilesHandlerServeFileProxiesHeadersAndStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/objects/reference/ref-1.png" {
			t.Fatalf("upstream path = %q, want /files/objects/reference/ref-1.png", r.URL.Path)
		}
		if r.URL.RawQuery != "download=1" {
			t.Fatalf("upstream query = %q, want download=1", r.URL.RawQuery)
		}
		if got := r.Header.Get("X-Internal-Token"); got != "oss-token" {
			t.Fatalf("upstream X-Internal-Token = %q, want oss-token", got)
		}
		if got := r.Header.Get("X-Storage-Provider"); got != "oss" {
			t.Fatalf("upstream X-Storage-Provider = %q, want oss", got)
		}
		if got := r.Header.Get("Range"); got != "bytes=0-3" {
			t.Fatalf("upstream Range = %q, want bytes=0-3", got)
		}

		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Disposition", `inline; filename="ref-1.png"`)
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("data"))
	}))
	defer upstream.Close()

	router := gin.New()
	h := NewAssetFilesHandler(upstream.URL, "oss-token", "oss", zap.NewNop())
	router.GET("/v1/assets/files/*path", h.ServeFile)

	req := httptest.NewRequest(http.MethodGet, "/v1/assets/files/objects/reference/ref-1.png?download=1", nil)
	req.Header.Set("Range", "bytes=0-3")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusPartialContent, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("accept-ranges = %q", rec.Header().Get("Accept-Ranges"))
	}
	if rec.Body.String() != "data" {
		t.Fatalf("body = %q, want data", rec.Body.String())
	}
}

func TestAssetFilesHandlerServeFileSetsDownloadFilenameHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const filename = "交付 文件.psd"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get(service.DownloadFilenameQueryParam); got != "" {
			t.Fatalf("upstream download_filename = %q, want empty", got)
		}
		if got := r.URL.Query().Get("download"); got != "1" {
			t.Fatalf("upstream download = %q, want 1", got)
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `inline; filename="storage.psd"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	}))
	defer upstream.Close()

	router := gin.New()
	h := NewAssetFilesHandler(upstream.URL, "oss-token", "oss", zap.NewNop())
	router.GET("/v1/assets/files/*path", h.ServeFile)

	req := httptest.NewRequest(http.MethodGet, "/v1/assets/files/objects/reference/ref-1.psd?download=1&"+service.DownloadFilenameQueryParam+"="+url.QueryEscape(filename), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	disposition := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, "attachment") {
		t.Fatalf("Content-Disposition = %q, want attachment", disposition)
	}
	if !strings.Contains(disposition, "filename*=") {
		t.Fatalf("Content-Disposition = %q, want encoded filename parameter", disposition)
	}
	if !strings.Contains(disposition, "%E4%BA%A4%E4%BB%98%20%E6%96%87%E4%BB%B6.psd") {
		t.Fatalf("Content-Disposition = %q, want encoded filename", disposition)
	}
	if rec.Body.String() != "data" {
		t.Fatalf("body = %q, want data", rec.Body.String())
	}
}

func TestAssetFilesHandlerServeFilePassesThroughUpstream404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "not found in oss")
	}))
	defer upstream.Close()

	router := gin.New()
	h := NewAssetFilesHandler(upstream.URL, "oss-token", "oss", zap.NewNop())
	router.GET("/v1/assets/files/*path", h.ServeFile)

	req := httptest.NewRequest(http.MethodGet, "/v1/assets/files/objects/reference/ref-upload-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if rec.Body.String() != "not found in oss" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestAssetFilesHandlerServeFileRedirectsToOSSDirectOnUpstream404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "not found in upload service")
	}))
	defer upstream.Close()

	router := gin.New()
	h := NewAssetFilesHandler(upstream.URL, "oss-token", "oss", zap.NewNop(), assetFilesPresignerStub{
		urls: map[string]string{
			"tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/ref.jpeg": "https://oss.example/ref.jpeg?sig=1",
		},
	})
	router.GET("/v1/assets/files/*path", h.ServeFile)

	req := httptest.NewRequest(http.MethodGet, "/v1/assets/files/tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/ref.jpeg", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302 body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "https://oss.example/ref.jpeg?sig=1" {
		t.Fatalf("Location = %q", got)
	}
}

type assetFilesPresignerStub struct {
	urls map[string]string
}

func (s assetFilesPresignerStub) PresignPreviewURL(objectKey string) *service.OSSDirectDownloadInfo {
	if url := s.urls[objectKey]; url != "" {
		return &service.OSSDirectDownloadInfo{DownloadURL: url}
	}
	return nil
}

func TestAssetFilesHandlerServeERPProductImageRedirectsWithValidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)

	storageKey := "tasks/RW-20260603-A-001080/assets/AST-0005/v1/delivery/image.jpg"
	mimeType := "image/jpeg"
	asset := &domain.TaskAsset{ID: 4395, TaskID: 1080, FileName: "image.jpg", MimeType: &mimeType, StorageKey: &storageKey}
	signer := service.NewERPImageProxySigner(service.ERPImageProxyConfig{
		PublicBaseURL: "https://yongbo.cloud",
		SigningSecret: "proxy-secret",
		TokenTTL:      time.Hour,
	})
	imageURL := signer.BuildImageURL(asset)
	if imageURL == nil {
		t.Fatal("BuildImageURL() = nil")
	}
	parsed, err := url.Parse(*imageURL)
	if err != nil {
		t.Fatalf("parse image url: %v", err)
	}

	router := gin.New()
	h := NewAssetFilesHandler("http://upload.invalid", "", "oss", zap.NewNop(), assetFilesPresignerStub{
		urls: map[string]string{storageKey: "https://oss.example/object.jpg?OSSAccessKeyId=1"},
	})
	h.SetERPImageProxy(assetFilesTaskAssetRepoStub{assets: map[int64]*domain.TaskAsset{4395: asset}}, signer)
	router.GET(service.ERPImageProxyPathPrefix+"/:version_id", h.ServeERPProductImage)

	req := httptest.NewRequest(http.MethodGet, parsed.RequestURI(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302 body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "https://oss.example/object.jpg?OSSAccessKeyId=1" {
		t.Fatalf("Location = %q", got)
	}
}

func TestAssetFilesHandlerServeERPProductImageRejectsInvalidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)

	storageKey := "tasks/RW-20260603-A-001080/assets/AST-0005/v1/delivery/image.jpg"
	mimeType := "image/jpeg"
	asset := &domain.TaskAsset{ID: 4395, TaskID: 1080, FileName: "image.jpg", MimeType: &mimeType, StorageKey: &storageKey}
	signer := service.NewERPImageProxySigner(service.ERPImageProxyConfig{
		PublicBaseURL: "https://yongbo.cloud",
		SigningSecret: "proxy-secret",
		TokenTTL:      time.Hour,
	})
	imageURL := signer.BuildImageURL(asset)
	if imageURL == nil {
		t.Fatal("BuildImageURL() = nil")
	}
	parsed, err := url.Parse(*imageURL)
	if err != nil {
		t.Fatalf("parse image url: %v", err)
	}
	query := parsed.Query()
	query.Set("sig", "bad-signature")
	parsed.RawQuery = query.Encode()

	router := gin.New()
	h := NewAssetFilesHandler("http://upload.invalid", "", "oss", zap.NewNop(), assetFilesPresignerStub{
		urls: map[string]string{storageKey: "https://oss.example/object.jpg?OSSAccessKeyId=1"},
	})
	h.SetERPImageProxy(assetFilesTaskAssetRepoStub{assets: map[int64]*domain.TaskAsset{4395: asset}}, signer)
	router.GET(service.ERPImageProxyPathPrefix+"/:version_id", h.ServeERPProductImage)

	req := httptest.NewRequest(http.MethodGet, parsed.RequestURI(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 body=%s", rec.Code, rec.Body.String())
	}
}

type assetFilesTaskAssetRepoStub struct {
	assets map[int64]*domain.TaskAsset
}

func (r assetFilesTaskAssetRepoStub) Create(context.Context, repo.Tx, *domain.TaskAsset) (int64, error) {
	return 0, nil
}

func (r assetFilesTaskAssetRepoStub) GetByID(_ context.Context, id int64) (*domain.TaskAsset, error) {
	return r.assets[id], nil
}

func (r assetFilesTaskAssetRepoStub) ListByTaskID(context.Context, int64) ([]*domain.TaskAsset, error) {
	return nil, nil
}

func (r assetFilesTaskAssetRepoStub) ListByAssetID(context.Context, int64) ([]*domain.TaskAsset, error) {
	return nil, nil
}

func (r assetFilesTaskAssetRepoStub) NextVersionNo(context.Context, repo.Tx, int64) (int, error) {
	return 0, nil
}

func (r assetFilesTaskAssetRepoStub) NextAssetVersionNo(context.Context, repo.Tx, int64) (int, error) {
	return 0, nil
}

func TestAssetFilesHandlerServeFileEscapesStorageKeyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		escapedStorageKey = "tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/%F0%9F%92%9A97%25%20%E8%83%BD%E9%87%8F%E5%85%85%E6%BB%A1%E5%95%A6.jpg"
	)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/files/"+escapedStorageKey {
			t.Fatalf("upstream escaped path = %q, want %q", got, "/files/"+escapedStorageKey)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	router := gin.New()
	h := NewAssetFilesHandler(upstream.URL, "oss-token", "oss", zap.NewNop())
	router.GET("/v1/assets/files/*path", h.ServeFile)

	req := httptest.NewRequest(http.MethodGet, "/v1/assets/files/"+escapedStorageKey, nil)
	req = req.WithContext(domain.ContextWithTraceID(req.Context(), "asset-files-escaped-path"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q, want ok", rec.Body.String())
	}
}

func TestAssetFilesHandlerServeFileRejectsMissingStoragePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	h := NewAssetFilesHandler("http://127.0.0.1:8092", "", "", zap.NewNop())
	router.GET("/v1/assets/files/*path", h.ServeFile)

	req := httptest.NewRequest(http.MethodGet, "/v1/assets/files/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAssetFilesHandlerServeFileDropsZeroContentLengthWhenBodyExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewAssetFilesHandler("http://example.invalid", "nas-token", "nas", zap.NewNop())
	h.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type":   []string{"image/png"},
					"Content-Length": []string{"0"},
				},
				Body:          io.NopCloser(bytes.NewReader([]byte("data"))),
				ContentLength: 0,
				Request:       r,
			}, nil
		}),
	}

	router := gin.New()
	router.GET("/v1/assets/files/*path", h.ServeFile)

	req := httptest.NewRequest(http.MethodGet, "/v1/assets/files/tasks/file.png", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "data" {
		t.Fatalf("body = %q, want data", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Length"); got != "" {
		t.Fatalf("content-length = %q, want empty", got)
	}
}

func TestAssetFilesHandlerLiveProbe(t *testing.T) {
	if os.Getenv("ASSET_FILES_LIVE_PROBE") != "1" {
		t.Skip("set ASSET_FILES_LIVE_PROBE=1 to run live NAS probe")
	}

	const (
		baseURL    = "http://100.111.214.38:8089"
		storageKey = "tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/p1hRH.png"
		token      = "nas-upload-token-2026"
	)

	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	h := NewAssetFilesHandler(baseURL, token, "nas", logger)
	router := gin.New()
	router.GET("/v1/assets/files/*path", h.ServeFile)

	req := httptest.NewRequest(http.MethodGet, "/v1/assets/files/"+storageKey, nil)
	req.Header.Set("X-Trace-ID", "asset-files-live-probe")
	req = req.WithContext(domain.ContextWithTraceID(req.Context(), "asset-files-live-probe"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	t.Logf("status=%d content_length=%q body_len=%d trace_id=%q", rec.Code, rec.Header().Get("Content-Length"), rec.Body.Len(), rec.Header().Get("X-Trace-Id"))
	for _, entry := range logs.All() {
		t.Logf("log=%s fields=%v", entry.Message, entry.ContextMap())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
