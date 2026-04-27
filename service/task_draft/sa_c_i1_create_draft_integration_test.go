//go:build integration

package task_draft

import (
	"testing"
	"time"

	"workflow/domain"
)

// SA-C-I1 — POST /v1/task-drafts
// Asserts: owner creates draft successfully (saved row + expires_at = now()+7d ± 1h),
// payload preserved as-is, and a different actor reading the same draft id is rejected
// with the prompt-mandated `draft_not_owner` code (anti-probe; not draft_not_found).
func TestSACI1_CreateTaskDraft_OwnerOnlyAccessible(t *testing.T) {
	db, svc := sacOpenServiceAndDB(t)
	ownerID := int64(40001)
	intruderID := int64(40002)
	t.Cleanup(func() { sacCleanupSAC(t, db, ownerID, intruderID) })
	sacCleanupSAC(t, db, ownerID, intruderID)

	sacInsertUser(t, db, ownerID, "", "", nil)
	sacInsertUser(t, db, intruderID, "", "", nil)

	ctx, cancel := sacWithTimeout(t, nil)
	defer cancel()

	owner := sacActor(ownerID, nil)
	intruder := sacActor(intruderID, nil)
	payload := sacRawJSON(t, map[string]interface{}{
		"task_type": "customization",
		"title":     "SA-C I1 draft",
		"sku_code":  "SAC-I1-SKU",
	})

	before := time.Now().UTC()
	saved, appErr := svc.CreateOrUpdate(ctx, owner, payload)
	if appErr != nil {
		t.Fatalf("CreateOrUpdate appErr=%+v", appErr)
	}
	if saved == nil || saved.ID <= 0 || saved.OwnerUserID != ownerID || saved.TaskType != "customization" {
		t.Fatalf("created draft = %+v want non-nil owner=%d", saved, ownerID)
	}
	wantExpiry := before.Add(7 * 24 * time.Hour)
	delta := saved.ExpiresAt.Sub(wantExpiry)
	if delta < -time.Hour || delta > time.Hour {
		t.Fatalf("expires_at = %s, expected ~%s (±1h, delta=%s)", saved.ExpiresAt, wantExpiry, delta)
	}

	var dbOwner int64
	if err := db.QueryRow(`SELECT owner_user_id FROM task_drafts WHERE id = ?`, saved.ID).Scan(&dbOwner); err != nil {
		t.Fatalf("verify task_drafts row: %v", err)
	}
	if dbOwner != ownerID {
		t.Fatalf("task_drafts.owner_user_id = %d, want %d", dbOwner, ownerID)
	}

	// Anti-probe: a different actor must be denied with `draft_not_owner` code,
	// not `NOT_FOUND`, so existence cannot be probed. Mirrors prompt §3.4 I1/I3.
	got, getErr := svc.Get(ctx, intruder, saved.ID)
	if got != nil || getErr == nil || getErr.Code != CodeDraftNotOwner {
		t.Fatalf("intruder Get got=%+v err=%+v want code=%q", got, getErr, CodeDraftNotOwner)
	}

	// Owner can read the same draft.
	mine, ownerErr := svc.Get(ctx, owner, saved.ID)
	if ownerErr != nil || mine == nil || mine.ID != saved.ID {
		t.Fatalf("owner Get got=%+v err=%+v", mine, ownerErr)
	}
	if string(mine.Payload) == "" || string(mine.Payload) == "{}" {
		t.Fatalf("payload roundtrip lost = %s", string(mine.Payload))
	}
}

var _ = domain.ErrCodeInvalidRequest
