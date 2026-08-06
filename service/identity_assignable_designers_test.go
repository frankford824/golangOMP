package service

import (
	"context"
	"fmt"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

type assignableAccessReader struct {
	byUser map[int64]*domain.EffectiveAccess
}

func (r assignableAccessReader) EffectiveAccess(_ context.Context, userID int64) (*domain.EffectiveAccess, error) {
	return r.byUser[userID], nil
}

func (r assignableAccessReader) EffectiveAccessMany(_ context.Context, userIDs []int64) (map[int64]*domain.EffectiveAccess, error) {
	result := make(map[int64]*domain.EffectiveAccess, len(userIDs))
	for _, userID := range userIDs {
		result[userID] = r.byUser[userID]
	}
	return result, nil
}

func seedAssignableUser(t *testing.T, repository *identityUserRepoStub, username string, status domain.UserStatus, legacyRoles ...domain.Role) int64 {
	t.Helper()
	id, err := repository.Create(context.Background(), identityTx{}, &domain.User{
		Username:       username,
		DisplayName:    username,
		Department:     domain.DepartmentDesign,
		Team:           "设计组",
		Status:         status,
		EmploymentType: domain.EmploymentTypeFullTime,
	})
	if err != nil {
		t.Fatalf("seed user %s: %v", username, err)
	}
	if len(legacyRoles) > 0 {
		if err := repository.ReplaceRoles(context.Background(), identityTx{}, id, legacyRoles); err != nil {
			t.Fatalf("seed legacy roles for %s: %v", username, err)
		}
	}
	return id
}

func assignableEffectiveAccess(userID, roleID int64, roleCode string, permission domain.PermissionCode) *domain.EffectiveAccess {
	assignment := domain.AccessAssignment{
		ID:         roleID,
		UserID:     userID,
		RoleID:     roleID,
		RoleCode:   roleCode,
		ScopeMode:  domain.AccessScopeGlobal,
		SourceType: "direct",
	}
	return &domain.EffectiveAccess{
		UserID:      userID,
		Permissions: []domain.PermissionCode{permission},
		Assignments: []domain.AccessAssignment{assignment},
		Sources: []domain.EffectiveAccessNote{{
			Permission: permission,
			RoleID:     roleID,
			RoleCode:   roleCode,
			SourceType: "direct",
			ScopeMode:  domain.AccessScopeGlobal,
		}},
	}
}

func collectUsernames(users []*domain.User) []string {
	out := make([]string, 0, len(users))
	for _, user := range users {
		if user != nil {
			out = append(out, user.Username)
		}
	}
	return out
}

func assertUsernamesExact(t *testing.T, users []*domain.User, want ...string) {
	t.Helper()
	got := collectUsernames(users)
	if len(got) != len(want) {
		t.Fatalf("usernames = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("usernames = %v, want %v", got, want)
		}
	}
}

func TestListAssignableDesignersUsesExplicitAccessForEveryLane(t *testing.T) {
	userRepo := newIdentityUserRepo()
	designerID := seedAssignableUser(t, userRepo, "designer", domain.UserStatusActive)
	customizationID := seedAssignableUser(t, userRepo, "customization", domain.UserStatusActive)
	auditorID := seedAssignableUser(t, userRepo, "auditor", domain.UserStatusActive)
	customAuditorID := seedAssignableUser(t, userRepo, "custom_auditor", domain.UserStatusActive)
	disabledID := seedAssignableUser(t, userRepo, "disabled_designer", domain.UserStatusDisabled)
	legacyOnlyID := seedAssignableUser(t, userRepo, "legacy_only", domain.UserStatusActive, domain.RoleDesigner, domain.RoleAuditA)

	reader := assignableAccessReader{byUser: map[int64]*domain.EffectiveAccess{
		designerID:      assignableEffectiveAccess(designerID, 11, "designer", domain.PermissionTaskUploadSource),
		customizationID: assignableEffectiveAccess(customizationID, 12, "customization_operator", domain.PermissionTaskUploadSource),
		auditorID:       assignableEffectiveAccess(auditorID, 13, "auditor", domain.PermissionTaskAudit),
		customAuditorID: assignableEffectiveAccess(customAuditorID, 14, "quality_reviewer", domain.PermissionTaskAudit),
		disabledID:      assignableEffectiveAccess(disabledID, 15, "designer", domain.PermissionTaskUploadSource),
		legacyOnlyID:    nil,
	}}
	svc := NewIdentityService(
		userRepo,
		&identitySessionRepoStub{},
		&identityPermissionLogRepoStub{},
		identityTxRunner{},
		WithIdentityEffectiveAccessReader(reader),
	)
	actor := &domain.RequestActor{ID: 100}

	normal, appErr := svc.ListAssignableDesigners(context.Background(), actor, AssignableLaneNormal)
	if appErr != nil {
		t.Fatalf("normal lane: %+v", appErr)
	}
	assertUsernamesExact(t, normal, "designer")

	customization, appErr := svc.ListAssignableDesigners(context.Background(), actor, AssignableLaneCustomization)
	if appErr != nil {
		t.Fatalf("customization lane: %+v", appErr)
	}
	assertUsernamesExact(t, customization, "customization")

	audit, appErr := svc.ListAssignableDesigners(context.Background(), actor, AssignableLaneAudit)
	if appErr != nil {
		t.Fatalf("audit lane: %+v", appErr)
	}
	assertUsernamesExact(t, audit, "custom_auditor", "auditor")

	all, appErr := svc.ListAssignableDesigners(context.Background(), actor, AssignableLaneAll)
	if appErr != nil {
		t.Fatalf("all lane: %+v", appErr)
	}
	assertUsernamesExact(t, all, "customization", "designer")

	if len(normal) != 1 || !containsPermissionAction(normal[0].FrontendAccess.Actions, string(domain.PermissionTaskUploadSource)) {
		t.Fatalf("candidate frontend access was not projected from auth_*: %+v", normal[0].FrontendAccess.Actions)
	}
}

func TestListAssignableDesignersIncludesEverySourceFileUploaderInTheNormalLane(t *testing.T) {
	userRepo := newIdentityUserRepo()
	designerID := seedAssignableUser(t, userRepo, "designer", domain.UserStatusActive)
	directorID := seedAssignableUser(t, userRepo, "design_director", domain.UserStatusActive)
	customRoleID := seedAssignableUser(t, userRepo, "retouch_specialist", domain.UserStatusActive)
	customizationID := seedAssignableUser(t, userRepo, "customization", domain.UserStatusActive)
	operationsID := seedAssignableUser(t, userRepo, "operations", domain.UserStatusActive)

	reader := assignableAccessReader{byUser: map[int64]*domain.EffectiveAccess{
		designerID:      assignableEffectiveAccess(designerID, 11, "designer", domain.PermissionTaskUploadSource),
		directorID:      assignableEffectiveAccess(directorID, 12, "design_director", domain.PermissionTaskUploadSource),
		customRoleID:    assignableEffectiveAccess(customRoleID, 13, "retouch_specialist", domain.PermissionTaskUploadSource),
		customizationID: assignableEffectiveAccess(customizationID, 14, "customization_operator", domain.PermissionTaskUploadSource),
		operationsID:    assignableEffectiveAccess(operationsID, 15, "operations", domain.PermissionTaskCreate),
	}}
	svc := NewIdentityService(
		userRepo,
		&identitySessionRepoStub{},
		&identityPermissionLogRepoStub{},
		identityTxRunner{},
		WithIdentityEffectiveAccessReader(reader),
	)
	actor := &domain.RequestActor{ID: 100}

	normal, appErr := svc.ListAssignableDesigners(context.Background(), actor, AssignableLaneNormal)
	if appErr != nil {
		t.Fatalf("normal lane: %+v", appErr)
	}
	assertUsernamesExact(t, normal, "retouch_specialist", "design_director", "designer")

	customization, appErr := svc.ListAssignableDesigners(context.Background(), actor, AssignableLaneCustomization)
	if appErr != nil {
		t.Fatalf("customization lane: %+v", appErr)
	}
	assertUsernamesExact(t, customization, "customization")
}

func TestListAssignableDesignersTraversesEveryActiveUserPage(t *testing.T) {
	userRepo := newIdentityUserRepo()
	reader := assignableAccessReader{byUser: map[int64]*domain.EffectiveAccess{}}
	var oldestCandidateID int64
	for index := 0; index < 125; index++ {
		userID := seedAssignableUser(t, userRepo, fmt.Sprintf("user-%03d", index), domain.UserStatusActive)
		if index == 0 {
			oldestCandidateID = userID
			reader.byUser[userID] = assignableEffectiveAccess(userID, 11, "designer", domain.PermissionTaskUploadSource)
		}
	}
	svc := NewIdentityService(
		userRepo,
		&identitySessionRepoStub{},
		&identityPermissionLogRepoStub{},
		identityTxRunner{},
		WithIdentityEffectiveAccessReader(reader),
	)

	users, appErr := svc.ListAssignableDesigners(context.Background(), &domain.RequestActor{ID: 999}, AssignableLaneNormal)
	if appErr != nil {
		t.Fatalf("list candidates: %+v", appErr)
	}
	if oldestCandidateID <= 0 {
		t.Fatal("oldest candidate was not seeded")
	}
	assertUsernamesExact(t, users, "user-000")
	if len(userRepo.listFilters) != 2 {
		t.Fatalf("list calls = %d, want 2", len(userRepo.listFilters))
	}
	for index, filter := range userRepo.listFilters {
		if filter.Page != index+1 || filter.PageSize != 100 {
			t.Fatalf("list filter[%d] = page %d size %d, want page %d size 100", index, filter.Page, filter.PageSize, index+1)
		}
	}
}

func TestListAssignableDesignersFailsClosedWithoutEffectiveAccessReader(t *testing.T) {
	userRepo := newIdentityUserRepo()
	seedAssignableUser(t, userRepo, "legacy_designer", domain.UserStatusActive, domain.RoleDesigner)
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	users, appErr := svc.ListAssignableDesigners(context.Background(), &domain.RequestActor{ID: 1}, AssignableLaneNormal)
	if appErr == nil || appErr.Code != domain.ErrCodeInternalError {
		t.Fatalf("missing effective access reader users/error = %+v/%+v", users, appErr)
	}
}

func TestListAssignableDesignersRejectsInvalidActorAndLane(t *testing.T) {
	userRepo := newIdentityUserRepo()
	reader := assignableAccessReader{byUser: map[int64]*domain.EffectiveAccess{}}
	svc := NewIdentityService(
		userRepo,
		&identitySessionRepoStub{},
		&identityPermissionLogRepoStub{},
		identityTxRunner{},
		WithIdentityEffectiveAccessReader(reader),
	)

	if users, appErr := svc.ListAssignableDesigners(context.Background(), nil, AssignableLaneNormal); appErr == nil || appErr.Code != domain.ErrCodeUnauthorized {
		t.Fatalf("nil actor users/error = %+v/%+v", users, appErr)
	}
	if users, appErr := svc.ListAssignableDesigners(context.Background(), &domain.RequestActor{ID: 1}, AssignableLane("warehouse")); appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("invalid lane users/error = %+v/%+v", users, appErr)
	}
	if len(userRepo.listFilters) != 0 {
		t.Fatalf("invalid requests reached repository: %+v", userRepo.listFilters)
	}
}

var _ IdentityEffectiveAccessBatchReader = assignableAccessReader{}
var _ repo.UserRepo = (*identityUserRepoStub)(nil)
