package domain

import "encoding/json"

const (
	WebSocketEventTaskPoolCountChanged = "task_pool_count_changed"
	WebSocketEventMyTaskUpdated        = "my_task_updated"
	WebSocketEventNotificationArrived  = "notification_arrived"
)

type WebSocketEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func NewWebSocketEvent(eventType string, payload interface{}) WebSocketEvent {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = json.RawMessage(`{}`)
	}
	return WebSocketEvent{Type: eventType, Payload: raw}
}
