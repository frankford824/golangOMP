package service

import (
	"context"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

type designSubmissionWorkflowRecorder struct {
	calls []struct {
		moduleKey string
		action    string
	}
}

func (r *designSubmissionWorkflowRecorder) ApplyAfterAction(_ context.Context, _ repo.Tx, _ *domain.Task, moduleKey, action string, _ *int64, _ int64) error {
	r.calls = append(r.calls, struct {
		moduleKey string
		action    string
	}{moduleKey: moduleKey, action: action})
	return nil
}

func TestApplyDesignSubmissionWorkflow(t *testing.T) {
	ctx := context.Background()
	task := &domain.Task{ID: 1}
	actorID := int64(99)

	t.Run("regular PendingAuditA triggers design submit", func(t *testing.T) {
		rec := &designSubmissionWorkflowRecorder{}
		err := applyDesignSubmissionWorkflow(ctx, nil, rec, task, designSubmissionTransition{
			TaskStatus: domain.TaskStatusPendingAuditA,
			ModuleKey:  domain.ModuleKeyDesign,
		}, actorID)
		if err != nil {
			t.Fatalf("applyDesignSubmissionWorkflow() err = %v", err)
		}
		if len(rec.calls) != 1 || rec.calls[0].moduleKey != domain.ModuleKeyDesign || rec.calls[0].action != domain.ModuleActionSubmit {
			t.Fatalf("calls = %+v, want design.submit once", rec.calls)
		}
	})

	t.Run("customization PendingCustomizationReview triggers customization submit", func(t *testing.T) {
		rec := &designSubmissionWorkflowRecorder{}
		err := applyDesignSubmissionWorkflow(ctx, nil, rec, task, designSubmissionTransition{
			TaskStatus: domain.TaskStatusPendingCustomizationReview,
			ModuleKey:  domain.ModuleKeyCustomization,
		}, actorID)
		if err != nil {
			t.Fatalf("applyDesignSubmissionWorkflow() err = %v", err)
		}
		if len(rec.calls) != 1 || rec.calls[0].moduleKey != domain.ModuleKeyCustomization || rec.calls[0].action != domain.ModuleActionSubmit {
			t.Fatalf("calls = %+v, want customization.submit once", rec.calls)
		}
	})

	t.Run("retouch completed skips workflow", func(t *testing.T) {
		rec := &designSubmissionWorkflowRecorder{}
		err := applyDesignSubmissionWorkflow(ctx, nil, rec, task, designSubmissionTransition{
			TaskStatus: domain.TaskStatusCompleted,
			ModuleKey:  domain.ModuleKeyRetouch,
		}, actorID)
		if err != nil {
			t.Fatalf("applyDesignSubmissionWorkflow() err = %v", err)
		}
		if len(rec.calls) != 0 {
			t.Fatalf("calls = %+v, want none", rec.calls)
		}
	})
}

func TestDesignAssetSourceModuleKeyForTask(t *testing.T) {
	regular := &domain.Task{CustomizationRequired: false}
	if got := designAssetSourceModuleKeyForTask(regular, domain.TaskAssetTypeReference); got != domain.ModuleKeyBasicInfo {
		t.Fatalf("reference source_module_key = %q, want %q", got, domain.ModuleKeyBasicInfo)
	}
	if got := designAssetSourceModuleKeyForTask(regular, domain.TaskAssetTypeDelivery); got != domain.ModuleKeyDesign {
		t.Fatalf("delivery source_module_key = %q, want %q", got, domain.ModuleKeyDesign)
	}
	audit := &domain.Task{CustomizationRequired: false, TaskStatus: domain.TaskStatusPendingAuditA}
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

func TestDesignSubmissionTransitionForTask(t *testing.T) {
	regular := designSubmissionTransitionForTask(&domain.Task{CustomizationRequired: false})
	if regular.TaskStatus != domain.TaskStatusPendingAuditA || regular.ModuleKey != domain.ModuleKeyDesign {
		t.Fatalf("regular transition = %+v", regular)
	}
	custom := designSubmissionTransitionForTask(&domain.Task{CustomizationRequired: true})
	if custom.TaskStatus != domain.TaskStatusPendingCustomizationReview || custom.ModuleKey != domain.ModuleKeyCustomization {
		t.Fatalf("customization transition = %+v", custom)
	}
}
