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
