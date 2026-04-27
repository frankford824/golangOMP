//go:build integration

package org_move_request

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestSABI9_RejectOrgMoveRequest_PreservesSourceDepartment(t *testing.T) {
	db, svc := saBOpenOrgMoveTestDB(t)
	userID := int64(30091)
	requesterID := int64(30092)
	superID := int64(30093)
	memberID := int64(30094)
	var requestIDs []int64
	saBCleanupOrgMove(t, db, nil, userID, requesterID, superID, memberID)
	defer func() { saBCleanupOrgMove(t, db, requestIDs, userID, requesterID, superID, memberID) }()

	sourceID := saBDepartmentID(t, db, string(domain.DepartmentOperations))
	targetID := saBDepartmentID(t, db, string(domain.DepartmentCloudWarehouse))
	saBInsertOrgMoveUser(t, db, userID, "sab_i9_target", string(domain.DepartmentOperations), "淘系一组", []domain.Role{domain.RoleMember}, nil)
	saBInsertOrgMoveUser(t, db, requesterID, "sab_i9_requester", string(domain.DepartmentOperations), "淘系一组", []domain.Role{domain.RoleDeptAdmin}, []string{string(domain.DepartmentOperations)})
	saBInsertOrgMoveUser(t, db, superID, "sab_i9_super", string(domain.DepartmentHR), "人事管理组", []domain.Role{domain.RoleSuperAdmin}, nil)
	saBInsertOrgMoveUser(t, db, memberID, "sab_i9_member", string(domain.DepartmentOperations), "淘系一组", []domain.Role{domain.RoleMember}, nil)

	item, appErr := svc.Create(context.Background(), saBActor(requesterID, "sab_i9_requester", []domain.Role{domain.RoleDeptAdmin}, string(domain.DepartmentOperations), "淘系一组", []string{string(domain.DepartmentOperations)}), sourceID, CreateParams{
		UserID:             userID,
		TargetDepartmentID: &targetID,
		Reason:             "SA-B.1 I9 create",
	})
	if appErr != nil {
		t.Fatalf("Create appErr=%v", appErr)
	}
	requestIDs = append(requestIDs, item.ID)

	rejectReason := "SA-B.1 I9 reject"
	if appErr := svc.Reject(context.Background(), saBActor(superID, "sab_i9_super", []domain.Role{domain.RoleSuperAdmin}, string(domain.DepartmentHR), "人事管理组", nil), item.ID, rejectReason); appErr != nil {
		t.Fatalf("Reject appErr=%v details=%+v requestID=%d", appErr, appErr.Details, item.ID)
	}
	var state, reason, department string
	if err := db.QueryRow(`SELECT state, reason FROM org_move_requests WHERE id = ?`, item.ID).Scan(&state, &reason); err != nil {
		t.Fatalf("select rejected request: %v", err)
	}
	if err := db.QueryRow(`SELECT department FROM users WHERE id = ?`, userID).Scan(&department); err != nil {
		t.Fatalf("select rejected user department: %v", err)
	}
	nonSuperErr := svc.Reject(context.Background(), saBActor(memberID, "sab_i9_member", []domain.Role{domain.RoleMember}, string(domain.DepartmentOperations), "淘系一组", nil), item.ID, "not allowed")
	missingReasonErr := svc.Reject(context.Background(), saBActor(superID, "sab_i9_super", []domain.Role{domain.RoleSuperAdmin}, string(domain.DepartmentHR), "人事管理组", nil), item.ID, "")
	if state != string(domain.OrgMoveRequestStateRejected) || reason != rejectReason || department != string(domain.DepartmentOperations) ||
		nonSuperErr == nil || nonSuperErr.Code != domain.ErrCodePermissionDenied || saBAppDenyCode(nonSuperErr) != "module_action_role_denied" ||
		missingReasonErr == nil || missingReasonErr.Code != domain.ErrCodeReasonRequired || saBAppDenyCode(missingReasonErr) != "reason_required" {
		t.Fatalf("state=%q reason=%q department=%q nonSuperErr=%v missingReasonErr=%v", state, reason, department, nonSuperErr, missingReasonErr)
	}
}
