//go:build integration

package notification

import (
	"database/sql"
	"testing"

	"workflow/domain"
)

// SA-C-I6 — POST /v1/me/notifications/{id}/read
// Asserts: owner marks unread notification as read; non-owner receives
// notification_not_owner rather than a not-found response.
func TestSACI6_MarkNotificationRead_NonOwnerReturns403(t *testing.T) {
	db, svc := sacNotificationOpenServiceAndDB(t)
	ownerID := int64(40013)
	otherID := int64(40014)
	t.Cleanup(func() { sacNotificationCleanupSAC(t, db, ownerID, otherID) })
	sacNotificationCleanupSAC(t, db, ownerID, otherID)

	sacNotificationInsertUser(t, db, ownerID)
	sacNotificationInsertUser(t, db, otherID)
	ownID := sacSeedNotification(t, db, ownerID, domain.NotificationTypeTaskAssignedToMe, false, 1)
	otherNotificationID := sacSeedNotification(t, db, otherID, domain.NotificationTypeTaskAssignedToMe, false, 1)

	ctx, cancel := sacWithNotificationTimeout()
	defer cancel()

	if appErr := svc.MarkRead(ctx, sacNotificationActor(ownerID), ownID); appErr != nil {
		t.Fatalf("owner MarkRead appErr=%+v", appErr)
	}
	var isRead bool
	var readAt sql.NullTime
	if err := db.QueryRowContext(ctx, `SELECT is_read, read_at FROM notifications WHERE id = ?`, ownID).Scan(&isRead, &readAt); err != nil {
		t.Fatalf("select read marker: %v", err)
	}
	if !isRead || !readAt.Valid {
		t.Fatalf("read marker is_read=%v read_at.Valid=%v want true/true", isRead, readAt.Valid)
	}

	appErr := svc.MarkRead(ctx, sacNotificationActor(ownerID), otherNotificationID)
	if appErr == nil || appErr.Code != CodeNotificationNotOwner {
		t.Fatalf("non-owner MarkRead appErr=%+v want code=%q", appErr, CodeNotificationNotOwner)
	}
}
