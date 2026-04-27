package domain

import (
	"encoding/json"
	"time"
)

type TaskDraft struct {
	ID          int64           `json:"draft_id" db:"id"`
	OwnerUserID int64           `json:"owner_user_id" db:"owner_user_id"`
	TaskType    string          `json:"task_type" db:"task_type"`
	Payload     json.RawMessage `json:"payload" db:"payload"`
	ExpiresAt   time.Time       `json:"expires_at" db:"expires_at"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

type TaskDraftPayloadRaw json.RawMessage

type TaskDraftListItem = TaskDraft
