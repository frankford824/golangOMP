package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type AccessPolicyRepository interface {
	ListPermissions(ctx context.Context) ([]domain.AccessPermission, error)
	ListRoles(ctx context.Context, includeArchived bool) ([]domain.AccessRole, error)
	GetRole(ctx context.Context, id int64) (*domain.AccessRole, error)
	GetPolicyRevision(ctx context.Context) (int64, error)
	BumpPolicyRevision(ctx context.Context, tx repo.Tx, expected int64, actorID int64, action, targetType, targetID, reason string, before, after interface{}) (int64, error)
	CreateRole(ctx context.Context, tx repo.Tx, role *domain.AccessRole, actorID int64) (int64, error)
	UpdateRole(ctx context.Context, tx repo.Tx, role *domain.AccessRole, expectedVersion, actorID int64) (bool, error)
	ArchiveRole(ctx context.Context, tx repo.Tx, roleID, expectedVersion, actorID int64) (bool, error)
	ReplaceRolePermissions(ctx context.Context, tx repo.Tx, roleID int64, permissions []domain.PermissionCode) error
	ReplaceUserAssignments(ctx context.Context, tx repo.Tx, userID, actorID int64, assignments []domain.AccessAssignment) error
	EffectiveAccess(ctx context.Context, userID int64) (*domain.EffectiveAccess, error)
	CountUsersWithRoleCode(ctx context.Context, roleCode string) (int64, error)
	GetOrgPolicy(ctx context.Context, subjectType domain.AccessSubjectType, subjectID int64) ([]domain.AccessOrgPolicy, error)
	ReplaceOrgPolicies(ctx context.Context, tx repo.Tx, subjectType domain.AccessSubjectType, subjectID, actorID int64, policies []domain.AccessOrgPolicy) error
	ListPolicyEvents(ctx context.Context, limit int) ([]domain.AccessPolicyEvent, error)
}

type AccessPolicyService interface {
	ListPermissions(ctx context.Context) ([]domain.AccessPermission, *domain.AppError)
	ListRoles(ctx context.Context, includeArchived bool) ([]domain.AccessRole, *domain.AppError)
	EffectiveAccess(ctx context.Context, userID int64) (*domain.EffectiveAccess, *domain.AppError)
	CreateRole(ctx context.Context, actor domain.RequestActor, request AccessRoleCreateRequest) (*AccessPolicyMutationResult, *domain.AppError)
	UpdateRole(ctx context.Context, actor domain.RequestActor, roleID int64, request AccessRoleUpdateRequest) (*AccessPolicyMutationResult, *domain.AppError)
	ArchiveRole(ctx context.Context, actor domain.RequestActor, roleID int64, request AccessRoleArchiveRequest) (*AccessPolicyMutationResult, *domain.AppError)
	ReplaceRolePermissions(ctx context.Context, actor domain.RequestActor, roleID int64, request ReplaceRolePermissionsRequest) (*AccessPolicyMutationResult, *domain.AppError)
	ReplaceUserAssignments(ctx context.Context, actor domain.RequestActor, userID int64, request domain.ReplaceAccessAssignmentsRequest) (*AccessPolicyMutationResult, *domain.AppError)
	GetOrgPolicies(ctx context.Context, subjectType domain.AccessSubjectType, subjectID int64) ([]domain.AccessOrgPolicy, *domain.AppError)
	ReplaceOrgPolicies(ctx context.Context, actor domain.RequestActor, subjectType domain.AccessSubjectType, subjectID int64, request ReplaceOrgPoliciesRequest) (*AccessPolicyMutationResult, *domain.AppError)
	ListEvents(ctx context.Context, limit int) ([]domain.AccessPolicyEvent, *domain.AppError)
}

type AccessRoleCreateRequest struct {
	Code                   string                  `json:"code"`
	Name                   string                  `json:"name"`
	Description            string                  `json:"description"`
	Permissions            []domain.PermissionCode `json:"permissions"`
	Reason                 string                  `json:"reason"`
	ExpectedPolicyRevision int64                   `json:"expected_policy_revision"`
}

type AccessRoleUpdateRequest struct {
	Name                   string `json:"name"`
	Description            string `json:"description"`
	ExpectedVersion        int64  `json:"expected_version"`
	Reason                 string `json:"reason"`
	ExpectedPolicyRevision int64  `json:"expected_policy_revision"`
}

type AccessRoleArchiveRequest struct {
	ExpectedVersion        int64  `json:"expected_version"`
	Reason                 string `json:"reason"`
	ExpectedPolicyRevision int64  `json:"expected_policy_revision"`
}

type ReplaceRolePermissionsRequest struct {
	Permissions            []domain.PermissionCode `json:"permissions"`
	ExpectedRoleVersion    int64                   `json:"expected_role_version"`
	Reason                 string                  `json:"reason"`
	ExpectedPolicyRevision int64                   `json:"expected_policy_revision"`
}

type ReplaceOrgPoliciesRequest struct {
	Policies               []domain.AccessOrgPolicy `json:"policies"`
	Reason                 string                   `json:"reason"`
	ExpectedPolicyRevision int64                    `json:"expected_policy_revision"`
}

type AccessPolicyMutationResult struct {
	PolicyRevision int64                    `json:"policy_revision"`
	Role           *domain.AccessRole       `json:"role,omitempty"`
	Effective      *domain.EffectiveAccess  `json:"effective,omitempty"`
	OrgPolicies    []domain.AccessOrgPolicy `json:"org_policies,omitempty"`
}

type accessPolicyService struct {
	repo     AccessPolicyRepository
	txRunner repo.TxRunner
	orgRepo  repo.OrgRepo
}

func NewAccessPolicyService(repository AccessPolicyRepository, txRunner repo.TxRunner, orgRepo repo.OrgRepo) AccessPolicyService {
	return &accessPolicyService{repo: repository, txRunner: txRunner, orgRepo: orgRepo}
}

func (s *accessPolicyService) ListPermissions(ctx context.Context) ([]domain.AccessPermission, *domain.AppError) {
	items, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return nil, infraError("list access permissions", err)
	}
	return items, nil
}

func (s *accessPolicyService) ListRoles(ctx context.Context, includeArchived bool) ([]domain.AccessRole, *domain.AppError) {
	items, err := s.repo.ListRoles(ctx, includeArchived)
	if err != nil {
		return nil, infraError("list access roles", err)
	}
	return items, nil
}

func (s *accessPolicyService) EffectiveAccess(ctx context.Context, userID int64) (*domain.EffectiveAccess, *domain.AppError) {
	if userID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id must be positive", nil)
	}
	view, err := s.repo.EffectiveAccess(ctx, userID)
	if err != nil {
		return nil, infraError("resolve effective access", err)
	}
	return view, nil
}

func (s *accessPolicyService) CreateRole(ctx context.Context, actor domain.RequestActor, request AccessRoleCreateRequest) (*AccessPolicyMutationResult, *domain.AppError) {
	if appErr := s.requireManage(ctx, actor); appErr != nil {
		return nil, appErr
	}
	if !roleCodePattern.MatchString(strings.TrimSpace(request.Code)) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "code must match [a-z0-9_]{2,64}", nil)
	}
	if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Reason) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "name and reason are required", nil)
	}
	if appErr := s.authorizePermissionGrant(ctx, actor, request.Permissions); appErr != nil {
		return nil, appErr
	}
	role := &domain.AccessRole{Code: strings.TrimSpace(request.Code), Name: strings.TrimSpace(request.Name), Description: strings.TrimSpace(request.Description), Permissions: normalizePermissionCodes(request.Permissions)}
	var next int64
	var roleID int64
	err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		roleID, err = s.repo.CreateRole(ctx, tx, role, actor.ID)
		if err != nil {
			return err
		}
		if err := s.repo.ReplaceRolePermissions(ctx, tx, roleID, role.Permissions); err != nil {
			return err
		}
		next, err = s.repo.BumpPolicyRevision(ctx, tx, request.ExpectedPolicyRevision, actor.ID, "role.created", "role", strconv.FormatInt(roleID, 10), request.Reason, nil, role)
		return err
	})
	if err != nil {
		return nil, mapAccessMutationError(err)
	}
	created, err := s.repo.GetRole(ctx, roleID)
	if err != nil {
		return nil, infraError("reload created access role", err)
	}
	return &AccessPolicyMutationResult{PolicyRevision: next, Role: created}, nil
}

func (s *accessPolicyService) UpdateRole(ctx context.Context, actor domain.RequestActor, roleID int64, request AccessRoleUpdateRequest) (*AccessPolicyMutationResult, *domain.AppError) {
	if appErr := s.requireManage(ctx, actor); appErr != nil {
		return nil, appErr
	}
	if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Reason) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "name and reason are required", nil)
	}
	before, err := s.repo.GetRole(ctx, roleID)
	if err != nil {
		return nil, mapAccessReadError(err)
	}
	after := *before
	after.Name = strings.TrimSpace(request.Name)
	after.Description = strings.TrimSpace(request.Description)
	var next int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		updated, err := s.repo.UpdateRole(ctx, tx, &after, request.ExpectedVersion, actor.ID)
		if err != nil {
			return err
		}
		if !updated {
			return repo.ErrConflict
		}
		next, err = s.repo.BumpPolicyRevision(ctx, tx, request.ExpectedPolicyRevision, actor.ID, "role.updated", "role", strconv.FormatInt(roleID, 10), request.Reason, before, &after)
		return err
	})
	if txErr != nil {
		return nil, mapAccessMutationError(txErr)
	}
	updated, err := s.repo.GetRole(ctx, roleID)
	if err != nil {
		return nil, infraError("reload updated access role", err)
	}
	return &AccessPolicyMutationResult{PolicyRevision: next, Role: updated}, nil
}

func (s *accessPolicyService) ArchiveRole(ctx context.Context, actor domain.RequestActor, roleID int64, request AccessRoleArchiveRequest) (*AccessPolicyMutationResult, *domain.AppError) {
	if appErr := s.requireManage(ctx, actor); appErr != nil {
		return nil, appErr
	}
	if strings.TrimSpace(request.Reason) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "reason is required", nil)
	}
	before, err := s.repo.GetRole(ctx, roleID)
	if err != nil {
		return nil, mapAccessReadError(err)
	}
	if before.SystemProtected {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "system-protected roles cannot be archived", nil)
	}
	var next int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		archived, err := s.repo.ArchiveRole(ctx, tx, roleID, request.ExpectedVersion, actor.ID)
		if err != nil {
			return err
		}
		if !archived {
			return repo.ErrConflict
		}
		next, err = s.repo.BumpPolicyRevision(ctx, tx, request.ExpectedPolicyRevision, actor.ID, "role.archived", "role", strconv.FormatInt(roleID, 10), request.Reason, before, nil)
		return err
	})
	if txErr != nil {
		return nil, mapAccessMutationError(txErr)
	}
	return &AccessPolicyMutationResult{PolicyRevision: next}, nil
}

func (s *accessPolicyService) ReplaceRolePermissions(ctx context.Context, actor domain.RequestActor, roleID int64, request ReplaceRolePermissionsRequest) (*AccessPolicyMutationResult, *domain.AppError) {
	if appErr := s.requireManage(ctx, actor); appErr != nil {
		return nil, appErr
	}
	if strings.TrimSpace(request.Reason) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "reason is required", nil)
	}
	permissions := normalizePermissionCodes(request.Permissions)
	if appErr := s.authorizePermissionGrant(ctx, actor, permissions); appErr != nil {
		return nil, appErr
	}
	before, err := s.repo.GetRole(ctx, roleID)
	if err != nil {
		return nil, mapAccessReadError(err)
	}
	after := *before
	after.Permissions = permissions
	var next int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		updated, err := s.repo.UpdateRole(ctx, tx, &after, request.ExpectedRoleVersion, actor.ID)
		if err != nil {
			return err
		}
		if !updated {
			return repo.ErrConflict
		}
		if err := s.repo.ReplaceRolePermissions(ctx, tx, roleID, permissions); err != nil {
			return err
		}
		next, err = s.repo.BumpPolicyRevision(ctx, tx, request.ExpectedPolicyRevision, actor.ID, "role.permissions_replaced", "role", strconv.FormatInt(roleID, 10), request.Reason, before, &after)
		return err
	})
	if txErr != nil {
		return nil, mapAccessMutationError(txErr)
	}
	updated, err := s.repo.GetRole(ctx, roleID)
	if err != nil {
		return nil, infraError("reload role permissions", err)
	}
	return &AccessPolicyMutationResult{PolicyRevision: next, Role: updated}, nil
}

func (s *accessPolicyService) ReplaceUserAssignments(ctx context.Context, actor domain.RequestActor, userID int64, request domain.ReplaceAccessAssignmentsRequest) (*AccessPolicyMutationResult, *domain.AppError) {
	if appErr := s.requireManage(ctx, actor); appErr != nil {
		return nil, appErr
	}
	if strings.TrimSpace(request.Reason) == "" || userID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id and reason are required", nil)
	}
	if appErr := s.validateAssignments(ctx, actor, userID, request.Assignments); appErr != nil {
		return nil, appErr
	}
	before, appErr := s.EffectiveAccess(ctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	removesLastSuperAdmin := effectiveHasRole(before, "super_admin") && !assignmentsHaveRoleCode(ctx, s.repo, request.Assignments, "super_admin")
	if removesLastSuperAdmin {
		count, err := s.repo.CountUsersWithRoleCode(ctx, "super_admin")
		if err != nil {
			return nil, infraError("count super administrators", err)
		}
		if count <= 1 {
			return nil, domain.NewAppError(domain.ErrCodeConflict, "the last SuperAdmin assignment cannot be removed", nil)
		}
	}
	var next int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.repo.ReplaceUserAssignments(ctx, tx, userID, actor.ID, request.Assignments); err != nil {
			return err
		}
		var err error
		next, err = s.repo.BumpPolicyRevision(ctx, tx, request.ExpectedPolicyRevision, actor.ID, "user.assignments_replaced", "user", strconv.FormatInt(userID, 10), request.Reason, before.Assignments, request.Assignments)
		return err
	})
	if txErr != nil {
		return nil, mapAccessMutationError(txErr)
	}
	after, appErr := s.EffectiveAccess(ctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	after.PolicyRevision = next
	return &AccessPolicyMutationResult{PolicyRevision: next, Effective: after}, nil
}

func (s *accessPolicyService) GetOrgPolicies(ctx context.Context, subjectType domain.AccessSubjectType, subjectID int64) ([]domain.AccessOrgPolicy, *domain.AppError) {
	if appErr := s.validateOrgSubject(ctx, subjectType, subjectID); appErr != nil {
		return nil, appErr
	}
	items, err := s.repo.GetOrgPolicy(ctx, subjectType, subjectID)
	if err != nil {
		return nil, infraError("list organization access policies", err)
	}
	return items, nil
}

func (s *accessPolicyService) ReplaceOrgPolicies(ctx context.Context, actor domain.RequestActor, subjectType domain.AccessSubjectType, subjectID int64, request ReplaceOrgPoliciesRequest) (*AccessPolicyMutationResult, *domain.AppError) {
	if appErr := s.requireManage(ctx, actor); appErr != nil {
		return nil, appErr
	}
	if strings.TrimSpace(request.Reason) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "reason is required", nil)
	}
	if appErr := s.validateOrgSubject(ctx, subjectType, subjectID); appErr != nil {
		return nil, appErr
	}
	for i := range request.Policies {
		request.Policies[i].SubjectType = subjectType
		request.Policies[i].SubjectID = subjectID
		if !request.Policies[i].ScopeMode.Valid() {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid organization policy scope_mode", map[string]interface{}{"index": i})
		}
		role, err := s.repo.GetRole(ctx, request.Policies[i].RoleID)
		if err != nil || role.ArchivedAt != nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "organization policy role does not exist or is archived", map[string]interface{}{"index": i, "role_id": request.Policies[i].RoleID})
		}
		if appErr := s.authorizePermissionGrant(ctx, actor, role.Permissions); appErr != nil {
			return nil, appErr
		}
	}
	before, appErr := s.GetOrgPolicies(ctx, subjectType, subjectID)
	if appErr != nil {
		return nil, appErr
	}
	var next int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.repo.ReplaceOrgPolicies(ctx, tx, subjectType, subjectID, actor.ID, request.Policies); err != nil {
			return err
		}
		var err error
		next, err = s.repo.BumpPolicyRevision(ctx, tx, request.ExpectedPolicyRevision, actor.ID, "org.policies_replaced", "org", fmt.Sprintf("%s:%d", subjectType, subjectID), request.Reason, before, request.Policies)
		return err
	})
	if txErr != nil {
		return nil, mapAccessMutationError(txErr)
	}
	after, appErr := s.GetOrgPolicies(ctx, subjectType, subjectID)
	if appErr != nil {
		return nil, appErr
	}
	return &AccessPolicyMutationResult{PolicyRevision: next, OrgPolicies: after}, nil
}

func (s *accessPolicyService) ListEvents(ctx context.Context, limit int) ([]domain.AccessPolicyEvent, *domain.AppError) {
	items, err := s.repo.ListPolicyEvents(ctx, limit)
	if err != nil {
		return nil, infraError("list access policy events", err)
	}
	return items, nil
}

var roleCodePattern = regexp.MustCompile(`^[a-z0-9_]{2,64}$`)

func (s *accessPolicyService) requireManage(ctx context.Context, actor domain.RequestActor) *domain.AppError {
	if actor.ID <= 0 {
		return domain.ErrUnauthorized
	}
	effective, err := s.actorEffectiveAccess(ctx, actor)
	if err != nil {
		return infraError("authorize access policy management", err)
	}
	if !effective.Has(domain.PermissionAccessPolicyAdmin) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "access_policy.manage is required", nil)
	}
	return nil
}

func (s *accessPolicyService) actorIsSuperAdmin(ctx context.Context, actor domain.RequestActor) bool {
	effective, err := s.actorEffectiveAccess(ctx, actor)
	if err != nil || effective == nil {
		return false
	}
	for _, assignment := range effective.Assignments {
		if assignment.RoleCode != "super_admin" {
			continue
		}
		role, err := s.repo.GetRole(ctx, assignment.RoleID)
		if err == nil && role.Code == "super_admin" && role.SystemProtected && role.ArchivedAt == nil {
			return true
		}
	}
	return false
}

func (s *accessPolicyService) actorEffectiveAccess(ctx context.Context, actor domain.RequestActor) (*domain.EffectiveAccess, error) {
	if actor.EffectiveAccess != nil && actor.EffectiveAccess.UserID == actor.ID {
		return actor.EffectiveAccess, nil
	}
	return s.repo.EffectiveAccess(ctx, actor.ID)
}

func (s *accessPolicyService) authorizePermissionGrant(ctx context.Context, actor domain.RequestActor, permissions []domain.PermissionCode) *domain.AppError {
	if s.actorIsSuperAdmin(ctx, actor) {
		return nil
	}
	catalog, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return infraError("load permission risk catalog", err)
	}
	riskByCode := make(map[domain.PermissionCode]string, len(catalog))
	for _, item := range catalog {
		riskByCode[item.Code] = item.RiskLevel
	}
	effective, err := s.actorEffectiveAccess(ctx, actor)
	if err != nil {
		return infraError("resolve actor permissions", err)
	}
	for _, permission := range normalizePermissionCodes(permissions) {
		if riskByCode[permission] == "high" {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "high-risk permissions may only be granted by SuperAdmin", map[string]interface{}{"permission": permission})
		}
		if !effective.Has(permission) {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "cannot grant a permission the actor does not hold", map[string]interface{}{"permission": permission})
		}
	}
	return nil
}

func (s *accessPolicyService) validateAssignments(ctx context.Context, actor domain.RequestActor, userID int64, assignments []domain.AccessAssignment) *domain.AppError {
	seen := map[int64]struct{}{}
	for i, assignment := range assignments {
		if assignment.RoleID <= 0 || !assignment.ScopeMode.Valid() {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid role_id or scope_mode", map[string]interface{}{"index": i})
		}
		if _, ok := seen[assignment.RoleID]; ok {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "duplicate role assignment", map[string]interface{}{"role_id": assignment.RoleID})
		}
		seen[assignment.RoleID] = struct{}{}
		if assignment.ScopeMode == domain.AccessScopeSelectedOrg && len(assignment.Subjects) == 0 {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "selected_org requires subjects", map[string]interface{}{"index": i})
		}
		if assignment.ScopeMode != domain.AccessScopeSelectedOrg && len(assignment.Subjects) > 0 {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "subjects are only allowed for selected_org", map[string]interface{}{"index": i})
		}
		for _, subject := range assignment.Subjects {
			if appErr := s.validateOrgSubject(ctx, subject.SubjectType, subject.SubjectID); appErr != nil {
				return appErr
			}
		}
		role, err := s.repo.GetRole(ctx, assignment.RoleID)
		if err != nil || role.ArchivedAt != nil {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "assigned role does not exist or is archived", map[string]interface{}{"role_id": assignment.RoleID})
		}
		if appErr := s.authorizePermissionGrant(ctx, actor, role.Permissions); appErr != nil {
			return appErr
		}
		if role.Code == "super_admin" {
			if !role.SystemProtected {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "super_admin must be the protected system role", nil)
			}
			if !s.actorIsSuperAdmin(ctx, actor) {
				if actor.ID == userID {
					return domain.NewAppError(domain.ErrCodePermissionDenied, "self-escalation to SuperAdmin is forbidden", nil)
				}
				return domain.NewAppError(domain.ErrCodePermissionDenied, "only a protected SuperAdmin may assign the SuperAdmin role", nil)
			}
		}
	}
	return nil
}

func (s *accessPolicyService) validateOrgSubject(ctx context.Context, subjectType domain.AccessSubjectType, subjectID int64) *domain.AppError {
	if !subjectType.Valid() || subjectID <= 0 || s.orgRepo == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "valid department/team subject is required", nil)
	}
	var err error
	if subjectType == domain.AccessSubjectDepartment {
		_, err = s.orgRepo.GetDepartmentByID(ctx, subjectID)
	} else {
		_, err = s.orgRepo.GetTeamByID(ctx, subjectID)
	}
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "organization subject does not exist", map[string]interface{}{"subject_type": subjectType, "subject_id": subjectID})
	}
	return nil
}

func normalizePermissionCodes(items []domain.PermissionCode) []domain.PermissionCode {
	seen := map[domain.PermissionCode]struct{}{}
	out := make([]domain.PermissionCode, 0, len(items))
	for _, item := range items {
		item = domain.PermissionCode(strings.TrimSpace(string(item)))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func effectiveHasRole(view *domain.EffectiveAccess, roleCode string) bool {
	if view == nil {
		return false
	}
	for _, assignment := range view.Assignments {
		if assignment.RoleCode == roleCode {
			return true
		}
	}
	return false
}

func assignmentsHaveRoleCode(ctx context.Context, repository AccessPolicyRepository, assignments []domain.AccessAssignment, roleCode string) bool {
	for _, assignment := range assignments {
		role, err := repository.GetRole(ctx, assignment.RoleID)
		if err == nil && role.Code == roleCode {
			return true
		}
	}
	return false
}

func mapAccessMutationError(err error) *domain.AppError {
	if errors.Is(err, repo.ErrConflict) {
		return domain.NewAppError(domain.ErrCodeConflict, "access policy revision or entity version is stale", nil)
	}
	return infraError("mutate access policy", err)
}

func mapAccessReadError(err error) *domain.AppError {
	if errors.Is(err, repo.ErrNotFound) {
		return domain.ErrNotFound
	}
	return infraError("read access policy", err)
}
