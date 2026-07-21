package domain

import (
	"strings"
	"time"
)

type AssetOwnerType string

const (
	AssetOwnerTypeTask                AssetOwnerType = "task"
	AssetOwnerTypeTaskAsset           AssetOwnerType = "task_asset"
	AssetOwnerTypeTaskCreateReference AssetOwnerType = "task_create_reference"
	AssetOwnerTypePlanningSKUCreate   AssetOwnerType = "planning_sku_create"
	AssetOwnerTypePlanningSKURevision AssetOwnerType = "planning_sku_revision_image"
	AssetOwnerTypeExportJob           AssetOwnerType = "export_job"
)

func (t AssetOwnerType) Valid() bool {
	switch t {
	case AssetOwnerTypeTask, AssetOwnerTypeTaskAsset, AssetOwnerTypeTaskCreateReference, AssetOwnerTypePlanningSKUCreate, AssetOwnerTypePlanningSKURevision, AssetOwnerTypeExportJob:
		return true
	default:
		return false
	}
}

type AssetStorageAdapter string

const (
	AssetStorageAdapterMockUpload         AssetStorageAdapter = "mock_upload"
	AssetStorageAdapterPlaceholderStorage AssetStorageAdapter = "placeholder_storage"
	AssetStorageAdapterExportPlaceholder  AssetStorageAdapter = "export_placeholder"
	AssetStorageAdapterOSSUploadService   AssetStorageAdapter = "oss_upload_service"
)

func (t AssetStorageAdapter) Valid() bool {
	switch t {
	case AssetStorageAdapterMockUpload, AssetStorageAdapterPlaceholderStorage, AssetStorageAdapterExportPlaceholder, AssetStorageAdapterOSSUploadService:
		return true
	default:
		return false
	}
}

type AssetStorageRefType string

const (
	AssetStorageRefTypeTaskAssetObject AssetStorageRefType = "task_asset_object"
	AssetStorageRefTypeExportResult    AssetStorageRefType = "export_result"
	AssetStorageRefTypeGenericObject   AssetStorageRefType = "generic_object"
)

func (t AssetStorageRefType) Valid() bool {
	switch t {
	case AssetStorageRefTypeTaskAssetObject, AssetStorageRefTypeExportResult, AssetStorageRefTypeGenericObject:
		return true
	default:
		return false
	}
}

type UploadRequestStatus string

const (
	UploadRequestStatusRequested UploadRequestStatus = "requested"
	UploadRequestStatusBound     UploadRequestStatus = "bound"
	UploadRequestStatusExpired   UploadRequestStatus = "expired"
	UploadRequestStatusCancelled UploadRequestStatus = "cancelled"
)

func (s UploadRequestStatus) Valid() bool {
	switch s {
	case UploadRequestStatusRequested, UploadRequestStatusBound, UploadRequestStatusExpired, UploadRequestStatusCancelled:
		return true
	default:
		return false
	}
}

type UploadRequestAdvanceAction string

const (
	UploadRequestAdvanceActionCancel UploadRequestAdvanceAction = "cancel"
	UploadRequestAdvanceActionExpire UploadRequestAdvanceAction = "expire"
)

func (a UploadRequestAdvanceAction) Valid() bool {
	switch a {
	case UploadRequestAdvanceActionCancel, UploadRequestAdvanceActionExpire:
		return true
	default:
		return false
	}
}

type AssetStorageRefStatus string

const (
	AssetStorageRefStatusRecorded   AssetStorageRefStatus = "recorded"
	AssetStorageRefStatusSuperseded AssetStorageRefStatus = "superseded"
	AssetStorageRefStatusArchived   AssetStorageRefStatus = "archived"
)

func (s AssetStorageRefStatus) Valid() bool {
	switch s {
	case AssetStorageRefStatusRecorded, AssetStorageRefStatusSuperseded, AssetStorageRefStatusArchived:
		return true
	default:
		return false
	}
}

type UploadRequest struct {
	RequestID            string                     `json:"request_id"`
	OwnerType            AssetOwnerType             `json:"owner_type"`
	OwnerID              int64                      `json:"owner_id"`
	ClientCreateID       string                     `json:"client_create_id,omitempty"`
	ClientItemID         string                     `json:"client_item_id,omitempty"`
	TaskID               int64                      `json:"task_id,omitempty"`
	AssetID              *int64                     `json:"asset_id,omitempty"`
	SourceAssetID        *int64                     `json:"source_asset_id,omitempty"`
	TargetSKUCode        string                     `json:"target_sku_code,omitempty"`
	RetouchRequirementID *int64                     `json:"retouch_requirement_id,omitempty"`
	TaskAssetType        *TaskAssetType             `json:"task_asset_type,omitempty"`
	StorageAdapter       AssetStorageAdapter        `json:"storage_adapter"`
	UploadMode           DesignAssetUploadMode      `json:"upload_mode,omitempty"`
	RefType              AssetStorageRefType        `json:"ref_type"`
	FileName             string                     `json:"file_name,omitempty"`
	MimeType             string                     `json:"mime_type,omitempty"`
	FileSize             *int64                     `json:"file_size,omitempty"`
	ExpectedSize         *int64                     `json:"expected_size,omitempty"`
	ChecksumHint         string                     `json:"checksum_hint,omitempty"`
	Status               UploadRequestStatus        `json:"status"`
	StorageProvider      DesignAssetStorageProvider `json:"storage_provider,omitempty"`
	SessionStatus        DesignAssetSessionStatus   `json:"session_status,omitempty"`
	RemoteUploadID       string                     `json:"remote_upload_id,omitempty"`
	RemoteFileID         string                     `json:"remote_file_id,omitempty"`
	IsPlaceholder        bool                       `json:"is_placeholder"`
	CanBind              bool                       `json:"can_bind"`
	CanCancel            bool                       `json:"can_cancel"`
	CanExpire            bool                       `json:"can_expire"`
	BoundAssetID         *int64                     `json:"bound_asset_id,omitempty"`
	BoundRefID           string                     `json:"bound_ref_id,omitempty"`
	CreatedBy            int64                      `json:"created_by,omitempty"`
	ExpiresAt            *time.Time                 `json:"expires_at,omitempty"`
	LastSyncedAt         *time.Time                 `json:"last_synced_at,omitempty"`
	Remark               string                     `json:"remark,omitempty"`
	CreatedAt            time.Time                  `json:"created_at"`
	UpdatedAt            time.Time                  `json:"updated_at"`
}

type AssetStorageRef struct {
	RefID           string                `json:"ref_id"`
	AssetID         *int64                `json:"asset_id,omitempty"`
	OwnerType       AssetOwnerType        `json:"owner_type"`
	OwnerID         int64                 `json:"owner_id"`
	UploadRequestID string                `json:"upload_request_id,omitempty"`
	StorageAdapter  AssetStorageAdapter   `json:"storage_adapter"`
	RefType         AssetStorageRefType   `json:"ref_type"`
	RefKey          string                `json:"ref_key"`
	FileName        string                `json:"file_name,omitempty"`
	MimeType        string                `json:"mime_type,omitempty"`
	FileSize        *int64                `json:"file_size,omitempty"`
	IsPlaceholder   bool                  `json:"is_placeholder"`
	ChecksumHint    string                `json:"checksum_hint,omitempty"`
	Status          AssetStorageRefStatus `json:"status"`
	CreatedAt       time.Time             `json:"created_at"`
}

func HydrateUploadRequestDerived(request *UploadRequest) {
	if request == nil {
		return
	}
	if request.TaskAssetType != nil {
		normalized := NormalizeTaskAssetType(*request.TaskAssetType)
		if normalized != "" {
			request.TaskAssetType = &normalized
		}
	}
	if request.TaskID == 0 && request.OwnerType == AssetOwnerTypeTask {
		request.TaskID = request.OwnerID
	}
	request.TargetSKUCode = strings.TrimSpace(request.TargetSKUCode)
	if request.ExpectedSize == nil && request.FileSize != nil {
		size := *request.FileSize
		request.ExpectedSize = &size
	}
	if !request.UploadMode.Valid() {
		request.UploadMode = DesignAssetUploadModeSmall
	}
	if !request.StorageProvider.Valid() {
		request.StorageProvider = DesignAssetStorageProviderOSS
	}
	if !request.SessionStatus.Valid() {
		request.SessionStatus = deriveUploadSessionStatus(request.Status)
	}
	if request.LastSyncedAt == nil && !request.UpdatedAt.IsZero() {
		lastSyncedAt := request.UpdatedAt
		request.LastSyncedAt = &lastSyncedAt
	}
	request.CanBind = request.Status == UploadRequestStatusRequested
	request.CanCancel = request.Status == UploadRequestStatusRequested
	request.CanExpire = request.Status == UploadRequestStatusRequested
}

func HydrateAssetStorageRefDerived(ref *AssetStorageRef) {
	if ref == nil {
		return
	}
}
