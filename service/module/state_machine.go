package module

import "workflow/domain"

func NextState(moduleKey string, current domain.ModuleState, actionName string) (domain.ModuleState, string, bool) {
	spec, ok := ActionSpecFor(moduleKey, actionName)
	if !ok {
		return "", domain.DenyModuleActionRoleDenied, false
	}
	if !stateAllowed(current, spec.AllowedStates) {
		return "", domain.DenyModuleStateMismatch, false
	}
	switch actionName {
	case domain.ModuleActionClaim:
		return domain.ModuleStateInProgress, "", true
	case domain.ModuleActionSubmit:
		return domain.ModuleStateSubmitted, "", true
	case domain.ModuleActionApprove:
		return domain.ModuleStateClosed, "", true
	case domain.ModuleActionReject:
		return domain.ModuleStateRejected, "", true
	case "prepare":
		return domain.ModuleStatePreparing, "", true
	case "receive":
		return domain.ModuleStateReceived, "", true
	case "complete":
		return domain.ModuleStateCompleted, "", true
	default:
		return current, "", true
	}
}

func stateAllowed(current domain.ModuleState, states []domain.ModuleState) bool {
	for _, state := range states {
		if state == current {
			return true
		}
	}
	return false
}
