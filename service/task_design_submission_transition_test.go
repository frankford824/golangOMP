package service

import (
	"testing"

	"workflow/domain"
)

func TestDesignAssetSourceModuleKeyForTask(t *testing.T) {
	regular := &domain.Task{CustomizationRequired: false}
	if got := designAssetSourceModuleKeyForTask(regular, domain.TaskAssetTypeReference); got != domain.ModuleKeyBasicInfo {
		t.Fatalf("reference source_module_key = %q, want %q", got, domain.ModuleKeyBasicInfo)
	}
	if got := designAssetSourceModuleKeyForTask(regular, domain.TaskAssetTypeDelivery); got != domain.ModuleKeyDesign {
		t.Fatalf("delivery source_module_key = %q, want %q", got, domain.ModuleKeyDesign)
	}
	audit := &domain.Task{CustomizationRequired: false, TaskStatus: domain.TaskStatusPendingAudit}
	if got := designAssetSourceModuleKeyForTask(audit, domain.TaskAssetTypeSource); got != domain.ModuleKeyAudit {
		t.Fatalf("audit source source_module_key = %q, want %q", got, domain.ModuleKeyAudit)
	}
	if got := designAssetSourceModuleKeyForTask(audit, domain.TaskAssetTypeDelivery); got != domain.ModuleKeyAudit {
		t.Fatalf("audit delivery source_module_key = %q, want %q", got, domain.ModuleKeyAudit)
	}
	custom := &domain.Task{CustomizationRequired: true}
	if got := designAssetSourceModuleKeyForTask(custom, domain.TaskAssetTypeSource); got != domain.ModuleKeyCustomization {
		t.Fatalf("customization source source_module_key = %q, want %q", got, domain.ModuleKeyCustomization)
	}
	retouch := &domain.Task{TaskType: domain.TaskTypeRetouchTask}
	if got := designAssetSourceModuleKeyForTask(retouch, domain.TaskAssetTypeSource); got != domain.ModuleKeyRetouch {
		t.Fatalf("retouch source source_module_key = %q, want %q", got, domain.ModuleKeyRetouch)
	}
}
