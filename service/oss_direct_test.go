package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func newTestOSSDirectService() *OSSDirectService {
	return NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        "oss-cn-hangzhou.aliyuncs.com",
		Bucket:          "test-bucket",
		AccessKeyID:     "LTAI5tTestKeyID",
		AccessKeySecret: "TestSecretKeyXYZ",
		PresignExpiry:   15 * time.Minute,
		PublicEndpoint:  "oss-cn-hangzhou.aliyuncs.com",
		PartSize:        10 * 1024 * 1024,
	})
}

func TestOSSDirectService_Enabled(t *testing.T) {
	svc := newTestOSSDirectService()
	if !svc.Enabled() {
		t.Fatal("expected enabled=true")
	}

	disabled := NewOSSDirectService(OSSDirectConfig{Enabled: false})
	if disabled.Enabled() {
		t.Fatal("expected enabled=false")
	}

	missingBucket := NewOSSDirectService(OSSDirectConfig{
		Enabled:  true,
		Endpoint: "oss-cn-hangzhou.aliyuncs.com",
		Bucket:   "",
	})
	if missingBucket.Enabled() {
		t.Fatal("expected enabled=false when bucket is empty")
	}
}

func TestBuildObjectKey(t *testing.T) {
	svc := newTestOSSDirectService()
	key := svc.BuildObjectKey("TASK-001", "A0001", 1, domain.TaskAssetTypeDelivery, "design.psd")
	if !strings.HasPrefix(key, "tasks/TASK-001/assets/A0001/v1/") {
		t.Fatalf("unexpected key prefix: %s", key)
	}
	if !strings.HasSuffix(key, ".psd") {
		t.Fatalf("unexpected key suffix: %s", key)
	}
	if strings.Contains(key, "design") {
		t.Fatalf("original filename leaked into key: %s", key)
	}
}

func TestBuildObjectKey_ASCIIStorageFilename(t *testing.T) {
	svc := newTestOSSDirectService()
	safeKeyPattern := regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	tests := []struct {
		name      string
		filename  string
		wantExt   string
		forbidden []string
	}{
		{name: "plus", filename: "a+b.png", wantExt: ".png", forbidden: []string{"a+b"}},
		{name: "unicode_plus", filename: "手淘_SKU_13_蒙的都对【送12色涂鸦笔+木架】.jpg", wantExt: ".jpg", forbidden: []string{"手淘", "+", "木架"}},
		{name: "spaces", filename: "file with spaces.jpg", wantExt: ".jpg", forbidden: []string{"file with spaces", " "}},
		{name: "query_like", filename: "weird&name=value.png", wantExt: ".png", forbidden: []string{"weird", "&", "="}},
		{name: "no_extension", filename: "NO_EXTENSION", wantExt: "", forbidden: []string{"NO_EXTENSION"}},
		{name: "double_dots", filename: "double..dots..png", wantExt: ".png", forbidden: []string{"double", ".."}},
		{name: "emoji", filename: "🎨.png", wantExt: ".png", forbidden: []string{"🎨"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := svc.BuildObjectKey("T1", "A1", 2, domain.TaskAssetTypeSource, tt.filename)
			key2 := svc.BuildObjectKey("T1", "A1", 2, domain.TaskAssetTypeSource, tt.filename)
			if !strings.HasPrefix(key1, "tasks/T1/assets/A1/v2/source/") {
				t.Fatalf("unexpected key prefix: %s", key1)
			}
			if !safeKeyPattern.MatchString(key1) {
				t.Fatalf("key contains unsafe characters: %s", key1)
			}
			if key1 == key2 {
				t.Fatalf("expected unique keys for repeated input, got %s", key1)
			}
			filename := key1[strings.LastIndex(key1, "/")+1:]
			if tt.wantExt != "" && !strings.HasSuffix(filename, tt.wantExt) {
				t.Fatalf("filename = %s, want suffix %s", filename, tt.wantExt)
			}
			if tt.wantExt == "" && strings.Contains(filename, ".") {
				t.Fatalf("filename = %s, want no extension", filename)
			}
			for _, forbidden := range tt.forbidden {
				if forbidden != "" && strings.Contains(key1, forbidden) {
					t.Fatalf("key %q contains forbidden original fragment %q", key1, forbidden)
				}
			}
		})
	}
}

func TestResolveAssetDownloadFilename(t *testing.T) {
	tests := []struct {
		name             string
		originalFilename string
		fileName         string
		assetID          int64
		want             string
	}{
		{
			name:             "original filename wins",
			originalFilename: " 原始文件.psd ",
			fileName:         "storage-name.psd",
			assetID:          42,
			want:             "原始文件.psd",
		},
		{
			name:     "file name fallback",
			fileName: " storage-name.psd ",
			assetID:  42,
			want:     "storage-name.psd",
		},
		{
			name:    "asset id fallback",
			assetID: 42,
			want:    "asset-42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAssetDownloadFilename(tt.originalFilename, tt.fileName, tt.assetID)
			if got != tt.want {
				t.Fatalf("ResolveAssetDownloadFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPresignDownloadURL(t *testing.T) {
	svc := newTestOSSDirectService()
	info := svc.PresignDownloadURL("tasks/T1/assets/A1/v1/delivery/test.psd")
	if info == nil {
		t.Fatal("expected non-nil download info")
	}
	if info.DownloadURL == "" {
		t.Fatal("expected non-empty download URL")
	}
	if info.ExpiresAt.Before(time.Now()) {
		t.Fatal("expected expires_at in the future")
	}

	u, err := url.Parse(info.DownloadURL)
	if err != nil {
		t.Fatalf("invalid download URL: %v", err)
	}
	if u.Query().Get("OSSAccessKeyId") != "LTAI5tTestKeyID" {
		t.Fatal("expected OSSAccessKeyId in URL")
	}
	if u.Query().Get("Signature") == "" {
		t.Fatal("expected Signature in URL")
	}
	if u.Query().Get("Expires") == "" {
		t.Fatal("expected Expires in URL")
	}
	if !strings.Contains(info.DownloadURL, "response-content-disposition") {
		t.Fatal("expected response-content-disposition in URL")
	}
	if !strings.Contains(info.DownloadURL, "attachment") {
		t.Fatal("expected attachment disposition for download")
	}
}

func TestPresignDownloadURLWithFilename(t *testing.T) {
	svc := newTestOSSDirectService()
	info := svc.PresignDownloadURLWithFilename("tasks/T1/assets/A1/v1/delivery/test.psd", "交付 文件.psd")
	if info == nil || info.DownloadURL == "" {
		t.Fatalf("PresignDownloadURLWithFilename() = %+v", info)
	}

	u, err := url.Parse(info.DownloadURL)
	if err != nil {
		t.Fatalf("invalid download URL: %v", err)
	}
	disposition := u.Query().Get("response-content-disposition")
	if disposition == "" {
		t.Fatal("expected response-content-disposition in URL")
	}
	if !strings.Contains(disposition, "attachment") {
		t.Fatalf("disposition = %q, want attachment", disposition)
	}
	if !strings.Contains(disposition, "filename*=") {
		t.Fatalf("disposition = %q, want encoded filename parameter", disposition)
	}
	if !strings.Contains(disposition, "%E4%BA%A4%E4%BB%98%20%E6%96%87%E4%BB%B6.psd") {
		t.Fatalf("disposition = %q, want encoded unicode filename", disposition)
	}
	expectedCanonicalResource := "/test-bucket/tasks/T1/assets/A1/v1/delivery/test.psd?response-content-disposition=" + disposition
	expectedSignature := svc.signV1(http.MethodGet, "", "", u.Query().Get("Expires"), "", expectedCanonicalResource)
	if got := u.Query().Get("Signature"); got != expectedSignature {
		t.Fatalf("signature = %q, want %q for canonical resource %q", got, expectedSignature, expectedCanonicalResource)
	}
}

func TestPresignPreviewURL(t *testing.T) {
	svc := newTestOSSDirectService()
	info := svc.PresignPreviewURL("tasks/T1/assets/A1/v1/delivery/test.jpg")
	if info == nil {
		t.Fatal("expected non-nil preview info")
	}
	if info.DownloadURL == "" {
		t.Fatal("expected non-empty preview URL")
	}
	if !strings.Contains(info.DownloadURL, "response-content-disposition") {
		t.Fatal("expected response-content-disposition in URL")
	}
	if !strings.Contains(info.DownloadURL, "inline") {
		t.Fatal("expected inline disposition for preview")
	}
}

func TestPresignPreviewURLWithProcess(t *testing.T) {
	svc := newTestOSSDirectService()
	info := svc.PresignPreviewURLWithProcess("tasks/T1/assets/A1/v1/source/test.tiff", "image/resize,w_1600,m_lfit/format,jpg")
	if info == nil || info.DownloadURL == "" {
		t.Fatalf("PresignPreviewURLWithProcess() = %+v", info)
	}
	if !strings.Contains(info.DownloadURL, "x-oss-process=") {
		t.Fatalf("preview url = %q, want x-oss-process", info.DownloadURL)
	}
	if !strings.Contains(info.DownloadURL, "resize%2Cw_1600%2Cm_lfit") {
		t.Fatalf("preview url = %q, want resize process", info.DownloadURL)
	}
	if !strings.Contains(info.DownloadURL, "format%2Cjpg") {
		t.Fatalf("preview url = %q, want format conversion", info.DownloadURL)
	}
}

func TestPresignPreviewURLWithProcessEmptyFallsBack(t *testing.T) {
	svc := newTestOSSDirectService()
	info := svc.PresignPreviewURLWithProcess("tasks/T1/assets/A1/v1/source/test.tiff", "")
	if info == nil || info.DownloadURL == "" {
		t.Fatalf("PresignPreviewURLWithProcess() = %+v", info)
	}
	if strings.Contains(info.DownloadURL, "x-oss-process=") {
		t.Fatalf("preview url = %q, unexpected x-oss-process", info.DownloadURL)
	}
}

func TestPresignDownloadURL_Disabled(t *testing.T) {
	svc := NewOSSDirectService(OSSDirectConfig{Enabled: false})
	info := svc.PresignDownloadURL("some/key")
	if info != nil {
		t.Fatal("expected nil when service disabled")
	}
}

func TestPresignDownloadURL_EmptyKey(t *testing.T) {
	svc := newTestOSSDirectService()
	info := svc.PresignDownloadURL("")
	if info != nil {
		t.Fatal("expected nil for empty key")
	}
	info = svc.PresignDownloadURL("   ")
	if info != nil {
		t.Fatal("expected nil for whitespace key")
	}
}

func TestCreateSingleUploadPlan(t *testing.T) {
	svc := newTestOSSDirectService()
	plan, err := svc.CreateSingleUploadPlan("tasks/T1/test.psd", "application/octet-stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Mode != "single_part" {
		t.Fatalf("expected mode=single_part, got %s", plan.Mode)
	}
	if plan.UploadURL == "" {
		t.Fatal("expected non-empty upload URL")
	}
	if plan.Method != "PUT" {
		t.Fatalf("expected method=PUT, got %s", plan.Method)
	}
	if plan.ObjectKey != "tasks/T1/test.psd" {
		t.Fatalf("unexpected object key: %s", plan.ObjectKey)
	}

	u, err := url.Parse(plan.UploadURL)
	if err != nil {
		t.Fatalf("invalid upload URL: %v", err)
	}
	if u.Query().Get("Signature") == "" {
		t.Fatal("expected Signature in URL")
	}
}

func TestPresignPartUploadURL_SignsDeclaredContentType(t *testing.T) {
	svc := newTestOSSDirectService()
	part := svc.presignPartUploadURL("tasks/T1/assets/A1/v1/delivery/test.psd", "UPLOAD123", 7, "application/octet-stream")

	u, err := url.Parse(part.UploadURL)
	if err != nil {
		t.Fatalf("invalid upload URL: %v", err)
	}
	query := u.Query()
	expires := query.Get("Expires")
	if expires == "" {
		t.Fatal("expected Expires in upload URL")
	}
	expected := svc.signV1(
		"PUT",
		"",
		"application/octet-stream",
		expires,
		"",
		"/test-bucket/tasks/T1/assets/A1/v1/delivery/test.psd?partNumber=7&uploadId=UPLOAD123",
	)
	if got := query.Get("Signature"); got != expected {
		t.Fatalf("signature = %q, want %q", got, expected)
	}
}

func TestPresignPartUploadURL_BlankContentTypeDefaultsConsistently(t *testing.T) {
	svc := newTestOSSDirectService()
	withoutContentType := svc.presignPartUploadURL("tasks/T1/assets/A1/v1/delivery/test.psd", "UPLOAD123", 1, "")
	withDefaultContentType := svc.presignPartUploadURL("tasks/T1/assets/A1/v1/delivery/test.psd", "UPLOAD123", 1, "application/octet-stream")

	withoutURL, err := url.Parse(withoutContentType.UploadURL)
	if err != nil {
		t.Fatalf("invalid upload URL without content type: %v", err)
	}
	withURL, err := url.Parse(withDefaultContentType.UploadURL)
	if err != nil {
		t.Fatalf("invalid upload URL with content type: %v", err)
	}
	if withoutURL.Query().Get("Signature") != withURL.Query().Get("Signature") {
		t.Fatal("expected blank content type to default to application/octet-stream")
	}
}

func TestCreateSingleUploadPlan_SignsDeclaredContentType(t *testing.T) {
	svc := newTestOSSDirectService()
	plan, err := svc.CreateSingleUploadPlan("tasks/T1/test.psd", "application/octet-stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u, err := url.Parse(plan.UploadURL)
	if err != nil {
		t.Fatalf("invalid upload URL: %v", err)
	}
	expires := u.Query().Get("Expires")
	expected := svc.signV1("PUT", "", "application/octet-stream", expires, "", "/test-bucket/tasks/T1/test.psd")
	if got := u.Query().Get("Signature"); got != expected {
		t.Fatalf("signature = %q, want %q", got, expected)
	}
}

func TestCreateSingleUploadPlan_DefaultsRequiredContentType(t *testing.T) {
	svc := newTestOSSDirectService()
	plan, err := svc.CreateSingleUploadPlan("tasks/T1/test.psd", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.RequiredContentType != "application/octet-stream" {
		t.Fatalf("required content type = %q", plan.RequiredContentType)
	}
	u, err := url.Parse(plan.UploadURL)
	if err != nil {
		t.Fatalf("invalid upload URL: %v", err)
	}
	expires := u.Query().Get("Expires")
	expected := svc.signV1("PUT", "", "application/octet-stream", expires, "", "/test-bucket/tasks/T1/test.psd")
	if got := u.Query().Get("Signature"); got != expected {
		t.Fatalf("signature = %q, want %q", got, expected)
	}
}

func TestSignV1_Deterministic(t *testing.T) {
	svc := newTestOSSDirectService()
	sig1 := svc.signV1("GET", "", "", "1700000000", "", "/test-bucket/key")
	sig2 := svc.signV1("GET", "", "", "1700000000", "", "/test-bucket/key")
	if sig1 != sig2 {
		t.Fatal("expected deterministic signatures")
	}

	sig3 := svc.signV1("PUT", "", "", "1700000000", "", "/test-bucket/key")
	if sig1 == sig3 {
		t.Fatal("expected different signature for different verb")
	}
}

func TestOSSEscapePath(t *testing.T) {
	result := ossEscapePath("tasks/T1/assets/A1/v1/delivery/file+name.psd")
	if strings.Contains(result, " ") {
		t.Fatalf("expected spaces to be escaped: %s", result)
	}
	if strings.Contains(result, "+") {
		t.Fatalf("expected plus signs to be escaped: %s", result)
	}
	if !strings.Contains(result, "file%2Bname.psd") {
		t.Fatalf("expected plus sign to be %%2B escaped: %s", result)
	}
	if !strings.Contains(result, "/") {
		t.Fatal("expected slashes to be preserved")
	}
}

func TestOSSDirectServiceOpenObjectSuccess(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.EscapedPath() != "/tasks/T1/assets/A1/v1/source/test.psd" {
			t.Fatalf("path = %s", r.URL.EscapedPath())
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "OSS LTAI5tTestKeyID:") {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, "oss-object-body")
	}))
	defer server.Close()

	svc := NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        strings.TrimPrefix(server.URL, "https://"),
		PublicEndpoint:  strings.TrimPrefix(server.URL, "https://"),
		Bucket:          "test-bucket",
		AccessKeyID:     "LTAI5tTestKeyID",
		AccessKeySecret: "TestSecretKeyXYZ",
		PresignExpiry:   15 * time.Minute,
	})
	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	httpClient := server.Client()
	httpClient.Transport = &rewriteHostTransport{
		base:  baseURL,
		inner: httpClient.Transport,
	}
	svc.httpClient = httpClient

	stream, err := svc.OpenObject(context.Background(), "tasks/T1/assets/A1/v1/source/test.psd")
	if err != nil {
		t.Fatalf("OpenObject() error = %v", err)
	}
	defer stream.Close()
	body, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read stream error = %v", err)
	}
	if string(body) != "oss-object-body" {
		t.Fatalf("body = %q, want oss-object-body", string(body))
	}
}

func TestOSSDirectServiceOpenObjectNon2xxReturnsError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "not found")
	}))
	defer server.Close()

	svc := NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        strings.TrimPrefix(server.URL, "https://"),
		PublicEndpoint:  strings.TrimPrefix(server.URL, "https://"),
		Bucket:          "test-bucket",
		AccessKeyID:     "LTAI5tTestKeyID",
		AccessKeySecret: "TestSecretKeyXYZ",
		PresignExpiry:   15 * time.Minute,
	})
	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	httpClient := server.Client()
	httpClient.Transport = &rewriteHostTransport{
		base:  baseURL,
		inner: httpClient.Transport,
	}
	svc.httpClient = httpClient

	stream, err := svc.OpenObject(context.Background(), "tasks/T1/missing.psd")
	if err == nil {
		if stream != nil {
			_ = stream.Close()
		}
		t.Fatal("OpenObject() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "status=404") {
		t.Fatalf("error = %v, want status=404", err)
	}
}
