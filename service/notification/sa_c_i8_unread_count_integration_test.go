//go:build integration

package notification

import (
	"testing"

	"workflow/domain"
)

// SA-C-I8 — GET /v1/me/notifications/unread-count
// Asserts: unread count returns the current actor's unread total and drops to
// zero after read-all.
func TestSACI8_UnreadCount_ZeroAfterReadAll(t *testing.T) {
	db, svc := sacNotificationOpenServiceAndDB(t)
	ownerID := int64(40017)
	t.Cleanup(func() { sacNotificationCleanupSAC(t, db, ownerID) })
	sacNotificationCleanupSAC(t, db, ownerID)

	sacNotificationInsertUser(t, db, ownerID)
	sacSeedNotification(t, db, ownerID, domain.NotificationTypeTaskAssignedToMe, false, 1)
	sacSeedNotification(t, db, ownerID, domain.NotificationTypeTaskRejected, false, 2)
	sacSeedNotification(t, db, ownerID, domain.NotificationTypeTaskCancelled, true, 3)

	ctx, cancel := sacWithNotificationTimeout()
	defer cancel()

	before, appErr := svc.UnreadCount(ctx, sacNotificationActor(ownerID))
	if appErr != nil {
		t.Fatalf("UnreadCount before appErr=%+v", appErr)
	}
	if before != 2 {
		t.Fatalf("UnreadCount before=%d want 2", before)
	}
	if appErr := svc.MarkAllRead(ctx, sacNotificationActor(ownerID)); appErr != nil {
		t.Fatalf("MarkAllRead appErr=%+v", appErr)
	}
	after, appErr := svc.UnreadCount(ctx, sacNotificationActor(ownerID))
	if appErr != nil {
		t.Fatalf("UnreadCount after appErr=%+v", appErr)
	}
	if after != 0 {
		t.Fatalf("UnreadCount after=%d want 0", after)
	}
}
