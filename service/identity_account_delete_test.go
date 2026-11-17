package service

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type accountDeletionUserRepo struct{ *identityUserRepoStub }

func (r accountDeletionUserRepo) MarkAccountDeleted(_ context.Context, _ repo.Tx, user *domain.User, at time.Time) (bool, error) {
	row := r.users[user.ID]
	row.Status, row.UpdatedAt = domain.UserStatusDeleted, at
	row.DepartmentID, row.TeamID = nil, nil
	return true, nil
}

type accountDeletionSessionRepo struct {
	identitySessionRepoStub
	revoked []int64
}

func (r *accountDeletionSessionRepo) RevokeActiveByUserID(_ context.Context, _ repo.Tx, userID int64, at time.Time) (int64, error) {
	r.revoked = append(r.revoked, userID)
	for _, session := range r.sessions {
		if session.UserID == userID {
			session.RevokedAt = &at
		}
	}
	return 1, nil
}

func TestIdentityDeleteAccountProtectsAdminsAndRevokesSessions(t *testing.T) {
	users := accountDeletionUserRepo{newIdentityUserRepo()}
	sessions := &accountDeletionSessionRepo{}
	logs := &identityPermissionLogRepoStub{}
	access := assignableAccessReader{byUser: make(map[int64]*domain.EffectiveAccess)}
	svc := NewIdentityService(users, sessions, logs, identityTxRunner{}, WithIdentityEffectiveAccessReader(access))
	accountID := seedAssignableUser(t, users.identityUserRepoStub, "account-for-deletion", domain.UserStatusActive)
	adminID := seedAssignableUser(t, users.identityUserRepoStub, "admin-for-deletion-test", domain.UserStatusActive)
	protectedID := seedAssignableUser(t, users.identityUserRepoStub, "protected-account", domain.UserStatusActive)
	access.byUser[protectedID] = &domain.EffectiveAccess{Assignments: []domain.AccessAssignment{{RoleCode: "super_admin"}}}
	departmentID := int64(15)
	users.users[accountID].DepartmentID = &departmentID
	ctx := identityGlobalAccessManageContext(adminID)

	for _, check := range []struct {
		ctx    context.Context
		target int64
	}{
		{ctx, adminID},
		{ctx, protectedID},
		{identityDepartmentAccessManageContext(adminID, 42), accountID},
	} {
		if appErr := svc.DeleteUser(check.ctx, check.target); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
			t.Fatalf("DeleteUser(%d) error = %+v, want permission denial", check.target, appErr)
		}
	}
	if len(sessions.revoked) != 0 || len(logs.logs) != 0 {
		t.Fatal("denied deletions must not revoke sessions or record successful deletion")
	}
	if _, err := sessions.Create(ctx, identityTx{}, &domain.UserSession{UserID: accountID, TokenHash: "existing-session", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if appErr := svc.DeleteUser(identityDepartmentAccessManageContext(adminID, departmentID), accountID); appErr != nil {
		t.Fatalf("DeleteUser() = %+v", appErr)
	}
	if row := users.users[accountID]; row.Status != domain.UserStatusDeleted || row.Username != "account-for-deletion" || row.DepartmentID != nil {
		t.Fatalf("deleted account row = %+v", row)
	}
	if session := sessions.sessions["existing-session"]; session.RevokedAt == nil {
		t.Fatal("existing session was not revoked")
	}
	if len(logs.logs) != 1 || logs.logs[0].ActionType != domain.PermissionActionUserDeleted {
		t.Fatalf("delete audit = %+v", logs.logs)
	}
	if _, appErr := svc.GetUser(ctx, accountID); appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("GetUser(deleted) = %+v, want not found", appErr)
	}
	if appErr := svc.ActivateUser(ctx, accountID); appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("ActivateUser(deleted) = %+v, want not found", appErr)
	}
}
