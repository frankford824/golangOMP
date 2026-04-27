//go:build integration

package notification

import (
	"testing"

	"workflow/domain"
)

// SA-C-I7 — POST /v1/me/notifications/read-all
// Asserts: current actor's unread notifications are flipped to read while
// another user's unread notifications remain untouched.
func TestSACI7_ReadAllNotifications_FlipsAllUnreadInScope(t *testing.T) {
	db, svc := sacNotificationOpenServiceAndDB(t)
	ownerID := int64(40015)
	otherID := int64(40016)
	t.Cleanup(func() { sacNotificationCleanupSAC(t, db, ownerID, otherID) })
	sacNotificationCleanupSAC(t, db, ownerID, otherID)

	sacNotificationInsertUser(t, db, ownerID)
	sacNotificationInsertUser(t, db, otherID)
	sacSeedNotification(t, db, ownerID, domain.NotificationTypeTaskAssignedToMe, false, 1)
	sacSeedNotification(t, db, ownerID, domain.NotificationTypeTaskRejected, false, 2)
	sacSeedNotification(t, db, ownerID, domain.NotificationTypeTaskCancelled, true, 3)
	sacSeedNotification(t, db, otherID, domain.NotificationTypeTaskAssignedToMe, false, 1)

	ctx, cancel := sacWithNotificationTimeout()
	defer cancel()

	if appErr := svc.MarkAllRead(ctx, sacNotificationActor(ownerID)); appErr != nil {
		t.Fatalf("MarkAllRead appErr=%+v", appErr)
	}
	ownerUnread, appErr := svc.UnreadCount(ctx, sacNotificationActor(ownerID))
	if appErr != nil {
		t.Fatalf("owner UnreadCount appErr=%+v", appErr)
	}
	otherUnread, appErr := svc.UnreadCount(ctx, sacNotificationActor(otherID))
	if appErr != nil {
		t.Fatalf("other UnreadCount appErr=%+v", appErr)
	}
	if ownerUnread != 0 || otherUnread != 1 {
		t.Fatalf("unread counts owner=%d other=%d want 0/1", ownerUnread, otherUnread)
	}
}
