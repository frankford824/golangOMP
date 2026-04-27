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
				{Key: domain.ModuleKeyWarehouse, InitialState: domain.ModuleStatePending},
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

func strPtr(value string) *string { return &value }
