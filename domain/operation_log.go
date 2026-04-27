package domain

import (
	"encoding/json"
	"time"
)

type OperationLogSource string

const (
	OperationLogSourceTask        OperationLogSource = "task_event"
	OperationLogSourceExport      OperationLogSource = "export_event"
	OperationLogSourceIntegration OperationLogSource = "integration_call"
)

type OperationLogEntry struct {
	Source        OperationLogSource `json:"source"`
	LogID         string             `json:"log_id"`
	ReferenceType string             `json:"reference_type"`
	ReferenceID   string             `json:"reference_id"`
	EventType     string             `json:"event_type"`
	Summary       string             `json:"summary"`
	ActorID       *int64             `json:"actor_id,omitempty"`
	ActorType     string             `json:"actor_type,omitempty"`
	Status        string             `json:"status,omitempty"`
	Payload       json.RawMessage    `json:"payload,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
}
