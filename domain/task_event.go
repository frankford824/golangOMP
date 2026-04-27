package domain

import (
	"encoding/json"
	"time"
)

// TaskEvent is a V7 task-scoped event log entry stored in task_event_logs.
//
// Design rationale for a separate table (not extending V6 event_logs):
//   - V6 event_logs is SKU-scoped: UNIQUE(sku_id, sequence); EventRepo is keyed on skuID.
//     Adding task_id would require schema + interface changes that break V6 invariants.
//   - V7 business events are task-centric; mixing them into SKU-scoped logs creates
//     ambiguity and complicates frontend sequence-gap recovery.
//   - task_event_logs has its own sequence counter (task_event_sequences) mirroring
//     the sku_sequences pattern, keeping the implementation consistent and isolated.
//   - If a unified log is needed in the future, a DB view or a fan-out worker can
//     merge both tables without changing either table's schema.
type TaskEvent struct {
	ID         string          `db:"id"          json:"id"` // UUID (event_id)
	TaskID     int64           `db:"task_id"     json:"task_id"`
	Sequence   int64           `db:"sequence"    json:"sequence"` // monotonically increasing per task
	EventType  string          `db:"event_type"  json:"event_type"`
	OperatorID *int64          `db:"operator_id" json:"operator_id,omitempty"`
	Payload    json.RawMessage `db:"payload"     json:"payload"` // raw JSON payload
	CreatedAt  time.Time       `db:"created_at"  json:"created_at"`
}
