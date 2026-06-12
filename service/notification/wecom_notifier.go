package notification

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

type WeComMarkdownSender interface {
	EnqueueMarkdown(ctx context.Context, content string) error
}

type WeComNotifier struct {
	sender WeComMarkdownSender
	tasks  repo.TaskRepo
	users  repo.UserRepo
	logger *zap.Logger
	now    func() time.Time
	ttl    time.Duration

	mu   sync.Mutex
	seen map[string]time.Time
}

func NewWeComNotifier(sender WeComMarkdownSender, tasks repo.TaskRepo, users repo.UserRepo, logger *zap.Logger) *WeComNotifier {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &WeComNotifier{
		sender: sender,
		tasks:  tasks,
		users:  users,
		logger: logger,
		now:    time.Now,
		ttl:    2 * time.Minute,
		seen:   map[string]time.Time{},
	}
}

func (n *WeComNotifier) Notify(ctx context.Context, notification domain.Notification) {
	if n == nil || n.sender == nil {
		return
	}
	message, key, ok := n.format(ctx, notification)
	if !ok || strings.TrimSpace(message) == "" {
		return
	}
	if n.shouldSuppress(key) {
		return
	}
	if err := n.sender.EnqueueMarkdown(ctx, message); err != nil {
		n.logger.Warn("enqueue wecom notification failed",
			zap.Int64("notification_id", notification.ID),
			zap.String("notification_type", string(notification.NotificationType)),
			zap.Error(err))
	}
}

func (n *WeComNotifier) format(ctx context.Context, notification domain.Notification) (string, string, bool) {
	p := payloadMap(notification.Payload)
	taskID := payloadInt64(p, "task_id")
	if taskID <= 0 {
		return "", "", false
	}
	task := n.loadTask(ctx, taskID)
	taskLabel := weComTaskLabel(taskID, payloadString(p, "task_no"), task)
	switch notification.NotificationType {
	case domain.NotificationTypeTaskAssignedToMe:
		creator := firstNonEmpty(payloadString(p, "creator_name"), n.nameByID(ctx, payloadInt64(p, "creator_id")), payloadString(p, "assigned_by_name"), n.nameByID(ctx, payloadInt64(p, "assigned_by")), "未知")
		assignee := firstNonEmpty(payloadString(p, "assigned_to_name"), n.nameByID(ctx, payloadInt64(p, "assigned_to_id")), payloadString(p, "designer_name"), n.nameByID(ctx, payloadInt64(p, "designer_id")), "未指定")
		return fmt.Sprintf("新任务 | %s\n创建人: %s  负责人: %s", taskLabel, creator, assignee),
			fmt.Sprintf("%s:%d:%d", notification.NotificationType, taskID, notification.UserID), true
	case domain.NotificationTypeTaskPendingAudit:
		designer := firstNonEmpty(payloadString(p, "designer_name"), n.nameByID(ctx, payloadInt64(p, "designer_id")), "设计")
		auditTarget := firstNonEmpty(displayTeamLabel(payloadString(p, "pool_team_code"), payloadString(p, "team_name")), "审核组")
		return fmt.Sprintf("待审核 | %s\n设计师: %s  下一步: %s审核", taskLabel, designer, auditTarget),
			fmt.Sprintf("%s:%d:%s", notification.NotificationType, taskID, payloadString(p, "pool_team_code")), true
	case domain.NotificationTypeTaskClosed:
		operator := firstNonEmpty(payloadString(p, "closed_by_name"), n.nameByID(ctx, payloadInt64(p, "closed_by")), "系统")
		return fmt.Sprintf("已结单 | %s\n结单人: %s", taskLabel, operator),
			fmt.Sprintf("%s:%d", notification.NotificationType, taskID), true
	default:
		return "", "", false
	}
}

func (n *WeComNotifier) shouldSuppress(key string) bool {
	if key == "" {
		return false
	}
	now := n.now()
	n.mu.Lock()
	defer n.mu.Unlock()
	for seenKey, expiresAt := range n.seen {
		if !expiresAt.After(now) {
			delete(n.seen, seenKey)
		}
	}
	if expiresAt, ok := n.seen[key]; ok && expiresAt.After(now) {
		return true
	}
	n.seen[key] = now.Add(n.ttl)
	return false
}

func (n *WeComNotifier) loadTask(ctx context.Context, taskID int64) *domain.Task {
	if n.tasks == nil || taskID <= 0 {
		return nil
	}
	task, err := n.tasks.GetByID(ctx, taskID)
	if err != nil {
		n.logger.Debug("load task for wecom notification failed", zap.Int64("task_id", taskID), zap.Error(err))
		return nil
	}
	return task
}

func (n *WeComNotifier) nameByID(ctx context.Context, userID int64) string {
	if n.users == nil || userID <= 0 {
		return ""
	}
	user, err := n.users.GetByID(ctx, userID)
	if err != nil || user == nil {
		return ""
	}
	for _, candidate := range []string{user.DisplayName, user.Name, user.RealName, user.Username} {
		if value := strings.TrimSpace(candidate); value != "" {
			return value
		}
	}
	return fmt.Sprintf("用户%d", userID)
}

func weComTaskLabel(taskID int64, taskNo string, task *domain.Task) string {
	if taskNo = strings.TrimSpace(taskNo); taskNo != "" {
		return taskNo
	}
	if task != nil && strings.TrimSpace(task.TaskNo) != "" {
		return strings.TrimSpace(task.TaskNo)
	}
	return fmt.Sprintf("任务ID %d", taskID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}
