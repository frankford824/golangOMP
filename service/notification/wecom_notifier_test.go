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
	want := "待审核 | RW-002\n设计师: 李四  下一步: 审核组审核"
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
	want := "待审核 | RW-003\n设计师: 张玉明  下一步: 审核组审核"
	if sender.messages[0] != want {
		t.Fatalf("message = %q, want %q", sender.messages[0], want)
	}
}

func TestWeComNotifierFormatsSystemBroadcastOnce(t *testing.T) {
	sender := &captureWeComSender{}
	notifier := NewWeComNotifier(sender, nil, nil, nil)
	payload := mustRaw(map[string]interface{}{
		"broadcast_id":              "broadcast-001",
		"title":                     "临时排班调整",
		"content":                   "今天 18:00 前完成待处理任务。",
		"broadcast_by_name":         "运营主管",
		"broadcast_audience":        "all",
		"broadcast_recipient_count": 42,
	})

	for i := int64(1); i <= 2; i++ {
		notifier.Notify(context.Background(), domain.Notification{
			ID:               i,
			UserID:           100 + i,
			NotificationType: domain.NotificationTypeSystemBroadcast,
			Payload:          payload,
		})
	}

	if len(sender.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(sender.messages))
	}
	want := "系统广播 | 临时排班调整\n今天 18:00 前完成待处理任务。\n发送人: 运营主管  接收: 全员 42人"
	if sender.messages[0] != want {
		t.Fatalf("message = %q, want %q", sender.messages[0], want)
	}
}

func TestWeComNotifierDoesNotSuppressChangedSKUSyncFailureFingerprint(t *testing.T) {
	sender := &captureWeComSender{}
	notifier := NewWeComNotifier(sender, nil, nil, nil)

	for _, failure := range []string{"ERP频控", "ERP成本不一致"} {
		notifier.Notify(context.Background(), domain.Notification{
			ID:               10,
			UserID:           100,
			NotificationType: domain.NotificationTypeTaskSKUSyncFailed,
			Payload: mustRaw(map[string]interface{}{
				"task_id":          9901,
				"task_no":          "RW-9901",
				"source":           string(domain.SKUSyncFailureSourceTaskFiling),
				"erp_sync_version": 3,
				"failed_count":     1,
				"failed_items": []map[string]interface{}{
					{"sku_code": "SKU-A", "error": failure},
				},
			}),
		})
	}

	if len(sender.messages) != 2 {
		t.Fatalf("messages = %d, want 2 for changed SKU failure payloads", len(sender.messages))
	}
}
