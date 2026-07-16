package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

func TestUserAdminHandlerListUsersPassesDepartmentTeamAndRoleFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/v1/users?keyword=ops&department=杩愯惀閮?&team=杩愯惀涓€缁?&role=Ops&page=2&page_size=5", nil)
	c.Request = req

	svc := &userAdminServiceStub{
		currentUser: &domain.User{
			ID:    1,
			Roles: []domain.Role{domain.RoleAdmin},
			FrontendAccess: domain.FrontendAccessView{
				IsSuperAdmin: true,
			},
		},
		listUsersResp: []*domain.User{{ID: 10, Username: "ops_user"}},
	}
	h := NewUserAdminHandler(svc, nil, nil)

	h.ListUsers(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListUsers() status = %d, want 200", rec.Code)
	}
	if svc.lastListFilter.Keyword != "ops" {
		t.Fatalf("ListUsers() keyword = %q", svc.lastListFilter.Keyword)
	}
	if svc.lastListFilter.Department == nil || *svc.lastListFilter.Department != domain.Department("杩愯惀閮?") {
		t.Fatalf("ListUsers() department = %+v", svc.lastListFilter.Department)
	}
	if svc.lastListFilter.Team != "杩愯惀涓€缁?" {
		t.Fatalf("ListUsers() team = %q", svc.lastListFilter.Team)
	}
	if svc.lastListFilter.Role == nil || *svc.lastListFilter.Role != domain.RoleOps {
		t.Fatalf("ListUsers() role = %+v", svc.lastListFilter.Role)
	}
	if svc.lastListFilter.Page != 2 || svc.lastListFilter.PageSize != 5 {
		t.Fatalf("ListUsers() pagination filter = %+v", svc.lastListFilter)
	}
}

func TestUserAdminHandlerListAccessPolicyUsersUsesDedicatedMinimalSelector(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/access/users?q=alice&page=2&page_size=8", nil)
	departmentID, teamID := int64(3), int64(7)
	serviceStub := &userAdminServiceStub{
		listAccessPolicyUsersResp: []*domain.User{{ID: 12, Username: "alice", DisplayName: "Alice", Department: domain.DepartmentOperations, DepartmentID: &departmentID, Team: "T1", TeamID: &teamID, Mobile: "secret"}},
		listAccessPolicyUsersMeta: domain.PaginationMeta{Page: 2, PageSize: 8, Total: 1},
	}
	NewUserAdminHandler(serviceStub, nil, nil).ListAccessPolicyUsers(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if serviceStub.lastAccessPolicyFilter.Keyword != "alice" || serviceStub.lastAccessPolicyFilter.Page != 2 || serviceStub.lastAccessPolicyFilter.PageSize != 8 {
		t.Fatalf("filter=%+v", serviceStub.lastAccessPolicyFilter)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"\"id\":12", "\"department_id\":3", "\"team_id\":7"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("response missing %s: %s", expected, body)
		}
	}
	if strings.Contains(body, "secret") || strings.Contains(body, "mobile") {
		t.Fatalf("minimal selector leaked contact data: %s", body)
	}
}

func TestUserAdminHandlerCreateUserBindsManagedCreatePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := `{"account":"new_user","employee_no":1030,"name":"New User","department":"杩愯惀閮?","group":"杩愯惀涓€缁?","phone":"13800001030","password":"Init1234","roles":["Ops"],"status":"active"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	svc := &userAdminServiceStub{
		createUserResp: &domain.User{ID: 11, Username: "new_user"},
	}
	h := NewUserAdminHandler(svc, nil, nil)

	h.CreateUser(c)

	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateUser() status = %d, want 201", rec.Code)
	}
	if svc.lastCreateParams.Username != "new_user" || svc.lastCreateParams.DisplayName != "New User" {
		t.Fatalf("CreateUser() params = %+v", svc.lastCreateParams)
	}
	if svc.lastCreateParams.EmployeeNo == nil || *svc.lastCreateParams.EmployeeNo != 1030 {
		t.Fatalf("CreateUser() employee_no = %+v", svc.lastCreateParams.EmployeeNo)
	}
	if svc.lastCreateParams.Team != "杩愯惀涓€缁?" || svc.lastCreateParams.Mobile != "13800001030" {
		t.Fatalf("CreateUser() org/contact = %+v", svc.lastCreateParams)
	}
	if len(svc.lastCreateParams.Roles) != 1 || svc.lastCreateParams.Roles[0] != domain.RoleOps {
		t.Fatalf("CreateUser() roles = %+v", svc.lastCreateParams.Roles)
	}
	if svc.lastCreateParams.Status == nil || *svc.lastCreateParams.Status != domain.UserStatusActive {
		t.Fatalf("CreateUser() status = %+v", svc.lastCreateParams.Status)
	}
}

// TestListDesignersHandler_InvokesAssignableMethod proves the Round D
// handler rewire: the `/v1/users/designers` handler must route to the
// dedicated `ListAssignableDesigners` service method (which bypasses
// authorizeUserListFilter) and must NOT call the standard `ListUsers`
// management path. The handler must also pass the request actor through
// and emit the `{data, pagination}` envelope.
func TestListDesignersHandler_InvokesAssignableMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/v1/users/designers", nil)
	ctx := domain.WithRequestActor(req.Context(), domain.RequestActor{
		ID:         42,
		Username:   "ops_user",
		Roles:      []domain.Role{domain.RoleOps},
		Department: string(domain.DepartmentOperations),
		Source:     domain.RequestActorSourceSessionToken,
		AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
	})
	c.Request = req.WithContext(ctx)

	svc := &userAdminServiceStub{
		listAssignableDesignersResp: []*domain.User{
			{ID: 10, Username: "designer_a", DisplayName: "设计A"},
			{ID: 11, Username: "designer_b", DisplayName: "设计B"},
		},
	}
	h := NewUserAdminHandler(svc, nil, nil)

	h.ListDesigners(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListDesigners() status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if svc.listAssignableDesignersCalls != 1 {
		t.Fatalf("ListDesigners() ListAssignableDesigners calls = %d, want 1", svc.listAssignableDesignersCalls)
	}
	if svc.listUsersCalls != 0 {
		t.Fatalf("ListDesigners() must not call ListUsers, got %d calls", svc.listUsersCalls)
	}
	if svc.lastAssignableActor.ID != 42 || svc.lastAssignableActor.Username != "ops_user" {
		t.Fatalf("ListDesigners() actor propagated = %+v, want id=42 user=ops_user", svc.lastAssignableActor)
	}
	if svc.lastAssignableLane != service.AssignableLaneNormal {
		t.Fatalf("ListDesigners() lane = %q, want normal", svc.lastAssignableLane)
	}
	body := rec.Body.String()
	if !bytes.Contains([]byte(body), []byte(`"designer_a"`)) || !bytes.Contains([]byte(body), []byte(`"designer_b"`)) {
		t.Fatalf("ListDesigners() body missing designer payload: %s", body)
	}
	if !bytes.Contains([]byte(body), []byte(`"pagination"`)) || !bytes.Contains([]byte(body), []byte(`"total":2`)) {
		t.Fatalf("ListDesigners() body missing pagination envelope with total=2: %s", body)
	}
}

func TestListDesignersHandler_ParsesWorkflowLane(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		wantLane service.AssignableLane
		username string
	}{
		{
			name:     "customization",
			query:    "customization",
			wantLane: service.AssignableLaneCustomization,
			username: "custom_operator_a",
		},
		{
			name:     "audit",
			query:    "audit",
			wantLane: service.AssignableLaneAudit,
			username: "regular_auditor_a",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			req := httptest.NewRequest(http.MethodGet, "/v1/users/designers?workflow_lane="+tc.query, nil)
			ctx := domain.WithRequestActor(req.Context(), domain.RequestActor{
				ID:       42,
				Username: "ops_user",
				Roles:    []domain.Role{domain.RoleOps},
				Source:   domain.RequestActorSourceSessionToken,
				AuthMode: domain.AuthModeSessionTokenRoleEnforced,
			})
			c.Request = req.WithContext(ctx)

			svc := &userAdminServiceStub{
				listAssignableDesignersResp: []*domain.User{
					{ID: 20, Username: tc.username, DisplayName: "候选人"},
				},
			}
			h := NewUserAdminHandler(svc, nil, nil)

			h.ListDesigners(c)

			if rec.Code != http.StatusOK {
				t.Fatalf("ListDesigners(%s) status = %d, want 200 (body=%s)", tc.query, rec.Code, rec.Body.String())
			}
			if svc.lastAssignableLane != tc.wantLane {
				t.Fatalf("ListDesigners(%s) lane = %q, want %q", tc.query, svc.lastAssignableLane, tc.wantLane)
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte(tc.username)) {
				t.Fatalf("ListDesigners(%s) body missing payload: %s", tc.query, rec.Body.String())
			}
		})
	}
}

func TestListAssignableDesigners_InvalidLaneIsRejectedAtHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/v1/users/designers?workflow_lane=bogus", nil)
	ctx := domain.WithRequestActor(req.Context(), domain.RequestActor{
		ID:       42,
		Username: "ops_user",
		Roles:    []domain.Role{domain.RoleOps},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	c.Request = req.WithContext(ctx)

	svc := &userAdminServiceStub{}
	h := NewUserAdminHandler(svc, nil, nil)

	h.ListDesigners(c)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ListDesigners(bogus) status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if svc.listAssignableDesignersCalls != 0 {
		t.Fatalf("ListDesigners(bogus) touched service, calls=%d", svc.listAssignableDesignersCalls)
	}
	body := rec.Body.String()
	if !bytes.Contains([]byte(body), []byte(`"code":"INVALID_REQUEST"`)) ||
		!bytes.Contains([]byte(body), []byte(`"field":"workflow_lane"`)) ||
		!bytes.Contains([]byte(body), []byte(`"deny_code":"workflow_lane_unsupported"`)) {
		t.Fatalf("ListDesigners(bogus) error response missing workflow_lane details: %s", body)
	}
}

func TestUserAdminHandlerResetPasswordCallsService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := `{"password":"Reset1234"}`
	req := httptest.NewRequest(http.MethodPut, "/v1/users/15/password", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "15"}}
	c.Request = req

	svc := &userAdminServiceStub{
		resetPasswordResp: &domain.User{ID: 15, Username: "reset_user"},
	}
	h := NewUserAdminHandler(svc, nil, nil)

	h.ResetPassword(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("ResetPassword() status = %d, want 200", rec.Code)
	}
	if svc.lastResetParams.UserID != 15 || svc.lastResetParams.NewPassword != "Reset1234" {
		t.Fatalf("ResetPassword() params = %+v", svc.lastResetParams)
	}
}

func TestUserAdminHandlerGetOrgOptionsSetsDeprecationHeaderForCompatibilityShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/v1/org/options", nil)
	c.Request = req

	svc := &userAdminServiceStub{
		currentUser: &domain.User{
			ID:    1,
			Roles: []domain.Role{domain.RoleHRAdmin},
			FrontendAccess: domain.FrontendAccessView{
				IsSuperAdmin: true,
				ViewAll:      true,
				Roles:        []string{"hr_admin"},
			},
		},
		orgOptions: &domain.OrgOptions{
			Departments: []domain.DepartmentOption{
				{Name: "运营部", Teams: []string{"淘系一组"}, Enabled: true},
			},
			TeamsByDepartment: map[string][]string{
				"运营部": {"淘系一组"},
			},
		},
	}
	h := NewUserAdminHandler(svc, nil, nil)

	h.GetOrgOptions(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("GetOrgOptions() status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Deprecation"); got != `version="v1.8"` {
		t.Fatalf("GetOrgOptions() Deprecation header = %q", got)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte(`"departments"`)) {
		t.Fatalf("GetOrgOptions() body missing departments payload: %s", body)
	}
}

func TestUserAdminHandlerGetOrgOptionsIncludingDisabledRequiresOrgMasterWriteRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/v1/org/options?include_disabled=true", nil)
	c.Request = req

	svc := &userAdminServiceStub{
		currentUser: &domain.User{
			ID:    2,
			Roles: []domain.Role{domain.RoleDeptAdmin},
			FrontendAccess: domain.FrontendAccessView{
				IsDepartmentAdmin: true,
				Roles:             []string{"department_admin"},
			},
		},
	}
	h := NewUserAdminHandler(svc, nil, nil)

	h.GetOrgOptions(c)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("GetOrgOptions(include_disabled) status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if svc.getOrgOptionsIncludingDisabledCalls != 0 {
		t.Fatalf("GetOrgOptionsIncludingDisabled() calls = %d, want 0", svc.getOrgOptionsIncludingDisabledCalls)
	}
}

func TestUserAdminHandlerGetOrgOptionsIncludingDisabledCallsServiceForHRAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/v1/org/options?include_disabled=true", nil)
	c.Request = req

	svc := &userAdminServiceStub{
		currentUser: &domain.User{
			ID:    3,
			Roles: []domain.Role{domain.RoleHRAdmin},
			FrontendAccess: domain.FrontendAccessView{
				IsSuperAdmin: true,
				Roles:        []string{"hr_admin"},
			},
		},
		orgOptionsIncludingDisabled: &domain.OrgOptions{
			Departments: []domain.DepartmentOption{
				{
					ID:      7,
					Name:    "采购部",
					Enabled: false,
					TeamItems: []domain.OrgTeamOption{
						{ID: 11, Name: "采购组", Enabled: false},
					},
				},
			},
		},
	}
	h := NewUserAdminHandler(svc, nil, nil)

	h.GetOrgOptions(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("GetOrgOptions(include_disabled) status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if svc.getOrgOptionsIncludingDisabledCalls != 1 {
		t.Fatalf("GetOrgOptionsIncludingDisabled() calls = %d, want 1", svc.getOrgOptionsIncludingDisabledCalls)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte(`"name":"采购部"`)) {
		t.Fatalf("GetOrgOptions(include_disabled) body missing disabled department: %s", body)
	}
	if body := rec.Body.Bytes(); bytes.Count(body, []byte(`"enabled":false`)) < 2 {
		t.Fatalf("GetOrgOptions(include_disabled) body must explicitly emit disabled department/team enabled=false: %s", string(body))
	}
}

type userAdminServiceStub struct {
	currentUser                 *domain.User
	listUsersResp               []*domain.User
	listUsersMeta               domain.PaginationMeta
	listAssignableDesignersResp []*domain.User
	listAccessPolicyUsersResp   []*domain.User
	listAccessPolicyUsersMeta   domain.PaginationMeta
	orgOptions                  *domain.OrgOptions
	orgOptionsIncludingDisabled *domain.OrgOptions
	createUserResp              *domain.User
	resetPasswordResp           *domain.User
	createDepartment            *domain.OrgDepartment
	updateDepartment            *domain.OrgDepartment
	createTeam                  *domain.OrgTeam
	updateTeam                  *domain.OrgTeam

	lastListFilter                      service.UserFilter
	lastAccessPolicyFilter              service.UserFilter
	lastCreateParams                    service.CreateManagedUserParams
	lastResetParams                     service.ResetUserPasswordParams
	lastAssignableActor                 domain.RequestActor
	lastAssignableLane                  service.AssignableLane
	listUsersCalls                      int
	listAssignableDesignersCalls        int
	getOrgOptionsIncludingDisabledCalls int
}

func (s *userAdminServiceStub) SyncConfiguredAuth(context.Context) *domain.AppError {
	return nil
}

func (s *userAdminServiceStub) GetRegistrationOptions(context.Context) (*domain.RegistrationOptions, *domain.AppError) {
	return nil, nil
}

func (s *userAdminServiceStub) GetOrgOptions(context.Context) (*domain.OrgOptions, *domain.AppError) {
	return s.orgOptions, nil
}

func (s *userAdminServiceStub) GetOrgOptionsIncludingDisabled(context.Context) (*domain.OrgOptions, *domain.AppError) {
	s.getOrgOptionsIncludingDisabledCalls++
	return s.orgOptionsIncludingDisabled, nil
}

func (s *userAdminServiceStub) CreateDepartment(context.Context, service.CreateOrgDepartmentParams) (*domain.OrgDepartment, *domain.AppError) {
	return s.createDepartment, nil
}

func (s *userAdminServiceStub) UpdateDepartment(context.Context, service.UpdateOrgDepartmentParams) (*domain.OrgDepartment, *domain.AppError) {
	return s.updateDepartment, nil
}

func (s *userAdminServiceStub) CreateTeam(context.Context, service.CreateOrgTeamParams) (*domain.OrgTeam, *domain.AppError) {
	return s.createTeam, nil
}

func (s *userAdminServiceStub) UpdateTeam(context.Context, service.UpdateOrgTeamParams) (*domain.OrgTeam, *domain.AppError) {
	return s.updateTeam, nil
}

func (s *userAdminServiceStub) MergeDepartment(context.Context, service.MergeOrgDepartmentParams) (*domain.OrgDepartment, *domain.AppError) {
	return s.updateDepartment, nil
}

func (s *userAdminServiceStub) MergeTeam(context.Context, service.MergeOrgTeamParams) (*domain.OrgTeam, *domain.AppError) {
	return s.updateTeam, nil
}

func (s *userAdminServiceStub) DeleteDepartment(context.Context, int64) *domain.AppError {
	return nil
}

func (s *userAdminServiceStub) DeleteTeam(context.Context, int64) *domain.AppError {
	return nil
}

func (s *userAdminServiceStub) Register(context.Context, service.RegisterUserParams) (*domain.AuthResult, *domain.AppError) {
	return nil, nil
}

func (s *userAdminServiceStub) RegisterAssetWorkbenchUser(context.Context, service.RegisterAssetWorkbenchUserParams) (*domain.AuthResult, *domain.AppError) {
	return nil, nil
}

func (s *userAdminServiceStub) Login(context.Context, service.LoginParams) (*domain.AuthResult, *domain.AppError) {
	return nil, nil
}

func (s *userAdminServiceStub) ChangePassword(context.Context, service.ChangePasswordParams) *domain.AppError {
	return nil
}

func (s *userAdminServiceStub) CreateManagedUser(_ context.Context, p service.CreateManagedUserParams) (*domain.User, *domain.AppError) {
	s.lastCreateParams = p
	return s.createUserResp, nil
}

func (s *userAdminServiceStub) ResetUserPassword(_ context.Context, p service.ResetUserPasswordParams) (*domain.User, *domain.AppError) {
	s.lastResetParams = p
	return s.resetPasswordResp, nil
}

func (s *userAdminServiceStub) GetCurrentUser(context.Context) (*domain.User, *domain.AppError) {
	return s.currentUser, nil
}

func (s *userAdminServiceStub) GetMe(context.Context) (*domain.User, *domain.AppError) {
	return s.currentUser, nil
}

func (s *userAdminServiceStub) UpdateMe(context.Context, service.UpdateMeParams) (*domain.User, *domain.AppError) {
	return s.currentUser, nil
}

func (s *userAdminServiceStub) UpdateMyAvatar(context.Context, service.UpdateMyAvatarParams) (*domain.User, *domain.AppError) {
	return s.currentUser, nil
}

func (s *userAdminServiceStub) GetMyOrg(context.Context) (*domain.MyOrgProfile, *domain.AppError) {
	return nil, nil
}

func (s *userAdminServiceStub) ListUsers(_ context.Context, filter service.UserFilter) ([]*domain.User, domain.PaginationMeta, *domain.AppError) {
	s.listUsersCalls++
	s.lastListFilter = filter
	meta := s.listUsersMeta
	if meta.Page == 0 {
		meta = domain.PaginationMeta{Page: 2, PageSize: 5, Total: int64(len(s.listUsersResp))}
	}
	return s.listUsersResp, meta, nil
}

func (s *userAdminServiceStub) ListAccessPolicyUsers(_ context.Context, filter service.UserFilter) ([]*domain.User, domain.PaginationMeta, *domain.AppError) {
	s.lastAccessPolicyFilter = filter
	return s.listAccessPolicyUsersResp, s.listAccessPolicyUsersMeta, nil
}

func (s *userAdminServiceStub) ListAssignableDesigners(_ context.Context, actor *domain.RequestActor, lane service.AssignableLane) ([]*domain.User, *domain.AppError) {
	s.listAssignableDesignersCalls++
	if actor != nil {
		s.lastAssignableActor = *actor
	}
	s.lastAssignableLane = lane
	return s.listAssignableDesignersResp, nil
}

func (s *userAdminServiceStub) GetUser(context.Context, int64) (*domain.User, *domain.AppError) {
	return nil, nil
}

func (s *userAdminServiceStub) UpdateUser(context.Context, service.UpdateUserParams) (*domain.User, *domain.AppError) {
	return nil, nil
}

func (s *userAdminServiceStub) ActivateUser(context.Context, int64) *domain.AppError {
	return nil
}

func (s *userAdminServiceStub) DeactivateUser(context.Context, int64) *domain.AppError {
	return nil
}

func (s *userAdminServiceStub) DeleteUser(context.Context, service.DeleteUserParams) *domain.AppError {
	return nil
}

func (s *userAdminServiceStub) SetUserRoles(context.Context, service.SetUserRolesParams) (*domain.User, *domain.AppError) {
	return nil, nil
}

func (s *userAdminServiceStub) AddUserRoles(context.Context, service.AddUserRolesParams) (*domain.User, *domain.AppError) {
	return nil, nil
}

func (s *userAdminServiceStub) RemoveUserRole(context.Context, service.RemoveUserRoleParams) (*domain.User, *domain.AppError) {
	return nil, nil
}

func (s *userAdminServiceStub) ListPermissionLogs(context.Context, service.PermissionLogFilter) ([]*domain.PermissionLog, domain.PaginationMeta, *domain.AppError) {
	return nil, domain.PaginationMeta{}, nil
}

func (s *userAdminServiceStub) ListRoles(context.Context) []domain.RoleCatalogEntry {
	return nil
}

func (s *userAdminServiceStub) ResolveRequestActor(context.Context, string) (*domain.RequestActor, *domain.AppError) {
	return nil, nil
}

func (s *userAdminServiceStub) RecordRouteAccess(context.Context, domain.PermissionLog) {}
