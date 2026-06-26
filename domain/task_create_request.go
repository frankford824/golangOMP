package domain

import "time"

const (
	TaskCreateRequestStatusInProgress = "in_progress"
	TaskCreateRequestStatusSucceeded  = "succeeded"
	TaskCreateRequestStatusFailed     = "failed"

	TaskCreateRequestReserveStarted         = "started"
	TaskCreateRequestReserveReplay          = "replay"
	TaskCreateRequestReserveInProgress      = "in_progress"
	TaskCreateRequestReservePayloadConflict = "payload_conflict"
)

type TaskCreateRequest struct {
	ID             int64      `db:"id" json:"id"`
	ClientCreateID string     `db:"client_create_id" json:"client_create_id"`
	ActorID        int64      `db:"actor_id" json:"actor_id"`
	PayloadHash    string     `db:"payload_hash" json:"payload_hash"`
	Status         string     `db:"status" json:"status"`
	TaskID         *int64     `db:"task_id" json:"task_id,omitempty"`
	ErrorMessage   string     `db:"error_message" json:"error_message,omitempty"`
	RequestPayload string     `db:"request_payload_json" json:"-"`
	ExpiresAt      *time.Time `db:"expires_at" json:"expires_at,omitempty"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
}
