package service

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"

	"workflow/domain"
)

const (
	MaxReferenceImageCount             = 3
	MaxReferenceImageSingleBytes       = 3 * 1024 * 1024
	MaxReferenceImageTotalBytes        = MaxReferenceImageCount * MaxReferenceImageSingleBytes
	maxReferenceImagesJSONStorageBytes = 15 * 1024 * 1024
)

const referenceImagesSuggestion = "use /v1/tasks/reference-upload / reference_file_refs"

func ValidateReferenceImages(refs []string) *domain.AppError {
	if len(refs) == 0 {
		return nil
	}

	if len(refs) > MaxReferenceImageCount {
		return newReferenceImagesLimitError(refs, nil, map[string]interface{}{
			"actual_total_bytes": referenceImagesTotalBytes(refs),
			"max_total_bytes":    MaxReferenceImageTotalBytes,
			"violation":          "too_many_images",
		})
	}

	totalBytes := 0
	oversizedIndexes := make([]int, 0)
	for i, ref := range refs {
		sizeBytes := referenceImageBytes(ref)
		totalBytes += sizeBytes
		if sizeBytes > MaxReferenceImageSingleBytes {
			oversizedIndexes = append(oversizedIndexes, i)
		}
	}

	if len(oversizedIndexes) > 0 {
		return newReferenceImagesLimitError(refs, oversizedIndexes, map[string]interface{}{
			"actual_total_bytes": totalBytes,
			"max_total_bytes":    MaxReferenceImageTotalBytes,
			"violation":          "image_too_large",
		})
	}

	if totalBytes > MaxReferenceImageTotalBytes {
		return newReferenceImagesLimitError(refs, nil, map[string]interface{}{
			"actual_total_bytes": totalBytes,
			"max_total_bytes":    MaxReferenceImageTotalBytes,
			"violation":          "total_size_too_large",
		})
	}

	return nil
}

func MarshalReferenceImagesJSON(refs []string) (string, *domain.AppError) {
	if len(refs) == 0 {
		return "[]", nil
	}
	if appErr := ValidateReferenceImages(refs); appErr != nil {
		return "", appErr
	}

	raw, err := json.Marshal(refs)
	if err != nil {
		return "", domain.NewAppError(domain.ErrCodeInternalError, "failed to encode reference_images", nil)
	}
	if len(raw) > maxReferenceImagesJSONStorageBytes {
		return "", newReferenceImagesLimitError(refs, nil, map[string]interface{}{
			"actual_total_bytes":              referenceImagesTotalBytes(refs),
			"max_total_bytes":                 MaxReferenceImageTotalBytes,
			"reference_images_json_bytes":     len(raw),
			"max_reference_images_json_bytes": maxReferenceImagesJSONStorageBytes,
			"violation":                       "reference_images_json_too_large",
		})
	}
	return string(raw), nil
}

func newReferenceImagesLimitError(refs []string, oversizedIndexes []int, extra map[string]interface{}) *domain.AppError {
	details := map[string]interface{}{
		"actual_count":      len(refs),
		"max_count":         MaxReferenceImageCount,
		"max_single_bytes":  MaxReferenceImageSingleBytes,
		"oversized_indexes": oversizedIndexesOrEmpty(oversizedIndexes),
		"suggestion":        referenceImagesSuggestion,
	}
	for k, v := range extra {
		details[k] = v
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "reference_images exceed upload limit", details)
}

func oversizedIndexesOrEmpty(indexes []int) []int {
	if len(indexes) == 0 {
		return []int{}
	}
	return indexes
}

func referenceImagesTotalBytes(refs []string) int {
	total := 0
	for _, ref := range refs {
		total += referenceImageBytes(ref)
	}
	return total
}

func referenceImageBytes(ref string) int {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return 0
	}
	if !strings.HasPrefix(trimmed, "data:") {
		return len(trimmed)
	}

	commaIdx := strings.Index(trimmed, ",")
	if commaIdx < 0 {
		return len(trimmed)
	}

	meta := trimmed[:commaIdx]
	payload := strings.TrimSpace(trimmed[commaIdx+1:])
	if payload == "" {
		return 0
	}
	if !strings.Contains(strings.ToLower(meta), ";base64") {
		return len(payload)
	}

	if n, ok := decodeBase64Bytes(payload); ok {
		return n
	}
	return len(payload)
}

func decodeBase64Bytes(payload string) (int, bool) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range encodings {
		n, err := io.Copy(io.Discard, base64.NewDecoder(enc, strings.NewReader(payload)))
		if err == nil {
			return int(n), true
		}
	}
	return 0, false
}
