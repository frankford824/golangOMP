//go:build integration

package task_draft

import "testing"

// SA-C-I2 — GET /v1/me/task-drafts
// Asserts: List is owner-scoped and cursor pagination returns the remaining
// owner rows without leaking another user's drafts.
func TestSACI2_ListMyTaskDrafts_ReturnsOwnerScopedRecords(t *testing.T) {
	db, svc := sacOpenServiceAndDB(t)
	ownerID := int64(40003)
	otherID := int64(40004)
	t.Cleanup(func() { sacCleanupSAC(t, db, ownerID, otherID) })
	sacCleanupSAC(t, db, ownerID, otherID)

	sacInsertUser(t, db, ownerID, "", "", nil)
	sacInsertUser(t, db, otherID, "", "", nil)
	for i := 0; i < 3; i++ {
		sacSeedTaskDraftFor(t, db, ownerID, "customization", sacRawJSON(t, map[string]interface{}{
			"task_type": "customization",
			"seq":       i,
		}))
	}
	sacSeedTaskDraftFor(t, db, otherID, "customization", sacRawJSON(t, map[string]interface{}{
		"task_type": "customization",
		"seq":       "other",
	}))

	ctx, cancel := sacWithTimeout(t, nil)
	defer cancel()

	first, cursor, appErr := svc.List(ctx, sacActor(ownerID, nil), ListDraftFilter{Limit: 2})
	if appErr != nil {
		t.Fatalf("owner list page1 appErr=%+v", appErr)
	}
	if len(first) != 2 || cursor == "" {
		t.Fatalf("owner list page1 len=%d cursor=%q want len=2 cursor", len(first), cursor)
	}
	for _, item := range first {
		if item.OwnerUserID != ownerID {
			t.Fatalf("owner list leaked draft %+v", item)
		}
	}

	second, next, appErr := svc.List(ctx, sacActor(ownerID, nil), ListDraftFilter{Limit: 2, Cursor: cursor})
	if appErr != nil {
		t.Fatalf("owner list page2 appErr=%+v", appErr)
	}
	if len(second) != 1 || next != "" {
		t.Fatalf("owner list page2 len=%d next=%q want len=1 no next", len(second), next)
	}
	if second[0].OwnerUserID != ownerID {
		t.Fatalf("owner list page2 leaked draft %+v", second[0])
	}

	other, _, appErr := svc.List(ctx, sacActor(otherID, nil), ListDraftFilter{TaskType: "purchase", Limit: 20})
	if appErr != nil {
		t.Fatalf("other filtered list appErr=%+v", appErr)
	}
	if len(other) != 0 {
		t.Fatalf("other user purchase list len=%d want 0", len(other))
	}
}
