package domain

type ModuleEventType string

const (
	ModuleEventEntered                  ModuleEventType = "entered"
	ModuleEventClaimed                  ModuleEventType = "claimed"
	ModuleEventSubmitted                ModuleEventType = "submitted"
	ModuleEventApproved                 ModuleEventType = "approved"
	ModuleEventRejected                 ModuleEventType = "rejected"
	ModuleEventReopened                 ModuleEventType = "reopened"
	ModuleEventClosed                   ModuleEventType = "closed"
	ModuleEventReceived                 ModuleEventType = "received"
	ModuleEventCompleted                ModuleEventType = "completed"
	ModuleEventReassigned               ModuleEventType = "reassigned"
	ModuleEventPoolReassignedByAdmin    ModuleEventType = "pool_reassigned_by_admin"
	ModuleEventTaskCancelled            ModuleEventType = "task_cancelled"
	ModuleEventForciblyClosed           ModuleEventType = "forcibly_closed"
	ModuleEventReferenceFilesUpdated    ModuleEventType = "reference_files_updated"
	ModuleEventMigratedFromV09          ModuleEventType = "migrated_from_v0_9"
	ModuleEventBackfillPlaceholder      ModuleEventType = "backfill_placeholder"
	ModuleEventAssetBackfillUnknownType ModuleEventType = "asset_backfill_unknown_type"
)
