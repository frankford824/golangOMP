package blueprint

import (
	"fmt"

	"workflow/domain"
)

type ModuleSpec struct {
	Key          string
	InitialState domain.ModuleState
	PoolTeamCode *string
}

type Blueprint struct {
	TaskType domain.TaskType
	Key      string
	Modules  []ModuleSpec
}

type Registry struct {
	byTaskType map[domain.TaskType]Blueprint
}

func NewRegistry() *Registry {
	return &Registry{byTaskType: map[domain.TaskType]Blueprint{
		domain.TaskTypeOriginalProductDevelopment: productBlueprint(domain.TaskTypeOriginalProductDevelopment),
		domain.TaskTypeNewProductDevelopment:      productBlueprint(domain.TaskTypeNewProductDevelopment),
		domain.TaskTypePurchaseTask: {
			TaskType: domain.TaskTypePurchaseTask,
			Key:      "purchase_task_v1",
			Modules: []ModuleSpec{
				basicInfo(),
				{Key: domain.ModuleKeyProcurement, InitialState: domain.ModuleStatePendingClaim, PoolTeamCode: strPtr(domain.TeamProcurementMain)},
				{Key: domain.ModuleKeyWarehouse, InitialState: domain.ModuleStatePending},
			},
		},
		domain.TaskTypeRetouchTask: {
			TaskType: domain.TaskTypeRetouchTask,
			Key:      "retouch_task_v1",
			Modules: []ModuleSpec{
				basicInfo(),
				{Key: domain.ModuleKeyRetouch, InitialState: domain.ModuleStatePendingClaim, PoolTeamCode: strPtr(domain.TeamDesignRetouch)},
			},
		},
		domain.TaskTypeCustomerCustomization: customizationBlueprint(domain.TaskTypeCustomerCustomization),
		domain.TaskTypeRegularCustomization:  customizationBlueprint(domain.TaskTypeRegularCustomization),
	}}
}

func (r *Registry) Get(taskType domain.TaskType) (Blueprint, bool) {
	if r == nil {
		r = NewRegistry()
	}
	bp, ok := r.byTaskType[taskType]
	return bp, ok
}

func (r *Registry) MustGet(taskType domain.TaskType) (Blueprint, error) {
	bp, ok := r.Get(taskType)
	if !ok {
		return Blueprint{}, fmt.Errorf("blueprint missing for task_type %q", taskType)
	}
	return bp, nil
}

func productBlueprint(taskType domain.TaskType) Blueprint {
	return Blueprint{
		TaskType: taskType,
		Key:      string(taskType) + "_v1",
		Modules: []ModuleSpec{
			basicInfo(),
			{Key: domain.ModuleKeyDesign, InitialState: domain.ModuleStatePendingClaim, PoolTeamCode: strPtr(domain.TeamDesignStandard)},
			{Key: domain.ModuleKeyAudit, InitialState: domain.ModuleStatePendingClaim, PoolTeamCode: strPtr(domain.TeamAuditStandard)},
			{Key: domain.ModuleKeyWarehouse, InitialState: domain.ModuleStatePending},
		},
	}
}

func customizationBlueprint(taskType domain.TaskType) Blueprint {
	return Blueprint{
		TaskType: taskType,
		Key:      string(taskType) + "_v1",
		Modules: []ModuleSpec{
			basicInfo(),
			{Key: domain.ModuleKeyCustomization, InitialState: domain.ModuleStatePendingClaim, PoolTeamCode: strPtr(domain.TeamCustomizationArt)},
			{Key: domain.ModuleKeyAudit, InitialState: domain.ModuleStatePendingClaim, PoolTeamCode: strPtr(domain.TeamAuditCustomization)},
			{Key: domain.ModuleKeyWarehouse, InitialState: domain.ModuleStatePending},
		},
	}
}

func basicInfo() ModuleSpec {
	return ModuleSpec{Key: domain.ModuleKeyBasicInfo, InitialState: domain.ModuleStateActive}
}

func customizationModuleSpec() ModuleSpec {
	return ModuleSpec{
		Key:          domain.ModuleKeyCustomization,
		InitialState: domain.ModuleStatePendingClaim,
		PoolTeamCode: strPtr(domain.TeamCustomizationArt),
	}
}

// RequiresHybridCustomizationModule is true when a product-development blueprint must
// also instantiate the customization module (e.g. new_product_development + customization lane).
func RequiresHybridCustomizationModule(task *domain.Task) bool {
	if task == nil {
		return false
	}
	switch task.TaskType {
	case domain.TaskTypeOriginalProductDevelopment, domain.TaskTypeNewProductDevelopment:
	default:
		return false
	}
	return isCustomizationTask(task)
}

// ModulesForTask returns blueprint modules, augmenting product-development blueprints with
// customization when RequiresHybridCustomizationModule is true.
func ModulesForTask(task *domain.Task, bp Blueprint) []ModuleSpec {
	if !RequiresHybridCustomizationModule(task) {
		return bp.Modules
	}
	return injectCustomizationModuleAfterDesign(bp.Modules)
}

func injectCustomizationModuleAfterDesign(modules []ModuleSpec) []ModuleSpec {
	for _, spec := range modules {
		if spec.Key == domain.ModuleKeyCustomization {
			return modules
		}
	}
	out := make([]ModuleSpec, 0, len(modules)+1)
	inserted := false
	for _, spec := range modules {
		out = append(out, spec)
		if !inserted && spec.Key == domain.ModuleKeyDesign {
			out = append(out, customizationModuleSpec())
			inserted = true
		}
	}
	if !inserted {
		out = append(out, customizationModuleSpec())
	}
	return out
}

func strPtr(value string) *string { return &value }
