package blueprint

import (
	"context"
	"encoding/json"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

type rulesTestModuleRepo struct {
	modules map[string]*domain.TaskModule
}

func (r *rulesTestModuleRepo) GetByTaskAndKey(_ context.Context, taskID int64, moduleKey string) (*domain.TaskModule, error) {
	if m := r.modules[moduleKey]; m != nil && m.TaskID == taskID {
		return m, nil
	}
	return nil, nil
}

func (r *rulesTestModuleRepo) ListByTask(context.Context, int64) ([]*domain.TaskModule, error) {
	out := make([]*domain.TaskModule, 0, len(r.modules))
	for _, m := range r.modules {
		out = append(out, m)
	}
	return out, nil
}

func (r *rulesTestModuleRepo) ClaimCAS(context.Context, repo.Tx, int64, string, string, int64, string, json.RawMessage) (bool, error) {
	return false, nil
}

func (r *rulesTestModuleRepo) Enter(_ context.Context, _ repo.Tx, taskID int64, moduleKey string, state domain.ModuleState, poolTeamCode *string, data json.RawMessage) (*domain.TaskModule, error) {
	m := &domain.TaskModule{
		ID:           int64(len(r.modules) + 1),
		TaskID:       taskID,
		ModuleKey:    moduleKey,
		State:        state,
		PoolTeamCode: poolTeamCode,
		Data:         data,
	}
	r.modules[moduleKey] = m
	return m, nil
}

func (r *rulesTestModuleRepo) UpdateState(context.Context, repo.Tx, int64, string, domain.ModuleState, bool, json.RawMessage) error {
	return nil
}

func (r *rulesTestModuleRepo) Reassign(context.Context, repo.Tx, int64, string, int64, string, json.RawMessage) error {
	return nil
}

func (r *rulesTestModuleRepo) PoolReassign(context.Context, repo.Tx, int64, string, string) error {
	return nil
}

func (r *rulesTestModuleRepo) CloseOpenModules(context.Context, repo.Tx, int64, domain.ModuleState) ([]*domain.TaskModule, error) {
	return nil, nil
}

func (r *rulesTestModuleRepo) InsertPlaceholder(context.Context, repo.Tx, int64, string) (*domain.TaskModule, error) {
	return nil, nil
}

type rulesTestEventRepo struct {
	events []*domain.TaskModuleEvent
}

func (r *rulesTestEventRepo) Insert(_ context.Context, _ repo.Tx, event *domain.TaskModuleEvent) (int64, error) {
	event.ID = int64(len(r.events) + 1)
	r.events = append(r.events, event)
	return event.ID, nil
}

func (r *rulesTestEventRepo) ListByTaskModule(context.Context, int64, int) ([]*domain.TaskModuleEvent, error) {
	return r.events, nil
}

func (r *rulesTestEventRepo) ListRecentByTask(context.Context, int64, int) ([]*domain.TaskModuleEvent, error) {
	return r.events, nil
}

func poolTeamCodeFromModule(m *domain.TaskModule) string {
	if m == nil || m.PoolTeamCode == nil {
		return ""
	}
	return *m.PoolTeamCode
}

func poolTeamCodeFromEventPayload(raw json.RawMessage) string {
	var p map[string]interface{}
	if err := json.Unmarshal(raw, &p); err != nil {
		return ""
	}
	v, _ := p["pool_team_code"].(string)
	return v
}

func TestRuleEngine_InitTask_HybridCustomizationIncludesCustomizationModule(t *testing.T) {
	ctx := context.Background()
	modules := &rulesTestModuleRepo{modules: map[string]*domain.TaskModule{}}
	events := &rulesTestEventRepo{}
	engine := NewRuleEngine(NewRegistry(), modules, events)

	task := &domain.Task{
		ID:                    807,
		TaskType:              domain.TaskTypeNewProductDevelopment,
		CustomizationRequired: true,
		BusinessLane:          domain.TaskBusinessLaneCustomization,
	}
	if err := engine.InitTask(ctx, nil, task); err != nil {
		t.Fatalf("InitTask() err = %v", err)
	}

	custom, err := modules.GetByTaskAndKey(ctx, task.ID, domain.ModuleKeyCustomization)
	if err != nil {
		t.Fatalf("GetByTaskAndKey(customization) err = %v", err)
	}
	if custom == nil {
		t.Fatal("expected customization module after InitTask for hybrid task")
	}
	if custom.State != domain.ModuleStatePendingClaim {
		t.Fatalf("customization state = %s, want %s", custom.State, domain.ModuleStatePendingClaim)
	}
	if got := poolTeamCodeFromModule(custom); got != domain.TeamCustomizationArt {
		t.Fatalf("customization pool_team_code = %q, want %q", got, domain.TeamCustomizationArt)
	}

	design, err := modules.GetByTaskAndKey(ctx, task.ID, domain.ModuleKeyDesign)
	if err != nil {
		t.Fatalf("GetByTaskAndKey(design) err = %v", err)
	}
	if design == nil {
		t.Fatal("expected design module to remain for hybrid new_product_development task")
	}
}

func TestRuleEngine_InitTask_RegularNewProductDevelopmentOmitsCustomizationModule(t *testing.T) {
	ctx := context.Background()
	modules := &rulesTestModuleRepo{modules: map[string]*domain.TaskModule{}}
	events := &rulesTestEventRepo{}
	engine := NewRuleEngine(NewRegistry(), modules, events)

	task := &domain.Task{
		ID:                    808,
		TaskType:              domain.TaskTypeNewProductDevelopment,
		CustomizationRequired: false,
		BusinessLane:          domain.TaskBusinessLaneNormal,
	}
	if err := engine.InitTask(ctx, nil, task); err != nil {
		t.Fatalf("InitTask() err = %v", err)
	}

	if custom, _ := modules.GetByTaskAndKey(ctx, task.ID, domain.ModuleKeyCustomization); custom != nil {
		t.Fatal("regular new_product_development must not instantiate customization module")
	}

	wantKeys := []string{
		domain.ModuleKeyBasicInfo,
		domain.ModuleKeyDesign,
		domain.ModuleKeyAudit,
		domain.ModuleKeyWarehouse,
	}
	list, err := modules.ListByTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListByTask() err = %v", err)
	}
	gotKeys := make(map[string]struct{}, len(list))
	for _, m := range list {
		gotKeys[m.ModuleKey] = struct{}{}
	}
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("module keys = %v, want %v", gotKeys, wantKeys)
	}
	for _, key := range wantKeys {
		if _, ok := gotKeys[key]; !ok {
			t.Fatalf("missing module key %s", key)
		}
	}

	design, err := modules.GetByTaskAndKey(ctx, task.ID, domain.ModuleKeyDesign)
	if err != nil {
		t.Fatalf("GetByTaskAndKey(design) err = %v", err)
	}
	if design.State != domain.ModuleStatePendingClaim {
		t.Fatalf("design state = %s, want %s", design.State, domain.ModuleStatePendingClaim)
	}
	if got := poolTeamCodeFromModule(design); got != domain.TeamDesignStandard {
		t.Fatalf("design pool_team_code = %q, want %q", got, domain.TeamDesignStandard)
	}
}

func TestRequiresHybridCustomizationModule_CustomizationRequiredAlone(t *testing.T) {
	task := &domain.Task{
		TaskType:              domain.TaskTypeNewProductDevelopment,
		CustomizationRequired: true,
		BusinessLane:          domain.TaskBusinessLaneNormal,
	}
	if !RequiresHybridCustomizationModule(task) {
		t.Fatal("expected hybrid customization module requirement when customization_required=true")
	}
}

func TestRuleEngine_InitTask_HybridCustomizationCustomizationOnlyRequired(t *testing.T) {
	ctx := context.Background()
	modules := &rulesTestModuleRepo{modules: map[string]*domain.TaskModule{}}
	engine := NewRuleEngine(NewRegistry(), modules, &rulesTestEventRepo{})

	task := &domain.Task{
		ID:                    809,
		TaskType:              domain.TaskTypeNewProductDevelopment,
		CustomizationRequired: true,
		BusinessLane:          domain.TaskBusinessLaneNormal,
	}
	if err := engine.InitTask(ctx, nil, task); err != nil {
		t.Fatalf("InitTask() err = %v", err)
	}
	if custom, _ := modules.GetByTaskAndKey(ctx, task.ID, domain.ModuleKeyCustomization); custom == nil {
		t.Fatal("expected customization module when customization_required=true")
	}
}

func TestRuleEngine_EnterAuditPoolForHybridCustomizationTask(t *testing.T) {
	ctx := context.Background()
	modules := &rulesTestModuleRepo{modules: map[string]*domain.TaskModule{}}
	events := &rulesTestEventRepo{}
	engine := NewRuleEngine(NewRegistry(), modules, events)

	task := &domain.Task{
		ID:                    798,
		TaskType:              domain.TaskTypeNewProductDevelopment,
		CustomizationRequired: true,
		BusinessLane:          domain.TaskBusinessLaneCustomization,
	}
	actorID := int64(701)

	if err := engine.ApplyAfterAction(ctx, nil, task, domain.ModuleKeyCustomization, domain.ModuleActionSubmit, &actorID, 0); err != nil {
		t.Fatalf("ApplyAfterAction(customization.submit) err = %v", err)
	}

	audit, err := modules.GetByTaskAndKey(ctx, task.ID, domain.ModuleKeyAudit)
	if err != nil {
		t.Fatalf("GetByTaskAndKey(audit) err = %v", err)
	}
	if got := poolTeamCodeFromModule(audit); got != domain.TeamAuditCustomization {
		t.Fatalf("audit module pool_team_code = %q, want %q", got, domain.TeamAuditCustomization)
	}

	var entered *domain.TaskModuleEvent
	for _, evt := range events.events {
		if evt.EventType == domain.ModuleEventEntered {
			entered = evt
			break
		}
	}
	if entered == nil {
		t.Fatal("expected audit entered event")
	}
	if got := poolTeamCodeFromEventPayload(entered.Payload); got != domain.TeamAuditCustomization {
		t.Fatalf("entered event payload.pool_team_code = %q, want %q", got, domain.TeamAuditCustomization)
	}
}

func TestRuleEngine_EnterAuditPoolForRegularProductTask(t *testing.T) {
	ctx := context.Background()
	modules := &rulesTestModuleRepo{modules: map[string]*domain.TaskModule{}}
	events := &rulesTestEventRepo{}
	engine := NewRuleEngine(NewRegistry(), modules, events)

	task := &domain.Task{
		ID:                    799,
		TaskType:              domain.TaskTypeNewProductDevelopment,
		CustomizationRequired: false,
		BusinessLane:          domain.TaskBusinessLaneNormal,
	}
	actorID := int64(702)

	if err := engine.ApplyAfterAction(ctx, nil, task, domain.ModuleKeyDesign, domain.ModuleActionSubmit, &actorID, 0); err != nil {
		t.Fatalf("ApplyAfterAction(design.submit) err = %v", err)
	}

	audit, err := modules.GetByTaskAndKey(ctx, task.ID, domain.ModuleKeyAudit)
	if err != nil {
		t.Fatalf("GetByTaskAndKey(audit) err = %v", err)
	}
	if got := poolTeamCodeFromModule(audit); got != domain.TeamAuditStandard {
		t.Fatalf("audit module pool_team_code = %q, want %q", got, domain.TeamAuditStandard)
	}

	var entered *domain.TaskModuleEvent
	for _, evt := range events.events {
		if evt.EventType == domain.ModuleEventEntered {
			entered = evt
			break
		}
	}
	if entered == nil {
		t.Fatal("expected audit entered event")
	}
	if got := poolTeamCodeFromEventPayload(entered.Payload); got != domain.TeamAuditStandard {
		t.Fatalf("entered event payload.pool_team_code = %q, want %q", got, domain.TeamAuditStandard)
	}
}
