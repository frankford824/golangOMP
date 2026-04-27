package permission

import "workflow/domain"

const (
	DenyModuleNotInstantiated      = domain.DenyModuleNotInstantiated
	DenyModuleOutOfScope           = domain.DenyModuleOutOfScope
	DenyModuleStateMismatch        = domain.DenyModuleStateMismatch
	DenyModuleActionRoleDenied     = domain.DenyModuleActionRoleDenied
	DenyModuleClaimConflict        = domain.DenyModuleClaimConflict
	DenyModuleBlueprintMissingTeam = domain.DenyModuleBlueprintMissingTeam
)

type Decision struct {
	OK       bool
	DenyCode string
	Message  string
}

func Allow() Decision { return Decision{OK: true} }

func Deny(code, message string) Decision {
	return Decision{DenyCode: code, Message: message}
}
