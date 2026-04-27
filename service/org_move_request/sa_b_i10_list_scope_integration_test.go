//go:build integration

package org_move_request

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestSABI10_ListOrgMoveRequests_DeptAdminSeesOwnDepartmentOnly(t *testing.T) {
	db, svc := saBOpenOrgMoveTestDB(t)
	ownUserID := int64(30101)
	otherUserID := int64(30102)
	deptAdminID := int64(30103)
	superID := int64(30104)
	var requestIDs []int64
	saBCleanupOrgMove(t, db, nil, ownUserID, otherUserID, deptAdminID, superID)
	defer func() { saBCleanupOrgMove(t, db, requestIDs, ownUserID, otherUserID, deptAdminID, superID) }()

	ownSourceID := saBDepartmentID(t, db, string(domain.DepartmentOperations))
	otherSourceID := saBDepartmentID(t, db, string(domain.DepartmentHR))
	targetID := saBDepartmentID(t, db, string(domain.DepartmentCloudWarehouse))
	saBInsertOrgMoveUser(t, db, ownUserID, "sab_i10_own_target", string(domain.DepartmentOperations), "淘系一组", []domain.Role{domain.RoleMember}, nil)
	saBInsertOrgMoveUser(t, db, otherUserID, "sab_i10_other_target", string(domain.DepartmentHR), "人事管理组", []domain.Role{domain.RoleMember}, nil)
	saBInsertOrgMoveUser(t, db, deptAdminID, "sab_i10_dept_admin", string(domain.DepartmentOperations), "淘系一组", []domain.Role{domain.RoleDeptAdmin}, []string{string(domain.DepartmentOperations)})
	saBInsertOrgMoveUser(t, db, superID, "sab_i10_super", string(domain.DepartmentHR), "人事管理组", []domain.Role{domain.RoleSuperAdmin}, nil)

	deptActor := saBActor(deptAdminID, "sab_i10_dept_admin", []domain.Role{domain.RoleDeptAdmin}, string(domain.DepartmentOperations), "淘系一组", []string{string(domain.DepartmentOperations)})
	superActor := saBActor(superID, "sab_i10_super", []domain.Role{domain.RoleSuperAdmin}, string(domain.DepartmentHR), "人事管理组", nil)
	own, appErr := svc.Create(context.Background(), deptActor, ownSourceID, CreateParams{UserID: ownUserID, TargetDepartmentID: &targetID, Reason: "SA-B.1 I10 own"})
	if appErr != nil {
		t.Fatalf("Create own appErr=%v", appErr)
	}
	requestIDs = append(requestIDs, own.ID)
	other, appErr := svc.Create(context.Background(), superActor, otherSourceID, CreateParams{UserID: otherUserID, TargetDepartmentID: &targetID, Reason: "SA-B.1 I10 other"})
	if appErr != nil {
		t.Fatalf("Create other appErr=%v", appErr)
	}
	requestIDs = append(requestIDs, other.ID)

	state := domain.OrgMoveRequestStatePendingSuperAdminConfirm
	items, _, appErr := svc.List(context.Background(), deptActor, ListFilter{State: &state, Page: 1, PageSize: 20})
	if appErr != nil {
		t.Fatalf("List appErr=%v details=%+v requestIDs=%v", appErr, appErr.Details, requestIDs)
	}
	seenOwn := false
	seenOther := false
	for _, item := range items {
		if item.SourceDepartment != string(domain.DepartmentOperations) {
			t.Fatalf("DeptAdmin saw source department %q outside own department", item.SourceDepartment)
		}
		if item.ID == own.ID {
			seenOwn = true
		}
		if item.ID == other.ID {
			seenOther = true
		}
	}
	if !seenOwn || seenOther {
		t.Fatalf("DeptAdmin list seenOwn=%v seenOther=%v items=%+v", seenOwn, seenOther, items)
	}
}
