package domain

import "time"

type ExportJobAttemptStatus string

const (
	ExportJobAttemptStatusRunning   ExportJobAttemptStatus = "running"
	ExportJobAttemptStatusSucceeded ExportJobAttemptStatus = "succeeded"
	ExportJobAttemptStatusFailed    ExportJobAttemptStatus = "failed"
	ExportJobAttemptStatusCancelled ExportJobAttemptStatus = "cancelled"
)

func (s ExportJobAttemptStatus) Valid() bool {
	switch s {
	case ExportJobAttemptStatusRunning, ExportJobAttemptStatusSucceeded, ExportJobAttemptStatusFailed, ExportJobAttemptStatusCancelled:
		return true
	default:
		return false
	}
}

type ExportJobRunnerAdapterKey string

const (
	ExportJobRunnerAdapterKeyManualPlaceholder ExportJobRunnerAdapterKey = "manual_placeholder_adapter"
)

type ExportJobAttempt struct {
	AttemptID                  string                    `json:"attempt_id"`
	ExportJobID                int64                     `json:"export_job_id"`
	DispatchID                 string                    `json:"dispatch_id,omitempty"`
	AttemptNo                  int                       `json:"attempt_no"`
	TriggerSource              string                    `json:"trigger_source"`
	ExecutionMode              ExportJobExecutionMode    `json:"execution_mode"`
	AdapterKey                 ExportJobRunnerAdapterKey `json:"adapter_key"`
	Status                     ExportJobAttemptStatus    `json:"status"`
	StartedAt                  time.Time                 `json:"started_at"`
	FinishedAt                 *time.Time                `json:"finished_at,omitempty"`
	ErrorMessage               string                    `json:"error_message,omitempty"`
	AdapterNote                string                    `json:"adapter_note,omitempty"`
	BlocksNewAttempt           bool                      `json:"blocks_new_attempt"`
	NextAttemptAdmissionReason string                    `json:"next_attempt_admission_reason,omitempty"`
	CreatedAt                  time.Time                 `json:"created_at"`
	UpdatedAt                  time.Time                 `json:"updated_at"`
}

func ExportJobAttemptNextAdmission(status ExportJobAttemptStatus) (bool, string) {
	switch status {
	case ExportJobAttemptStatusRunning:
		return false, "attempt_running_blocks_new_attempt"
	case ExportJobAttemptStatusFailed:
		return true, "attempt_failed_requires_job_requeue"
	case ExportJobAttemptStatusCancelled:
		return true, "attempt_cancelled_requires_job_requeue"
	case ExportJobAttemptStatusSucceeded:
		return false, "attempt_succeeded_no_new_attempt_expected"
	default:
		return false, "attempt_status_unknown"
	}
}

func HydrateExportJobAttemptDerived(attempt *ExportJobAttempt) {
	if attempt == nil {
		return
	}
	nextAllowed, reason := ExportJobAttemptNextAdmission(attempt.Status)
	attempt.BlocksNewAttempt = !nextAllowed
	attempt.NextAttemptAdmissionReason = reason
}
