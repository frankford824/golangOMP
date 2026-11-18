package objectstore_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"workflow/internal/oa/objectstore"
	"workflow/service"
)

func TestOSSDirectRoundTrip(t *testing.T) {
	const secret = "fixture-secret-at-least-thirty-two-characters"
	server := httptest.NewServer(objectstore.New(t.TempDir(), "test-id", secret))
	defer server.Close()
	base := server.URL + "/oa-objects"
	oss := service.NewOSSDirectService(service.OSSDirectConfig{Enabled: true, AccessKeyID: "test-id", AccessKeySecret: secret, Bucket: "oa-objects", Endpoint: "fixture", BucketURL: base, PublicBucketURL: base})
	ctx := context.Background()
	key := "documents/测试 + draft.txt"
	plan, err := oss.CreateSingleUploadPlan(key, "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	put := func(target, contentType, data string) string {
		t.Helper()
		req, err := http.NewRequest("PUT", target, bytes.NewBufferString(data))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", contentType)
		reply, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer reply.Body.Close()
		if reply.StatusCode != 200 {
			body, _ := io.ReadAll(reply.Body)
			t.Fatalf("PUT %d: %s", reply.StatusCode, body)
		}
		return reply.Header.Get("ETag")
	}
	put(plan.UploadURL, plan.RequiredContentType, "hello")
	status, ok, err := oss.StatObject(ctx, key)
	if err != nil || !ok || status.ContentLength != 5 {
		t.Fatalf("head: %v %v %v", status, ok, err)
	}
	if err := oss.CopyObject(ctx, key, "files/copy"); err != nil {
		t.Fatal(err)
	}
	download := oss.PresignDownloadURLWithFilename("files/copy", "draft.txt")
	reply, err := http.Get(download.DownloadURL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(reply.Body)
	reply.Body.Close()
	if reply.StatusCode != 200 || string(body) != "hello" || reply.Header.Get("Content-Disposition") == "" {
		t.Fatalf("download: %d %q", reply.StatusCode, body)
	}
	tampered, err := http.Get(download.DownloadURL + "&partNumber=1")
	if err != nil {
		t.Fatal(err)
	}
	tampered.Body.Close()
	if tampered.StatusCode != 403 {
		t.Fatalf("tampered signature: %d", tampered.StatusCode)
	}
	if err := oss.DeleteObject(ctx, key); err != nil {
		t.Fatal(err)
	}
	if ok, err := oss.HeadObject(ctx, key); err != nil || ok {
		t.Fatalf("deleted key: %v %v", ok, err)
	}
	multi, err := oss.CreateMultipartUploadPlan(ctx, "files/multi", 12<<20, "application/octet-stream")
	if err != nil {
		t.Fatal(err)
	}
	if len(multi.Parts) != 2 {
		t.Fatalf("parts: %d", len(multi.Parts))
	}
	parts := []service.OSSCompletePart{}
	for _, part := range multi.Parts {
		parts = append(parts, service.OSSCompletePart{PartNumber: part.PartNumber, ETag: put(part.UploadURL, multi.RequiredContentType, "part")})
	}
	if err := oss.CompleteMultipartUpload(ctx, "files/multi", multi.UploadID, parts); err != nil {
		t.Fatal(err)
	}
	final, err := oss.OpenObject(ctx, "files/multi")
	if err != nil {
		t.Fatal(err)
	}
	merged, _ := io.ReadAll(final)
	final.Close()
	if string(merged) != "partpart" {
		t.Fatalf("merged = %q", merged)
	}
}
