package task_pool

import (
	"testing"

	"workflow/domain"
)

func TestAuditBusinessTeamMatchesTechnicalPoolCode(t *testing.T) {
	actor := domain.RequestActor{
		ID:         198,
		Department: string(domain.DepartmentAudit),
		Team:       "普通审核组",
		Roles:      []domain.Role{domain.RoleMember},
	}

	if !actorMatchesPool(actor, domain.TeamAuditStandard) {
		t.Fatal("actorMatchesPool() = false, want true for 普通审核组 -> audit_standard")
	}
	if got := matchedTeam(actor, domain.TeamAuditStandard); got != "普通审核组" {
		t.Fatalf("matchedTeam() = %q, want 普通审核组", got)
	}
	if !contains(actorPoolCodes(actor), domain.TeamAuditStandard) {
		t.Fatalf("actorPoolCodes() = %v, want audit_standard included", actorPoolCodes(actor))
	}
}

func TestCustomizationAuditBusinessTeamMatchesTechnicalPoolCode(t *testing.T) {
	actor := domain.RequestActor{
		ID:         199,
		Department: string(domain.DepartmentAudit),
		Team:       "定制审核组",
		Roles:      []domain.Role{domain.RoleMember},
	}

	if !actorMatchesPool(actor, domain.TeamAuditCustomization) {
		t.Fatal("actorMatchesPool() = false, want true for 定制审核组 -> audit_customization")
	}
}
