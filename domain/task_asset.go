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
	TaskAssetTypeERPProduct  TaskAssetType = "erp_product_image"

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
	case TaskAssetTypeERPProduct:
		return TaskAssetTypeERPProduct
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

func (t TaskAssetType) IsERPProductImage() bool {
	return t.Canonical() == TaskAssetTypeERPProduct
}

func NormalizeTaskAssetType(assetType TaskAssetType) TaskAssetType {
	return assetType.Canonical()
}

type TaskAssetFlowReviewStatus string

const (
	TaskAssetFlowReviewStatusNotApplicable TaskAssetFlowReviewStatus = "not_applicable"
	TaskAssetFlowReviewStatusPendingReview TaskAssetFlowReviewStatus = "pending_review"
	TaskAssetFlowReviewStatusApproved      TaskAssetFlowReviewStatus = "approved"
	TaskAssetFlowReviewStatusRejected      TaskAssetFlowReviewStatus = "rejected"
	TaskAssetFlowReviewStatusSuperseded    TaskAssetFlowReviewStatus = "superseded"
	TaskAssetFlowReviewStatusCleaned       TaskAssetFlowReviewStatus = "cleaned"
)

func (s TaskAssetFlowReviewStatus) Valid() bool {
	switch s {
	case TaskAssetFlowReviewStatusNotApplicable,
		TaskAssetFlowReviewStatusPendingReview,
		TaskAssetFlowReviewStatusApproved,
		TaskAssetFlowReviewStatusRejected,
		TaskAssetFlowReviewStatusSuperseded,
		TaskAssetFlowReviewStatusCleaned:
		return true
	default:
		return false
	}
}

func NormalizeTaskAssetFlowReviewStatus(status TaskAssetFlowReviewStatus, assetType TaskAssetType) TaskAssetFlowReviewStatus {
	if status.Valid() {
		return status
	}
	if NormalizeTaskAssetType(assetType).IsDelivery() {
		return TaskAssetFlowReviewStatusPendingReview
	}
	return TaskAssetFlowReviewStatusNotApplicable
}

type TaskAssetUsableState string

const (
	TaskAssetUsableStateNotApplicable TaskAssetUsableState = "not_applicable"
	TaskAssetUsableStatePendingReview TaskAssetUsableState = "pending_review"
	TaskAssetUsableStateReadyForUse   TaskAssetUsableState = "ready_for_use"
	TaskAssetUsableStateRejected      TaskAssetUsableState = "rejected"
	TaskAssetUsableStateHistory       TaskAssetUsableState = "history"
	TaskAssetUsableStateCleaned       TaskAssetUsableState = "cleaned"
)

func DeriveTaskAssetUsableState(asset TaskAsset) TaskAssetUsableState {
	if asset.CleanedAt != nil || asset.StorageKey == nil || *asset.StorageKey == "" {
		if NormalizeTaskAssetType(asset.AssetType).IsDelivery() {
			return TaskAssetUsableStateCleaned
		}
		return TaskAssetUsableStateNotApplicable
	}
	status := NormalizeTaskAssetFlowReviewStatus(asset.FlowReviewStatus, asset.AssetType)
	switch status {
	case TaskAssetFlowReviewStatusApproved:
		return TaskAssetUsableStateReadyForUse
	case TaskAssetFlowReviewStatusRejected:
		return TaskAssetUsableStateRejected
	case TaskAssetFlowReviewStatusSuperseded:
		return TaskAssetUsableStateHistory
	case TaskAssetFlowReviewStatusCleaned:
		return TaskAssetUsableStateCleaned
	case TaskAssetFlowReviewStatusPendingReview:
		return TaskAssetUsableStatePendingReview
	default:
		return TaskAssetUsableStateNotApplicable
	}
}

// TaskAsset is the lightweight V7 task asset record used by the frontend asset timeline.
// It deliberately does not reuse V6 asset_versions semantics.
type TaskAsset struct {
	ID                    int64                     `db:"id"                json:"id"`
	TaskID                int64                     `db:"task_id"           json:"task_id"`
	AssetID               *int64                    `db:"asset_id"          json:"asset_id,omitempty"`
	ScopeSKUCode          *string                   `db:"scope_sku_code"            json:"scope_sku_code,omitempty"`
	RetouchRequirementID  *int64                    `db:"retouch_requirement_id"    json:"retouch_requirement_id,omitempty"`
	AssetType             TaskAssetType             `db:"asset_type" json:"asset_type"`
	VersionNo             int                       `db:"version_no"        json:"version_no"`
	AssetVersionNo        *int                      `db:"asset_version_no"  json:"asset_version_no,omitempty"`
	UploadMode            *string                   `db:"upload_mode"       json:"upload_mode,omitempty"`
	UploadRequestID       *string                   `db:"upload_request_id" json:"upload_request_id,omitempty"`
	StorageRefID          *string                   `db:"storage_ref_id"    json:"storage_ref_id,omitempty"`
	FileName              string                    `db:"file_name"         json:"file_name"`
	OriginalName          *string                   `db:"original_filename" json:"original_filename,omitempty"`
	RemoteFileID          *string                   `db:"remote_file_id"    json:"remote_file_id,omitempty"`
	MimeType              *string                   `db:"mime_type"         json:"mime_type,omitempty"`
	FileSize              *int64                    `db:"file_size"         json:"file_size,omitempty"`
	FilePath              *string                   `db:"file_path"         json:"file_path,omitempty"`
	StorageKey            *string                   `db:"storage_key"       json:"storage_key,omitempty"`
	WholeHash             *string                   `db:"whole_hash"        json:"whole_hash,omitempty"`
	UploadStatus          *string                   `db:"upload_status"     json:"upload_status,omitempty"`
	PreviewStatus         *string                   `db:"preview_status"    json:"preview_status,omitempty"`
	UploadedBy            int64                     `db:"uploaded_by"       json:"uploaded_by"`
	UploadedByName        string                    `json:"uploader_name,omitempty"`
	UploadedAt            *time.Time                `db:"uploaded_at"       json:"uploaded_at,omitempty"`
	Remark                string                    `db:"remark"            json:"remark"`
	CreatedAt             time.Time                 `db:"created_at"        json:"created_at"`
	StorageRef            *AssetStorageRef          `json:"storage_ref,omitempty"`
	SourceModuleKey       string                    `db:"source_module_key" json:"source_module_key,omitempty"`
	SourceTaskModuleID    *int64                    `db:"source_task_module_id" json:"source_task_module_id,omitempty"`
	IsArchived            bool                      `db:"is_archived" json:"is_archived,omitempty"`
	ArchivedAt            *time.Time                `db:"archived_at" json:"archived_at,omitempty"`
	ArchivedBy            *int64                    `db:"archived_by" json:"archived_by,omitempty"`
	FlowReviewStatus      TaskAssetFlowReviewStatus `db:"flow_review_status" json:"flow_review_status,omitempty"`
	ApprovedAt            *time.Time                `db:"approved_at" json:"approved_at,omitempty"`
	ApprovedBy            *int64                    `db:"approved_by" json:"approved_by,omitempty"`
	RejectedAt            *time.Time                `db:"rejected_at" json:"rejected_at,omitempty"`
	RejectedBy            *int64                    `db:"rejected_by" json:"rejected_by,omitempty"`
	SupersededByVersionID *int64                    `db:"superseded_by_version_id" json:"superseded_by_version_id,omitempty"`
	SupersededAt          *time.Time                `db:"superseded_at" json:"superseded_at,omitempty"`
	CleanupAfterAt        *time.Time                `db:"cleanup_after_at" json:"cleanup_after_at,omitempty"`
	SourceAssetVersionID  *int64                    `db:"source_asset_version_id" json:"source_asset_version_id,omitempty"`
	CleanedAt             *time.Time                `db:"cleaned_at" json:"cleaned_at,omitempty"`
	DeletedAt             *time.Time                `db:"deleted_at" json:"deleted_at,omitempty"`
}
