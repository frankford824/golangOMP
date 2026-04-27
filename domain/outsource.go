package domain

import "time"

// OutsourceOrder represents a customisation/outsource sub-process (V7 §6.2).
// One task can have at most one active OutsourceOrder.
type OutsourceOrder struct {
	ID                  int64           `db:"id"                   json:"id"`
	OutsourceNo         string          `db:"outsource_no"         json:"outsource_no"`
	TaskID              int64           `db:"task_id"              json:"task_id"`
	VendorName          string          `db:"vendor_name"          json:"vendor_name"`
	OutsourceType       string          `db:"outsource_type"       json:"outsource_type"`
	DeliveryRequirement string          `db:"delivery_requirement" json:"delivery_requirement"`
	SettlementNote      string          `db:"settlement_note"      json:"settlement_note"`
	Status              OutsourceStatus `db:"status"               json:"status"`
	ReturnedAt          *time.Time      `db:"returned_at"          json:"returned_at,omitempty"`
	CreatedAt           time.Time       `db:"created_at"           json:"created_at"`
	UpdatedAt           time.Time       `db:"updated_at"           json:"updated_at"`
}
