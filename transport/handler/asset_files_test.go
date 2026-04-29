package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"workflow/domain"
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
