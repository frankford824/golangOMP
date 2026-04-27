package service

import (
	"context"
	"log"
	"strings"
	"sync"

	"workflow/domain"
)

var dataScopeManagedDepartmentFallbackLogged sync.Map

// DataScope describes resolved data visibility boundaries.
// V1 keeps role-based resolution conservative and leaves room for
// user-level overrides / cross-department grants in future.
type DataScope struct {
	ViewAll                bool
	DepartmentCodes        []string
	TeamCodes              []string
	ManagedDepartmentCodes []string
	ManagedTeamCodes       []string
	UserIDs                []int64
	StageVisibilities      []StageVisibility
}

type StageVisibility struct {
	Statuses []domain.TaskStatus
	Lane     *domain.WorkflowLane
}

type DataScopeResolver interface {
	Resolve(ctx context.Context, user *domain.User) (*DataScope, error)
}

type roleBasedScopeResolver struct{}

func NewRoleBasedDataScopeResolver() DataScopeResolver {
	return &roleBasedScopeResolver{}
}

func (r *roleBasedScopeResolver) Resolve(_ context.Context, user *domain.User) (*DataScope, error) {
	scope := &DataScope{}
	if user == nil {
		return scope, nil
	}
	if hasAnyRoleValue(user.Roles, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleRoleAdmin, domain.RoleHRAdmin) {
		scope.ViewAll = true
		return scope, nil
	}
	if hasAnyRoleValue(user.Roles, domain.RoleDeptAdmin, domain.RoleDesignDirector) {
		departmentSet := map[string]struct{}{}
		managedDepartmentSet := map[string]struct{}{}
		if user.Department != "" {
			normalizedDepartment := normalizeTaskDepartmentCode(string(user.Department))
			if normalizedDepartment != "" {
				if len(user.ManagedDepartments) == 0 {
					logManagedDepartmentFallbackOnce(user.ID, normalizedDepartment)
				}
				departmentSet[normalizedDepartment] = struct{}{}
			}
		}
		for _, department := range user.ManagedDepartments {
			normalizedDepartment := normalizeTaskDepartmentCode(department)
			if normalizedDepartment != "" {
				departmentSet[normalizedDepartment] = struct{}{}
				managedDepartmentSet[normalizedDepartment] = struct{}{}
			}
		}
		for department := range departmentSet {
			scope.DepartmentCodes = append(scope.DepartmentCodes, department)
		}
		for department := range managedDepartmentSet {
			scope.ManagedDepartmentCodes = append(scope.ManagedDepartmentCodes, department)
		}
		for _, team := range user.ManagedTeams {
			trimmed := strings.TrimSpace(team)
			if trimmed != "" {
				scope.ManagedTeamCodes = append(scope.ManagedTeamCodes, trimmed)
			}
		}
	}
	if hasRoleValue(user.Roles, domain.RoleTeamLead) {
		// TeamLead reads department-wide (all tasks in own department) but
		// write actions are restricted to own team by taskActionActorCanUseScope.
		if user.Department != "" {
			deptCode := normalizeTaskDepartmentCode(string(user.Department))
			if deptCode != "" {
				alreadyHasDept := false
				for _, existing := range scope.DepartmentCodes {
					if existing == deptCode {
						alreadyHasDept = true
						break
					}
				}
				if !alreadyHasDept {
					scope.DepartmentCodes = append(scope.DepartmentCodes, deptCode)
				}
			}
		}
		teamSet := map[string]struct{}{}
		if strings.TrimSpace(user.Team) != "" {
			teamSet[strings.TrimSpace(user.Team)] = struct{}{}
		}
		for _, team := range user.ManagedTeams {
			trimmed := strings.TrimSpace(team)
			if trimmed != "" {
				teamSet[trimmed] = struct{}{}
			}
		}
		for team := range teamSet {
			scope.TeamCodes = append(scope.TeamCodes, team)
		}
	}
	if user.ID > 0 {
		scope.UserIDs = append(scope.UserIDs, user.ID)
	}
	scope.StageVisibilities = buildRoleBasedStageVisibilities(user.Roles, string(user.Department))
	return scope, nil
}

func logManagedDepartmentFallbackOnce(userID int64, department string) {
	if userID <= 0 || strings.TrimSpace(department) == "" {
		return
	}
	if _, loaded := dataScopeManagedDepartmentFallbackLogged.LoadOrStore(userID, struct{}{}); loaded {
		return
	}
	log.Printf("WARN data_scope: managed_departments empty, falling back to user.Department user_id=%d department=%s", userID, department)
}

func buildRoleBasedStageVisibilities(roles []domain.Role, department string) []StageVisibility {
	stageBuilder := newStageVisibilityBuilder()
	if hasRoleValue(roles, domain.RoleAuditA) {
		stageBuilder.Grant(domain.WorkflowLaneNormal,
			domain.TaskStatusPendingAuditA,
			domain.TaskStatusRejectedByAuditA,
		)
	}
	if hasRoleValue(roles, domain.RoleAuditB) {
		stageBuilder.Grant(domain.WorkflowLaneNormal,
			domain.TaskStatusPendingAuditB,
			domain.TaskStatusRejectedByAuditB,
		)
	}
	if hasRoleValue(roles, domain.RoleWarehouse) {
		stageBuilder.GrantNoLane(
			domain.TaskStatusPendingWarehouseQC,
			domain.TaskStatusPendingWarehouseReceive,
			domain.TaskStatusRejectedByWarehouse,
			domain.TaskStatusPendingProductionTransfer,
		)
	}
	if hasRoleValue(roles, domain.RoleOutsource) {
		stageBuilder.GrantNoLane(
			domain.TaskStatusPendingOutsource,
			domain.TaskStatusOutsourcing,
			domain.TaskStatusPendingOutsourceReview,
		)
	}
	if hasRoleValue(roles, domain.RoleCustomizationOperator) {
		stageBuilder.Grant(domain.WorkflowLaneCustomization,
			domain.TaskStatusInProgress,
			domain.TaskStatusPendingCustomizationProduction,
			domain.TaskStatusRejectedByAuditA,
			domain.TaskStatusRejectedByAuditB,
		)
	}
	if hasRoleValue(roles, domain.RoleCustomizationReviewer) {
		stageBuilder.Grant(domain.WorkflowLaneCustomization,
			domain.TaskStatusPendingCustomizationReview,
			domain.TaskStatusPendingEffectReview,
			domain.TaskStatusPendingEffectRevision,
		)
	}
	if hasRoleValue(roles, domain.RoleDeptAdmin) {
		switch domain.Department(normalizeTaskDepartmentCode(department)) {
		case domain.DepartmentAudit:
			stageBuilder.Grant(domain.WorkflowLaneNormal,
				domain.TaskStatusPendingAuditA,
				domain.TaskStatusRejectedByAuditA,
				domain.TaskStatusPendingAuditB,
				domain.TaskStatusRejectedByAuditB,
			)
			stageBuilder.Grant(domain.WorkflowLaneCustomization,
				domain.TaskStatusPendingCustomizationReview,
				domain.TaskStatusPendingEffectReview,
				domain.TaskStatusPendingEffectRevision,
			)
		case domain.DepartmentCloudWarehouse:
			stageBuilder.GrantNoLane(
				domain.TaskStatusPendingWarehouseQC,
				domain.TaskStatusPendingWarehouseReceive,
				domain.TaskStatusRejectedByWarehouse,
				domain.TaskStatusPendingProductionTransfer,
			)
		case domain.DepartmentCustomizationArt:
			stageBuilder.Grant(domain.WorkflowLaneCustomization,
				domain.TaskStatusInProgress,
				domain.TaskStatusPendingCustomizationProduction,
				domain.TaskStatusRejectedByAuditA,
				domain.TaskStatusRejectedByAuditB,
				domain.TaskStatusPendingCustomizationReview,
				domain.TaskStatusPendingEffectReview,
				domain.TaskStatusPendingEffectRevision,
			)
		}
	}
	return stageBuilder.Build()
}

type stageVisibilityBuilder struct {
	order   []string
	buckets map[string]*stageVisibilityBucket
}

type stageVisibilityBucket struct {
	lane     *domain.WorkflowLane
	statuses []domain.TaskStatus
	seen     map[domain.TaskStatus]struct{}
}

func newStageVisibilityBuilder() *stageVisibilityBuilder {
	return &stageVisibilityBuilder{
		order:   []string{},
		buckets: map[string]*stageVisibilityBucket{},
	}
}

func (b *stageVisibilityBuilder) Grant(lane domain.WorkflowLane, statuses ...domain.TaskStatus) {
	laneCopy := lane
	b.grant(&laneCopy, statuses...)
}

func (b *stageVisibilityBuilder) GrantNoLane(statuses ...domain.TaskStatus) {
	b.grant(nil, statuses...)
}

func (b *stageVisibilityBuilder) grant(lane *domain.WorkflowLane, statuses ...domain.TaskStatus) {
	key := stageVisibilityBucketKey(lane)
	bucket, ok := b.buckets[key]
	if !ok {
		bucket = &stageVisibilityBucket{
			lane:     cloneWorkflowLane(lane),
			statuses: []domain.TaskStatus{},
			seen:     map[domain.TaskStatus]struct{}{},
		}
		b.buckets[key] = bucket
		b.order = append(b.order, key)
	}
	for _, status := range statuses {
		if status == "" {
			continue
		}
		if _, exists := bucket.seen[status]; exists {
			continue
		}
		bucket.seen[status] = struct{}{}
		bucket.statuses = append(bucket.statuses, status)
	}
}

func (b *stageVisibilityBuilder) Build() []StageVisibility {
	out := make([]StageVisibility, 0, len(b.order))
	for _, key := range b.order {
		bucket := b.buckets[key]
		if bucket == nil || len(bucket.statuses) == 0 {
			continue
		}
		out = append(out, StageVisibility{
			Statuses: append([]domain.TaskStatus(nil), bucket.statuses...),
			Lane:     cloneWorkflowLane(bucket.lane),
		})
	}
	return out
}

func stageVisibilityBucketKey(lane *domain.WorkflowLane) string {
	if lane == nil {
		return "*"
	}
	return string(*lane)
}

func cloneWorkflowLane(lane *domain.WorkflowLane) *domain.WorkflowLane {
	if lane == nil {
		return nil
	}
	copyValue := *lane
	return &copyValue
}

func hasRoleValue(roles []domain.Role, target domain.Role) bool {
	for _, role := range roles {
		if role == target {
			return true
		}
	}
	return false
}

func hasAnyRoleValue(roles []domain.Role, targets ...domain.Role) bool {
	for _, target := range targets {
		if hasRoleValue(roles, target) {
			return true
		}
	}
	return false
}
