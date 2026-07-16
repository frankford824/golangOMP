package domain

import "time"

type PlanningSKUERPSyncMode string

const (
	PlanningSKUERPSyncNone  PlanningSKUERPSyncMode = "none"
	PlanningSKUERPSyncAsync PlanningSKUERPSyncMode = "async"
)

type PlanningSKUItemInput struct {
	ClientItemID    string  `json:"client_item_id"`
	DescriptionSpec string  `json:"description_spec"`
	Quantity        int64   `json:"quantity"`
	TargetPrice     *string `json:"target_price,omitempty"`
	Note            string  `json:"note,omitempty"`
	ReferenceURL    string  `json:"reference_url,omitempty"`
	ImageUploadRef  string  `json:"image_upload_ref,omitempty"`
	ERPProductIID   string  `json:"erp_product_i_id,omitempty"`
	ERPProductName  string  `json:"erp_product_name,omitempty"`
}

type CreatePlanningSKUTaskRequest struct {
	ClientCreateID string                 `json:"client_create_id"`
	ERPSyncMode    PlanningSKUERPSyncMode `json:"erp_sync_mode"`
	Items          []PlanningSKUItemInput `json:"planning_sku_items"`
}

type PlanningSKUSettings struct {
	TaskID             int64                  `json:"task_id"`
	ERPSyncMode        PlanningSKUERPSyncMode `json:"erp_sync_mode"`
	CodeRuleRevisionID int64                  `json:"code_rule_revision_id"`
	ClientCreateID     string                 `json:"client_create_id"`
	CreatedBy          int64                  `json:"created_by"`
	CreatedAt          time.Time              `json:"created_at"`
}

type PlanningSKUDetail struct {
	TaskSKUItemID     int64                `json:"task_sku_item_id"`
	CurrentRevisionID int64                `json:"current_revision_id"`
	LockVersion       int64                `json:"lock_version"`
	Revision          *PlanningSKURevision `json:"revision,omitempty"`
}

type PlanningSKURevision struct {
	ID                int64     `json:"id"`
	TaskSKUItemID     int64     `json:"task_sku_item_id"`
	VersionNo         int       `json:"version_no"`
	DescriptionSpec   string    `json:"description_spec"`
	Quantity          int64     `json:"quantity"`
	TargetPrice       *string   `json:"target_price,omitempty"`
	Currency          string    `json:"currency"`
	Note              string    `json:"note,omitempty"`
	ReferenceURL      string    `json:"reference_url,omitempty"`
	ERPProductIID     string    `json:"erp_product_i_id,omitempty"`
	ERPProductName    string    `json:"erp_product_name,omitempty"`
	ProductImageRefID string    `json:"product_image_ref_id,omitempty"`
	Reason            string    `json:"reason"`
	CreatedBy         int64     `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
}

type UpdatePlanningSKURequest struct {
	ExpectedVersion int64   `json:"expected_version"`
	Reason          string  `json:"reason"`
	DescriptionSpec string  `json:"description_spec"`
	Quantity        int64   `json:"quantity"`
	TargetPrice     *string `json:"target_price,omitempty"`
	Note            string  `json:"note,omitempty"`
	ReferenceURL    string  `json:"reference_url,omitempty"`
	ImageUploadRef  string  `json:"image_upload_ref,omitempty"`
	RemoveImage     bool    `json:"remove_image,omitempty"`
}

type PlanningSKUExportRequest struct {
	TaskIDs        []int64 `json:"task_ids,omitempty"`
	TaskSKUItemIDs []int64 `json:"task_sku_item_ids,omitempty"`
}

type PlanningSKUExcelParseError struct {
	Row    int    `json:"row"`
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

type PlanningSKUExcelParseResult struct {
	Items  []PlanningSKUItemInput       `json:"planning_sku_items"`
	Errors []PlanningSKUExcelParseError `json:"errors"`
	Valid  bool                         `json:"valid"`
}

type PlanningSKUResultItem struct {
	TaskSKUItemID int64                `json:"task_sku_item_id"`
	SequenceNo    int                  `json:"sequence_no"`
	SKUCode       string               `json:"sku_code"`
	Quantity      int64                `json:"quantity"`
	ERPStatus     FilingStatus         `json:"erp_status,omitempty"`
	Revision      *PlanningSKURevision `json:"revision,omitempty"`
}

type PlanningSKUCreateResult struct {
	TaskID           int64                   `json:"task_id"`
	TaskNo           string                  `json:"task_no"`
	TaskStatus       TaskStatus              `json:"task_status"`
	WorkflowRevision int64                   `json:"workflow_revision"`
	Items            []PlanningSKUResultItem `json:"items"`
}

type PlanningSKUUpdateLock struct {
	TaskID          int64                  `json:"task_id"`
	TaskSKUItemID   int64                  `json:"task_sku_item_id"`
	SKUCode         string                 `json:"sku_code"`
	LockVersion     int64                  `json:"lock_version"`
	ERPSyncMode     PlanningSKUERPSyncMode `json:"erp_sync_mode"`
	CurrentRevision PlanningSKURevision    `json:"current_revision"`
}

type PlanningSKUExportRow struct {
	TaskID          int64      `json:"task_id"`
	TaskNo          string     `json:"task_no"`
	SequenceNo      int        `json:"sequence_no"`
	TaskSKUItemID   int64      `json:"task_sku_item_id"`
	SKUCode         string     `json:"sku_code"`
	ImageRefID      string     `json:"image_ref_id,omitempty"`
	DescriptionSpec string     `json:"description_spec"`
	Quantity        int64      `json:"quantity"`
	TargetPrice     *string    `json:"target_price,omitempty"`
	Note            string     `json:"note,omitempty"`
	ReferenceURL    string     `json:"reference_url,omitempty"`
	ERPStatus       string     `json:"erp_status,omitempty"`
	CreatorName     string     `json:"creator_name"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}
