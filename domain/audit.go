package domain

import "time"

// V7 task-scoped event type constants for task_event_logs.
// Naming: "task.<module>.<action>" to avoid collision with V6 event types.
const (
	TaskEventCreated                     = "task.created"
	TaskEventBatchItemsCreated           = "task.batch_items_created"
	TaskEventStatusChanged               = "task.status.changed"
	TaskEventAuditClaimed                = "task.audit.claimed"
	TaskEventAuditApproved               = "task.audit.approved"
	TaskEventAuditRejected               = "task.audit.rejected"
	TaskEventAuditTransferred            = "task.audit.transferred"
	TaskEventAuditHandedOver             = "task.audit.handed_over"
	TaskEventAuditTakenOver              = "task.audit.taken_over"
	TaskEventAssigned                    = "task.assigned"
	TaskEventReassigned                  = "task.reassigned"
	TaskEventBusinessInfoUpdated         = "task.business_info.updated"
	TaskEventDesignSubmitted             = "task.design.submitted"
	TaskEventAssetMockUploaded           = "task.asset.mock_uploaded"
	TaskEventAssetUploadSessionCreated   = "task.asset.upload_session.created"
	TaskEventAssetVersionCreated         = "task.asset.version.created"
	TaskEventAssetUploadSessionCompleted = "task.asset.upload_session.completed"
	TaskEventAssetUploadSessionCancelled = "task.asset.upload_session.cancelled"
	TaskEventOutsourceCreated            = "task.outsource.created"
	TaskEventOutsourceReturned           = "task.outsource.returned"
	TaskEventOutsourceReviewed           = "task.outsource.reviewed"
	TaskEventProcurementUpdated          = "task.procurement.updated"
	TaskEventProcurementAdvanced         = "task.procurement.advanced"
	TaskEventFilingTriggered             = "task.filing.triggered"
	TaskEventWarehousePrepared           = "task.warehouse.prepared"
	TaskEventWarehouseReceived           = "task.warehouse.received"
	TaskEventWarehouseRejected           = "task.warehouse.rejected"
	TaskEventWarehouseCompleted          = "task.warehouse.completed"
	TaskEventClosed                      = "task.closed"
	TaskEventReminded                    = "task.reminded"
	TaskEventBatchAssigned               = "task.batch_assigned"
)

// AuditRecord records one audit action against a Task (V7 §11).
// Distinct from V6 AuditAction, which is asset-version scoped.
// A task can have multiple AuditRecords (one per action).
type AuditRecord struct {
	ID             int64            `db:"id"              json:"id"`
	TaskID         int64            `db:"task_id"         json:"task_id"`
	Stage          AuditRecordStage `db:"stage"           json:"stage"`
	Action         AuditActionType  `db:"action"          json:"action"`
	AuditorID      int64            `db:"auditor_id"      json:"auditor_id"`
	IssueTypesJSON string           `db:"issue_types_json" json:"issue_types_json"` // JSON array of AuditIssueCategory
	Comment        string           `db:"comment"         json:"comment"`
	AffectsLaunch  bool             `db:"affects_launch"  json:"affects_launch"`
	NeedOutsource  bool             `db:"need_outsource"  json:"need_outsource"`
	CreatedAt      time.Time        `db:"created_at"      json:"created_at"`
}

// AuditHandover records an audit shift-handover event (V7 §6.3).
// A handover moves audit responsibility from one auditor to another.
// The task status does NOT change during handover (spec V7 §8 constraint).
type AuditHandover struct {
	ID               int64          `db:"id"                json:"id"`
	HandoverNo       string         `db:"handover_no"       json:"handover_no"`
	TaskID           int64          `db:"task_id"           json:"task_id"`
	FromAuditorID    int64          `db:"from_auditor_id"   json:"from_auditor_id"`
	ToAuditorID      int64          `db:"to_auditor_id"     json:"to_auditor_id"`
	Reason           string         `db:"reason"            json:"reason"`
	CurrentJudgement string         `db:"current_judgement" json:"current_judgement"`
	RiskRemark       string         `db:"risk_remark"       json:"risk_remark"`
	Status           HandoverStatus `db:"status"            json:"status"`
	CreatedAt        time.Time      `db:"created_at"        json:"created_at"`
	UpdatedAt        time.Time      `db:"updated_at"        json:"updated_at"`
}
