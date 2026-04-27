//go:build integration

package org_move_request

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestSABI8_ApproveOrgMoveRequest_UpdatesDepartmentAndClearsTeam(t *testing.T) {
	db, svc := saBOpenOrgMoveTestDB(t)
	userID := int64(30081)
	requesterID := int64(30082)
	superID := int64(30083)
	var requestIDs []int64
	saBCleanupOrgMove(t, db, nil, userID, requesterID, superID)
	defer func() { saBCleanupOrgMove(t, db, requestIDs, userID, requesterID, superID) }()

	sourceID := saBDepartmentID(t, db, string(domain.DepartmentOperations))
	targetID := saBDepartmentID(t, db, string(domain.DepartmentCloudWarehouse))
	saBInsertOrgMoveUser(t, db, userID, "sab_i8_target", string(domain.DepartmentOperations), "淘系一组", []domain.Role{domain.RoleMember}, nil)
	saBInsertOrgMoveUser(t, db, requesterID, "sab_i8_requester", string(domain.DepartmentOperations), "淘系一组", []domain.Role{domain.RoleDeptAdmin}, []string{string(domain.DepartmentOperations)})
	saBInsertOrgMoveUser(t, db, superID, "sab_i8_super", string(domain.DepartmentHR), "人事管理组", []domain.Role{domain.RoleSuperAdmin}, nil)

	item, appErr := svc.Create(context.Background(), saBActor(requesterID, "sab_i8_requester", []domain.Role{domain.RoleDeptAdmin}, string(domain.DepartmentOperations), "淘系一组", []string{string(domain.DepartmentOperations)}), sourceID, CreateParams{
		UserID:             userID,
		TargetDepartmentID: &targetID,
		Reason:             "SA-B.1 I8 create",
	})
	if appErr != nil {
		t.Fatalf("Create appErr=%v", appErr)
	}
	requestIDs = append(requestIDs, item.ID)

	superActor := saBActor(superID, "sab_i8_super", []domain.Role{domain.RoleSuperAdmin}, string(domain.DepartmentHR), "人事管理组", nil)
	if appErr := svc.Approve(context.Background(), superActor, item.ID); appErr != nil {
		t.Fatalf("Approve appErr=%v details=%+v requestID=%d", appErr, appErr.Details, item.ID)
	}
	var department, team string
	if err := db.QueryRow(`SELECT department, team FROM users WHERE id = ?`, userID).Scan(&department, &team); err != nil {
		t.Fatalf("select approved user org: %v", err)
	}
	repeatErr := svc.Approve(context.Background(), superActor, item.ID)
	if department != string(domain.DepartmentCloudWarehouse) || team != "" ||
		saBPermissionLogCount(t, db, "user_department_changed_by_admin", userID) == 0 ||
		repeatErr == nil || repeatErr.Code != domain.ErrCodeConflict || saBAppDenyCode(repeatErr) != "org_move_request_already_decided" {
		t.Fatalf("department=%q team=%q changed_logs=%d repeatErr=%v", department, team, saBPermissionLogCount(t, db, "user_department_changed_by_admin", userID), repeatErr)
	}
}
