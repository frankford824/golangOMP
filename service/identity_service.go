package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"workflow/domain"
	"workflow/repo"
)

var (
	mobilePattern           = regexp.MustCompile(`^1[3-9]\d{9}$`)
	emailPattern            = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	managedAvatarURLPattern = regexp.MustCompile(`^/v1/me/avatar-files/avatar-[0-9a-f]{32}\.(jpg|jpeg|png|webp)$`)
)

const maxEmployeeNo = 9999

type RegisterUserParams struct {
	Username    string
	DisplayName string
	Department  domain.Department
	Team        string
	Mobile      string
	Email       string
	Password    string
}

type RegisterAssetWorkbenchUserParams struct {
	Username    string
	DisplayName string
	Mobile      string
	Email       string
	Password    string
}

type LoginParams struct {
	Username string
	Password string
}

type ChangePasswordParams struct {
	OldPassword string
	NewPassword string
	Confirm     string
}

type CreateManagedUserParams struct {
	Username           string
	EmployeeNo         *int
	DisplayName        string
	Department         domain.Department
	DepartmentID       *int64
	Team               string
	TeamID             *int64
	Mobile             string
	Email              string
	Password           string
	Status             *domain.UserStatus
	EmploymentType     *domain.EmploymentType
	ManagedDepartments *[]string
}

type ResetUserPasswordParams struct {
	UserID      int64
	NewPassword string
}

type UserFilter struct {
	Keyword    string
	Status     *domain.UserStatus
	Role       *domain.Role
	Department *domain.Department
	Team       string
	Page       int
	PageSize   int
}

// AssignableLane is intentionally distinct from domain.WorkflowLane. This
// value is a role-filter knob for an admin candidate-pool API, not a task's
// persisted workflow lane, so keeping it service-local avoids coupling
// HR/user APIs to task workflow semantics.
type AssignableLane string

const (
	AssignableLaneNormal        AssignableLane = "normal"
	AssignableLaneCustomization AssignableLane = "customization"
	AssignableLaneAudit         AssignableLane = "audit"
	AssignableLaneAll           AssignableLane = "all"
)

type UpdateUserParams struct {
	UserID             int64
	EmployeeNo         *int
	DisplayName        *string
	Status             *domain.UserStatus
	EmploymentType     *domain.EmploymentType
	Department         *domain.Department
	DepartmentID       *int64
	Team               *string
	TeamID             *int64
	Mobile             *string
	Email              *string
	ManagedDepartments *[]string
	ManagedTeams       *[]string
}

type UpdateMeParams struct {
	DisplayName *string
	Mobile      *string
	Email       *string
}

type UpdateMyAvatarParams struct {
	AvatarURL string
	Method    string
}

type PermissionLogFilter struct {
	ActorID        *int64
	ActorUsername  string
	ActionType     string
	TargetUserID   *int64
	TargetUsername string
	Granted        *bool
	Method         string
	RoutePath      string
	Page           int
	PageSize       int
}

type IdentityService interface {
	SyncConfiguredAuth(ctx context.Context) *domain.AppError
	GetRegistrationOptions(ctx context.Context) (*domain.RegistrationOptions, *domain.AppError)
	GetOrgOptions(ctx context.Context) (*domain.OrgOptions, *domain.AppError)
	GetOrgOptionsIncludingDisabled(ctx context.Context) (*domain.OrgOptions, *domain.AppError)
	CreateDepartment(ctx context.Context, p CreateOrgDepartmentParams) (*domain.OrgDepartment, *domain.AppError)
	UpdateDepartment(ctx context.Context, p UpdateOrgDepartmentParams) (*domain.OrgDepartment, *domain.AppError)
	CreateTeam(ctx context.Context, p CreateOrgTeamParams) (*domain.OrgTeam, *domain.AppError)
	UpdateTeam(ctx context.Context, p UpdateOrgTeamParams) (*domain.OrgTeam, *domain.AppError)
	MergeDepartment(ctx context.Context, p MergeOrgDepartmentParams) (*domain.OrgDepartment, *domain.AppError)
	MergeTeam(ctx context.Context, p MergeOrgTeamParams) (*domain.OrgTeam, *domain.AppError)
	DeleteDepartment(ctx context.Context, id int64) *domain.AppError
	DeleteTeam(ctx context.Context, id int64) *domain.AppError
	Register(ctx context.Context, p RegisterUserParams) (*domain.AuthResult, *domain.AppError)
	RegisterAssetWorkbenchUser(ctx context.Context, p RegisterAssetWorkbenchUserParams) (*domain.AuthResult, *domain.AppError)
	Login(ctx context.Context, p LoginParams) (*domain.AuthResult, *domain.AppError)
	ChangePassword(ctx context.Context, p ChangePasswordParams) *domain.AppError
	CreateManagedUser(ctx context.Context, p CreateManagedUserParams) (*domain.User, *domain.AppError)
	ResetUserPassword(ctx context.Context, p ResetUserPasswordParams) (*domain.User, *domain.AppError)
	GetCurrentUser(ctx context.Context) (*domain.User, *domain.AppError)
	GetMe(ctx context.Context) (*domain.User, *domain.AppError)
	UpdateMe(ctx context.Context, p UpdateMeParams) (*domain.User, *domain.AppError)
	UpdateMyAvatar(ctx context.Context, p UpdateMyAvatarParams) (*domain.User, *domain.AppError)
	GetMyOrg(ctx context.Context) (*domain.MyOrgProfile, *domain.AppError)
	ListUsers(ctx context.Context, filter UserFilter) ([]*domain.User, domain.PaginationMeta, *domain.AppError)
	// ListAccessPolicyUsers is a capability-guarded selector for the explicit
	// access-policy UI. It intentionally does not infer authorization from
	// legacy roles; the /v1/access/users route guard is the sole authorization
	// boundary and the handler returns only minimal identity fields.
	ListAccessPolicyUsers(ctx context.Context, filter UserFilter) ([]*domain.User, domain.PaginationMeta, *domain.AppError)
	// ListAssignableDesigners returns active task candidates projected from the
	// explicit auth_* policy model. Legacy user_roles never select candidates.
	ListAssignableDesigners(ctx context.Context, actor *domain.RequestActor, lane AssignableLane) ([]*domain.User, *domain.AppError)
	GetUser(ctx context.Context, userID int64) (*domain.User, *domain.AppError)
	UpdateUser(ctx context.Context, p UpdateUserParams) (*domain.User, *domain.AppError)
	ActivateUser(ctx context.Context, userID int64) *domain.AppError
	DeactivateUser(ctx context.Context, userID int64) *domain.AppError
	ListPermissionLogs(ctx context.Context, filter PermissionLogFilter) ([]*domain.PermissionLog, domain.PaginationMeta, *domain.AppError)
	ResolveRequestActor(ctx context.Context, bearerToken string) (*domain.RequestActor, *domain.AppError)
	RecordRouteAccess(ctx context.Context, entry domain.PermissionLog)
}

type IdentityServiceOption func(*identityService)

// IdentityAccessAssignmentWriter persists the explicit v8 role assignments
// that must be created atomically with a user. Legacy user_roles are retained
// only as identity/display compatibility and are not an authorization source.
type IdentityAccessAssignmentWriter interface {
	EnsureExplicitRoleAssignment(ctx context.Context, tx repo.Tx, userID int64, roleCode string, scopeMode domain.AccessScopeMode) error
}

// IdentityEffectiveAccessReader projects the explicit v8 capability model into
// frontend_access so business pages no longer need a parallel authorization track.
type IdentityEffectiveAccessReader interface {
	EffectiveAccess(ctx context.Context, userID int64) (*domain.EffectiveAccess, error)
}

type IdentityEffectiveAccessBatchReader interface {
	EffectiveAccessMany(ctx context.Context, userIDs []int64) (map[int64]*domain.EffectiveAccess, error)
}

func WithIdentityAccessAssignmentWriter(writer IdentityAccessAssignmentWriter) IdentityServiceOption {
	return func(s *identityService) {
		s.accessAssignmentWriter = writer
	}
}

func WithIdentityEffectiveAccessReader(reader IdentityEffectiveAccessReader) IdentityServiceOption {
	return func(s *identityService) {
		s.effectiveAccessReader = reader
	}
}

func WithIdentitySettings(authSettings domain.AuthSettings, frontendAccessSettings domain.FrontendAccessSettings) IdentityServiceOption {
	return func(s *identityService) {
		s.authSettings = authSettings
		s.frontendAccessSettings = frontendAccessSettings
	}
}

// WithIdentityLogger injects the structured logger used for observability-only
// telemetry emitted by the identity service (actor role hydration and
// authorize_user_{read,list_filter}_denied default-deny paths). Supplying a
// nil logger is safe and is equivalent to not enabling telemetry.
func WithIdentityLogger(logger *zap.Logger) IdentityServiceOption {
	return func(s *identityService) {
		if logger == nil {
			s.logger = zap.NewNop()
			return
		}
		s.logger = logger
	}
}

type identityService struct {
	userRepo               repo.UserRepo
	orgRepo                repo.OrgRepo
	sessionRepo            repo.UserSessionRepo
	permissionLogRepo      repo.PermissionLogRepo
	accessAssignmentWriter IdentityAccessAssignmentWriter
	effectiveAccessReader  IdentityEffectiveAccessReader
	txRunner               repo.TxRunner
	sessionTTL             time.Duration
	authSettings           domain.AuthSettings
	frontendAccessSettings domain.FrontendAccessSettings
	logger                 *zap.Logger
	orgOptionsOnce         sync.Once
	orgOptionsCache        *domain.OrgOptions
}

type explicitIdentityRole struct {
	Code  string
	Scope domain.AccessScopeMode
}

func (s *identityService) ensureExplicitRoleAssignments(ctx context.Context, tx repo.Tx, userID int64, roles ...explicitIdentityRole) error {
	if s.accessAssignmentWriter == nil {
		// Tests and non-production embedders created before the v8 access cutover
		// may omit the optional writer. cmd/server always wires the concrete
		// access-policy repository and has a regression test for that contract.
		return nil
	}
	for _, role := range roles {
		if err := s.accessAssignmentWriter.EnsureExplicitRoleAssignment(ctx, tx, userID, role.Code, role.Scope); err != nil {
			return fmt.Errorf("ensure explicit identity role %s: %w", role.Code, err)
		}
	}
	return nil
}

func (s *identityService) resolveStableUserOrgIDs(ctx context.Context, departmentID, teamID *int64, department domain.Department, team string) (*int64, *int64, *domain.AppError) {
	if departmentID == nil && teamID == nil {
		return nil, nil, nil
	}
	if s.orgRepo == nil {
		return nil, nil, domain.NewAppError(domain.ErrCodeInternalError, "organization repository is unavailable", nil)
	}
	if departmentID == nil || *departmentID <= 0 {
		return nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "department_id is required when stable organization ids are supplied", nil)
	}
	departmentRow, err := s.orgRepo.GetDepartmentByID(ctx, *departmentID)
	if err != nil {
		return nil, nil, infraError("load user department by id", err)
	}
	if departmentRow == nil || !departmentRow.Enabled || (strings.TrimSpace(string(department)) != "" && departmentRow.Name != strings.TrimSpace(string(department))) {
		return nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "department_id does not match an enabled department", nil)
	}
	departmentValue := departmentRow.ID
	if strings.TrimSpace(team) == "" && teamID == nil {
		return &departmentValue, nil, nil
	}
	if teamID == nil || *teamID <= 0 {
		return nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "team_id is required when a team is supplied", nil)
	}
	teamRow, err := s.orgRepo.GetTeamByID(ctx, *teamID)
	if err != nil {
		return nil, nil, infraError("load user team by id", err)
	}
	if teamRow == nil || !teamRow.Enabled || teamRow.DepartmentID != departmentRow.ID || (strings.TrimSpace(team) != "" && teamRow.Name != strings.TrimSpace(team)) {
		return nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "team_id does not match the selected department and team", nil)
	}
	teamValue := teamRow.ID
	return &departmentValue, &teamValue, nil
}

func equalOptionalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// userRoleRawReader is an optional repo extension used by ResolveRequestActor
// to observe raw role strings before NormalizeRoles drops unknown entries.
// Implementations that do not support it keep the pre-existing behaviour;
// the resolver will still emit zero-known-roles telemetry from the normalized
// slice and will never fail a request based on this interface availability.
type userRoleRawReader interface {
	ListRolesRaw(ctx context.Context, userID int64) ([]string, error)
}

type sessionActorBundleReader interface {
	ResolveActorBundle(ctx context.Context, tokenHash string, at time.Time) (*domain.UserSession, *domain.User, []string, error)
}

const defaultSessionTTL = 24 * time.Hour

const userTeamUngroupedAlias = "ungrouped"

func NewIdentityService(
	userRepo repo.UserRepo,
	sessionRepo repo.UserSessionRepo,
	permissionLogRepo repo.PermissionLogRepo,
	txRunner repo.TxRunner,
	opts ...IdentityServiceOption,
) IdentityService {
	svc := &identityService{
		userRepo:               userRepo,
		sessionRepo:            sessionRepo,
		permissionLogRepo:      permissionLogRepo,
		txRunner:               txRunner,
		sessionTTL:             defaultSessionTTL,
		authSettings:           defaultAuthSettings(),
		frontendAccessSettings: defaultFrontendAccessSettings(),
		logger:                 zap.NewNop(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *identityService) SyncConfiguredAuth(ctx context.Context) *domain.AppError {
	if appErr := s.syncOrgMasterData(ctx); appErr != nil {
		return appErr
	}
	configured := make(map[string]domain.ConfiguredSuperAdmin, len(s.authSettings.SuperAdmins))
	for _, entry := range s.authSettings.SuperAdmins {
		entry = s.normalizeConfiguredSuperAdminOrg(entry)
		username := normalizeUsername(entry.Username)
		if username == "" {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "configured super admin username is required", nil)
		}
		if strings.TrimSpace(entry.Password) != "" {
			if appErr := s.validatePassword(entry.Password, "configured super admin password"); appErr != nil {
				return appErr
			}
		}
		if appErr := s.validateDepartment(entry.Department); appErr != nil {
			return appErr
		}
		if appErr := s.validateTeam(entry.Department, entry.Team); appErr != nil {
			return appErr
		}
		if appErr := validateMobile(entry.Mobile); appErr != nil {
			return appErr
		}
		if appErr := validateOptionalEmail(entry.Email); appErr != nil {
			return appErr
		}
		if _, appErr := s.resolveConfiguredSuperAdminRoles(entry); appErr != nil {
			return appErr
		}
		if _, appErr := s.resolveConfiguredSuperAdminManagedDepartments(entry); appErr != nil {
			return appErr
		}
		if _, appErr := s.resolveConfiguredSuperAdminManagedTeams(entry); appErr != nil {
			return appErr
		}
		if _, appErr := resolveConfiguredSuperAdminStatus(entry); appErr != nil {
			return appErr
		}
		if _, appErr := resolveConfiguredSuperAdminEmploymentType(entry); appErr != nil {
			return appErr
		}
		entry.Username = username
		configured[username] = entry
	}

	for _, entry := range configured {
		if appErr := s.upsertConfiguredSuperAdmin(ctx, entry); appErr != nil {
			return appErr
		}
	}

	currentManaged, err := s.userRepo.ListConfigManagedAdmins(ctx)
	if err != nil {
		return infraError("list config managed admins", err)
	}
	for _, user := range currentManaged {
		if user == nil {
			continue
		}
		if _, ok := configured[normalizeUsername(user.Username)]; ok {
			continue
		}
		roles, err := s.userRepo.ListRoles(ctx, user.ID)
		if err != nil {
			return infraError("list config managed admin roles", err)
		}
		nextRoles := removeRole(removeRole(roles, domain.RoleAdmin), domain.RoleSuperAdmin)
		if appErr := s.ensureAdminRoleSafety(ctx, roles, nextRoles); appErr != nil {
			return appErr
		}
		user.IsConfigSuperAdmin = false
		user.Roles = nextRoles
		user.UpdatedAt = time.Now().UTC()
		if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			if err := s.userRepo.Update(ctx, tx, user); err != nil {
				return err
			}
			return s.userRepo.ReplaceRoles(ctx, tx, user.ID, nextRoles)
		}); err != nil {
			return infraError("remove config managed super admin role", err)
		}
		s.recordSystemPermissionAction(ctx, domain.PermissionLog{
			ActionType:     domain.PermissionActionRoleRemoved,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    []domain.Role{domain.RoleAdmin, domain.RoleSuperAdmin},
			Granted:        true,
			Reason:         "config-managed super admin removed",
			Method:         "SYSTEM",
			RoutePath:      "config/auth_identity.json",
		})
	}
	return nil
}

func (s *identityService) GetRegistrationOptions(_ context.Context) (*domain.RegistrationOptions, *domain.AppError) {
	orgOptions, appErr := s.buildOrgOptions(context.Background(), false)
	if appErr != nil {
		return nil, appErr
	}
	options := &domain.RegistrationOptions{
		Departments: make([]domain.DepartmentOption, 0, len(orgOptions.Departments)),
	}
	for _, department := range orgOptions.Departments {
		options.Departments = append(options.Departments, domain.DepartmentOption{
			ID:        department.ID,
			Name:      department.Name,
			Teams:     append([]string{}, department.Teams...),
			TeamItems: append([]domain.OrgTeamOption{}, department.TeamItems...),
			Enabled:   department.Enabled,
		})
	}
	return options, nil
}

func (s *identityService) GetOrgOptions(ctx context.Context) (*domain.OrgOptions, *domain.AppError) {
	options, appErr := s.buildOrgOptions(ctx, false)
	if appErr != nil {
		return nil, appErr
	}
	return cloneOrgOptions(options), nil
}

func (s *identityService) GetOrgOptionsIncludingDisabled(ctx context.Context) (*domain.OrgOptions, *domain.AppError) {
	options, appErr := s.buildOrgOptions(ctx, true)
	if appErr != nil {
		return nil, appErr
	}
	return cloneOrgOptions(options), nil
}

func (s *identityService) Register(ctx context.Context, p RegisterUserParams) (*domain.AuthResult, *domain.AppError) {
	username := normalizeUsername(p.Username)
	displayName := strings.TrimSpace(p.DisplayName)
	team := strings.TrimSpace(p.Team)
	mobile := strings.TrimSpace(p.Mobile)
	email := strings.TrimSpace(p.Email)

	if username == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "account is required", nil)
	}
	if displayName == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "name is required", nil)
	}
	if appErr := s.validateDepartment(p.Department); appErr != nil {
		return nil, appErr
	}
	if appErr := s.validateTeam(p.Department, team); appErr != nil {
		return nil, appErr
	}
	if appErr := validateMobile(mobile); appErr != nil {
		return nil, appErr
	}
	if appErr := validateOptionalEmail(email); appErr != nil {
		return nil, appErr
	}
	if appErr := s.validatePassword(p.Password, "password"); appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureUniqueIdentity(ctx, username, mobile, 0); appErr != nil {
		return nil, appErr
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, infraError("hash password", err)
	}

	roles := []domain.Role{domain.RoleMember}
	now := time.Now().UTC()
	user := &domain.User{
		Username:       username,
		DisplayName:    displayName,
		Department:     p.Department,
		Team:           team,
		Mobile:         mobile,
		Email:          email,
		PasswordHash:   string(hash),
		Status:         domain.UserStatusActive,
		EmploymentType: domain.EmploymentTypeFullTime,
		// Self-registration never establishes management scope. Administrators
		// must grant stable-ID scopes through the explicit access-policy API.
		ManagedDepartments: nil,
		LastLoginAt:        &now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	rawToken, tokenHash, err := generateSessionToken()
	if err != nil {
		return nil, infraError("generate session token during register", err)
	}

	session := &domain.UserSession{
		SessionID:  uuid.NewString(),
		TokenHash:  tokenHash,
		ExpiresAt:  now.Add(s.sessionTTL),
		LastSeenAt: &now,
		CreatedAt:  now,
	}

	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		userID, err := s.userRepo.Create(ctx, tx, user)
		if err != nil {
			return err
		}
		user.ID = userID
		if err := s.userRepo.ReplaceRoles(ctx, tx, userID, roles); err != nil {
			return err
		}
		if err := s.ensureExplicitRoleAssignments(ctx, tx, userID, explicitIdentityRole{Code: "member", Scope: domain.AccessScopeSelf}); err != nil {
			return err
		}
		session.UserID = userID
		if _, err := s.sessionRepo.Create(ctx, tx, session); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, infraError("register user tx", err)
	}

	user.Roles = domain.NormalizeRoleValues(roles)
	s.prepareUserForResponse(user)
	s.recordPermissionAction(ctx, domain.PermissionLog{
		ActionType:     domain.PermissionActionRegister,
		TargetUserID:   actorIDPtr(user.ID),
		TargetUsername: user.Username,
		TargetRoles:    user.Roles,
		Granted:        true,
		Reason:         "user registered with explicit member access",
		Method:         "POST",
		RoutePath:      "/v1/auth/register",
	})
	return &domain.AuthResult{
		User: user,
		Session: &domain.AuthSession{
			SessionID: session.SessionID,
			Token:     rawToken,
			TokenType: "Bearer",
			ExpiresAt: session.ExpiresAt,
		},
	}, nil
}

func (s *identityService) RegisterAssetWorkbenchUser(ctx context.Context, p RegisterAssetWorkbenchUserParams) (*domain.AuthResult, *domain.AppError) {
	username := normalizeUsername(p.Username)
	displayName := strings.TrimSpace(p.DisplayName)
	mobile := strings.TrimSpace(p.Mobile)
	email := strings.TrimSpace(p.Email)

	if username == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "account is required", nil)
	}
	if displayName == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "name is required", nil)
	}
	if appErr := validateMobile(mobile); appErr != nil {
		return nil, appErr
	}
	if appErr := validateOptionalEmail(email); appErr != nil {
		return nil, appErr
	}
	if appErr := s.validatePassword(p.Password, "password"); appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureUniqueIdentity(ctx, username, mobile, 0); appErr != nil {
		return nil, appErr
	}

	team, appErr := s.defaultUnassignedPoolTeam()
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.validateDepartment(domain.DepartmentUnassigned); appErr != nil {
		return nil, appErr
	}
	if appErr := s.validateTeam(domain.DepartmentUnassigned, team); appErr != nil {
		return nil, appErr
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, infraError("hash asset workbench password", err)
	}

	now := time.Now().UTC()
	user := &domain.User{
		Username:       username,
		DisplayName:    displayName,
		Department:     domain.DepartmentUnassigned,
		Team:           team,
		Mobile:         mobile,
		Email:          email,
		PasswordHash:   string(hash),
		Status:         domain.UserStatusActive,
		EmploymentType: domain.EmploymentTypePartTime,
		LastLoginAt:    &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	roles := []domain.Role{domain.RoleAssetSubmitter}

	rawToken, tokenHash, err := generateSessionToken()
	if err != nil {
		return nil, infraError("generate session token during asset workbench register", err)
	}
	session := &domain.UserSession{
		SessionID:  uuid.NewString(),
		TokenHash:  tokenHash,
		ExpiresAt:  now.Add(s.sessionTTL),
		LastSeenAt: &now,
		CreatedAt:  now,
	}

	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		userID, err := s.userRepo.Create(ctx, tx, user)
		if err != nil {
			return err
		}
		user.ID = userID
		if err := s.userRepo.ReplaceRoles(ctx, tx, userID, roles); err != nil {
			return err
		}
		if err := s.ensureExplicitRoleAssignments(ctx, tx, userID,
			explicitIdentityRole{Code: "member", Scope: domain.AccessScopeSelf},
			explicitIdentityRole{Code: "asset_submitter", Scope: domain.AccessScopeSelf},
		); err != nil {
			return err
		}
		session.UserID = userID
		if _, err := s.sessionRepo.Create(ctx, tx, session); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, infraError("register asset workbench user tx", err)
	}

	user.Roles = domain.NormalizeRoleValues(roles)
	s.prepareUserForResponse(user)
	s.recordPermissionAction(ctx, domain.PermissionLog{
		ActionType:     domain.PermissionActionRegister,
		TargetUserID:   actorIDPtr(user.ID),
		TargetUsername: user.Username,
		TargetRoles:    user.Roles,
		Granted:        true,
		Reason:         "asset workbench user self-registered",
		Method:         "POST",
		RoutePath:      "/v1/asset-workbench/register",
	})
	return &domain.AuthResult{
		User: user,
		Session: &domain.AuthSession{
			SessionID: session.SessionID,
			Token:     rawToken,
			TokenType: "Bearer",
			ExpiresAt: session.ExpiresAt,
		},
	}, nil
}

func (s *identityService) Login(ctx context.Context, p LoginParams) (*domain.AuthResult, *domain.AppError) {
	username := normalizeUsername(p.Username)
	if username == "" || strings.TrimSpace(p.Password) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "account and password are required", nil)
	}

	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, infraError("get user by username during login", err)
	}
	if user == nil {
		appErr := domain.NewAppError(domain.ErrCodeUnauthorized, "invalid account or password", nil)
		s.recordPermissionAction(ctx, domain.PermissionLog{
			ActionType:     domain.PermissionActionLoginFailed,
			ActorUsername:  username,
			TargetUsername: username,
			Granted:        false,
			Reason:         appErr.Message,
			Method:         "POST",
			RoutePath:      "/v1/auth/login",
		})
		return nil, appErr
	}
	if user.Status != domain.UserStatusActive {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "user is disabled", nil)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(p.Password)); err != nil {
		appErr := domain.NewAppError(domain.ErrCodeUnauthorized, "invalid account or password", nil)
		s.recordPermissionAction(ctx, domain.PermissionLog{
			ActionType:     domain.PermissionActionLoginFailed,
			ActorID:        actorIDPtr(user.ID),
			ActorUsername:  user.Username,
			ActorSource:    domain.RequestActorSourceSessionToken,
			AuthMode:       domain.AuthModeSessionTokenRoleEnforced,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			Granted:        false,
			Reason:         appErr.Message,
			Method:         "POST",
			RoutePath:      "/v1/auth/login",
		})
		return nil, appErr
	}

	now := time.Now().UTC()
	rawToken, tokenHash, err := generateSessionToken()
	if err != nil {
		return nil, infraError("generate session token during login", err)
	}
	session := &domain.UserSession{
		SessionID:  uuid.NewString(),
		UserID:     user.ID,
		TokenHash:  tokenHash,
		ExpiresAt:  now.Add(s.sessionTTL),
		LastSeenAt: &now,
		CreatedAt:  now,
	}

	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if _, err := s.sessionRepo.Create(ctx, tx, session); err != nil {
			return err
		}
		return s.userRepo.UpdateLastLogin(ctx, tx, user.ID, now)
	}); err != nil {
		return nil, infraError("login tx", err)
	}

	roles, err := s.userRepo.ListRoles(ctx, user.ID)
	if err != nil {
		return nil, infraError("list user roles during login", err)
	}
	user.Roles = roles
	user.LastLoginAt = &now
	s.prepareUserForResponse(user)
	s.recordPermissionAction(ctx, domain.PermissionLog{
		ActionType:     domain.PermissionActionLogin,
		ActorID:        actorIDPtr(user.ID),
		ActorUsername:  user.Username,
		ActorSource:    domain.RequestActorSourceSessionToken,
		AuthMode:       domain.AuthModeSessionTokenRoleEnforced,
		ActorRoles:     user.Roles,
		TargetUserID:   actorIDPtr(user.ID),
		TargetUsername: user.Username,
		TargetRoles:    user.Roles,
		Granted:        true,
		Reason:         "login succeeded",
		Method:         "POST",
		RoutePath:      "/v1/auth/login",
	})
	return &domain.AuthResult{
		User: user,
		Session: &domain.AuthSession{
			SessionID: session.SessionID,
			Token:     rawToken,
			TokenType: "Bearer",
			ExpiresAt: session.ExpiresAt,
		},
	}, nil
}

func (s *identityService) ChangePassword(ctx context.Context, p ChangePasswordParams) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || !domain.IsSessionBackedRequestActor(actor) {
		return domain.ErrUnauthorized
	}
	user, err := s.userRepo.GetByID(ctx, actor.ID)
	if err != nil {
		return infraError("get user for change password", err)
	}
	if user == nil {
		return domain.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(p.OldPassword)); err != nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "old password is incorrect", map[string]string{"deny_code": "old_password_mismatch"})
	}
	if strings.TrimSpace(p.Confirm) != "" && p.NewPassword != p.Confirm {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "new password confirmation does not match", map[string]string{"deny_code": "password_confirmation_mismatch"})
	}
	if appErr := s.validatePassword(p.NewPassword, "new password"); appErr != nil {
		return appErr
	}
	if p.NewPassword == p.OldPassword {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "new password must be different from old password", nil)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(p.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return infraError("hash new password", err)
	}
	now := time.Now().UTC()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.userRepo.UpdatePassword(ctx, tx, user.ID, string(hash), now)
	}); err != nil {
		return infraError("change password tx", err)
	}
	s.recordPermissionAction(ctx, domain.PermissionLog{
		ActionType:     domain.PermissionActionPasswordChanged,
		ActorID:        actorIDPtr(user.ID),
		ActorUsername:  user.Username,
		ActorSource:    domain.RequestActorSourceSessionToken,
		AuthMode:       domain.AuthModeSessionTokenRoleEnforced,
		TargetUserID:   actorIDPtr(user.ID),
		TargetUsername: user.Username,
		Granted:        true,
		Reason:         "password changed",
		Method:         "PUT",
		RoutePath:      "/v1/auth/password",
	})
	return nil
}

func (s *identityService) GetMe(ctx context.Context) (*domain.User, *domain.AppError) {
	return s.GetCurrentUser(ctx)
}

func (s *identityService) UpdateMe(ctx context.Context, p UpdateMeParams) (*domain.User, *domain.AppError) {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || !domain.IsSessionBackedRequestActor(actor) {
		return nil, domain.ErrUnauthorized
	}
	user, err := s.userRepo.GetByID(ctx, actor.ID)
	if err != nil {
		return nil, infraError("get current user for update", err)
	}
	if user == nil {
		return nil, domain.ErrNotFound
	}
	if err := s.attachRoles(ctx, user); err != nil {
		return nil, infraError("attach current user roles", err)
	}
	// Self profile edits are allowed for the account owner even without
	// organization-management roles; keep the writable set narrow.
	return s.updateUserBypassManagementScope(ctx, user, p, "PATCH", "/v1/me")
}

func (s *identityService) UpdateMyAvatar(ctx context.Context, p UpdateMyAvatarParams) (*domain.User, *domain.AppError) {
	avatarURL, appErr := normalizeManagedAvatarURLForService(p.AvatarURL)
	if appErr != nil {
		return nil, appErr
	}
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || !domain.IsSessionBackedRequestActor(actor) {
		return nil, domain.ErrUnauthorized
	}
	user, err := s.userRepo.GetByID(ctx, actor.ID)
	if err != nil {
		return nil, infraError("get current user for avatar update", err)
	}
	if user == nil {
		return nil, domain.ErrNotFound
	}
	if err := s.attachRoles(ctx, user); err != nil {
		return nil, infraError("attach current user roles", err)
	}
	if avatarURL == user.AvatarURL {
		s.prepareUserForResponse(user)
		return user, nil
	}
	user.AvatarURL = avatarURL
	user.UpdatedAt = time.Now().UTC()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.userRepo.Update(ctx, tx, user); err != nil {
			return err
		}
		method := strings.TrimSpace(p.Method)
		if method == "" {
			method = "POST"
		}
		return s.recordPermissionActionTx(ctx, tx, domain.PermissionLog{
			ActionType:     domain.PermissionActionUserUpdated,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    user.Roles,
			Granted:        true,
			Reason:         "updated fields: avatar_url",
			Method:         method,
			RoutePath:      "/v1/me/avatar",
		})
	}); err != nil {
		return nil, infraError("update current user avatar tx", err)
	}
	updated, appErr := s.GetCurrentUser(ctx)
	if appErr != nil {
		return nil, appErr
	}
	return updated, nil
}

func (s *identityService) GetMyOrg(ctx context.Context) (*domain.MyOrgProfile, *domain.AppError) {
	user, appErr := s.GetCurrentUser(ctx)
	if appErr != nil {
		return nil, appErr
	}
	return &domain.MyOrgProfile{
		Department:         string(user.Department),
		Team:               user.Team,
		ManagedDepartments: append([]string{}, user.ManagedDepartments...),
		ManagedTeams:       append([]string{}, user.ManagedTeams...),
		Roles:              append([]domain.Role{}, user.Roles...),
	}, nil
}

func (s *identityService) CreateManagedUser(ctx context.Context, p CreateManagedUserParams) (*domain.User, *domain.AppError) {
	username := normalizeUsername(p.Username)
	displayName := strings.TrimSpace(p.DisplayName)
	team := strings.TrimSpace(p.Team)
	mobile := strings.TrimSpace(p.Mobile)
	email := strings.TrimSpace(p.Email)
	employeeNo, appErr := requiredEmployeeNo(p.EmployeeNo)
	if appErr != nil {
		return nil, appErr
	}

	if username == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "username is required", nil)
	}
	if displayName == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "display_name is required", nil)
	}
	if appErr := s.validateDepartment(p.Department); appErr != nil {
		return nil, appErr
	}
	if appErr := s.validateTeam(p.Department, team); appErr != nil {
		return nil, appErr
	}
	departmentID, teamID, orgErr := s.resolveStableUserOrgIDs(ctx, p.DepartmentID, p.TeamID, p.Department, team)
	if orgErr != nil {
		return nil, orgErr
	}
	if appErr := validateMobile(mobile); appErr != nil {
		return nil, appErr
	}
	if appErr := validateOptionalEmail(email); appErr != nil {
		return nil, appErr
	}
	if appErr := s.validatePassword(p.Password, "password"); appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureUniqueIdentity(ctx, username, mobile, 0); appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureUniqueEmployeeNo(ctx, employeeNo, 0); appErr != nil {
		return nil, appErr
	}

	roles := []domain.Role{domain.RoleMember}
	if appErr := s.authorizeCreateManagedUser(ctx, &domain.User{
		Department:   p.Department,
		DepartmentID: departmentID,
		Team:         team,
		TeamID:       teamID,
	}); appErr != nil {
		return nil, appErr
	}
	managedDepartments, appErr := s.resolveCreateManagedDepartments(p.Department, roles, p.ManagedDepartments)
	if appErr != nil {
		return nil, appErr
	}

	status := domain.UserStatusActive
	if p.Status != nil {
		if !p.Status.Valid() {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "status must be active or disabled", nil)
		}
		status = *p.Status
	}
	employmentType := domain.EmploymentTypeFullTime
	if p.EmploymentType != nil {
		if !p.EmploymentType.Valid() {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "employment_type must be full_time or part_time", nil)
		}
		employmentType = *p.EmploymentType
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, infraError("hash managed user password", err)
	}

	now := time.Now().UTC()
	user := &domain.User{
		Username:           username,
		EmployeeNo:         &employeeNo,
		DisplayName:        displayName,
		Department:         p.Department,
		DepartmentID:       departmentID,
		Team:               team,
		TeamID:             teamID,
		Mobile:             mobile,
		Email:              email,
		PasswordHash:       string(hash),
		Status:             status,
		EmploymentType:     employmentType,
		ManagedDepartments: managedDepartments,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		userID, err := s.userRepo.Create(ctx, tx, user)
		if err != nil {
			return err
		}
		user.ID = userID
		if err := s.userRepo.ReplaceRoles(ctx, tx, userID, roles); err != nil {
			return err
		}
		return s.ensureExplicitRoleAssignments(ctx, tx, userID, explicitIdentityRole{Code: "member", Scope: domain.AccessScopeSelf})
	}); err != nil {
		if appErr := s.employeeNoWriteConflict(ctx, err, employeeNo, 0); appErr != nil {
			return nil, appErr
		}
		return nil, infraError("create managed user tx", err)
	}

	created, appErr := s.GetUser(ctx, user.ID)
	if appErr != nil {
		return nil, appErr
	}
	s.recordPermissionAction(ctx, domain.PermissionLog{
		ActionType:     domain.PermissionActionUserCreated,
		TargetUserID:   actorIDPtr(created.ID),
		TargetUsername: created.Username,
		TargetRoles:    created.Roles,
		Granted:        true,
		Reason:         "managed user created",
		Method:         "POST",
		RoutePath:      "/v1/users",
	})
	return created, nil
}

func (s *identityService) ResetUserPassword(ctx context.Context, p ResetUserPasswordParams) (*domain.User, *domain.AppError) {
	if p.UserID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id is required", nil)
	}
	if appErr := s.validatePassword(p.NewPassword, "password"); appErr != nil {
		return nil, appErr
	}
	user, appErr := s.GetUser(ctx, p.UserID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.authorizeResetUserPassword(ctx, user); appErr != nil {
		return nil, appErr
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(p.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, infraError("hash reset password", err)
	}
	now := time.Now().UTC()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.userRepo.UpdatePassword(ctx, tx, user.ID, string(hash), now)
	}); err != nil {
		return nil, infraError("reset user password tx", err)
	}
	updated, appErr := s.GetUser(ctx, user.ID)
	if appErr != nil {
		return nil, appErr
	}
	s.recordPermissionAction(ctx, domain.PermissionLog{
		ActionType:     domain.PermissionActionPasswordReset,
		TargetUserID:   actorIDPtr(updated.ID),
		TargetUsername: updated.Username,
		TargetRoles:    updated.Roles,
		Granted:        true,
		Reason:         "password reset by admin",
		Method:         "PUT",
		RoutePath:      "/v1/users/:id/password",
	})
	return updated, nil
}

func (s *identityService) GetCurrentUser(ctx context.Context) (*domain.User, *domain.AppError) {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || !domain.IsSessionBackedRequestActor(actor) {
		return nil, domain.ErrUnauthorized
	}
	user, err := s.userRepo.GetByID(ctx, actor.ID)
	if err != nil {
		return nil, infraError("get current user", err)
	}
	if user == nil {
		return nil, domain.ErrNotFound
	}
	if err := s.attachRoles(ctx, user); err != nil {
		return nil, infraError("attach current user roles", err)
	}
	s.prepareUserForResponse(user)
	return user, nil
}

func (s *identityService) ListUsers(ctx context.Context, filter UserFilter) ([]*domain.User, domain.PaginationMeta, *domain.AppError) {
	if filter.Role != nil && *filter.Role != "" && !domain.IsKnownRole(*filter.Role) {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "role is invalid", nil)
	}
	if filter.Department != nil && strings.TrimSpace(string(*filter.Department)) != "" {
		if appErr := s.validateDepartment(*filter.Department); appErr != nil {
			if appErr.Code == domain.ErrCodeInvalidRequest {
				if len(domain.OrgDepartmentAliases(string(*filter.Department))) <= 1 {
					return []*domain.User{}, buildPaginationMeta(filter.Page, filter.PageSize, 0), nil
				}
			} else {
				return nil, domain.PaginationMeta{}, appErr
			}
		}
	}
	if appErr := s.authorizeUserListFilter(ctx, &filter); appErr != nil {
		return nil, domain.PaginationMeta{}, appErr
	}
	actor, hasActor := domain.RequestActorFromContext(ctx)
	permission := identityReadPermission(actor)
	access := domain.ResourceGroupAccessFilterForActor(actor, permission)
	scopeUserIDs := make([]int64, 0, 1)
	if access.Self && actor.ID > 0 {
		scopeUserIDs = append(scopeUserIDs, actor.ID)
	}
	users, total, err := s.userRepo.List(ctx, repo.UserListFilter{
		Keyword:            filter.Keyword,
		Status:             filter.Status,
		Role:               filter.Role,
		Department:         filter.Department,
		Team:               strings.TrimSpace(filter.Team),
		ScopeRestricted:    hasActor && actor.ID > 0,
		ScopeGlobal:        access.Global,
		ScopeUserIDs:       scopeUserIDs,
		ScopeDepartmentIDs: access.DepartmentIDs,
		ScopeTeamIDs:       access.TeamIDs,
		Page:               filter.Page,
		PageSize:           filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list users", err)
	}
	if err := s.attachRolesForUsers(ctx, users); err != nil {
		return nil, domain.PaginationMeta{}, infraError("attach user roles", err)
	}
	if err := s.prepareUsersForResponse(ctx, users); err != nil {
		return nil, domain.PaginationMeta{}, infraError("prepare user access", err)
	}
	return users, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *identityService) ListAccessPolicyUsers(ctx context.Context, filter UserFilter) ([]*domain.User, domain.PaginationMeta, *domain.AppError) {
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 30
	}
	users, total, err := s.userRepo.List(ctx, repo.UserListFilter{
		Keyword:  filter.Keyword,
		Status:   filter.Status,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list access policy users", err)
	}
	if err := s.prepareUsersForResponse(ctx, users); err != nil {
		return nil, domain.PaginationMeta{}, infraError("prepare access policy users", err)
	}
	return users, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

// ListAssignableDesigners implements the task candidate selector. The route
// checks that the caller may assign, reassign, create, or hand over a task; this
// service independently derives candidates from effective auth_* assignments.
// Display organization names and legacy user_roles are deliberately ignored.
func (s *identityService) ListAssignableDesigners(ctx context.Context, actor *domain.RequestActor, lane AssignableLane) ([]*domain.User, *domain.AppError) {
	if actor == nil || actor.ID <= 0 {
		return nil, domain.ErrUnauthorized
	}
	if lane == "" {
		lane = AssignableLaneNormal
	}
	switch lane {
	case AssignableLaneNormal, AssignableLaneCustomization, AssignableLaneAudit, AssignableLaneAll:
	default:
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			"workflow_lane is not supported",
			map[string]string{
				"field":     "workflow_lane",
				"deny_code": "workflow_lane_unsupported",
			},
		)
	}
	if s.effectiveAccessReader == nil {
		return nil, infraError("list assignable candidates", fmt.Errorf("effective access reader is not configured"))
	}
	status := domain.UserStatusActive
	count, err := s.userRepo.Count(ctx)
	if err != nil {
		return nil, infraError("count assignable candidates", err)
	}
	pageSize := int(count)
	if pageSize < 1 {
		return []*domain.User{}, nil
	}
	users, _, err := s.userRepo.List(ctx, repo.UserListFilter{Status: &status, Page: 1, PageSize: pageSize})
	if err != nil {
		return nil, infraError("list assignable candidates", err)
	}
	userIDs := make([]int64, 0, len(users))
	for _, user := range users {
		if user != nil && user.ID > 0 {
			userIDs = append(userIDs, user.ID)
		}
	}
	effectiveByUser := make(map[int64]*domain.EffectiveAccess, len(userIDs))
	if batchReader, ok := s.effectiveAccessReader.(IdentityEffectiveAccessBatchReader); ok {
		effectiveByUser, err = batchReader.EffectiveAccessMany(ctx, userIDs)
		if err != nil {
			return nil, infraError("load assignable candidate access", err)
		}
	} else {
		for _, userID := range userIDs {
			effective, readErr := s.effectiveAccessReader.EffectiveAccess(ctx, userID)
			if readErr != nil {
				return nil, infraError("load assignable candidate access", readErr)
			}
			effectiveByUser[userID] = effective
		}
	}
	filtered := make([]*domain.User, 0, len(users))
	for _, user := range users {
		if user == nil || !effectiveAccessMatchesAssignableLane(effectiveByUser[user.ID], lane) {
			continue
		}
		filtered = append(filtered, user)
	}
	if err := s.prepareUsersForResponse(ctx, filtered); err != nil {
		return nil, infraError("prepare assignable candidate access", err)
	}
	return filtered, nil
}

func effectiveAccessMatchesAssignableLane(access *domain.EffectiveAccess, lane AssignableLane) bool {
	if access == nil {
		return false
	}
	for _, source := range access.Sources {
		switch lane {
		case AssignableLaneAudit:
			if source.Permission == domain.PermissionTaskAudit {
				return true
			}
		case AssignableLaneNormal:
			if source.Permission == domain.PermissionTaskUploadSource && source.RoleCode == "designer" {
				return true
			}
		case AssignableLaneCustomization:
			if source.Permission == domain.PermissionTaskUploadSource && source.RoleCode == "customization_operator" {
				return true
			}
		case AssignableLaneAll:
			if source.Permission == domain.PermissionTaskUploadSource && (source.RoleCode == "designer" || source.RoleCode == "customization_operator") {
				return true
			}
		}
	}
	return false
}

func (s *identityService) GetUser(ctx context.Context, userID int64) (*domain.User, *domain.AppError) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, infraError("get user", err)
	}
	if user == nil {
		return nil, domain.ErrNotFound
	}
	if err := s.attachRoles(ctx, user); err != nil {
		return nil, infraError("attach user roles", err)
	}
	if appErr := s.authorizeUserRead(ctx, user); appErr != nil {
		return nil, appErr
	}
	return user, nil
}

func (s *identityService) UpdateUser(ctx context.Context, p UpdateUserParams) (*domain.User, *domain.AppError) {
	user, appErr := s.GetUser(ctx, p.UserID)
	if appErr != nil {
		return nil, appErr
	}

	changes := make([]string, 0, 6)
	if p.EmployeeNo != nil {
		employeeNo, appErr := optionalEmployeeNo(p.EmployeeNo)
		if appErr != nil {
			return nil, appErr
		}
		if appErr := s.ensureUniqueEmployeeNo(ctx, employeeNo, user.ID); appErr != nil {
			return nil, appErr
		}
		if user.EmployeeNo == nil || *user.EmployeeNo != employeeNo {
			user.EmployeeNo = &employeeNo
			changes = append(changes, "employee_no")
		}
	}
	if p.DisplayName != nil {
		displayName := strings.TrimSpace(*p.DisplayName)
		if displayName == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "display_name is required", nil)
		}
		if displayName != user.DisplayName {
			user.DisplayName = displayName
			changes = append(changes, "display_name")
		}
	}
	if p.Status != nil {
		if !p.Status.Valid() {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "status must be active or disabled", nil)
		}
		if appErr := s.ensurePrivilegedUserStatusSafety(ctx, user, *p.Status); appErr != nil {
			return nil, appErr
		}
		if *p.Status != user.Status {
			user.Status = *p.Status
			changes = append(changes, "status")
		}
	}
	if p.EmploymentType != nil {
		if !p.EmploymentType.Valid() {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "employment_type must be full_time or part_time", nil)
		}
		if *p.EmploymentType != user.EmploymentType {
			user.EmploymentType = *p.EmploymentType
			changes = append(changes, "employment_type")
		}
	}
	if p.Department != nil {
		if appErr := s.validateDepartment(*p.Department); appErr != nil {
			return nil, appErr
		}
	}

	nextDepartment := user.Department
	nextTeam := user.Team
	if p.Department != nil {
		nextDepartment = *p.Department
	}

	teamInput, teamProvided := "", p.Team != nil
	if teamProvided {
		teamInput = strings.TrimSpace(*p.Team)
	}
	if teamProvided {
		if strings.EqualFold(teamInput, userTeamUngroupedAlias) {
			if !s.authSettings.UnassignedPoolEnabled {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "unassigned pool is disabled", nil)
			}
			nextDepartment = domain.DepartmentUnassigned
			unassignedTeam, appErr := s.defaultUnassignedPoolTeam()
			if appErr != nil {
				return nil, appErr
			}
			nextTeam = unassignedTeam
		} else {
			nextTeam = teamInput
		}
	} else if p.Department != nil && nextDepartment == domain.DepartmentUnassigned && s.authSettings.UnassignedPoolEnabled {
		unassignedTeam, appErr := s.defaultUnassignedPoolTeam()
		if appErr != nil {
			return nil, appErr
		}
		nextTeam = unassignedTeam
	}

	if appErr := s.validateTeam(nextDepartment, nextTeam); appErr != nil {
		return nil, appErr
	}
	nextDepartmentID := user.DepartmentID
	nextTeamID := user.TeamID
	if p.Department != nil || p.Team != nil || p.DepartmentID != nil || p.TeamID != nil {
		departmentID, teamID, orgErr := s.resolveStableUserOrgIDs(ctx, p.DepartmentID, p.TeamID, nextDepartment, nextTeam)
		if orgErr != nil {
			return nil, orgErr
		}
		nextDepartmentID = departmentID
		nextTeamID = teamID
	}
	if appErr := s.authorizeUserUpdate(ctx, user, nextDepartmentID, nextTeamID); appErr != nil {
		return nil, appErr
	}
	if nextDepartment != user.Department {
		user.Department = nextDepartment
		changes = append(changes, "department")
	}
	if nextTeam != user.Team {
		user.Team = nextTeam
		changes = append(changes, "team")
	}
	if p.Department != nil || p.Team != nil || p.DepartmentID != nil || p.TeamID != nil {
		if !equalOptionalInt64(user.DepartmentID, nextDepartmentID) {
			user.DepartmentID = nextDepartmentID
			changes = append(changes, "department_id")
		}
		if !equalOptionalInt64(user.TeamID, nextTeamID) {
			user.TeamID = nextTeamID
			changes = append(changes, "team_id")
		}
	}
	if p.Mobile != nil {
		mobile := strings.TrimSpace(*p.Mobile)
		if appErr := validateMobile(mobile); appErr != nil {
			return nil, appErr
		}
		if appErr := s.ensureUniqueIdentity(ctx, user.Username, mobile, user.ID); appErr != nil {
			return nil, appErr
		}
		if mobile != user.Mobile {
			user.Mobile = mobile
			changes = append(changes, "mobile")
		}
	}
	if p.Email != nil {
		email := strings.TrimSpace(*p.Email)
		if appErr := validateOptionalEmail(email); appErr != nil {
			return nil, appErr
		}
		if email != user.Email {
			user.Email = email
			changes = append(changes, "email")
		}
	}
	if p.ManagedDepartments != nil {
		managedDepartments, appErr := s.validateManagedDepartments(*p.ManagedDepartments)
		if appErr != nil {
			return nil, appErr
		}
		if !sameStringSlice(user.ManagedDepartments, managedDepartments) {
			user.ManagedDepartments = managedDepartments
			changes = append(changes, "managed_departments")
		}
	}
	if p.ManagedTeams != nil {
		managedTeams, appErr := s.validateManagedTeams(user.Department, *p.ManagedTeams)
		if appErr != nil {
			return nil, appErr
		}
		if !sameStringSlice(user.ManagedTeams, managedTeams) {
			user.ManagedTeams = managedTeams
			changes = append(changes, "managed_teams")
		}
	}
	if len(changes) == 0 {
		return user, nil
	}

	user.UpdatedAt = time.Now().UTC()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.userRepo.Update(ctx, tx, user)
	}); err != nil {
		if p.EmployeeNo != nil {
			if appErr := s.employeeNoWriteConflict(ctx, err, *p.EmployeeNo, user.ID); appErr != nil {
				return nil, appErr
			}
		}
		return nil, infraError("update user tx", err)
	}
	updated, appErr := s.GetUser(ctx, p.UserID)
	if appErr != nil {
		return nil, appErr
	}
	s.recordUserUpdateLogs(ctx, updated, changes)
	return updated, nil
}

func (s *identityService) updateUserBypassManagementScope(ctx context.Context, user *domain.User, p UpdateMeParams, method, routePath string) (*domain.User, *domain.AppError) {
	changes := make([]string, 0, 4)
	if p.DisplayName != nil {
		displayName := strings.TrimSpace(*p.DisplayName)
		if displayName == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "display_name is required", nil)
		}
		if displayName != user.DisplayName {
			user.DisplayName = displayName
			changes = append(changes, "display_name")
		}
	}
	if p.Mobile != nil {
		mobile := strings.TrimSpace(*p.Mobile)
		if appErr := validateMobile(mobile); appErr != nil {
			return nil, appErr
		}
		if appErr := s.ensureUniqueIdentity(ctx, user.Username, mobile, user.ID); appErr != nil {
			return nil, appErr
		}
		if mobile != user.Mobile {
			user.Mobile = mobile
			changes = append(changes, "mobile")
		}
	}
	if p.Email != nil {
		email := strings.TrimSpace(*p.Email)
		if appErr := validateOptionalEmail(email); appErr != nil {
			return nil, appErr
		}
		if email != user.Email {
			user.Email = email
			changes = append(changes, "email")
		}
	}
	if len(changes) == 0 {
		s.prepareUserForResponse(user)
		return user, nil
	}
	user.UpdatedAt = time.Now().UTC()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.userRepo.Update(ctx, tx, user); err != nil {
			return err
		}
		return s.recordPermissionActionTx(ctx, tx, domain.PermissionLog{
			ActionType:     domain.PermissionActionUserUpdated,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    user.Roles,
			Granted:        true,
			Reason:         "updated fields: " + strings.Join(changes, ","),
			Method:         method,
			RoutePath:      routePath,
		})
	}); err != nil {
		return nil, infraError("update current user tx", err)
	}
	return s.GetCurrentUser(ctx)
}

func (s *identityService) ActivateUser(ctx context.Context, userID int64) *domain.AppError {
	return s.setUserStatusFromEndpoint(ctx, userID, domain.UserStatusActive, domain.PermissionActionUserActivated, "/v1/users/:id/activate")
}

func (s *identityService) DeactivateUser(ctx context.Context, userID int64) *domain.AppError {
	return s.setUserStatusFromEndpoint(ctx, userID, domain.UserStatusDisabled, domain.PermissionActionUserDeactivated, "/v1/users/:id/deactivate")
}

func (s *identityService) setUserStatusFromEndpoint(ctx context.Context, userID int64, status domain.UserStatus, action, routePath string) *domain.AppError {
	if userID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id is required", nil)
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return infraError("get user for status update", err)
	}
	if user == nil {
		return domain.ErrNotFound
	}
	if err := s.attachRoles(ctx, user); err != nil {
		return infraError("attach status user roles", err)
	}
	if appErr := s.authorizeUserStatusEndpoint(ctx, user); appErr != nil {
		return appErr
	}
	if actor, ok := domain.RequestActorFromContext(ctx); ok && actor.ID == user.ID && status != domain.UserStatusActive {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "不能停用当前登录账号。", map[string]interface{}{"deny_code": "self_deactivate_denied"})
	}
	if appErr := s.ensurePrivilegedUserStatusSafety(ctx, user, status); appErr != nil {
		return appErr
	}
	if user.Status == status {
		return nil
	}
	user.Status = status
	user.UpdatedAt = time.Now().UTC()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.userRepo.Update(ctx, tx, user); err != nil {
			return err
		}
		return s.recordPermissionActionTx(ctx, tx, domain.PermissionLog{
			ActionType:     action,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    user.Roles,
			Granted:        true,
			Reason:         "to=" + string(status),
			Method:         "POST",
			RoutePath:      routePath,
		})
	}); err != nil {
		return infraError("update user status tx", err)
	}
	return nil
}

func (s *identityService) ListPermissionLogs(ctx context.Context, filter PermissionLogFilter) ([]*domain.PermissionLog, domain.PaginationMeta, *domain.AppError) {
	logs, total, err := s.permissionLogRepo.List(ctx, repo.PermissionLogListFilter{
		ActorID:        filter.ActorID,
		ActorUsername:  filter.ActorUsername,
		ActionType:     filter.ActionType,
		TargetUserID:   filter.TargetUserID,
		TargetUsername: filter.TargetUsername,
		Granted:        filter.Granted,
		Method:         filter.Method,
		RoutePath:      filter.RoutePath,
		Page:           filter.Page,
		PageSize:       filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list permission logs", err)
	}
	return logs, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *identityService) ResolveRequestActor(ctx context.Context, bearerToken string) (*domain.RequestActor, *domain.AppError) {
	bearerToken = strings.TrimSpace(bearerToken)
	if bearerToken == "" {
		return nil, nil
	}

	if bundleReader, ok := s.sessionRepo.(sessionActorBundleReader); ok {
		actor, appErr := s.resolveRequestActorBundle(ctx, bearerToken, bundleReader)
		if appErr == nil || appErr.Code != domain.ErrCodeInternalError {
			return actor, appErr
		}
	}

	session, err := s.sessionRepo.GetByTokenHash(ctx, hashToken(bearerToken))
	if err != nil {
		return nil, infraError("get session by token", err)
	}
	if session == nil || session.RevokedAt != nil || session.ExpiresAt.Before(time.Now().UTC()) {
		return nil, domain.ErrUnauthorized
	}

	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		return nil, infraError("get user by session", err)
	}
	if user == nil {
		return nil, domain.ErrUnauthorized
	}
	if user.Status != domain.UserStatusActive {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "user is disabled", nil)
	}

	// Raw vs normalized role observation. When the repo supports raw reads we
	// can detect unknown role strings that NormalizeRoles dropped; otherwise
	// we fall back to the pre-existing ListRoles path (which already filters
	// unknowns) and can only report the zero-known-roles degraded case.
	var (
		rawRoles        []string
		rawRolesCount   int
		rawRolesKnown   bool
		normalizedRoles []domain.Role
	)
	if rawReader, ok := s.userRepo.(userRoleRawReader); ok {
		rawRoles, err = rawReader.ListRolesRaw(ctx, user.ID)
		if err != nil {
			return nil, infraError("list raw roles by session", err)
		}
		rawRolesCount = len(rawRoles)
		rawRolesKnown = true
		normalizedRoles = domain.NormalizeRoles(rawRoles)
	} else {
		normalizedRoles, err = s.userRepo.ListRoles(ctx, user.ID)
		if err != nil {
			return nil, infraError("list roles by session", err)
		}
	}

	user.Roles = normalizedRoles
	s.prepareUserForResponse(user)
	// prepareUserForResponse may have applied the [Member] default when the
	// normalized role slice was empty. The authoritative post-default slice
	// lives on user.Roles and is the same slice used to derive
	// user.FrontendAccess; it is also the canonical source for the actor
	// roles to keep authorizeUserRead / authorizeUserListFilter consistent
	// with frontend_access.roles.
	canonicalRoles := append([]domain.Role(nil), user.Roles...)

	s.emitActorRoleHydrationTelemetry(ctx, user, rawRolesKnown, rawRoles, rawRolesCount, normalizedRoles, canonicalRoles)

	now := time.Now().UTC()
	_ = s.sessionRepo.Touch(ctx, session.SessionID, now)
	actor := &domain.RequestActor{
		ID:                 user.ID,
		Username:           user.Username,
		Roles:              canonicalRoles,
		Department:         string(user.Department),
		DepartmentID:       user.DepartmentID,
		Team:               user.Team,
		TeamID:             user.TeamID,
		ManagedDepartments: append([]string(nil), user.ManagedDepartments...),
		ManagedTeams:       append([]string(nil), user.ManagedTeams...),
		FrontendAccess:     user.FrontendAccess,
		Source:             domain.RequestActorSourceSessionToken,
		AuthMode:           domain.AuthModeSessionTokenRoleEnforced,
	}
	return actor, nil
}

func (s *identityService) resolveRequestActorBundle(ctx context.Context, bearerToken string, reader sessionActorBundleReader) (*domain.RequestActor, *domain.AppError) {
	now := time.Now().UTC()
	session, user, rawRoles, err := reader.ResolveActorBundle(ctx, hashToken(bearerToken), now)
	if err != nil {
		return nil, infraError("get session actor bundle", err)
	}
	if session == nil || session.RevokedAt != nil || session.ExpiresAt.Before(now) {
		return nil, domain.ErrUnauthorized
	}
	if user == nil {
		return nil, domain.ErrUnauthorized
	}
	if user.Status != domain.UserStatusActive {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "user is disabled", nil)
	}

	normalizedRoles := domain.NormalizeRoles(rawRoles)
	user.Roles = normalizedRoles
	s.prepareUserForResponse(user)
	canonicalRoles := append([]domain.Role(nil), user.Roles...)

	s.emitActorRoleHydrationTelemetry(ctx, user, true, rawRoles, len(rawRoles), normalizedRoles, canonicalRoles)

	return &domain.RequestActor{
		ID:                 user.ID,
		Username:           user.Username,
		Roles:              canonicalRoles,
		Department:         string(user.Department),
		DepartmentID:       user.DepartmentID,
		Team:               user.Team,
		TeamID:             user.TeamID,
		ManagedDepartments: append([]string(nil), user.ManagedDepartments...),
		ManagedTeams:       append([]string(nil), user.ManagedTeams...),
		FrontendAccess:     user.FrontendAccess,
		Source:             domain.RequestActorSourceSessionToken,
		AuthMode:           domain.AuthModeSessionTokenRoleEnforced,
	}, nil
}

// emitActorRoleHydrationTelemetry emits a warn-level structured log entry
// when ListRoles returns zero known roles or when NormalizeRoles dropped one
// or more raw role strings. This path is observability-only and never fails
// the request.
func (s *identityService) emitActorRoleHydrationTelemetry(
	ctx context.Context,
	user *domain.User,
	rawRolesKnown bool,
	rawRoles []string,
	rawRolesCount int,
	normalizedRoles []domain.Role,
	canonicalRoles []domain.Role,
) {
	if s.logger == nil {
		return
	}
	droppedRoles := make([]string, 0)
	if rawRolesKnown {
		normalizedSet := make(map[domain.Role]struct{}, len(normalizedRoles))
		for _, role := range normalizedRoles {
			normalizedSet[role] = struct{}{}
		}
		seenDropped := make(map[string]struct{})
		for _, raw := range rawRoles {
			trimmed := strings.TrimSpace(raw)
			if trimmed == "" {
				continue
			}
			if _, ok := normalizedSet[domain.Role(trimmed)]; ok {
				continue
			}
			if _, dup := seenDropped[trimmed]; dup {
				continue
			}
			seenDropped[trimmed] = struct{}{}
			droppedRoles = append(droppedRoles, trimmed)
		}
	}
	zeroKnown := len(normalizedRoles) == 0
	if !zeroKnown && len(droppedRoles) == 0 {
		return
	}
	normalizedStrings := make([]string, 0, len(normalizedRoles))
	for _, role := range normalizedRoles {
		normalizedStrings = append(normalizedStrings, string(role))
	}
	canonicalStrings := make([]string, 0, len(canonicalRoles))
	for _, role := range canonicalRoles {
		canonicalStrings = append(canonicalStrings, string(role))
	}
	s.logger.Warn("actor_role_hydration_degraded",
		zap.String("event", "actor_role_hydration_degraded"),
		zap.String("trace_id", domain.TraceIDFromContext(ctx)),
		zap.Int64("user_id", user.ID),
		zap.String("department", string(user.Department)),
		zap.String("team", user.Team),
		zap.Int("raw_roles_count", rawRolesCount),
		zap.Bool("raw_roles_observed", rawRolesKnown),
		zap.Strings("normalized_roles", normalizedStrings),
		zap.Strings("canonical_roles", canonicalStrings),
		zap.Strings("dropped_roles", droppedRoles),
		zap.Bool("zero_known_roles", zeroKnown),
		zap.String("auth_mode", string(domain.AuthModeSessionTokenRoleEnforced)),
	)
}

func (s *identityService) RecordRouteAccess(ctx context.Context, entry domain.PermissionLog) {
	if s.permissionLogRepo == nil {
		return
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(entry.ActionType) == "" {
		entry.ActionType = domain.PermissionActionRouteAccess
	}
	if entry.ActionType == domain.PermissionActionRouteAccess && entry.Granted {
		entryCopy := entry
		go func() {
			logCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = s.permissionLogRepo.Create(logCtx, &entryCopy)
		}()
		return
	}
	_ = s.permissionLogRepo.Create(ctx, &entry)
}

func (s *identityService) attachRoles(ctx context.Context, user *domain.User) error {
	roles, err := s.userRepo.ListRoles(ctx, user.ID)
	if err != nil {
		return err
	}
	user.Roles = roles
	s.prepareUserForResponse(user)
	return nil
}

type batchUserRolesReader interface {
	ListRolesByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]domain.Role, error)
}

func (s *identityService) attachRolesForUsers(ctx context.Context, users []*domain.User) error {
	if len(users) == 0 {
		return nil
	}
	if batchReader, ok := s.userRepo.(batchUserRolesReader); ok {
		userIDs := make([]int64, 0, len(users))
		for _, user := range users {
			if user == nil || user.ID <= 0 {
				continue
			}
			userIDs = append(userIDs, user.ID)
		}
		rolesByUser, err := batchReader.ListRolesByUserIDs(ctx, userIDs)
		if err != nil {
			return err
		}
		for _, user := range users {
			if user == nil {
				continue
			}
			user.Roles = append([]domain.Role(nil), rolesByUser[user.ID]...)
		}
		return nil
	}
	for _, user := range users {
		if user == nil {
			continue
		}
		if err := s.attachRoles(ctx, user); err != nil {
			return err
		}
	}
	return nil
}

func cloneOrgOptions(options *domain.OrgOptions) *domain.OrgOptions {
	if options == nil {
		return &domain.OrgOptions{}
	}
	cloned := &domain.OrgOptions{
		Departments:           make([]domain.DepartmentOption, 0, len(options.Departments)),
		TeamsByDepartment:     make(map[string][]string, len(options.TeamsByDepartment)),
		UnassignedPoolEnabled: options.UnassignedPoolEnabled,
		ConfiguredAssignments: append([]domain.ConfiguredUserAssignment{}, options.ConfiguredAssignments...),
	}
	for _, department := range options.Departments {
		teamItems := make([]domain.OrgTeamOption, 0, len(department.TeamItems))
		for _, item := range department.TeamItems {
			teamItems = append(teamItems, domain.OrgTeamOption{
				ID:          item.ID,
				Name:        item.Name,
				Enabled:     item.Enabled,
				MemberCount: item.MemberCount,
			})
		}
		cloned.Departments = append(cloned.Departments, domain.DepartmentOption{
			ID:          department.ID,
			Name:        department.Name,
			Teams:       append([]string{}, department.Teams...),
			TeamItems:   teamItems,
			Enabled:     department.Enabled,
			MemberCount: department.MemberCount,
		})
	}
	for department, teams := range options.TeamsByDepartment {
		cloned.TeamsByDepartment[department] = append([]string{}, teams...)
	}
	return cloned
}

func (s *identityService) ensureAdminRoleSafety(ctx context.Context, currentRoles, nextRoles []domain.Role) *domain.AppError {
	currentSuperAdmin := containsRole(currentRoles, domain.RoleSuperAdmin)
	nextSuperAdmin := containsRole(nextRoles, domain.RoleSuperAdmin)
	if !currentSuperAdmin || nextSuperAdmin {
		return nil
	}
	activeAdminUsers, err := s.activeUsersWithRoles(ctx, domain.RoleSuperAdmin)
	if err != nil {
		return infraError("list super admin users", err)
	}
	if len(activeAdminUsers) <= 1 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "at least one SuperAdmin user must remain", map[string]interface{}{
			"deny_code": "last_super_admin_removal_denied",
		})
	}
	return nil
}

func (s *identityService) recordUserUpdateLogs(ctx context.Context, user *domain.User, changes []string) {
	if user == nil || len(changes) == 0 {
		return
	}
	reason := "updated fields: " + strings.Join(changes, ",")
	orgChanged := containsStringValue(changes, "department") || containsStringValue(changes, "team")
	scopeChanged := containsStringValue(changes, "managed_departments") || containsStringValue(changes, "managed_teams")
	statusChanged := containsStringValue(changes, "status")
	profileChanged := containsStringValue(changes, "display_name") ||
		containsStringValue(changes, "email") ||
		containsStringValue(changes, "mobile")
	if user.Department == domain.DepartmentUnassigned && user.Team == "未分配池" {
		reason += " (unassigned_pool)"
	} else if orgChanged {
		reason += " (assigned_to_formal_org)"
	}
	if orgChanged && user.Department != domain.DepartmentUnassigned {
		s.recordPermissionAction(ctx, domain.PermissionLog{
			ActionType:     domain.PermissionActionPoolAssigned,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    user.Roles,
			Granted:        true,
			Reason:         reason,
			Method:         "PATCH",
			RoutePath:      "/v1/users/:id",
		})
	}
	if orgChanged {
		s.recordPermissionAction(ctx, domain.PermissionLog{
			ActionType:     domain.PermissionActionUserOrgChanged,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    user.Roles,
			Granted:        true,
			Reason:         reason,
			Method:         "PATCH",
			RoutePath:      "/v1/users/:id",
		})
	}
	if scopeChanged {
		s.recordPermissionAction(ctx, domain.PermissionLog{
			ActionType:     domain.PermissionActionUserScopeChanged,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    user.Roles,
			Granted:        true,
			Reason:         reason,
			Method:         "PATCH",
			RoutePath:      "/v1/users/:id",
		})
	}
	if statusChanged {
		s.recordPermissionAction(ctx, domain.PermissionLog{
			ActionType:     domain.PermissionActionUserStatusChanged,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    user.Roles,
			Granted:        true,
			Reason:         reason,
			Method:         "PATCH",
			RoutePath:      "/v1/users/:id",
		})
	}
	if profileChanged {
		s.recordPermissionAction(ctx, domain.PermissionLog{
			ActionType:     domain.PermissionActionUserUpdated,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    user.Roles,
			Granted:        true,
			Reason:         reason,
			Method:         "PATCH",
			RoutePath:      "/v1/users/:id",
		})
	}
}

func (s *identityService) recordPermissionAction(ctx context.Context, entry domain.PermissionLog) {
	if s.permissionLogRepo == nil {
		return
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	if entry.ActorID == nil || entry.ActorUsername == "" || entry.ActorSource == "" || entry.AuthMode == "" || len(entry.ActorRoles) == 0 {
		actor, ok := domain.RequestActorFromContext(ctx)
		if ok {
			if entry.ActorID == nil && actor.ID > 0 {
				entry.ActorID = actorIDPtr(actor.ID)
			}
			if entry.ActorUsername == "" {
				entry.ActorUsername = actor.Username
			}
			if entry.ActorSource == "" {
				entry.ActorSource = actor.Source
			}
			if entry.AuthMode == "" {
				entry.AuthMode = actor.AuthMode
			}
			if len(entry.ActorRoles) == 0 {
				entry.ActorRoles = actor.Roles
			}
		}
	}
	if entry.ActionType == "" {
		entry.ActionType = domain.PermissionActionRouteAccess
	}
	if entry.ActorSource == "" {
		entry.ActorSource = domain.RequestActorSourceAnonymous
	}
	if entry.AuthMode == "" {
		entry.AuthMode = domain.AuthModeSessionTokenRoleEnforced
	}
	if entry.Readiness == "" {
		entry.Readiness = domain.APIReadinessReadyForFrontend
	}
	_ = s.permissionLogRepo.Create(ctx, &entry)
}

type identityPermissionLogTxRepo interface {
	CreateTx(ctx context.Context, tx repo.Tx, entry *domain.PermissionLog) error
}

func (s *identityService) recordPermissionActionTx(ctx context.Context, tx repo.Tx, entry domain.PermissionLog) error {
	if s.permissionLogRepo == nil {
		return nil
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	if entry.ActorID == nil || entry.ActorUsername == "" || entry.ActorSource == "" || entry.AuthMode == "" || len(entry.ActorRoles) == 0 {
		actor, ok := domain.RequestActorFromContext(ctx)
		if ok {
			if entry.ActorID == nil && actor.ID > 0 {
				entry.ActorID = actorIDPtr(actor.ID)
			}
			if entry.ActorUsername == "" {
				entry.ActorUsername = actor.Username
			}
			if entry.ActorSource == "" {
				entry.ActorSource = actor.Source
			}
			if entry.AuthMode == "" {
				entry.AuthMode = actor.AuthMode
			}
			if len(entry.ActorRoles) == 0 {
				entry.ActorRoles = actor.Roles
			}
		}
	}
	if entry.ActorSource == "" {
		entry.ActorSource = domain.RequestActorSourceAnonymous
	}
	if entry.AuthMode == "" {
		entry.AuthMode = domain.AuthModeSessionTokenRoleEnforced
	}
	if entry.Readiness == "" {
		entry.Readiness = domain.APIReadinessReadyForFrontend
	}
	if txRepo, ok := s.permissionLogRepo.(identityPermissionLogTxRepo); ok {
		return txRepo.CreateTx(ctx, tx, &entry)
	}
	return s.permissionLogRepo.Create(ctx, &entry)
}

func (s *identityService) recordSystemPermissionAction(ctx context.Context, entry domain.PermissionLog) {
	entry.ActorUsername = "system_bootstrap"
	entry.ActorSource = domain.RequestActorSourceSystemFallback
	entry.AuthMode = domain.AuthModePlaceholderNoEnforcement
	s.recordPermissionAction(ctx, entry)
}

func (s *identityService) prepareUserForResponse(user *domain.User) {
	if user == nil {
		return
	}
	s.prepareUserBaseResponse(user)
	if s.effectiveAccessReader != nil && user.ID > 0 {
		if effective, err := s.effectiveAccessReader.EffectiveAccess(context.Background(), user.ID); err == nil && effective != nil {
			user.FrontendAccess = domain.MergeEffectiveAccessIntoFrontendAccess(user.FrontendAccess, effective)
		}
	}
}

func (s *identityService) prepareUsersForResponse(ctx context.Context, users []*domain.User) error {
	userIDs := make([]int64, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		s.prepareUserBaseResponse(user)
		if user.ID > 0 {
			userIDs = append(userIDs, user.ID)
		}
	}
	if s.effectiveAccessReader == nil || len(userIDs) == 0 {
		return nil
	}
	if batchReader, ok := s.effectiveAccessReader.(IdentityEffectiveAccessBatchReader); ok {
		effectiveByUser, err := batchReader.EffectiveAccessMany(ctx, userIDs)
		if err != nil {
			return err
		}
		for _, user := range users {
			if user == nil {
				continue
			}
			user.FrontendAccess = domain.MergeEffectiveAccessIntoFrontendAccess(user.FrontendAccess, effectiveByUser[user.ID])
		}
		return nil
	}
	for _, user := range users {
		if user == nil || user.ID <= 0 {
			continue
		}
		effective, err := s.effectiveAccessReader.EffectiveAccess(ctx, user.ID)
		if err != nil {
			return err
		}
		user.FrontendAccess = domain.MergeEffectiveAccessIntoFrontendAccess(user.FrontendAccess, effective)
	}
	return nil
}

func (s *identityService) prepareUserBaseResponse(user *domain.User) {
	if len(user.Roles) == 0 {
		user.Roles = []domain.Role{domain.RoleMember}
	}
	if !user.EmploymentType.Valid() {
		user.EmploymentType = domain.EmploymentTypeFullTime
	}
	user.Account = user.Username
	user.Name = user.DisplayName
	user.RealName = user.DisplayName
	user.Group = user.Team
	user.Phone = user.Mobile
	user.Avatar = user.AvatarURL
	user.FrontendAccess = domain.BuildFrontendAccess(user, s.frontendAccessSettings)
}

func (s *identityService) validateDepartment(department domain.Department) *domain.AppError {
	if department == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "department is required", nil)
	}
	if s.orgRepo != nil {
		item, err := s.orgRepo.GetDepartmentByName(context.Background(), strings.TrimSpace(string(department)))
		if err != nil {
			return infraError("get org department by name for validation", err)
		}
		if item != nil && item.Enabled {
			return nil
		}
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "department is invalid", map[string]interface{}{"department": department})
	}
	for _, candidate := range s.authSettings.Departments {
		if department == candidate {
			return nil
		}
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "department is invalid", map[string]interface{}{"department": department})
}

func (s *identityService) validateTeam(department domain.Department, team string) *domain.AppError {
	team = strings.TrimSpace(team)
	if s.orgRepo != nil {
		trimmedDepartment := strings.TrimSpace(string(department))
		departmentItem, err := s.orgRepo.GetDepartmentByName(context.Background(), trimmedDepartment)
		if err != nil {
			return infraError("get org department for team validation", err)
		}
		if departmentItem == nil || !departmentItem.Enabled {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "department is invalid", map[string]interface{}{"department": department})
		}
		teams, err := s.orgRepo.ListTeams(context.Background(), false)
		if err != nil {
			return infraError("list org teams for validation", err)
		}
		departmentTeams := make([]string, 0)
		for _, candidate := range teams {
			if candidate == nil || candidate.DepartmentID != departmentItem.ID || !candidate.Enabled {
				continue
			}
			departmentTeams = append(departmentTeams, candidate.Name)
			if team != "" && team == candidate.Name {
				return nil
			}
		}
		if len(departmentTeams) == 0 {
			if team != "" {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "team is invalid for department", map[string]interface{}{
					"department": department,
					"team":       team,
				})
			}
			return nil
		}
		if team == "" {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "team is required", map[string]interface{}{"department": department})
		}
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "team must belong to department", map[string]interface{}{
			"department": department,
			"team":       team,
			"teams":      departmentTeams,
		})
	}
	teams := s.authSettings.DepartmentTeams[string(department)]
	if len(teams) == 0 {
		if team != "" {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "team is invalid for department", map[string]interface{}{
				"department": department,
				"team":       team,
			})
		}
		return nil
	}
	if team == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "team is required", map[string]interface{}{
			"department": department,
		})
	}
	for _, candidate := range teams {
		if team == candidate {
			return nil
		}
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "team must belong to department", map[string]interface{}{
		"department": department,
		"team":       team,
		"teams":      teams,
	})
}

func (s *identityService) defaultUnassignedPoolTeam() (string, *domain.AppError) {
	if s.orgRepo != nil {
		teams, err := s.orgRepo.ListTeams(context.Background(), false)
		if err != nil {
			return "", infraError("list org teams for unassigned pool", err)
		}
		for _, team := range teams {
			if team == nil || team.Department != string(domain.DepartmentUnassigned) || !team.Enabled {
				continue
			}
			trimmed := strings.TrimSpace(team.Name)
			if trimmed != "" {
				return trimmed, nil
			}
		}
	}
	teams := s.authSettings.DepartmentTeams[string(domain.DepartmentUnassigned)]
	for _, team := range teams {
		trimmed := strings.TrimSpace(team)
		if trimmed != "" {
			return trimmed, nil
		}
	}
	return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "unassigned pool team is not configured", map[string]interface{}{
		"department": domain.DepartmentUnassigned,
	})
}

func (s *identityService) validateManagedDepartments(raw []string) ([]string, *domain.AppError) {
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, department := range raw {
		department = strings.TrimSpace(department)
		if department == "" {
			continue
		}
		if appErr := s.validateDepartment(domain.Department(department)); appErr != nil {
			return nil, appErr
		}
		if _, ok := seen[department]; ok {
			continue
		}
		seen[department] = struct{}{}
		out = append(out, department)
	}
	return out, nil
}

func (s *identityService) resolveCreateManagedDepartments(department domain.Department, roles []domain.Role, explicit *[]string) ([]string, *domain.AppError) {
	if explicit != nil {
		return s.validateManagedDepartments(*explicit)
	}
	if !containsAnyRole(roles, domain.RoleDeptAdmin, domain.RoleDesignDirector) {
		return nil, nil
	}
	trimmedDepartment := strings.TrimSpace(string(department))
	if trimmedDepartment == "" {
		return nil, nil
	}
	return []string{trimmedDepartment}, nil
}

func (s *identityService) validateManagedTeams(department domain.Department, raw []string) ([]string, *domain.AppError) {
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, team := range raw {
		team = strings.TrimSpace(team)
		if team == "" {
			continue
		}
		if appErr := s.validateTeam(department, team); appErr != nil {
			return nil, appErr
		}
		if _, ok := seen[team]; ok {
			continue
		}
		seen[team] = struct{}{}
		out = append(out, team)
	}
	return out, nil
}

func (s *identityService) authorizeUserListFilter(ctx context.Context, filter *UserFilter) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return nil
	}
	_ = filter
	if permission := identityReadPermission(actor); !domain.ActorHasPermission(actor, permission) {
		s.emitAuthorizeDefaultDenyTelemetry(ctx, "authorize_user_list_filter_denied", "access_view_required", actor, nil)
		return identityPermissionDenied("access_view_required", "access.view or access.manage is required")
	}
	return nil
}

func (s *identityService) authorizeUserRead(ctx context.Context, user *domain.User) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || user == nil {
		return nil
	}
	permission := identityReadPermission(actor)
	if !identityAccessAllowsUser(actor, permission, user) {
		s.emitAuthorizeDefaultDenyTelemetry(ctx, "authorize_user_read_denied", "access_scope_denied", actor, user)
		return identityPermissionDenied("access_scope_denied", "the user is outside the granted access scope")
	}
	return nil
}

// emitAuthorizeDefaultDenyTelemetry emits a warn-level structured log entry
// when authorizeUserRead / authorizeUserListFilter fall through to the
// management_access_required default branch. This is observability-only and
// does not alter deny semantics. targetUser may be nil for list filters.
func (s *identityService) emitAuthorizeDefaultDenyTelemetry(
	ctx context.Context,
	event string,
	denyCode string,
	actor domain.RequestActor,
	targetUser *domain.User,
) {
	if s.logger == nil {
		return
	}
	actorRoleStrings := make([]string, 0, len(actor.Roles))
	for _, role := range actor.Roles {
		actorRoleStrings = append(actorRoleStrings, string(role))
	}
	fields := []zap.Field{
		zap.String("event", event),
		zap.String("trace_id", domain.TraceIDFromContext(ctx)),
		zap.Int64("actor_id", actor.ID),
		zap.String("actor_username", actor.Username),
		zap.Strings("actor_roles", actorRoleStrings),
		zap.String("actor_source", actor.Source),
		zap.String("auth_mode", string(actor.AuthMode)),
		zap.String("actor_department", actor.Department),
		zap.String("actor_team", actor.Team),
		zap.String("deny_code", denyCode),
	}
	if targetUser != nil {
		fields = append(fields,
			zap.Int64("target_user_id", targetUser.ID),
			zap.String("target_department", string(targetUser.Department)),
			zap.String("target_team", targetUser.Team),
		)
	}
	s.logger.Warn(event, fields...)
}

func (s *identityService) authorizeCreateManagedUser(ctx context.Context, user *domain.User) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return nil
	}
	if !identityAccessAllowsUser(actor, domain.PermissionAccessManage, user) {
		return identityPermissionDenied("access_scope_denied", "access.manage does not cover the new user's organization")
	}
	return nil
}

func (s *identityService) authorizeResetUserPassword(ctx context.Context, user *domain.User) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || user == nil {
		return nil
	}
	if !identityAccessAllowsUser(actor, domain.PermissionAccessManage, user) {
		return identityPermissionDenied("access_scope_denied", "access.manage does not cover this user")
	}
	return nil
}

func (s *identityService) authorizeUserUpdate(ctx context.Context, current *domain.User, nextDepartmentID, nextTeamID *int64) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || current == nil {
		return nil
	}
	if !identityAccessAllowsUser(actor, domain.PermissionAccessManage, current) {
		return identityPermissionDenied("access_scope_denied", "access.manage does not cover this user")
	}
	next := *current
	next.DepartmentID = nextDepartmentID
	next.TeamID = nextTeamID
	if !identityAccessAllowsUser(actor, domain.PermissionAccessManage, &next) {
		return identityPermissionDenied("access_scope_denied", "access.manage does not cover the user's new organization")
	}
	return nil
}

func (s *identityService) authorizeUserStatusEndpoint(ctx context.Context, user *domain.User) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || user == nil {
		return nil
	}
	if !identityAccessAllowsUser(actor, domain.PermissionAccessManage, user) {
		return identityPermissionDenied("access_scope_denied", "access.manage does not cover this user")
	}
	return nil
}

func fieldDenied(code, message string) *domain.AppError {
	return domain.NewAppError(domain.ErrCodePermissionDenied, message, map[string]interface{}{"deny_code": code})
}

func identityReadPermission(actor domain.RequestActor) domain.PermissionCode {
	if domain.ActorHasPermission(actor, domain.PermissionAccessManage) {
		return domain.PermissionAccessManage
	}
	return domain.PermissionAccessView
}

func identityAccessAllowsUser(actor domain.RequestActor, permission domain.PermissionCode, user *domain.User) bool {
	if user == nil || !domain.ActorHasPermission(actor, permission) {
		return false
	}
	access := domain.ResourceGroupAccessFilterForActor(actor, permission)
	if access.Global || (access.Self && user.ID > 0 && user.ID == actor.ID) {
		return true
	}
	if user.DepartmentID != nil && containsInt64(access.DepartmentIDs, *user.DepartmentID) {
		return true
	}
	return user.TeamID != nil && containsInt64(access.TeamIDs, *user.TeamID)
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func authorizeGlobalAccessManage(ctx context.Context) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return nil
	}
	if !domain.ActorHasPermission(actor, domain.PermissionAccessManage) {
		return identityPermissionDenied("access_manage_required", "access.manage is required")
	}
	if !domain.ResourceGroupAccessFilterForActor(actor, domain.PermissionAccessManage).Global {
		return identityPermissionDenied("global_access_manage_required", "global access.manage is required")
	}
	return nil
}

func identityPermissionDenied(code, message string) *domain.AppError {
	return domain.NewAppError(domain.ErrCodePermissionDenied, message, map[string]interface{}{
		"deny_code": code,
	})
}

func (s *identityService) validatePassword(password, field string) *domain.AppError {
	password = strings.TrimSpace(password)
	if len(password) < 8 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, field+" must be at least 8 characters", nil)
	}
	var hasLetter, hasNumber bool
	for _, r := range password {
		switch {
		case r >= '0' && r <= '9':
			hasNumber = true
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			hasLetter = true
		}
	}
	if !hasLetter || !hasNumber {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, field+" must include letters and numbers", nil)
	}
	return nil
}

func validateMobile(mobile string) *domain.AppError {
	if strings.TrimSpace(mobile) == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "mobile is required", nil)
	}
	if !mobilePattern.MatchString(strings.TrimSpace(mobile)) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "mobile format is invalid", nil)
	}
	return nil
}

func validateOptionalEmail(email string) *domain.AppError {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	if !emailPattern.MatchString(email) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "email format is invalid", nil)
	}
	return nil
}

func requiredEmployeeNo(employeeNo *int) (int, *domain.AppError) {
	if employeeNo == nil {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "工号必填。", map[string]interface{}{"deny_code": "employee_no_required"})
	}
	return optionalEmployeeNo(employeeNo)
}

func optionalEmployeeNo(employeeNo *int) (int, *domain.AppError) {
	if employeeNo == nil {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "工号不能为空。", map[string]interface{}{"deny_code": "employee_no_required"})
	}
	if *employeeNo < 0 || *employeeNo > maxEmployeeNo {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "工号必须是 0 到 9999 之间的纯数字。", map[string]interface{}{"deny_code": "employee_no_invalid"})
	}
	return *employeeNo, nil
}

func (s *identityService) ensureUniqueEmployeeNo(ctx context.Context, employeeNo int, excludeUserID int64) *domain.AppError {
	existing, err := s.userRepo.GetByEmployeeNo(ctx, employeeNo)
	if err != nil {
		return infraError("get user by employee_no", err)
	}
	if existing == nil || existing.ID == excludeUserID {
		return nil
	}
	return employeeNoConflictError(employeeNo, existing)
}

func (s *identityService) employeeNoWriteConflict(ctx context.Context, err error, employeeNo int, excludeUserID int64) *domain.AppError {
	if err == nil {
		return nil
	}
	message := err.Error()
	if !strings.Contains(message, "uq_users_employee_no") && !strings.Contains(message, "employee_no") {
		return nil
	}
	if appErr := s.ensureUniqueEmployeeNo(ctx, employeeNo, excludeUserID); appErr != nil {
		return appErr
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("工号 %d 已被其他账号使用，请核对后再保存。", employeeNo), map[string]interface{}{
		"deny_code":   "employee_no_conflict",
		"employee_no": employeeNo,
	})
}

func employeeNoConflictError(employeeNo int, existing *domain.User) *domain.AppError {
	name := strings.TrimSpace(existing.DisplayName)
	if name == "" {
		name = existing.Username
	}
	return domain.NewAppError(
		domain.ErrCodeInvalidRequest,
		fmt.Sprintf("工号 %d 已被 %s（登录账号 %s）使用，请核对后再保存。", employeeNo, name, existing.Username),
		map[string]interface{}{
			"deny_code":        "employee_no_conflict",
			"employee_no":      employeeNo,
			"existing_user_id": existing.ID,
		},
	)
}

func ensureMemberRole(roles []domain.Role) []domain.Role {
	if containsRole(roles, domain.RoleMember) {
		return domain.NormalizeRoleValues(roles)
	}
	return mergeRoles(roles, []domain.Role{domain.RoleMember})
}

func normalizeManagedAvatarURLForService(raw string) (string, *domain.AppError) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if !managedAvatarURLPattern.MatchString(value) {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "avatar must be uploaded through profile avatar upload", map[string]string{"deny_code": "avatar_url_not_managed"})
	}
	return value, nil
}

func (s *identityService) ensureUniqueIdentity(ctx context.Context, username, mobile string, excludeUserID int64) *domain.AppError {
	if existing, err := s.userRepo.GetByUsername(ctx, username); err != nil {
		return infraError("get user by username", err)
	} else if existing != nil && existing.ID != excludeUserID {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "account already exists", nil)
	}
	if s.authSettings.PhoneUnique {
		if existing, err := s.userRepo.GetByMobile(ctx, mobile); err != nil {
			return infraError("get user by mobile", err)
		} else if existing != nil && existing.ID != excludeUserID {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "mobile already exists", nil)
		}
	}
	return nil
}

func (s *identityService) ensurePrivilegedUserStatusSafety(ctx context.Context, user *domain.User, nextStatus domain.UserStatus) *domain.AppError {
	if user == nil || nextStatus == domain.UserStatusActive || user.Status == nextStatus {
		return nil
	}
	if !containsRole(user.Roles, domain.RoleSuperAdmin) {
		return nil
	}
	activeAdminUsers, err := s.activeUsersWithRoles(ctx, domain.RoleSuperAdmin)
	if err != nil {
		return infraError("list active super admin users", err)
	}
	if len(activeAdminUsers) <= 1 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "at least one active SuperAdmin user must remain", map[string]interface{}{
			"deny_code": "last_super_admin_deactivate_denied",
		})
	}
	return nil
}

func (s *identityService) activeUsersWithRoles(ctx context.Context, roles ...domain.Role) (map[int64]struct{}, error) {
	userIDs := map[int64]struct{}{}
	active := domain.UserStatusActive
	for _, role := range roles {
		role := role
		users, _, err := s.userRepo.List(ctx, repo.UserListFilter{
			Status:   &active,
			Role:     &role,
			Page:     1,
			PageSize: 1000,
		})
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			if user != nil {
				userIDs[user.ID] = struct{}{}
			}
		}
	}
	return userIDs, nil
}

func (s *identityService) normalizeConfiguredSuperAdminOrg(entry domain.ConfiguredSuperAdmin) domain.ConfiguredSuperAdmin {
	if s.orgRepo == nil {
		return entry
	}
	entry.Department = domain.Department(s.normalizeConfiguredDepartment(string(entry.Department)))
	entry.Team = s.normalizeConfiguredTeam(entry.Department, entry.Team)
	entry.ManagedDepartments = s.normalizeConfiguredDepartments(entry.ManagedDepartments)
	entry.ManagedTeams = s.normalizeConfiguredTeams(entry.Department, entry.ManagedTeams)
	return entry
}

func (s *identityService) normalizeConfiguredDepartments(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := s.normalizeConfiguredDepartment(value)
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	return out
}

func (s *identityService) normalizeConfiguredTeams(department domain.Department, values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := s.normalizeConfiguredTeam(department, value)
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	return out
}

func (s *identityService) normalizeConfiguredDepartment(value string) string {
	raw := strings.TrimSpace(value)
	normalized := domain.NormalizeOrgDepartmentAlias(raw)
	if normalized != "" && s.orgDepartmentExists(normalized) {
		return normalized
	}
	return raw
}

func (s *identityService) normalizeConfiguredTeam(department domain.Department, value string) string {
	raw := strings.TrimSpace(value)
	normalized := domain.NormalizeOrgTeamAlias(raw)
	if normalized != "" && s.orgTeamExists(department, normalized) {
		return normalized
	}
	return raw
}

func (s *identityService) orgDepartmentExists(department string) bool {
	item, err := s.orgRepo.GetDepartmentByName(context.Background(), strings.TrimSpace(department))
	return err == nil && item != nil && item.Enabled
}

func (s *identityService) orgTeamExists(department domain.Department, team string) bool {
	departmentItem, err := s.orgRepo.GetDepartmentByName(context.Background(), strings.TrimSpace(string(department)))
	if err != nil || departmentItem == nil || !departmentItem.Enabled {
		return false
	}
	teams, err := s.orgRepo.ListTeams(context.Background(), false)
	if err != nil {
		return false
	}
	for _, candidate := range teams {
		if candidate != nil && candidate.DepartmentID == departmentItem.ID && candidate.Enabled && candidate.Name == strings.TrimSpace(team) {
			return true
		}
	}
	return false
}

func (s *identityService) upsertConfiguredSuperAdmin(ctx context.Context, entry domain.ConfiguredSuperAdmin) *domain.AppError {
	existing, err := s.userRepo.GetByUsername(ctx, entry.Username)
	if err != nil {
		return infraError("get configured super admin by username", err)
	}
	if appErr := s.ensureUniqueIdentity(ctx, entry.Username, entry.Mobile, existingUserID(existing)); appErr != nil {
		return appErr
	}
	roles, appErr := s.resolveConfiguredSuperAdminRoles(entry)
	if appErr != nil {
		return appErr
	}
	managedDepartments, appErr := s.resolveConfiguredSuperAdminManagedDepartments(entry)
	if appErr != nil {
		return appErr
	}
	managedTeams, appErr := s.resolveConfiguredSuperAdminManagedTeams(entry)
	if appErr != nil {
		return appErr
	}
	status, appErr := resolveConfiguredSuperAdminStatus(entry)
	if appErr != nil {
		return appErr
	}
	employmentType, appErr := resolveConfiguredSuperAdminEmploymentType(entry)
	if appErr != nil {
		return appErr
	}
	now := time.Now().UTC()
	if existing == nil {
		if strings.TrimSpace(entry.Password) == "" {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "configured super admin password is required when creating a new config-managed admin", nil)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(entry.Password), bcrypt.DefaultCost)
		if err != nil {
			return infraError("hash configured super admin password", err)
		}
		user := &domain.User{
			Username:           entry.Username,
			DisplayName:        strings.TrimSpace(entry.DisplayName),
			Department:         entry.Department,
			Team:               strings.TrimSpace(entry.Team),
			ManagedDepartments: managedDepartments,
			ManagedTeams:       managedTeams,
			Mobile:             strings.TrimSpace(entry.Mobile),
			Email:              strings.TrimSpace(entry.Email),
			PasswordHash:       string(hash),
			Status:             status,
			EmploymentType:     employmentType,
			IsConfigSuperAdmin: true,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			userID, err := s.userRepo.Create(ctx, tx, user)
			if err != nil {
				return err
			}
			user.ID = userID
			if err := s.userRepo.ReplaceRoles(ctx, tx, userID, roles); err != nil {
				return err
			}
			return s.ensureExplicitRoleAssignments(ctx, tx, userID,
				explicitIdentityRole{Code: "member", Scope: domain.AccessScopeSelf},
				explicitIdentityRole{Code: "super_admin", Scope: domain.AccessScopeGlobal},
			)
		}); err != nil {
			return infraError("create configured super admin", err)
		}
		s.recordSystemPermissionAction(ctx, domain.PermissionLog{
			ActionType:     domain.PermissionActionRoleAssigned,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    roles,
			Granted:        true,
			Reason:         "config-managed super admin created",
			Method:         "SYSTEM",
			RoutePath:      "config/auth_identity.json",
		})
		return nil
	}

	existing.DisplayName = strings.TrimSpace(entry.DisplayName)
	existing.Department = entry.Department
	existing.Team = strings.TrimSpace(entry.Team)
	existing.ManagedDepartments = managedDepartments
	existing.ManagedTeams = managedTeams
	existing.Mobile = strings.TrimSpace(entry.Mobile)
	existing.Email = strings.TrimSpace(entry.Email)
	existing.Status = status
	existing.EmploymentType = employmentType
	existing.IsConfigSuperAdmin = true
	existing.UpdatedAt = now
	if len(entry.Roles) == 0 {
		currentRoles, err := s.userRepo.ListRoles(ctx, existing.ID)
		if err != nil {
			return infraError("list existing super admin roles", err)
		}
		roles = mergeRoles(currentRoles, roles)
	}
	var passwordHash string
	if strings.TrimSpace(entry.Password) != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(entry.Password), bcrypt.DefaultCost)
		if err != nil {
			return infraError("hash configured super admin password", err)
		}
		passwordHash = string(hash)
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.userRepo.Update(ctx, tx, existing); err != nil {
			return err
		}
		if passwordHash != "" {
			if err := s.userRepo.UpdatePassword(ctx, tx, existing.ID, passwordHash, now); err != nil {
				return err
			}
		}
		if err := s.userRepo.ReplaceRoles(ctx, tx, existing.ID, roles); err != nil {
			return err
		}
		return s.ensureExplicitRoleAssignments(ctx, tx, existing.ID,
			explicitIdentityRole{Code: "member", Scope: domain.AccessScopeSelf},
			explicitIdentityRole{Code: "super_admin", Scope: domain.AccessScopeGlobal},
		)
	}); err != nil {
		return infraError("update configured super admin", err)
	}
	return nil
}

func (s *identityService) resolveConfiguredSuperAdminRoles(entry domain.ConfiguredSuperAdmin) ([]domain.Role, *domain.AppError) {
	if len(entry.Roles) == 0 {
		return []domain.Role{domain.RoleMember, domain.RoleSuperAdmin}, nil
	}
	return validateRoleInputs(entry.Roles)
}

func (s *identityService) resolveConfiguredSuperAdminManagedDepartments(entry domain.ConfiguredSuperAdmin) ([]string, *domain.AppError) {
	return s.validateManagedDepartments(entry.ManagedDepartments)
}

func (s *identityService) resolveConfiguredSuperAdminManagedTeams(entry domain.ConfiguredSuperAdmin) ([]string, *domain.AppError) {
	return s.validateManagedTeams(entry.Department, entry.ManagedTeams)
}

func resolveConfiguredSuperAdminStatus(entry domain.ConfiguredSuperAdmin) (domain.UserStatus, *domain.AppError) {
	if entry.Status == "" {
		return domain.UserStatusActive, nil
	}
	if !entry.Status.Valid() {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "configured super admin status is invalid", nil)
	}
	return entry.Status, nil
}

func resolveConfiguredSuperAdminEmploymentType(entry domain.ConfiguredSuperAdmin) (domain.EmploymentType, *domain.AppError) {
	if entry.EmploymentType == "" {
		return domain.EmploymentTypeFullTime, nil
	}
	if !entry.EmploymentType.Valid() {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "configured super admin employment type is invalid", nil)
	}
	return entry.EmploymentType, nil
}

func validateRoleInputs(raw []domain.Role) ([]domain.Role, *domain.AppError) {
	if len(raw) == 0 {
		return []domain.Role{}, nil
	}
	roles := make([]domain.Role, 0, len(raw))
	for _, value := range raw {
		role := domain.Role(strings.TrimSpace(string(value)))
		if role == "" || !domain.IsKnownRole(role) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "one or more roles are invalid", nil)
		}
		roles = append(roles, role)
	}
	return domain.NormalizeRoleValues(roles), nil
}

func validateSingleRole(raw domain.Role) (domain.Role, *domain.AppError) {
	role := domain.Role(strings.TrimSpace(string(raw)))
	if role == "" || !domain.IsKnownRole(role) {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "role is invalid", nil)
	}
	return role, nil
}

func mergeRoles(current, additions []domain.Role) []domain.Role {
	merged := make([]domain.Role, 0, len(current)+len(additions))
	merged = append(merged, current...)
	merged = append(merged, additions...)
	return domain.NormalizeRoleValues(merged)
}

func removeRole(current []domain.Role, role domain.Role) []domain.Role {
	next := make([]domain.Role, 0, len(current))
	for _, item := range domain.NormalizeRoleValues(current) {
		if item == role {
			continue
		}
		next = append(next, item)
	}
	return next
}

func containsRole(roles []domain.Role, target domain.Role) bool {
	for _, role := range domain.NormalizeRoleValues(roles) {
		if role == target {
			return true
		}
	}
	return false
}

func containsAnyRole(roles []domain.Role, targets ...domain.Role) bool {
	for _, target := range targets {
		if containsRole(roles, target) {
			return true
		}
	}
	return false
}

func containsStringValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sameStringSlice(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func actorIDPtr(actorID int64) *int64 {
	if actorID <= 0 {
		return nil
	}
	id := actorID
	return &id
}

func existingUserID(user *domain.User) int64 {
	if user == nil {
		return 0
	}
	return user.ID
}

func generateSessionToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(buf)
	return token, hashToken(token), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func normalizeUsername(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// defaultAuthSettings is the in-memory fallback used when no auth_identity
// config file is present (primarily test paths). It intentionally keeps the
// legacy department/team compatibility values so historical tests and
// persisted user rows remain valid. The runtime authoritative source is
// config/auth_identity.json, which has been converged to the v1.0 official
// baseline; this Go fallback is not served as production data.
func defaultAuthSettings() domain.AuthSettings {
	return domain.AuthSettings{
		// The in-code fallback must keep recognizing retired departments and
		// teams because persisted users/tasks may still reference them; the
		// org-master seeding path filters compatibility entries separately.
		Departments:     append(domain.DefaultDepartments(), domain.CompatibilityDepartments()...),
		DepartmentTeams: domain.MergedOrgDepartmentTeamsWithCompatibility(),
		PhoneUnique:     true,
		DepartmentAdminKeys: map[string][]string{
			string(domain.DepartmentHR):               {"superAdmin"},
			string(domain.DepartmentDesignRD):         {"superAdmin"},
			string(domain.DepartmentCustomizationArt): {"superAdmin"},
			string(domain.DepartmentAudit):            {"superAdmin"},
			string(domain.DepartmentOperations):       {"superAdmin"},
			string(domain.DepartmentCloudWarehouse):   {"superAdmin"},
			string(domain.DepartmentUnassigned):       {"superAdmin"},
			string(domain.DepartmentDesign):           {"superAdmin"},
			string(domain.DepartmentProcurement):      {"superAdmin"},
			string(domain.DepartmentWarehouse):        {"superAdmin"},
			string(domain.DepartmentBakeryWH):         {"superAdmin"},
		},
		SuperAdmins: []domain.ConfiguredSuperAdmin{
			{
				Username:    "admin",
				DisplayName: "系统管理员",
				Department:  domain.DepartmentUnassigned,
				Team:        "未分配池",
				Mobile:      "13900000000",
				Password:    "ChangeMeAdmin123",
			},
			{
				Username:           "HRAdmin",
				Password:           "ChangeMeAdmin123",
				DisplayName:        "HR 管理员",
				Mobile:             "13900000001",
				Email:              "hradmin@seed.local",
				Department:         domain.DepartmentHR,
				Team:               "人事管理组",
				Roles:              []domain.Role{domain.RoleHRAdmin, domain.RoleOrgAdmin},
				ManagedDepartments: []string{string(domain.DepartmentHR)},
				Status:             domain.UserStatusActive,
				EmploymentType:     domain.EmploymentTypeFullTime,
			},
		},
		UnassignedPoolEnabled: true,
		TaskTeamMappings:      domain.DefaultTaskTeamMappings(),
		ConfiguredAssignments: []domain.ConfiguredUserAssignment{
			{
				DisplayName:        "刘芸菲",
				Department:         domain.DepartmentHR,
				Team:               "人事管理组",
				Roles:              []domain.Role{domain.RoleHRAdmin, domain.RoleOrgAdmin},
				ManagedDepartments: []string{string(domain.DepartmentHR)},
				Status:             domain.UserStatusActive,
			},
			{
				DisplayName:        "王亚琳",
				Department:         domain.DepartmentDesign,
				Team:               "设计审核组",
				Roles:              []domain.Role{domain.RoleDeptAdmin, domain.RoleDesignDirector},
				ManagedDepartments: []string{string(domain.DepartmentDesign)},
				Status:             domain.UserStatusActive,
			},
			{
				DisplayName: "马雨琪",
				Department:  domain.DepartmentDesign,
				Team:        "设计审核组",
				Roles:       []domain.Role{domain.RoleDesignReviewer},
				Status:      domain.UserStatusActive,
			},
			{
				DisplayName:  "章鹏鹏",
				Department:   domain.DepartmentDesign,
				Team:         "定制美工组",
				Roles:        []domain.Role{domain.RoleTeamLead},
				ManagedTeams: []string{"定制美工组"},
				Status:       domain.UserStatusActive,
			},
			{
				DisplayName:        "方晓兵",
				Department:         domain.DepartmentProcurement,
				Team:               "采购组",
				Roles:              []domain.Role{domain.RoleDeptAdmin},
				ManagedDepartments: []string{string(domain.DepartmentProcurement), string(domain.DepartmentWarehouse), string(domain.DepartmentBakeryWH)},
				Status:             domain.UserStatusActive,
			},
		},
	}
}

func defaultFrontendAccessSettings() domain.FrontendAccessSettings {
	return domain.FrontendAccessSettings{
		Version: "v8-explicit-access",
		Defaults: domain.FrontendAccessDefaults{
			AllAuthenticated: domain.FrontendAccessSpec{
				Roles:   []string{"member"},
				Scopes:  []string{"authenticated"},
				Menus:   []string{"dashboard"},
				Pages:   []string{"dashboard", "profile_me"},
				Actions: []string{string(domain.PermissionAccountUse)},
			},
		},
		Departments: map[string]domain.DepartmentAccessEntry{},
		Teams:       map[string]domain.TeamEntry{},
		Roles:       map[string]domain.FrontendAccessSpec{},
		Identities:  map[string]domain.FrontendAccessSpec{},
	}
}
