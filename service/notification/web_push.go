package notification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

const (
	webPushChannel              = "web_push"
	webPushTestCooldown         = time.Minute
	maxWebPushPayloadBodyRunes  = 120
	maxSKUSyncFailureItemsShown = 20
)

type WebPushConfigView struct {
	Enabled      bool   `json:"enabled"`
	PublicKey    string `json:"public_key,omitempty"`
	VAPIDKeyHash string `json:"vapid_key_hash,omitempty"`
	Subject      string `json:"subject,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

type WebPushSubscriptionInput struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256DH string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	UserAgent string `json:"user_agent"`
	Platform  string `json:"platform"`
}

type NotificationPreferencesPatch struct {
	WebPushEnabled *bool `json:"web_push_enabled"`
}

type NotificationPreferencesView struct {
	UserID         int64      `json:"user_id"`
	WebPushEnabled bool       `json:"web_push_enabled"`
	LastTestSentAt *time.Time `json:"last_test_sent_at,omitempty"`
	VAPIDKeyHash   string     `json:"vapid_key_hash,omitempty"`
}

type WebPushSubscriptionView struct {
	ID           int64                            `json:"id"`
	UserID       int64                            `json:"user_id"`
	EndpointHash string                           `json:"endpoint_hash"`
	Status       domain.WebPushSubscriptionStatus `json:"status"`
	VAPIDKeyHash string                           `json:"vapid_key_hash,omitempty"`
	LastSeenAt   time.Time                        `json:"last_seen_at"`
}

type WebPushTestResult struct {
	NotificationID int64  `json:"notification_id,omitempty"`
	Queued         bool   `json:"queued"`
	Message        string `json:"message,omitempty"`
}

func (s *Service) WebPushConfig(ctx context.Context, actor domain.RequestActor) (*WebPushConfigView, *domain.AppError) {
	_ = ctx
	_ = actor
	view := &WebPushConfigView{
		Enabled:      s.webPushRuntimeEnabled(),
		PublicKey:    strings.TrimSpace(s.webPush.PublicKey),
		VAPIDKeyHash: strings.TrimSpace(s.webPush.KeyHash),
		Subject:      strings.TrimSpace(s.webPush.Subject),
	}
	if !view.Enabled {
		view.PublicKey = ""
		if strings.TrimSpace(s.webPush.PublicKey) == "" || strings.TrimSpace(s.webPush.PrivateKey) == "" {
			view.Reason = "web_push_keys_missing"
		} else {
			view.Reason = "web_push_disabled"
		}
	}
	return view, nil
}

func (s *Service) GetPreferences(ctx context.Context, actor domain.RequestActor) (*NotificationPreferencesView, *domain.AppError) {
	pref, err := s.notifications.GetNotificationPreference(ctx, actor.ID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	currentKeyHash := strings.TrimSpace(s.webPush.KeyHash)
	storedKeyHash := strings.TrimSpace(pref.VAPIDKeyHash)
	if storedKeyHash == "" {
		pref.VAPIDKeyHash = currentKeyHash
	} else if currentKeyHash != "" && storedKeyHash != currentKeyHash {
		if _, err := s.notifications.MarkStaleWebPushSubscriptionsByUserExceptKeyHash(ctx, actor.ID, currentKeyHash, s.now().UTC()); err != nil {
			s.logger.Warn("mark stale web push subscriptions failed", zap.Int64("user_id", actor.ID), zap.Error(err))
		}
	}
	return notificationPreferencesView(pref), nil
}

func (s *Service) PatchPreferences(ctx context.Context, actor domain.RequestActor, patch NotificationPreferencesPatch) (*NotificationPreferencesView, *domain.AppError) {
	pref, err := s.notifications.GetNotificationPreference(ctx, actor.ID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if patch.WebPushEnabled != nil {
		pref.WebPushEnabled = *patch.WebPushEnabled
	}
	pref.VAPIDKeyHash = strings.TrimSpace(s.webPush.KeyHash)
	if s.txRunner == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "notification transaction runner is not configured", nil)
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.notifications.UpsertNotificationPreference(ctx, tx, pref)
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	updated, err := s.notifications.GetNotificationPreference(ctx, actor.ID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return notificationPreferencesView(updated), nil
}

func (s *Service) RegisterWebPushSubscription(ctx context.Context, actor domain.RequestActor, input WebPushSubscriptionInput) (*WebPushSubscriptionView, *domain.AppError) {
	if !s.webPushRuntimeEnabled() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "web push is disabled", map[string]interface{}{"deny_code": "web_push_disabled"})
	}
	endpoint := strings.TrimSpace(input.Endpoint)
	p256dh := strings.TrimSpace(input.Keys.P256DH)
	auth := strings.TrimSpace(input.Keys.Auth)
	if endpoint == "" || p256dh == "" || auth == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "endpoint and keys are required", map[string]interface{}{"deny_code": "web_push_subscription_invalid"})
	}
	if s.txRunner == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "notification transaction runner is not configured", nil)
	}
	var saved *domain.WebPushSubscription
	now := s.now().UTC()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		saved, err = s.notifications.UpsertWebPushSubscription(ctx, tx, &domain.WebPushSubscription{
			UserID:       actor.ID,
			Endpoint:     endpoint,
			EndpointHash: endpointHash(endpoint),
			P256DH:       p256dh,
			Auth:         auth,
			UserAgent:    compactString(input.UserAgent, 512),
			Platform:     compactString(input.Platform, 64),
			VAPIDKeyHash: strings.TrimSpace(s.webPush.KeyHash),
			LastSeenAt:   now,
		})
		if err != nil {
			return err
		}
		return s.notifications.UpsertNotificationPreference(ctx, tx, &domain.NotificationPreference{
			UserID:         actor.ID,
			WebPushEnabled: true,
			VAPIDKeyHash:   strings.TrimSpace(s.webPush.KeyHash),
		})
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return webPushSubscriptionView(saved), nil
}

func (s *Service) DeleteCurrentWebPushSubscription(ctx context.Context, actor domain.RequestActor, endpoint string) *domain.AppError {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil
	}
	if _, err := s.notifications.DisableWebPushSubscriptionByEndpointHash(ctx, actor.ID, endpointHash(endpoint), s.now().UTC(), "client_deleted"); err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return nil
}

func (s *Service) SendWebPushTest(ctx context.Context, actor domain.RequestActor) (*WebPushTestResult, *domain.AppError) {
	if !s.webPushRuntimeEnabled() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "web push is disabled", map[string]interface{}{"deny_code": "web_push_disabled"})
	}
	pref, err := s.notifications.GetNotificationPreference(ctx, actor.ID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	now := s.now().UTC()
	if pref.LastTestSentAt != nil && now.Sub(*pref.LastTestSentAt) < webPushTestCooldown {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "web push test is rate limited", map[string]interface{}{"deny_code": "web_push_test_rate_limited"})
	}
	var n *domain.Notification
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		pref.WebPushEnabled = true
		pref.LastTestSentAt = &now
		pref.VAPIDKeyHash = strings.TrimSpace(s.webPush.KeyHash)
		if err := s.notifications.UpsertNotificationPreference(ctx, tx, pref); err != nil {
			return err
		}
		var createErr error
		n, createErr = s.CreateNotification(ctx, tx, actor.ID, domain.NotificationTypeSystemBroadcast, mustRaw(map[string]interface{}{
			"title":        "Web Push 测试通知",
			"content":      "如果你看到这条系统通知，说明当前设备已可以接收系统级提醒。",
			"broadcast_id": fmt.Sprintf("web_push_test_%d_%d", actor.ID, now.UnixNano()),
		}))
		return createErr
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	result := &WebPushTestResult{Queued: true, Message: "测试通知已写入投递队列"}
	if n != nil {
		result.NotificationID = n.ID
	}
	return result, nil
}

func (s *Service) CreateDedupedNotification(ctx context.Context, userID int64, ntype domain.NotificationType, payload json.RawMessage, dedupeScope, dedupeKey string) (*domain.Notification, bool, error) {
	if s == nil || s.txRunner == nil {
		return nil, false, fmt.Errorf("notification transaction runner is not configured")
	}
	dedupeScope = strings.TrimSpace(dedupeScope)
	dedupeKey = strings.TrimSpace(dedupeKey)
	if dedupeScope == "" || dedupeKey == "" {
		return nil, false, fmt.Errorf("dedupe scope and key are required")
	}
	var n *domain.Notification
	created := false
	err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		claim := domain.NotificationDedupeClaim{
			UserID:           userID,
			NotificationType: ntype,
			DedupeScope:      dedupeScope,
			DedupeKey:        dedupeKey,
		}
		ok, err := s.notifications.TryCreateNotificationDedupeClaim(ctx, tx, claim)
		if err != nil || !ok {
			return err
		}
		created = true
		n, err = s.CreateNotification(ctx, tx, userID, ntype, payload)
		if err != nil {
			return err
		}
		if n != nil {
			return s.notifications.UpdateNotificationDedupeClaimNotificationID(ctx, tx, claim, n.ID)
		}
		return nil
	})
	return n, created, err
}

func (s *Service) ClearNotificationDedupeScope(ctx context.Context, scope string) error {
	return s.notifications.ClearNotificationDedupeScope(ctx, strings.TrimSpace(scope))
}

func (s *Service) enqueueWebPushDeliveries(ctx context.Context, tx repo.Tx, n *domain.Notification) error {
	if !s.webPushRuntimeEnabled() || n == nil || n.UserID <= 0 {
		return nil
	}
	pref, err := s.notifications.GetNotificationPreference(ctx, n.UserID)
	if err != nil {
		return err
	}
	if !pref.WebPushEnabled {
		return nil
	}
	subs, err := s.notifications.ListActiveWebPushSubscriptionsForUser(ctx, tx, n.UserID, strings.TrimSpace(s.webPush.KeyHash))
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return nil
	}
	payload := s.webPushPayload(n)
	now := s.now().UTC()
	for _, sub := range subs {
		if err := s.notifications.EnqueueNotificationDelivery(ctx, tx, &domain.NotificationDeliveryOutbox{
			NotificationID: n.ID,
			SubscriptionID: sub.ID,
			UserID:         n.UserID,
			Channel:        webPushChannel,
			Payload:        payload,
			NextAttemptAt:  now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ProcessWebPushOutbox(ctx context.Context, limit int) (int, *domain.AppError) {
	if !s.webPushRuntimeEnabled() {
		return 0, nil
	}
	now := s.now().UTC()
	claimToken := fmt.Sprintf("webpush-%d", now.UnixNano())
	leaseTTL := s.webPush.LeaseTTL
	if leaseTTL <= 0 {
		leaseTTL = 2 * time.Minute
	}
	items, err := s.notifications.ClaimWebPushDeliveries(ctx, limit, claimToken, now.Add(leaseTTL), now)
	if err != nil {
		return 0, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return len(items), domain.NewAppError(domain.ErrCodeInternalError, ctx.Err().Error(), nil)
		}
		s.sendOneWebPushDelivery(ctx, item, claimToken)
	}
	return len(items), nil
}

func (s *Service) sendOneWebPushDelivery(ctx context.Context, item domain.NotificationDeliveryOutbox, claimToken string) {
	code, err := s.sendWebPush(ctx, item)
	now := s.now().UTC()
	if err == nil && code >= 200 && code < 300 {
		if markErr := s.notifications.MarkWebPushDeliverySent(ctx, item.ID, claimToken, now); markErr != nil {
			s.logger.Warn("mark web push sent failed", zap.Int64("delivery_id", item.ID), zap.Error(markErr))
		}
		return
	}
	statusCode := (*int)(nil)
	if code > 0 {
		statusCode = &code
	}
	errText := "web push send failed"
	if err != nil {
		errText = err.Error()
	}
	if isTerminalWebPushStatus(code) {
		if code == http.StatusGone || code == http.StatusNotFound {
			_ = s.notifications.DisableWebPushSubscriptionByID(ctx, item.SubscriptionID, now, fmt.Sprintf("provider_%d", code))
		}
		if markErr := s.notifications.MarkWebPushDeliveryDead(ctx, item.ID, claimToken, errText, statusCode, now); markErr != nil {
			s.logger.Warn("mark web push dead failed", zap.Int64("delivery_id", item.ID), zap.Error(markErr))
		}
		return
	}
	if item.AttemptCount+1 >= s.maxWebPushAttempts() {
		if markErr := s.notifications.MarkWebPushDeliveryDead(ctx, item.ID, claimToken, errText, statusCode, now); markErr != nil {
			s.logger.Warn("mark web push max attempts dead failed", zap.Int64("delivery_id", item.ID), zap.Error(markErr))
		}
		return
	}
	nextAttempt := now.Add(s.webPushRetryDelay(item.AttemptCount))
	if markErr := s.notifications.MarkWebPushDeliveryRetry(ctx, item.ID, claimToken, nextAttempt, errText, statusCode, now); markErr != nil {
		s.logger.Warn("mark web push retry failed", zap.Int64("delivery_id", item.ID), zap.Error(markErr))
	}
}

func (s *Service) sendWebPush(ctx context.Context, item domain.NotificationDeliveryOutbox) (int, error) {
	sub := &webpush.Subscription{
		Endpoint: item.SubscriptionEndpoint,
		Keys: webpush.Keys{
			Auth:   item.SubscriptionAuth,
			P256dh: item.SubscriptionP256DH,
		},
	}
	resp, err := webpush.SendNotificationWithContext(ctx, item.Payload, sub, &webpush.Options{
		Subscriber:      strings.TrimSpace(s.webPush.Subject),
		VAPIDPublicKey:  strings.TrimSpace(s.webPush.PublicKey),
		VAPIDPrivateKey: strings.TrimSpace(s.webPush.PrivateKey),
		TTL:             4 * 60 * 60,
	})
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if resp != nil {
			return resp.StatusCode, err
		}
		return 0, err
	}
	if resp == nil {
		return 0, fmt.Errorf("web push provider returned empty response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("web push provider status %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func (s *Service) ReconcileSKUSyncFailureNotifications(ctx context.Context, limit int) (int, *domain.AppError) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	total := 0
	filing, err := s.notifications.ListRecentTaskFilingFailures(ctx, limit)
	if err != nil {
		return total, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	for _, req := range filing {
		if err := s.NotifyTaskSKUSyncFailure(ctx, req); err != nil {
			s.logger.Warn("reconcile task filing failure notification failed", zap.Int64("task_id", req.TaskID), zap.Error(err))
		}
		total++
	}
	remaining := limit - total
	if remaining <= 0 {
		return total, nil
	}
	pm, err := s.notifications.ListRecentProductBaseSyncFailures(ctx, remaining)
	if err != nil {
		return total, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	for _, req := range pm {
		if err := s.NotifyTaskSKUSyncFailure(ctx, req); err != nil {
			s.logger.Warn("reconcile product base failure notification failed", zap.Int64("record_id", req.RecordID), zap.Error(err))
		}
		total++
	}
	return total, nil
}

func (s *Service) NotifyTaskSKUSyncFailure(ctx context.Context, req domain.SKUSyncFailureNotificationRequest) error {
	if req.TaskID <= 0 {
		return nil
	}
	task, err := s.tasks.GetByID(ctx, req.TaskID)
	if err != nil || task == nil {
		return err
	}
	recipients, err := s.resolveSKUSyncFailureRecipients(ctx, task)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return nil
	}
	req.TaskNo = firstNonEmpty(req.TaskNo, task.TaskNo)
	req.FailureItems = normalizeSKUSyncFailureItems(req.FailureItems)
	if len(req.FailureItems) == 0 {
		req.FailureItems = []domain.SKUSyncFailureItem{{SKUCode: task.SKUCode, ProductName: task.ProductNameSnapshot, Error: req.Summary}}
	}
	scope := skuSyncDedupeScope(req)
	key := skuSyncDedupeKey(req)
	payload := mustRaw(skuSyncFailurePayload(req, task))
	for _, userID := range recipients {
		if _, _, err := s.CreateDedupedNotification(ctx, userID, domain.NotificationTypeTaskSKUSyncFailed, payload, scope, key); err != nil {
			s.logger.Warn("create sku sync failure notification failed", zap.Int64("task_id", req.TaskID), zap.Int64("user_id", userID), zap.Error(err))
		}
	}
	return nil
}

func (s *Service) resolveSKUSyncFailureRecipients(ctx context.Context, task *domain.Task) ([]int64, error) {
	if task == nil || s.users == nil {
		return nil, nil
	}
	seen := map[int64]struct{}{}
	out := make([]int64, 0, 4)
	if task.CreatorID > 0 {
		user, err := s.users.GetByID(ctx, task.CreatorID)
		if err != nil {
			return nil, err
		}
		if user != nil && user.Status == domain.UserStatusActive {
			seen[user.ID] = struct{}{}
			out = append(out, user.ID)
		}
	}
	department := strings.TrimSpace(task.OwnerDepartment)
	team := strings.TrimSpace(task.OwnerTeam)
	if department == "" || team == "" {
		return out, nil
	}
	status := domain.UserStatusActive
	role := domain.RoleTeamLead
	dept := domain.Department(department)
	users, _, err := s.users.List(ctx, repo.UserListFilter{Status: &status, Role: &role, Department: &dept, Team: team, Page: 1, PageSize: 100})
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		if user == nil || user.ID <= 0 {
			continue
		}
		if _, ok := seen[user.ID]; ok {
			continue
		}
		seen[user.ID] = struct{}{}
		out = append(out, user.ID)
	}
	return out, nil
}

func (s *Service) webPushRuntimeEnabled() bool {
	return s != nil && s.webPush.Enabled &&
		strings.TrimSpace(s.webPush.PublicKey) != "" &&
		strings.TrimSpace(s.webPush.PrivateKey) != "" &&
		strings.TrimSpace(s.webPush.KeyHash) != ""
}

func (s *Service) webPushPayload(n *domain.Notification) json.RawMessage {
	p := payloadMap(n.Payload)
	text := notificationDisplayText(n.NotificationType, p)
	taskID := payloadInt64(p, "task_id")
	body := compactString(text.Content, maxWebPushPayloadBodyRunes)
	data := map[string]interface{}{
		"notification_id": n.ID,
		"type":            string(n.NotificationType),
		"title":           text.Title,
		"body":            body,
		"url":             notificationURL(taskID),
		"tag":             fmt.Sprintf("workflow-notification-%d", n.ID),
	}
	if taskID > 0 {
		data["task_id"] = taskID
	}
	return mustRaw(data)
}

func notificationURL(taskID int64) string {
	if taskID > 0 {
		return fmt.Sprintf("/tasks/%d", taskID)
	}
	return "/me/notifications"
}

type notificationText struct {
	Title   string
	Content string
}

func notificationDisplayText(ntype domain.NotificationType, p map[string]interface{}) notificationText {
	taskNo := strings.TrimSpace(payloadString(p, "task_no"))
	taskLabel := "该任务"
	if taskNo != "" {
		taskLabel = "任务 " + taskNo
	}
	switch ntype {
	case domain.NotificationTypeTaskSKUSyncFailed:
		count := payloadInt64(p, "failed_count")
		if count <= 0 {
			count = int64(len(payloadArray(p, "failed_items")))
		}
		return notificationText{Title: "SKU同步失败", Content: fmt.Sprintf("%s 有 %d 个SKU同步失败，请进入站内查看详情。", taskLabel, count)}
	case domain.NotificationTypeSystemBroadcast:
		return notificationText{Title: firstNonEmpty(payloadString(p, "title"), "系统广播"), Content: firstNonEmpty(payloadString(p, "content"), "你收到一条系统广播")}
	case domain.NotificationTypeTaskAssignedToMe:
		return notificationText{Title: "新任务分配", Content: taskLabel}
	case domain.NotificationTypeTaskPendingAudit:
		return notificationText{Title: "任务待审核", Content: taskLabel + "等待审核"}
	case domain.NotificationTypeTaskClosed:
		return notificationText{Title: "任务已结单", Content: taskLabel + "已结单"}
	default:
		return notificationText{Title: "系统通知", Content: "你收到一条新通知"}
	}
}

func payloadArray(p map[string]interface{}, keys ...string) []interface{} {
	for _, key := range keys {
		if value, ok := p[key].([]interface{}); ok {
			return value
		}
	}
	return nil
}

func isTerminalWebPushStatus(code int) bool {
	switch code {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusGone:
		return true
	default:
		return false
	}
}

func (s *Service) maxWebPushAttempts() int {
	if s.webPush.MaxAttempts <= 0 {
		return 5
	}
	return s.webPush.MaxAttempts
}

func (s *Service) webPushRetryDelay(attemptCount int) time.Duration {
	base := s.webPush.RetryBase
	if base <= 0 {
		base = 30 * time.Second
	}
	multiplier := 1 << minInt(attemptCount, 6)
	delay := time.Duration(multiplier) * base
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}

func endpointHash(endpoint string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(endpoint)))
	return hex.EncodeToString(sum[:])
}

func PublicKeyHash(publicKey string) string {
	publicKey = strings.TrimSpace(publicKey)
	if publicKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(publicKey))
	return hex.EncodeToString(sum[:])
}

func compactString(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func notificationPreferencesView(pref *domain.NotificationPreference) *NotificationPreferencesView {
	if pref == nil {
		return nil
	}
	return &NotificationPreferencesView{
		UserID:         pref.UserID,
		WebPushEnabled: pref.WebPushEnabled,
		LastTestSentAt: pref.LastTestSentAt,
		VAPIDKeyHash:   pref.VAPIDKeyHash,
	}
}

func webPushSubscriptionView(sub *domain.WebPushSubscription) *WebPushSubscriptionView {
	if sub == nil {
		return nil
	}
	return &WebPushSubscriptionView{
		ID:           sub.ID,
		UserID:       sub.UserID,
		EndpointHash: sub.EndpointHash,
		Status:       sub.Status,
		VAPIDKeyHash: sub.VAPIDKeyHash,
		LastSeenAt:   sub.LastSeenAt,
	}
}

func normalizeSKUSyncFailureItems(items []domain.SKUSyncFailureItem) []domain.SKUSyncFailureItem {
	out := make([]domain.SKUSyncFailureItem, 0, len(items))
	for _, item := range items {
		item.SKUCode = strings.TrimSpace(item.SKUCode)
		item.ProductName = strings.TrimSpace(item.ProductName)
		item.Error = normalizeFailureError(item.Error)
		if item.SKUCode == "" && item.Error == "" {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SKUCode == out[j].SKUCode {
			return out[i].SKUItemID < out[j].SKUItemID
		}
		return out[i].SKUCode < out[j].SKUCode
	})
	return out
}

func normalizeFailureError(value string) string {
	return compactString(value, 160)
}

func skuSyncDedupeScope(req domain.SKUSyncFailureNotificationRequest) string {
	switch req.Source {
	case domain.SKUSyncFailureSourceProductBaseSync:
		return fmt.Sprintf("task_sku_sync_failed:v1:pm_base:%d", req.RecordID)
	default:
		return fmt.Sprintf("task_sku_sync_failed:v1:filing:%d", req.TaskID)
	}
}

func skuSyncDedupeKey(req domain.SKUSyncFailureNotificationRequest) string {
	items := normalizeSKUSyncFailureItems(req.FailureItems)
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.SKUCode+":"+item.Error)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	switch req.Source {
	case domain.SKUSyncFailureSourceProductBaseSync:
		return fmt.Sprintf("task_sku_sync_failed:v1:pm_base:%d:%s", req.RecordID, hex.EncodeToString(sum[:]))
	default:
		return fmt.Sprintf("task_sku_sync_failed:v1:filing:%d:%d:%s", req.TaskID, req.ERPSyncVersion, hex.EncodeToString(sum[:]))
	}
}

func skuSyncFailurePayload(req domain.SKUSyncFailureNotificationRequest, task *domain.Task) map[string]interface{} {
	items := normalizeSKUSyncFailureItems(req.FailureItems)
	visible := items
	if len(visible) > maxSKUSyncFailureItemsShown {
		visible = visible[:maxSKUSyncFailureItemsShown]
	}
	failedItems := make([]map[string]interface{}, 0, len(visible))
	for _, item := range visible {
		row := map[string]interface{}{
			"sku_code": item.SKUCode,
			"error":    item.Error,
		}
		if item.SKUItemID > 0 {
			row["sku_item_id"] = item.SKUItemID
		}
		if item.ProductName != "" {
			row["product_name"] = item.ProductName
		}
		failedItems = append(failedItems, row)
	}
	return map[string]interface{}{
		"task_id":          req.TaskID,
		"task_no":          firstNonEmpty(req.TaskNo, task.TaskNo),
		"source":           string(req.Source),
		"record_id":        req.RecordID,
		"erp_sync_version": req.ERPSyncVersion,
		"failed_count":     len(items),
		"failed_items":     failedItems,
		"summary":          firstNonEmpty(req.Summary, summarizeFailureItems(items)),
		"url":              notificationURL(req.TaskID),
	}
}

func summarizeFailureItems(items []domain.SKUSyncFailureItem) string {
	if len(items) == 0 {
		return "SKU同步失败"
	}
	first := items[0]
	if first.Error != "" {
		return first.SKUCode + "：" + first.Error
	}
	return first.SKUCode + " 同步失败"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
