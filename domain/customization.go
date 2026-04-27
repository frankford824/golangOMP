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

type CustomizationReviewDecision string

const (
	CustomizationReviewDecisionApproved         CustomizationReviewDecision = "approved"
	CustomizationReviewDecisionReturnToDesigner CustomizationReviewDecision = "return_to_designer"
	CustomizationReviewDecisionReviewerFixed    CustomizationReviewDecision = "reviewer_fixed"
)

func (d CustomizationReviewDecision) Valid() bool {
	switch d {
	case CustomizationReviewDecisionApproved, CustomizationReviewDecisionReturnToDesigner, CustomizationReviewDecisionReviewerFixed:
		return true
	default:
		return false
	}
}

type CustomizationJobDecisionType string

const (
	CustomizationJobDecisionTypeFinal         CustomizationJobDecisionType = "final"
	CustomizationJobDecisionTypeEffectPreview CustomizationJobDecisionType = "effect_preview"
)

func (d CustomizationJobDecisionType) Valid() bool {
	switch d {
	case CustomizationJobDecisionTypeFinal, CustomizationJobDecisionTypeEffectPreview:
		return true
	default:
		return false
	}
}

type CustomizationJobStatus string

const (
	CustomizationJobStatusPendingCustomizationReview     CustomizationJobStatus = "pending_customization_review"
	CustomizationJobStatusPendingCustomizationProduction CustomizationJobStatus = "pending_customization_production"
	CustomizationJobStatusPendingEffectReview            CustomizationJobStatus = "pending_effect_review"
	CustomizationJobStatusPendingEffectRevision          CustomizationJobStatus = "pending_effect_revision"
	CustomizationJobStatusPendingProductionTransfer      CustomizationJobStatus = "pending_production_transfer"
	CustomizationJobStatusPendingWarehouseQC             CustomizationJobStatus = "pending_warehouse_qc"
	CustomizationJobStatusRejectedByWarehouse            CustomizationJobStatus = "rejected_by_warehouse"
	CustomizationJobStatusCompleted                      CustomizationJobStatus = "completed"
)

func (s CustomizationJobStatus) Valid() bool {
	switch s {
	case CustomizationJobStatusPendingCustomizationReview,
		CustomizationJobStatusPendingCustomizationProduction,
		CustomizationJobStatusPendingEffectReview,
		CustomizationJobStatusPendingEffectRevision,
		CustomizationJobStatusPendingProductionTransfer,
		CustomizationJobStatusPendingWarehouseQC,
		CustomizationJobStatusRejectedByWarehouse,
		CustomizationJobStatusCompleted:
		return true
	default:
		return false
	}
}

type CustomizationJob struct {
	ID                          int64                        `db:"id" json:"id"`
	TaskID                      int64                        `db:"task_id" json:"task_id"`
	OrderNo                     string                       `db:"order_no" json:"order_no"`
	SourceAssetID               *int64                       `db:"source_asset_id" json:"source_asset_id,omitempty"`
	CurrentAssetID              *int64                       `db:"current_asset_id" json:"current_asset_id,omitempty"`
	CustomizationLevelCode      string                       `db:"customization_level_code" json:"customization_level_code"`
	CustomizationLevelName      string                       `db:"customization_level_name" json:"customization_level_name"`
	ReviewReferenceUnitPrice    *float64                     `db:"review_reference_unit_price" json:"review_reference_unit_price,omitempty"`
	ReviewReferenceWeightFactor *float64                     `db:"review_reference_weight_factor" json:"review_reference_weight_factor,omitempty"`
	UnitPrice                   *float64                     `db:"unit_price" json:"unit_price,omitempty"`
	WeightFactor                *float64                     `db:"weight_factor" json:"weight_factor,omitempty"`
	Note                        string                       `db:"note" json:"note"`
	ReviewDecision              CustomizationReviewDecision  `db:"customization_review_decision" json:"customization_review_decision"`
	DecisionType                CustomizationJobDecisionType `db:"decision_type" json:"decision_type"`
	AssignedOperatorID          *int64                       `db:"assigned_operator_id" json:"assigned_operator_id,omitempty"`
	LastOperatorID              *int64                       `db:"last_operator_id" json:"last_operator_id,omitempty"`
	PricingWorkerType           EmploymentType               `db:"pricing_worker_type" json:"pricing_worker_type,omitempty"`
	Status                      CustomizationJobStatus       `db:"status" json:"status"`
	WarehouseRejectReason       string                       `db:"warehouse_reject_reason" json:"warehouse_reject_reason,omitempty"`
	WarehouseRejectCategory     string                       `db:"warehouse_reject_category" json:"warehouse_reject_category,omitempty"`
	CreatedAt                   time.Time                    `db:"created_at" json:"created_at"`
	UpdatedAt                   time.Time                    `db:"updated_at" json:"updated_at"`
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
