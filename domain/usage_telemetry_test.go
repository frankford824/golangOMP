package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUsagePrivacyAndAppAttribution(t *testing.T) {
	if got := SanitizeTraceURL("https://user:password@assets.yongbo.cloud/drive?q=private&token=secret#fragment"); got != "https://assets.yongbo.cloud/drive" {
		t.Fatal(got)
	}
	if got := UsageAppID("https://assets.yongbo.cloud", "", "main-ops"); got != "asset-workbench" {
		t.Fatal(got)
	}
	if got := UsageAppID("", "https://yongbo.cloud/tasks?token=secret", "asset-workbench"); got != "main-ops" {
		t.Fatal(got)
	}
	raw, usage, err := NormalizeUsagePayload([]byte(`{"telemetry_schema":"usage.v1","app_id":"main-ops","session_id":"session","visible_ms":999999,"file_size":12,"token":"secret","filename":"private.psd","signed_url":"https://x/?sig=secret","nested":{"password":"secret"},"route_pattern":"/tasks?q=private#fragment"}`), "asset-workbench", true)
	if err != nil || !usage {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "private") {
		t.Fatal(string(raw))
	}
	var d map[string]interface{}
	_ = json.Unmarshal(raw, &d)
	if d["app_id"] != "asset-workbench" || d["automation"] != true || d["visible_ms"] != float64(60000) || d["file_size"] != float64(12) {
		t.Fatal(d)
	}
	legacy := json.RawMessage(`{"feedback_note":"manual cost feedback","input":{"width":30}}`)
	out, usage, err := NormalizeUsagePayload(legacy, "unknown", false)
	if err != nil || usage || string(out) != string(legacy) {
		t.Fatal("legacy feedback changed")
	}
}

func TestUsageClassificationsAndBounds(t *testing.T) {
	actor := RequestActor{ID: 1}
	for _, tc := range []struct{ path, ua, automation, app, want string }{
		{"/api/cost/history", "node", "false", "unknown", "integration"},
		{"/v1/tasks", "HeadlessChrome", "false", "main-ops", "automation_hint"},
		{"/v1/tasks", "Mozilla", "false", "main-ops", "authenticated_browser_hint"},
		{"/v1/tasks", "sdk", "false", "unknown", "authenticated_api"},
	} {
		if got := UsageTrafficClass(actor, tc.path, tc.ua, tc.automation, tc.app); got != tc.want {
			t.Fatal(got, tc)
		}
	}
	if UsageRequestKind("GET", "/v1/me/notifications") != "polling_or_refresh" {
		t.Fatal("polling")
	}
	if UsageRequestKind("POST", "/v1/tasks/1/audit/decision") != "mutation_request" {
		t.Fatal("mutation")
	}
	if _, _, err := NormalizeUsagePayload([]byte(`{"telemetry_schema":"usage.v1"}`), "unknown", false); err == nil {
		t.Fatal("unknown app accepted")
	}
	if _, _, err := NormalizeUsagePayload([]byte(strings.Repeat(" ", 17000)), "main-ops", false); err == nil {
		t.Fatal("oversized payload accepted")
	}
}
