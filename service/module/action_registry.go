package module

import "workflow/domain"

type RoleFilter string

const (
	RoleScopedWorker          RoleFilter = "scoped_worker"
	RoleSelfOnly              RoleFilter = "self_only"
	RoleSelfOrLead            RoleFilter = "self_or_lead"
	RoleLeadOrDepartmentAdmin RoleFilter = "lead_or_department_admin"
	RoleDepartmentAdmin       RoleFilter = "department_admin"
	RoleCreatorOrOpsLead      RoleFilter = "creator_or_ops_lead"
	RoleCreatorOrAdmin        RoleFilter = "creator_or_admin"
)

type ActionSpec struct {
	Action        string
	AllowedStates []domain.ModuleState
	RoleFilter    RoleFilter
}

func action(name string, states []domain.ModuleState, filter RoleFilter) ActionSpec {
	return ActionSpec{Action: name, AllowedStates: states, RoleFilter: filter}
}

func ActionSpecFor(moduleKey, actionName string) (ActionSpec, bool) {
	desc, ok := DescriptorFor(moduleKey)
	if !ok {
		return ActionSpec{}, false
	}
	for _, spec := range desc.Actions {
		if spec.Action == actionName {
			return spec, true
		}
	}
	return ActionSpec{}, false
}

func AllActions(moduleKey string) []ActionSpec {
	desc, ok := DescriptorFor(moduleKey)
	if !ok {
		return nil
	}
	out := make([]ActionSpec, len(desc.Actions))
	copy(out, desc.Actions)
	return out
}
