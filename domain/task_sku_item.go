package domain

import (
	"encoding/json"
	"time"
)

type TaskBatchMode string

const (
	TaskBatchModeSingle   TaskBatchMode = "single"
	TaskBatchModeMultiSKU TaskBatchMode = "multi_sku"
)

func (m TaskBatchMode) Valid() bool {
	switch m {
	case TaskBatchModeSingle, TaskBatchModeMultiSKU:
		return true
	default:
		return false
	}
}

type TaskSKUGenerationStatus string

const (
	TaskSKUGenerationStatusNotApplicable TaskSKUGenerationStatus = "not_applicable"
	TaskSKUGenerationStatusPending       TaskSKUGenerationStatus = "pending"
	TaskSKUGenerationStatusPartial       TaskSKUGenerationStatus = "partial"
	TaskSKUGenerationStatusCompleted     TaskSKUGenerationStatus = "completed"
	TaskSKUGenerationStatusFailed        TaskSKUGenerationStatus = "failed"
)

func (s TaskSKUGenerationStatus) Valid() bool {
	switch s {
	case TaskSKUGenerationStatusNotApplicable, TaskSKUGenerationStatusPending, TaskSKUGenerationStatusPartial, TaskSKUGenerationStatusCompleted, TaskSKUGenerationStatusFailed:
		return true
	default:
		return false
	}
}

type TaskSKUStatus string

const (
	TaskSKUStatusReserved     TaskSKUStatus = "reserved"
	TaskSKUStatusGenerated    TaskSKUStatus = "generated"
	TaskSKUStatusFiled        TaskSKUStatus = "filed"
	TaskSKUStatusFilingFailed TaskSKUStatus = "filing_failed"
)

func (s TaskSKUStatus) Valid() bool {
	switch s {
	case TaskSKUStatusReserved, TaskSKUStatusGenerated, TaskSKUStatusFiled, TaskSKUStatusFilingFailed:
		return true
	default:
		return false
	}
}

type TaskSKUItem struct {
	ID                  int64              `db:"id"                    json:"id"`
	TaskID              int64              `db:"task_id"               json:"task_id"`
	SequenceNo          int                `db:"sequence_no"           json:"sequence_no"`
	SKUCode             string             `db:"sku_code"              json:"sku_code"`
	SKUStatus           TaskSKUStatus      `db:"sku_status"            json:"sku_status"`
	ProductID           *int64             `db:"product_id"            json:"product_id,omitempty"`
	ERPProductID        *string            `db:"erp_product_id"        json:"erp_product_id,omitempty"`
	ProductNameSnapshot string             `db:"product_name_snapshot" json:"product_name_snapshot"`
	ProductShortName    string             `db:"product_short_name"    json:"product_short_name,omitempty"`
	ProductIID          string             `db:"-"                     json:"product_i_id,omitempty"`
	CategoryCode        string             `db:"category_code"         json:"category_code,omitempty"`
	MaterialMode        string             `db:"material_mode"         json:"material_mode,omitempty"`
	CostPriceMode       string             `db:"cost_price_mode"       json:"cost_price_mode,omitempty"`
	Quantity            *int64             `db:"quantity"              json:"quantity,omitempty"`
	BaseSalePrice       *float64           `db:"base_sale_price"       json:"base_sale_price,omitempty"`
	DesignRequirement   string             `db:"design_requirement"    json:"design_requirement,omitempty"`
	ChangeRequest       string             `db:"-"                     json:"change_request,omitempty"`
	VariantJSON         json.RawMessage    `db:"variant_json"          json:"variant_json,omitempty"`
	ReferenceFileRefs   []ReferenceFileRef `db:"-"                  json:"reference_file_refs"`
	DedupeKey           string             `db:"dedupe_key"            json:"dedupe_key,omitempty"`
	CreatedAt           time.Time          `db:"created_at"            json:"created_at"`
	UpdatedAt           time.Time          `db:"updated_at"            json:"updated_at"`
}

type ProcurementRecordItem struct {
	ID                  int64             `db:"id"                    json:"id"`
	ProcurementRecordID int64             `db:"procurement_record_id" json:"procurement_record_id"`
	TaskID              int64             `db:"task_id"               json:"task_id"`
	TaskSKUItemID       int64             `db:"task_sku_item_id"      json:"task_sku_item_id"`
	SKUCode             string            `db:"sku_code"              json:"sku_code"`
	Status              ProcurementStatus `db:"status"                json:"status"`
	Quantity            *int64            `db:"quantity"              json:"quantity,omitempty"`
	CostPrice           *float64          `db:"cost_price"            json:"cost_price,omitempty"`
	BaseSalePrice       *float64          `db:"base_sale_price"       json:"base_sale_price,omitempty"`
	CreatedAt           time.Time         `db:"created_at"            json:"created_at"`
	UpdatedAt           time.Time         `db:"updated_at"            json:"updated_at"`
}
