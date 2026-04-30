package notification

import (
	"context"
	"encoding/json"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestGeneratorCandidates_AuditEnteredCreatesPendingAuditNotification(t *testing.T) {
	modules := &generatorModuleRepoStub{users: []int64{198, 199}}
	gen := NewGenerator(nil, modules, nil)

	candidates, err := gen.candidates(context.Background(), nil, domain.TaskModuleEvent{
		TaskModuleID: 629,
		EventType:    domain.ModuleEventEntered,
		Payload:      json.RawMessage(`{"pool_team_code":"audit_standard"}`),
	})
	if err != nil {
		t.Fatalf("candidates() error = %v", err)
	}
	if modules.team != domain.TeamAuditStandard {
		t.Fatalf("ListActiveUserIDsByTeam team = %q, want %q", modules.team, domain.TeamAuditStandard)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates len = %d, want 2", len(candidates))
	}
	for _, candidate := range candidates {
		if candidate.Type != domain.NotificationTypeTaskPendingAudit {
			t.Fatalf("candidate type = %s, want %s", candidate.Type, domain.NotificationTypeTaskPendingAudit)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(candidate.Payload, &payload); err != nil {
			t.Fatalf("payload unmarshal: %v", err)
		}
		if payload["module_key"] != domain.ModuleKeyAudit || payload["pool_team_code"] != domain.TeamAuditStandard {
			t.Fatalf("payload = %+v, want audit pool payload", payload)
		}
	}
}

type generatorModuleRepoStub struct {
	team  string
	users []int64
}

func (r *generatorModuleRepoStub) GetTaskModuleByID(context.Context, repo.Tx, int64) (*domain.TaskModule, error) {
	return &domain.TaskModule{TaskID: 629, ModuleKey: domain.ModuleKeyAudit}, nil
}

func (r *generatorModuleRepoStub) ListActiveUserIDsByTeam(_ context.Context, _ repo.Tx, teamCode string, _ *int64) ([]int64, error) {
	r.team = teamCode
	return r.users, nil
}

func (r *generatorModuleRepoStub) ListClaimedUserIDsByTask(context.Context, repo.Tx, int64, *int64) ([]int64, error) {
	return nil, nil
}
