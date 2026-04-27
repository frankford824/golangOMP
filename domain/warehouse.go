package domain

import "time"

// WarehouseReceipt records warehouse receive/reject/complete actions for a task.
// One task owns at most one warehouse receipt record in Step 03.
type WarehouseReceipt struct {
	ID                    int64                  `db:"id"            json:"id"`
	TaskID                int64                  `db:"task_id"       json:"task_id"`
	ReceiptNo             string                 `db:"receipt_no"    json:"receipt_no"`
	WorkflowLane          WorkflowLane           `json:"workflow_lane"`
	SourceDepartment      string                 `json:"source_department,omitempty"`
	TaskType              string                 `json:"task_type,omitempty"`
	Status                WarehouseReceiptStatus `db:"status"        json:"status"`
	ReceiverID            *int64                 `db:"receiver_id"   json:"receiver_id,omitempty"`
	ReceivedAt            *time.Time             `db:"received_at"   json:"received_at,omitempty"`
	CompletedAt           *time.Time             `db:"completed_at"  json:"completed_at,omitempty"`
	RejectReason          string                 `db:"reject_reason" json:"reject_reason"`
	Remark                string                 `db:"remark"        json:"remark"`
	CreatedAt             time.Time              `db:"created_at"    json:"created_at"`
	UpdatedAt             time.Time              `db:"updated_at"    json:"updated_at"`
	WarehouseReadyVersion *DesignAssetVersion    `json:"warehouse_ready_version,omitempty"`
}
