//go:build integration

package task_draft

import "testing"

// SA-C-I3 — GET /v1/task-drafts/{draft_id}
// Asserts: owner can read a draft; a different actor receives draft_not_owner
// instead of a not-found response.
func TestSACI3_GetTaskDraft_NonOwnerReturns403DraftNotOwner(t *testing.T) {
	db, svc := sacOpenServiceAndDB(t)
	ownerID := int64(40005)
	intruderID := int64(40006)
	t.Cleanup(func() { sacCleanupSAC(t, db, ownerID, intruderID) })
	sacCleanupSAC(t, db, ownerID, intruderID)

	sacInsertUser(t, db, ownerID, "", "", nil)
	sacInsertUser(t, db, intruderID, "", "", nil)
	draftID := sacSeedTaskDraftFor(t, db, ownerID, "customization", sacRawJSON(t, map[string]interface{}{
		"task_type": "customization",
		"title":     "SA-C I3 draft",
	}))

	ctx, cancel := sacWithTimeout(t, nil)
	defer cancel()

	got, appErr := svc.Get(ctx, sacActor(ownerID, nil), draftID)
	if appErr != nil || got == nil || got.ID != draftID || got.OwnerUserID != ownerID {
		t.Fatalf("owner Get got=%+v appErr=%+v", got, appErr)
	}
	got, appErr = svc.Get(ctx, sacActor(intruderID, nil), draftID)
	if got != nil || appErr == nil || appErr.Code != CodeDraftNotOwner {
		t.Fatalf("intruder Get got=%+v appErr=%+v want code=%q", got, appErr, CodeDraftNotOwner)
	}
}
