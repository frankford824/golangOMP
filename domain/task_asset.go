package domain

import "time"

// TaskAssetType identifies the task-scoped asset role in the V7 timeline.
type TaskAssetType string

const (
	TaskAssetTypeReference   TaskAssetType = "reference"
	TaskAssetTypeSource      TaskAssetType = "source"
	TaskAssetTypeDelivery    TaskAssetType = "delivery"
	TaskAssetTypePreview     TaskAssetType = "preview"
	TaskAssetTypeDesignThumb TaskAssetType = "design_thumb"

	// Legacy aliases kept for backward-compatible input normalization.
	TaskAssetTypeOriginal        TaskAssetType = "original"
	TaskAssetTypeDraft           TaskAssetType = "draft"
	TaskAssetTypeRevised         TaskAssetType = "revised"
	TaskAssetTypeFinal           TaskAssetType = "final"
	TaskAssetTypeOutsourceReturn TaskAssetType = "outsource_return"
)

func (t TaskAssetType) Canonical() TaskAssetType {
	switch t {
	case TaskAssetTypeReference:
		return TaskAssetTypeReference
	case TaskAssetTypeSource, TaskAssetTypeOriginal:
		return TaskAssetTypeSource
	case TaskAssetTypeDelivery, TaskAssetTypeDraft, TaskAssetTypeRevised, TaskAssetTypeFinal, TaskAssetTypeOutsourceReturn:
		return TaskAssetTypeDelivery
	case TaskAssetTypePreview:
		return TaskAssetTypePreview
	case TaskAssetTypeDesignThumb:
		return TaskAssetTypeDesignThumb
	default:
		return ""
	}
}

func (t TaskAssetType) Valid() bool {
	return t.Canonical() != ""
}

func (t TaskAssetType) IsReference() bool {
	return t.Canonical() == TaskAssetTypeReference
}

func (t TaskAssetType) IsSource() bool {
	return t.Canonical() == TaskAssetTypeSource
}

func (t TaskAssetType) IsDelivery() bool {
	return t.Canonical() == TaskAssetTypeDelivery
}

func (t TaskAssetType) IsPreview() bool {
	return t.Canonical() == TaskAssetTypePreview
}

func (t TaskAssetType) IsDesignThumb() bool {
	return t.Canonical() == TaskAssetTypeDesignThumb
}

func NormalizeTaskAssetType(assetType TaskAssetType) TaskAssetType {
	return assetType.Canonical()
}

// TaskAsset is the lightweight V7 task asset record used by the frontend asset timeline.
// It deliberately does not reuse V6 asset_versions semantics.
type TaskAsset struct {
	ID                 int64            `db:"id"                json:"id"`
	TaskID             int64            `db:"task_id"           json:"task_id"`
	AssetID            *int64           `db:"asset_id"          json:"asset_id,omitempty"`
	ScopeSKUCode       *string          `db:"scope_sku_code"    json:"scope_sku_code,omitempty"`
	AssetType          TaskAssetType    `db:"asset_type"        json:"asset_type"`
	VersionNo          int              `db:"version_no"        json:"version_no"`
	AssetVersionNo     *int             `db:"asset_version_no"  json:"asset_version_no,omitempty"`
	UploadMode         *string          `db:"upload_mode"       json:"upload_mode,omitempty"`
	UploadRequestID    *string          `db:"upload_request_id" json:"upload_request_id,omitempty"`
	StorageRefID       *string          `db:"storage_ref_id"    json:"storage_ref_id,omitempty"`
	FileName           string           `db:"file_name"         json:"file_name"`
	OriginalName       *string          `db:"original_filename" json:"original_filename,omitempty"`
	RemoteFileID       *string          `db:"remote_file_id"    json:"remote_file_id,omitempty"`
	MimeType           *string          `db:"mime_type"         json:"mime_type,omitempty"`
	FileSize           *int64           `db:"file_size"         json:"file_size,omitempty"`
	FilePath           *string          `db:"file_path"         json:"file_path,omitempty"`
	StorageKey         *string          `db:"storage_key"       json:"storage_key,omitempty"`
	WholeHash          *string          `db:"whole_hash"        json:"whole_hash,omitempty"`
	UploadStatus       *string          `db:"upload_status"     json:"upload_status,omitempty"`
	PreviewStatus      *string          `db:"preview_status"    json:"preview_status,omitempty"`
	UploadedBy         int64            `db:"uploaded_by"       json:"uploaded_by"`
	UploadedByName     string           `json:"uploader_name,omitempty"`
	UploadedAt         *time.Time       `db:"uploaded_at"       json:"uploaded_at,omitempty"`
	Remark             string           `db:"remark"            json:"remark"`
	CreatedAt          time.Time        `db:"created_at"        json:"created_at"`
	StorageRef         *AssetStorageRef `json:"storage_ref,omitempty"`
	SourceModuleKey    string           `db:"source_module_key" json:"source_module_key,omitempty"`
	SourceTaskModuleID *int64           `db:"source_task_module_id" json:"source_task_module_id,omitempty"`
	IsArchived         bool             `db:"is_archived" json:"is_archived,omitempty"`
	ArchivedAt         *time.Time       `db:"archived_at" json:"archived_at,omitempty"`
	ArchivedBy         *int64           `db:"archived_by" json:"archived_by,omitempty"`
	CleanedAt          *time.Time       `db:"cleaned_at" json:"cleaned_at,omitempty"`
	DeletedAt          *time.Time       `db:"deleted_at" json:"deleted_at,omitempty"`
}
