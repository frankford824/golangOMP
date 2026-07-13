package service

import (
	"strings"
	"testing"
	"time"
)

func TestResolveLegacyReferenceDownloadPrefersOSSOverStoredProxyURL(t *testing.T) {
	svc := &taskAssetCenterService{ossDirectService: newTestOSSDirectService()}
	existingExpiry := time.Now().Add(-time.Hour)

	downloadURL, expiresAt := svc.resolveLegacyReferenceDownload(
		"/v1/assets/files/tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/reference.zip",
		"tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/reference.zip",
		"中文参考资料.zip",
		&existingExpiry,
	)

	if strings.HasPrefix(downloadURL, "/v1/assets/files/") {
		t.Fatalf("downloadURL = %q, want direct OSS URL", downloadURL)
	}
	if !strings.HasPrefix(downloadURL, "https://test-bucket.oss-cn-hangzhou.aliyuncs.com/") {
		t.Fatalf("downloadURL = %q, want test OSS host", downloadURL)
	}
	if !strings.Contains(downloadURL, "response-content-disposition") {
		t.Fatalf("downloadURL = %q, want attachment filename disposition", downloadURL)
	}
	if expiresAt == nil || !expiresAt.After(time.Now()) {
		t.Fatalf("expiresAt = %v, want fresh future expiry", expiresAt)
	}
}

func TestResolveLegacyReferenceDownloadRetainsProxyWhenOSSDisabled(t *testing.T) {
	svc := &taskAssetCenterService{}
	const existingURL = "/v1/assets/files/tasks/reference.zip"

	downloadURL, expiresAt := svc.resolveLegacyReferenceDownload(existingURL, "tasks/reference.zip", "reference.zip", nil)

	if downloadURL != existingURL {
		t.Fatalf("downloadURL = %q, want %q", downloadURL, existingURL)
	}
	if expiresAt != nil {
		t.Fatalf("expiresAt = %v, want nil", expiresAt)
	}
}

func TestResolveLegacyReferenceDownloadBuildsAuthenticatedProxyFallback(t *testing.T) {
	svc := &taskAssetCenterService{}

	downloadURL, _ := svc.resolveLegacyReferenceDownload("", "tasks/reference.zip", "参考资料.zip", nil)

	if !strings.HasPrefix(downloadURL, "/v1/assets/files/tasks/reference.zip?") {
		t.Fatalf("downloadURL = %q, want proxy fallback", downloadURL)
	}
	if !strings.Contains(downloadURL, "download_filename=") {
		t.Fatalf("downloadURL = %q, want download filename query", downloadURL)
	}
}
