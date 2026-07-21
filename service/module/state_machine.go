package module

import "workflow/domain"

func NextState(moduleKey string, current domain.ModuleState, actionName string) (domain.ModuleState, string, bool) {
	if actionName != domain.ModuleActionSubmit || (moduleKey != domain.ModuleKeyCustomization && moduleKey != domain.ModuleKeyRetouch) {
		return "", domain.DenyModuleActionRoleDenied, false
	}
	if current != domain.ModuleStateInProgress {
		return "", domain.DenyModuleStateMismatch, false
	}
	return domain.ModuleStateSubmitted, "", true
}
