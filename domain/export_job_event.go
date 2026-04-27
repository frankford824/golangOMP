package domain

import (
	"encoding/json"
	"time"
)

const (
	ExportJobEventCreated             = "export_job.created"
	ExportJobEventDispatchSubmitted   = "export_job.dispatch_submitted"
	ExportJobEventDispatchReceived    = "export_job.dispatch_received"
	ExportJobEventDispatchRejected    = "export_job.dispatch_rejected"
	ExportJobEventDispatchExpired     = "export_job.dispatch_expired"
	ExportJobEventDispatchNotExecuted = "export_job.dispatch_not_executed"
	ExportJobEventRunnerInitiated     = "export_job.runner_initiated"
	ExportJobEventStarted             = "export_job.started"
	ExportJobEventAttemptSucceeded    = "export_job.attempt_succeeded"
	ExportJobEventAttemptFailed       = "export_job.attempt_failed"
	ExportJobEventAttemptCancelled    = "export_job.attempt_cancelled"
	ExportJobEventAdvancedToQueued    = "export_job.advanced_to_queued"
	ExportJobEventAdvancedToRunning   = "export_job.advanced_to_running"
	ExportJobEventAdvancedToReady     = "export_job.advanced_to_ready"
	ExportJobEventAdvancedToFailed    = "export_job.advanced_to_failed"
	ExportJobEventAdvancedToCancelled = "export_job.advanced_to_cancelled"
	ExportJobEventResultRefUpdated    = "export_job.result_ref_updated"
	ExportJobEventDownloadClaimed     = "export_job.download_claimed"
	ExportJobEventDownloadRead        = "export_job.download_read"
	ExportJobEventDownloadExpired     = "export_job.download_expired"
	ExportJobEventDownloadRefreshed   = "export_job.download_refreshed"
)

func IsExportJobRunnerEventType(eventType string) bool {
	switch eventType {
	case ExportJobEventRunnerInitiated, ExportJobEventStarted, ExportJobEventAttemptSucceeded, ExportJobEventAttemptFailed, ExportJobEventAttemptCancelled:
		return true
	default:
		return false
	}
}

type ExportJobEvent struct {
	EventID     string           `db:"event_id"      json:"event_id"`
	ExportJobID int64            `db:"export_job_id" json:"export_job_id"`
	Sequence    int64            `db:"sequence"      json:"sequence"`
	EventType   string           `db:"event_type"    json:"event_type"`
	FromStatus  *ExportJobStatus `db:"from_status"   json:"from_status,omitempty"`
	ToStatus    *ExportJobStatus `db:"to_status"     json:"to_status,omitempty"`
	ActorID     int64            `db:"actor_id"      json:"actor_id"`
	ActorType   string           `db:"actor_type"    json:"actor_type"`
	Note        string           `db:"note"          json:"note"`
	Payload     json.RawMessage  `db:"payload"       json:"payload"`
	CreatedAt   time.Time        `db:"created_at"    json:"created_at"`
}

type ExportJobEventSummary struct {
	EventID     string           `json:"event_id"`
	ExportJobID int64            `json:"export_job_id"`
	Sequence    int64            `json:"sequence"`
	EventType   string           `json:"event_type"`
	FromStatus  *ExportJobStatus `json:"from_status,omitempty"`
	ToStatus    *ExportJobStatus `json:"to_status,omitempty"`
	ActorID     int64            `json:"actor_id"`
	ActorType   string           `json:"actor_type"`
	Note        string           `json:"note"`
	CreatedAt   time.Time        `json:"created_at"`
}

func SummarizeExportJobEvent(event *ExportJobEvent) *ExportJobEventSummary {
	if event == nil {
		return nil
	}
	return &ExportJobEventSummary{
		EventID:     event.EventID,
		ExportJobID: event.ExportJobID,
		Sequence:    event.Sequence,
		EventType:   event.EventType,
		FromStatus:  cloneExportJobStatusPtr(event.FromStatus),
		ToStatus:    cloneExportJobStatusPtr(event.ToStatus),
		ActorID:     event.ActorID,
		ActorType:   event.ActorType,
		Note:        event.Note,
		CreatedAt:   event.CreatedAt,
	}
}

func cloneExportJobStatusPtr(value *ExportJobStatus) *ExportJobStatus {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
