package domain

import (
	"encoding/json"
	"strings"
	"time"
)

type NotificationType string

const (
	NotificationTypeTaskAssignedToMe  NotificationType = "task_assigned_to_me"
	NotificationTypeTaskRejected      NotificationType = "task_rejected"
	NotificationTypeTaskPendingAudit  NotificationType = "task_pending_audit"
	NotificationTypeTaskClosed        NotificationType = "task_closed"
	NotificationTypeClaimConflict     NotificationType = "claim_conflict"
	NotificationTypePoolReassigned    NotificationType = "pool_reassigned"
	NotificationTypeTaskCancelled     NotificationType = "task_cancelled"
	NotificationTypeSystemBroadcast   NotificationType = "system_broadcast"
	NotificationTypeTaskSKUSyncFailed NotificationType = "task_sku_sync_failed"

	NotificationTypeAssetWorkbenchProfileIncomplete  NotificationType = "asset_workbench_profile_incomplete"
	NotificationTypeAssetWorkbenchSubmissionCreated  NotificationType = "asset_workbench_submission_created"
	NotificationTypeAssetWorkbenchQCUpdated          NotificationType = "asset_workbench_qc_updated"
	NotificationTypeAssetWorkbenchSettlementUpdated  NotificationType = "asset_workbench_settlement_updated"
	NotificationTypeAssetWorkbenchSupplementAccess   NotificationType = "asset_workbench_supplement_access"
	NotificationTypeAssetWorkbenchPreviewFailed      NotificationType = "asset_workbench_preview_failed"
	NotificationTypeAssetWorkbenchBatchJobCompleted  NotificationType = "asset_workbench_batch_job_completed"
	NotificationTypeAssetWorkbenchBatchJobFailed     NotificationType = "asset_workbench_batch_job_failed"
	NotificationTypeAssetWorkbenchNotificationPrefix                  = "asset_workbench_"
)

func (t NotificationType) Valid() bool {
	switch t {
	case NotificationTypeTaskAssignedToMe,
		NotificationTypeTaskRejected,
		NotificationTypeTaskPendingAudit,
		NotificationTypeTaskClosed,
		NotificationTypeClaimConflict,
		NotificationTypePoolReassigned,
		NotificationTypeTaskCancelled,
		NotificationTypeSystemBroadcast,
		NotificationTypeTaskSKUSyncFailed,
		NotificationTypeAssetWorkbenchProfileIncomplete,
		NotificationTypeAssetWorkbenchSubmissionCreated,
		NotificationTypeAssetWorkbenchQCUpdated,
		NotificationTypeAssetWorkbenchSettlementUpdated,
		NotificationTypeAssetWorkbenchSupplementAccess,
		NotificationTypeAssetWorkbenchPreviewFailed,
		NotificationTypeAssetWorkbenchBatchJobCompleted,
		NotificationTypeAssetWorkbenchBatchJobFailed:
		return true
	default:
		return false
	}
}

func (t NotificationType) IsAssetWorkbench() bool {
	return strings.HasPrefix(string(t), NotificationTypeAssetWorkbenchNotificationPrefix)
}

type Notification struct {
	ID               int64            `json:"id" db:"id"`
	UserID           int64            `json:"user_id,omitempty" db:"user_id"`
	NotificationType NotificationType `json:"notification_type" db:"notification_type"`
	Payload          json.RawMessage  `json:"payload" db:"payload"`
	IsRead           bool             `json:"is_read" db:"is_read"`
	ReadAt           *time.Time       `json:"read_at,omitempty" db:"read_at"`
	CreatedAt        time.Time        `json:"created_at" db:"created_at"`
}

type NotificationPayload map[string]interface{}

type WebPushSubscriptionStatus string

const (
	WebPushSubscriptionActive   WebPushSubscriptionStatus = "active"
	WebPushSubscriptionDisabled WebPushSubscriptionStatus = "disabled"
	WebPushSubscriptionStale    WebPushSubscriptionStatus = "stale"
)

type NotificationDeliveryStatus string

const (
	NotificationDeliveryPending NotificationDeliveryStatus = "pending"
	NotificationDeliverySending NotificationDeliveryStatus = "sending"
	NotificationDeliveryRetry   NotificationDeliveryStatus = "retry"
	NotificationDeliverySent    NotificationDeliveryStatus = "sent"
	NotificationDeliveryDead    NotificationDeliveryStatus = "dead"
)

type WebPushSubscription struct {
	ID             int64                     `json:"id" db:"id"`
	UserID         int64                     `json:"user_id" db:"user_id"`
	Endpoint       string                    `json:"endpoint" db:"endpoint"`
	EndpointHash   string                    `json:"endpoint_hash" db:"endpoint_hash"`
	P256DH         string                    `json:"p256dh" db:"p256dh"`
	Auth           string                    `json:"auth" db:"auth"`
	UserAgent      string                    `json:"user_agent,omitempty" db:"user_agent"`
	Platform       string                    `json:"platform,omitempty" db:"platform"`
	Status         WebPushSubscriptionStatus `json:"status" db:"status"`
	VAPIDKeyHash   string                    `json:"vapid_key_hash,omitempty" db:"vapid_key_hash"`
	LastSeenAt     time.Time                 `json:"last_seen_at" db:"last_seen_at"`
	DisabledAt     *time.Time                `json:"disabled_at,omitempty" db:"disabled_at"`
	DisabledReason string                    `json:"disabled_reason,omitempty" db:"disabled_reason"`
	CreatedAt      time.Time                 `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at" db:"updated_at"`
}

type NotificationPreference struct {
	UserID         int64      `json:"user_id" db:"user_id"`
	WebPushEnabled bool       `json:"web_push_enabled" db:"web_push_enabled"`
	LastTestSentAt *time.Time `json:"last_test_sent_at,omitempty" db:"last_test_sent_at"`
	VAPIDKeyHash   string     `json:"vapid_key_hash,omitempty" db:"vapid_key_hash"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

type NotificationDeliveryOutbox struct {
	ID                 int64                      `json:"id" db:"id"`
	NotificationID     int64                      `json:"notification_id" db:"notification_id"`
	SubscriptionID     int64                      `json:"subscription_id" db:"subscription_id"`
	UserID             int64                      `json:"user_id" db:"user_id"`
	Channel            string                     `json:"channel" db:"channel"`
	Payload            json.RawMessage            `json:"payload" db:"payload"`
	Status             NotificationDeliveryStatus `json:"status" db:"status"`
	AttemptCount       int                        `json:"attempt_count" db:"attempt_count"`
	NextAttemptAt      time.Time                  `json:"next_attempt_at" db:"next_attempt_at"`
	LeaseUntil         *time.Time                 `json:"lease_until,omitempty" db:"lease_until"`
	ClaimToken         string                     `json:"claim_token,omitempty" db:"claim_token"`
	LastError          string                     `json:"last_error,omitempty" db:"last_error"`
	ProviderStatusCode *int                       `json:"provider_status_code,omitempty" db:"provider_status_code"`
	SentAt             *time.Time                 `json:"sent_at,omitempty" db:"sent_at"`
	CreatedAt          time.Time                  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time                  `json:"updated_at" db:"updated_at"`

	SubscriptionEndpoint string `json:"-" db:"subscription_endpoint"`
	SubscriptionP256DH   string `json:"-" db:"subscription_p256dh"`
	SubscriptionAuth     string `json:"-" db:"subscription_auth"`
}

type NotificationDedupeClaim struct {
	UserID           int64            `json:"user_id" db:"user_id"`
	NotificationType NotificationType `json:"notification_type" db:"notification_type"`
	DedupeScope      string           `json:"dedupe_scope" db:"dedupe_scope"`
	DedupeKey        string           `json:"dedupe_key" db:"dedupe_key"`
	NotificationID   *int64           `json:"notification_id,omitempty" db:"notification_id"`
}

type SKUSyncFailureSource string

const (
	SKUSyncFailureSourceTaskFiling      SKUSyncFailureSource = "filing"
	SKUSyncFailureSourceProductBaseSync SKUSyncFailureSource = "pm_base"
)

type SKUSyncFailureItem struct {
	SKUItemID   int64  `json:"sku_item_id,omitempty"`
	SKUCode     string `json:"sku_code"`
	ProductName string `json:"product_name,omitempty"`
	Error       string `json:"error,omitempty"`
}

type SKUSyncFailureNotificationRequest struct {
	Source         SKUSyncFailureSource `json:"source"`
	TaskID         int64                `json:"task_id"`
	TaskNo         string               `json:"task_no,omitempty"`
	RecordID       int64                `json:"record_id,omitempty"`
	ERPSyncVersion int64                `json:"erp_sync_version,omitempty"`
	Summary        string               `json:"summary,omitempty"`
	FailureItems   []SKUSyncFailureItem `json:"failure_items"`
}
