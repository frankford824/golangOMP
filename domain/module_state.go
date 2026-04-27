package domain

type ModuleState string

const (
	ModuleStateActive         ModuleState = "active"
	ModuleStatePendingClaim   ModuleState = "pending_claim"
	ModuleStateInProgress     ModuleState = "in_progress"
	ModuleStateSubmitted      ModuleState = "submitted"
	ModuleStateApproved       ModuleState = "approved"
	ModuleStateRejected       ModuleState = "rejected"
	ModuleStateClosed         ModuleState = "closed"
	ModuleStateForciblyClosed ModuleState = "forcibly_closed"
	ModuleStateClosedByAdmin  ModuleState = "closed_by_admin"
	ModuleStatePending        ModuleState = "pending"
	ModuleStatePreparing      ModuleState = "preparing"
	ModuleStateReceived       ModuleState = "received"
	ModuleStateCompleted      ModuleState = "completed"
	ModuleStateReview         ModuleState = "review"
)

func (s ModuleState) Terminal() bool {
	switch s {
	case ModuleStateClosed, ModuleStateForciblyClosed, ModuleStateClosedByAdmin, ModuleStateCompleted:
		return true
	default:
		return false
	}
}
