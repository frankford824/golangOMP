package notification

import (
	"context"
	"encoding/json"
	"testing"

	"workflow/domain"
)

func TestCreateNotificationEnqueuesWebPushPerSubscription(t *testing.T) {
	keyHash := PublicKeyHash("public-key")
	repo := &broadcastNotificationRepo{
		pref: domain.NotificationPreference{UserID: 7, WebPushEnabled: true, VAPIDKeyHash: keyHash},
		subs: []domain.WebPushSubscription{
			{ID: 101, UserID: 7, Status: domain.WebPushSubscriptionActive, VAPIDKeyHash: keyHash},
			{ID: 102, UserID: 7, Status: domain.WebPushSubscriptionActive, VAPIDKeyHash: keyHash},
			{ID: 103, UserID: 8, Status: domain.WebPushSubscriptionActive, VAPIDKeyHash: keyHash},
		},
	}
	svc := NewService(repo, nil, nil, nil,
		WithTxRunner(broadcastTxRunner{}),
		WithWebPushConfig(WebPushConfig{Enabled: true, PublicKey: "public-key", PrivateKey: "private-key"}),
	)

	n, err := svc.CreateNotification(context.Background(), broadcastTx{}, 7, domain.NotificationTypeTaskSKUSyncFailed, mustRaw(map[string]interface{}{
		"task_id":      99,
		"task_no":      "RW-99",
		"failed_count": 2,
		"failed_items": []map[string]interface{}{
			{"sku_code": "SKU-A", "error": "bad"},
			{"sku_code": "SKU-B", "error": "bad"},
		},
	}))
	if err != nil {
		t.Fatalf("CreateNotification error = %v", err)
	}
	if n == nil || n.ID == 0 {
		t.Fatalf("notification = %#v, want persisted id", n)
	}
	if len(repo.deliveries) != 2 {
		t.Fatalf("deliveries = %d, want 2", len(repo.deliveries))
	}
	for _, delivery := range repo.deliveries {
		if delivery.NotificationID != n.ID || delivery.UserID != 7 {
			t.Fatalf("delivery = %#v, want notification/user", delivery)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(delivery.Payload, &payload); err != nil {
			t.Fatalf("delivery payload json error = %v", err)
		}
		if _, ok := payload["failed_items"]; ok {
			t.Fatalf("web push payload leaked failed_items: %#v", payload)
		}
		if payload["notification_id"] == nil || payload["title"] == nil || payload["body"] == nil {
			t.Fatalf("web push payload missing compact fields: %#v", payload)
		}
	}
}

func TestCreateDedupedNotificationDedupesAndKeepsOutboxPerCreatedNotification(t *testing.T) {
	keyHash := PublicKeyHash("public-key")
	repo := &broadcastNotificationRepo{
		pref: domain.NotificationPreference{UserID: 7, WebPushEnabled: true, VAPIDKeyHash: keyHash},
		subs: []domain.WebPushSubscription{
			{ID: 101, UserID: 7, Status: domain.WebPushSubscriptionActive, VAPIDKeyHash: keyHash},
		},
	}
	svc := NewService(repo, nil, nil, nil,
		WithTxRunner(broadcastTxRunner{}),
		WithWebPushConfig(WebPushConfig{Enabled: true, PublicKey: "public-key", PrivateKey: "private-key"}),
	)

	payload := mustRaw(map[string]interface{}{"task_id": 99, "failed_count": 1, "failed_items": []map[string]interface{}{{"sku_code": "SKU-A", "error": "bad"}}})
	first, created, err := svc.CreateDedupedNotification(context.Background(), 7, domain.NotificationTypeTaskSKUSyncFailed, payload, "scope-1", "key-1")
	if err != nil || !created || first == nil {
		t.Fatalf("first CreateDedupedNotification = (%#v, %t, %v), want created", first, created, err)
	}
	second, created, err := svc.CreateDedupedNotification(context.Background(), 7, domain.NotificationTypeTaskSKUSyncFailed, payload, "scope-1", "key-1")
	if err != nil || created || second != nil {
		t.Fatalf("second CreateDedupedNotification = (%#v, %t, %v), want deduped", second, created, err)
	}
	changedPayload := mustRaw(map[string]interface{}{"task_id": 99, "failed_count": 1, "failed_items": []map[string]interface{}{{"sku_code": "SKU-A", "error": "changed"}}})
	third, created, err := svc.CreateDedupedNotification(context.Background(), 7, domain.NotificationTypeTaskSKUSyncFailed, changedPayload, "scope-1", "key-2")
	if err != nil || !created || third == nil {
		t.Fatalf("third CreateDedupedNotification = (%#v, %t, %v), want created for changed key", third, created, err)
	}
	if len(repo.created) != 2 {
		t.Fatalf("created notifications = %d, want 2", len(repo.created))
	}
	if len(repo.deliveries) != 2 {
		t.Fatalf("web push deliveries = %d, want 2", len(repo.deliveries))
	}
}

func TestCreateDedupedNotificationRollsBackDedupeClaimWhenCreateFails(t *testing.T) {
	repo := &broadcastNotificationRepo{failCreate: true}
	svc := NewService(repo, nil, nil, nil, WithTxRunner(broadcastRollbackTxRunner{repo: repo}))
	payload := mustRaw(map[string]interface{}{"title": "失败"})

	if _, created, err := svc.CreateDedupedNotification(context.Background(), 7, domain.NotificationTypeSystemBroadcast, payload, "scope-rollback", "key-rollback"); err == nil || !created {
		t.Fatalf("failed CreateDedupedNotification = created %t err %v, want create error after claim", created, err)
	}
	if len(repo.dedupeClaims) != 0 {
		t.Fatalf("dedupe claims after rollback = %d, want 0", len(repo.dedupeClaims))
	}
	repo.failCreate = false
	n, created, err := svc.CreateDedupedNotification(context.Background(), 7, domain.NotificationTypeSystemBroadcast, payload, "scope-rollback", "key-rollback")
	if err != nil || !created || n == nil {
		t.Fatalf("retry CreateDedupedNotification = (%#v, %t, %v), want created after rollback", n, created, err)
	}
}

func TestSKUSyncDedupeKeyStableForSortedFailures(t *testing.T) {
	a := domain.SKUSyncFailureNotificationRequest{
		Source:         domain.SKUSyncFailureSourceTaskFiling,
		TaskID:         9,
		ERPSyncVersion: 3,
		FailureItems: []domain.SKUSyncFailureItem{
			{SKUCode: "SKU-B", Error: "  remote error  "},
			{SKUCode: "SKU-A", Error: "remote error"},
		},
	}
	b := domain.SKUSyncFailureNotificationRequest{
		Source:         domain.SKUSyncFailureSourceTaskFiling,
		TaskID:         9,
		ERPSyncVersion: 3,
		FailureItems: []domain.SKUSyncFailureItem{
			{SKUCode: "SKU-A", Error: "remote error"},
			{SKUCode: "SKU-B", Error: "remote error"},
		},
	}
	if skuSyncDedupeKey(a) != skuSyncDedupeKey(b) {
		t.Fatalf("dedupe key should be stable across item ordering")
	}
	b.FailureItems[1].Error = "different"
	if skuSyncDedupeKey(a) == skuSyncDedupeKey(b) {
		t.Fatalf("dedupe key should change when failure content changes")
	}
}

func TestGetPreferencesMarksOldVAPIDSubscriptionsStale(t *testing.T) {
	currentKeyHash := PublicKeyHash("new-public-key")
	repo := &broadcastNotificationRepo{
		pref: domain.NotificationPreference{UserID: 7, WebPushEnabled: true, VAPIDKeyHash: "old-key-hash"},
	}
	svc := NewService(repo, nil, nil, nil,
		WithWebPushConfig(WebPushConfig{Enabled: true, PublicKey: "new-public-key", PrivateKey: "private-key"}),
	)

	view, appErr := svc.GetPreferences(context.Background(), domain.RequestActor{ID: 7})
	if appErr != nil {
		t.Fatalf("GetPreferences appErr = %+v", appErr)
	}
	if view == nil || view.VAPIDKeyHash != "old-key-hash" {
		t.Fatalf("view VAPIDKeyHash = %#v, want old-key-hash", view)
	}
	if repo.staleUserID != 7 || repo.staleKeyHash != currentKeyHash {
		t.Fatalf("stale marker = (%d, %q), want (7, %q)", repo.staleUserID, repo.staleKeyHash, currentKeyHash)
	}
}
