package domain

import "time"

type ExportJobDispatchStatus string

const (
	ExportJobDispatchStatusSubmitted   ExportJobDispatchStatus = "submitted"
	ExportJobDispatchStatusReceived    ExportJobDispatchStatus = "received"
	ExportJobDispatchStatusRejected    ExportJobDispatchStatus = "rejected"
	ExportJobDispatchStatusExpired     ExportJobDispatchStatus = "expired"
	ExportJobDispatchStatusNotExecuted ExportJobDispatchStatus = "not_executed"
)

func (s ExportJobDispatchStatus) Valid() bool {
	switch s {
	case ExportJobDispatchStatusSubmitted, ExportJobDispatchStatusReceived, ExportJobDispatchStatusRejected, ExportJobDispatchStatusExpired, ExportJobDispatchStatusNotExecuted:
		return true
	default:
		return false
	}
}

type ExportJobDispatch struct {
	DispatchID           string                    `json:"dispatch_id"`
	ExportJobID          int64                     `json:"export_job_id"`
	DispatchNo           int                       `json:"dispatch_no"`
	TriggerSource        string                    `json:"trigger_source"`
	ExecutionMode        ExportJobExecutionMode    `json:"execution_mode"`
	AdapterKey           ExportJobRunnerAdapterKey `json:"adapter_key"`
	Status               ExportJobDispatchStatus   `json:"status"`
	SubmittedAt          time.Time                 `json:"submitted_at"`
	ReceivedAt           *time.Time                `json:"received_at,omitempty"`
	FinishedAt           *time.Time                `json:"finished_at,omitempty"`
	ExpiresAt            *time.Time                `json:"expires_at,omitempty"`
	StatusReason         string                    `json:"status_reason,omitempty"`
	AdapterNote          string                    `json:"adapter_note,omitempty"`
	StartAdmissible      bool                      `json:"start_admissible"`
	StartAdmissionReason string                    `json:"start_admission_reason,omitempty"`
	CreatedAt            time.Time                 `json:"created_at"`
	UpdatedAt            time.Time                 `json:"updated_at"`
}

func ExportJobDispatchAdmission(jobStatus ExportJobStatus, latestDispatch *ExportJobDispatch) (bool, string) {
	if jobStatus != ExportJobStatusQueued {
		switch jobStatus {
		case ExportJobStatusRunning:
			return false, ExportJobAdmissionReasonRunningDispatchBlocked
		case ExportJobStatusReady:
			return false, ExportJobAdmissionReasonReadyDispatchBlocked
		case ExportJobStatusFailed:
			return false, ExportJobAdmissionReasonFailedDispatchBlocked
		case ExportJobStatusCancelled:
			return false, ExportJobAdmissionReasonCancelledDispatchBlocked
		default:
			return false, ExportJobAdmissionReasonUnknownDispatchBlocked
		}
	}
	if latestDispatch == nil {
		return true, ExportJobAdmissionReasonQueuedWithoutDispatch
	}
	switch latestDispatch.Status {
	case ExportJobDispatchStatusSubmitted:
		return false, ExportJobAdmissionReasonLatestDispatchSubmittedPendingResolution
	case ExportJobDispatchStatusReceived:
		return false, ExportJobAdmissionReasonLatestDispatchReceivedPendingStartOrResolution
	case ExportJobDispatchStatusRejected:
		return true, ExportJobAdmissionReasonLatestDispatchRejectedRedispatchAllowed
	case ExportJobDispatchStatusExpired:
		return true, ExportJobAdmissionReasonLatestDispatchExpiredRedispatchAllowed
	case ExportJobDispatchStatusNotExecuted:
		return true, ExportJobAdmissionReasonLatestDispatchNotExecutedRedispatchAllowed
	default:
		return false, ExportJobAdmissionReasonLatestDispatchUnknownStatus
	}
}

func ExportJobCanDispatch(jobStatus ExportJobStatus, latestDispatch *ExportJobDispatch) bool {
	allowed, _ := ExportJobDispatchAdmission(jobStatus, latestDispatch)
	return allowed
}

func ExportJobRedispatchAdmission(jobStatus ExportJobStatus, dispatchCount int64, latestDispatch *ExportJobDispatch) (bool, string) {
	if dispatchCount <= 0 {
		return false, ExportJobAdmissionReasonNoHistoricalDispatch
	}
	return ExportJobDispatchAdmission(jobStatus, latestDispatch)
}

func ExportJobCanRedispatch(jobStatus ExportJobStatus, dispatchCount int64, latestDispatch *ExportJobDispatch) bool {
	allowed, _ := ExportJobRedispatchAdmission(jobStatus, dispatchCount, latestDispatch)
	return allowed
}

func ExportJobDispatchStartAdmission(status ExportJobDispatchStatus) (bool, string) {
	switch status {
	case ExportJobDispatchStatusReceived:
		return true, ExportJobDispatchStartAdmissionReasonReceivedStartAdmitted
	case ExportJobDispatchStatusSubmitted:
		return false, ExportJobDispatchStartAdmissionReasonSubmittedPending
	case ExportJobDispatchStatusRejected:
		return false, ExportJobDispatchStartAdmissionReasonRejectedRequiresRedispatch
	case ExportJobDispatchStatusExpired:
		return false, ExportJobDispatchStartAdmissionReasonExpiredRequiresRedispatch
	case ExportJobDispatchStatusNotExecuted:
		return false, ExportJobDispatchStartAdmissionReasonNotExecutedRequiresRedispatch
	default:
		return false, ExportJobDispatchStartAdmissionReasonUnknown
	}
}

func HydrateExportJobDispatchDerived(dispatch *ExportJobDispatch) {
	if dispatch == nil {
		return
	}
	dispatch.StartAdmissible, dispatch.StartAdmissionReason = ExportJobDispatchStartAdmission(dispatch.Status)
}
