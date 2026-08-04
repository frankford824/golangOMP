package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestIdentityServiceAuthorizeUserMutationUsesExplicitStableScope(t *testing.T) {
	departmentID := int64(41)
	teamID := int64(51)
	otherDepartmentID := int64(42)
	target := &domain.User{
		ID:           30001,
		Department:   domain.DepartmentOperations,
		DepartmentID: &departmentID,
		Team:         "淘系一组",
		TeamID:       &teamID,
		Roles:        []domain.Role{domain.RoleMember},
	}
	svc := NewIdentityService(newIdentityUserRepo(), &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}).(*identityService)

	tests := []struct {
		name      string
		actor     domain.RequestActor
		nextDept  *int64
		nextTeam  *int64
		wantAllow bool
		wantStatus bool
	}{
		{name: "global manage", actor: identityAccessActor(7, domain.PermissionAccessManage, domain.AccessScopeGlobal, nil), nextDept: &otherDepartmentID, nextTeam: &teamID, wantAllow: true, wantStatus: true},
		{name: "selected department", actor: identityAccessActor(8, domain.PermissionAccessManage, domain.AccessScopeSelectedOrg, []domain.AccessScopeSubject{{SubjectType: domain.AccessSubjectDepartment, SubjectID: departmentID}}), nextDept: &departmentID, nextTeam: &teamID, wantAllow: true, wantStatus: true},
		{name: "selected department cannot move outside", actor: identityAccessActor(8, domain.PermissionAccessManage, domain.AccessScopeSelectedOrg, []domain.AccessScopeSubject{{SubjectType: domain.AccessSubjectDepartment, SubjectID: departmentID}}), nextDept: &otherDepartmentID, nextTeam: &teamID, wantAllow: false, wantStatus: true},
		{name: "view is not write", actor: identityAccessActor(9, domain.PermissionAccessView, domain.AccessScopeGlobal, nil), nextDept: &departmentID, nextTeam: &teamID, wantAllow: false, wantStatus: false},
		{name: "legacy role alone is not authorization", actor: domain.RequestActor{ID: 10, Roles: []domain.Role{domain.RoleSuperAdmin}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced}, nextDept: &departmentID, nextTeam: &teamID, wantAllow: false, wantStatus: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := domain.WithRequestActor(context.Background(), tc.actor)
			updateErr := svc.authorizeUserUpdate(ctx, target, tc.nextDept, tc.nextTeam)
			statusErr := svc.authorizeUserStatusEndpoint(ctx, target)
			if (updateErr == nil) != tc.wantAllow {
				t.Fatalf("update=%+v, want allow=%v", updateErr, tc.wantAllow)
			}
			if (statusErr == nil) != tc.wantStatus {
				t.Fatalf("status=%+v, want allow=%v", statusErr, tc.wantStatus)
			}
		})
	}
}

func TestIdentityServiceAuthorizeUserMutationOwnTeamUsesStableID(t *testing.T) {
	departmentID := int64(41)
	teamID := int64(51)
	otherTeamID := int64(52)
	target := &domain.User{ID: 30002, DepartmentID: &departmentID, TeamID: &teamID}
	svc := NewIdentityService(newIdentityUserRepo(), &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}).(*identityService)

	actor := identityAccessActor(11, domain.PermissionAccessManage, domain.AccessScopeOwnTeam, nil)
	actor.DepartmentID = &departmentID
	actor.TeamID = &teamID
	ctx := domain.WithRequestActor(context.Background(), actor)
	if appErr := svc.authorizeUserUpdate(ctx, target, &departmentID, &teamID); appErr != nil {
		t.Fatalf("same stable team denied: %+v", appErr)
	}
	if appErr := svc.authorizeUserUpdate(ctx, target, &departmentID, &otherTeamID); appErr == nil {
		t.Fatal("move outside own stable team allowed")
	}
}

func identityAccessActor(actorID int64, permission domain.PermissionCode, scope domain.AccessScopeMode, subjects []domain.AccessScopeSubject) domain.RequestActor {
	roleID := actorID + 1000
	assignment := domain.AccessAssignment{
		ID:         roleID,
		UserID:     actorID,
		RoleID:     roleID,
		RoleCode:   "access_operator",
		ScopeMode:  scope,
		Subjects:   subjects,
		SourceType: "direct",
	}
	effective := &domain.EffectiveAccess{
		UserID:      actorID,
		Permissions: []domain.PermissionCode{permission},
		Assignments: []domain.AccessAssignment{assignment},
		Sources: []domain.EffectiveAccessNote{{
			Permission: permission,
			RoleID:     roleID,
			RoleCode:   assignment.RoleCode,
			SourceType: assignment.SourceType,
			ScopeMode:  scope,
		}},
	}
	return domain.RequestActor{
		ID:              actorID,
		Permissions:     effective.Permissions,
		EffectiveAccess: effective,
		Source:          domain.RequestActorSourceSessionToken,
		AuthMode:        domain.AuthModeSessionTokenRoleEnforced,
	}
}
