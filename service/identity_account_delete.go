package service

import (
	"context"
	"errors"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type identityAccountDeletionRepo interface {
	MarkAccountDeleted(ctx context.Context, tx repo.Tx, user *domain.User, at time.Time) (bool, error)
}

type identitySessionRevoker interface {
	RevokeActiveByUserID(ctx context.Context, tx repo.Tx, userID int64, at time.Time) (int64, error)
}

func (s *identityService) DeleteUser(ctx context.Context, userID int64) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return domain.ErrUnauthorized
	}
	if userID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user id", nil)
	}
	if !domain.ActorHasPermission(actor, domain.PermissionAccessManage) {
		return domain.ErrPermissionDenied
	}
	if userID == actor.ID {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "不能删除当前登录账号。", nil)
	}
	user, appErr := s.GetUser(ctx, userID)
	if appErr != nil {
		return appErr
	}
	if appErr := s.authorizeUserStatusEndpoint(ctx, user); appErr != nil {
		return appErr
	}
	if user.IsConfigSuperAdmin || containsRole(user.Roles, domain.RoleSuperAdmin) {
		return protectedAccountDeleteError()
	}
	accounts, hasAccounts := s.userRepo.(identityAccountDeletionRepo)
	sessions, hasSessions := s.sessionRepo.(identitySessionRevoker)
	if !hasAccounts || !hasSessions || s.effectiveAccessReader == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "account deletion is unavailable", nil)
	}
	effective, err := s.effectiveAccessReader.EffectiveAccess(ctx, user.ID)
	if err != nil {
		return infraError("load account permissions for deletion", err)
	}
	if effectiveHasRole(effective, "super_admin") {
		return protectedAccountDeleteError()
	}

	now := time.Now().UTC()
	err = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		deleted, err := accounts.MarkAccountDeleted(ctx, tx, user, now)
		if err != nil {
			return err
		}
		if !deleted {
			return domain.NewAppError(domain.ErrCodeConflict, "账号已发生变化，请刷新列表后重新确认。", nil)
		}
		if _, err := sessions.RevokeActiveByUserID(ctx, tx, user.ID, now); err != nil {
			return err
		}
		return s.recordPermissionActionTx(ctx, tx, domain.PermissionLog{
			ActionType:     domain.PermissionActionUserDeleted,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    user.Roles,
			Granted:        true,
			Reason:         "account deleted by administrator",
			Method:         "DELETE",
			RoutePath:      "/v1/users/:id",
		})
	})
	var conflict *domain.AppError
	if errors.As(err, &conflict) {
		return conflict
	}
	if err != nil {
		return infraError("delete user account", err)
	}
	return nil
}

func protectedAccountDeleteError() *domain.AppError {
	return domain.NewAppError(domain.ErrCodePermissionDenied, "超级管理员账号受保护，无法删除。", nil)
}
