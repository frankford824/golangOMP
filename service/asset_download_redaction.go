package service

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"workflow/domain"
)

// TaskAssetDownloadAllowed reports whether the request actor may download
// files for the exact task scope. Read-model routes can require task.view or
// asset.view without granting asset.download, so callers must not infer
// download access from successful read authorization.
func TaskAssetDownloadAllowed(ctx context.Context, task *domain.Task) bool {
	if task == nil {
		return false
	}
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok {
		// Services are also used by trusted internal projections that have no
		// request actor. Public HTTP routes always inject an actor.
		return true
	}
	if actor.ID <= 0 {
		return false
	}
	return domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetDownload, task.AccessSubject())
}

// RedactTaskReadModelDownloads removes controlled object locators and download
// URLs from a task response while preserving the remaining read model.
func RedactTaskReadModelDownloads(readModel *domain.TaskReadModel) {
	if readModel == nil {
		return
	}
	readModel.ReferenceFileRefs = RedactReferenceFileRefDownloads(readModel.ReferenceFileRefs)
	readModel.SKUItems = redactTaskSKUItemDownloads(readModel.SKUItems)
	readModel.RetouchRequirements = redactRetouchRequirementDownloads(readModel.RetouchRequirements)
}

// RedactReferenceFileRefDownloads returns a copy so callers never mutate
// repository-backed or cached reference values in place.
func RedactReferenceFileRefDownloads(refs []domain.ReferenceFileRef) []domain.ReferenceFileRef {
	if refs == nil {
		return nil
	}
	out := make([]domain.ReferenceFileRef, len(refs))
	copy(out, refs)
	for index := range out {
		out[index].URL = nil
		out[index].DownloadURL = nil
		out[index].StorageKey = ""
		out[index].DownloadURLExpiresAt = nil
	}
	return out
}

// RedactDesignAssetDownloads returns copies of the asset and version records so
// response shaping cannot corrupt a repository object that another caller may
// reuse.
func RedactDesignAssetDownloads(assets []*domain.DesignAsset) []*domain.DesignAsset {
	if assets == nil {
		return nil
	}
	out := make([]*domain.DesignAsset, 0, len(assets))
	for _, asset := range assets {
		out = append(out, RedactDesignAssetDownload(asset))
	}
	return out
}

func RedactDesignAssetDownload(asset *domain.DesignAsset) *domain.DesignAsset {
	if asset == nil {
		return nil
	}
	cloned := *asset
	cloned.CurrentVersion = RedactDesignAssetVersionDownload(asset.CurrentVersion)
	cloned.ApprovedVersion = RedactDesignAssetVersionDownload(asset.ApprovedVersion)
	return &cloned
}

func RedactDesignAssetVersionDownloads(versions []*domain.DesignAssetVersion) []*domain.DesignAssetVersion {
	if versions == nil {
		return nil
	}
	out := make([]*domain.DesignAssetVersion, 0, len(versions))
	for _, version := range versions {
		out = append(out, RedactDesignAssetVersionDownload(version))
	}
	return out
}

func RedactDesignAssetVersionDownload(version *domain.DesignAssetVersion) *domain.DesignAssetVersion {
	if version == nil {
		return nil
	}
	cloned := *version
	cloned.StorageKey = ""
	cloned.DownloadURL = nil
	cloned.PublicDownloadAllowed = false
	cloned.AccessHint = ""
	return &cloned
}

// RedactTaskEventDownloads copies event rows and rewrites only their response
// payload. Stored task_event_logs bytes remain unchanged.
func RedactTaskEventDownloads(events []*domain.TaskEvent) []*domain.TaskEvent {
	out := CloneTaskEvents(events)
	for _, event := range out {
		if event == nil {
			continue
		}
		event.Payload = RedactAssetDownloadJSON(event.Payload)
	}
	return out
}

func CloneTaskEvents(events []*domain.TaskEvent) []*domain.TaskEvent {
	if events == nil {
		return nil
	}
	out := make([]*domain.TaskEvent, 0, len(events))
	for _, event := range events {
		if event == nil {
			out = append(out, nil)
			continue
		}
		cloned := *event
		cloned.Payload = cloneRawMessage(event.Payload)
		out = append(out, &cloned)
	}
	return out
}

// RedactAssetDownloadJSON removes download-bearing fields from JSON response
// snapshots. Invalid historical JSON becomes an empty object so corruption
// cannot bypass response redaction; the database value is never mutated.
func RedactAssetDownloadJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return json.RawMessage(`{}`)
	}
	redactAssetDownloadJSONValue(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return encoded
}

// RedactLegacyReferenceImagesJSON removes inline image content and object
// locators from the retired reference_images_json []string shape. Unlike
// ReferenceFileRef objects, these values have no typed field that can be
// selectively cleared, so ambiguous non-string history fails closed.
func RedactLegacyReferenceImagesJSON(raw string) string {
	var values []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &values); err != nil {
		return "[]"
	}
	safe := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || legacyReferenceImageStringIsLocator(value) {
			continue
		}
		safe = append(safe, value)
	}
	encoded, err := json.Marshal(safe)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func legacyReferenceImageStringIsLocator(value string) bool {
	normalized := strings.TrimSpace(value)
	lower := strings.ToLower(normalized)
	for _, prefix := range []string{
		"http:", "https:", "ftp:", "file:", "data:", "blob:",
		"s3:", "oss:", "gs:", "//",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	if strings.ContainsAny(normalized, `/\?`) {
		return true
	}
	decoded, err := url.PathUnescape(normalized)
	if err != nil {
		return true
	}
	return decoded != normalized && strings.ContainsAny(decoded, `/\?`)
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func redactTaskSKUItemDownloads(items []*domain.TaskSKUItem) []*domain.TaskSKUItem {
	if items == nil {
		return nil
	}
	out := make([]*domain.TaskSKUItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			out = append(out, nil)
			continue
		}
		cloned := *item
		cloned.ReferenceFileRefs = RedactReferenceFileRefDownloads(item.ReferenceFileRefs)
		out = append(out, &cloned)
	}
	return out
}

func redactRetouchRequirementDownloads(items []domain.TaskRetouchRequirement) []domain.TaskRetouchRequirement {
	if items == nil {
		return nil
	}
	out := make([]domain.TaskRetouchRequirement, len(items))
	copy(out, items)
	for index := range out {
		out[index].ReferenceFileRefs = RedactReferenceFileRefDownloads(items[index].ReferenceFileRefs)
		out[index].SourceAssets = RedactDesignAssetDownloads(items[index].SourceAssets)
	}
	return out
}

func redactAssetDownloadJSONValue(value interface{}) {
	switch typed := value.(type) {
	case []interface{}:
		for _, item := range typed {
			redactAssetDownloadJSONValue(item)
		}
	case map[string]interface{}:
		_, hasDownloadURL := typed["download_url"]
		_, hasStorageKey := typed["storage_key"]
		_, hasFilePath := typed["file_path"]
		_, hasObjectKey := typed["object_key"]
		_, hasSignedURL := typed["signed_url"]
		_, hasPresignedURL := typed["presigned_url"]
		_, hasAssetID := typed["asset_id"]
		_, hasRefID := typed["ref_id"]
		isAssetObject := hasDownloadURL ||
			hasStorageKey ||
			hasFilePath ||
			hasObjectKey ||
			hasSignedURL ||
			hasPresignedURL ||
			hasAssetID ||
			hasRefID
		if isAssetObject {
			delete(typed, "download_url")
			delete(typed, "download_url_expires_at")
			delete(typed, "storage_key")
			delete(typed, "file_path")
			delete(typed, "object_key")
			delete(typed, "signed_url")
			delete(typed, "presigned_url")
			if _, ok := typed["url"]; ok {
				delete(typed, "url")
			}
			if _, ok := typed["public_download_allowed"]; ok {
				typed["public_download_allowed"] = false
			}
			if _, ok := typed["access_hint"]; ok {
				typed["access_hint"] = ""
			}
		}
		for _, item := range typed {
			redactAssetDownloadJSONValue(item)
		}
	}
}
