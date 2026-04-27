package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
	mobilePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)
	emailPattern  = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

type RegisterUserParams struct {
	Username           string
	DisplayName        string
	Department         domain.Department
	Team               string
	Mobile             string
	Email              string
	Password           string
	AdminKey           string
	ManagedDepartments *[]string
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
	DisplayName        string
	Department         domain.Department
	Team               string
	Mobile             string
	Email              string
	Password           string
	Roles              []domain.Role
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
	AssignableLaneAll           AssignableLane = "all"
)

type UpdateUserParams struct {
	UserID             int64
	DisplayName        *string
	Status             *domain.UserStatus
	EmploymentType     *domain.EmploymentType
	Department         *domain.Department
	Team               *string
	Group              *string
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

type DeleteUserParams struct {
	UserID int64
	Reason string
}

type SetUserRolesParams struct {
	UserID int64
	Roles  []domain.Role
}

type AddUserRolesParams struct {
	UserID int64
	Roles  []domain.Role
}

type RemoveUserRoleParams struct {
	UserID int64
	Role   domain.Role
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
	CreateDepartment(ctx context.Context, p CreateOrgDepartmentParams) (*domain.OrgDepartment, *domain.AppError)
	UpdateDepartment(ctx context.Context, p UpdateOrgDepartmentParams) (*domain.OrgDepartment, *domain.AppError)
	CreateTeam(ctx context.Context, p CreateOrgTeamParams) (*domain.OrgTeam, *domain.AppError)
	UpdateTeam(ctx context.Context, p UpdateOrgTeamParams) (*domain.OrgTeam, *domain.AppError)
	Register(ctx context.Context, p RegisterUserParams) (*domain.AuthResult, *domain.AppError)
	Login(ctx context.Context, p LoginParams) (*domain.AuthResult, *domain.AppError)
	ChangePassword(ctx context.Context, p ChangePasswordParams) *domain.AppError
	CreateManagedUser(ctx context.Context, p CreateManagedUserParams) (*domain.User, *domain.AppError)
	ResetUserPassword(ctx context.Context, p ResetUserPasswordParams) (*domain.User, *domain.AppError)
	GetCurrentUser(ctx context.Context) (*domain.User, *domain.AppError)
	GetMe(ctx context.Context) (*domain.User, *domain.AppError)
	UpdateMe(ctx context.Context, p UpdateMeParams) (*domain.User, *domain.AppError)
	GetMyOrg(ctx context.Context) (*domain.MyOrgProfile, *domain.AppError)
	ListUsers(ctx context.Context, filter UserFilter) ([]*domain.User, domain.PaginationMeta, *domain.AppError)
	// ListAssignableDesigners returns the full set of assignable users for the
	// requested candidate-pool lane. It is the Round D dedicated "assignment
	// candidate pool" service path, extended in Round N with lane-aware role
	// filtering. It intentionally bypasses authorizeUserListFilter so that Ops
	// (the canonical task-creator role) can look up candidates cross-department
	// without widening management access on ListUsers. Access control for this
	// method is enforced exclusively by the route guard mounted on
	// `/v1/users/designers` (see transport/http.go).
	ListAssignableDesigners(ctx context.Context, actor *domain.RequestActor, lane AssignableLane) ([]*domain.User, *domain.AppError)
	GetUser(ctx context.Context, userID int64) (*domain.User, *domain.AppError)
	UpdateUser(ctx context.Context, p UpdateUserParams) (*domain.User, *domain.AppError)
	ActivateUser(ctx context.Context, userID int64) *domain.AppError
	DeactivateUser(ctx context.Context, userID int64) *domain.AppError
	DeleteUser(ctx context.Context, p DeleteUserParams) *domain.AppError
	SetUserRoles(ctx context.Context, p SetUserRolesParams) (*domain.User, *domain.AppError)
	AddUserRoles(ctx context.Context, p AddUserRolesParams) (*domain.User, *domain.AppError)
	RemoveUserRole(ctx context.Context, p RemoveUserRoleParams) (*domain.User, *domain.AppError)
	ListPermissionLogs(ctx context.Context, filter PermissionLogFilter) ([]*domain.PermissionLog, domain.PaginationMeta, *domain.AppError)
	ListRoles(ctx context.Context) []domain.RoleCatalogEntry
	ResolveRequestActor(ctx context.Context, bearerToken string) (*domain.RequestActor, *domain.AppError)
	RecordRouteAccess(ctx context.Context, entry domain.PermissionLog)
}

type IdentityServiceOption func(*identityService)

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
	txRunner               repo.TxRunner
	sessionTTL             time.Duration
	authSettings           domain.AuthSettings
	frontendAccessSettings domain.FrontendAccessSettings
	logger                 *zap.Logger
	orgOptionsOnce         sync.Once
	orgOptionsCache        *domain.OrgOptions
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
		username := normalizeUsername(entry.Username)
		if username == "" {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "configured super admin username is required", nil)
		}
		if appErr := s.validatePassword(entry.Password, "configured super admin password"); appErr != nil {
			return appErr
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

func (s *identityService) Register(ctx context.Context, p RegisterUserParams) (*domain.AuthResult, *domain.AppError) {
	username := normalizeUsername(p.Username)
	displayName := strings.TrimSpace(p.DisplayName)
	team := strings.TrimSpace(p.Team)
	mobile := strings.TrimSpace(p.Mobile)
	email := strings.TrimSpace(p.Email)
	adminKey := strings.TrimSpace(p.AdminKey)

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
	if s.matchesDepartmentAdminKey(p.Department, adminKey) {
		roles = append(roles, domain.RoleDeptAdmin)
		for _, businessRole := range domain.DepartmentDefaultBusinessRoles(p.Department) {
			if !containsRole(roles, businessRole) {
				roles = append(roles, businessRole)
			}
		}
	}
	managedDepartments, appErr := s.resolveCreateManagedDepartments(p.Department, roles, p.ManagedDepartments)
	if appErr != nil {
		return nil, appErr
	}

	now := time.Now().UTC()
	user := &domain.User{
		Username:           username,
		DisplayName:        displayName,
		Department:         p.Department,
		Team:               team,
		Mobile:             mobile,
		Email:              email,
		PasswordHash:       string(hash),
		Status:             domain.UserStatusActive,
		EmploymentType:     domain.EmploymentTypeFullTime,
		ManagedDepartments: managedDepartments,
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
	reason := "user registered"
	if containsRole(user.Roles, domain.RoleDeptAdmin) {
		reason = "user registered as department admin"
	}
	s.recordPermissionAction(ctx, domain.PermissionLog{
		ActionType:     domain.PermissionActionRegister,
		TargetUserID:   actorIDPtr(user.ID),
		TargetUsername: user.Username,
		TargetRoles:    user.Roles,
		Granted:        true,
		Reason:         reason,
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
	params := UpdateUserParams{
		UserID:      user.ID,
		DisplayName: p.DisplayName,
		Mobile:      p.Mobile,
		Email:       p.Email,
	}
	// Self profile edits are allowed for the account owner even without
	// organization-management roles; keep the writable set narrow.
	return s.updateUserBypassManagementScope(ctx, user, params, "PATCH", "/v1/me")
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

	roles, appErr := validateRoleInputs(p.Roles)
	if appErr != nil {
		return nil, appErr
	}
	if len(roles) == 0 {
		roles = []domain.Role{domain.RoleMember}
	}
	if appErr := s.authorizeCreateManagedUser(ctx, p.Department, roles); appErr != nil {
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
		DisplayName:        displayName,
		Department:         p.Department,
		Team:               team,
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
		return s.userRepo.ReplaceRoles(ctx, tx, userID, roles)
	}); err != nil {
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
	return user, nil
}

func (s *identityService) ListUsers(ctx context.Context, filter UserFilter) ([]*domain.User, domain.PaginationMeta, *domain.AppError) {
	if filter.Role != nil && *filter.Role != "" && !domain.IsKnownRole(*filter.Role) {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "role is invalid", nil)
	}
	if filter.Department != nil {
		if appErr := s.validateDepartment(*filter.Department); appErr != nil {
			return nil, domain.PaginationMeta{}, appErr
		}
	}
	if appErr := s.authorizeUserListFilter(ctx, &filter); appErr != nil {
		return nil, domain.PaginationMeta{}, appErr
	}
	users, total, err := s.userRepo.List(ctx, repo.UserListFilter{
		Keyword:    filter.Keyword,
		Status:     filter.Status,
		Role:       filter.Role,
		Department: filter.Department,
		Team:       strings.TrimSpace(filter.Team),
		Page:       filter.Page,
		PageSize:   filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list users", err)
	}
	if err := s.attachRolesForUsers(ctx, users); err != nil {
		return nil, domain.PaginationMeta{}, infraError("attach user roles", err)
	}
	return users, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

// ListAssignableDesigners implements the dedicated assignment candidate-pool
// service path for `/v1/users/designers`. It returns every active user for the
// requested lane regardless of the actor's department/team scope and is
// intentionally NOT routed through authorizeUserListFilter — the route guard
// mounted on `/v1/users/designers` is the sole access control for this method.
// The method accepts no keyword/department/team/pagination filter; its only
// implicit filter is the lane-selected role plus status=active.
func (s *identityService) ListAssignableDesigners(ctx context.Context, actor *domain.RequestActor, lane AssignableLane) ([]*domain.User, *domain.AppError) {
	if actor == nil || actor.ID <= 0 {
		return nil, domain.ErrUnauthorized
	}
	var users []*domain.User
	switch lane {
	case "", AssignableLaneNormal:
		normalUsers, err := s.userRepo.ListActiveByRole(ctx, domain.RoleDesigner)
		if err != nil {
			return nil, infraError("list assignable designers", err)
		}
		users = normalUsers
	case AssignableLaneCustomization:
		customizationUsers, err := s.userRepo.ListActiveByRole(ctx, domain.RoleCustomizationOperator)
		if err != nil {
			return nil, infraError("list assignable customization operators", err)
		}
		users = customizationUsers
	case AssignableLaneAll:
		normalUsers, err := s.userRepo.ListActiveByRole(ctx, domain.RoleDesigner)
		if err != nil {
			return nil, infraError("list assignable designers", err)
		}
		customizationUsers, err := s.userRepo.ListActiveByRole(ctx, domain.RoleCustomizationOperator)
		if err != nil {
			return nil, infraError("list assignable customization operators", err)
		}
		users = dedupeAssignableUsersByID(normalUsers, customizationUsers)
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
	if err := s.attachRolesForUsers(ctx, users); err != nil {
		return nil, infraError("attach assignable designer roles", err)
	}
	for _, user := range users {
		s.prepareUserForResponse(user)
	}
	return users, nil
}

func dedupeAssignableUsersByID(groups ...[]*domain.User) []*domain.User {
	seen := make(map[int64]struct{})
	out := make([]*domain.User, 0)
	for _, group := range groups {
		for _, user := range group {
			if user == nil {
				continue
			}
			if _, ok := seen[user.ID]; ok {
				continue
			}
			seen[user.ID] = struct{}{}
			out = append(out, user)
		}
	}
	return out
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

	teamInput, teamProvided, appErr := resolveTeamPatchInput(p.Team, p.Group)
	if appErr != nil {
		return nil, appErr
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
	if appErr := s.authorizeUserUpdate(ctx, user, p, nextDepartment, nextTeam); appErr != nil {
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
		return nil, infraError("update user tx", err)
	}
	updated, appErr := s.GetUser(ctx, p.UserID)
	if appErr != nil {
		return nil, appErr
	}
	s.recordUserUpdateLogs(ctx, updated, changes)
	return updated, nil
}

func (s *identityService) updateUserBypassManagementScope(ctx context.Context, user *domain.User, p UpdateUserParams, method, routePath string) (*domain.User, *domain.AppError) {
	changes := make([]string, 0, 3)
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

func (s *identityService) DeleteUser(ctx context.Context, p DeleteUserParams) *domain.AppError {
	if p.UserID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id is required", nil)
	}
	reason := strings.TrimSpace(p.Reason)
	if reason == "" {
		return domain.NewAppError(domain.ErrCodeReasonRequired, "reason is required", map[string]string{"deny_code": "reason_required"})
	}
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || !identityActorHasAnyRole(actor, domain.RoleSuperAdmin, domain.RoleAdmin) {
		return identityPermissionDenied("module_action_role_denied", "only SuperAdmin can delete users")
	}
	user, err := s.userRepo.GetByID(ctx, p.UserID)
	if err != nil {
		return infraError("get user for delete", err)
	}
	if user == nil {
		return domain.ErrNotFound
	}
	if err := s.attachRoles(ctx, user); err != nil {
		return infraError("attach delete user roles", err)
	}
	user.Status = domain.UserStatusDeleted
	user.UpdatedAt = time.Now().UTC()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.userRepo.Update(ctx, tx, user); err != nil {
			return err
		}
		return s.recordPermissionActionTx(ctx, tx, domain.PermissionLog{
			ActionType:     domain.PermissionActionUserDeleted,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    user.Roles,
			Granted:        true,
			Reason:         reason,
			Method:         "DELETE",
			RoutePath:      "/v1/users/:id",
		})
	}); err != nil {
		return infraError("delete user tx", err)
	}
	return nil
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

func (s *identityService) SetUserRoles(ctx context.Context, p SetUserRolesParams) (*domain.User, *domain.AppError) {
	roles, appErr := validateRoleInputs(p.Roles)
	if appErr != nil {
		return nil, appErr
	}
	return s.applyUserRoleChange(ctx, p.UserID, roles, "PUT", "/v1/users/:id/roles")
}

func (s *identityService) AddUserRoles(ctx context.Context, p AddUserRolesParams) (*domain.User, *domain.AppError) {
	roles, appErr := validateRoleInputs(p.Roles)
	if appErr != nil {
		return nil, appErr
	}
	user, appErr := s.GetUser(ctx, p.UserID)
	if appErr != nil {
		return nil, appErr
	}
	return s.applyUserRoleChange(ctx, p.UserID, mergeRoles(user.Roles, roles), "POST", "/v1/users/:id/roles")
}

func (s *identityService) RemoveUserRole(ctx context.Context, p RemoveUserRoleParams) (*domain.User, *domain.AppError) {
	if p.UserID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id is required", nil)
	}
	role, appErr := validateSingleRole(p.Role)
	if appErr != nil {
		return nil, appErr
	}
	user, appErr := s.GetUser(ctx, p.UserID)
	if appErr != nil {
		return nil, appErr
	}
	return s.applyUserRoleChange(ctx, p.UserID, removeRole(user.Roles, role), "DELETE", "/v1/users/:id/roles/:role")
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

func (s *identityService) ListRoles(_ context.Context) []domain.RoleCatalogEntry {
	return domain.DefaultRoleCatalog()
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
		Team:               user.Team,
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
		Team:               user.Team,
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
			s.prepareUserForResponse(user)
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
		RoleCatalogSummary:    append([]domain.RoleCatalogEntry{}, options.RoleCatalogSummary...),
		UnassignedPoolEnabled: options.UnassignedPoolEnabled,
		ConfiguredAssignments: append([]domain.ConfiguredUserAssignment{}, options.ConfiguredAssignments...),
	}
	for _, department := range options.Departments {
		teamItems := make([]domain.OrgTeamOption, 0, len(department.TeamItems))
		for _, item := range department.TeamItems {
			teamItems = append(teamItems, domain.OrgTeamOption{
				ID:      item.ID,
				Name:    item.Name,
				Enabled: item.Enabled,
			})
		}
		cloned.Departments = append(cloned.Departments, domain.DepartmentOption{
			ID:        department.ID,
			Name:      department.Name,
			Teams:     append([]string{}, department.Teams...),
			TeamItems: teamItems,
			Enabled:   department.Enabled,
		})
	}
	for department, teams := range options.TeamsByDepartment {
		cloned.TeamsByDepartment[department] = append([]string{}, teams...)
	}
	return cloned
}

func (s *identityService) applyUserRoleChange(ctx context.Context, userID int64, roles []domain.Role, method, routePath string) (*domain.User, *domain.AppError) {
	if userID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id is required", nil)
	}
	user, appErr := s.GetUser(ctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	currentRoles := domain.NormalizeRoleValues(user.Roles)
	nextRoles := domain.NormalizeRoleValues(roles)
	if appErr := s.authorizeUserRoleChange(ctx, user, nextRoles); appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureAdminRoleSafety(ctx, currentRoles, nextRoles); appErr != nil {
		return nil, appErr
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.userRepo.ReplaceRoles(ctx, tx, userID, nextRoles)
	}); err != nil {
		return nil, infraError("replace user roles tx", err)
	}
	updated, appErr := s.GetUser(ctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	addedRoles, removedRoles := diffRoles(currentRoles, nextRoles)
	s.recordRoleChangeLogs(ctx, updated, addedRoles, removedRoles, method, routePath)
	return updated, nil
}

func (s *identityService) ensureAdminRoleSafety(ctx context.Context, currentRoles, nextRoles []domain.Role) *domain.AppError {
	currentAdminClass := containsAnyRole(currentRoles, domain.RoleAdmin, domain.RoleSuperAdmin)
	nextAdminClass := containsAnyRole(nextRoles, domain.RoleAdmin, domain.RoleSuperAdmin)
	if !currentAdminClass || nextAdminClass {
		return nil
	}
	activeAdminUsers, err := s.activeUsersWithRoles(ctx, domain.RoleAdmin, domain.RoleSuperAdmin)
	if err != nil {
		return infraError("list admin users", err)
	}
	if len(activeAdminUsers) <= 1 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "at least one admin user must remain", nil)
	}
	return nil
}

func (s *identityService) recordRoleChangeLogs(ctx context.Context, user *domain.User, addedRoles, removedRoles []domain.Role, method, routePath string) {
	if user == nil {
		return
	}
	if len(addedRoles) > 0 {
		s.recordPermissionAction(ctx, domain.PermissionLog{
			ActionType:     domain.PermissionActionRoleAssigned,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    addedRoles,
			Granted:        true,
			Reason:         "roles assigned",
			Method:         method,
			RoutePath:      routePath,
		})
	}
	if len(removedRoles) > 0 {
		s.recordPermissionAction(ctx, domain.PermissionLog{
			ActionType:     domain.PermissionActionRoleRemoved,
			TargetUserID:   actorIDPtr(user.ID),
			TargetUsername: user.Username,
			TargetRoles:    removedRoles,
			Granted:        true,
			Reason:         "roles removed",
			Method:         method,
			RoutePath:      routePath,
		})
	}
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
	if len(user.Roles) == 0 {
		user.Roles = []domain.Role{domain.RoleMember}
	}
	if !user.EmploymentType.Valid() {
		user.EmploymentType = domain.EmploymentTypeFullTime
	}
	user.Account = user.Username
	user.Name = user.DisplayName
	user.Group = user.Team
	user.Phone = user.Mobile
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

func resolveTeamPatchInput(team, group *string) (string, bool, *domain.AppError) {
	trimmedTeam := ""
	trimmedGroup := ""
	if team != nil {
		trimmedTeam = strings.TrimSpace(*team)
	}
	if group != nil {
		trimmedGroup = strings.TrimSpace(*group)
	}
	if team != nil && group != nil && trimmedTeam != trimmedGroup {
		return "", false, domain.NewAppError(domain.ErrCodeInvalidRequest, "team and group must be the same when both are provided", map[string]interface{}{
			"team":  trimmedTeam,
			"group": trimmedGroup,
		})
	}
	if team != nil {
		return trimmedTeam, true, nil
	}
	if group != nil {
		return trimmedGroup, true, nil
	}
	return "", false, nil
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
	switch {
	case identityActorCanManageAllUsers(actor), identityActorHasAnyRole(actor, domain.RoleOrgAdmin, domain.RoleRoleAdmin):
		return nil
	case identityActorHasAnyRole(actor, domain.RoleDeptAdmin):
		department := identityActorDepartment(actor)
		if department == "" {
			return identityPermissionDenied("department_admin_scope_missing", "department admin scope is not configured")
		}
		if filter.Department != nil &&
			*filter.Department != domain.Department(department) &&
			*filter.Department != domain.DepartmentUnassigned {
			return identityPermissionDenied("department_scope_only", "department admin can only view users in own department")
		}
		if filter.Department == nil {
			dept := domain.Department(department)
			filter.Department = &dept
		}
		filter.Team = strings.TrimSpace(filter.Team)
		return nil
	case identityActorHasAnyRole(actor, domain.RoleTeamLead):
		department := identityActorDepartment(actor)
		team := identityActorTeam(actor)
		if department == "" || team == "" {
			return identityPermissionDenied("team_scope_missing", "team lead scope is not configured")
		}
		if filter.Department != nil && *filter.Department != domain.Department(department) {
			return identityPermissionDenied("team_scope_only", "team lead can only view users in own team")
		}
		if trimmedTeam := strings.TrimSpace(filter.Team); trimmedTeam != "" && !strings.EqualFold(trimmedTeam, team) {
			return identityPermissionDenied("team_scope_only", "team lead can only view users in own team")
		}
		dept := domain.Department(department)
		filter.Department = &dept
		filter.Team = team
		return nil
	default:
		s.emitAuthorizeDefaultDenyTelemetry(ctx, "authorize_user_list_filter_denied", actor, nil)
		return identityPermissionDenied("management_access_required", "management access is required")
	}
}

func (s *identityService) authorizeUserRead(ctx context.Context, user *domain.User) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || user == nil {
		return nil
	}
	switch {
	case identityActorCanManageAllUsers(actor), identityActorHasAnyRole(actor, domain.RoleOrgAdmin, domain.RoleRoleAdmin):
		return nil
	case identityActorHasAnyRole(actor, domain.RoleDeptAdmin):
		if strings.EqualFold(identityActorDepartment(actor), string(user.Department)) ||
			user.Department == domain.DepartmentUnassigned {
			return nil
		}
		return identityPermissionDenied("department_scope_only", "department admin can only access users in own department")
	case identityActorHasAnyRole(actor, domain.RoleTeamLead):
		if strings.EqualFold(identityActorDepartment(actor), string(user.Department)) &&
			strings.EqualFold(identityActorTeam(actor), user.Team) {
			return nil
		}
		return identityPermissionDenied("team_scope_only", "team lead can only access users in own team")
	default:
		s.emitAuthorizeDefaultDenyTelemetry(ctx, "authorize_user_read_denied", actor, user)
		return identityPermissionDenied("management_access_required", "management access is required")
	}
}

// emitAuthorizeDefaultDenyTelemetry emits a warn-level structured log entry
// when authorizeUserRead / authorizeUserListFilter fall through to the
// management_access_required default branch. This is observability-only and
// does not alter deny semantics. targetUser may be nil for list filters.
func (s *identityService) emitAuthorizeDefaultDenyTelemetry(
	ctx context.Context,
	event string,
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
		zap.String("deny_code", "management_access_required"),
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

func (s *identityService) emitAuthorizeRoleChangeDeniedTelemetry(
	ctx context.Context,
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
	requiredRoles := []string{string(domain.RoleHRAdmin), string(domain.RoleSuperAdmin)}
	fields := []zap.Field{
		zap.String("event", "authorize_user_role_change_denied"),
		zap.String("trace_id", domain.TraceIDFromContext(ctx)),
		zap.Int64("actor_id", actor.ID),
		zap.String("actor_username", actor.Username),
		zap.Strings("actor_roles", actorRoleStrings),
		zap.String("actor_source", actor.Source),
		zap.String("auth_mode", string(actor.AuthMode)),
		zap.String("actor_department", actor.Department),
		zap.String("actor_team", actor.Team),
		zap.String("deny_code", "role_change_not_allowed"),
		zap.Strings("required_roles", requiredRoles),
	}
	if targetUser != nil {
		fields = append(fields,
			zap.Int64("target_user_id", targetUser.ID),
			zap.String("target_department", string(targetUser.Department)),
			zap.String("target_team", targetUser.Team),
		)
	}
	s.logger.Warn("authorize_user_role_change_denied", fields...)
}

func (s *identityService) authorizeCreateManagedUser(ctx context.Context, department domain.Department, roles []domain.Role) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return nil
	}
	switch {
	case identityActorCanManageAllUsers(actor):
		return nil
	case identityActorHasAnyRole(actor, domain.RoleDeptAdmin):
		actorDepartment := identityActorDepartment(actor)
		if actorDepartment == "" || !strings.EqualFold(actorDepartment, string(department)) {
			return identityPermissionDenied("department_scope_only", "department admin can only create users in own department")
		}
		for _, role := range roles {
			if !departmentAdminCanAssignRole(domain.Department(actorDepartment), role) {
				return identityPermissionDenied("role_assignment_not_allowed", "department admin can only assign department business roles")
			}
		}
		return nil
	default:
		return identityPermissionDenied("account_create_not_allowed", "only HRAdmin, SuperAdmin, legacy admin compatibility, or DepartmentAdmin can create users")
	}
}

func (s *identityService) authorizeResetUserPassword(ctx context.Context, user *domain.User) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || user == nil {
		return nil
	}
	switch {
	case identityActorCanManageAllUsers(actor):
		return nil
	case identityActorHasAnyRole(actor, domain.RoleDeptAdmin):
		if strings.EqualFold(identityActorDepartment(actor), string(user.Department)) {
			return nil
		}
		return identityPermissionDenied("department_scope_only", "department admin can only reset passwords in own department")
	default:
		return identityPermissionDenied("password_reset_not_allowed", "only HRAdmin, SuperAdmin, legacy admin compatibility, or DepartmentAdmin can reset passwords")
	}
}

func (s *identityService) authorizeUserUpdate(ctx context.Context, current *domain.User, p UpdateUserParams, nextDepartment domain.Department, nextTeam string) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || current == nil {
		return nil
	}
	switch {
	case identityActorCanManageAllUsers(actor):
		if identityActorHasAnyRole(actor, domain.RoleHRAdmin) && p.Department != nil && *p.Department != current.Department {
			return nil
		}
		return nil
	case identityActorHasAnyRole(actor, domain.RoleOrgAdmin):
		if p.Status != nil || p.DisplayName != nil || p.Email != nil || p.Mobile != nil || p.EmploymentType != nil {
			return identityPermissionDenied("org_admin_scope_only", "legacy OrgAdmin compatibility is limited to organization scope updates")
		}
		return nil
	case identityActorHasAnyRole(actor, domain.RoleDeptAdmin):
		if p.ManagedDepartments != nil || p.ManagedTeams != nil {
			return fieldDenied("user_update_field_denied_by_scope", "department admin cannot change managed scope settings")
		}
		actorDepartment := identityActorDepartment(actor)
		if actorDepartment == "" {
			return identityPermissionDenied("department_admin_scope_missing", "department admin scope is not configured")
		}
		targetDepartment := string(current.Department)
		if strings.EqualFold(targetDepartment, string(domain.DepartmentUnassigned)) {
			if !strings.EqualFold(string(nextDepartment), actorDepartment) {
				return fieldDenied("user_update_field_denied_by_scope", "department admin can only assign unassigned users into own department")
			}
			return nil
		}
		if !strings.EqualFold(targetDepartment, actorDepartment) || !strings.EqualFold(string(nextDepartment), actorDepartment) {
			return fieldDenied("user_update_field_denied_by_scope", "department admin can only manage users within own department")
		}
		return nil
	case identityActorHasAnyRole(actor, domain.RoleTeamLead):
		return fieldDenied("user_update_field_denied_by_scope", "team lead cannot update user fields from organization management APIs")
	default:
		return fieldDenied("user_update_field_denied_by_scope", "management access is required")
	}
}

func (s *identityService) authorizeUserRoleChange(ctx context.Context, user *domain.User, nextRoles []domain.Role) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || user == nil {
		return nil
	}
	switch {
	case identityActorHasAnyRole(actor, domain.RoleSuperAdmin):
		return nil
	case identityActorHasAnyRole(actor, domain.RoleHRAdmin):
		if containsRole(nextRoles, domain.RoleSuperAdmin) || containsRole(nextRoles, domain.RoleAdmin) {
			return roleDenied("role_assignment_denied_by_scope", "HRAdmin cannot assign SuperAdmin")
		}
		return nil
	case identityActorHasAnyRole(actor, domain.RoleDeptAdmin):
		if !strings.EqualFold(identityActorDepartment(actor), string(user.Department)) {
			return roleDenied("role_assignment_denied_by_scope", "DepartmentAdmin can only assign roles in own department")
		}
		for _, role := range nextRoles {
			if containsRole([]domain.Role{domain.RoleDeptAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleAdmin}, role) {
				return roleDenied("role_assignment_denied_by_scope", "DepartmentAdmin cannot assign admin roles")
			}
		}
		return nil
	}
	s.emitAuthorizeRoleChangeDeniedTelemetry(ctx, actor, user)
	return roleDenied("role_assignment_denied_by_scope", "actor cannot change user roles")
}

func (s *identityService) authorizeUserStatusEndpoint(ctx context.Context, user *domain.User) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || user == nil {
		return nil
	}
	switch {
	case identityActorHasAnyRole(actor, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleAdmin):
		return nil
	case identityActorHasAnyRole(actor, domain.RoleDeptAdmin):
		if strings.EqualFold(identityActorDepartment(actor), string(user.Department)) {
			return nil
		}
		return fieldDenied("user_update_field_denied_by_scope", "DepartmentAdmin can only change status in own department")
	case identityActorHasAnyRole(actor, domain.RoleTeamLead):
		if strings.EqualFold(identityActorDepartment(actor), string(user.Department)) && strings.EqualFold(identityActorTeam(actor), user.Team) {
			return nil
		}
		return fieldDenied("user_update_field_denied_by_scope", "TeamLead can only change status in own team")
	default:
		return fieldDenied("user_update_field_denied_by_scope", "management access is required")
	}
}

func fieldDenied(code, message string) *domain.AppError {
	return domain.NewAppError(domain.ErrCodePermissionDenied, message, map[string]interface{}{"deny_code": code})
}

func roleDenied(code, message string) *domain.AppError {
	return domain.NewAppError(domain.ErrCodePermissionDenied, message, map[string]interface{}{"deny_code": code})
}

func identityActorCanManageAllUsers(actor domain.RequestActor) bool {
	return identityActorHasAnyRole(actor, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin)
}

func identityActorHasAnyRole(actor domain.RequestActor, roles ...domain.Role) bool {
	for _, role := range actor.Roles {
		for _, candidate := range roles {
			if role == candidate {
				return true
			}
		}
	}
	return false
}

func identityActorDepartment(actor domain.RequestActor) string {
	department := strings.TrimSpace(actor.Department)
	if department == "" {
		department = strings.TrimSpace(actor.FrontendAccess.Department)
	}
	return department
}

func identityActorTeam(actor domain.RequestActor) string {
	team := strings.TrimSpace(actor.Team)
	if team == "" {
		team = strings.TrimSpace(actor.FrontendAccess.Team)
	}
	if team == "" && len(actor.ManagedTeams) > 0 {
		team = strings.TrimSpace(actor.ManagedTeams[0])
	}
	if team == "" && len(actor.FrontendAccess.ManagedTeams) > 0 {
		team = strings.TrimSpace(actor.FrontendAccess.ManagedTeams[0])
	}
	return team
}

func departmentAdminCanAssignRole(department domain.Department, role domain.Role) bool {
	switch role {
	case domain.RoleMember, domain.RoleTeamLead:
		return true
	}
	for _, candidate := range domain.DepartmentDefaultBusinessRoles(department) {
		if candidate == role {
			return true
		}
	}
	return false
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
	if !containsAnyRole(user.Roles, domain.RoleAdmin, domain.RoleSuperAdmin) {
		return nil
	}
	activeAdminUsers, err := s.activeUsersWithRoles(ctx, domain.RoleAdmin, domain.RoleSuperAdmin)
	if err != nil {
		return infraError("list active admin users", err)
	}
	if len(activeAdminUsers) <= 1 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "at least one active admin user must remain", nil)
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

func (s *identityService) matchesDepartmentAdminKey(department domain.Department, providedKey string) bool {
	if providedKey == "" {
		return false
	}
	for _, candidate := range s.authSettings.DepartmentAdminKeys[string(department)] {
		if strings.TrimSpace(candidate) != "" && providedKey == strings.TrimSpace(candidate) {
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
	hash, err := bcrypt.GenerateFromPassword([]byte(entry.Password), bcrypt.DefaultCost)
	if err != nil {
		return infraError("hash configured super admin password", err)
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
			return s.userRepo.ReplaceRoles(ctx, tx, userID, roles)
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
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.userRepo.Update(ctx, tx, existing); err != nil {
			return err
		}
		if err := s.userRepo.UpdatePassword(ctx, tx, existing.ID, string(hash), now); err != nil {
			return err
		}
		return s.userRepo.ReplaceRoles(ctx, tx, existing.ID, roles)
	}); err != nil {
		return infraError("update configured super admin", err)
	}
	return nil
}

func (s *identityService) resolveConfiguredSuperAdminRoles(entry domain.ConfiguredSuperAdmin) ([]domain.Role, *domain.AppError) {
	if len(entry.Roles) == 0 {
		return []domain.Role{domain.RoleMember, domain.RoleAdmin, domain.RoleSuperAdmin}, nil
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

func diffRoles(before, after []domain.Role) ([]domain.Role, []domain.Role) {
	beforeSet := map[domain.Role]struct{}{}
	afterSet := map[domain.Role]struct{}{}
	for _, role := range domain.NormalizeRoleValues(before) {
		beforeSet[role] = struct{}{}
	}
	for _, role := range domain.NormalizeRoleValues(after) {
		afterSet[role] = struct{}{}
	}
	added := make([]domain.Role, 0)
	removed := make([]domain.Role, 0)
	for role := range afterSet {
		if _, ok := beforeSet[role]; !ok {
			added = append(added, role)
		}
	}
	for role := range beforeSet {
		if _, ok := afterSet[role]; !ok {
			removed = append(removed, role)
		}
	}
	return domain.NormalizeRoleValues(added), domain.NormalizeRoleValues(removed)
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
		Departments:     domain.DefaultDepartments(),
		DepartmentTeams: domain.DefaultOrgDepartmentTeams(),
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
		Version: "1.1.0",
		Defaults: domain.FrontendAccessDefaults{
			AllAuthenticated: domain.FrontendAccessSpec{
				Roles:   []string{"member"},
				Scopes:  []string{"frontend_ready", "self_only"},
				Menus:   []string{"dashboard"},
				Pages:   []string{"dashboard_home", "profile_me"},
				Actions: []string{"auth.me.read", "profile.view"},
			},
		},
		Identities: map[string]domain.FrontendAccessSpec{
			"super_admin": {
				Roles:   []string{"super_admin"},
				Scopes:  []string{"view_all", "identity_admin", "organization_admin", "all_departments"},
				Menus:   []string{"user_admin", "org_admin", "role_admin", "logs_center"},
				Pages:   []string{"admin_users", "admin_roles", "admin_permission_logs", "admin_operation_logs", "org_options"},
				Actions: []string{"user.manage", "role.assign", "role.remove", "permission_logs.read", "operation_logs.read", "organization.manage"},
			},
			"department_admin": {
				Roles:   []string{"department_admin"},
				Scopes:  []string{"department_scope"},
				Menus:   []string{"org_admin", "user_admin"},
				Pages:   []string{"department_users", "org_options"},
				Actions: []string{"department.manage", "department.users.read"},
			},
			"member": {
				Scopes: []string{"authenticated"},
			},
		},
		Roles: map[string]domain.FrontendAccessSpec{
			string(domain.RoleSuperAdmin):     {Roles: []string{"super_admin"}, Scopes: []string{"view_all", "identity_admin", "organization_admin"}, Menus: []string{"user_admin", "org_admin", "role_admin", "logs_center"}, Pages: []string{"admin_users", "admin_roles", "admin_permission_logs", "admin_operation_logs", "org_options"}, Actions: []string{"user.manage", "role.assign", "role.remove", "permission_logs.read", "operation_logs.read", "organization.manage"}},
			string(domain.RoleHRAdmin):        {Roles: []string{"hr_admin"}, Scopes: []string{"view_all", "hr_admin"}, Menus: []string{"user_admin", "org_admin", "logs_center"}, Pages: []string{"admin_users", "admin_permission_logs", "admin_operation_logs", "org_options"}, Actions: []string{"user.manage", "org.assign", "permission_logs.read", "operation_logs.read"}},
			string(domain.RoleOrgAdmin):       {Roles: []string{"org_admin"}, Scopes: []string{"org_admin"}, Menus: []string{"org_admin", "user_admin"}, Pages: []string{"admin_users", "org_options"}, Actions: []string{"org.manage", "user.org.assign"}},
			string(domain.RoleRoleAdmin):      {Roles: []string{"role_admin"}, Scopes: []string{"role_admin"}, Menus: []string{"role_admin", "user_admin"}, Pages: []string{"admin_users", "admin_roles"}, Actions: []string{"role.assign", "role.remove", "role.read"}},
			string(domain.RoleAdmin):          {Roles: []string{"admin"}, Scopes: []string{"workflow_admin", "identity_admin"}, Menus: []string{"user_admin", "org_admin", "role_admin", "logs_center"}, Pages: []string{"admin_users", "admin_roles", "admin_permission_logs", "admin_operation_logs", "org_options"}, Actions: []string{"user.manage", "role.assign", "role.remove", "permission_logs.read", "operation_logs.read", "task.full_access", "organization.manage"}},
			string(domain.RoleDeptAdmin):      {Roles: []string{"department_admin"}, Scopes: []string{"department_scope"}, Menus: []string{"org_admin", "user_admin"}, Pages: []string{"department_users", "org_options"}, Actions: []string{"department.manage", "department.users.read"}},
			string(domain.RoleTeamLead):       {Roles: []string{"team_lead"}, Scopes: []string{"team_scope"}, Pages: []string{"team_users"}, Actions: []string{"team.users.read"}},
			string(domain.RoleDesignDirector): {Roles: []string{"design_director"}, Scopes: []string{"design_department_scope"}, Menus: []string{"design_workspace", "user_admin"}, Pages: []string{"design_workspace", "department_users"}, Actions: []string{"design.review.read", "department.users.read"}},
			string(domain.RoleDesignReviewer): {Roles: []string{"design_reviewer"}, Scopes: []string{"design_review_scope"}, Menus: []string{"design_workspace"}, Pages: []string{"design_workspace", "audit_workspace"}, Actions: []string{"design.review", "task.audit.review"}},
			string(domain.RoleMember):         {Roles: []string{"member"}, Scopes: []string{"self_only"}, Actions: []string{"profile.view"}},
			string(domain.RoleOps):            {Roles: []string{"ops"}, Scopes: []string{"workflow_ops"}, Menus: []string{"task_create", "business_info", "task_board", "task_list", "warehouse_receive", "warehouse_processing", "export_center", "resource_management", "customization_management"}, Pages: []string{"task_board", "task_list", "task_create", "products", "categories", "cost_rules", "warehouse_receive", "warehouse_processing", "outsource_orders", "workbench", "export_jobs", "code_rules", "assets_index", "task_assets", "asset_detail", "customization_jobs", "customization_job_detail"}, Actions: []string{"task.create", "task.business_info", "task.list", "warehouse.prepare", "task.close"}},
			string(domain.RoleDesigner):       {Roles: []string{"designer"}, Scopes: []string{"design_workspace"}, Menus: []string{"design_workspace", "task_list", "export_center", "resource_management"}, Pages: []string{"design_workspace", "my_tasks", "design_submit", "design_rework", "export_jobs", "assets_index", "task_assets", "asset_detail"}, Actions: []string{"task.design_submit", "task.asset_upload", "task.list"}},
			string(domain.RoleCustomizationOperator): {
				Roles:   []string{"customization_operator"},
				Scopes:  []string{"customization_workspace"},
				Menus:   []string{"customization_management", "resource_management", "task_list"},
				Pages:   []string{"customization_jobs", "customization_job_detail", "task_assets", "asset_detail", "assets_index", "task_list"},
				Actions: []string{"task.customization.submit", "task.customization.transfer", "task.asset_upload", "task.list"},
			},
			string(domain.RoleAuditA):    {Roles: []string{"audit_a"}, Scopes: []string{"audit_workspace"}, Menus: []string{"audit_queue", "task_board", "task_list", "export_center"}, Pages: []string{"task_board", "task_list", "audit_workspace", "export_jobs"}, Actions: []string{"task.audit.claim", "task.audit.review", "task.list"}},
			string(domain.RoleAuditB):    {Roles: []string{"audit_b"}, Scopes: []string{"audit_workspace"}, Menus: []string{"audit_queue", "task_board", "task_list", "export_center"}, Pages: []string{"task_board", "task_list", "audit_workspace", "export_jobs"}, Actions: []string{"task.audit.claim", "task.audit.review", "task.audit.takeover", "task.list"}},
			string(domain.RoleWarehouse): {Roles: []string{"warehouse"}, Scopes: []string{"warehouse_workspace"}, Menus: []string{"warehouse_receive", "warehouse_processing", "task_board", "task_list", "export_center"}, Pages: []string{"warehouse_receive", "warehouse_processing", "task_list", "task_board", "export_jobs"}, Actions: []string{"warehouse.receive", "warehouse.reject", "warehouse.complete", "task.list"}},
			string(domain.RoleOutsource): {Roles: []string{"outsource"}, Scopes: []string{"outsource_workspace"}, Menus: []string{"task_list"}, Pages: []string{"outsource_orders", "task_list"}, Actions: []string{"outsource.manage", "task.list"}},
			string(domain.RoleCustomizationReviewer): {
				Roles:   []string{"customization_reviewer"},
				Scopes:  []string{"customization_review_scope"},
				Menus:   []string{"customization_management", "resource_management", "task_list"},
				Pages:   []string{"customization_jobs", "customization_job_detail", "task_assets", "asset_detail", "assets_index", "task_list"},
				Actions: []string{"task.customization.review", "task.customization.effect_review", "task.list"},
			},
			string(domain.RoleERP): {Roles: []string{"erp"}, Scopes: []string{"erp_internal"}, Menus: []string{"integration_center"}, Pages: []string{"erp_sync_console"}, Actions: []string{"erp.sync"}},
		},
		Departments: map[string]domain.DepartmentAccessEntry{
			string(domain.DepartmentHR):               {Code: "hr", FrontendAccessSpec: domain.FrontendAccessSpec{Scopes: []string{"department_hr"}}},
			string(domain.DepartmentDesignRD):         {Code: "design_rd", FrontendAccessSpec: domain.FrontendAccessSpec{Scopes: []string{"department_design_rd"}}},
			string(domain.DepartmentCustomizationArt): {Code: "customization_art", FrontendAccessSpec: domain.FrontendAccessSpec{Scopes: []string{"department_customization_art"}}},
			string(domain.DepartmentAudit):            {Code: "audit", FrontendAccessSpec: domain.FrontendAccessSpec{Scopes: []string{"department_audit"}}},
			string(domain.DepartmentOperations):       {Code: "operations", FrontendAccessSpec: domain.FrontendAccessSpec{Scopes: []string{"department_operations"}}},
			string(domain.DepartmentCloudWarehouse):   {Code: "cloud_warehouse", FrontendAccessSpec: domain.FrontendAccessSpec{Scopes: []string{"department_cloud_warehouse"}}},
			string(domain.DepartmentUnassigned):       {Code: "unassigned", FrontendAccessSpec: domain.FrontendAccessSpec{Scopes: []string{"department_unassigned"}}},
			string(domain.DepartmentDesign):           {Code: "design", FrontendAccessSpec: domain.FrontendAccessSpec{Scopes: []string{"department_design"}}},
			string(domain.DepartmentProcurement):      {Code: "procurement", FrontendAccessSpec: domain.FrontendAccessSpec{Scopes: []string{"department_procurement"}}},
			string(domain.DepartmentWarehouse):        {Code: "warehouse", FrontendAccessSpec: domain.FrontendAccessSpec{Scopes: []string{"department_warehouse"}}},
			string(domain.DepartmentBakeryWH):         {Code: "bakery_warehouse", FrontendAccessSpec: domain.FrontendAccessSpec{Scopes: []string{"department_bakery_warehouse"}}},
		},
	}
}
