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

func TestAssetFilesHandlerServeFileRedirectsToOSSDirectWithoutProbingUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalled = true
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
	if upstreamCalled {
		t.Fatal("legacy upload-service upstream must not be probed when OSS direct signing is available")
	}
}

func TestAssetFilesHandlerServeFileRejectsUnknownStorageKeyWhenAccessPolicyConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		t.Fatalf("upstream must not be called for unauthorized storage key")
	}))
	defer upstream.Close()

	router := gin.New()
	h := NewAssetFilesHandler(upstream.URL, "oss-token", "oss", zap.NewNop())
	h.SetFileAccessPolicy(
		assetFilesTaskRepoStub{},
		assetFilesTaskAssetRepoStub{},
		assetFilesStorageRefRepoStub{},
		nil,
	)
	router.GET("/v1/assets/files/*path", h.ServeFile)

	req := httptest.NewRequest(http.MethodGet, "/v1/assets/files/tasks/secret/file.png", nil)
	req = req.WithContext(domain.WithRequestActor(req.Context(), assetFilesSessionActor()))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 body=%s", rec.Code, rec.Body.String())
	}
	if upstreamCalled {
		t.Fatal("upstream was called for unauthorized storage key")
	}
}

func TestAssetFilesHandlerServeFileAllowsAuthorizedTaskAssetStorageKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const storageKey = "tasks/RW-1/assets/AST-1/v1/delivery/file.png"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/files/"+storageKey {
			t.Fatalf("upstream path = %q, want /files/%s", got, storageKey)
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	router := gin.New()
	h := NewAssetFilesHandler(upstream.URL, "oss-token", "oss", zap.NewNop())
	h.SetFileAccessPolicy(
		assetFilesTaskRepoStub{tasks: map[int64]*domain.Task{101: {ID: 101}}},
		assetFilesTaskAssetRepoStub{
			assetsByKey: map[string]*domain.TaskAsset{
				storageKey: {ID: 501, TaskID: 101},
			},
		},
		assetFilesStorageRefRepoStub{},
		nil,
	)
	router.GET("/v1/assets/files/*path", h.ServeFile)

	req := httptest.NewRequest(http.MethodGet, "/v1/assets/files/"+storageKey, nil)
	req = req.WithContext(domain.WithRequestActor(req.Context(), assetFilesSessionActor()))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q, want ok", rec.Body.String())
	}
}

func TestAssetFilesHandlerServeFileAllowsAttachedTaskCreateReferenceViewer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		storageKey = "tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/reference.zip"
		refID      = "ref-precreate-1"
		taskID     = int64(2171)
		designerID = int64(228)
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "zip-bytes")
	}))
	defer upstream.Close()

	h := NewAssetFilesHandler(upstream.URL, "oss-token", "nas", zap.NewNop())
	h.SetFileAccessPolicy(
		assetFilesTaskRepoStub{tasks: map[int64]*domain.Task{
			taskID: {ID: taskID, DesignerID: assetFilesInt64Ptr(designerID)},
		}},
		assetFilesTaskAssetRepoStub{},
		assetFilesStorageRefRepoStub{
			refsByKey: map[string]*domain.AssetStorageRef{
				storageKey: {
					RefID:     refID,
					OwnerType: domain.AssetOwnerTypeTaskCreateReference,
					OwnerID:   249,
					RefKey:    storageKey,
				},
			},
			taskIDsByRef: map[string][]int64{refID: {taskID}},
		},
		nil,
	)
	router := gin.New()
	router.GET("/v1/assets/files/*path", h.ServeFile)

	req := httptest.NewRequest(http.MethodGet, "/v1/assets/files/"+storageKey, nil)
	req = req.WithContext(domain.WithRequestActor(req.Context(), assetFilesTaskViewActor(designerID, domain.AccessScopeSelf)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "zip-bytes" {
		t.Fatalf("body = %q, want zip-bytes", rec.Body.String())
	}
}

func TestAssetFilesHandlerServeFileRejectsUnattachedTaskCreateReferenceViewer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const storageKey = "tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/unattached.zip"
	h := NewAssetFilesHandler("http://example.invalid", "oss-token", "nas", zap.NewNop())
	h.SetFileAccessPolicy(
		assetFilesTaskRepoStub{},
		assetFilesTaskAssetRepoStub{},
		assetFilesStorageRefRepoStub{refsByKey: map[string]*domain.AssetStorageRef{
			storageKey: {
				RefID:     "ref-unattached",
				OwnerType: domain.AssetOwnerTypeTaskCreateReference,
				OwnerID:   249,
				RefKey:    storageKey,
			},
		}},
		nil,
	)
	router := gin.New()
	router.GET("/v1/assets/files/*path", h.ServeFile)

	req := httptest.NewRequest(http.MethodGet, "/v1/assets/files/"+storageKey, nil)
	req = req.WithContext(domain.WithRequestActor(req.Context(), domain.RequestActor{
		ID:       228,
		Username: "designer",
		Roles:    []domain.Role{domain.RoleDesigner},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 body=%s", rec.Code, rec.Body.String())
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
	assets      map[int64]*domain.TaskAsset
	assetsByKey map[string]*domain.TaskAsset
}

func (r assetFilesTaskAssetRepoStub) Create(context.Context, repo.Tx, *domain.TaskAsset) (int64, error) {
	return 0, nil
}

func (r assetFilesTaskAssetRepoStub) GetByID(_ context.Context, id int64) (*domain.TaskAsset, error) {
	return r.assets[id], nil
}

func (r assetFilesTaskAssetRepoStub) GetByStorageKey(_ context.Context, storageKey string) (*domain.TaskAsset, error) {
	return r.assetsByKey[storageKey], nil
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

type assetFilesStorageRefRepoStub struct {
	refsByKey     map[string]*domain.AssetStorageRef
	taskIDsByRef  map[string][]int64
	attachedTasks error
}

func (r assetFilesStorageRefRepoStub) GetByRefKey(_ context.Context, refKey string) (*domain.AssetStorageRef, error) {
	return r.refsByKey[refKey], nil
}

func (r assetFilesStorageRefRepoStub) ListAttachedTaskIDsByRefID(_ context.Context, refID string) ([]int64, error) {
	if r.attachedTasks != nil {
		return nil, r.attachedTasks
	}
	return r.taskIDsByRef[refID], nil
}

type assetFilesTaskRepoStub struct {
	tasks map[int64]*domain.Task
}

func (r assetFilesTaskRepoStub) Create(context.Context, repo.Tx, *domain.Task, *domain.TaskDetail) (int64, error) {
	return 0, nil
}

func (r assetFilesTaskRepoStub) CreateSKUItems(context.Context, repo.Tx, []*domain.TaskSKUItem) error {
	return nil
}

func (r assetFilesTaskRepoStub) GetByID(_ context.Context, id int64) (*domain.Task, error) {
	return r.tasks[id], nil
}

func (r assetFilesTaskRepoStub) GetDetailByTaskID(context.Context, int64) (*domain.TaskDetail, error) {
	return nil, nil
}

func (r assetFilesTaskRepoStub) GetSKUItemBySKUCode(context.Context, string) (*domain.TaskSKUItem, error) {
	return nil, nil
}

func (r assetFilesTaskRepoStub) ListSKUItemsByTaskID(context.Context, int64) ([]*domain.TaskSKUItem, error) {
	return nil, nil
}

func (r assetFilesTaskRepoStub) List(context.Context, repo.TaskListFilter) ([]*domain.TaskListItem, int64, error) {
	return nil, 0, nil
}

func (r assetFilesTaskRepoStub) UpdateDetailBusinessInfo(context.Context, repo.Tx, *domain.TaskDetail) error {
	return nil
}

func (r assetFilesTaskRepoStub) UpdatePriority(context.Context, repo.Tx, int64, domain.TaskPriority) error {
	return nil
}

func (r assetFilesTaskRepoStub) UpdateProductBinding(context.Context, repo.Tx, *domain.Task) error {
	return nil
}

func (r assetFilesTaskRepoStub) UpdateStatus(context.Context, repo.Tx, int64, domain.TaskStatus) error {
	return nil
}

func (r assetFilesTaskRepoStub) UpdateDesigner(context.Context, repo.Tx, int64, *int64) error {
	return nil
}

func (r assetFilesTaskRepoStub) UpdateHandler(context.Context, repo.Tx, int64, *int64) error {
	return nil
}

func (r assetFilesTaskRepoStub) UpdateCustomizationState(context.Context, repo.Tx, int64, *int64, string, string) error {
	return nil
}

func assetFilesSessionActor() domain.RequestActor {
	return assetFilesTaskViewActor(1, domain.AccessScopeGlobal)
}

func assetFilesTaskViewActor(id int64, scope domain.AccessScopeMode) domain.RequestActor {
	assignment := domain.AccessAssignment{
		UserID: id, RoleID: 71, RoleCode: "task_viewer", ScopeMode: scope, SourceType: "direct",
	}
	return domain.RequestActor{
		ID: id, Username: "task-viewer", Source: domain.RequestActorSourceSessionToken,
		AuthMode:    domain.AuthModeSessionTokenRoleEnforced,
		Permissions: []domain.PermissionCode{domain.PermissionTaskView},
		EffectiveAccess: &domain.EffectiveAccess{
			UserID: id, Permissions: []domain.PermissionCode{domain.PermissionTaskView},
			Assignments: []domain.AccessAssignment{assignment},
			Sources: []domain.EffectiveAccessNote{{
				Permission: domain.PermissionTaskView, RoleID: assignment.RoleID,
				RoleCode: assignment.RoleCode, SourceType: assignment.SourceType, ScopeMode: scope,
			}},
		},
	}
}

func assetFilesInt64Ptr(value int64) *int64 { return &value }

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
