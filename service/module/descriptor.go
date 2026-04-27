package module

import "workflow/domain"

type Descriptor struct {
	Key            string
	Department     domain.Department
	InitialState   domain.ModuleState
	TerminalStates []domain.ModuleState
	Actions        []ActionSpec
}

func DescriptorFor(moduleKey string) (Descriptor, bool) {
	d, ok := descriptors()[moduleKey]
	return d, ok
}

func descriptors() map[string]Descriptor {
	return map[string]Descriptor{
		domain.ModuleKeyBasicInfo: {
			Key:          domain.ModuleKeyBasicInfo,
			Department:   domain.DepartmentOperations,
			InitialState: domain.ModuleStateActive,
			Actions: []ActionSpec{
				action(domain.ModuleActionUpdateBasicInfo, []domain.ModuleState{domain.ModuleStateActive}, RoleCreatorOrOpsLead),
				action(domain.ModuleActionUpdateDeadline, []domain.ModuleState{domain.ModuleStateActive}, RoleCreatorOrOpsLead),
				action(domain.ModuleActionUpdatePriority, []domain.ModuleState{domain.ModuleStateActive}, RoleCreatorOrOpsLead),
				action(domain.ModuleActionCancelTask, []domain.ModuleState{domain.ModuleStateActive}, RoleCreatorOrAdmin),
			},
		},
		domain.ModuleKeyDesign:        workModule(domain.ModuleKeyDesign, domain.DepartmentDesignRD),
		domain.ModuleKeyRetouch:       workModule(domain.ModuleKeyRetouch, domain.DepartmentDesignRD),
		domain.ModuleKeyCustomization: workModule(domain.ModuleKeyCustomization, domain.DepartmentCustomizationArt),
		domain.ModuleKeyAudit: {
			Key:            domain.ModuleKeyAudit,
			Department:     domain.DepartmentAudit,
			InitialState:   domain.ModuleStatePendingClaim,
			TerminalStates: []domain.ModuleState{domain.ModuleStateClosed, domain.ModuleStateForciblyClosed, domain.ModuleStateClosedByAdmin},
			Actions: []ActionSpec{
				action(domain.ModuleActionClaim, []domain.ModuleState{domain.ModuleStatePendingClaim}, RoleScopedWorker),
				action(domain.ModuleActionApprove, []domain.ModuleState{domain.ModuleStateInProgress}, RoleSelfOnly),
				action(domain.ModuleActionReject, []domain.ModuleState{domain.ModuleStateInProgress}, RoleSelfOnly),
				action(domain.ModuleActionReassign, []domain.ModuleState{domain.ModuleStatePendingClaim, domain.ModuleStateInProgress}, RoleLeadOrDepartmentAdmin),
				action(domain.ModuleActionPoolReassign, []domain.ModuleState{domain.ModuleStatePendingClaim}, RoleDepartmentAdmin),
				action(domain.ModuleActionUpdateReferenceFiles, []domain.ModuleState{domain.ModuleStateInProgress}, RoleSelfOrLead),
			},
		},
		domain.ModuleKeyWarehouse: {
			Key:            domain.ModuleKeyWarehouse,
			Department:     domain.DepartmentCloudWarehouse,
			InitialState:   domain.ModuleStatePending,
			TerminalStates: []domain.ModuleState{domain.ModuleStateCompleted, domain.ModuleStateClosed, domain.ModuleStateForciblyClosed, domain.ModuleStateClosedByAdmin},
			Actions: []ActionSpec{
				action(domain.ModuleActionClaim, []domain.ModuleState{domain.ModuleStatePendingClaim, domain.ModuleStatePending}, RoleScopedWorker),
				action("prepare", []domain.ModuleState{domain.ModuleStatePending, domain.ModuleStatePendingClaim}, RoleScopedWorker),
				action("receive", []domain.ModuleState{domain.ModuleStatePreparing, domain.ModuleStatePending}, RoleSelfOrLead),
				action("complete", []domain.ModuleState{domain.ModuleStateReceived, domain.ModuleStatePreparing}, RoleSelfOrLead),
				action(domain.ModuleActionReassign, []domain.ModuleState{domain.ModuleStatePendingClaim, domain.ModuleStateInProgress, domain.ModuleStatePending, domain.ModuleStatePreparing}, RoleLeadOrDepartmentAdmin),
				action(domain.ModuleActionPoolReassign, []domain.ModuleState{domain.ModuleStatePendingClaim, domain.ModuleStatePending}, RoleDepartmentAdmin),
			},
		},
		domain.ModuleKeyProcurement: {
			Key:            domain.ModuleKeyProcurement,
			Department:     domain.DepartmentProcurement,
			InitialState:   domain.ModuleStatePendingClaim,
			TerminalStates: []domain.ModuleState{domain.ModuleStateClosed, domain.ModuleStateForciblyClosed, domain.ModuleStateClosedByAdmin},
			Actions: []ActionSpec{
				action(domain.ModuleActionClaim, []domain.ModuleState{domain.ModuleStatePendingClaim}, RoleScopedWorker),
				action(domain.ModuleActionSubmit, []domain.ModuleState{domain.ModuleStateInProgress, domain.ModuleStateReview}, RoleSelfOnly),
				action(domain.ModuleActionReassign, []domain.ModuleState{domain.ModuleStatePendingClaim, domain.ModuleStateInProgress}, RoleLeadOrDepartmentAdmin),
				action(domain.ModuleActionPoolReassign, []domain.ModuleState{domain.ModuleStatePendingClaim}, RoleDepartmentAdmin),
			},
		},
	}
}

func workModule(key string, department domain.Department) Descriptor {
	return Descriptor{
		Key:            key,
		Department:     department,
		InitialState:   domain.ModuleStatePendingClaim,
		TerminalStates: []domain.ModuleState{domain.ModuleStateClosed, domain.ModuleStateForciblyClosed, domain.ModuleStateClosedByAdmin},
		Actions: []ActionSpec{
			action(domain.ModuleActionClaim, []domain.ModuleState{domain.ModuleStatePendingClaim}, RoleScopedWorker),
			action(domain.ModuleActionSubmit, []domain.ModuleState{domain.ModuleStateInProgress}, RoleSelfOnly),
			action(domain.ModuleActionReassign, []domain.ModuleState{domain.ModuleStatePendingClaim, domain.ModuleStateInProgress}, RoleLeadOrDepartmentAdmin),
			action(domain.ModuleActionPoolReassign, []domain.ModuleState{domain.ModuleStatePendingClaim}, RoleDepartmentAdmin),
			action(domain.ModuleActionAssetUploadSessionCreate, []domain.ModuleState{domain.ModuleStateInProgress}, RoleSelfOrLead),
			action(domain.ModuleActionUpdateReferenceFiles, []domain.ModuleState{domain.ModuleStateInProgress}, RoleSelfOrLead),
		},
	}
}
