package domain

import (
	"encoding/json"
	"time"
)

type NotificationType string

const (
	NotificationTypeTaskAssignedToMe NotificationType = "task_assigned_to_me"
	NotificationTypeTaskRejected     NotificationType = "task_rejected"
	NotificationTypeClaimConflict    NotificationType = "claim_conflict"
	NotificationTypePoolReassigned   NotificationType = "pool_reassigned"
	NotificationTypeTaskCancelled    NotificationType = "task_cancelled"
)

func (t NotificationType) Valid() bool {
	switch t {
	case NotificationTypeTaskAssignedToMe,
		NotificationTypeTaskRejected,
		NotificationTypeClaimConflict,
		NotificationTypePoolReassigned,
		NotificationTypeTaskCancelled:
		return true
	default:
		return false
	}
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
