package asset_center

import (
	"time"

	"workflow/domain"
)

const ErrCodeAssetGone = "ASSET_GONE"

type Actor struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name,omitempty"`
}

type AssetVersion struct {
	VersionID  int64     `json:"version_id"`
	VersionNo  int       `json:"version_no"`
	StorageKey *string   `json:"storage_key"`
	FileSize   *int64    `json:"file_size"`
	MimeType   *string   `json:"mime_type"`
	CreatedAt  time.Time `json:"created_at"`
	CreatedBy  Actor     `json:"created_by"`
}

type AssetDetail struct {
	ID                int64                          `json:"id"`
	TaskID            int64                          `json:"task_id"`
	AssetNo           string                         `json:"asset_no,omitempty"`
	ScopeSKUCode      string                         `json:"scope_sku_code,omitempty"`
	AssetType         domain.TaskAssetType           `json:"asset_type"`
	CurrentVersionID  *int64                         `json:"current_version_id,omitempty"`
	SourceModuleKey   string                         `json:"source_module_key"`
	LifecycleState    domain.AssetLifecycleState     `json:"lifecycle_state"`
	ArchiveStatus     domain.AssetArchiveStatus      `json:"archive_status,omitempty"`
	UploadStatus      domain.DesignAssetUploadStatus `json:"upload_status,omitempty"`
	CurrentStorageKey *string                        `json:"storage_key,omitempty"`
	FileName          string                         `json:"file_name,omitempty"`
	OriginalFilename  string                         `json:"original_filename,omitempty"`
	FileSize          *int64                         `json:"file_size,omitempty"`
	MimeType          string                         `json:"mime_type,omitempty"`
	TaskNo            string                         `json:"task_no,omitempty"`
	TaskStatus        domain.TaskStatus              `json:"task_status,omitempty"`
	OwnerTeamCode     string                         `json:"owner_team_code,omitempty"`
	CreatedBy         int64                          `json:"created_by,omitempty"`
	CreatedAt         time.Time                      `json:"created_at"`
	UpdatedAt         time.Time                      `json:"updated_at"`
	Versions          []AssetVersion                 `json:"versions,omitempty"`
	ArchivedAt        *time.Time                     `json:"archived_at,omitempty"`
	ArchivedBy        *Actor                         `json:"archived_by,omitempty"`
	CleanedAt         *time.Time                     `json:"cleaned_at,omitempty"`
	DeletedAt         *time.Time                     `json:"deleted_at,omitempty"`
}

type SearchResult struct {
	Items []*AssetDetail
	Total int64
	Page  int
	Size  int
}
