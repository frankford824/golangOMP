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

func TestRenamedDesignDepartmentMatchesDesignPoolCode(t *testing.T) {
	actor := domain.RequestActor{
		ID:         200,
		Department: "视觉研创部",
		Team:       "默认组",
		Roles:      []domain.Role{domain.RoleDesigner},
	}

	if !actorMatchesPool(actor, domain.TeamDesignStandard) {
		t.Fatal("actorMatchesPool() = false, want true for 视觉研创部/默认组 -> design_standard")
	}
	if got := matchedTeam(actor, domain.TeamDesignStandard); got != "默认组" {
		t.Fatalf("matchedTeam() = %q, want 默认组", got)
	}
}
