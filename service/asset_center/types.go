package asset_center

import (
	"time"

	"workflow/domain"
)

const ErrCodeAssetGone = "ASSET_GONE"

type Actor struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username,omitempty"`
	Name     string `json:"name,omitempty"`
}

type AssetVersion struct {
	VersionID             int64                            `json:"version_id"`
	VersionNo             int                              `json:"version_no"`
	StorageKey            *string                          `json:"storage_key"`
	FileSize              *int64                           `json:"file_size"`
	MimeType              *string                          `json:"mime_type"`
	FlowReviewStatus      domain.TaskAssetFlowReviewStatus `json:"flow_review_status,omitempty"`
	UsableState           domain.TaskAssetUsableState      `json:"usable_state,omitempty"`
	ApprovedAt            *time.Time                       `json:"approved_at,omitempty"`
	ApprovedBy            *int64                           `json:"approved_by,omitempty"`
	RejectedAt            *time.Time                       `json:"rejected_at,omitempty"`
	RejectedBy            *int64                           `json:"rejected_by,omitempty"`
	SupersededByVersionID *int64                           `json:"superseded_by_version_id,omitempty"`
	SupersededAt          *time.Time                       `json:"superseded_at,omitempty"`
	CleanupAfterAt        *time.Time                       `json:"cleanup_after_at,omitempty"`
	CreatedAt             time.Time                        `json:"created_at"`
	CreatedBy             Actor                            `json:"created_by"`
}

type AssetDetail struct {
	ID                    int64                            `json:"id"`
	ResourceID            string                           `json:"resource_id,omitempty"`
	SourceType            string                           `json:"source_type,omitempty"`
	SourceLabel           string                           `json:"source_label,omitempty"`
	TaskID                int64                            `json:"task_id"`
	AssetNo               string                           `json:"asset_no,omitempty"`
	ScopeSKUCode          string                           `json:"scope_sku_code,omitempty"`
	AssetType             domain.TaskAssetType             `json:"asset_type"`
	CurrentVersionID      *int64                           `json:"current_version_id,omitempty"`
	SourceModuleKey       string                           `json:"source_module_key"`
	LifecycleState        domain.AssetLifecycleState       `json:"lifecycle_state"`
	ArchiveStatus         domain.AssetArchiveStatus        `json:"archive_status,omitempty"`
	UploadStatus          domain.DesignAssetUploadStatus   `json:"upload_status,omitempty"`
	CurrentStorageKey     *string                          `json:"storage_key,omitempty"`
	FileName              string                           `json:"file_name,omitempty"`
	OriginalFilename      string                           `json:"original_filename,omitempty"`
	FileSize              *int64                           `json:"file_size,omitempty"`
	MimeType              string                           `json:"mime_type,omitempty"`
	DownloadURL           *string                          `json:"download_url,omitempty"`
	PreviewAvailable      bool                             `json:"preview_available,omitempty"`
	FlowReviewStatus      domain.TaskAssetFlowReviewStatus `json:"flow_review_status,omitempty"`
	UsableState           domain.TaskAssetUsableState      `json:"usable_state,omitempty"`
	UsableLabel           string                           `json:"usable_label,omitempty"`
	ApprovedAt            *time.Time                       `json:"approved_at,omitempty"`
	ApprovedBy            *int64                           `json:"approved_by,omitempty"`
	RejectedAt            *time.Time                       `json:"rejected_at,omitempty"`
	RejectedBy            *int64                           `json:"rejected_by,omitempty"`
	SupersededByVersionID *int64                           `json:"superseded_by_version_id,omitempty"`
	SupersededAt          *time.Time                       `json:"superseded_at,omitempty"`
	CleanupAfterAt        *time.Time                       `json:"cleanup_after_at,omitempty"`
	TaskNo                string                           `json:"task_no,omitempty"`
	SKUCode               string                           `json:"sku_code,omitempty"`
	PrimarySKUCode        string                           `json:"primary_sku_code,omitempty"`
	ProductName           string                           `json:"product_name,omitempty"`
	TaskStatus            domain.TaskStatus                `json:"task_status,omitempty"`
	OwnerTeamCode         string                           `json:"owner_team_code,omitempty"`
	CreatedBy             int64                            `json:"created_by,omitempty"`
	CreatedByUsername     string                           `json:"created_by_username,omitempty"`
	CreatedByName         string                           `json:"created_by_name,omitempty"`
	TaskCreatorID         int64                            `json:"task_creator_id,omitempty"`
	TaskCreatorUsername   string                           `json:"task_creator_username,omitempty"`
	TaskCreatorName       string                           `json:"task_creator_name,omitempty"`
	CreatedAt             time.Time                        `json:"created_at"`
	UpdatedAt             time.Time                        `json:"updated_at"`
	Versions              []AssetVersion                   `json:"versions,omitempty"`
	ArchivedAt            *time.Time                       `json:"archived_at,omitempty"`
	ArchivedBy            *Actor                           `json:"archived_by,omitempty"`
	CleanedAt             *time.Time                       `json:"cleaned_at,omitempty"`
	DeletedAt             *time.Time                       `json:"deleted_at,omitempty"`
	ExternalKind          string                           `json:"external_kind,omitempty"`
	ExternalMountPath     string                           `json:"external_mount_path,omitempty"`
	ExternalDriver        string                           `json:"external_driver,omitempty"`
	OriginPath            string                           `json:"origin_path,omitempty"`
	OSSSyncStatus         string                           `json:"oss_sync_status,omitempty"`
	ExternalPreviewStatus string                           `json:"external_preview_status,omitempty"`
	LastPrepareError      string                           `json:"last_prepare_error,omitempty"`
}

type SearchResult struct {
	Items []*AssetDetail
	Total int64
	Page  int
	Size  int
}

type MaterialBrowseQuery struct {
	Path   string
	Source domain.AssetResourceSource
	Page   int
	Size   int
}

type MaterialFolder struct {
	Path            string `json:"path"`
	Name            string `json:"name"`
	SourceType      string `json:"source_type"`
	FileCount       int64  `json:"file_count"`
	DirectFileCount int64  `json:"direct_file_count"`
}

type MaterialBrowseResult struct {
	Path    string           `json:"path"`
	Folders []MaterialFolder `json:"folders"`
	Files   []*AssetDetail   `json:"files"`
	Total   int64            `json:"total"`
	Page    int              `json:"page"`
	Size    int              `json:"size"`
}
