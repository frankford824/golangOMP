package permission

import (
	"testing"

	"workflow/domain"
)

func TestScopeAllows_AuditBusinessTeamOnTechnicalPoolCode(t *testing.T) {
	task := &domain.Task{ID: 629, CreatorID: 197}
	module := &domain.TaskModule{
		TaskID:       629,
		ModuleKey:    domain.ModuleKeyAudit,
		State:        domain.ModuleStatePendingClaim,
		PoolTeamCode: strPtr(domain.TeamAuditStandard),
	}
	actor := domain.RequestActor{
		ID:         198,
		Department: string(domain.DepartmentAudit),
		Team:       "普通审核组",
		Roles:      []domain.Role{domain.RoleMember},
	}

	if !ScopeAllows(actor, task, module) {
		t.Fatal("ScopeAllows() = false, want true for 普通审核组 on audit_standard pool")
	}
}

func strPtr(value string) *string { return &value }
