package domain

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"time"
)

const (
	ReferenceFileRefSourceTaskReferenceUpload   = "task_reference_upload"
	ReferenceFileRefSourceTaskCreateAssetCenter = "task_create_asset_center"
	ReferenceFileRefStatusUploaded              = "uploaded"
)

// ReferenceFileRef is the formal task-create reference object contract.
// New writes persist object arrays, while legacy string-array payloads remain readable.
type ReferenceFileRef struct {
	AssetID              string     `json:"asset_id"`
	RefID                string     `json:"ref_id,omitempty"`
	UploadRequestID      string     `json:"upload_request_id,omitempty"`
	Filename             string     `json:"filename,omitempty"`
	MimeType             string     `json:"mime_type,omitempty"`
	FileSize             *int64     `json:"file_size,omitempty"`
	URL                  *string    `json:"url,omitempty"`
	DownloadURL          *string    `json:"download_url,omitempty"`
	Source               string     `json:"source,omitempty"`
	Status               string     `json:"status,omitempty"`
	StorageKey           string     `json:"storage_key,omitempty"`
	DownloadURLExpiresAt *time.Time `json:"download_url_expires_at,omitempty"`
}

func (r *ReferenceFileRef) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*r = ReferenceFileRef{}
		return nil
	}

	var refID string
	if err := json.Unmarshal(trimmed, &refID); err == nil {
		*r = ReferenceFileRef{AssetID: strings.TrimSpace(refID)}
		r.Normalize()
		return nil
	}

	type alias ReferenceFileRef
	var decoded alias
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return err
	}
	*r = ReferenceFileRef(decoded)

	var raw map[string]interface{}
	if err := json.Unmarshal(trimmed, &raw); err == nil {
		if strings.TrimSpace(r.AssetID) == "" {
			r.AssetID = firstStringValue(raw, "asset_id", "ref_id", "reference_file_ref", "id")
		}
		if strings.TrimSpace(r.RefID) == "" {
			r.RefID = firstStringValue(raw, "ref_id", "reference_file_ref", "asset_id", "id")
		}
		if r.DownloadURL == nil {
			if url := firstStringValue(raw, "download_url", "url"); url != "" {
				r.DownloadURL = &url
			}
		}
		if strings.TrimSpace(r.StorageKey) == "" {
			r.StorageKey = firstStringValue(raw, "storage_key")
		}
		if r.DownloadURLExpiresAt == nil {
			if expiresAt := firstStringValue(raw, "download_url_expires_at"); expiresAt != "" {
				if parsed, err := time.Parse(time.RFC3339, expiresAt); err == nil {
					r.DownloadURLExpiresAt = &parsed
				}
			}
		}
	}
	r.Normalize()
	return nil
}

func (r ReferenceFileRef) CanonicalID() string {
	return firstNonEmptyReferenceValue(
		strings.TrimSpace(r.AssetID),
		strings.TrimSpace(r.RefID),
	)
}

func (r *ReferenceFileRef) Normalize() bool {
	if r == nil {
		return false
	}
	r.AssetID = strings.TrimSpace(r.AssetID)
	r.RefID = strings.TrimSpace(r.RefID)
	r.UploadRequestID = strings.TrimSpace(r.UploadRequestID)
	r.Filename = strings.TrimSpace(r.Filename)
	r.MimeType = strings.TrimSpace(r.MimeType)
	r.Source = strings.TrimSpace(r.Source)
	r.Status = strings.TrimSpace(r.Status)
	r.StorageKey = strings.TrimSpace(r.StorageKey)
	if id := r.CanonicalID(); id != "" {
		r.AssetID = id
		if r.RefID == "" {
			r.RefID = id
		}
	}
	if r.DownloadURL == nil && r.URL != nil {
		url := strings.TrimSpace(*r.URL)
		if url != "" {
			r.DownloadURL = &url
		}
	}
	if r.URL == nil && r.DownloadURL != nil {
		url := strings.TrimSpace(*r.DownloadURL)
		if url != "" {
			r.URL = &url
		}
	}
	if r.StorageKey == "" {
		if r.DownloadURL != nil {
			r.StorageKey = extractReferenceStorageKeyFromLegacyURL(*r.DownloadURL)
		}
		if r.StorageKey == "" && r.URL != nil {
			r.StorageKey = extractReferenceStorageKeyFromLegacyURL(*r.URL)
		}
	}
	return r.AssetID != ""
}

func NormalizeReferenceFileRefs(refs []ReferenceFileRef) []ReferenceFileRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]ReferenceFileRef, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if !ref.Normalize() {
			continue
		}
		if _, ok := seen[ref.AssetID]; ok {
			continue
		}
		seen[ref.AssetID] = struct{}{}
		out = append(out, ref)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func ParseReferenceFileRefsJSON(raw string) []ReferenceFileRef {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var refs []ReferenceFileRef
	if err := json.Unmarshal([]byte(raw), &refs); err != nil {
		return nil
	}
	return NormalizeReferenceFileRefs(refs)
}

func firstStringValue(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		if text, ok := raw.(string); ok {
			text = strings.TrimSpace(text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func firstNonEmptyReferenceValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func extractReferenceStorageKeyFromLegacyURL(raw string) string {
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
