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
)

func TestAuditLogHandlerRejectsOrdinaryUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(routeActor(domain.RequestActor{
		ID:         9,
		Roles:      []domain.Role{domain.RoleOps},
		Department: "ops",
		Team:       "ops-team-1",
	}))
	router.GET("/v1/audit-logs", NewAuditLogHandler(auditLogAuditRepo{}, &routeTaskRepo{}, &routeUserRepo{}).List)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit-logs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuditLogHandlerFiltersDepartmentAdminToOwnDepartment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(routeActor(domain.RequestActor{
		ID:         10,
		Roles:      []domain.Role{domain.RoleDeptAdmin},
		Department: "ops",
		Team:       "ops-team-9",
	}))
	router.GET("/v1/audit-logs", NewAuditLogHandler(
		auditLogAuditRepo{
			records: []*domain.AuditRecord{
				{ID: 1, TaskID: 11, Stage: domain.AuditRecordStageB, Action: domain.AuditActionTypeApprove, AuditorID: 101, CreatedAt: time.Now().UTC()},
				{ID: 2, TaskID: 12, Stage: domain.AuditRecordStageB, Action: domain.AuditActionTypeReject, AuditorID: 102, CreatedAt: time.Now().UTC()},
			},
		},
		&routeTaskRepo{
			tasks: map[int64]*domain.Task{
				11: {ID: 11, TaskNo: "TASK-11", OwnerDepartment: "ops", OwnerOrgTeam: "ops-team-1"},
				12: {ID: 12, TaskNo: "TASK-12", OwnerDepartment: "design", OwnerOrgTeam: "design-team-1"},
			},
		},
		&routeUserRepo{
			users: map[int64]*domain.User{
				101: {ID: 101, Username: "auditor_ops", DisplayName: "Auditor Ops"},
				102: {ID: 102, Username: "auditor_design", DisplayName: "Auditor Design"},
			},
		},
	).List)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit-logs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "\"task_id\":12") {
		t.Fatalf("body includes out-of-scope task: %s", body)
	}
	if !strings.Contains(body, "\"task_id\":11") {
		t.Fatalf("body missing in-scope task: %s", body)
	}
}

func TestAuditLogHandlerAllowsAuditRoleFullRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(routeActor(domain.RequestActor{
		ID:    11,
		Roles: []domain.Role{domain.RoleAuditA},
	}))
	router.GET("/v1/audit-logs", NewAuditLogHandler(
		auditLogAuditRepo{
			records: []*domain.AuditRecord{
				{ID: 1, TaskID: 21, Stage: domain.AuditRecordStageA, Action: domain.AuditActionTypeClaim, AuditorID: 101, CreatedAt: time.Now().UTC()},
				{ID: 2, TaskID: 22, Stage: domain.AuditRecordStageB, Action: domain.AuditActionTypeApprove, AuditorID: 102, CreatedAt: time.Now().UTC()},
			},
		},
		&routeTaskRepo{
			tasks: map[int64]*domain.Task{
				21: {ID: 21, TaskNo: "TASK-21", OwnerDepartment: "ops", OwnerOrgTeam: "ops-team-1"},
				22: {ID: 22, TaskNo: "TASK-22", OwnerDepartment: "design", OwnerOrgTeam: "design-team-1"},
			},
		},
		&routeUserRepo{
			users: map[int64]*domain.User{
				101: {ID: 101, Username: "auditor_a"},
				102: {ID: 102, Username: "auditor_b"},
			},
		},
	).List)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit-logs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			Data []map[string]interface{} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rec.Body.String())
	}
	if len(resp.Data.Data) != 2 {
		t.Fatalf("data length = %d, want 2 body=%s", len(resp.Data.Data), rec.Body.String())
	}
}

type auditLogAuditRepo struct {
	records []*domain.AuditRecord
}

func (r auditLogAuditRepo) CreateRecord(_ context.Context, _ repo.Tx, _ *domain.AuditRecord) (int64, error) {
	return 1, nil
}

func (r auditLogAuditRepo) ListRecordsByTaskID(_ context.Context, _ int64) ([]*domain.AuditRecord, error) {
	return r.records, nil
}

func (r auditLogAuditRepo) ListRecords(_ context.Context, _ repo.AuditRecordListFilter) ([]*domain.AuditRecord, error) {
	return r.records, nil
}

func (r auditLogAuditRepo) CreateHandover(_ context.Context, _ repo.Tx, _ *domain.AuditHandover) (int64, error) {
	return 1, nil
}

func (r auditLogAuditRepo) GetHandoverByID(_ context.Context, _ int64) (*domain.AuditHandover, error) {
	return nil, nil
}

func (r auditLogAuditRepo) ListHandoversByTaskID(_ context.Context, _ int64) ([]*domain.AuditHandover, error) {
	return []*domain.AuditHandover{}, nil
}

func (r auditLogAuditRepo) UpdateHandoverStatus(_ context.Context, _ repo.Tx, _ int64, _ domain.HandoverStatus) error {
	return nil
}
