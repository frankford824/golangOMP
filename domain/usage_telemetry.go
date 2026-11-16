package domain

import (
	"encoding/json"
	"net/url"
	"strings"
)

// These fields are analytics hints, never authorization or proof of a human.
func UsageAppID(origin, referer, declared string) string {
	for _, raw := range []string{origin, referer} {
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		switch strings.ToLower(u.Hostname()) {
		case "assets.yongbo.cloud":
			return "asset-workbench"
		case "yongbo.cloud", "www.yongbo.cloud":
			return "main-ops"
		}
	}
	switch declared {
	case "main-ops", "asset-workbench":
		return declared
	}
	return "unknown"
}

func SanitizeTraceURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "" && u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	return u.String()
}

func UsageRequestKind(method, path string) string {
	if strings.HasPrefix(path, "/v1/integration/") || strings.HasPrefix(path, "/api/cost/") || strings.HasPrefix(path, "/v1/public/") {
		return "integration"
	}
	if method != "GET" && method != "HEAD" && method != "OPTIONS" {
		return "mutation_request"
	}
	if strings.HasSuffix(path, "/preview") || strings.Contains(path, "/previews/") {
		return "media_read"
	}
	if strings.Contains(path, "/notifications") || strings.Contains(path, "/jobs/") || strings.HasSuffix(path, "/cost-specification-todos") {
		return "polling_or_refresh"
	}
	return "read_request"
}

func UsageTrafficClass(actor RequestActor, path, userAgent, automation, app string) string {
	if UsageRequestKind("GET", path) == "integration" {
		return "integration"
	}
	ua := strings.ToLower(userAgent)
	if automation == "true" || strings.Contains(ua, "headless") || strings.Contains(ua, "playwright") || strings.Contains(ua, "bot") || strings.Contains(ua, "python") || strings.Contains(ua, "curl/") {
		return "automation_hint"
	}
	if actor.ID <= 0 {
		return "anonymous"
	}
	if app != "unknown" {
		return "authenticated_browser_hint"
	}
	return "authenticated_api"
}

// New usage events have a deliberately narrow payload. Legacy business feedback
// is handled separately and does not become a usage.v1 event merely by arriving
// at the same endpoint.
func NormalizeUsagePayload(raw json.RawMessage, app string, automated bool) (json.RawMessage, bool, *AppError) {
	var input map[string]interface{}
	if len(raw) == 0 {
		return raw, false, nil
	}
	if len(raw) > 16*1024 {
		return nil, false, NewAppError(ErrCodeInvalidRequest, "trace payload exceeds 16 KiB", nil)
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, false, NewAppError(ErrCodeInvalidRequest, "trace payload must be an object", nil)
	}
	if input["telemetry_schema"] != "usage.v1" {
		return raw, false, nil
	}
	if app == "unknown" {
		return nil, true, NewAppError(ErrCodeInvalidRequest, "usage app is required", nil)
	}
	out := map[string]interface{}{"telemetry_schema": "usage.v1", "app_id": app, "automation": automated}
	for _, key := range []string{"session_id", "page_view_id", "client_event_id", "operation_id", "upload_session_id", "route_pattern", "phase", "phase_detail", "transfer_mode", "error_code", "error_type"} {
		if value, ok := input[key].(string); ok {
			if len(value) > 256 {
				return nil, true, NewAppError(ErrCodeInvalidRequest, "usage field too long", nil)
			}
			if key == "route_pattern" {
				value = SanitizeTraceURL(value)
			}
			out[key] = value
		}
	}
	for _, key := range []string{"file_size", "received_bytes", "duration_ms", "visible_ms", "http_status", "dropped_events", "requested_files", "successful_files", "failed_files"} {
		if value, ok := input[key].(float64); ok && value >= 0 && value <= 9007199254740991 {
			if key == "visible_ms" && value > 60000 {
				value = 60000
			}
			out[key] = value
		}
	}
	encoded, _ := json.Marshal(out)
	return encoded, true, nil
}
