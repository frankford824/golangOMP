package domain

import (
	"strings"
	"time"
)

type DesignAssetUploadMode string

const (
	DesignAssetUploadModeSmall     DesignAssetUploadMode = "small"
	DesignAssetUploadModeMultipart DesignAssetUploadMode = "multipart"
)

func (m DesignAssetUploadMode) Valid() bool {
	switch m {
	case DesignAssetUploadModeSmall, DesignAssetUploadModeMultipart:
		return true
	default:
		return false
	}
}

type DesignAssetUploadStatus string

const (
	DesignAssetUploadStatusPending   DesignAssetUploadStatus = "pending"
	DesignAssetUploadStatusUploaded  DesignAssetUploadStatus = "uploaded"
	DesignAssetUploadStatusCancelled DesignAssetUploadStatus = "cancelled"
)

func (s DesignAssetUploadStatus) Valid() bool {
	switch s {
	case DesignAssetUploadStatusPending, DesignAssetUploadStatusUploaded, DesignAssetUploadStatusCancelled:
		return true
	default:
		return false
	}
}

type DesignAssetPreviewStatus string

const (
	DesignAssetPreviewStatusPending       DesignAssetPreviewStatus = "pending"
	DesignAssetPreviewStatusNotApplicable DesignAssetPreviewStatus = "not_applicable"
)

func (s DesignAssetPreviewStatus) Valid() bool {
	switch s {
	case DesignAssetPreviewStatusPending, DesignAssetPreviewStatusNotApplicable:
		return true
	default:
		return false
	}
}

type DesignAssetSessionStatus string

const (
	DesignAssetSessionStatusCreated   DesignAssetSessionStatus = "created"
	DesignAssetSessionStatusCompleted DesignAssetSessionStatus = "completed"
	DesignAssetSessionStatusCancelled DesignAssetSessionStatus = "cancelled"
	DesignAssetSessionStatusExpired   DesignAssetSessionStatus = "expired"
)

func (s DesignAssetSessionStatus) Valid() bool {
	switch s {
	case DesignAssetSessionStatusCreated, DesignAssetSessionStatusCompleted, DesignAssetSessionStatusCancelled, DesignAssetSessionStatusExpired:
		return true
	default:
		return false
	}
}

type DesignAssetStorageProvider string

const (
	DesignAssetStorageProviderOSS DesignAssetStorageProvider = "oss"
)

func (p DesignAssetStorageProvider) Valid() bool {
	switch p {
	case DesignAssetStorageProviderOSS:
		return true
	default:
		return false
	}
}

type AssetArchiveStatus string

const (
	AssetArchiveStatusActive   AssetArchiveStatus = "active"
	AssetArchiveStatusArchived AssetArchiveStatus = "archived"
)

func (s AssetArchiveStatus) Valid() bool {
	switch s {
	case AssetArchiveStatusActive, AssetArchiveStatusArchived:
		return true
	default:
		return false
	}
}

type DesignAssetAccessPolicy string

const (
	DesignAssetAccessPolicyReferenceDirect  DesignAssetAccessPolicy = "reference_direct"
	DesignAssetAccessPolicySourceControlled DesignAssetAccessPolicy = "source_controlled"
	DesignAssetAccessPolicyDeliveryFlow     DesignAssetAccessPolicy = "delivery_flow"
	DesignAssetAccessPolicyPreviewAssist    DesignAssetAccessPolicy = "preview_assist"
)

type DesignAssetSourceAccessMode string

const (
	DesignAssetSourceAccessModeStandard DesignAssetSourceAccessMode = "standard"
)

type AssetDownloadMode string

const (
	AssetDownloadModeDirect AssetDownloadMode = "direct"
	AssetDownloadModeProxy  AssetDownloadMode = "proxy"
)

type AssetDownloadInfo struct {
	DownloadMode     AssetDownloadMode `json:"download_mode"`
	DownloadURL      *string           `json:"download_url"`
	AccessHint       string            `json:"access_hint"`
	PreviewAvailable bool              `json:"preview_available"`
	Filename         string            `json:"filename"`
	FileSize         int64             `json:"file_size"`
	MimeType         string            `json:"mime_type"`
	ExpiresAt        *time.Time        `json:"expires_at,omitempty"`
}

type DesignAsset struct {
	ID                      int64                   `db:"id"                 json:"id"`
	TaskID                  int64                   `db:"task_id"            json:"task_id"`
	AssetNo                 string                  `db:"asset_no"           json:"asset_no"`
	SourceAssetID           *int64                  `db:"source_asset_id"    json:"source_asset_id,omitempty"`
	ScopeSKUCode            string                  `db:"scope_sku_code"     json:"scope_sku_code,omitempty"`
	AssetType               TaskAssetType           `db:"asset_type"         json:"asset_type"`
	CurrentVersionID        *int64                  `db:"current_version_id" json:"current_version_id,omitempty"`
	ApprovedVersionID       *int64                  `json:"approved_version_id,omitempty"`
	WarehouseReadyVersionID *int64                  `json:"warehouse_ready_version_id,omitempty"`
	UploadStatus            DesignAssetUploadStatus `json:"upload_status,omitempty"`
	ArchiveStatus           AssetArchiveStatus      `json:"archive_status,omitempty"`
	ArchivedAt              *time.Time              `json:"archived_at,omitempty"`
	LastAccessAt            *time.Time              `json:"last_access_at,omitempty"`
	CreatedBy               int64                   `db:"created_by"         json:"created_by"`
	CreatedAt               time.Time               `db:"created_at"         json:"created_at"`
	UpdatedAt               time.Time               `db:"updated_at"         json:"updated_at"`
	CurrentVersion          *DesignAssetVersion     `json:"current_version,omitempty"`
	ApprovedVersion         *DesignAssetVersion     `json:"approved_version,omitempty"`
	WarehouseReadyVersion   *DesignAssetVersion     `json:"warehouse_ready_version,omitempty"`
}

type DesignAssetVersion struct {
	ID                    int64                       `json:"id"`
	TaskID                int64                       `json:"task_id"`
	TaskNo                string                      `json:"task_no,omitempty"`
	AssetID               int64                       `json:"asset_id"`
	AssetNo               string                      `json:"asset_no,omitempty"`
	SourceAssetID         *int64                      `json:"source_asset_id,omitempty"`
	ScopeSKUCode          string                      `json:"scope_sku_code,omitempty"`
	AssetType             TaskAssetType               `json:"asset_type"`
	VersionNo             int                         `json:"version_no"`
	TimelineVersionNo     int                         `json:"timeline_version_no"`
	UploadMode            DesignAssetUploadMode       `json:"upload_mode"`
	OriginalFilename      string                      `json:"original_filename"`
	RemoteFileID          *string                     `json:"remote_file_id,omitempty"`
	StorageKey            string                      `json:"storage_key"`
	FileSize              *int64                      `json:"file_size,omitempty"`
	FileHash              *string                     `json:"file_hash,omitempty"`
	MimeType              string                      `json:"mime_type,omitempty"`
	UploadStatus          DesignAssetUploadStatus     `json:"upload_status"`
	PreviewStatus         DesignAssetPreviewStatus    `json:"preview_status"`
	UploadedBy            int64                       `json:"uploaded_by"`
	UploadedByName        string                      `json:"uploader_name,omitempty"`
	UploadedAt            *time.Time                  `json:"uploaded_at,omitempty"`
	Remark                string                      `json:"remark,omitempty"`
	UploadSessionID       *string                     `json:"upload_session_id,omitempty"`
	IsSourceFile          bool                        `json:"is_source_file"`
	IsDeliveryFile        bool                        `json:"is_delivery_file"`
	IsPreviewFile         bool                        `json:"is_preview_file"`
	IsDesignThumb         bool                        `json:"is_design_thumb"`
	SourceAccessMode      DesignAssetSourceAccessMode `json:"source_access_mode"`
	AccessPolicy          DesignAssetAccessPolicy     `json:"access_policy"`
	StorageRefStatus      AssetStorageRefStatus       `json:"-"`
	PreviewAvailable      bool                        `json:"preview_available"`
	WarehouseReady        bool                        `json:"warehouse_ready"`
	ApprovedForFlow       bool                        `json:"approved_for_flow"`
	CurrentVersionRole    string                      `json:"current_version_role,omitempty"`
	DownloadURL           *string                     `json:"download_url"`
	PublicDownloadAllowed bool                        `json:"public_download_allowed"`
	PreviewPublicAllowed  bool                        `json:"preview_public_allowed"`
	AccessHint            string                      `json:"access_hint,omitempty"`
	Notes                 string                      `json:"notes,omitempty"`
}

type UploadSession struct {
	ID               string                     `json:"id"`
	TaskID           int64                      `json:"task_id"`
	AssetID          *int64                     `json:"asset_id,omitempty"`
	AssetType        *TaskAssetType             `json:"asset_type,omitempty"`
	TargetSKUCode    string                     `json:"target_sku_code,omitempty"`
	UploadMode       DesignAssetUploadMode      `json:"upload_mode"`
	Filename         string                     `json:"filename"`
	ExpectedSize     *int64                     `json:"expected_size,omitempty"`
	MimeType         string                     `json:"mime_type,omitempty"`
	StorageProvider  DesignAssetStorageProvider `json:"storage_provider"`
	SessionStatus    DesignAssetSessionStatus   `json:"session_status"`
	RemoteUploadID   string                     `json:"remote_upload_id,omitempty"`
	RemoteFileID     *string                    `json:"remote_file_id,omitempty"`
	CreatedBy        int64                      `json:"created_by"`
	CreatedAt        time.Time                  `json:"created_at"`
	ExpiresAt        *time.Time                 `json:"expires_at,omitempty"`
	LastSyncedAt     *time.Time                 `json:"last_synced_at,omitempty"`
	Remark           string                     `json:"remark,omitempty"`
	CurrentVersionID *int64                     `json:"current_version_id,omitempty"`
}

func BuildDesignAssetVersion(taskAsset *TaskAsset) *DesignAssetVersion {
	if taskAsset == nil || taskAsset.AssetID == nil || taskAsset.AssetVersionNo == nil {
		return nil
	}
	assetType := NormalizeTaskAssetType(taskAsset.AssetType)
	originalFilename := taskAsset.FileName
	if taskAsset.OriginalName != nil && *taskAsset.OriginalName != "" {
		originalFilename = *taskAsset.OriginalName
	}
	storageKey := ""
	if taskAsset.StorageKey != nil {
		storageKey = *taskAsset.StorageKey
	} else if taskAsset.StorageRef != nil {
		storageKey = taskAsset.StorageRef.RefKey
	}
	storageRefStatus := AssetStorageRefStatus("")
	if taskAsset.StorageRef != nil {
		storageRefStatus = taskAsset.StorageRef.Status
		if storageRefStatus == AssetStorageRefStatusArchived {
			storageKey = ""
		}
	}
	mimeType := ""
	if taskAsset.MimeType != nil {
		mimeType = *taskAsset.MimeType
	}
	uploadStatus := DesignAssetUploadStatusPending
	if taskAsset.UploadStatus != nil && DesignAssetUploadStatus(*taskAsset.UploadStatus).Valid() {
		uploadStatus = DesignAssetUploadStatus(*taskAsset.UploadStatus)
	}
	previewStatus := DesignAssetPreviewStatusPending
	if taskAsset.PreviewStatus != nil && DesignAssetPreviewStatus(*taskAsset.PreviewStatus).Valid() {
		previewStatus = DesignAssetPreviewStatus(*taskAsset.PreviewStatus)
	}
	uploadMode := DesignAssetUploadModeSmall
	if taskAsset.UploadMode != nil && DesignAssetUploadMode(*taskAsset.UploadMode).Valid() {
		uploadMode = DesignAssetUploadMode(*taskAsset.UploadMode)
	}
	return &DesignAssetVersion{
		ID:                taskAsset.ID,
		TaskID:            taskAsset.TaskID,
		AssetID:           *taskAsset.AssetID,
		ScopeSKUCode:      optionalTrimmedString(taskAsset.ScopeSKUCode),
		AssetType:         assetType,
		VersionNo:         *taskAsset.AssetVersionNo,
		TimelineVersionNo: taskAsset.VersionNo,
		UploadMode:        uploadMode,
		OriginalFilename:  originalFilename,
		RemoteFileID:      taskAsset.RemoteFileID,
		StorageKey:        storageKey,
		FileSize:          taskAsset.FileSize,
		FileHash:          taskAsset.WholeHash,
		MimeType:          mimeType,
		UploadStatus:      uploadStatus,
		PreviewStatus:     previewStatus,
		UploadedBy:        taskAsset.UploadedBy,
		UploadedAt:        taskAsset.UploadedAt,
		Remark:            taskAsset.Remark,
		UploadSessionID:   taskAsset.UploadRequestID,
		StorageRefStatus:  storageRefStatus,
	}
}

func BuildUploadSession(request *UploadRequest) *UploadSession {
	if request == nil {
		return nil
	}
	taskID := request.TaskID
	if taskID == 0 && request.OwnerType == AssetOwnerTypeTask {
		taskID = request.OwnerID
	}
	expectedSize := request.ExpectedSize
	if expectedSize == nil {
		expectedSize = request.FileSize
	}
	sessionStatus := request.SessionStatus
	if !sessionStatus.Valid() {
		sessionStatus = deriveUploadSessionStatus(request.Status)
	}
	uploadMode := request.UploadMode
	if !uploadMode.Valid() {
		uploadMode = DesignAssetUploadModeSmall
	}
	storageProvider := request.StorageProvider
	if !storageProvider.Valid() {
		storageProvider = DesignAssetStorageProviderOSS
	}
	return &UploadSession{
		ID:               request.RequestID,
		TaskID:           taskID,
		AssetID:          request.AssetID,
		AssetType:        normalizeTaskAssetTypePtr(request.TaskAssetType),
		TargetSKUCode:    strings.TrimSpace(request.TargetSKUCode),
		UploadMode:       uploadMode,
		Filename:         request.FileName,
		ExpectedSize:     expectedSize,
		MimeType:         request.MimeType,
		StorageProvider:  storageProvider,
		SessionStatus:    sessionStatus,
		RemoteUploadID:   request.RemoteUploadID,
		RemoteFileID:     buildOptionalString(request.RemoteFileID),
		CreatedBy:        request.CreatedBy,
		CreatedAt:        request.CreatedAt,
		ExpiresAt:        request.ExpiresAt,
		LastSyncedAt:     request.LastSyncedAt,
		Remark:           request.Remark,
		CurrentVersionID: request.BoundAssetID,
	}
}

func normalizeTaskAssetTypePtr(assetType *TaskAssetType) *TaskAssetType {
	if assetType == nil {
		return nil
	}
	normalized := NormalizeTaskAssetType(*assetType)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func buildOptionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func optionalTrimmedString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func deriveUploadSessionStatus(status UploadRequestStatus) DesignAssetSessionStatus {
	switch status {
	case UploadRequestStatusBound:
		return DesignAssetSessionStatusCompleted
	case UploadRequestStatusCancelled:
		return DesignAssetSessionStatusCancelled
	case UploadRequestStatusExpired:
		return DesignAssetSessionStatusExpired
	default:
		return DesignAssetSessionStatusCreated
	}
}
