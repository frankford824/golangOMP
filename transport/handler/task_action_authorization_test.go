package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/repo"
	"workflow/service"
)

func TestTaskActionRouteAuthorizationRegression(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("detail_read_allow_and_deny", func(t *testing.T) {
		taskRepo := &routeTaskRepo{
			tasks: map[int64]*domain.Task{
				1: {
					ID:              1,
					TaskNo:          "T-001",
					SKUCode:         "SKU-001",
					TaskType:        domain.TaskTypeNewProductDevelopment,
					SourceMode:      domain.TaskSourceModeNewProduct,
					TaskStatus:      domain.TaskStatusPendingAssign,
					OwnerDepartment: "ops",
					OwnerOrgTeam:    "ops-team-1",
					CreatorID:       300,
					CreatedAt:       time.Now().UTC(),
					UpdatedAt:       time.Now().UTC(),
				},
			},
			details: map[int64]*domain.TaskDetail{1: routeReadyDetail(1)},
		}
		taskSvc := service.NewTaskService(taskRepo, &routeTaskAssetRepo{}, &routeTaskEventRepo{}, nil, nil, routeTxRunner{})
		h := NewTaskHandler(taskSvc, nil, nil)

		router := gin.New()
		router.Use(routeActor(routeCapabilityActor(101, domain.PermissionTaskView)))
		router.GET("/v1/tasks/:id", h.GetByID)
		rec := performJSON(router, http.MethodGet, "/v1/tasks/1", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("allow GET /v1/tasks/1 code=%d body=%s", rec.Code, rec.Body.String())
		}

		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 102, Roles: []domain.Role{domain.RoleDeptAdmin}}))
		router.GET("/v1/tasks/:id", h.GetByID)
		rec = performJSON(router, http.MethodGet, "/v1/tasks/1", "")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("legacy role-only read code=%d body=%s, want 403", rec.Code, rec.Body.String())
		}
	})

	t.Run("assign_allow_and_deny", func(t *testing.T) {
		taskRepo := &routeTaskRepo{
			tasks: map[int64]*domain.Task{
				2: {
					ID:              2,
					TaskType:        domain.TaskTypeNewProductDevelopment,
					TaskStatus:      domain.TaskStatusPendingAssign,
					OwnerDepartment: "ops",
					OwnerOrgTeam:    "ops-team-1",
					CreatorID:       200,
				},
			},
		}
		h := NewTaskAssignmentHandler(service.NewTaskAssignmentService(taskRepo, &routeTaskEventRepo{}, routeTxRunner{}))

		router := gin.New()
		router.Use(routeActor(routeCapabilityActor(201, domain.PermissionTaskAssign)))
		router.POST("/v1/tasks/:id/assign", h.Assign)
		rec := performJSON(router, http.MethodPost, "/v1/tasks/2/assign", `{"designer_id":77,"assigned_by":201}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("allow assign code=%d body=%s", rec.Code, rec.Body.String())
		}

		taskRepo.tasks[2] = &domain.Task{
			ID:              2,
			TaskType:        domain.TaskTypeNewProductDevelopment,
			TaskStatus:      domain.TaskStatusPendingAssign,
			OwnerDepartment: "ops",
			OwnerOrgTeam:    "ops-team-1",
			CreatorID:       200,
		}
		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 202, Roles: []domain.Role{domain.RoleTeamLead}}))
		router.POST("/v1/tasks/:id/assign", h.Assign)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/2/assign", `{"designer_id":77,"assigned_by":202}`)
		assertTaskPermissionDenied(t, rec, "task_assignment_permission_or_scope_denied")

		currentDesignerID := int64(41)
		taskRepo.tasks[2] = &domain.Task{
			ID:               2,
			TaskType:         domain.TaskTypeNewProductDevelopment,
			TaskStatus:       domain.TaskStatusInProgress,
			OwnerDepartment:  "ops",
			OwnerOrgTeam:     "ops-team-1",
			CreatorID:        200,
			DesignerID:       &currentDesignerID,
			CurrentHandlerID: &currentDesignerID,
		}
		router = gin.New()
		router.Use(routeActor(routeCapabilityActor(203, domain.PermissionTaskReassign)))
		router.POST("/v1/tasks/:id/assign", h.Assign)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/2/assign", `{"designer_id":78,"assigned_by":203}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("allow reassign code=%d body=%s", rec.Code, rec.Body.String())
		}

		taskRepo.tasks[2] = &domain.Task{
			ID:               2,
			TaskType:         domain.TaskTypeNewProductDevelopment,
			TaskStatus:       domain.TaskStatusInProgress,
			OwnerDepartment:  "ops",
			OwnerOrgTeam:     "ops-team-1",
			CreatorID:        200,
			DesignerID:       &currentDesignerID,
			CurrentHandlerID: &currentDesignerID,
		}
		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 204, Roles: []domain.Role{domain.RoleTeamLead}}))
		router.POST("/v1/tasks/:id/assign", h.Assign)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/2/assign", `{"designer_id":78,"assigned_by":204}`)
		assertTaskPermissionDenied(t, rec, "task_assignment_permission_or_scope_denied")

		taskRepo.tasks[2] = &domain.Task{
			ID:               2,
			TaskType:         domain.TaskTypeNewProductDevelopment,
			TaskStatus:       domain.TaskStatusInProgress,
			OwnerDepartment:  "ops",
			OwnerOrgTeam:     "ops-team-1",
			CreatorID:        200,
			DesignerID:       &currentDesignerID,
			CurrentHandlerID: &currentDesignerID,
		}
		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 205, Roles: []domain.Role{domain.RoleOps}, Department: "ops"}))
		router.POST("/v1/tasks/:id/assign", h.Assign)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/2/assign", `{"designer_id":78,"assigned_by":205}`)
		assertTaskPermissionDenied(t, rec, "task_assignment_permission_or_scope_denied")

		taskRepo.tasks[2] = &domain.Task{
			ID:               2,
			TaskType:         domain.TaskTypeNewProductDevelopment,
			TaskStatus:       domain.TaskStatusPendingAudit,
			OwnerDepartment:  "ops",
			OwnerOrgTeam:     "ops-team-1",
			CreatorID:        200,
			DesignerID:       &currentDesignerID,
			CurrentHandlerID: &currentDesignerID,
		}
		router = gin.New()
		router.Use(routeActor(routeCapabilityActor(206, domain.PermissionTaskReassign)))
		router.POST("/v1/tasks/:id/assign", h.Assign)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/2/assign", `{"designer_id":78,"assigned_by":206}`)
		assertTaskPermissionDenied(t, rec, "task_status_not_actionable")
	})

}

func routeCapabilityActor(id int64, permission domain.PermissionCode) domain.RequestActor {
	roleID := int64(1)
	return domain.RequestActor{
		ID: id,
		EffectiveAccess: &domain.EffectiveAccess{
			UserID:      id,
			Permissions: []domain.PermissionCode{permission},
			Assignments: []domain.AccessAssignment{{RoleID: roleID, UserID: id, ScopeMode: domain.AccessScopeGlobal}},
			Sources:     []domain.EffectiveAccessNote{{RoleID: roleID, Permission: permission, ScopeMode: domain.AccessScopeGlobal}},
		},
	}
}

func routeActor(actor domain.RequestActor) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(domain.WithRequestActor(c.Request.Context(), actor))
		c.Next()
	}
}

func performJSON(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func assertTaskPermissionDenied(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code=%d want=403 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Error struct {
			Code    string                 `json:"code"`
			Details map[string]interface{} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rec.Body.String())
	}
	if resp.Error.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("error.code=%q want=%q body=%s", resp.Error.Code, domain.ErrCodePermissionDenied, rec.Body.String())
	}
	if got := resp.Error.Details["deny_code"]; got != want {
		t.Fatalf("details.deny_code=%v want=%s body=%s", got, want, rec.Body.String())
	}
}

func routeReadyDetail(taskID int64) *domain.TaskDetail {
	now := time.Now().UTC()
	cost := 12.5
	return &domain.TaskDetail{
		TaskID:       taskID,
		Category:     "Lightbox",
		CategoryCode: "LIGHTBOX",
		SpecText:     "spec",
		CostPrice:    &cost,
		FiledAt:      &now,
	}
}

type routeTaskRepo struct {
	tasks   map[int64]*domain.Task
	details map[int64]*domain.TaskDetail
}

func (r *routeTaskRepo) Create(context.Context, repo.Tx, *domain.Task, *domain.TaskDetail) (int64, error) {
	return 0, nil
}
func (r *routeTaskRepo) CreateSKUItems(context.Context, repo.Tx, []*domain.TaskSKUItem) error {
	return nil
}
func (r *routeTaskRepo) GetByID(_ context.Context, id int64) (*domain.Task, error) {
	return cloneRouteTask(r.tasks[id]), nil
}
func (r *routeTaskRepo) GetDetailByTaskID(_ context.Context, taskID int64) (*domain.TaskDetail, error) {
	return cloneRouteDetail(r.details[taskID]), nil
}
func (r *routeTaskRepo) GetSKUItemBySKUCode(context.Context, string) (*domain.TaskSKUItem, error) {
	return nil, nil
}
func (r *routeTaskRepo) ListSKUItemsByTaskID(context.Context, int64) ([]*domain.TaskSKUItem, error) {
	return []*domain.TaskSKUItem{}, nil
}
func (r *routeTaskRepo) List(context.Context, repo.TaskListFilter) ([]*domain.TaskListItem, int64, error) {
	return []*domain.TaskListItem{}, 0, nil
}
func (r *routeTaskRepo) UpdateDetailBusinessInfo(_ context.Context, _ repo.Tx, detail *domain.TaskDetail) error {
	r.details[detail.TaskID] = cloneRouteDetail(detail)
	return nil
}
func (r *routeTaskRepo) UpdatePriority(_ context.Context, _ repo.Tx, id int64, priority domain.TaskPriority) error {
	if task := r.tasks[id]; task != nil {
		task.Priority = priority
	}
	return nil
}
func (r *routeTaskRepo) UpdateProductBinding(_ context.Context, _ repo.Tx, task *domain.Task) error {
	r.tasks[task.ID] = cloneRouteTask(task)
	return nil
}
func (r *routeTaskRepo) UpdateStatus(_ context.Context, _ repo.Tx, id int64, status domain.TaskStatus) error {
	if task := r.tasks[id]; task != nil {
		task.TaskStatus = status
	}
	return nil
}
func (r *routeTaskRepo) UpdateDesigner(_ context.Context, _ repo.Tx, id int64, designerID *int64) error {
	if task := r.tasks[id]; task != nil {
		task.DesignerID = cloneRouteInt64(designerID)
	}
	return nil
}
func (r *routeTaskRepo) UpdateHandler(_ context.Context, _ repo.Tx, id int64, handlerID *int64) error {
	if task := r.tasks[id]; task != nil {
		task.CurrentHandlerID = cloneRouteInt64(handlerID)
	}
	return nil
}

func (r *routeTaskRepo) UpdateCustomizationState(_ context.Context, _ repo.Tx, id int64, lastOperatorID *int64, rejectReason, rejectCategory string) error {
	if task := r.tasks[id]; task != nil {
		task.LastCustomizationOperatorID = cloneRouteInt64(lastOperatorID)
		task.WarehouseRejectReason = rejectReason
		task.WarehouseRejectCategory = rejectCategory
	}
	return nil
}

type routeTaskAssetRepo struct {
	byID   map[int64]*domain.TaskAsset
	byTask map[int64][]*domain.TaskAsset
	nextID int64
}

func (r *routeTaskAssetRepo) Create(_ context.Context, _ repo.Tx, asset *domain.TaskAsset) (int64, error) {
	if r.byID == nil {
		r.byID = map[int64]*domain.TaskAsset{}
	}
	if r.byTask == nil {
		r.byTask = map[int64][]*domain.TaskAsset{}
	}
	r.nextID++
	asset.ID = r.nextID
	r.byID[asset.ID] = cloneRouteAsset(asset)
	r.byTask[asset.TaskID] = append(r.byTask[asset.TaskID], cloneRouteAsset(asset))
	return asset.ID, nil
}
func (r *routeTaskAssetRepo) GetByID(_ context.Context, id int64) (*domain.TaskAsset, error) {
	return cloneRouteAsset(r.byID[id]), nil
}
func (r *routeTaskAssetRepo) ListByTaskID(_ context.Context, taskID int64) ([]*domain.TaskAsset, error) {
	items := r.byTask[taskID]
	out := make([]*domain.TaskAsset, 0, len(items))
	for _, item := range items {
		out = append(out, cloneRouteAsset(item))
	}
	return out, nil
}
func (r *routeTaskAssetRepo) ListByAssetID(context.Context, int64) ([]*domain.TaskAsset, error) {
	return []*domain.TaskAsset{}, nil
}
func (r *routeTaskAssetRepo) NextVersionNo(_ context.Context, _ repo.Tx, taskID int64) (int, error) {
	return len(r.byTask[taskID]) + 1, nil
}
func (r *routeTaskAssetRepo) NextAssetVersionNo(context.Context, repo.Tx, int64) (int, error) {
	return 1, nil
}

type routeTaskEventRepo struct{}

func (r *routeTaskEventRepo) Append(_ context.Context, _ repo.Tx, taskID int64, eventType string, operatorID *int64, payload interface{}) (*domain.TaskEvent, error) {
	raw, _ := json.Marshal(payload)
	return &domain.TaskEvent{ID: eventType, TaskID: taskID, EventType: eventType, OperatorID: operatorID, Payload: raw, CreatedAt: time.Now().UTC()}, nil
}
func (r *routeTaskEventRepo) ListByTaskID(context.Context, int64) ([]*domain.TaskEvent, error) {
	return []*domain.TaskEvent{}, nil
}
func (r *routeTaskEventRepo) ListRecent(context.Context, repo.TaskEventListFilter) ([]*domain.TaskEvent, int64, error) {
	return []*domain.TaskEvent{}, 0, nil
}

type routeAuditRepo struct{}

func (r *routeAuditRepo) CreateRecord(context.Context, repo.Tx, *domain.AuditRecord) (int64, error) {
	return 1, nil
}
func (r *routeAuditRepo) ListRecordsByTaskID(context.Context, int64) ([]*domain.AuditRecord, error) {
	return []*domain.AuditRecord{}, nil
}
func (r *routeAuditRepo) CreateHandover(context.Context, repo.Tx, *domain.AuditHandover) (int64, error) {
	return 1, nil
}
func (r *routeAuditRepo) GetHandoverByID(context.Context, int64) (*domain.AuditHandover, error) {
	return nil, nil
}
func (r *routeAuditRepo) ListHandoversByTaskID(context.Context, int64) ([]*domain.AuditHandover, error) {
	return []*domain.AuditHandover{}, nil
}
func (r *routeAuditRepo) UpdateHandoverStatus(context.Context, repo.Tx, int64, domain.HandoverStatus) error {
	return nil
}

type routeUploadRequestRepo struct{}

func (r *routeUploadRequestRepo) Create(context.Context, repo.Tx, *domain.UploadRequest) (*domain.UploadRequest, error) {
	return nil, nil
}
func (r *routeUploadRequestRepo) GetByRequestID(context.Context, string) (*domain.UploadRequest, error) {
	return nil, nil
}
func (r *routeUploadRequestRepo) List(context.Context, repo.UploadRequestListFilter) ([]*domain.UploadRequest, int64, error) {
	return []*domain.UploadRequest{}, 0, nil
}
func (r *routeUploadRequestRepo) UpdateLifecycle(context.Context, repo.Tx, repo.UploadRequestLifecycleUpdate) error {
	return nil
}
func (r *routeUploadRequestRepo) UpdateBinding(context.Context, repo.Tx, string, *int64, string, domain.UploadRequestStatus, string) error {
	return nil
}
func (r *routeUploadRequestRepo) UpdateSession(context.Context, repo.Tx, repo.UploadRequestSessionUpdate) error {
	return nil
}

type routeAssetStorageRefRepo struct{}

func (r *routeAssetStorageRefRepo) Create(_ context.Context, _ repo.Tx, ref *domain.AssetStorageRef) (*domain.AssetStorageRef, error) {
	return ref, nil
}
func (r *routeAssetStorageRefRepo) GetByRefID(_ context.Context, refID string) (*domain.AssetStorageRef, error) {
	return &domain.AssetStorageRef{RefID: refID}, nil
}
func (r *routeAssetStorageRefRepo) UpdateStatus(context.Context, repo.Tx, string, domain.AssetStorageRefStatus) error {
	return nil
}

type routeUserRepo struct {
	users map[int64]*domain.User
}

func (r *routeUserRepo) Count(context.Context) (int64, error)                         { return 0, nil }
func (r *routeUserRepo) CountByRole(context.Context, domain.Role) (int64, error)      { return 0, nil }
func (r *routeUserRepo) CountByDepartment(context.Context, string) (int64, error)     { return 0, nil }
func (r *routeUserRepo) CountByTeam(context.Context, string) (int64, error)           { return 0, nil }
func (r *routeUserRepo) Create(context.Context, repo.Tx, *domain.User) (int64, error) { return 0, nil }
func (r *routeUserRepo) GetByID(_ context.Context, id int64) (*domain.User, error) {
	return r.users[id], nil
}
func (r *routeUserRepo) GetByUsername(context.Context, string) (*domain.User, error) { return nil, nil }
func (r *routeUserRepo) GetByMobile(context.Context, string) (*domain.User, error)   { return nil, nil }
func (r *routeUserRepo) GetByEmployeeNo(context.Context, int) (*domain.User, error)  { return nil, nil }
func (r *routeUserRepo) GetByJstUID(context.Context, int64) (*domain.User, error)    { return nil, nil }
func (r *routeUserRepo) List(context.Context, repo.UserListFilter) ([]*domain.User, int64, error) {
	return []*domain.User{}, 0, nil
}
func (r *routeUserRepo) ListConfigManagedAdmins(context.Context) ([]*domain.User, error) {
	return []*domain.User{}, nil
}
func (r *routeUserRepo) Update(context.Context, repo.Tx, *domain.User) error { return nil }
func (r *routeUserRepo) UpdateJstFields(context.Context, repo.Tx, int64, string, string, string, string, []string, []string, string, *int64, *time.Time) error {
	return nil
}
func (r *routeUserRepo) UpdatePassword(context.Context, repo.Tx, int64, string, time.Time) error {
	return nil
}
func (r *routeUserRepo) UpdateLastLogin(context.Context, repo.Tx, int64, time.Time) error { return nil }
func (r *routeUserRepo) ReplaceRoles(context.Context, repo.Tx, int64, []domain.Role) error {
	return nil
}
func (r *routeUserRepo) ListRoles(context.Context, int64) ([]domain.Role, error) { return nil, nil }

type routeTx struct{}

func (routeTx) IsTx() {}

type routeTxRunner struct{}

func (routeTxRunner) RunInTx(_ context.Context, fn func(repo.Tx) error) error {
	return fn(routeTx{})
}

func cloneRouteInt64(v *int64) *int64 {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func cloneRouteTask(task *domain.Task) *domain.Task {
	if task == nil {
		return nil
	}
	out := *task
	out.DesignerID = cloneRouteInt64(task.DesignerID)
	out.CurrentHandlerID = cloneRouteInt64(task.CurrentHandlerID)
	return &out
}

func cloneRouteDetail(detail *domain.TaskDetail) *domain.TaskDetail {
	if detail == nil {
		return nil
	}
	out := *detail
	return &out
}

func cloneRouteAsset(asset *domain.TaskAsset) *domain.TaskAsset {
	if asset == nil {
		return nil
	}
	out := *asset
	return &out
}
