package asset_center

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"workflow/repo"
	baseservice "workflow/service"
)

func TestWriteProductionPackageZIPCreatesOrderAddressCopiesAndFailureManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "image-bytes")
	}))
	defer server.Close()
	manifest := &ExcelPackageManifest{
		Items: []ExcelPackageItem{{OrderNo: "ORDER-1", SKUCode: "SKU-1", SKUName: "商品",
			Quantity: 2, Filename: "SKU-1.jpg", PackageFolder: "SKU-1 商品", Address: "张三 138****0000", DownloadURL: server.URL}},
		Failures: []ExcelPackageFailure{{OrderNo: "ORDER-2", SKUCode: "SKU-2", Reason: "asset_not_found", Message: "未找到"}},
	}
	var output bytes.Buffer
	if err := writeProductionPackageZIP(context.Background(), &output, manifest); err != nil {
		t.Fatalf("writeProductionPackageZIP() error = %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(reader.File))
	contents := map[string]string{}
	for _, file := range reader.File {
		names = append(names, file.Name)
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(rc)
		_ = rc.Close()
		contents[file.Name] = string(raw)
	}
	sort.Strings(names)
	want := []string{
		"ORDER-1（隐私号）/SKU-1 商品/SKU-1-1.jpg",
		"ORDER-1（隐私号）/SKU-1 商品/SKU-1-2.jpg",
		"ORDER-1（隐私号）/地址.txt",
		"打包失败清单.txt",
	}
	if strings.Join(names, "|") != strings.Join(want, "|") {
		t.Fatalf("zip entries = %#v, want %#v", names, want)
	}
	if !strings.Contains(contents["ORDER-1（隐私号）/地址.txt"], "138****0000") {
		t.Fatalf("address entry = %q", contents["ORDER-1（隐私号）/地址.txt"])
	}
	if !strings.Contains(contents["打包失败清单.txt"], "SKU-2") {
		t.Fatalf("failure manifest = %q", contents["打包失败清单.txt"])
	}
}

func TestFinalizedPackageFilenameContainsSKUBoundaries(t *testing.T) {
	item := productionPackageAssetForTest("成品-HSC34548-正面.tif")
	if !finalizedPackageFilenameContainsSKU(item, "HSC34548") {
		t.Fatal("expected exact filename SKU match")
	}
	if finalizedPackageFilenameContainsSKU(item, "SC3454") {
		t.Fatal("substring must not match another SKU")
	}
}

func TestProductionPackageNoFilesMessageExplainsRowFailures(t *testing.T) {
	message := productionPackageNoFilesMessage(&ExcelPackageManifest{FailureCount: 3})
	if message != "未找到可打包的最终成品：3 行均未匹配所选格式，请查看逐行异常明细。" {
		t.Fatalf("message = %q", message)
	}
}

func TestUploadProductionPackageFileUsesMultipartAndCompletesETags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 || r.ContentLength != int64(len(body)) {
			t.Errorf("part upload body/content-length = %d/%d", len(body), r.ContentLength)
		}
		w.Header().Set("ETag", `"etag-`+strings.TrimPrefix(r.URL.Path, "/")+`"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	store := &productionPackageStoreStub{plan: &baseservice.OSSDirectUploadPlan{
		Mode: "multipart", UploadID: "upload-1", PartSize: 4, RequiredContentType: "application/zip",
		Parts: []baseservice.OSSPresignedPart{{PartNumber: 1, UploadURL: server.URL + "/1"}, {PartNumber: 2, UploadURL: server.URL + "/2"}},
	}}
	file, err := os.CreateTemp("", "production-package-upload-test-*")
	if err != nil {
		t.Fatal(err)
	}
	filePath := file.Name()
	defer os.Remove(filePath)
	_, _ = file.Write([]byte("12345678"))
	_ = file.Close()
	if err := uploadProductionPackageFile(context.Background(), store, "packages/test.zip", filePath); err != nil {
		t.Fatalf("uploadProductionPackageFile() error = %v", err)
	}
	if store.completedObjectKey != "packages/test.zip" || store.completedUploadID != "upload-1" || len(store.completedParts) != 2 {
		t.Fatalf("complete call = key:%q upload:%q parts:%+v", store.completedObjectKey, store.completedUploadID, store.completedParts)
	}
	if store.completedParts[0].ETag == "" || store.completedParts[1].ETag == "" {
		t.Fatalf("completed ETags = %+v", store.completedParts)
	}
}

type productionPackageStoreStub struct {
	plan               *baseservice.OSSDirectUploadPlan
	completedObjectKey string
	completedUploadID  string
	completedParts     []baseservice.OSSCompletePart
}

func (s *productionPackageStoreStub) Enabled() bool { return true }
func (s *productionPackageStoreStub) CreateUploadPlan(context.Context, string, int64, string) (*baseservice.OSSDirectUploadPlan, error) {
	return s.plan, nil
}
func (s *productionPackageStoreStub) CompleteMultipartUpload(_ context.Context, key, uploadID string, parts []baseservice.OSSCompletePart) error {
	s.completedObjectKey, s.completedUploadID = key, uploadID
	s.completedParts = append([]baseservice.OSSCompletePart{}, parts...)
	return nil
}
func (s *productionPackageStoreStub) AbortMultipartUpload(context.Context, string, string) error {
	return nil
}
func (s *productionPackageStoreStub) PresignDownloadURLWithFilename(string, string) *baseservice.OSSDirectDownloadInfo {
	return &baseservice.OSSDirectDownloadInfo{DownloadURL: "https://example.test/package.zip", ExpiresAt: time.Now().Add(time.Hour)}
}

func productionPackageAssetForTest(filename string) (item repo.ProductionPackageAsset) {
	item.FileName = filename
	return item
}
