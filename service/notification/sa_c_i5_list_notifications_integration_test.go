//go:build integration

package notification

import (
	"testing"

	"workflow/domain"
)

// SA-C-I5 — GET /v1/me/notifications
// Asserts: List is actor scoped, supports unread filtering, and cursor
// pagination stays inside the current user's notifications.
func TestSACI5_ListNotifications_ReturnsOnlyOwnUserNotifications(t *testing.T) {
	db, svc := sacNotificationOpenServiceAndDB(t)
	ownerID := int64(40011)
	otherID := int64(40012)
	t.Cleanup(func() { sacNotificationCleanupSAC(t, db, ownerID, otherID) })
	sacNotificationCleanupSAC(t, db, ownerID, otherID)

	sacNotificationInsertUser(t, db, ownerID)
	sacNotificationInsertUser(t, db, otherID)
	sacSeedNotification(t, db, ownerID, domain.NotificationTypeTaskAssignedToMe, false, 1)
	sacSeedNotification(t, db, ownerID, domain.NotificationTypeTaskRejected, false, 2)
	sacSeedNotification(t, db, ownerID, domain.NotificationTypeTaskAssignedToMe, true, 3)
	sacSeedNotification(t, db, otherID, domain.NotificationTypeTaskAssignedToMe, false, 0)

	ctx, cancel := sacWithNotificationTimeout()
	defer cancel()

	unread := false
	first, cursor, appErr := svc.List(ctx, sacNotificationActor(ownerID), ListFilter{IsRead: &unread, Limit: 1})
	if appErr != nil {
		t.Fatalf("list notifications page1 appErr=%+v", appErr)
	}
	if len(first) != 1 || cursor == "" {
		t.Fatalf("list notifications page1 len=%d cursor=%q want len=1 cursor", len(first), cursor)
	}
	if first[0].UserID != ownerID || first[0].IsRead {
		t.Fatalf("page1 notification = %+v want owner unread", first[0])
	}

	second, next, appErr := svc.List(ctx, sacNotificationActor(ownerID), ListFilter{IsRead: &unread, Limit: 1, Cursor: cursor})
	if appErr != nil {
		t.Fatalf("list notifications page2 appErr=%+v", appErr)
	}
	if len(second) != 1 || next != "" {
		t.Fatalf("list notifications page2 len=%d next=%q want len=1 no next", len(second), next)
	}
	if second[0].UserID != ownerID || second[0].IsRead {
		t.Fatalf("page2 notification = %+v want owner unread", second[0])
	}
}
