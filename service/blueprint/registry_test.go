package blueprint

import (
	"testing"

	"workflow/domain"
)

func TestRegistry_CoversSixTaskTypes(t *testing.T) {
	reg := NewRegistry()
	for _, taskType := range domain.V1TaskTypes() {
		bp, ok := reg.Get(taskType)
		if !ok {
			t.Fatalf("missing blueprint for %s", taskType)
		}
		wantMinModules := 3
		if taskType == domain.TaskTypeRetouchTask {
			wantMinModules = 2
		}
		if len(bp.Modules) < wantMinModules {
			t.Fatalf("blueprint %s modules = %d, want at least %d", taskType, len(bp.Modules), wantMinModules)
		}
		if bp.Modules[0].Key != domain.ModuleKeyBasicInfo {
			t.Fatalf("blueprint %s first module = %s", taskType, bp.Modules[0].Key)
		}
	}
}

func TestModulesForTask_HybridNewProductDevelopmentAddsCustomization(t *testing.T) {
	reg := NewRegistry()
	bp, ok := reg.Get(domain.TaskTypeNewProductDevelopment)
	if !ok {
		t.Fatal("missing new_product_development blueprint")
	}
	task := &domain.Task{
		TaskType:              domain.TaskTypeNewProductDevelopment,
		CustomizationRequired: true,
		BusinessLane:          domain.TaskBusinessLaneCustomization,
	}
	modules := ModulesForTask(task, bp)
	got := make([]string, 0, len(modules))
	for _, spec := range modules {
		got = append(got, spec.Key)
	}
	want := []string{
		domain.ModuleKeyBasicInfo,
		domain.ModuleKeyDesign,
		domain.ModuleKeyCustomization,
		domain.ModuleKeyAudit,
		domain.ModuleKeyWarehouse,
	}
	if len(got) != len(want) {
		t.Fatalf("hybrid modules = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("hybrid modules = %v, want %v", got, want)
		}
	}
	var customization *ModuleSpec
	for i := range modules {
		if modules[i].Key == domain.ModuleKeyCustomization {
			customization = &modules[i]
			break
		}
	}
	if customization == nil {
		t.Fatal("missing customization module spec")
	}
	if customization.InitialState != domain.ModuleStatePendingClaim {
		t.Fatalf("customization initial state = %s, want %s", customization.InitialState, domain.ModuleStatePendingClaim)
	}
	if customization.PoolTeamCode == nil || *customization.PoolTeamCode != domain.TeamCustomizationArt {
		t.Fatalf("customization pool = %v, want %q", customization.PoolTeamCode, domain.TeamCustomizationArt)
	}
}

func TestModulesForTask_RegularNewProductDevelopmentUnchanged(t *testing.T) {
	reg := NewRegistry()
	bp, ok := reg.Get(domain.TaskTypeNewProductDevelopment)
	if !ok {
		t.Fatal("missing new_product_development blueprint")
	}
	task := &domain.Task{
		TaskType:              domain.TaskTypeNewProductDevelopment,
		CustomizationRequired: false,
		BusinessLane:          domain.TaskBusinessLaneNormal,
	}
	modules := ModulesForTask(task, bp)
	if len(modules) != len(bp.Modules) {
		t.Fatalf("regular modules len = %d, want %d", len(modules), len(bp.Modules))
	}
	for i := range modules {
		if modules[i].Key != bp.Modules[i].Key {
			t.Fatalf("module[%d] key = %s, want %s", i, modules[i].Key, bp.Modules[i].Key)
		}
	}
}

func TestRegistry_RetouchTaskIsDesignOnly(t *testing.T) {
	bp, ok := NewRegistry().Get(domain.TaskTypeRetouchTask)
	if !ok {
		t.Fatal("missing retouch blueprint")
	}
	got := make([]string, 0, len(bp.Modules))
	for _, module := range bp.Modules {
		got = append(got, module.Key)
	}
	want := []string{domain.ModuleKeyBasicInfo, domain.ModuleKeyRetouch}
	if len(got) != len(want) {
		t.Fatalf("retouch modules = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("retouch modules = %v, want %v", got, want)
		}
	}
}
