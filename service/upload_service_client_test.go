package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func TestUploadServiceClientCreateGetCompleteAndAbort(t *testing.T) {
	var requestCount int
	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Header.Get("X-Internal-Token") != "internal-token" {
			t.Fatalf("X-Internal-Token = %q, want internal-token", r.Header.Get("X-Internal-Token"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/upload/sessions":
			writeJSON(t, w, http.StatusCreated, map[string]interface{}{
				"data": map[string]interface{}{
					"session_id":               "upl-123",
					"upload_mode":              "multipart",
					"upload_status":            "pending",
					"part_upload_url_template": serverURL + "/upload/sessions/upl-123/parts/{part_no}",
					"complete_url":             serverURL + "/upload/sessions/upl-123/complete",
					"abort_url":                serverURL + "/upload/sessions/upl-123/abort",
					"part_size_hint":           8388608,
					"expires_at":               "2026-03-15T09:00:00Z",
					"task_ref":                 "T-1",
					"asset_no":                 "AST-0001",
					"version_no":               1,
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/upload/sessions/upl-123":
			writeJSON(t, w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"session_id":     "upl-123",
					"remote_file_id": "file-123",
					"upload_status":  "completed",
					"last_synced_at": "2026-03-14T09:05:00Z",
					"storage_key":    "nas/design-assets/file-123",
					"file_size":      4096,
					"mime_type":      "image/vnd.adobe.photoshop",
					"upload_mode":    "multipart",
					"uploaded_at":    "2026-03-14T09:10:00Z",
					"complete_url":   serverURL + "/upload/sessions/upl-123/complete",
					"abort_url":      serverURL + "/upload/sessions/upl-123/abort",
					"upload_url":     serverURL + "/upload/sessions/upl-123",
					"part_size_hint": 8388608,
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/upload/sessions/upl-123/complete":
			writeJSON(t, w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"remote_file_id": "file-123",
					"storage_key":    "nas/design-assets/file-123",
					"file_size":      4096,
					"file_hash":      "hash-123",
					"mime_type":      "image/vnd.adobe.photoshop",
					"uploaded_at":    "2026-03-14T09:10:00Z",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/upload/sessions/upl-123/abort":
			writeJSON(t, w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{"status": "aborted"}})
		case r.Method == http.MethodGet && (r.URL.Path == "/upload/sessions/upl-123/file-meta" || r.URL.Path == "/v1/files/file-123/meta"):
			writeJSON(t, w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"file_id":     "file-123",
					"storage_key": "nas/design-assets/file-123",
					"file_size":   4096,
					"file_hash":   "hash-123",
					"mime_type":   "image/vnd.adobe.photoshop",
					"uploaded_at": "2026-03-14T09:10:00Z",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewUploadServiceClient(UploadServiceClientConfig{
		Enabled:         true,
		BaseURL:         server.URL,
		Timeout:         5 * time.Second,
		InternalToken:   "internal-token",
		StorageProvider: "nas",
	})

	created, err := client.CreateUploadSession(context.Background(), RemoteCreateUploadSessionRequest{
		TaskID:       1,
		AssetType:    domain.TaskAssetTypeOriginal,
		UploadMode:   domain.DesignAssetUploadModeMultipart,
		Filename:     "hero.psd",
		ExpectedSize: uploadRequestInt64Ptr(4096),
		MimeType:     "image/vnd.adobe.photoshop",
		CreatedBy:    501,
	})
	if err != nil {
		t.Fatalf("CreateUploadSession() error = %v", err)
	}
	if created.UploadID != "upl-123" || created.PartSizeHint != 8388608 {
		t.Fatalf("CreateUploadSession() = %+v", created)
	}
	if created.Headers != nil {
		t.Fatalf("CreateUploadSession() headers = %+v, want no internal auth headers for browser", created.Headers)
	}

	session, err := client.GetUploadSession(context.Background(), RemoteGetUploadSessionRequest{RemoteUploadID: "upl-123"})
	if err != nil {
		t.Fatalf("GetUploadSession() error = %v", err)
	}
	if session.SessionStatus != domain.DesignAssetSessionStatusCompleted || session.FileID == nil || *session.FileID != "file-123" {
		t.Fatalf("GetUploadSession() = %+v", session)
	}

	meta, err := client.CompleteUploadSession(context.Background(), RemoteCompleteUploadRequest{
		RemoteUploadID: "upl-123",
		Filename:       "hero.psd",
		ExpectedSize:   uploadRequestInt64Ptr(4096),
		MimeType:       "image/vnd.adobe.photoshop",
	})
	if err != nil {
		t.Fatalf("CompleteUploadSession() error = %v", err)
	}
	if meta.FileID == nil || *meta.FileID != "file-123" || meta.StorageKey != "nas/design-assets/file-123" {
		t.Fatalf("CompleteUploadSession() = %+v", meta)
	}

	fileMeta, err := client.GetFileMeta(context.Background(), RemoteGetFileMetaRequest{RemoteFileID: "file-123"})
	if err != nil {
		t.Fatalf("GetFileMeta() error = %v", err)
	}
	if fileMeta.FileHash == nil || *fileMeta.FileHash != "hash-123" {
		t.Fatalf("GetFileMeta() = %+v", fileMeta)
	}

	if err := client.AbortUploadSession(context.Background(), RemoteAbortUploadRequest{RemoteUploadID: "upl-123"}); err != nil {
		t.Fatalf("AbortUploadSession() error = %v", err)
	}
	if requestCount != 5 {
		t.Fatalf("requestCount = %d, want 5", requestCount)
	}
}

func TestUploadServiceClientMultipartTargetsRebasePrivateAbsoluteURLsToBrowserBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/upload/sessions" {
			writeJSON(t, w, http.StatusCreated, map[string]interface{}{
				"data": map[string]interface{}{
					"session_id":               "upl-lan-1",
					"upload_mode":              "multipart",
					"upload_status":            "pending",
					"upload_url":               "http://100.111.214.38:8089/upload/sessions/upl-lan-1",
					"part_upload_url_template": "http://100.111.214.38:8089/upload/sessions/upl-lan-1/parts/{part_no}",
					"complete_url":             "http://100.111.214.38:8089/upload/sessions/upl-lan-1/complete",
					"abort_url":                "http://100.111.214.38:8089/upload/sessions/upl-lan-1/abort",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewUploadServiceClient(UploadServiceClientConfig{
		Enabled:                 true,
		BaseURL:                 server.URL,
		BrowserMultipartBaseURL: "http://192.168.0.125:8089",
		Timeout:                 5 * time.Second,
		InternalToken:           "internal-token",
		StorageProvider:         "nas",
	})

	plan, err := client.CreateUploadSession(context.Background(), RemoteCreateUploadSessionRequest{
		TaskID:       1,
		AssetType:    domain.TaskAssetTypeDelivery,
		UploadMode:   domain.DesignAssetUploadModeMultipart,
		Filename:     "delivery.psd",
		ExpectedSize: uploadRequestInt64Ptr(4096),
		MimeType:     "application/octet-stream",
		CreatedBy:    99,
	})
	if err != nil {
		t.Fatalf("CreateUploadSession() error = %v", err)
	}
	if plan.BaseURL != "http://192.168.0.125:8089/" {
		t.Fatalf("plan.BaseURL = %q, want rebased browser base", plan.BaseURL)
	}
	if plan.UploadURL != "http://192.168.0.125:8089/upload/sessions/upl-lan-1" {
		t.Fatalf("plan.UploadURL = %q", plan.UploadURL)
	}
	if !strings.HasPrefix(plan.PartUploadURLTemplate, "http://192.168.0.125:8089/upload/sessions/upl-lan-1/parts/") {
		t.Fatalf("plan.PartUploadURLTemplate = %q", plan.PartUploadURLTemplate)
	}
	if plan.CompleteURL != "http://192.168.0.125:8089/upload/sessions/upl-lan-1/complete" {
		t.Fatalf("plan.CompleteURL = %q", plan.CompleteURL)
	}
	if plan.AbortURL != "http://192.168.0.125:8089/upload/sessions/upl-lan-1/abort" {
		t.Fatalf("plan.AbortURL = %q", plan.AbortURL)
	}
	if plan.Headers != nil {
		t.Fatalf("plan.Headers = %+v, want no internal auth headers for browser", plan.Headers)
	}
}

func TestUploadServiceClientMultipartTargetsDropPrivateHostWhenBrowserBaseIsRelative(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/upload/sessions" {
			writeJSON(t, w, http.StatusCreated, map[string]interface{}{
				"data": map[string]interface{}{
					"session_id":               "upl-relative-1",
					"upload_mode":              "multipart",
					"upload_status":            "pending",
					"upload_url":               "http://192.168.0.125:8089/upload/sessions/upl-relative-1",
					"part_upload_url_template": "http://192.168.0.125:8089/upload/sessions/upl-relative-1/parts/{part_no}",
					"complete_url":             "http://192.168.0.125:8089/upload/sessions/upl-relative-1/complete",
					"abort_url":                "http://192.168.0.125:8089/upload/sessions/upl-relative-1/abort",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewUploadServiceClient(UploadServiceClientConfig{
		Enabled:                 true,
		BaseURL:                 server.URL,
		BrowserMultipartBaseURL: "/",
		Timeout:                 5 * time.Second,
		InternalToken:           "internal-token",
		StorageProvider:         "oss",
	})

	plan, err := client.CreateUploadSession(context.Background(), RemoteCreateUploadSessionRequest{
		TaskID:       1,
		AssetType:    domain.TaskAssetTypeDelivery,
		UploadMode:   domain.DesignAssetUploadModeMultipart,
		Filename:     "delivery.psd",
		ExpectedSize: uploadRequestInt64Ptr(4096),
		MimeType:     "application/octet-stream",
		CreatedBy:    99,
	})
	if err != nil {
		t.Fatalf("CreateUploadSession() error = %v", err)
	}
	if plan.BaseURL != "/" {
		t.Fatalf("plan.BaseURL = %q, want relative browser base", plan.BaseURL)
	}
	if plan.UploadURL != "/upload/sessions/upl-relative-1" {
		t.Fatalf("plan.UploadURL = %q", plan.UploadURL)
	}
	if !strings.HasPrefix(plan.PartUploadURLTemplate, "/upload/sessions/upl-relative-1/parts/") {
		t.Fatalf("plan.PartUploadURLTemplate = %q", plan.PartUploadURLTemplate)
	}
	if plan.CompleteURL != "/upload/sessions/upl-relative-1/complete" {
		t.Fatalf("plan.CompleteURL = %q", plan.CompleteURL)
	}
	if plan.AbortURL != "/upload/sessions/upl-relative-1/abort" {
		t.Fatalf("plan.AbortURL = %q", plan.AbortURL)
	}
}

func TestUploadServiceClientSinglePartTargetsDropPrivateHostWhenBrowserBaseIsRelative(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/upload/sessions" {
			writeJSON(t, w, http.StatusCreated, map[string]interface{}{
				"data": map[string]interface{}{
					"session_id":    "upl-small-relative-1",
					"upload_mode":   "small",
					"upload_status": "pending",
					"base_url":      "http://100.111.214.38:8089",
					"upload_url":    "http://100.111.214.38:8089/upload/files",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewUploadServiceClient(UploadServiceClientConfig{
		Enabled:                 true,
		BaseURL:                 server.URL,
		BrowserMultipartBaseURL: "/",
		Timeout:                 5 * time.Second,
		InternalToken:           "internal-token",
		StorageProvider:         "oss",
	})

	plan, err := client.CreateUploadSession(context.Background(), RemoteCreateUploadSessionRequest{
		TaskID:       1,
		AssetType:    domain.TaskAssetTypeReference,
		UploadMode:   domain.DesignAssetUploadModeSmall,
		Filename:     "reference.txt",
		ExpectedSize: uploadRequestInt64Ptr(24),
		MimeType:     "text/plain",
		CreatedBy:    99,
	})
	if err != nil {
		t.Fatalf("CreateUploadSession() error = %v", err)
	}
	if plan.BaseURL != "/" {
		t.Fatalf("plan.BaseURL = %q, want relative browser base", plan.BaseURL)
	}
	if plan.UploadURL != "/upload/files" {
		t.Fatalf("plan.UploadURL = %q, want browser-safe relative upload_url", plan.UploadURL)
	}
	if plan.Method != http.MethodPost {
		t.Fatalf("plan.Method = %q, want POST", plan.Method)
	}
	if plan.Headers != nil {
		t.Fatalf("plan.Headers = %+v, want no internal auth headers for browser", plan.Headers)
	}
}

func TestUploadServiceClientProbeStoredFileEscapesStorageKeyPath(t *testing.T) {
	const storageKey = "tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/💚97% 能量充满啦.jpg"
	var requestPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.EscapedPath()
		_, _ = io.WriteString(w, "hello world!")
	}))
	defer server.Close()

	client := NewUploadServiceClient(UploadServiceClientConfig{
		Enabled:         true,
		BaseURL:         server.URL,
		Timeout:         5 * time.Second,
		InternalToken:   "internal-token",
		StorageProvider: "nas",
	})

	probe, err := client.ProbeStoredFile(context.Background(), RemoteProbeStoredFileRequest{StorageKey: storageKey})
	if err != nil {
		t.Fatalf("ProbeStoredFile() error = %v", err)
	}
	if probe == nil || probe.BytesRead != int64(len("hello world!")) {
		t.Fatalf("ProbeStoredFile() = %+v", probe)
	}
	const wantPath = "/files/tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/%F0%9F%92%9A97%25%20%E8%83%BD%E9%87%8F%E5%85%85%E6%BB%A1%E5%95%A6.jpg"
	if requestPath != wantPath {
		t.Fatalf("requestPath = %q, want %q", requestPath, wantPath)
	}
}

func TestUploadServiceClientSmallSessionUploadUsesPOSTWhenMethodMissing(t *testing.T) {
	var uploadMethod string
	var contentType string
	var contentLength int64
	var transferEncoding []string
	var filePartContentType string
	fields := map[string]string{}
	var fileBytes []byte
	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/upload/sessions":
			writeJSON(t, w, http.StatusCreated, map[string]interface{}{
				"data": map[string]interface{}{
					"session_id":    "upl-small-1",
					"upload_mode":   "small",
					"upload_status": "created",
					"upload_url":    serverURL + "/upload/files",
				},
			})
		case r.URL.Path == "/upload/files":
			uploadMethod = r.Method
			contentType = r.Header.Get("Content-Type")
			contentLength = r.ContentLength
			transferEncoding = append([]string{}, r.TransferEncoding...)
			reader, err := r.MultipartReader()
			if err != nil {
				t.Fatalf("MultipartReader() error = %v", err)
			}
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("NextPart() error = %v", err)
				}
				body, err := io.ReadAll(part)
				if err != nil {
					t.Fatalf("ReadAll(part) error = %v", err)
				}
				if part.FormName() == "file" {
					filePartContentType = part.Header.Get("Content-Type")
					fileBytes = append([]byte{}, body...)
					continue
				}
				fields[part.FormName()] = string(body)
			}
			writeJSON(t, w, http.StatusCreated, map[string]interface{}{
				"data": map[string]interface{}{
					"file_id":     "file-small-1",
					"storage_key": "tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/reference.png",
					"file_size":   12,
					"mime_type":   "image/png",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewUploadServiceClient(UploadServiceClientConfig{
		Enabled:         true,
		BaseURL:         server.URL,
		Timeout:         5 * time.Second,
		InternalToken:   "internal-token",
		StorageProvider: "nas",
	})

	plan, err := client.CreateUploadSession(context.Background(), RemoteCreateUploadSessionRequest{
		TaskID:       1,
		TaskRef:      "task-create-reference",
		AssetNo:      "PRECREATE-REFERENCE",
		AssetType:    domain.TaskAssetTypeReference,
		UploadMode:   domain.DesignAssetUploadModeSmall,
		Filename:     "reference.png",
		ExpectedSize: uploadRequestInt64Ptr(12),
		MimeType:     "image/png",
		CreatedBy:    9,
	})
	if err != nil {
		t.Fatalf("CreateUploadSession() error = %v", err)
	}
	if plan.Method != http.MethodPost {
		t.Fatalf("plan.Method = %q, want POST", plan.Method)
	}

	meta, err := client.UploadFileToSession(context.Background(), RemoteSessionFileUploadRequest{
		UploadURL:      plan.UploadURL,
		Method:         plan.Method,
		Headers:        plan.Headers,
		RemoteUploadID: plan.UploadID,
		TaskRef:        "task-create-reference",
		AssetNo:        "PRECREATE-REFERENCE",
		AssetType:      domain.TaskAssetTypeReference,
		VersionNo:      1,
		Filename:       "reference.png",
		MimeType:       "image/png",
		ExpectedSize:   uploadRequestInt64Ptr(12),
		CreatedBy:      9,
		File:           bytes.NewBufferString("hello world!"),
		FileFieldName:  "file",
	})
	if err != nil {
		t.Fatalf("UploadFileToSession() error = %v", err)
	}
	if meta == nil {
		t.Fatal("UploadFileToSession() meta = nil")
	}
	if valueOrEmpty(meta.FileID) != "file-small-1" {
		t.Fatalf("UploadFileToSession() file_id = %q", valueOrEmpty(meta.FileID))
	}
	if uploadMethod != http.MethodPost {
		t.Fatalf("upload method = %q, want POST", uploadMethod)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Fatalf("content type = %q, want multipart/form-data", contentType)
	}
	if contentLength <= 0 {
		t.Fatalf("content length = %d, want > 0", contentLength)
	}
	if len(transferEncoding) != 0 {
		t.Fatalf("transfer encoding = %v, want none", transferEncoding)
	}
	if filePartContentType != "image/png" {
		t.Fatalf("file part content type = %q, want image/png", filePartContentType)
	}
	if string(fileBytes) != "hello world!" {
		t.Fatalf("file bytes = %q", string(fileBytes))
	}
	if fields["task_ref"] != "task-create-reference" {
		t.Fatalf("task_ref = %q", fields["task_ref"])
	}
	if fields["asset_no"] != "PRECREATE-REFERENCE" {
		t.Fatalf("asset_no = %q", fields["asset_no"])
	}
	if fields["expected_size"] != "12" || fields["file_size"] != "12" {
		t.Fatalf("size fields = %+v", fields)
	}
}

func TestUploadServiceClientCreateSessionPrefersSignedBrowserDirectURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/upload/sessions" {
			writeJSON(t, w, http.StatusCreated, map[string]interface{}{
				"data": map[string]interface{}{
					"session_id":          "upl-direct-1",
					"upload_mode":         "multipart",
					"upload_url":          "http://192.168.0.125:8089/upload/sessions/upl-direct-1",
					"direct_upload_url":   "https://oss-example.oss-cn-hangzhou.aliyuncs.com/tasks/T-1/A-1?signature=abc",
					"direct_complete_url": "https://oss-example.oss-cn-hangzhou.aliyuncs.com/tasks/T-1/A-1?complete=1",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewUploadServiceClient(UploadServiceClientConfig{
		Enabled:                 true,
		BaseURL:                 server.URL,
		BrowserMultipartBaseURL: "/",
		Timeout:                 5 * time.Second,
		InternalToken:           "internal-token",
		StorageProvider:         "oss",
	})
	plan, err := client.CreateUploadSession(context.Background(), RemoteCreateUploadSessionRequest{
		TaskID:       1,
		AssetType:    domain.TaskAssetTypeDelivery,
		UploadMode:   domain.DesignAssetUploadModeMultipart,
		Filename:     "delivery.psd",
		ExpectedSize: uploadRequestInt64Ptr(4096),
		MimeType:     "application/octet-stream",
		CreatedBy:    99,
	})
	if err != nil {
		t.Fatalf("CreateUploadSession() error = %v", err)
	}
	if !strings.HasPrefix(plan.UploadURL, "https://oss-example.oss-cn-hangzhou.aliyuncs.com/") {
		t.Fatalf("plan.UploadURL = %q, want signed direct OSS URL", plan.UploadURL)
	}
}

func TestUploadServiceClientBuildBrowserFileURLUsesDirectDownloadBase(t *testing.T) {
	client := NewUploadServiceClient(UploadServiceClientConfig{
		Enabled:                true,
		BaseURL:                "http://127.0.0.1:8092",
		BrowserDownloadBaseURL: "https://oss-example.oss-cn-hangzhou.aliyuncs.com",
		Timeout:                5 * time.Second,
		StorageProvider:        "oss",
	})
	got := client.BuildBrowserFileURL("tasks/T001/assets/A001/v1/delivery/image.png")
	if got == nil || *got != "https://oss-example.oss-cn-hangzhou.aliyuncs.com/tasks/T001/assets/A001/v1/delivery/image.png" {
		t.Fatalf("BuildBrowserFileURL() = %v", got)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, payload interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode json: %v", err)
	}
}
