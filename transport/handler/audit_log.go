package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/repo"
)

// AuditLogHandler handles GET /v1/audit-logs (cross-task audit records list).
type AuditLogHandler struct {
	auditV7Repo repo.AuditV7Repo
	taskRepo    repo.TaskRepo
	userRepo    repo.UserRepo
}

// NewAuditLogHandler creates the handler.
func NewAuditLogHandler(auditV7Repo repo.AuditV7Repo, taskRepo repo.TaskRepo, userRepo repo.UserRepo) *AuditLogHandler {
	return &AuditLogHandler{
		auditV7Repo: auditV7Repo,
		taskRepo:    taskRepo,
		userRepo:    userRepo,
	}
}

// List handles GET /v1/audit-logs
func (h *AuditLogHandler) List(c *gin.Context) {
	actor, ok := domain.RequestActorFromContext(c.Request.Context())
	if !ok || actor.ID <= 0 {
		respondError(c, domain.ErrUnauthorized)
		return
	}
	if !canReadAuditLogs(actor) {
		respondError(c, domain.NewAppError(domain.ErrCodePermissionDenied, "audit logs require audit or management access", map[string]interface{}{
			"deny_code": "audit_log_access_denied",
		}))
		return
	}
	filter := repo.AuditRecordListFilter{
		TaskNo:   c.Query("taskNo"),
		Auditor:  c.Query("auditor"),
		Action:   c.Query("action"),
		StartAt:  c.Query("start"),
		EndAt:    c.Query("end"),
		Page:     1,
		PageSize: 100,
	}
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			filter.Page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 500 {
			filter.PageSize = v
		}
	}

	records, err := h.auditV7Repo.ListRecords(c.Request.Context(), filter)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "list audit logs failed", map[string]interface{}{"error": err.Error()}))
		return
	}

	// Enrich with task_no and auditor display_name
	items := make([]map[string]interface{}, 0, len(records))
	for _, r := range records {
		taskNo := ""
		task, _ := h.taskRepo.GetByID(c.Request.Context(), r.TaskID)
		if task != nil {
			if !auditLogVisibleToActor(actor, task) {
				continue
			}
			taskNo = task.TaskNo
		}
		auditorName := ""
		if user, _ := h.userRepo.GetByID(c.Request.Context(), r.AuditorID); user != nil {
			auditorName = user.DisplayName
			if auditorName == "" {
				auditorName = user.Username
			}
		}
		items = append(items, map[string]interface{}{
			"id":             r.ID,
			"task_id":        r.TaskID,
			"task_no":        taskNo,
			"stage":          string(r.Stage),
			"action":         string(r.Action),
			"auditor_id":     r.AuditorID,
			"auditor_name":   auditorName,
			"comment":        r.Comment,
			"affects_launch": r.AffectsLaunch,
			"need_outsource": r.NeedOutsource,
			"created_at":     r.CreatedAt,
		})
	}

	respondOK(c, gin.H{"data": items})
}

func canReadAuditLogs(actor domain.RequestActor) bool {
	return hasAnyAuditLogRole(actor.Roles,
		domain.RoleAdmin,
		domain.RoleSuperAdmin,
		domain.RoleHRAdmin,
		domain.RoleDeptAdmin,
		domain.RoleTeamLead,
		domain.RoleAuditA,
		domain.RoleAuditB,
		domain.RoleCustomizationReviewer,
	)
}

func auditLogVisibleToActor(actor domain.RequestActor, task *domain.Task) bool {
	if task == nil {
		return false
	}
	if hasAnyAuditLogRole(actor.Roles,
		domain.RoleAdmin,
		domain.RoleSuperAdmin,
		domain.RoleHRAdmin,
		domain.RoleAuditA,
		domain.RoleAuditB,
		domain.RoleCustomizationReviewer,
	) {
		return true
	}
	actorDepartment := strings.TrimSpace(actor.Department)
	if actorDepartment == "" {
		actorDepartment = strings.TrimSpace(actor.FrontendAccess.Department)
	}
	if hasAnyAuditLogRole(actor.Roles, domain.RoleDeptAdmin) {
		return strings.EqualFold(actorDepartment, strings.TrimSpace(task.OwnerDepartment))
	}
	if hasAnyAuditLogRole(actor.Roles, domain.RoleTeamLead) {
		return strings.EqualFold(actorDepartment, strings.TrimSpace(task.OwnerDepartment))
	}
	return false
}

func hasAnyAuditLogRole(roles []domain.Role, targets ...domain.Role) bool {
	for _, role := range domain.NormalizeRoleValues(roles) {
		for _, target := range targets {
			if role == target {
				return true
			}
		}
	}
	return false
}
