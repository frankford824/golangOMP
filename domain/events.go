package domain

// Event type constants for event_logs — single source of truth.
// The EventDispatcher fans these out to WebSocket clients (spec §4.1).
// Any new event type MUST be added here.
const (
	// SKU lifecycle
	EventSKUCreated       = "sku.created"
	EventSKUStatusChanged = "sku.status_changed"

	// Asset version lifecycle
	EventVersionCreated = "version.created"
	EventVersionStable  = "version.stable"
	EventVersionMissing = "version.missing"

	// Audit
	EventAuditSubmitted = "audit.submitted"

	// Distribution job lifecycle
	EventJobCreated         = "job.created"
	EventJobRunning         = "job.running"
	EventJobDone            = "job.done"
	EventJobFailed          = "job.failed"
	EventJobStale           = "job.stale"
	EventJobRetrying        = "job.retrying"
	EventJobExceededRetries = "job.exceeded_retries"
	EventJobCancelled       = "job.cancelled"

	// Verification
	EventVerifyStarted = "verify.started"
	EventVerifyPassed  = "verify.passed"
	EventVerifyFailed  = "verify.failed"

	// Incident lifecycle
	EventIncidentCreated  = "incident.created"
	EventIncidentAssigned = "incident.assigned"
	EventIncidentResolved = "incident.resolved"
	EventIncidentClosed   = "incident.closed"
)
