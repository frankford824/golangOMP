package service

import (
	"net/url"
	"strings"
	"time"

	"workflow/domain"
)

type ReferenceFileRefsEnricher struct {
	ossDirect *OSSDirectService
	nowFn     func() time.Time
}

func NewReferenceFileRefsEnricher(oss *OSSDirectService, nowFn func() time.Time) *ReferenceFileRefsEnricher {
	if nowFn == nil {
		nowFn = time.Now
	}
	return &ReferenceFileRefsEnricher{
		ossDirect: oss,
		nowFn:     nowFn,
	}
}

func (e *ReferenceFileRefsEnricher) EnrichAll(refs []domain.ReferenceFileRef) []domain.ReferenceFileRef {
	if e == nil || e.ossDirect == nil || !e.ossDirect.Enabled() {
		return refs
	}

	out := make([]domain.ReferenceFileRef, len(refs))
	copy(out, refs)

	for i := range out {
		ref := out[i]
		storageKey := strings.TrimSpace(ref.StorageKey)
		if storageKey == "" && ref.DownloadURL != nil {
			storageKey = extractStorageKeyFromLegacyURL(*ref.DownloadURL)
		}
		if storageKey == "" && ref.URL != nil {
			storageKey = extractStorageKeyFromLegacyURL(*ref.URL)
		}
		if storageKey == "" {
			continue
		}

		presigned := e.ossDirect.PresignPreviewURL(storageKey)
		if presigned == nil {
			continue
		}

		expiresAt := presigned.ExpiresAt
		if expiresAt.IsZero() {
			expiresAt = e.now().Add(e.ossDirect.Config().PresignExpiry)
		}
		ref.StorageKey = storageKey
		ref.DownloadURL = stringPtr(presigned.DownloadURL)
		ref.URL = stringPtr(presigned.DownloadURL)
		ref.DownloadURLExpiresAt = &expiresAt
		out[i] = ref
	}

	return out
}

func extractStorageKeyFromLegacyURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	path := raw
	if parsed, err := url.Parse(raw); err == nil && parsed != nil && parsed.Path != "" {
		path = parsed.Path
	}

	const prefix = "/v1/assets/files/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	trimmed := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if trimmed == "" {
		return ""
	}

	segments := strings.Split(trimmed, "/")
	for i, segment := range segments {
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			return ""
		}
		segments[i] = decoded
	}

	return strings.TrimSpace(strings.Join(segments, "/"))
}

func (e *ReferenceFileRefsEnricher) now() time.Time {
	if e == nil || e.nowFn == nil {
		return time.Now()
	}
	return e.nowFn()
}

func stringPtr(value string) *string {
	return &value
}
