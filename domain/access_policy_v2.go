package domain

import (
	"sort"
	"time"
)

type PermissionCode string

const (
	PermissionTaskView                 PermissionCode = "task.view"
	PermissionTaskCreate               PermissionCode = "task.create"
	PermissionTaskAssign               PermissionCode = "task.assign"
	PermissionTaskReassign             PermissionCode = "task.reassign"
	PermissionTaskTerminate            PermissionCode = "task.terminate"
	PermissionTaskUploadSource         PermissionCode = "task.upload_source"
	PermissionTaskAudit                PermissionCode = "task.audit"
	PermissionTaskAuditHandover        PermissionCode = "task.audit_handover"
	PermissionTaskReopen               PermissionCode = "task.reopen"
	PermissionPlanningSKUView          PermissionCode = "planning_sku.view"
	PermissionPlanningSKUCreate        PermissionCode = "planning_sku.create"
	PermissionPlanningSKUEdit          PermissionCode = "planning_sku.edit"
	PermissionPlanningSKUExport        PermissionCode = "planning_sku.export"
	PermissionPlanningSKUSync          PermissionCode = "planning_sku.erp_sync"
	PermissionPlanningSKURetry         PermissionCode = "planning_sku.erp_retry"
	PermissionAssetView                PermissionCode = "asset.view"
	PermissionAssetDownload            PermissionCode = "asset.download"
	PermissionAssetExport              PermissionCode = "asset.export"
	PermissionAssetPublish             PermissionCode = "asset.publish"
	PermissionAssetManage              PermissionCode = "asset.manage"
	PermissionAssetWorkbenchUse        PermissionCode = "asset_workbench.use"
	PermissionAssetWorkbenchSubmit     PermissionCode = "asset_workbench.submit"
	PermissionAssetWorkbenchMembers    PermissionCode = "asset_workbench.members.manage"
	PermissionAssetWorkbenchProfiles   PermissionCode = "asset_workbench.profiles.manage"
	PermissionAssetWorkbenchGroups     PermissionCode = "asset_workbench.groups.manage"
	PermissionAssetWorkbenchDrive      PermissionCode = "asset_workbench.drive.manage"
	PermissionAssetWorkbenchBatch      PermissionCode = "asset_workbench.batch.manage"
	PermissionAssetWorkbenchTemplates  PermissionCode = "asset_workbench.templates.manage"
	PermissionAssetWorkbenchQC         PermissionCode = "asset_workbench.qc.manage"
	PermissionAssetWorkbenchSettlement PermissionCode = "asset_workbench.settlement.manage"
	PermissionAssetWorkbenchAuditView  PermissionCode = "asset_workbench.audit.view"
	PermissionCatalogView              PermissionCode = "catalog.view"
	PermissionCatalogManage            PermissionCode = "catalog.manage"
	PermissionERPManage                PermissionCode = "erp.manage"
	PermissionAccountUse               PermissionCode = "account.use"
	PermissionReportView               PermissionCode = "report.view"
	PermissionSystemManage             PermissionCode = "system.manage"
	PermissionAccessView               PermissionCode = "access.view"
	PermissionAccessManage             PermissionCode = "access.manage"

	// One-to-one aliases keep existing task/access call sites readable while the
	// persisted capability names move to operation language.
	PermissionTaskDesignSubmit  = PermissionTaskUploadSource
	PermissionTaskAuditDecision = PermissionTaskAudit
	PermissionAccessPolicyView  = PermissionAccessView
	PermissionAccessPolicyAdmin = PermissionAccessManage
)

// PermissionSupportsTaskTypes reports whether an operation may be restricted by task type.
func PermissionSupportsTaskTypes(code PermissionCode) bool {
	switch code {
	case PermissionTaskCreate, PermissionTaskAssign, PermissionTaskReassign, PermissionTaskTerminate, PermissionTaskUploadSource, PermissionTaskAudit:
		return true
	default:
		return false
	}
}

// AccessTaskTypeValid restricts new access-policy grants to task types that
// remain writable in the current workflow. Historical task type values are
// read-only evidence and must never be introduced by a new policy.
func AccessTaskTypeValid(taskType TaskType) bool {
	switch taskType {
	case TaskTypeOriginalProductDevelopment, TaskTypeNewProductDevelopment, TaskTypeRetouchTask,
		TaskTypeSKUPlanning, TaskTypeCustomerCustomization, TaskTypeRegularCustomization:
		return true
	default:
		return false
	}
}

type AccessScopeMode string

const (
	AccessScopeSelf          AccessScopeMode = "self"
	AccessScopeOwnDepartment AccessScopeMode = "own_department"
	AccessScopeOwnTeam       AccessScopeMode = "own_team"
	AccessScopeSelectedOrg   AccessScopeMode = "selected_org"
	AccessScopeGlobal        AccessScopeMode = "global"
)

func (m AccessScopeMode) Valid() bool {
	switch m {
	case AccessScopeSelf, AccessScopeOwnDepartment, AccessScopeOwnTeam, AccessScopeSelectedOrg, AccessScopeGlobal:
		return true
	default:
		return false
	}
}

type AccessSubjectType string

const (
	AccessSubjectDepartment AccessSubjectType = "department"
	AccessSubjectTeam       AccessSubjectType = "team"
)

func (t AccessSubjectType) Valid() bool {
	return t == AccessSubjectDepartment || t == AccessSubjectTeam
}

type AccessPermission struct {
	Code        PermissionCode `json:"code"`
	Module      string         `json:"module"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	RiskLevel   string         `json:"risk_level"`
	Enabled     bool           `json:"enabled"`
}

// AccessRolePermission is one operation granted to a role, optionally limited to task types.
// Empty TaskTypes means all task types are allowed for that operation.
type AccessRolePermission struct {
	Code      PermissionCode `json:"code"`
	TaskTypes []string       `json:"task_types,omitempty"`
}

type AccessRole struct {
	ID              int64                  `json:"id"`
	Code            string                 `json:"code"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	SystemProtected bool                   `json:"system_protected"`
	ArchivedAt      *time.Time             `json:"archived_at,omitempty"`
	Version         int64                  `json:"version"`
	Permissions     []AccessRolePermission `json:"permissions"`
}

func (r AccessRole) PermissionCodes() []PermissionCode {
	out := make([]PermissionCode, 0, len(r.Permissions))
	for _, item := range r.Permissions {
		out = append(out, item.Code)
	}
	return out
}

type AccessScopeSubject struct {
	SubjectType AccessSubjectType `json:"subject_type"`
	SubjectID   int64             `json:"subject_id"`
	SubjectName string            `json:"subject_name,omitempty"`
}

type AccessAssignment struct {
	ID         int64                `json:"id,omitempty"`
	UserID     int64                `json:"user_id"`
	RoleID     int64                `json:"role_id"`
	RoleCode   string               `json:"role_code,omitempty"`
	RoleName   string               `json:"role_name,omitempty"`
	ScopeMode  AccessScopeMode      `json:"scope_mode"`
	Subjects   []AccessScopeSubject `json:"subjects"`
	SourceType string               `json:"source_type,omitempty"`
	SourceRef  *int64               `json:"source_ref_id,omitempty"`
	Version    int64                `json:"version"`
}

type EffectiveAccess struct {
	UserID         int64                 `json:"user_id"`
	PolicyRevision int64                 `json:"policy_revision"`
	Permissions    []PermissionCode      `json:"permissions"`
	Assignments    []AccessAssignment    `json:"assignments"`
	Sources        []EffectiveAccessNote `json:"sources"`
}

type EffectiveAccessNote struct {
	Permission PermissionCode  `json:"permission"`
	RoleID     int64           `json:"role_id"`
	RoleCode   string          `json:"role_code"`
	SourceType string          `json:"source_type"`
	ScopeMode  AccessScopeMode `json:"scope_mode"`
	TaskTypes  []string        `json:"task_types,omitempty"`
}

type TaskAccessSubject struct {
	TaskID            int64
	CreatorID         int64
	RequesterID       *int64
	DesignerID        *int64
	CurrentHandlerID  *int64
	OwnerDepartmentID *int64
	OwnerTeamID       *int64
	TaskType          TaskType
}

func EffectiveAccessAllowsTask(actor RequestActor, permission PermissionCode, subject TaskAccessSubject) bool {
	if actor.EffectiveAccess == nil || !actor.EffectiveAccess.Has(permission) {
		return false
	}
	assignmentsByRole := make(map[int64][]AccessAssignment)
	for _, assignment := range actor.EffectiveAccess.Assignments {
		assignmentsByRole[assignment.RoleID] = append(assignmentsByRole[assignment.RoleID], assignment)
	}
	for _, source := range actor.EffectiveAccess.Sources {
		if source.Permission != permission {
			continue
		}
		if !sourceAllowsTaskType(source, subject.TaskType) {
			continue
		}
		for _, assignment := range assignmentsByRole[source.RoleID] {
			if accessAssignmentAllowsTask(actor, assignment, subject) {
				return true
			}
		}
	}
	return false
}

// EffectiveAccessAllowsTaskReassign preserves stable organization scope as the
// default while also allowing the explicitly assigned current designer/handler
// to delegate an in-progress task when that actor owns a task.reassign grant.
// The grant's task-type restriction still applies; legacy roles and display
// organization names never participate.
func EffectiveAccessAllowsTaskReassign(actor RequestActor, subject TaskAccessSubject) bool {
	if EffectiveAccessAllowsTask(actor, PermissionTaskReassign, subject) {
		return true
	}
	if actor.EffectiveAccess == nil || !actor.EffectiveAccess.Has(PermissionTaskReassign) {
		return false
	}
	if !pointerEqualsActor(subject.DesignerID, actor.ID) && !pointerEqualsActor(subject.CurrentHandlerID, actor.ID) {
		return false
	}
	assignmentsByRole := make(map[int64]struct{}, len(actor.EffectiveAccess.Assignments))
	for _, assignment := range actor.EffectiveAccess.Assignments {
		assignmentsByRole[assignment.RoleID] = struct{}{}
	}
	for _, source := range actor.EffectiveAccess.Sources {
		if source.Permission != PermissionTaskReassign || !sourceAllowsTaskType(source, subject.TaskType) {
			continue
		}
		if _, ok := assignmentsByRole[source.RoleID]; ok {
			return true
		}
	}
	return false
}

func sourceAllowsTaskType(source EffectiveAccessNote, taskType TaskType) bool {
	if !PermissionSupportsTaskTypes(source.Permission) || len(source.TaskTypes) == 0 {
		return true
	}
	if taskType == "" {
		return false
	}
	wanted := string(taskType)
	for _, candidate := range source.TaskTypes {
		if candidate == wanted {
			return true
		}
	}
	return false
}

func accessAssignmentAllowsTask(actor RequestActor, assignment AccessAssignment, subject TaskAccessSubject) bool {
	switch assignment.ScopeMode {
	case AccessScopeGlobal:
		return true
	case AccessScopeSelf:
		return actor.ID == subject.CreatorID || pointerEqualsActor(subject.RequesterID, actor.ID) || pointerEqualsActor(subject.DesignerID, actor.ID) || pointerEqualsActor(subject.CurrentHandlerID, actor.ID)
	case AccessScopeOwnDepartment:
		return actor.DepartmentID != nil && subject.OwnerDepartmentID != nil && *actor.DepartmentID == *subject.OwnerDepartmentID
	case AccessScopeOwnTeam:
		return actor.TeamID != nil && subject.OwnerTeamID != nil && *actor.TeamID == *subject.OwnerTeamID
	case AccessScopeSelectedOrg:
		for _, selected := range assignment.Subjects {
			if selected.SubjectType == AccessSubjectDepartment && subject.OwnerDepartmentID != nil && selected.SubjectID == *subject.OwnerDepartmentID {
				return true
			}
			if selected.SubjectType == AccessSubjectTeam && subject.OwnerTeamID != nil && selected.SubjectID == *subject.OwnerTeamID {
				return true
			}
		}
	}
	return false
}

func pointerEqualsActor(value *int64, actorID int64) bool {
	return value != nil && *value == actorID
}

func (e EffectiveAccess) Has(permission PermissionCode) bool {
	for _, candidate := range e.Permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

// ResourceGroupAccessFilterForActor translates the effective access model to
// the SQL-safe scope filter shared by resource-group lists and global search.
// Role names and organization display names are deliberately not consulted.
func ResourceGroupAccessFilterForActor(actor RequestActor, permission PermissionCode) ResourceGroupAccessFilter {
	filter := ResourceGroupAccessFilter{ActorID: actor.ID}
	if actor.EffectiveAccess == nil {
		return filter
	}
	grantedRoles := map[int64]struct{}{}
	for _, source := range actor.EffectiveAccess.Sources {
		if source.Permission == permission {
			grantedRoles[source.RoleID] = struct{}{}
		}
	}
	departmentIDs := map[int64]struct{}{}
	teamIDs := map[int64]struct{}{}
	for _, assignment := range actor.EffectiveAccess.Assignments {
		if _, ok := grantedRoles[assignment.RoleID]; !ok {
			continue
		}
		switch assignment.ScopeMode {
		case AccessScopeGlobal:
			filter.Global = true
		case AccessScopeSelf:
			filter.Self = true
		case AccessScopeOwnDepartment:
			if actor.DepartmentID != nil {
				departmentIDs[*actor.DepartmentID] = struct{}{}
			}
		case AccessScopeOwnTeam:
			if actor.TeamID != nil {
				teamIDs[*actor.TeamID] = struct{}{}
			}
		case AccessScopeSelectedOrg:
			for _, subject := range assignment.Subjects {
				switch subject.SubjectType {
				case AccessSubjectDepartment:
					departmentIDs[subject.SubjectID] = struct{}{}
				case AccessSubjectTeam:
					teamIDs[subject.SubjectID] = struct{}{}
				}
			}
		}
	}
	for id := range departmentIDs {
		filter.DepartmentIDs = append(filter.DepartmentIDs, id)
	}
	for id := range teamIDs {
		filter.TeamIDs = append(filter.TeamIDs, id)
	}
	sort.Slice(filter.DepartmentIDs, func(i, j int) bool { return filter.DepartmentIDs[i] < filter.DepartmentIDs[j] })
	sort.Slice(filter.TeamIDs, func(i, j int) bool { return filter.TeamIDs[i] < filter.TeamIDs[j] })
	return filter
}

type ReplaceAccessAssignmentsRequest struct {
	ExpectedPolicyRevision int64              `json:"expected_policy_revision"`
	Reason                 string             `json:"reason"`
	Assignments            []AccessAssignment `json:"assignments"`
}

type AccessOrgPolicy struct {
	ID          int64             `json:"id,omitempty"`
	SubjectType AccessSubjectType `json:"subject_type"`
	SubjectID   int64             `json:"subject_id"`
	RoleID      int64             `json:"role_id"`
	ScopeMode   AccessScopeMode   `json:"scope_mode"`
	Enabled     bool              `json:"enabled"`
	Version     int64             `json:"version"`
	Reason      string            `json:"reason"`
}

type AccessPolicyEvent struct {
	ID             int64       `json:"id"`
	PolicyRevision int64       `json:"policy_revision"`
	ActorID        int64       `json:"actor_id"`
	Action         string      `json:"action"`
	TargetType     string      `json:"target_type"`
	TargetID       string      `json:"target_id"`
	Reason         string      `json:"reason"`
	Before         interface{} `json:"before,omitempty"`
	After          interface{} `json:"after,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
}
