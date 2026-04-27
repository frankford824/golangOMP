package notification

import (
	"encoding/json"
	"fmt"
	"strings"

	"workflow/domain"
)

type Candidate struct {
	UserID  int64
	Type    domain.NotificationType
	Payload json.RawMessage
}

func payloadMap(raw json.RawMessage) map[string]interface{} {
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = map[string]interface{}{}
	}
	return out
}

func payloadString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func payloadInt64(m map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		switch v := m[key].(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		case string:
			var out int64
			_, _ = fmt.Sscan(strings.TrimSpace(v), &out)
			return out
		}
	}
	return 0
}

func mustRaw(v interface{}) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}
