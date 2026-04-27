package service

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func TestReferenceFileRefsEnricherOSSDisabledReturnsInputUnchanged(t *testing.T) {
	input := []domain.ReferenceFileRef{{
		AssetID:     "ref-1",
		StorageKey:  "tasks/ref-1.png",
		DownloadURL: stringPtr("/v1/assets/files/tasks/ref-1.png"),
		URL:         stringPtr("/v1/assets/files/tasks/ref-1.png"),
	}}

	enricher := NewReferenceFileRefsEnricher(NewOSSDirectService(OSSDirectConfig{Enabled: false}), nil)
	got := enricher.EnrichAll(input)
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("EnrichAll() = %+v, want unchanged %+v", got, input)
	}
}

func TestReferenceFileRefsEnricherPresignsWhenStorageKeyPresent(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	oss := newReferenceFileRefsTestOSS(now)
	enricher := NewReferenceFileRefsEnricher(oss, func() time.Time { return now })
	expected := oss.PresignPreviewURL("tasks/refs/ref-2.png")

	input := []domain.ReferenceFileRef{{AssetID: "ref-2", StorageKey: "tasks/refs/ref-2.png"}}
	got := enricher.EnrichAll(input)
	if len(got) != 1 {
		t.Fatalf("len(EnrichAll()) = %d, want 1", len(got))
	}
	if got[0].DownloadURL == nil || *got[0].DownloadURL != expected.DownloadURL {
		t.Fatalf("download_url = %v, want %q", got[0].DownloadURL, expected.DownloadURL)
	}
	if got[0].URL == nil || *got[0].URL != expected.DownloadURL {
		t.Fatalf("url = %v, want %q", got[0].URL, expected.DownloadURL)
	}
	if got[0].DownloadURLExpiresAt == nil || !got[0].DownloadURLExpiresAt.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("download_url_expires_at = %v, want %v", got[0].DownloadURLExpiresAt, now.Add(15*time.Minute))
	}
}

func TestReferenceFileRefsEnricherRecoversStorageKeyFromLegacyDownloadURL(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 5, 0, 0, time.UTC)
	oss := newReferenceFileRefsTestOSS(now)
	enricher := NewReferenceFileRefsEnricher(oss, func() time.Time { return now })
	expected := oss.PresignPreviewURL("tasks/legacy/ref-3.png")

	got := enricher.EnrichAll([]domain.ReferenceFileRef{{
		AssetID:     "ref-3",
		DownloadURL: stringPtr("/v1/assets/files/tasks/legacy/ref-3.png"),
	}})
	if got[0].StorageKey != "tasks/legacy/ref-3.png" {
		t.Fatalf("storage_key = %q", got[0].StorageKey)
	}
	if got[0].DownloadURL == nil || *got[0].DownloadURL != expected.DownloadURL {
		t.Fatalf("download_url = %v, want %q", got[0].DownloadURL, expected.DownloadURL)
	}
}

func TestExtractStorageKeyFromLegacyURLDecodesEachSegment(t *testing.T) {
	raw := "https://api.test/v1/assets/files/tasks/%E4%B8%AD%E6%96%87/%E5%9B%BE%E7%89%87%20final.png"
	got := extractStorageKeyFromLegacyURL(raw)
	if got != "tasks/中文/图片 final.png" {
		t.Fatalf("extractStorageKeyFromLegacyURL() = %q", got)
	}
}

func TestReferenceFileRefsEnricherLeavesRefsWithoutUsableKeyUnchanged(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 10, 0, 0, time.UTC)
	oss := newReferenceFileRefsTestOSS(now)
	enricher := NewReferenceFileRefsEnricher(oss, func() time.Time { return now })

	input := []domain.ReferenceFileRef{{
		AssetID:     "ref-5",
		DownloadURL: stringPtr("https://example.com/not-legacy/path.png"),
	}}
	got := enricher.EnrichAll(input)
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("EnrichAll() = %+v, want unchanged %+v", got, input)
	}
}

func TestReferenceFileRefsEnricherDoesNotMutateCallerSlice(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 15, 0, 0, time.UTC)
	oss := newReferenceFileRefsTestOSS(now)
	enricher := NewReferenceFileRefsEnricher(oss, func() time.Time { return now })

	legacyURL := "/v1/assets/files/tasks/legacy/ref-6.png"
	input := []domain.ReferenceFileRef{{
		AssetID:     "ref-6",
		DownloadURL: &legacyURL,
		URL:         &legacyURL,
	}}
	got := enricher.EnrichAll(input)

	if input[0].DownloadURL == nil || *input[0].DownloadURL != legacyURL {
		t.Fatalf("input mutated: %+v", input)
	}
	if got[0].DownloadURL == nil || strings.HasPrefix(*got[0].DownloadURL, "/v1/assets/files/") {
		t.Fatalf("output download_url = %v, want presigned URL", got[0].DownloadURL)
	}
}

func TestReferenceFileRefsEnricherNilSafety(t *testing.T) {
	input := []domain.ReferenceFileRef{{AssetID: "ref-7"}}

	var nilReceiver *ReferenceFileRefsEnricher
	if got := nilReceiver.EnrichAll(input); !reflect.DeepEqual(got, input) {
		t.Fatalf("nil receiver EnrichAll() = %+v, want %+v", got, input)
	}

	enricher := NewReferenceFileRefsEnricher(nil, nil)
	if got := enricher.EnrichAll(input); !reflect.DeepEqual(got, input) {
		t.Fatalf("nil oss EnrichAll() = %+v, want %+v", got, input)
	}
}

func newReferenceFileRefsTestOSS(now time.Time) *OSSDirectService {
	svc := NewOSSDirectService(OSSDirectConfig{
		Enabled:         true,
		Endpoint:        "oss-cn-hangzhou.aliyuncs.com",
		Bucket:          "yongbooss",
		AccessKeyID:     "ak-test",
		AccessKeySecret: "secret-test",
		PresignExpiry:   15 * time.Minute,
		PublicEndpoint:  "oss-cn-hangzhou.aliyuncs.com",
	})
	svc.nowFn = func() time.Time { return now }
	return svc
}
