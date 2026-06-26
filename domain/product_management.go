package domain

import (
	"encoding/json"
	"time"
)

type ProductManagementImageSource string

const (
	ProductManagementImageSourceManual         ProductManagementImageSource = "manual"
	ProductManagementImageSourceERPProduct     ProductManagementImageSource = "erp_product_image"
	ProductManagementImageSourceDelivery       ProductManagementImageSource = "delivery"
	ProductManagementImageSourceDerivedPreview ProductManagementImageSource = "derived_preview"
	ProductManagementImageSourceTaskReference  ProductManagementImageSource = "task_reference"
	ProductManagementImageSourceAutoOnClose    ProductManagementImageSource = "auto_on_close"
	ProductManagementImageSourceMissing        ProductManagementImageSource = "missing"
)

type ProductManagementImageSelectionMode string

const (
	ProductManagementImageSelectionAuto   ProductManagementImageSelectionMode = "auto"
	ProductManagementImageSelectionManual ProductManagementImageSelectionMode = "manual"
)

type ProductManagementERPSyncStatus string

const (
	ProductManagementERPSyncStatusPendingSync  ProductManagementERPSyncStatus = "pending_sync"
	ProductManagementERPSyncStatusQueued       ProductManagementERPSyncStatus = "queued"
	ProductManagementERPSyncStatusSyncing      ProductManagementERPSyncStatus = "syncing"
	ProductManagementERPSyncStatusSynced       ProductManagementERPSyncStatus = "synced"
	ProductManagementERPSyncStatusFailed       ProductManagementERPSyncStatus = "failed"
	ProductManagementERPSyncStatusCoolingDown  ProductManagementERPSyncStatus = "cooling_down"
	ProductManagementERPSyncStatusWaitingImage ProductManagementERPSyncStatus = "waiting_image"
)

type ProductManagementCostTrace struct {
	RuleName                 string          `json:"rule_name,omitempty"`
	RuleSource               string          `json:"rule_source,omitempty"`
	MatchedRuleVersion       *int            `json:"matched_rule_version,omitempty"`
	PrefillSource            string          `json:"prefill_source,omitempty"`
	RequiresManualReview     bool            `json:"requires_manual_review"`
	ManualCostOverride       bool            `json:"manual_cost_override"`
	ManualCostOverrideReason string          `json:"manual_cost_override_reason,omitempty"`
	InputSnapshot            json.RawMessage `json:"input_snapshot,omitempty"`
	CalculationSnapshot      json.RawMessage `json:"calculation_snapshot,omitempty"`
	SnapshotAt               *time.Time      `json:"snapshot_at,omitempty"`
}

type ProductManagementAreaTrace struct {
	WidthM      *float64 `json:"width_m,omitempty"`
	HeightM     *float64 `json:"height_m,omitempty"`
	Quantity    *float64 `json:"quantity,omitempty"`
	AreaM2      *float64 `json:"area_m2,omitempty"`
	Formula     string   `json:"formula,omitempty"`
	Source      string   `json:"source,omitempty"`
	SourceLabel string   `json:"source_label,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	Warning     string   `json:"warning,omitempty"`
}

type ProductManagementRecord struct {
	ID                  int64                               `json:"id"`
	RecordKey           string                              `json:"record_key"`
	TaskID              int64                               `json:"task_id"`
	TaskSKUItemID       *int64                              `json:"task_sku_item_id,omitempty"`
	TaskNo              string                              `json:"task_no"`
	TaskType            string                              `json:"task_type,omitempty"`
	SourceMode          string                              `json:"source_mode,omitempty"`
	SKUCode             string                              `json:"sku_code"`
	ProductIID          string                              `json:"product_i_id"`
	ERPIID              string                              `json:"erp_i_id"`
	CategoryName        string                              `json:"category_name,omitempty"`
	ProductFamily       string                              `json:"product_family,omitempty"`
	ProductName         string                              `json:"product_name"`
	CostPrice           *float64                            `json:"cost_price,omitempty"`
	CostTrace           *ProductManagementCostTrace         `json:"cost_trace,omitempty"`
	SpecText            string                              `json:"spec_text,omitempty"`
	SizeText            string                              `json:"size_text,omitempty"`
	AreaTrace           *ProductManagementAreaTrace         `json:"area_trace,omitempty"`
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
	ImageSyncSource     ProductManagementImageSource        `json:"image_sync_source,omitempty"`
	ERPSyncStatus       ProductManagementERPSyncStatus      `json:"erp_sync_status"`
	BaseSyncStatus      ProductManagementERPSyncStatus      `json:"base_sync_status"`
	ImageSyncStatus     ProductManagementERPSyncStatus      `json:"image_sync_status"`
	LastERPSyncedAt     *time.Time                          `json:"last_erp_synced_at,omitempty"`
	LastERPCheckedAt    *time.Time                          `json:"last_erp_checked_at,omitempty"`
	LastBaseSyncedAt    *time.Time                          `json:"last_base_synced_at,omitempty"`
	LastImageSyncedAt   *time.Time                          `json:"last_image_synced_at,omitempty"`
	SyncCooldownUntil   *time.Time                          `json:"sync_cooldown_until,omitempty"`
	LastSyncError       string                              `json:"last_sync_error,omitempty"`
	BaseSyncError       string                              `json:"base_sync_error,omitempty"`
	ImageSyncError      string                              `json:"image_sync_error,omitempty"`
	ImageRequired       bool                                `json:"image_required"`
	CanMaintainImage    bool                                `json:"can_maintain_image"`
	CanCrossTaskSelect  bool                                `json:"can_cross_task_select"`
	CanSyncERP          bool                                `json:"can_sync_erp"`
	CanForceOverride    bool                                `json:"can_force_override"`
	CreatedAt           time.Time                           `json:"created_at"`
	UpdatedAt           time.Time                           `json:"updated_at"`

	DimensionVariantJSON  json.RawMessage `json:"-"`
	DimensionTaskSpecText string          `json:"-"`
	DimensionTaskSizeText string          `json:"-"`
	DimensionTaskWidthM   *float64        `json:"-"`
	DimensionTaskHeightM  *float64        `json:"-"`
	DimensionTaskAreaM2   *float64        `json:"-"`
	DimensionSKUQuantity  *float64        `json:"-"`
	DimensionTaskQuantity *float64        `json:"-"`
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
	case ProductManagementImageSourceERPProduct:
		return "专项 ERP 商品图"
	case ProductManagementImageSourceAutoOnClose:
		return "结单成品图自动同步"
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
