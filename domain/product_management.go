package domain

import "time"

type ProductManagementImageSource string

const (
	ProductManagementImageSourceManual         ProductManagementImageSource = "manual"
	ProductManagementImageSourceDelivery       ProductManagementImageSource = "delivery"
	ProductManagementImageSourceDerivedPreview ProductManagementImageSource = "derived_preview"
	ProductManagementImageSourceTaskReference  ProductManagementImageSource = "task_reference"
	ProductManagementImageSourceMissing        ProductManagementImageSource = "missing"
)

type ProductManagementImageSelectionMode string

const (
	ProductManagementImageSelectionAuto   ProductManagementImageSelectionMode = "auto"
	ProductManagementImageSelectionManual ProductManagementImageSelectionMode = "manual"
)

type ProductManagementERPSyncStatus string

const (
	ProductManagementERPSyncStatusPendingSync ProductManagementERPSyncStatus = "pending_sync"
	ProductManagementERPSyncStatusQueued      ProductManagementERPSyncStatus = "queued"
	ProductManagementERPSyncStatusSyncing     ProductManagementERPSyncStatus = "syncing"
	ProductManagementERPSyncStatusSynced      ProductManagementERPSyncStatus = "synced"
	ProductManagementERPSyncStatusFailed      ProductManagementERPSyncStatus = "failed"
	ProductManagementERPSyncStatusCoolingDown ProductManagementERPSyncStatus = "cooling_down"
)

type ProductManagementRecord struct {
	ID                  int64                               `json:"id"`
	RecordKey           string                              `json:"record_key"`
	TaskID              int64                               `json:"task_id"`
	TaskSKUItemID       *int64                              `json:"task_sku_item_id,omitempty"`
	TaskNo              string                              `json:"task_no"`
	SKUCode             string                              `json:"sku_code"`
	ProductIID          string                              `json:"product_i_id"`
	ProductName         string                              `json:"product_name"`
	CostPrice           *float64                            `json:"cost_price,omitempty"`
	CreatorID           int64                               `json:"creator_id"`
	CreatorName         string                              `json:"creator_name"`
	TaskCreatedAt       time.Time                           `json:"task_created_at"`
	ImageSource         ProductManagementImageSource        `json:"image_source"`
	ImageSourceLabel    string                              `json:"image_source_label"`
	ImageSelectionMode  ProductManagementImageSelectionMode `json:"image_selection_mode"`
	ImageAssetID        *int64                              `json:"image_asset_id,omitempty"`
	ImageAssetVersionID *int64                              `json:"image_asset_version_id,omitempty"`
	ImagePreviewURL     string                              `json:"image_preview_url,omitempty"`
	ImageFilename       string                              `json:"image_filename,omitempty"`
	ImageMimeType       string                              `json:"image_mime_type,omitempty"`
	ImageMissingReason  string                              `json:"image_missing_reason,omitempty"`
	ERPSyncStatus       ProductManagementERPSyncStatus      `json:"erp_sync_status"`
	LastERPSyncedAt     *time.Time                          `json:"last_erp_synced_at,omitempty"`
	LastERPCheckedAt    *time.Time                          `json:"last_erp_checked_at,omitempty"`
	SyncCooldownUntil   *time.Time                          `json:"sync_cooldown_until,omitempty"`
	LastSyncError       string                              `json:"last_sync_error,omitempty"`
	CanMaintainImage    bool                                `json:"can_maintain_image"`
	CanCrossTaskSelect  bool                                `json:"can_cross_task_select"`
	CanSyncERP          bool                                `json:"can_sync_erp"`
	CanForceOverride    bool                                `json:"can_force_override"`
	CreatedAt           time.Time                           `json:"created_at"`
	UpdatedAt           time.Time                           `json:"updated_at"`
}

type ProductManagementImageCandidate struct {
	AssetID        int64                        `json:"asset_id"`
	AssetVersionID int64                        `json:"asset_version_id"`
	TaskID         int64                        `json:"task_id"`
	TaskNo         string                       `json:"task_no"`
	SKUCode        string                       `json:"sku_code,omitempty"`
	Source         ProductManagementImageSource `json:"source"`
	SourceLabel    string                       `json:"source_label"`
	PreviewURL     string                       `json:"preview_url,omitempty"`
	FileName       string                       `json:"file_name"`
	MimeType       string                       `json:"mime_type,omitempty"`
	CreatedAt      time.Time                    `json:"created_at"`
}

func ProductManagementImageSourceLabel(source ProductManagementImageSource) string {
	switch source {
	case ProductManagementImageSourceManual:
		return "人工指定 ERP 图"
	case ProductManagementImageSourceDelivery:
		return "当前 SKU 成品图"
	case ProductManagementImageSourceDerivedPreview:
		return "成品图派生预览图"
	case ProductManagementImageSourceTaskReference:
		return "当前任务参考图"
	default:
		return "ERP 图片待补充"
	}
}
