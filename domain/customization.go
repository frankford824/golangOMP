package domain

import "time"

type CustomizationSourceType string

const (
	CustomizationSourceTypeNewProduct      CustomizationSourceType = "new_product"
	CustomizationSourceTypeExistingProduct CustomizationSourceType = "existing_product"
)

func (s CustomizationSourceType) Valid() bool {
	switch s {
	case CustomizationSourceTypeNewProduct, CustomizationSourceTypeExistingProduct:
		return true
	default:
		return false
	}
}

type CustomizationJobStatus string

const (
	CustomizationJobStatusInProgress     CustomizationJobStatus = "in_progress"
	CustomizationJobStatusReadyForSubmit CustomizationJobStatus = "ready_for_submit"
	CustomizationJobStatusCompleted      CustomizationJobStatus = "completed"
)

func (s CustomizationJobStatus) Valid() bool {
	switch s {
	case CustomizationJobStatusInProgress,
		CustomizationJobStatusReadyForSubmit,
		CustomizationJobStatusCompleted:
		return true
	default:
		return false
	}
}

type CustomizationJob struct {
	ID                          int64    `db:"id" json:"id"`
	TaskID                      int64    `db:"task_id" json:"task_id"`
	OrderNo                     string   `db:"order_no" json:"order_no"`
	SourceAssetID               *int64   `db:"source_asset_id" json:"source_asset_id,omitempty"`
	CurrentAssetID              *int64   `db:"current_asset_id" json:"current_asset_id,omitempty"`
	CustomizationLevelCode      string   `db:"customization_level_code" json:"customization_level_code"`
	CustomizationLevelName      string   `db:"customization_level_name" json:"customization_level_name"`
	ReviewReferenceUnitPrice    *float64 `db:"review_reference_unit_price" json:"review_reference_unit_price,omitempty"`
	ReviewReferenceWeightFactor *float64 `db:"review_reference_weight_factor" json:"review_reference_weight_factor,omitempty"`
	UnitPrice                   *float64 `db:"unit_price" json:"unit_price,omitempty"`
	WeightFactor                *float64 `db:"weight_factor" json:"weight_factor,omitempty"`
	Note                        string   `db:"note" json:"note"`
	// The following persisted values are historical evidence only. The v8 flow
	// never reads or writes them as workflow decisions.
	LegacyReviewDecision        string                 `db:"customization_review_decision" json:"-"`
	LegacyDecisionType          string                 `db:"decision_type" json:"-"`
	AssignedOperatorID          *int64                 `db:"assigned_operator_id" json:"assigned_operator_id,omitempty"`
	LastOperatorID              *int64                 `db:"last_operator_id" json:"last_operator_id,omitempty"`
	PricingWorkerType           EmploymentType         `db:"pricing_worker_type" json:"pricing_worker_type,omitempty"`
	Status                      CustomizationJobStatus `db:"status" json:"status"`
	LegacyWarehouseRejectReason string                 `db:"warehouse_reject_reason" json:"-"`
	LegacyWarehouseRejectType   string                 `db:"warehouse_reject_category" json:"-"`
	CreatedAt                   time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt                   time.Time              `db:"updated_at" json:"updated_at"`
}

type CustomizationPricingRule struct {
	ID                     int64          `db:"id" json:"id"`
	CustomizationLevelCode string         `db:"customization_level_code" json:"customization_level_code"`
	EmploymentType         EmploymentType `db:"employment_type" json:"employment_type"`
	UnitPrice              float64        `db:"unit_price" json:"unit_price"`
	WeightFactor           float64        `db:"weight_factor" json:"weight_factor"`
	IsEnabled              bool           `db:"is_enabled" json:"is_enabled"`
	CreatedAt              time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time      `db:"updated_at" json:"updated_at"`
}
