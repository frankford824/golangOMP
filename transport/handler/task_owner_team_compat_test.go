package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/repo"
	"workflow/service"
)

func TestTaskHandlerCreateOrgTeamCompatOwnerTeamSucceeds(t *testing.T) {
	for _, mapping := range service.ListTaskOwnerTeamCompatMappings() {
		mapping := mapping
		t.Run(mapping.OrgTeam, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			taskSvc := newOwnerTeamCompatTaskService()
			handler := NewTaskHandler(taskSvc, nil, nil)
			router.POST("/v1/tasks", handler.Create)

			body := map[string]interface{}{
				"task_type":          "new_product_development",
				"source_mode":        "new_product",
				"creator_id":         9,
				"sku_code":           "TEST-" + mapping.OrgTeam,
				"owner_team":         mapping.OrgTeam,
				"due_at":             "2026-04-01T00:00:00Z",
				"category_code":      "LIGHTBOX",
				"material_mode":      "preset",
				"material":           "AL",
				"product_name":       "Compat Product",
				"product_short_name": "Compat",
				"design_requirement": "need design",
			}
			raw, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Trace-ID", "handler-owner-team-compat")
			req = req.WithContext(domain.ContextWithTraceID(req.Context(), "handler-owner-team-compat"))

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("POST /v1/tasks code = %d, want 201 body=%s", rec.Code, rec.Body.String())
			}

			var resp struct {
				Data struct {
					OwnerTeam       string `json:"owner_team"`
					OwnerDepartment string `json:"owner_department"`
					OwnerOrgTeam    string `json:"owner_org_team"`
				} `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("json.Unmarshal() error = %v body=%s", err, rec.Body.String())
			}
			if resp.Data.OwnerTeam != mapping.LegacyOwnerTeam {
				t.Fatalf("response owner_team = %q, want %q", resp.Data.OwnerTeam, mapping.LegacyOwnerTeam)
			}
			if resp.Data.OwnerDepartment == "" {
				t.Fatalf("response owner_department is empty for %q", mapping.OrgTeam)
			}
			if resp.Data.OwnerOrgTeam != mapping.OrgTeam {
				t.Fatalf("response owner_org_team = %q, want %q", resp.Data.OwnerOrgTeam, mapping.OrgTeam)
			}
		})
	}
}

func TestTaskHandlerCreateInvalidOwnerTeamStillReturnsViolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	taskSvc := newOwnerTeamCompatTaskService()
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks", handler.Create)

	body := map[string]interface{}{
		"task_type":          "new_product_development",
		"source_mode":        "new_product",
		"creator_id":         9,
		"owner_team":         "\u4e0d\u5b58\u5728\u7684\u7ec4",
		"due_at":             "2026-04-01T00:00:00Z",
		"category_code":      "LIGHTBOX",
		"material_mode":      "preset",
		"material":           "AL",
		"product_name":       "Invalid Team Product",
		"product_short_name": "Invalid",
		"design_requirement": "need design",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/tasks code = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Details struct {
				Violations []struct {
					Field string `json:"field"`
					Code  string `json:"code"`
				} `json:"violations"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v body=%s", err, rec.Body.String())
	}
	if resp.Error.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("error.code = %q, want %q", resp.Error.Code, domain.ErrCodeInvalidRequest)
	}
	if len(resp.Error.Details.Violations) == 0 {
		t.Fatalf("violations missing: %s", rec.Body.String())
	}
	if resp.Error.Details.Violations[0].Field != "owner_team" {
		t.Fatalf("violation.field = %q, want owner_team", resp.Error.Details.Violations[0].Field)
	}
	if resp.Error.Details.Violations[0].Code != "invalid_owner_team" {
		t.Fatalf("violation.code = %q, want invalid_owner_team", resp.Error.Details.Violations[0].Code)
	}
}

func newOwnerTeamCompatTaskService() service.TaskService {
	realSvc := service.NewTaskService(
		&ownerTeamTaskRepo{},
		&ownerTeamTaskAssetRepo{},
		&ownerTeamTaskEventRepo{},
		nil,
		ownerTeamCodeRuleService{},
		ownerTeamTxRunner{},
	)
	return &ownerTeamCompatTaskServiceProxy{createSvc: realSvc}
}

type ownerTeamCompatTaskServiceProxy struct {
	createSvc service.TaskService
	lastTask  *domain.Task
}

func (s *ownerTeamCompatTaskServiceProxy) Create(ctx context.Context, p service.CreateTaskParams) (*domain.Task, *domain.AppError) {
	task, appErr := s.createSvc.Create(ctx, p)
	if task != nil {
		copied := *task
		s.lastTask = &copied
	}
	return task, appErr
}

func (s *ownerTeamCompatTaskServiceProxy) List(context.Context, service.TaskFilter) ([]*domain.TaskListItem, domain.PaginationMeta, *domain.AppError) {
	return nil, domain.PaginationMeta{}, nil
}

func (s *ownerTeamCompatTaskServiceProxy) GetByID(_ context.Context, id int64) (*domain.TaskReadModel, *domain.AppError) {
	if s.lastTask == nil || s.lastTask.ID != id {
		return nil, domain.ErrNotFound
	}
	return &domain.TaskReadModel{Task: *s.lastTask, ReferenceFileRefs: []domain.ReferenceFileRef{}}, nil
}

func (s *ownerTeamCompatTaskServiceProxy) GetFilingStatus(context.Context, int64) (*domain.TaskFilingStatusView, *domain.AppError) {
	return nil, nil
}

func (s *ownerTeamCompatTaskServiceProxy) RetryFiling(context.Context, service.RetryTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError) {
	return nil, nil
}

func (s *ownerTeamCompatTaskServiceProxy) TriggerFiling(context.Context, service.TriggerTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError) {
	return nil, nil
}

func (s *ownerTeamCompatTaskServiceProxy) UpdateBusinessInfo(context.Context, service.UpdateTaskBusinessInfoParams) (*domain.TaskDetail, *domain.AppError) {
	return nil, nil
}

func (s *ownerTeamCompatTaskServiceProxy) UpdateSKUItemInfo(context.Context, service.UpdateTaskSKUItemInfoParams) (*domain.TaskSKUItem, *domain.AppError) {
	return nil, nil
}

type ownerTeamTaskRepo struct {
	tasks map[int64]*domain.Task
}

func (r *ownerTeamTaskRepo) Create(_ context.Context, _ repo.Tx, task *domain.Task, detail *domain.TaskDetail) (int64, error) {
	if r.tasks == nil {
		r.tasks = map[int64]*domain.Task{}
	}
	if task.ID == 0 {
		task.ID = int64(len(r.tasks) + 1)
	}
	detail.TaskID = task.ID
	r.tasks[task.ID] = task
	return task.ID, nil
}

func (r *ownerTeamTaskRepo) CreateSKUItems(context.Context, repo.Tx, []*domain.TaskSKUItem) error {
	return nil
}

func (r *ownerTeamTaskRepo) GetByID(_ context.Context, id int64) (*domain.Task, error) {
	return r.tasks[id], nil
}

func (r *ownerTeamTaskRepo) GetDetailByTaskID(_ context.Context, taskID int64) (*domain.TaskDetail, error) {
	return &domain.TaskDetail{
		TaskID:            taskID,
		CategoryCode:      "LIGHTBOX",
		MaterialMode:      string(domain.MaterialModePreset),
		Material:          "AL",
		ProductShortName:  "Compat",
		DesignRequirement: "need design",
	}, nil
}

func (r *ownerTeamTaskRepo) GetSKUItemBySKUCode(context.Context, string) (*domain.TaskSKUItem, error) {
	return nil, nil
}

func (r *ownerTeamTaskRepo) ListSKUItemsByTaskID(context.Context, int64) ([]*domain.TaskSKUItem, error) {
	return []*domain.TaskSKUItem{}, nil
}

func (r *ownerTeamTaskRepo) List(context.Context, repo.TaskListFilter) ([]*domain.TaskListItem, int64, error) {
	return []*domain.TaskListItem{}, 0, nil
}

func (r *ownerTeamTaskRepo) UpdateDetailBusinessInfo(context.Context, repo.Tx, *domain.TaskDetail) error {
	return nil
}

func (r *ownerTeamTaskRepo) UpdatePriority(context.Context, repo.Tx, int64, domain.TaskPriority) error {
	return nil
}

func (r *ownerTeamTaskRepo) UpdateProductBinding(context.Context, repo.Tx, *domain.Task) error {
	return nil
}

func (r *ownerTeamTaskRepo) UpdateStatus(context.Context, repo.Tx, int64, domain.TaskStatus) error {
	return nil
}

func (r *ownerTeamTaskRepo) UpdateDesigner(context.Context, repo.Tx, int64, *int64) error {
	return nil
}

func (r *ownerTeamTaskRepo) UpdateHandler(context.Context, repo.Tx, int64, *int64) error {
	return nil
}

func (r *ownerTeamTaskRepo) UpdateCustomizationState(context.Context, repo.Tx, int64, *int64, string, string) error {
	return nil
}

type ownerTeamTaskAssetRepo struct{}

func (r *ownerTeamTaskAssetRepo) Create(context.Context, repo.Tx, *domain.TaskAsset) (int64, error) {
	return 0, nil
}

func (r *ownerTeamTaskAssetRepo) GetByID(context.Context, int64) (*domain.TaskAsset, error) {
	return nil, nil
}

func (r *ownerTeamTaskAssetRepo) ListByTaskID(context.Context, int64) ([]*domain.TaskAsset, error) {
	return []*domain.TaskAsset{}, nil
}

func (r *ownerTeamTaskAssetRepo) ListByAssetID(context.Context, int64) ([]*domain.TaskAsset, error) {
	return []*domain.TaskAsset{}, nil
}

func (r *ownerTeamTaskAssetRepo) NextVersionNo(context.Context, repo.Tx, int64) (int, error) {
	return 1, nil
}

func (r *ownerTeamTaskAssetRepo) NextAssetVersionNo(context.Context, repo.Tx, int64) (int, error) {
	return 1, nil
}

type ownerTeamTaskEventRepo struct{}

func (r *ownerTeamTaskEventRepo) Append(_ context.Context, _ repo.Tx, taskID int64, eventType string, operatorID *int64, payload interface{}) (*domain.TaskEvent, error) {
	raw, _ := json.Marshal(payload)
	return &domain.TaskEvent{
		ID:         "evt",
		TaskID:     taskID,
		EventType:  eventType,
		OperatorID: operatorID,
		Payload:    raw,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

func (r *ownerTeamTaskEventRepo) ListByTaskID(context.Context, int64) ([]*domain.TaskEvent, error) {
	return []*domain.TaskEvent{}, nil
}

func (r *ownerTeamTaskEventRepo) ListRecent(context.Context, repo.TaskEventListFilter) ([]*domain.TaskEvent, int64, error) {
	return []*domain.TaskEvent{}, 0, nil
}

type ownerTeamCodeRuleService struct{}

func (ownerTeamCodeRuleService) List(context.Context) ([]*domain.CodeRule, *domain.AppError) {
	return nil, nil
}

func (ownerTeamCodeRuleService) Preview(context.Context, int64) (*domain.CodePreview, *domain.AppError) {
	return nil, nil
}

func (ownerTeamCodeRuleService) GenerateCode(_ context.Context, ruleType domain.CodeRuleType) (string, *domain.AppError) {
	if ruleType == domain.CodeRuleTypeNewSKU {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "legacy CodeRule new_sku is archived", nil)
	}
	return "RW-TEST", nil
}

type ownerTeamTx struct{}

func (ownerTeamTx) IsTx() {}

type ownerTeamTxRunner struct{}

func (ownerTeamTxRunner) RunInTx(_ context.Context, fn func(tx repo.Tx) error) error {
	return fn(ownerTeamTx{})
}
