package org_move_request

import (
	"context"
	"errors"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type CreateParams struct {
	UserID             int64
	TargetDepartmentID *int64
	Reason             string
}

type ListFilter struct {
	State              *domain.OrgMoveRequestState
	UserID             *int64
	SourceDepartmentID *int64
	Page               int
	PageSize           int
}

type Service interface {
	Create(ctx context.Context, actor domain.RequestActor, sourceDepartmentID int64, p CreateParams) (*domain.OrgMoveRequest, *domain.AppError)
	List(ctx context.Context, actor domain.RequestActor, filter ListFilter) ([]*domain.OrgMoveRequest, domain.PaginationMeta, *domain.AppError)
	Approve(ctx context.Context, actor domain.RequestActor, requestID int64) *domain.AppError
	Reject(ctx context.Context, actor domain.RequestActor, requestID int64, reason string) *domain.AppError
}

type service struct {
	users    repo.UserRepo
	orgs     repo.OrgRepo
	requests repo.OrgMoveRequestRepo
	logs     repo.PermissionLogRepo
	txRunner repo.TxRunner
}

func NewService(users repo.UserRepo, orgs repo.OrgRepo, requests repo.OrgMoveRequestRepo, logs repo.PermissionLogRepo, txRunner repo.TxRunner) Service {
	return &service{users: users, orgs: orgs, requests: requests, logs: logs, txRunner: txRunner}
}

func (s *service) Create(ctx context.Context, actor domain.RequestActor, sourceDepartmentID int64, p CreateParams) (*domain.OrgMoveRequest, *domain.AppError) {
	if actor.ID <= 0 {
		return nil, domain.ErrUnauthorized
	}
	if p.UserID <= 0 || sourceDepartmentID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id and source department are required", nil)
	}
	reason := strings.TrimSpace(p.Reason)
	if reason == "" {
		return nil, domain.NewAppError(domain.ErrCodeReasonRequired, "reason is required", map[string]string{"deny_code": "reason_required"})
	}
	source, target, appErr := s.resolveDepartments(ctx, sourceDepartmentID, p.TargetDepartmentID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.authorizeCreate(actor, source.Name); appErr != nil {
		return nil, appErr
	}
	user, err := s.users.GetByID(ctx, p.UserID)
	if err != nil {
		return nil, infraError("get org move user", err)
	}
	if user == nil {
		return nil, domain.ErrNotFound
	}
	if !strings.EqualFold(string(user.Department), source.Name) {
		return nil, scopeDenied("org_move_source_department_mismatch", "target user does not belong to source department")
	}
	targetName := ""
	if target != nil {
		targetName = target.Name
	}
	if strings.EqualFold(source.Name, targetName) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "target department must differ from source department", nil)
	}

	now := time.Now().UTC()
	request := &domain.OrgMoveRequest{
		UserID:             p.UserID,
		SourceDepartmentID: source.ID,
		TargetDepartmentID: p.TargetDepartmentID,
		SourceDepartment:   source.Name,
		TargetDepartment:   targetName,
		State:              domain.OrgMoveRequestStatePendingSuperAdminConfirm,
		RequestedByUserID:  actor.ID,
		Reason:             reason,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		id, err := s.requests.Create(ctx, tx, request)
		if err != nil {
			return err
		}
		request.ID = id
		return s.recordPermissionActionTx(ctx, tx, actor, domain.PermissionActionOrgMoveRequested, request.UserID, "POST", "/v1/departments/:id/org-move-requests", reason)
	}); err != nil {
		return nil, infraError("create org move request tx", err)
	}
	return request, nil
}

func (s *service) List(ctx context.Context, actor domain.RequestActor, filter ListFilter) ([]*domain.OrgMoveRequest, domain.PaginationMeta, *domain.AppError) {
	if actor.ID <= 0 {
		return nil, domain.PaginationMeta{}, domain.ErrUnauthorized
	}
	if filter.State != nil && !filter.State.Valid() {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "state is invalid", nil)
	}
	repoFilter := repo.OrgMoveRequestFilter{
		State:              filter.State,
		UserID:             filter.UserID,
		SourceDepartmentID: filter.SourceDepartmentID,
		Page:               filter.Page,
		PageSize:           filter.PageSize,
	}
	switch {
	case hasAnyRole(actor, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleAdmin):
	case hasAnyRole(actor, domain.RoleDeptAdmin):
		departments := actor.ManagedDepartments
		if len(departments) == 0 && strings.TrimSpace(actor.Department) != "" {
			departments = []string{actor.Department}
		}
		if len(departments) == 0 {
			return nil, domain.PaginationMeta{}, scopeDenied("department_admin_scope_missing", "department admin scope is not configured")
		}
		repoFilter.SourceDepartments = departments
		repoFilter.SourceDepartmentID = nil
	default:
		return nil, domain.PaginationMeta{}, scopeDenied("org_move_list_denied", "org move requests require organization management role")
	}
	items, total, err := s.requests.List(ctx, repoFilter)
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list org move requests", err)
	}
	return items, pagination(filter.Page, filter.PageSize, total), nil
}

func (s *service) Approve(ctx context.Context, actor domain.RequestActor, requestID int64) *domain.AppError {
	if requestID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "request id is required", nil)
	}
	if !hasAnyRole(actor, domain.RoleSuperAdmin, domain.RoleAdmin) {
		return scopeDenied("module_action_role_denied", "only SuperAdmin can approve org move requests")
	}
	request, appErr := s.getPending(ctx, requestID)
	if appErr != nil {
		return appErr
	}
	user, err := s.users.GetByID(ctx, request.UserID)
	if err != nil {
		return infraError("get org move target user", err)
	}
	if user == nil {
		return domain.ErrNotFound
	}
	targetDepartment := domain.Department(request.TargetDepartment)
	now := time.Now().UTC()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		ok, err := s.requests.UpdateState(ctx, tx, request.ID, domain.OrgMoveRequestStatePendingSuperAdminConfirm, domain.OrgMoveRequestStateApproved, actor.ID, request.Reason, now)
		if err != nil {
			return err
		}
		if !ok {
			return errAlreadyDecided
		}
		user.Department = targetDepartment
		user.Team = ""
		user.UpdatedAt = now
		if err := s.users.Update(ctx, tx, user); err != nil {
			return err
		}
		if err := s.recordPermissionActionTx(ctx, tx, actor, domain.PermissionActionOrgMoveApproved, user.ID, "POST", "/v1/org-move-requests/:id/approve", request.Reason); err != nil {
			return err
		}
		return s.recordPermissionActionTx(ctx, tx, actor, domain.PermissionActionUserDepartmentChangedByAdmin, user.ID, "POST", "/v1/org-move-requests/:id/approve", request.SourceDepartment+" -> "+request.TargetDepartment)
	}); err != nil {
		if errors.Is(err, errAlreadyDecided) {
			return alreadyDecided()
		}
		return infraError("approve org move request tx", err)
	}
	return nil
}

func (s *service) Reject(ctx context.Context, actor domain.RequestActor, requestID int64, reason string) *domain.AppError {
	reason = strings.TrimSpace(reason)
	if requestID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "request id is required", nil)
	}
	if reason == "" {
		return domain.NewAppError(domain.ErrCodeReasonRequired, "reason is required", map[string]string{"deny_code": "reason_required"})
	}
	if !hasAnyRole(actor, domain.RoleSuperAdmin, domain.RoleAdmin) {
		return scopeDenied("module_action_role_denied", "only SuperAdmin can reject org move requests")
	}
	request, appErr := s.getPending(ctx, requestID)
	if appErr != nil {
		return appErr
	}
	now := time.Now().UTC()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		ok, err := s.requests.UpdateState(ctx, tx, request.ID, domain.OrgMoveRequestStatePendingSuperAdminConfirm, domain.OrgMoveRequestStateRejected, actor.ID, reason, now)
		if err != nil {
			return err
		}
		if !ok {
			return errAlreadyDecided
		}
		return s.recordPermissionActionTx(ctx, tx, actor, domain.PermissionActionOrgMoveRejected, request.UserID, "POST", "/v1/org-move-requests/:id/reject", reason)
	}); err != nil {
		if errors.Is(err, errAlreadyDecided) {
			return alreadyDecided()
		}
		return infraError("reject org move request tx", err)
	}
	return nil
}

func (s *service) resolveDepartments(ctx context.Context, sourceDepartmentID int64, targetDepartmentID *int64) (*domain.OrgDepartment, *domain.OrgDepartment, *domain.AppError) {
	source, err := s.orgs.GetDepartmentByID(ctx, sourceDepartmentID)
	if err != nil {
		return nil, nil, infraError("get source department", err)
	}
	if source == nil {
		return nil, nil, domain.ErrNotFound
	}
	var target *domain.OrgDepartment
	if targetDepartmentID != nil && *targetDepartmentID > 0 {
		target, err = s.orgs.GetDepartmentByID(ctx, *targetDepartmentID)
		if err != nil {
			return nil, nil, infraError("get target department", err)
		}
		if target == nil {
			return nil, nil, domain.ErrNotFound
		}
	}
	return source, target, nil
}

func (s *service) authorizeCreate(actor domain.RequestActor, sourceDepartment string) *domain.AppError {
	switch {
	case hasAnyRole(actor, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleAdmin):
		return nil
	case hasAnyRole(actor, domain.RoleDeptAdmin):
		if actorManagesDepartment(actor, sourceDepartment) {
			return nil
		}
		return scopeDenied("department_scope_only", "department admin can only request moves from managed departments")
	default:
		return scopeDenied("org_move_request_denied", "org move request requires DeptAdmin, HRAdmin, or SuperAdmin")
	}
}

func (s *service) getPending(ctx context.Context, id int64) (*domain.OrgMoveRequest, *domain.AppError) {
	request, err := s.requests.Get(ctx, id)
	if err != nil {
		return nil, infraError("get org move request", err)
	}
	if request == nil {
		return nil, domain.ErrNotFound
	}
	if request.State != domain.OrgMoveRequestStatePendingSuperAdminConfirm {
		return nil, alreadyDecided()
	}
	return request, nil
}

type permissionLogTxRepo interface {
	CreateTx(ctx context.Context, tx repo.Tx, entry *domain.PermissionLog) error
}

func (s *service) recordPermissionActionTx(ctx context.Context, tx repo.Tx, actor domain.RequestActor, action string, targetUserID int64, method, routePath, reason string) error {
	if s.logs == nil {
		return nil
	}
	entry := &domain.PermissionLog{
		ActorID:       &actor.ID,
		ActorUsername: actor.Username,
		ActorSource:   actor.Source,
		AuthMode:      actor.AuthMode,
		Readiness:     domain.APIReadinessReadyForFrontend,
		ActionType:    action,
		ActorRoles:    actor.Roles,
		TargetUserID:  &targetUserID,
		Method:        method,
		RoutePath:     routePath,
		Granted:       true,
		Reason:        reason,
		CreatedAt:     time.Now().UTC(),
	}
	if txRepo, ok := s.logs.(permissionLogTxRepo); ok {
		return txRepo.CreateTx(ctx, tx, entry)
	}
	return s.logs.Create(ctx, entry)
}

var errAlreadyDecided = errors.New("org move request already decided")

func alreadyDecided() *domain.AppError {
	return domain.NewAppError(domain.ErrCodeConflict, "org move request already decided", map[string]string{"deny_code": "org_move_request_already_decided"})
}

func infraError(message string, err error) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInternalError, message, map[string]string{"cause": err.Error()})
}

func scopeDenied(code, message string) *domain.AppError {
	return domain.NewAppError(domain.ErrCodePermissionDenied, message, map[string]string{"deny_code": code})
}

func hasAnyRole(actor domain.RequestActor, roles ...domain.Role) bool {
	for _, role := range actor.Roles {
		for _, candidate := range roles {
			if role == candidate {
				return true
			}
		}
	}
	return false
}

func actorManagesDepartment(actor domain.RequestActor, department string) bool {
	for _, managed := range actor.ManagedDepartments {
		if strings.EqualFold(strings.TrimSpace(managed), strings.TrimSpace(department)) {
			return true
		}
	}
	return strings.EqualFold(strings.TrimSpace(actor.Department), strings.TrimSpace(department))
}

func pagination(page, pageSize int, total int64) domain.PaginationMeta {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return domain.PaginationMeta{Page: page, PageSize: pageSize, Total: total}
}
