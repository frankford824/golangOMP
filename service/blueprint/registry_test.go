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
		if len(bp.Modules) < 3 {
			t.Fatalf("blueprint %s modules = %d, want at least 3", taskType, len(bp.Modules))
		}
		if bp.Modules[0].Key != domain.ModuleKeyBasicInfo {
			t.Fatalf("blueprint %s first module = %s", taskType, bp.Modules[0].Key)
		}
	}
}
