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
		taskSvc := service.NewTaskService(taskRepo, &routeProcurementRepo{}, &routeTaskAssetRepo{}, &routeTaskEventRepo{}, nil, &routeWarehouseRepo{}, nil, routeTxRunner{},
			service.WithTaskDataScopeResolver(service.NewRoleBasedDataScopeResolver()),
			service.WithTaskScopeUserRepo(&routeUserRepo{
				users: map[int64]*domain.User{
					101: {ID: 101, Department: domain.Department("ops"), Roles: []domain.Role{domain.RoleDeptAdmin}},
					102: {ID: 102, Department: domain.Department("design"), Roles: []domain.Role{domain.RoleDeptAdmin}},
				},
			}))
		h := NewTaskHandler(taskSvc, nil, nil)

		router := gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 101, Roles: []domain.Role{domain.RoleDeptAdmin}, Department: "ops"}))
		router.GET("/v1/tasks/:id", h.GetByID)
		rec := performJSON(router, http.MethodGet, "/v1/tasks/1", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("allow GET /v1/tasks/1 code=%d body=%s", rec.Code, rec.Body.String())
		}

		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 102, Roles: []domain.Role{domain.RoleDeptAdmin}, Department: "design"}))
		router.GET("/v1/tasks/:id", h.GetByID)
		rec = performJSON(router, http.MethodGet, "/v1/tasks/1", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("main flow read should allow cross-department GET /v1/tasks/1 code=%d body=%s", rec.Code, rec.Body.String())
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
		router.Use(routeActor(domain.RequestActor{ID: 201, Roles: []domain.Role{domain.RoleTeamLead}, Team: "ops-team-1"}))
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
		router.Use(routeActor(domain.RequestActor{ID: 202, Roles: []domain.Role{domain.RoleTeamLead}, Team: "ops-team-9"}))
		router.POST("/v1/tasks/:id/assign", h.Assign)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/2/assign", `{"designer_id":77,"assigned_by":202}`)
		assertTaskPermissionDenied(t, rec, "task_out_of_team_scope")

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
		router.Use(routeActor(domain.RequestActor{ID: 203, Roles: []domain.Role{domain.RoleTeamLead}, Team: "ops-team-1"}))
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
		router.Use(routeActor(domain.RequestActor{ID: 204, Roles: []domain.Role{domain.RoleTeamLead}, Team: "ops-team-9"}))
		router.POST("/v1/tasks/:id/assign", h.Assign)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/2/assign", `{"designer_id":78,"assigned_by":204}`)
		assertTaskPermissionDenied(t, rec, "task_out_of_team_scope")

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
		assertTaskPermissionDenied(t, rec, "task_reassign_requires_requester_or_manager")

		taskRepo.tasks[2] = &domain.Task{
			ID:               2,
			TaskType:         domain.TaskTypeNewProductDevelopment,
			TaskStatus:       domain.TaskStatusPendingAuditA,
			OwnerDepartment:  "ops",
			OwnerOrgTeam:     "ops-team-1",
			CreatorID:        200,
			DesignerID:       &currentDesignerID,
			CurrentHandlerID: &currentDesignerID,
		}
		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 206, Roles: []domain.Role{domain.RoleTeamLead}, Team: "ops-team-1"}))
		router.POST("/v1/tasks/:id/assign", h.Assign)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/2/assign", `{"designer_id":78,"assigned_by":206}`)
		assertTaskPermissionDenied(t, rec, "task_not_reassignable")
	})

	t.Run("submit_design_allow_and_deny", func(t *testing.T) {
		designerID := int64(301)
		taskRepo := &routeTaskRepo{
			tasks: map[int64]*domain.Task{
				3: {
					ID:               3,
					TaskType:         domain.TaskTypeNewProductDevelopment,
					TaskStatus:       domain.TaskStatusInProgress,
					OwnerDepartment:  "ops",
					OwnerOrgTeam:     "ops-team-1",
					DesignerID:       &designerID,
					CurrentHandlerID: &designerID,
				},
			},
		}
		taskAssetRepo := &routeTaskAssetRepo{}
		h := NewDesignSubmissionHandler(service.NewTaskAssetService(taskRepo, taskAssetRepo, &routeTaskEventRepo{}, &routeUploadRequestRepo{}, &routeAssetStorageRefRepo{}, routeTxRunner{}), nil, nil)

		router := gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 301, Roles: []domain.Role{domain.RoleDesigner}}))
		router.POST("/v1/tasks/:id/submit-design", h.Submit)
		rec := performJSON(router, http.MethodPost, "/v1/tasks/3/submit-design", `{"asset_type":"delivery","file_name":"proof.jpg","uploaded_by":301}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("allow submit-design code=%d body=%s", rec.Code, rec.Body.String())
		}

		otherHandler := int64(999)
		taskRepo.tasks[3] = &domain.Task{
			ID:               3,
			TaskType:         domain.TaskTypeNewProductDevelopment,
			TaskStatus:       domain.TaskStatusInProgress,
			OwnerDepartment:  "ops",
			OwnerOrgTeam:     "ops-team-1",
			DesignerID:       &designerID,
			CurrentHandlerID: &otherHandler,
		}
		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 302, Roles: []domain.Role{domain.RoleDesigner}}))
		router.POST("/v1/tasks/:id/submit-design", h.Submit)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/3/submit-design", `{"asset_type":"delivery","file_name":"proof.jpg","uploaded_by":302}`)
		assertTaskPermissionDenied(t, rec, "task_not_assigned_to_actor")
	})

	t.Run("audit_approve_allow_and_deny", func(t *testing.T) {
		currentHandler := int64(900)
		taskRepo := &routeTaskRepo{
			tasks: map[int64]*domain.Task{
				4: {
					ID:               4,
					TaskType:         domain.TaskTypeNewProductDevelopment,
					TaskStatus:       domain.TaskStatusPendingAuditA,
					OwnerDepartment:  "ops",
					OwnerOrgTeam:     "ops-team-1",
					CurrentHandlerID: &currentHandler,
				},
			},
		}
		h := NewAuditV7Handler(service.NewAuditV7Service(taskRepo, &routeAuditRepo{}, &routeTaskEventRepo{}, nil, routeTxRunner{}), nil)

		router := gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 401, Roles: []domain.Role{domain.RoleDeptAdmin}, Department: "ops"}))
		router.POST("/v1/tasks/:id/audit/approve", h.Approve)
		rec := performJSON(router, http.MethodPost, "/v1/tasks/4/audit/approve", `{"stage":"A","next_status":"PendingAuditB","auditor_id":401}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("allow audit approve code=%d body=%s", rec.Code, rec.Body.String())
		}

		taskRepo.tasks[4] = &domain.Task{
			ID:               4,
			TaskType:         domain.TaskTypeNewProductDevelopment,
			TaskStatus:       domain.TaskStatusPendingAuditA,
			OwnerDepartment:  "ops",
			OwnerOrgTeam:     "ops-team-1",
			CurrentHandlerID: &currentHandler,
		}
		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 402, Roles: []domain.Role{domain.RoleDeptAdmin}, Department: "design"}))
		router.POST("/v1/tasks/:id/audit/approve", h.Approve)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/4/audit/approve", `{"stage":"A","next_status":"PendingAuditB","auditor_id":402}`)
		assertTaskPermissionDenied(t, rec, "task_out_of_stage_scope")
	})

	t.Run("audit_stage_and_role_are_stage_specific", func(t *testing.T) {
		handlerID := int64(410)
		taskRepo := &routeTaskRepo{
			tasks: map[int64]*domain.Task{
				41: {
					ID:               41,
					TaskType:         domain.TaskTypeNewProductDevelopment,
					TaskStatus:       domain.TaskStatusPendingAuditA,
					OwnerDepartment:  "ops",
					OwnerOrgTeam:     "ops-team-1",
					CurrentHandlerID: &handlerID,
				},
				42: {
					ID:               42,
					TaskType:         domain.TaskTypeNewProductDevelopment,
					TaskStatus:       domain.TaskStatusPendingAuditB,
					OwnerDepartment:  "ops",
					OwnerOrgTeam:     "ops-team-1",
					CurrentHandlerID: &handlerID,
				},
			},
		}
		h := NewAuditV7Handler(service.NewAuditV7Service(taskRepo, &routeAuditRepo{}, &routeTaskEventRepo{}, nil, routeTxRunner{}), nil)

		router := gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 410, Roles: []domain.Role{domain.RoleAuditB}}))
		router.POST("/v1/tasks/:id/audit/approve", h.Approve)
		rec := performJSON(router, http.MethodPost, "/v1/tasks/41/audit/approve", `{"stage":"A","next_status":"PendingAuditB","auditor_id":410}`)
		assertTaskPermissionDenied(t, rec, "missing_required_role")

		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 410, Roles: []domain.Role{domain.RoleAuditA}}))
		router.POST("/v1/tasks/:id/audit/approve", h.Approve)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/41/audit/approve", `{"stage":"B","next_status":"PendingAuditB","auditor_id":410}`)
		assertTaskPermissionDenied(t, rec, "audit_stage_mismatch")

		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 410, Roles: []domain.Role{domain.RoleAuditB}}))
		router.POST("/v1/tasks/:id/audit/reject", h.Reject)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/42/audit/reject", `{"stage":"B","comment":"need rework","auditor_id":410}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("allow audit B reject code=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("warehouse_complete_allow_and_deny", func(t *testing.T) {
		receiverID := int64(501)
		taskRepo := &routeTaskRepo{
			tasks: map[int64]*domain.Task{
				5: {
					ID:               5,
					TaskNo:           "T-005",
					SKUCode:          "SKU-005",
					TaskType:         domain.TaskTypeNewProductDevelopment,
					TaskStatus:       domain.TaskStatusPendingWarehouseReceive,
					OwnerDepartment:  "ops",
					OwnerOrgTeam:     "ops-team-1",
					CurrentHandlerID: &receiverID,
				},
			},
		}
		taskAssetRepo := &routeTaskAssetRepo{}
		warehouseRepo := &routeWarehouseRepo{receipts: map[int64]*domain.WarehouseReceipt{
			5: {TaskID: 5, Status: domain.WarehouseReceiptStatusReceived, ReceiverID: &receiverID},
		}}
		h := NewWarehouseHandler(service.NewWarehouseService(taskRepo, taskAssetRepo, warehouseRepo, &routeTaskEventRepo{}, routeTxRunner{}))

		router := gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 501, Roles: []domain.Role{domain.RoleWarehouse}}))
		router.POST("/v1/tasks/:id/warehouse/complete", h.Complete)
		rec := performJSON(router, http.MethodPost, "/v1/tasks/5/warehouse/complete", `{"receiver_id":501}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("allow warehouse complete code=%d body=%s", rec.Code, rec.Body.String())
		}

		taskRepo.tasks[5] = &domain.Task{
			ID:               5,
			TaskNo:           "T-005",
			SKUCode:          "SKU-005",
			TaskType:         domain.TaskTypeNewProductDevelopment,
			TaskStatus:       domain.TaskStatusPendingWarehouseReceive,
			OwnerDepartment:  "ops",
			OwnerOrgTeam:     "ops-team-1",
			CurrentHandlerID: &receiverID,
		}
		warehouseRepo.receipts[5] = &domain.WarehouseReceipt{TaskID: 5, Status: domain.WarehouseReceiptStatusReceived, ReceiverID: &receiverID}
		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 502, Roles: []domain.Role{domain.RoleWarehouse}}))
		router.POST("/v1/tasks/:id/warehouse/complete", h.Complete)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/5/warehouse/complete", `{"receiver_id":502}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("allow warehouse complete via stage scope code=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("warehouse_receive_status_scope_and_role", func(t *testing.T) {
		otherHandler := int64(888)
		taskRepo := &routeTaskRepo{
			tasks: map[int64]*domain.Task{
				51: {
					ID:              51,
					TaskType:        domain.TaskTypeNewProductDevelopment,
					TaskStatus:      domain.TaskStatusPendingWarehouseReceive,
					OwnerDepartment: "ops",
					OwnerOrgTeam:    "ops-team-1",
				},
				52: {
					ID:               52,
					TaskType:         domain.TaskTypeNewProductDevelopment,
					TaskStatus:       domain.TaskStatusPendingWarehouseReceive,
					OwnerDepartment:  "ops",
					OwnerOrgTeam:     "ops-team-1",
					CurrentHandlerID: &otherHandler,
				},
				53: {
					ID:              53,
					TaskType:        domain.TaskTypeNewProductDevelopment,
					TaskStatus:      domain.TaskStatusPendingClose,
					OwnerDepartment: "ops",
					OwnerOrgTeam:    "ops-team-1",
				},
			},
		}
		h := NewWarehouseHandler(service.NewWarehouseService(taskRepo, &routeTaskAssetRepo{}, &routeWarehouseRepo{}, &routeTaskEventRepo{}, routeTxRunner{}))

		router := gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 511, Roles: []domain.Role{domain.RoleWarehouse}, Team: "ops-team-1"}))
		router.POST("/v1/tasks/:id/warehouse/receive", h.Receive)
		rec := performJSON(router, http.MethodPost, "/v1/tasks/51/warehouse/receive", `{"receiver_id":511}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("allow warehouse receive code=%d body=%s", rec.Code, rec.Body.String())
		}

		taskRepo.tasks[51] = &domain.Task{
			ID:              51,
			TaskType:        domain.TaskTypeNewProductDevelopment,
			TaskStatus:      domain.TaskStatusPendingWarehouseReceive,
			OwnerDepartment: "ops",
			OwnerOrgTeam:    "ops-team-1",
		}
		h = NewWarehouseHandler(service.NewWarehouseService(taskRepo, &routeTaskAssetRepo{}, &routeWarehouseRepo{}, &routeTaskEventRepo{}, routeTxRunner{}))
		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 512, Roles: []domain.Role{domain.RoleWarehouse}, Team: "ops-team-9"}))
		router.POST("/v1/tasks/:id/warehouse/receive", h.Receive)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/51/warehouse/receive", `{"receiver_id":512}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("allow warehouse receive via stage scope code=%d body=%s", rec.Code, rec.Body.String())
		}

		h = NewWarehouseHandler(service.NewWarehouseService(taskRepo, &routeTaskAssetRepo{}, &routeWarehouseRepo{}, &routeTaskEventRepo{}, routeTxRunner{}))
		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 513, Roles: []domain.Role{domain.RoleWarehouse}, Team: "ops-team-1"}))
		router.POST("/v1/tasks/:id/warehouse/receive", h.Receive)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/52/warehouse/receive", `{"receiver_id":513}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("allow assigned warehouse receive via stage scope code=%d body=%s", rec.Code, rec.Body.String())
		}

		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 514, Roles: []domain.Role{domain.RoleWarehouse}, Team: "ops-team-1"}))
		router.POST("/v1/tasks/:id/warehouse/receive", h.Receive)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/53/warehouse/receive", `{"receiver_id":514}`)
		assertTaskPermissionDenied(t, rec, "warehouse_stage_mismatch")

		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 515, Roles: []domain.Role{domain.RoleDesigner}, Team: "ops-team-1"}))
		router.POST("/v1/tasks/:id/warehouse/receive", h.Receive)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/51/warehouse/receive", `{"receiver_id":515}`)
		assertTaskPermissionDenied(t, rec, "missing_required_role")
	})

	t.Run("close_allow_and_deny", func(t *testing.T) {
		taskRepo := &routeTaskRepo{
			tasks: map[int64]*domain.Task{
				6: {
					ID:              6,
					TaskNo:          "T-006",
					SKUCode:         "SKU-006",
					TaskType:        domain.TaskTypeNewProductDevelopment,
					SourceMode:      domain.TaskSourceModeNewProduct,
					TaskStatus:      domain.TaskStatusPendingClose,
					OwnerDepartment: "ops",
					OwnerOrgTeam:    "ops-team-1",
					CreatorID:       600,
					CreatedAt:       time.Now().UTC(),
					UpdatedAt:       time.Now().UTC(),
				},
			},
			details: map[int64]*domain.TaskDetail{6: routeReadyDetail(6)},
		}
		taskAssetRepo := &routeTaskAssetRepo{
			byTask: map[int64][]*domain.TaskAsset{
				6: {{ID: 61, TaskID: 6, AssetType: domain.TaskAssetTypeDelivery}},
			},
		}
		warehouseRepo := &routeWarehouseRepo{receipts: map[int64]*domain.WarehouseReceipt{
			6: {TaskID: 6, Status: domain.WarehouseReceiptStatusCompleted},
		}}
		taskSvc := service.NewTaskService(taskRepo, &routeProcurementRepo{}, taskAssetRepo, &routeTaskEventRepo{}, nil, warehouseRepo, nil, routeTxRunner{},
			service.WithTaskDataScopeResolver(service.NewRoleBasedDataScopeResolver()),
			service.WithTaskScopeUserRepo(&routeUserRepo{
				users: map[int64]*domain.User{
					601: {ID: 601, Department: domain.Department("ops"), Roles: []domain.Role{domain.RoleDeptAdmin}},
					602: {ID: 602, Department: domain.Department("design"), Roles: []domain.Role{domain.RoleDeptAdmin}},
					603: {ID: 603, Department: domain.DepartmentCloudWarehouse, Roles: []domain.Role{domain.RoleDeptAdmin, domain.RoleWarehouse}},
				},
			}))
		h := NewTaskHandler(taskSvc, nil, nil)

		router := gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 601, Roles: []domain.Role{domain.RoleDeptAdmin}, Department: "ops"}))
		router.POST("/v1/tasks/:id/close", h.Close)
		rec := performJSON(router, http.MethodPost, "/v1/tasks/6/close", `{"operator_id":601}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("allow close code=%d body=%s", rec.Code, rec.Body.String())
		}

		taskRepo.tasks[6] = &domain.Task{
			ID:              6,
			TaskNo:          "T-006",
			SKUCode:         "SKU-006",
			TaskType:        domain.TaskTypeNewProductDevelopment,
			SourceMode:      domain.TaskSourceModeNewProduct,
			TaskStatus:      domain.TaskStatusPendingClose,
			OwnerDepartment: "ops",
			OwnerOrgTeam:    "ops-team-1",
			CreatorID:       600,
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		}
		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 602, Roles: []domain.Role{domain.RoleDeptAdmin}, Department: "design"}))
		router.POST("/v1/tasks/:id/close", h.Close)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/6/close", `{"operator_id":602}`)
		assertTaskPermissionDenied(t, rec, "task_out_of_stage_scope")

		taskRepo.tasks[6] = &domain.Task{
			ID:              6,
			TaskNo:          "T-006",
			SKUCode:         "SKU-006",
			TaskType:        domain.TaskTypeNewProductDevelopment,
			SourceMode:      domain.TaskSourceModeNewProduct,
			TaskStatus:      domain.TaskStatusPendingClose,
			OwnerDepartment: "ops",
			OwnerOrgTeam:    "ops-team-1",
			CreatorID:       600,
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		}
		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 603, Roles: []domain.Role{domain.RoleDeptAdmin, domain.RoleWarehouse}, Department: string(domain.DepartmentCloudWarehouse)}))
		router.POST("/v1/tasks/:id/close", h.Close)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/6/close", `{"operator_id":603}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("warehouse stage-scope close code=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("close_status_and_role_guardrails", func(t *testing.T) {
		taskRepo := &routeTaskRepo{
			tasks: map[int64]*domain.Task{
				61: {
					ID:              61,
					TaskNo:          "T-061",
					SKUCode:         "SKU-061",
					TaskType:        domain.TaskTypeNewProductDevelopment,
					SourceMode:      domain.TaskSourceModeNewProduct,
					TaskStatus:      domain.TaskStatusInProgress,
					OwnerDepartment: "ops",
					OwnerOrgTeam:    "ops-team-1",
				},
				62: {
					ID:              62,
					TaskNo:          "T-062",
					SKUCode:         "SKU-062",
					TaskType:        domain.TaskTypeNewProductDevelopment,
					SourceMode:      domain.TaskSourceModeNewProduct,
					TaskStatus:      domain.TaskStatusPendingClose,
					OwnerDepartment: "ops",
					OwnerOrgTeam:    "ops-team-1",
				},
			},
			details: map[int64]*domain.TaskDetail{
				61: routeReadyDetail(61),
				62: routeReadyDetail(62),
			},
		}
		taskAssetRepo := &routeTaskAssetRepo{
			byTask: map[int64][]*domain.TaskAsset{
				61: {{ID: 611, TaskID: 61, AssetType: domain.TaskAssetTypeDelivery}},
				62: {{ID: 621, TaskID: 62, AssetType: domain.TaskAssetTypeDelivery}},
			},
		}
		warehouseRepo := &routeWarehouseRepo{receipts: map[int64]*domain.WarehouseReceipt{
			61: {TaskID: 61, Status: domain.WarehouseReceiptStatusCompleted},
			62: {TaskID: 62, Status: domain.WarehouseReceiptStatusCompleted},
		}}
		taskSvc := service.NewTaskService(taskRepo, &routeProcurementRepo{}, taskAssetRepo, &routeTaskEventRepo{}, nil, warehouseRepo, nil, routeTxRunner{})
		h := NewTaskHandler(taskSvc, nil, nil)

		router := gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 611, Roles: []domain.Role{domain.RoleAdmin}}))
		router.POST("/v1/tasks/:id/close", h.Close)
		rec := performJSON(router, http.MethodPost, "/v1/tasks/61/close", `{"operator_id":611}`)
		assertTaskPermissionDenied(t, rec, "task_not_closable")

		router = gin.New()
		router.Use(routeActor(domain.RequestActor{ID: 612, Roles: []domain.Role{domain.RoleDesigner}, Department: "ops"}))
		router.POST("/v1/tasks/:id/close", h.Close)
		rec = performJSON(router, http.MethodPost, "/v1/tasks/62/close", `{"operator_id":612}`)
		assertTaskPermissionDenied(t, rec, "missing_required_role")
	})
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
func (r *routeTaskRepo) ListBoardCandidates(context.Context, repo.TaskBoardCandidateFilter) ([]*domain.TaskListItem, error) {
	return []*domain.TaskListItem{}, nil
}
func (r *routeTaskRepo) UpdateDetailBusinessInfo(_ context.Context, _ repo.Tx, detail *domain.TaskDetail) error {
	r.details[detail.TaskID] = cloneRouteDetail(detail)
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

type routeWarehouseRepo struct {
	receipts map[int64]*domain.WarehouseReceipt
}

func (r *routeWarehouseRepo) Create(_ context.Context, _ repo.Tx, receipt *domain.WarehouseReceipt) (int64, error) {
	if r.receipts == nil {
		r.receipts = map[int64]*domain.WarehouseReceipt{}
	}
	r.receipts[receipt.TaskID] = cloneRouteReceipt(receipt)
	return int64(len(r.receipts)), nil
}
func (r *routeWarehouseRepo) GetByID(context.Context, int64) (*domain.WarehouseReceipt, error) {
	return nil, nil
}
func (r *routeWarehouseRepo) GetByTaskID(_ context.Context, taskID int64) (*domain.WarehouseReceipt, error) {
	return cloneRouteReceipt(r.receipts[taskID]), nil
}
func (r *routeWarehouseRepo) List(context.Context, repo.WarehouseListFilter) ([]*domain.WarehouseReceipt, int64, error) {
	return []*domain.WarehouseReceipt{}, 0, nil
}
func (r *routeWarehouseRepo) Update(_ context.Context, _ repo.Tx, receipt *domain.WarehouseReceipt) error {
	r.receipts[receipt.TaskID] = cloneRouteReceipt(receipt)
	return nil
}

type routeAuditRepo struct{}

func (r *routeAuditRepo) CreateRecord(context.Context, repo.Tx, *domain.AuditRecord) (int64, error) {
	return 1, nil
}
func (r *routeAuditRepo) ListRecordsByTaskID(context.Context, int64) ([]*domain.AuditRecord, error) {
	return []*domain.AuditRecord{}, nil
}
func (r *routeAuditRepo) ListRecords(context.Context, repo.AuditRecordListFilter) ([]*domain.AuditRecord, error) {
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

type routeProcurementRepo struct{}

func (r *routeProcurementRepo) GetByTaskID(context.Context, int64) (*domain.ProcurementRecord, error) {
	return nil, nil
}
func (r *routeProcurementRepo) ListItemsByTaskID(context.Context, int64) ([]*domain.ProcurementRecordItem, error) {
	return []*domain.ProcurementRecordItem{}, nil
}
func (r *routeProcurementRepo) Upsert(context.Context, repo.Tx, *domain.ProcurementRecord) error {
	return nil
}
func (r *routeProcurementRepo) CreateItems(context.Context, repo.Tx, []*domain.ProcurementRecordItem) error {
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
func (r *routeUserRepo) GetByJstUID(context.Context, int64) (*domain.User, error)    { return nil, nil }
func (r *routeUserRepo) List(context.Context, repo.UserListFilter) ([]*domain.User, int64, error) {
	return []*domain.User{}, 0, nil
}
func (r *routeUserRepo) ListActiveByRole(context.Context, domain.Role) ([]*domain.User, error) {
	return []*domain.User{}, nil
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

func cloneRouteReceipt(receipt *domain.WarehouseReceipt) *domain.WarehouseReceipt {
	if receipt == nil {
		return nil
	}
	out := *receipt
	out.ReceiverID = cloneRouteInt64(receipt.ReceiverID)
	return &out
}
