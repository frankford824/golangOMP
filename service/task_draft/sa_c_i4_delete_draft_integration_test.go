//go:build integration

package task_draft

import (
	"database/sql"
	"testing"

	"workflow/domain"
)

// SA-C-I4 — DELETE /v1/task-drafts/{draft_id}
// Asserts: non-owner delete is denied with draft_not_owner, owner delete removes
// the row, and a repeated delete is currently fixed to 404/task_draft_not_found.
func TestSACI4_DeleteTaskDraft_NonOwnerReturns403_Idempotent(t *testing.T) {
	db, svc := sacOpenServiceAndDB(t)
	ownerID := int64(40007)
	intruderID := int64(40008)
	t.Cleanup(func() { sacCleanupSAC(t, db, ownerID, intruderID) })
	sacCleanupSAC(t, db, ownerID, intruderID)

	sacInsertUser(t, db, ownerID, "", "", nil)
	sacInsertUser(t, db, intruderID, "", "", nil)
	draftID := sacSeedTaskDraftFor(t, db, ownerID, "customization", sacRawJSON(t, map[string]interface{}{
		"task_type": "customization",
		"title":     "SA-C I4 draft",
	}))

	ctx, cancel := sacWithTimeout(t, nil)
	defer cancel()

	appErr := svc.Delete(ctx, sacActor(intruderID, nil), draftID)
	if appErr == nil || appErr.Code != CodeDraftNotOwner {
		t.Fatalf("intruder Delete appErr=%+v want code=%q", appErr, CodeDraftNotOwner)
	}

	if appErr = svc.Delete(ctx, sacActor(ownerID, nil), draftID); appErr != nil {
		t.Fatalf("owner Delete appErr=%+v", appErr)
	}
	var dbID int64
	err := db.QueryRowContext(ctx, `SELECT id FROM task_drafts WHERE id = ?`, draftID).Scan(&dbID)
	if err != sql.ErrNoRows {
		t.Fatalf("post-delete row scan err=%v id=%d want sql.ErrNoRows", err, dbID)
	}

	appErr = svc.Delete(ctx, sacActor(ownerID, nil), draftID)
	if appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("second Delete appErr=%+v want code=%q", appErr, domain.ErrCodeNotFound)
	}
}
