package domain

import "time"

type ServerLog struct {
	ID        int64          `json:"id"`
	Level     string         `json:"level"`
	Msg       string         `json:"msg"`
	Details   map[string]any `json:"details,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}
