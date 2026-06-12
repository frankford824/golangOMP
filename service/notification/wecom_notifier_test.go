package notification

import (
	"context"
	"encoding/json"
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
	want := "已结单 | RW-001\n操作: 王五"
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
		"pool_team_code": "audit_a",
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
	want := "待审核 | RW-002\n完成: 李四  审核: audit_a"
	if sender.messages[0] != want {
		t.Fatalf("message = %q, want %q", sender.messages[0], want)
	}
}
