package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

func TestValidateMeAvatarFile(t *testing.T) {
	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	tooLarge := make([]byte, int(maxMeAvatarBytes)+1)
	copy(tooLarge, pngBytes)

	tests := []struct {
		name     string
		filename string
		content  []byte
		wantCode string
	}{
		{name: "valid png", filename: "avatar.png", content: pngBytes},
		{name: "unsupported extension", filename: "avatar.gif", content: pngBytes, wantCode: domain.ErrCodeInvalidRequest},
		{name: "disguised text", filename: "avatar.png", content: []byte("not an image"), wantCode: domain.ErrCodeInvalidRequest},
		{name: "too large", filename: "avatar.png", content: tooLarge, wantCode: domain.ErrCodeInvalidRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := newTestAvatarFileHeader(t, tt.filename, tt.content)
			err := validateMeAvatarFile(file)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("validateMeAvatarFile() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateMeAvatarFile() error = nil, want code %s", tt.wantCode)
			}
			if err.Code != tt.wantCode {
				t.Fatalf("validateMeAvatarFile() code = %s, want %s", err.Code, tt.wantCode)
			}
		})
	}
}

func TestManagedMeAvatarFilenameValidation(t *testing.T) {
	filename, err := newMeAvatarFilename(".PNG")
	if err != nil {
		t.Fatalf("newMeAvatarFilename() error = %v", err)
	}
	if !strings.HasPrefix(filename, "avatar-") || !strings.HasSuffix(filename, ".png") {
		t.Fatalf("newMeAvatarFilename() = %q, want opaque avatar-*.png without user id", filename)
	}
	if !isManagedMeAvatarFilename(filename) {
		t.Fatalf("isManagedMeAvatarFilename(%q) = false, want true", filename)
	}

	invalidValues := []string{
		"u123-456-deadbeef.png",
		"../avatar-" + strings.Repeat("a", 32) + ".png",
		"avatar-" + strings.Repeat("z", 32) + ".png",
		"avatar-" + strings.Repeat("b", 32) + ".gif",
		"avatar-" + strings.Repeat("c", 31) + ".png",
	}
	for _, value := range invalidValues {
		t.Run(value, func(t *testing.T) {
			if isManagedMeAvatarFilename(value) {
				t.Fatalf("isManagedMeAvatarFilename(%q) = true, want false", value)
			}
		})
	}
}

func TestServeMyAvatarPublicOpaqueFilesOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	t.Setenv("USER_AVATAR_DIR", dir)

	filename := "avatar-" + strings.Repeat("a", 32) + ".png"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte("png"), 0o644); err != nil {
		t.Fatalf("write avatar file: %v", err)
	}

	router := gin.New()
	router.GET("/v1/me/avatar-files/:filename", NewAuthHandler(nil).ServeMyAvatar)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/me/avatar-files/"+filename, nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET avatar status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Fatalf("Cache-Control = %q, want public max-age", got)
	}

	for _, path := range []string{
		"/v1/me/avatar-files/u1-123-deadbeef.png",
		"/v1/me/avatar-files/avatar-" + strings.Repeat("b", 32) + ".gif",
		"/v1/me/avatar-files/avatar-" + strings.Repeat("c", 32) + ".png",
	} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("GET %s status = %d body=%s, want 404", path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRemoveStoredMeAvatarOnlyDeletesImageBasename(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USER_AVATAR_DIR", dir)

	imageName := "u1-legacy-avatar.png"
	imagePath := filepath.Join(dir, imageName)
	if err := os.WriteFile(imagePath, []byte("old avatar"), 0o644); err != nil {
		t.Fatalf("write legacy avatar: %v", err)
	}
	otherPath := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(otherPath, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write keep file: %v", err)
	}

	removeStoredMeAvatar("/v1/me/avatar-files/" + imageName)
	if _, err := os.Stat(imagePath); !os.IsNotExist(err) {
		t.Fatalf("legacy image stat err = %v, want removed", err)
	}

	removeStoredMeAvatar("/v1/me/avatar-files/keep.txt")
	if _, err := os.Stat(otherPath); err != nil {
		t.Fatalf("keep file stat err = %v, want preserved", err)
	}
}

func newTestAvatarFileHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	reader := multipart.NewReader(&body, writer.Boundary())
	form, err := reader.ReadForm(maxMeAvatarBytes + 1024)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })
	files := form.File["file"]
	if len(files) != 1 {
		t.Fatalf("multipart files = %d, want 1", len(files))
	}
	return files[0]
}
