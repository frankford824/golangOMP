package aiagent

import "testing"

func TestParseTaskSummaryTextFromFencedJSON(t *testing.T) {
	raw := "```json\n{\"headline\":\"任务正常推进\",\"current_status\":\"已进入审核\",\"people\":[],\"timeline\":[],\"stuck_points\":[],\"sku_asset_erp_cost\":[],\"next_actions\":[\"等待审核\"],\"confidence\":\"high\"}\n```"
	got, err := ParseTaskSummaryText(raw)
	if err != nil {
		t.Fatalf("ParseTaskSummaryText() error = %v", err)
	}
	if got.Headline != "任务正常推进" {
		t.Fatalf("headline = %q", got.Headline)
	}
	if len(got.NextActions) != 1 || got.NextActions[0] != "等待审核" {
		t.Fatalf("next actions = %#v", got.NextActions)
	}
}

func TestMessagesURL(t *testing.T) {
	got := messagesURL("https://api.minimaxi.com/anthropic/")
	want := "https://api.minimaxi.com/anthropic/v1/messages"
	if got != want {
		t.Fatalf("messagesURL() = %q, want %q", got, want)
	}
}
