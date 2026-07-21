package service

import (
	"context"
	"time"

	"workflow/domain"
)

func float64Ptr(value float64) *float64 { return &value }

func uploadRequestInt64Ptr(value int64) *int64 { return &value }

func authzInt64Ptr(value int64) *int64 { return &value }

func timePtr() *time.Time {
	value := time.Now()
	return &value
}

func identityGlobalAccessManageContext(userID int64) context.Context {
	return identityAccessManageContext(userID, domain.AccessScopeGlobal, nil)
}

func identityDepartmentAccessManageContext(userID, departmentID int64) context.Context {
	return identityAccessManageContext(userID, domain.AccessScopeSelectedOrg, []domain.AccessScopeSubject{{
		SubjectType: domain.AccessSubjectDepartment,
		SubjectID:   departmentID,
	}})
}

func identityAccessManageContext(userID int64, scope domain.AccessScopeMode, subjects []domain.AccessScopeSubject) context.Context {
	const roleID int64 = 1
	effective := &domain.EffectiveAccess{
		UserID:      userID,
		Permissions: []domain.PermissionCode{domain.PermissionAccessManage},
		Assignments: []domain.AccessAssignment{{
			ID:        roleID,
			UserID:    userID,
			RoleID:    roleID,
			RoleCode:  "access_manager",
			ScopeMode: scope,
			Subjects:  subjects,
		}},
		Sources: []domain.EffectiveAccessNote{{
			Permission: domain.PermissionAccessManage,
			RoleID:     roleID,
			RoleCode:   "access_manager",
			SourceType: "direct",
			ScopeMode:  scope,
		}},
	}
	return domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:              userID,
		Permissions:     effective.Permissions,
		EffectiveAccess: effective,
	})
}
