package notification

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"workflow/domain"
)

type captureWeComSender struct {
	messages []string
}

func (s *captureWeComSender) EnqueueMarkdown(_ context.Context, content string) error {
	s.messages = append(s.messages, content)
	return nil
}

func TestWeComNotifierFormatsTaskClosedConciseMessage(t *testing.T) {
	sender := &captureWeComSender{}
	notifier := NewWeComNotifier(sender, nil, nil, nil)
	notifier.Notify(context.Background(), domain.Notification{
		ID:               1,
		UserID:           10,
		NotificationType: domain.NotificationTypeTaskClosed,
		Payload: mustRaw(map[string]interface{}{
			"task_id":        1001,
			"task_no":        "RW-001",
			"closed_by_name": "王五",
		}),
	})

	if len(sender.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(sender.messages))
	}
	want := "已结单 | RW-001\n结单人: 王五"
	if sender.messages[0] != want {
		t.Fatalf("message = %q, want %q", sender.messages[0], want)
	}
}

func TestWeComNotifierSuppressesDuplicatePendingAuditMessages(t *testing.T) {
	sender := &captureWeComSender{}
	notifier := NewWeComNotifier(sender, nil, nil, nil)
	payload, _ := json.Marshal(map[string]interface{}{
		"task_id":        1002,
		"task_no":        "RW-002",
		"designer_name":  "李四",
		"pool_team_code": domain.TeamAuditStandard,
	})

	for i := int64(1); i <= 2; i++ {
		notifier.Notify(context.Background(), domain.Notification{
			ID:               i,
			UserID:           100 + i,
			NotificationType: domain.NotificationTypeTaskPendingAudit,
			Payload:          payload,
		})
	}

	if len(sender.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(sender.messages))
	}
	want := "待审核 | RW-002\n设计师: 李四  下一步: 常规审核组审核"
	if sender.messages[0] != want {
		t.Fatalf("message = %q, want %q", sender.messages[0], want)
	}
}

func TestWeComNotifierDoesNotExposeTechnicalTeamCode(t *testing.T) {
	sender := &captureWeComSender{}
	notifier := NewWeComNotifier(sender, nil, nil, nil)
	notifier.Notify(context.Background(), domain.Notification{
		ID:               3,
		UserID:           12,
		NotificationType: domain.NotificationTypeTaskPendingAudit,
		Payload: mustRaw(map[string]interface{}{
			"task_id":       1003,
			"task_no":       "RW-003",
			"designer_name": "张玉明",
			"team_name":     "audit_standard",
		}),
	})

	if len(sender.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(sender.messages))
	}
	if strings.Contains(sender.messages[0], "audit_standard") {
		t.Fatalf("message exposes technical code: %q", sender.messages[0])
	}
	want := "待审核 | RW-003\n设计师: 张玉明  下一步: 常规审核组审核"
	if sender.messages[0] != want {
		t.Fatalf("message = %q, want %q", sender.messages[0], want)
	}
}
