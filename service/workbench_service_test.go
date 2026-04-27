package service

import (
	"context"
	"fmt"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestWorkbenchServiceGetPreferencesRequiresUserScopedActor(t *testing.T) {
	svc := NewWorkbenchService(newWorkbenchPreferenceRepoStub()).(*workbenchService)

	result, appErr := svc.GetPreferences(context.Background())
	if appErr == nil || appErr.Code != domain.ErrCodeUnauthorized {
		t.Fatalf("GetPreferences() appErr = %+v, want unauthorized", appErr)
	}
	if result != nil {
		t.Fatalf("GetPreferences() result = %+v, want nil", result)
	}
}

func TestWorkbenchServiceGetPreferencesReturnsEffectiveDefaultsAndConfigForSessionActor(t *testing.T) {
	svc := NewWorkbenchService(newWorkbenchPreferenceRepoStub()).(*workbenchService)

	result, appErr := svc.GetPreferences(domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       7,
		Roles:    []domain.Role{domain.RoleOps},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}))
	if appErr != nil {
		t.Fatalf("GetPreferences() unexpected error: %+v", appErr)
	}
	if result.Actor.ID != 7 || result.Actor.AuthMode != domain.AuthModeSessionTokenRoleEnforced {
		t.Fatalf("actor = %+v", result.Actor)
	}
	if result.Preferences.DefaultPageSize != defaultWorkbenchPageSize {
		t.Fatalf("default_page_size = %d, want %d", result.Preferences.DefaultPageSize, defaultWorkbenchPageSize)
	}
	if result.Preferences.DefaultSort != domain.WorkbenchSortUpdatedAtDesc {
		t.Fatalf("default_sort = %s, want %s", result.Preferences.DefaultSort, domain.WorkbenchSortUpdatedAtDesc)
	}
	if len(result.WorkbenchConfig.Queues) != len(taskBoardPresetDefinitions()) {
		t.Fatalf("workbench queue config count = %d, want %d", len(result.WorkbenchConfig.Queues), len(taskBoardPresetDefinitions()))
	}
	if result.WorkbenchConfig.Queues[0].OwnershipHint == "" {
		t.Fatal("first queue ownership_hint should not be empty")
	}
}

func TestWorkbenchServicePatchPreferencesPersistsBySessionActorScope(t *testing.T) {
	repoStub := newWorkbenchPreferenceRepoStub()
	svc := NewWorkbenchService(repoStub).(*workbenchService)

	actorCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       42,
		Roles:    []domain.Role{domain.RoleDesigner, domain.RoleOps},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})

	defaultQueueKey := "design_pending_submit"
	pinnedQueueKeys := []string{"design_pending_submit", "ops_pending_material", "design_pending_submit"}
	defaultPageSize := 50
	defaultSort := domain.WorkbenchSortUpdatedAtDesc
	defaultFilters := domain.TaskQueryTemplate{
		TaskType:       "original_product_development",
		DesignerID:     workbenchInt64Ptr(42),
		SubStatusCode:  "designing,rework_required",
		SubStatusScope: "design",
	}

	result, appErr := svc.PatchPreferences(actorCtx, domain.WorkbenchPreferencesPatch{
		DefaultQueueKey: &defaultQueueKey,
		PinnedQueueKeys: &pinnedQueueKeys,
		DefaultFilters:  &defaultFilters,
		DefaultPageSize: &defaultPageSize,
		DefaultSort:     &defaultSort,
	})
	if appErr != nil {
		t.Fatalf("PatchPreferences() unexpected error: %+v", appErr)
	}

	if result.Preferences.DefaultQueueKey != "design_pending_submit" {
		t.Fatalf("default_queue_key = %q", result.Preferences.DefaultQueueKey)
	}
	if len(result.Preferences.PinnedQueueKeys) != 2 {
		t.Fatalf("pinned_queue_keys = %+v, want deduplicated length 2", result.Preferences.PinnedQueueKeys)
	}
	if result.Preferences.DefaultPageSize != 50 {
		t.Fatalf("default_page_size = %d, want 50", result.Preferences.DefaultPageSize)
	}

	stored, err := repoStub.GetByActorScope(context.Background(), repo.WorkbenchPreferenceScope{
		ActorID:       42,
		ActorRolesKey: "Designer,Ops",
		AuthMode:      domain.AuthModeSessionTokenRoleEnforced,
	})
	if err != nil {
		t.Fatalf("GetByActorScope() unexpected error: %v", err)
	}
	if stored == nil {
		t.Fatal("stored preferences missing")
	}
	if stored.Preferences.DefaultFilters.SubStatusScope != "design" {
		t.Fatalf("stored default_filters.sub_status_scope = %q", stored.Preferences.DefaultFilters.SubStatusScope)
	}

	otherActorCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       43,
		Roles:    []domain.Role{domain.RoleDesigner},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	other, appErr := svc.GetPreferences(otherActorCtx)
	if appErr != nil {
		t.Fatalf("GetPreferences(other actor) unexpected error: %+v", appErr)
	}
	if other.Preferences.DefaultQueueKey != "" {
		t.Fatalf("other actor default_queue_key = %q, want empty", other.Preferences.DefaultQueueKey)
	}
}

func TestWorkbenchServiceRejectsDebugActorScope(t *testing.T) {
	svc := NewWorkbenchService(newWorkbenchPreferenceRepoStub()).(*workbenchService)

	_, appErr := svc.GetPreferences(domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       42,
		Roles:    []domain.Role{domain.RoleOps},
		Source:   domain.RequestActorSourceDebugHeader,
		AuthMode: domain.AuthModeDebugHeaderRoleEnforced,
	}))
	if appErr == nil || appErr.Code != domain.ErrCodeUnauthorized {
		t.Fatalf("GetPreferences(debug actor) appErr = %+v, want unauthorized", appErr)
	}
}

func TestWorkbenchServiceRejectsUnsupportedQueueAndInvalidFilterShape(t *testing.T) {
	svc := NewWorkbenchService(newWorkbenchPreferenceRepoStub()).(*workbenchService)

	unknownQueue := "finance_queue"
	_, appErr := svc.PatchPreferences(domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       7,
		Roles:    []domain.Role{domain.RoleOps},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}), domain.WorkbenchPreferencesPatch{
		DefaultQueueKey: &unknownQueue,
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("expected invalid request for unknown queue, got %+v", appErr)
	}

	invalidFilters := domain.TaskQueryTemplate{
		SubStatusScope: "design",
	}
	_, appErr = svc.PatchPreferences(domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       7,
		Roles:    []domain.Role{domain.RoleOps},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}), domain.WorkbenchPreferencesPatch{
		DefaultFilters: &invalidFilters,
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("expected invalid request for invalid filter shape, got %+v", appErr)
	}
}

type workbenchPreferenceRepoStub struct {
	records map[string]*domain.WorkbenchPreferenceRecord
}

func newWorkbenchPreferenceRepoStub() *workbenchPreferenceRepoStub {
	return &workbenchPreferenceRepoStub{
		records: map[string]*domain.WorkbenchPreferenceRecord{},
	}
}

func (r *workbenchPreferenceRepoStub) GetByActorScope(_ context.Context, scope repo.WorkbenchPreferenceScope) (*domain.WorkbenchPreferenceRecord, error) {
	record, ok := r.records[workbenchScopeKey(scope)]
	if !ok {
		return nil, nil
	}
	copyRecord := *record
	copyRecord.Preferences.PinnedQueueKeys = append([]string(nil), record.Preferences.PinnedQueueKeys...)
	return &copyRecord, nil
}

func (r *workbenchPreferenceRepoStub) UpsertByActorScope(_ context.Context, record *domain.WorkbenchPreferenceRecord) error {
	copyRecord := *record
	copyRecord.Preferences.PinnedQueueKeys = append([]string(nil), record.Preferences.PinnedQueueKeys...)
	r.records[workbenchScopeKey(repo.WorkbenchPreferenceScope{
		ActorID:       record.ActorID,
		ActorRolesKey: record.ActorRolesKey,
		AuthMode:      record.AuthMode,
	})] = &copyRecord
	return nil
}

func workbenchScopeKey(scope repo.WorkbenchPreferenceScope) string {
	return fmt.Sprintf("%s|%d|%s", scope.AuthMode, scope.ActorID, scope.ActorRolesKey)
}

func workbenchInt64Ptr(v int64) *int64 {
	return &v
}
