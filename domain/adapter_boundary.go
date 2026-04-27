package domain

import (
	"strings"
	"time"
)

type BoundaryAdapterMode string

const (
	BoundaryAdapterModeDispatchThenAttempt         BoundaryAdapterMode = "dispatch_then_attempt"
	BoundaryAdapterModeCallLogThenExecution        BoundaryAdapterMode = "call_log_then_execution"
	BoundaryAdapterModeUploadRequestThenStorageRef BoundaryAdapterMode = "upload_request_then_storage_ref"
)

type BoundaryDispatchMode string

const (
	BoundaryDispatchModeDispatchRecord       BoundaryDispatchMode = "dispatch_record"
	BoundaryDispatchModeExecutionProgress    BoundaryDispatchMode = "execution_progress"
	BoundaryDispatchModeUploadRequestBinding BoundaryDispatchMode = "upload_request_binding"
)

type BoundaryStorageMode string

const (
	BoundaryStorageModeLifecycleManagedResultRef BoundaryStorageMode = "lifecycle_managed_result_ref"
	BoundaryStorageModeAssetStorageRef           BoundaryStorageMode = "asset_storage_ref"
)

type AdapterRefSummary struct {
	RefType       string `json:"ref_type"`
	RefKey        string `json:"ref_key"`
	IsPlaceholder bool   `json:"is_placeholder"`
	Note          string `json:"note,omitempty"`
}

type ResourceRefSummary struct {
	RefType       string     `json:"ref_type"`
	RefKey        string     `json:"ref_key"`
	FileName      string     `json:"file_name,omitempty"`
	MimeType      string     `json:"mime_type,omitempty"`
	FileSize      *int64     `json:"file_size,omitempty"`
	ChecksumHint  string     `json:"checksum_hint,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	IsPlaceholder bool       `json:"is_placeholder"`
	Note          string     `json:"note,omitempty"`
}

type HandoffRefSummary struct {
	RefType       string     `json:"ref_type"`
	RefKey        string     `json:"ref_key"`
	Status        string     `json:"status,omitempty"`
	RequestedAt   *time.Time `json:"requested_at,omitempty"`
	ReceivedAt    *time.Time `json:"received_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	IsPlaceholder bool       `json:"is_placeholder"`
	Note          string     `json:"note,omitempty"`
}

func BuildAdapterRefSummary(refType string, refKey string, isPlaceholder bool, note string) *AdapterRefSummary {
	refType = strings.TrimSpace(refType)
	refKey = strings.TrimSpace(refKey)
	if refType == "" || refKey == "" {
		return nil
	}
	return &AdapterRefSummary{
		RefType:       refType,
		RefKey:        refKey,
		IsPlaceholder: isPlaceholder,
		Note:          strings.TrimSpace(note),
	}
}

func BuildResourceRefSummary(refType string, refKey string, fileName string, mimeType string, fileSize *int64, checksumHint string, expiresAt *time.Time, isPlaceholder bool, note string) *ResourceRefSummary {
	refType = strings.TrimSpace(refType)
	refKey = strings.TrimSpace(refKey)
	if refType == "" || refKey == "" {
		return nil
	}
	return &ResourceRefSummary{
		RefType:       refType,
		RefKey:        refKey,
		FileName:      strings.TrimSpace(fileName),
		MimeType:      strings.TrimSpace(mimeType),
		FileSize:      cloneInt64Ptr(fileSize),
		ChecksumHint:  strings.TrimSpace(checksumHint),
		ExpiresAt:     cloneTimePtr(expiresAt),
		IsPlaceholder: isPlaceholder,
		Note:          strings.TrimSpace(note),
	}
}

func BuildHandoffRefSummary(refType string, refKey string, status string, requestedAt *time.Time, receivedAt *time.Time, finishedAt *time.Time, expiresAt *time.Time, isPlaceholder bool, note string) *HandoffRefSummary {
	refType = strings.TrimSpace(refType)
	refKey = strings.TrimSpace(refKey)
	if refType == "" || refKey == "" {
		return nil
	}
	return &HandoffRefSummary{
		RefType:       refType,
		RefKey:        refKey,
		Status:        strings.TrimSpace(status),
		RequestedAt:   cloneTimePtr(requestedAt),
		ReceivedAt:    cloneTimePtr(receivedAt),
		FinishedAt:    cloneTimePtr(finishedAt),
		ExpiresAt:     cloneTimePtr(expiresAt),
		IsPlaceholder: isPlaceholder,
		Note:          strings.TrimSpace(note),
	}
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := value.UTC()
	return &copyValue
}
