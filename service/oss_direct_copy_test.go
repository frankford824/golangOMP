package service

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCopyObject_Success(t *testing.T) {
	ossServer := newFakeOSSDirectServer(t)
	defer ossServer.Close()

	svc := NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        ossServer.EndpointHost(),
		PublicEndpoint:  ossServer.EndpointHost(),
		Bucket:          "test-bucket",
		AccessKeyID:     "test-key",
		AccessKeySecret: "test-secret",
		PresignExpiry:   15 * time.Minute,
		PartSize:        10 * 1024 * 1024,
	})
	svc.httpClient = ossServer.Client()

	srcKey := "tasks/T1/assets/A1/v1/source/【4条装】毕业手持横幅组合A(1).psd"
	dstKey := "tasks/T1/assets/A1/v1/source/1700000000000000000_ab12cd34.psd"
	if err := svc.CopyObject(context.Background(), srcKey, dstKey); err != nil {
		t.Fatalf("CopyObject() error = %v", err)
	}

	ossServer.mu.Lock()
	defer ossServer.mu.Unlock()
	if ossServer.copyCalls != 1 {
		t.Fatalf("copyCalls = %d, want 1", ossServer.copyCalls)
	}
	if !strings.HasPrefix(ossServer.lastCopySrc, "/test-bucket/tasks/T1/assets/A1/v1/source/") {
		t.Fatalf("copy source = %q", ossServer.lastCopySrc)
	}
	if strings.Contains(ossServer.lastCopySrc, "+") || strings.Contains(ossServer.lastCopySrc, " ") {
		t.Fatalf("copy source should be URL-escaped, got %q", ossServer.lastCopySrc)
	}
}

func TestCopyObject_SignatureMatchesSpec(t *testing.T) {
	ossServer := newFakeOSSDirectServer(t)
	defer ossServer.Close()

	svc := NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        ossServer.EndpointHost(),
		PublicEndpoint:  ossServer.EndpointHost(),
		Bucket:          "test-bucket",
		AccessKeyID:     "test-key",
		AccessKeySecret: "test-secret",
		PresignExpiry:   15 * time.Minute,
		PartSize:        10 * 1024 * 1024,
	})
	svc.httpClient = ossServer.Client()

	srcKey := "tasks/T1/assets/A1/v1/source/a+b.psd"
	dstKey := "tasks/T1/assets/A1/v1/source/1700000000000000000_abcdef12.psd"
	if err := svc.CopyObject(context.Background(), srcKey, dstKey); err != nil {
		t.Fatalf("CopyObject() error = %v", err)
	}

	ossServer.mu.Lock()
	date := ossServer.lastCopyDate
	auth := ossServer.lastCopyAuth
	copySource := ossServer.lastCopySrc
	ossServer.mu.Unlock()

	if copySource != "/test-bucket/tasks/T1/assets/A1/v1/source/a%2Bb.psd" {
		t.Fatalf("copy source = %q", copySource)
	}
	canonHeaders := canonicalOSSHeaders(map[string]string{"x-oss-copy-source": copySource})
	if canonHeaders != "x-oss-copy-source:"+copySource+"\n" {
		t.Fatalf("canonical headers = %q", canonHeaders)
	}
	expected := "OSS test-key:" + svc.signV1(
		"PUT",
		"",
		"",
		date,
		canonHeaders,
		"/test-bucket/"+dstKey,
	)
	if auth != expected {
		t.Fatalf("Authorization = %q, want %q", auth, expected)
	}
}
